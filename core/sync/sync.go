// Package sync keeps several Shellvane installations in step through a
// folder they share (Syncthing, Dropbox, iCloud Drive, a network share).
//
// Each device writes only its own file, <device>.tsync, so the folder
// service never sees two devices editing the same file. A file is the
// device's full state, gzipped JSON encrypted with XChaCha20-Poly1305
// under a key derived (Argon2id) from the sync passphrase: the folder
// service sees nothing in clear. Merging keeps, for every record, the
// most recent change (updated_at); deletions travel as tombstones.
// Secrets (vault, keys) are not synced.
package sync

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes.
const (
	ErrWrongPassphrase errs.Code = "sync.wrongPassphrase"
	ErrFolder          errs.Code = "sync.folder"
	ErrCorrupt         errs.Code = "sync.corrupt"
	ErrNotConfigured   errs.Code = "sync.notConfigured"
)

// Tables are synced in this order (parents before children).
var Tables = []string{"groups", "hosts", "snippets", "tunnels", "rules", "environments", "runbooks", "themes"}

// tombstoneTTL is how long deletions are remembered.
const tombstoneTTL = 90 * 24 * time.Hour

// Record is one row, or the deletion of one.
type Record struct {
	Table   string         `json:"t"`
	ID      string         `json:"id"`
	Updated string         `json:"u"` // RFC 3339; compared as text
	Deleted bool           `json:"d,omitempty"`
	Row     map[string]any `json:"r,omitempty"`
}

func (r Record) key() string { return r.Table + "/" + r.ID }

// snapshot is a device's file.
type snapshot struct {
	Device  string   `json:"device"`
	Name    string   `json:"name"`
	Written string   `json:"written"`
	Records []Record `json:"records"`
}

// folderInfo is sync.json: the parameters every device derives the key with.
type folderInfo struct {
	Version  int    `json:"version"`
	Salt     []byte `json:"salt"`
	Time     uint32 `json:"time"`
	Memory   uint32 `json:"memory"`
	Threads  uint8  `json:"threads"`
	Verifier []byte `json:"verifier"` // verifierText encrypted with the key
}

// KDF are the Argon2id parameters of new sync folders.
type KDF struct {
	Time    uint32
	Memory  uint32
	Threads uint8
}

// DefaultKDF matches the vault's.
var DefaultKDF = KDF{Time: 3, Memory: 64 * 1024, Threads: 4}

// Format identifier from before the rename to Shellvane: never change it,
// or data written by earlier versions could no longer be read or verified.
const verifierText = "termora-sync"

// Dir is the folder Shellvane uses inside the shared folder.
func Dir(folder string) string { return filepath.Join(folder, DirName) }

// DirName is the folder Shellvane uses inside a shared folder.
const DirName = "Shellvane-sync"

// OpenKey derives the key of a shared folder; see OpenKeyIn.
func OpenKey(folder, passphrase string, kdf KDF) ([]byte, error) {
	return OpenKeyIn(context.Background(), FolderStore{Folder: folder}, passphrase, kdf)
}

// OpenKeyIn derives the store's key from the passphrase, setting the store
// up (with new parameters) when it is empty.
func OpenKeyIn(ctx context.Context, st Store, passphrase string, kdf KDF) ([]byte, error) {
	if sess, ok := st.(Session); ok {
		if err := sess.Begin(ctx); err != nil {
			return nil, err
		}
	}
	data, err := st.Get(ctx, "sync.json")
	if errors.Is(err, os.ErrNotExist) {
		info := folderInfo{Version: 1, Salt: make([]byte, 16), Time: kdf.Time, Memory: kdf.Memory, Threads: kdf.Threads}
		rand.Read(info.Salt)
		key := derive(passphrase, info)
		if info.Verifier, err = seal(key, []byte(verifierText)); err != nil {
			return nil, err
		}
		out, _ := json.MarshalIndent(info, "", "  ")
		if err := st.Put(ctx, "sync.json", out); err != nil {
			return nil, err
		}
		if sess, ok := st.(Session); ok {
			if err := sess.End(ctx, "setup"); err != nil {
				return nil, err
			}
		}
		return key, nil
	}
	if err != nil {
		return nil, err
	}
	var info folderInfo
	if err := json.Unmarshal(data, &info); err != nil || info.Version != 1 {
		return nil, errs.New(ErrCorrupt, errs.Params{"file": "sync.json"})
	}
	key := derive(passphrase, info)
	if v, err := open(key, info.Verifier); err != nil || string(v) != verifierText {
		return nil, errs.New(ErrWrongPassphrase, nil)
	}
	return key, nil
}

func derive(passphrase string, info folderInfo) []byte {
	return argon2.IDKey([]byte(passphrase), info.Salt, info.Time, info.Memory, info.Threads, chacha20poly1305.KeySize)
}

func seal(key, plain []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plain)+aead.Overhead())
	rand.Read(nonce)
	return aead.Seal(nonce, nonce, plain, []byte("termora-sync-v1")), nil
}

func open(key, data []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	if len(data) < aead.NonceSize() {
		return nil, errors.New("short")
	}
	return aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte("termora-sync-v1"))
}

// State is what a device remembers between syncs.
type State struct {
	Device string `json:"device"`
	// Known are the records present after the last sync (key → updated):
	// one missing now was deleted here.
	Known map[string]string `json:"known"`
	// Tombstones are deletions still being propagated.
	Tombstones map[string]Record `json:"tombstones"`
	LastSync   string            `json:"lastSync"`
}

// Report tells what a sync changed.
type Report struct {
	Updated   int      `json:"updated"`   // records changed here from other devices
	Deleted   int      `json:"deleted"`   // records deleted here
	Devices   int      `json:"devices"`   // other devices seen
	Conflicts []string `json:"conflicts"` // records that could not be applied
}

// Syncer merges a database with a store (a shared folder by default).
type Syncer struct {
	DB *sql.DB
	// Store holds the files; when nil, the shared folder Folder.
	Store  Store
	Folder string
	Key    []byte
	Name   string // this device's name, shown to the others
	Now    func() time.Time
}

// Sync reads the other devices' files, applies their newer changes here,
// and writes this device's file. state is updated in place.
func (s *Syncer) Sync(ctx context.Context, state *State) (Report, error) {
	var rep Report
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	stamp := now().UTC().Format(time.RFC3339Nano)
	if state.Known == nil {
		state.Known = map[string]string{}
	}
	if state.Tombstones == nil {
		state.Tombstones = map[string]Record{}
	}

	st := s.store()
	if sess, ok := st.(Session); ok {
		if err := sess.Begin(ctx); err != nil {
			return rep, err
		}
	}
	local, err := s.readLocal(ctx)
	if err != nil {
		return rep, err
	}
	// Records known at the last sync and gone now were deleted here.
	for k, upd := range state.Known {
		if _, ok := local[k]; !ok {
			if _, ok := state.Tombstones[k]; !ok {
				table, id, _ := strings.Cut(k, "/")
				// Dated now, or just after the version seen, if the clock lags.
				when := stamp
				if !after(stamp, upd) {
					t, _ := time.Parse(time.RFC3339Nano, upd)
					when = t.Add(time.Millisecond).UTC().Format(time.RFC3339Nano)
				}
				state.Tombstones[k] = Record{Table: table, ID: id, Updated: when, Deleted: true}
			}
		}
	}

	// The newest version of every record among all devices.
	best := map[string]Record{}
	for k, r := range local {
		best[k] = r
	}
	for k, t := range state.Tombstones {
		if r, ok := best[k]; !ok || !after(r.Updated, t.Updated) {
			best[k] = t
		}
	}
	remote, err := s.readRemote(ctx, state.Device)
	if err != nil {
		return rep, err
	}
	rep.Devices = len(remote)
	for _, snap := range remote {
		for _, r := range snap.Records {
			if cur, ok := best[r.key()]; !ok || after(r.Updated, cur.Updated) {
				best[r.key()] = r
			}
		}
	}

	// Apply what differs from here.
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return rep, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `PRAGMA defer_foreign_keys = ON`); err != nil {
		return rep, err
	}
	keys := make([]string, 0, len(best))
	for k := range best {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return order(keys[i]) < order(keys[j]) || (order(keys[i]) == order(keys[j]) && keys[i] < keys[j])
	})
	for _, k := range keys {
		r := best[k]
		cur, here := local[k]
		switch {
		case r.Deleted && here:
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+r.Table+` WHERE id = ?`, r.ID); err != nil {
				rep.Conflicts = append(rep.Conflicts, k)
				continue
			}
			rep.Deleted++
		case !r.Deleted && (!here || after(r.Updated, cur.Updated)):
			if err := upsert(ctx, tx, r); err != nil {
				rep.Conflicts = append(rep.Conflicts, k)
				delete(best, k)
				if here {
					best[k] = cur
				}
				continue
			}
			rep.Updated++
		}
	}
	if err := tx.Commit(); err != nil {
		return rep, err
	}

	// Remember the result and publish it.
	state.Known = map[string]string{}
	cutoff := now().Add(-tombstoneTTL).UTC().Format(time.RFC3339Nano)
	snap := snapshot{Device: state.Device, Name: s.Name, Written: stamp}
	for k, r := range best {
		if r.Deleted {
			if after(cutoff, r.Updated) {
				delete(state.Tombstones, k)
				continue
			}
			state.Tombstones[k] = r
		} else {
			delete(state.Tombstones, k)
			state.Known[k] = r.Updated
		}
		snap.Records = append(snap.Records, r)
	}
	sort.Slice(snap.Records, func(i, j int) bool { return snap.Records[i].key() < snap.Records[j].key() })
	if err := s.write(ctx, snap); err != nil {
		return rep, err
	}
	if sess, ok := st.(Session); ok {
		if err := sess.End(ctx, s.Name); err != nil {
			return rep, err
		}
	}
	state.LastSync = stamp
	return rep, nil
}

// after tells whether timestamp a is later than b (RFC 3339, any precision).
func after(a, b string) bool {
	ta, errA := time.Parse(time.RFC3339Nano, a)
	tb, errB := time.Parse(time.RFC3339Nano, b)
	if errA != nil || errB != nil {
		return a > b
	}
	return ta.After(tb)
}

func order(key string) int {
	table, _, _ := strings.Cut(key, "/")
	for i, t := range Tables {
		if t == table {
			return i
		}
	}
	return len(Tables)
}

// readLocal reads every row of the synced tables.
func (s *Syncer) readLocal(ctx context.Context) (map[string]Record, error) {
	out := map[string]Record{}
	for _, table := range Tables {
		rows, err := s.DB.QueryContext(ctx, `SELECT * FROM `+table)
		if err != nil {
			return nil, err
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return nil, err
		}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				rows.Close()
				return nil, err
			}
			row := map[string]any{}
			for i, c := range cols {
				if b, ok := vals[i].([]byte); ok {
					vals[i] = string(b)
				}
				row[c] = vals[i]
			}
			r := Record{Table: table, ID: fmt.Sprint(row["id"]), Updated: fmt.Sprint(row["updated_at"]), Row: row}
			out[r.key()] = r
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// upsert writes a row, keeping the children of an existing row (no REPLACE,
// which would delete it and cascade).
func upsert(ctx context.Context, tx *sql.Tx, r Record) error {
	if !validTable(r.Table) {
		return fmt.Errorf("unknown table %q", r.Table)
	}
	cols := make([]string, 0, len(r.Row))
	for c := range r.Row {
		if !validColumn(c) {
			return fmt.Errorf("bad column %q", c)
		}
		cols = append(cols, c)
	}
	sort.Strings(cols)
	args := make([]any, len(cols))
	sets := make([]string, 0, len(cols))
	for i, c := range cols {
		args[i] = jsonValue(r.Row[c])
		if c != "id" {
			sets = append(sets, c+" = excluded."+c)
		}
	}
	q := `INSERT INTO ` + r.Table + ` (` + strings.Join(cols, ", ") + `) VALUES (?` + strings.Repeat(", ?", len(cols)-1) + `)
		ON CONFLICT (id) DO UPDATE SET ` + strings.Join(sets, ", ")
	_, err := tx.ExecContext(ctx, q, args...)
	return err
}

func validTable(t string) bool {
	for _, x := range Tables {
		if x == t {
			return true
		}
	}
	return false
}

func validColumn(c string) bool {
	if c == "" {
		return false
	}
	for _, r := range c {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// jsonValue turns decoded JSON numbers back into integers when they are.
func jsonValue(v any) any {
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			return i
		}
		f, _ := n.Float64()
		return f
	}
	return v
}

func (s *Syncer) store() Store {
	if s.Store != nil {
		return s.Store
	}
	return FolderStore{Folder: s.Folder}
}

func (s *Syncer) write(ctx context.Context, snap snapshot) error {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := json.NewEncoder(zw).Encode(snap); err != nil {
		return err
	}
	zw.Close()
	data, err := seal(s.Key, buf.Bytes())
	if err != nil {
		return err
	}
	return s.store().Put(ctx, snap.Device+".tsync", data)
}

// readRemote reads the other devices' files; damaged or foreign ones
// (another key, a folder service's conflict copy) are skipped.
func (s *Syncer) readRemote(ctx context.Context, self string) ([]snapshot, error) {
	st := s.store()
	names, err := st.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []snapshot
	for _, name := range names {
		if !strings.HasSuffix(name, ".tsync") || strings.TrimSuffix(name, ".tsync") == self {
			continue
		}
		data, err := st.Get(ctx, name)
		if err != nil {
			continue
		}
		plain, err := open(s.Key, data)
		if err != nil {
			continue
		}
		zr, err := gzip.NewReader(bytes.NewReader(plain))
		if err != nil {
			continue
		}
		raw, err := io.ReadAll(io.LimitReader(zr, 256<<20))
		if err != nil {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		var snap snapshot
		if dec.Decode(&snap) == nil && snap.Device != self {
			out = append(out, snap)
		}
	}
	return out, nil
}

// Devices lists the other devices in the folder with when they last wrote.
func (s *Syncer) Devices(ctx context.Context, self string) ([]DeviceInfo, error) {
	snaps, err := s.readRemote(ctx, self)
	if err != nil {
		return nil, err
	}
	var out []DeviceInfo
	for _, sn := range snaps {
		out = append(out, DeviceInfo{ID: sn.Device, Name: sn.Name, Written: sn.Written})
	}
	return out, nil
}

// DeviceInfo describes another device.
type DeviceInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Written string `json:"written"`
}
