package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/sync"
	"github.com/csturiale/shellvane/core/vault"
)

// EventSyncDone is emitted after a sync that changed something here.
const EventSyncDone = "sync:done"

// syncKeyScope is where the folder key is kept in the vault, and
// syncPasswordScope the WebDAV password or the S3 secret key.
const (
	syncKeyScope      = "sync:key"
	syncPasswordScope = "sync:password"
)

// Sync backends.
const (
	SyncFolder = "folder"
	SyncWebDAV = "webdav"
	SyncGit    = "git"
	SyncS3     = "s3"
)

// ErrSyncKind is returned for an unknown backend or a missing location.
const ErrSyncKind errs.Code = "sync.invalidSetup"

// SyncSetup chooses where the encrypted files go.
type SyncSetup struct {
	// Kind is "folder", "webdav", "git" or "s3".
	Kind   string `json:"kind"`
	Folder string `json:"folder"`
	// URL is the WebDAV collection, the Git repository or the S3 endpoint.
	URL string `json:"url"`
	// User and Password log in to the WebDAV server; for S3 they are the
	// access key and the secret key.
	User     string `json:"user"`
	Password string `json:"password"`
	// Region and Bucket locate the S3 bucket.
	Region     string `json:"region"`
	Bucket     string `json:"bucket"`
	Passphrase string `json:"passphrase"`
}

// syncInterval is how often automatic sync runs.
const syncInterval = 5 * time.Minute

// SyncStatus describes the sync setup.
type SyncStatus struct {
	Configured bool   `json:"configured"`
	Kind       string `json:"kind"`
	// Folder is where the files are: a path or a URL without credentials.
	Folder    string            `json:"folder"`
	User      string            `json:"user"`
	Auto      bool              `json:"auto"`
	LastSync  string            `json:"lastSync"`
	LastError *errs.Payload     `json:"lastError,omitempty"`
	Devices   []sync.DeviceInfo `json:"devices"`
}

type syncConfig struct {
	Kind   string `json:"kind,omitempty"` // empty: a folder (older setups)
	Folder string `json:"folder"`
	URL    string `json:"url,omitempty"`
	User   string `json:"user,omitempty"`
	Region string `json:"region,omitempty"`
	Bucket string `json:"bucket,omitempty"`
	Auto   bool   `json:"auto"`
}

func (c syncConfig) configured() bool { return c.Folder != "" || c.URL != "" }

// store opens the configured backend; the WebDAV password comes from the
// vault.
func (s *SyncService) store(ctx context.Context, c syncConfig) (sync.Store, error) {
	switch c.Kind {
	case "", SyncFolder:
		return sync.FolderStore{Folder: c.Folder}, nil
	case SyncWebDAV:
		pw, _, err := s.vault.Credential(ctx, syncPasswordScope)
		if err != nil {
			return nil, err
		}
		defer vault.Zero(pw)
		return sync.WebDAVStore{URL: c.URL, User: c.User, Password: string(pw)}, nil
	case SyncGit:
		return sync.GitStore{Remote: c.URL, Dir: filepath.Join(s.dataDir, "sync-git")}, nil
	case SyncS3:
		secret, _, err := s.vault.Credential(ctx, syncPasswordScope)
		if err != nil {
			return nil, err
		}
		defer vault.Zero(secret)
		return sync.S3Store{Endpoint: c.URL, Region: c.Region, Bucket: c.Bucket, AccessKey: c.User, SecretKey: string(secret)}, nil
	}
	return nil, errs.New(ErrSyncKind, nil)
}

// SyncService keeps this installation in step with others through a
// shared folder, end-to-end encrypted.
type SyncService struct {
	db      *sql.DB
	vault   *vault.Vault
	dataDir string
	emit    Emitter

	mu      gosync.Mutex
	cfg     syncConfig
	state   sync.State
	lastErr *errs.Payload
}

// NewSyncService loads the setup from dataDir and syncs every few minutes
// while the vault is unlocked.
func NewSyncService(db *sql.DB, v *vault.Vault, dataDir string, emit Emitter) *SyncService {
	s := &SyncService{db: db, vault: v, dataDir: dataDir, emit: emit}
	readJSON(filepath.Join(dataDir, "sync.json"), &s.cfg)
	readJSON(filepath.Join(dataDir, "sync-state.json"), &s.state)
	if s.state.Device == "" {
		s.state.Device = uuid.NewString()
	}
	go func() {
		time.Sleep(10 * time.Second) // let the app start
		for {
			s.mu.Lock()
			auto := s.cfg.Auto && s.cfg.configured()
			s.mu.Unlock()
			if auto {
				if st, _ := v.State(context.Background()); st == vault.StateUnlocked {
					s.SyncNow(context.Background())
				}
			}
			time.Sleep(syncInterval)
		}
	}()
	return s
}

func readJSON(path string, v any) {
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, v)
	}
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Status reports the setup and the other devices.
func (s *SyncService) Status(ctx context.Context) SyncStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	kind := s.cfg.Kind
	if kind == "" {
		kind = SyncFolder
	}
	st := SyncStatus{Configured: s.cfg.configured(), Kind: kind, Folder: s.cfg.Folder, User: s.cfg.User, Auto: s.cfg.Auto, LastSync: s.state.LastSync, LastError: s.lastErr, Devices: []sync.DeviceInfo{}}
	if st.Configured {
		store, err := s.store(ctx, s.cfg)
		if err != nil {
			return st
		}
		st.Folder = store.Where()
		if key, ok, _ := s.vault.Credential(ctx, syncKeyScope); ok {
			syncer := sync.Syncer{Store: store, Key: key}
			if devs, err := syncer.Devices(ctx, s.state.Device); err == nil && devs != nil {
				st.Devices = devs
			}
			vault.Zero(key)
		}
	}
	return st
}

// Setup uses a shared folder, a WebDAV collection or a Git repository with
// a passphrase (the same on every device), keeps the key in the vault
// (which must be unlocked) and syncs once.
func (s *SyncService) Setup(ctx context.Context, in SyncSetup) (sync.Report, error) {
	if len(in.Passphrase) < vault.MinPasswordLength {
		return sync.Report{}, errs.New(vault.ErrWeakPassword, errs.Params{"min": vault.MinPasswordLength})
	}
	cfg := syncConfig{Kind: in.Kind, Auto: true}
	switch in.Kind {
	case "", SyncFolder:
		cfg.Kind, cfg.Folder = SyncFolder, strings.TrimSpace(in.Folder)
		if cfg.Folder == "" {
			return sync.Report{}, errs.New(ErrSyncKind, nil)
		}
	case SyncWebDAV, SyncGit, SyncS3:
		cfg.URL, cfg.User = strings.TrimSpace(in.URL), strings.TrimSpace(in.User)
		cfg.Region, cfg.Bucket = strings.TrimSpace(in.Region), strings.TrimSpace(in.Bucket)
		if cfg.URL == "" || (in.Kind == SyncS3 && (cfg.Bucket == "" || cfg.User == "")) {
			return sync.Report{}, errs.New(ErrSyncKind, nil)
		}
	default:
		return sync.Report{}, errs.New(ErrSyncKind, nil)
	}
	if st, err := s.vault.State(ctx); err != nil || st != vault.StateUnlocked {
		return sync.Report{}, errs.New(vault.ErrLocked, nil)
	}
	if in.Kind == SyncWebDAV || in.Kind == SyncS3 {
		if err := s.vault.SaveCredential(ctx, syncPasswordScope, "Sync ("+in.Kind+")", []byte(in.Password)); err != nil {
			return sync.Report{}, err
		}
	}
	if in.Kind == SyncGit {
		os.RemoveAll(filepath.Join(s.dataDir, "sync-git")) // a fresh clone of this repository
	}
	store, err := s.store(ctx, cfg)
	if err != nil {
		return sync.Report{}, err
	}
	key, err := sync.OpenKeyIn(ctx, store, in.Passphrase, sync.DefaultKDF)
	if err != nil {
		return sync.Report{}, err
	}
	defer vault.Zero(key)
	if err := s.vault.SaveCredential(ctx, syncKeyScope, "Sync", key); err != nil {
		return sync.Report{}, err
	}
	s.mu.Lock()
	s.cfg = cfg
	err = writeJSON(filepath.Join(s.dataDir, "sync.json"), s.cfg)
	s.mu.Unlock()
	if err != nil {
		return sync.Report{}, err
	}
	return s.SyncNow(ctx)
}

// SetAuto turns automatic sync on or off.
func (s *SyncService) SetAuto(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg.Auto = on
	return writeJSON(filepath.Join(s.dataDir, "sync.json"), s.cfg)
}

// Disable stops syncing here; the folder and other devices are untouched.
func (s *SyncService) Disable(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = syncConfig{}
	s.state = sync.State{Device: s.state.Device}
	os.Remove(filepath.Join(s.dataDir, "sync.json"))
	os.Remove(filepath.Join(s.dataDir, "sync-state.json"))
	os.RemoveAll(filepath.Join(s.dataDir, "sync-git"))
	s.vault.DeleteCredential(ctx, syncPasswordScope)
	return s.vault.DeleteCredential(ctx, syncKeyScope)
}

// SyncNow syncs with the folder. The vault must be unlocked.
func (s *SyncService) SyncNow(ctx context.Context) (sync.Report, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cfg.configured() {
		return sync.Report{}, errs.New(sync.ErrNotConfigured, nil)
	}
	rep, err := s.syncLocked(ctx)
	if err != nil {
		p := errs.ToPayload(err)
		s.lastErr = &p
		return rep, err
	}
	s.lastErr = nil
	if rep.Updated > 0 || rep.Deleted > 0 {
		s.emit(EventSyncDone, rep)
	}
	return rep, nil
}

func (s *SyncService) syncLocked(ctx context.Context) (sync.Report, error) {
	key, ok, err := s.vault.Credential(ctx, syncKeyScope)
	if err != nil {
		return sync.Report{}, err
	}
	if !ok {
		return sync.Report{}, errs.New(sync.ErrNotConfigured, nil)
	}
	defer vault.Zero(key)
	name, _ := os.Hostname()
	store, err := s.store(ctx, s.cfg)
	if err != nil {
		return sync.Report{}, err
	}
	syncer := sync.Syncer{DB: s.db, Store: store, Key: key, Name: name}
	rep, err := syncer.Sync(ctx, &s.state)
	if err != nil {
		return rep, err
	}
	if rep.Conflicts == nil {
		rep.Conflicts = []string{}
	}
	return rep, writeJSON(filepath.Join(s.dataDir, "sync-state.json"), s.state)
}

// CloudFolders lists the cloud drive folders found on this computer
// (iCloud Drive, Google Drive, Dropbox, OneDrive, Syncthing), offered as
// shared folders.
func (s *SyncService) CloudFolders() []sync.CloudFolder { return sync.CloudFolders() }
