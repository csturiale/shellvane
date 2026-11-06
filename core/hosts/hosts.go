// Package hosts manages the host inventory: nested groups and the hosts they
// contain. Groups form a tree; an empty parent or group ID means the root.
// Deleting a group deletes everything below it.
package hosts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/settings"
)

// Error codes returned by this package.
const (
	ErrNameRequired     errs.Code = "hosts.nameRequired"
	ErrHostnameRequired errs.Code = "hosts.hostnameRequired"
	ErrHostnameInvalid  errs.Code = "hosts.hostnameInvalid"
	ErrUserInvalid      errs.Code = "hosts.userInvalid"
	ErrPortInvalid      errs.Code = "hosts.portInvalid"
	ErrGroupCycle       errs.Code = "hosts.groupCycle"
	ErrGroupNotFound    errs.Code = "hosts.groupNotFound"
	ErrHostNotFound     errs.Code = "hosts.hostNotFound"
)

// Group is a folder of hosts and other groups.
type Group struct {
	ID       string            `json:"id"`
	ParentID string            `json:"parentId"`
	Name     string            `json:"name"`
	Color    string            `json:"color"`
	Settings settings.Settings `json:"settings"`
}

// Host is a connection target.
type Host struct {
	ID       string            `json:"id"`
	GroupID  string            `json:"groupId"`
	Name     string            `json:"name"`
	Hostname string            `json:"hostname"`
	Notes    string            `json:"notes"`
	Settings settings.Settings `json:"settings"`
	Tags     []string          `json:"tags"`
	Favorite bool              `json:"favorite"`
	// LastUsed is the last connection (RFC 3339), "" for never.
	LastUsed string `json:"lastUsed"`
}

// GroupInput holds the editable fields of a group.
type GroupInput struct {
	ParentID string            `json:"parentId"`
	Name     string            `json:"name"`
	Color    string            `json:"color"`
	Settings settings.Settings `json:"settings"`
}

// HostInput holds the editable fields of a host.
type HostInput struct {
	GroupID  string            `json:"groupId"`
	Name     string            `json:"name"`
	Hostname string            `json:"hostname"`
	Notes    string            `json:"notes"`
	Settings settings.Settings `json:"settings"`
	Tags     []string          `json:"tags"`
	Favorite bool              `json:"favorite"`
}

// Inventory is the full tree, flat: clients rebuild the hierarchy from IDs.
type Inventory struct {
	Groups []Group `json:"groups"`
	Hosts  []Host  `json:"hosts"`
}

// Repo stores groups and hosts.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// New returns a Repo backed by a migrated database (see core/store).
func New(db *sql.DB) *Repo {
	return &Repo{db: db, now: time.Now}
}

// List returns all groups and hosts sorted by name.
func (r *Repo) List(ctx context.Context) (Inventory, error) {
	inv := Inventory{Groups: []Group{}, Hosts: []Host{}}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, COALESCE(parent_id, ''), name, color, settings FROM groups ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return inv, err
	}
	for rows.Next() {
		var g Group
		var raw string
		if err := rows.Scan(&g.ID, &g.ParentID, &g.Name, &g.Color, &raw); err != nil {
			rows.Close()
			return inv, err
		}
		if err := json.Unmarshal([]byte(raw), &g.Settings); err != nil {
			rows.Close()
			return inv, fmt.Errorf("group %s settings: %w", g.ID, err)
		}
		inv.Groups = append(inv.Groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return inv, err
	}

	rows, err = r.db.QueryContext(ctx,
		`SELECT id, COALESCE(group_id, ''), name, hostname, notes, settings, tags, favorite, last_used FROM hosts ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return inv, err
	}
	defer rows.Close()
	for rows.Next() {
		var h Host
		var raw, tags string
		if err := rows.Scan(&h.ID, &h.GroupID, &h.Name, &h.Hostname, &h.Notes, &raw, &tags, &h.Favorite, &h.LastUsed); err != nil {
			return inv, err
		}
		if err := json.Unmarshal([]byte(raw), &h.Settings); err != nil {
			return inv, fmt.Errorf("host %s settings: %w", h.ID, err)
		}
		h.Tags = parseTags(tags)
		inv.Hosts = append(inv.Hosts, h)
	}
	return inv, rows.Err()
}

// CreateGroup adds a group under in.ParentID.
func (r *Repo) CreateGroup(ctx context.Context, in GroupInput) (Group, error) {
	if err := validateGroup(&in); err != nil {
		return Group{}, err
	}
	if err := r.requireGroup(ctx, in.ParentID); err != nil {
		return Group{}, err
	}
	g := Group{ID: newID(), ParentID: in.ParentID, Name: in.Name, Color: in.Color, Settings: in.Settings}
	raw, err := json.Marshal(g.Settings)
	if err != nil {
		return Group{}, err
	}
	now := r.timestamp()
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO groups (id, parent_id, name, color, settings, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.ID, nullable(g.ParentID), g.Name, g.Color, string(raw), now, now)
	return g, err
}

// UpdateGroup replaces a group's fields, moving it if the parent changed.
func (r *Repo) UpdateGroup(ctx context.Context, id string, in GroupInput) (Group, error) {
	if err := validateGroup(&in); err != nil {
		return Group{}, err
	}
	if err := r.checkMove(ctx, id, in.ParentID); err != nil {
		return Group{}, err
	}
	raw, err := json.Marshal(in.Settings)
	if err != nil {
		return Group{}, err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE groups SET parent_id = ?, name = ?, color = ?, settings = ?, updated_at = ? WHERE id = ?`,
		nullable(in.ParentID), in.Name, in.Color, string(raw), r.timestamp(), id)
	if err := affectedOne(res, err, ErrGroupNotFound, id); err != nil {
		return Group{}, err
	}
	return Group{ID: id, ParentID: in.ParentID, Name: in.Name, Color: in.Color, Settings: in.Settings}, nil
}

// MoveGroup changes a group's parent. Moving a group into itself or one of
// its descendants fails with ErrGroupCycle.
func (r *Repo) MoveGroup(ctx context.Context, id, parentID string) error {
	if err := r.checkMove(ctx, id, parentID); err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE groups SET parent_id = ?, updated_at = ? WHERE id = ?`,
		nullable(parentID), r.timestamp(), id)
	return affectedOne(res, err, ErrGroupNotFound, id)
}

// GroupContents counts what a group contains, recursively.
type GroupContents struct {
	Groups int `json:"groups"`
	Hosts  int `json:"hosts"`
}

// Contents counts the subgroups and hosts below a group, so the UI can warn
// before deleting it.
func (r *Repo) Contents(ctx context.Context, id string) (GroupContents, error) {
	var c GroupContents
	err := r.db.QueryRowContext(ctx, `
		WITH RECURSIVE tree(id) AS (
			SELECT id FROM groups WHERE id = ?
			UNION ALL
			SELECT g.id FROM groups g JOIN tree t ON g.parent_id = t.id
		)
		SELECT (SELECT COUNT(*) - 1 FROM tree),
		       (SELECT COUNT(*) FROM hosts WHERE group_id IN (SELECT id FROM tree))`, id).Scan(&c.Groups, &c.Hosts)
	if err != nil {
		return c, err
	}
	if c.Groups < 0 {
		return GroupContents{}, errs.New(ErrGroupNotFound, errs.Params{"id": id})
	}
	return c, nil
}

// DeleteGroup deletes a group with all its subgroups and hosts.
func (r *Repo) DeleteGroup(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id)
	return affectedOne(res, err, ErrGroupNotFound, id)
}

// GroupLabel describes where a host sits: the group path from the root and
// the color of the nearest group that has one.
type GroupLabel struct {
	Path  []string `json:"path"`
	Color string   `json:"color"`
}

// Label returns the path and color of a group ("" is the root).
func (r *Repo) Label(ctx context.Context, groupID string) (GroupLabel, error) {
	var l GroupLabel
	seen := map[string]bool{}
	for id := groupID; id != "" && !seen[id]; {
		seen[id] = true
		var name, color, parent string
		err := r.db.QueryRowContext(ctx, `SELECT name, color, COALESCE(parent_id, '') FROM groups WHERE id = ?`, id).
			Scan(&name, &color, &parent)
		if errors.Is(err, sql.ErrNoRows) {
			return GroupLabel{}, errs.New(ErrGroupNotFound, errs.Params{"id": id})
		}
		if err != nil {
			return GroupLabel{}, err
		}
		l.Path = append([]string{name}, l.Path...)
		if l.Color == "" {
			l.Color = color
		}
		id = parent
	}
	return l, nil
}

// Host returns one host.
func (r *Repo) Host(ctx context.Context, id string) (Host, error) {
	h := Host{ID: id}
	var raw, tags string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(group_id, ''), name, hostname, notes, settings, tags, favorite, last_used FROM hosts WHERE id = ?`, id).
		Scan(&h.GroupID, &h.Name, &h.Hostname, &h.Notes, &raw, &tags, &h.Favorite, &h.LastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return Host{}, errs.New(ErrHostNotFound, errs.Params{"id": id})
	}
	if err != nil {
		return Host{}, err
	}
	h.Tags = parseTags(tags)
	return h, json.Unmarshal([]byte(raw), &h.Settings)
}

// Resolve returns a host's effective settings: its own, then its groups',
// then the built-in defaults.
func (r *Repo) Resolve(ctx context.Context, h Host) (settings.Resolved, error) {
	chain, err := r.Chain(ctx, h.GroupID)
	if err != nil {
		return settings.Resolved{}, err
	}
	return settings.Resolve(append([]settings.Level{{ID: h.ID, Settings: h.Settings}}, chain...)...), nil
}

// CreateHost adds a host to in.GroupID.
func (r *Repo) CreateHost(ctx context.Context, in HostInput) (Host, error) {
	if err := validateHost(&in); err != nil {
		return Host{}, err
	}
	if err := r.requireGroup(ctx, in.GroupID); err != nil {
		return Host{}, err
	}
	h := Host{ID: newID(), GroupID: in.GroupID, Name: in.Name, Hostname: in.Hostname, Notes: in.Notes, Settings: in.Settings,
		Tags: normalizeTags(in.Tags), Favorite: in.Favorite}
	raw, err := json.Marshal(h.Settings)
	if err != nil {
		return Host{}, err
	}
	tags, _ := json.Marshal(h.Tags)
	now := r.timestamp()
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO hosts (id, group_id, name, hostname, notes, settings, tags, favorite, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		h.ID, nullable(h.GroupID), h.Name, h.Hostname, h.Notes, string(raw), string(tags), h.Favorite, now, now)
	return h, err
}

// UpdateHost replaces a host's fields.
func (r *Repo) UpdateHost(ctx context.Context, id string, in HostInput) (Host, error) {
	if err := validateHost(&in); err != nil {
		return Host{}, err
	}
	if err := r.requireGroup(ctx, in.GroupID); err != nil {
		return Host{}, err
	}
	raw, err := json.Marshal(in.Settings)
	if err != nil {
		return Host{}, err
	}
	in.Tags = normalizeTags(in.Tags)
	tags, _ := json.Marshal(in.Tags)
	res, err := r.db.ExecContext(ctx,
		`UPDATE hosts SET group_id = ?, name = ?, hostname = ?, notes = ?, settings = ?, tags = ?, favorite = ?, updated_at = ? WHERE id = ?`,
		nullable(in.GroupID), in.Name, in.Hostname, in.Notes, string(raw), string(tags), in.Favorite, r.timestamp(), id)
	if err := affectedOne(res, err, ErrHostNotFound, id); err != nil {
		return Host{}, err
	}
	return r.Host(ctx, id)
}

// SetFavorite marks or unmarks a host as a favorite.
func (r *Repo) SetFavorite(ctx context.Context, id string, on bool) error {
	res, err := r.db.ExecContext(ctx, `UPDATE hosts SET favorite = ? WHERE id = ?`, on, id)
	return affectedOne(res, err, ErrHostNotFound, id)
}

// Touch records a connection to the host now.
func (r *Repo) Touch(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE hosts SET last_used = ? WHERE id = ?`, r.timestamp(), id)
	return affectedOne(res, err, ErrHostNotFound, id)
}

// DuplicateHost copies a host (settings, tags, notes) as "<name> (copy)"
// in the same group.
func (r *Repo) DuplicateHost(ctx context.Context, id, copySuffix string) (Host, error) {
	h, err := r.Host(ctx, id)
	if err != nil {
		return Host{}, err
	}
	return r.CreateHost(ctx, HostInput{GroupID: h.GroupID, Name: h.Name + copySuffix, Hostname: h.Hostname, Notes: h.Notes,
		Settings: h.Settings, Tags: h.Tags})
}

// MoveHost puts a host in another group ("" for the root).
func (r *Repo) MoveHost(ctx context.Context, id, groupID string) error {
	if err := r.requireGroup(ctx, groupID); err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE hosts SET group_id = ?, updated_at = ? WHERE id = ?`,
		nullable(groupID), r.timestamp(), id)
	return affectedOne(res, err, ErrHostNotFound, id)
}

// DeleteHost deletes a host.
func (r *Repo) DeleteHost(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM hosts WHERE id = ?`, id)
	return affectedOne(res, err, ErrHostNotFound, id)
}

// Chain returns the inheritance chain for settings resolution, from the given
// group up to the root, followed by the built-in defaults.
func (r *Repo) Chain(ctx context.Context, groupID string) ([]settings.Level, error) {
	var chain []settings.Level
	seen := map[string]bool{}
	for id := groupID; id != ""; {
		if seen[id] {
			return nil, errs.New(ErrGroupCycle, errs.Params{"id": id})
		}
		seen[id] = true

		var parent, raw string
		err := r.db.QueryRowContext(ctx, `SELECT COALESCE(parent_id, ''), settings FROM groups WHERE id = ?`, id).Scan(&parent, &raw)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errs.New(ErrGroupNotFound, errs.Params{"id": id})
		}
		if err != nil {
			return nil, err
		}
		lvl := settings.Level{ID: id}
		if err := json.Unmarshal([]byte(raw), &lvl.Settings); err != nil {
			return nil, err
		}
		chain = append(chain, lvl)
		id = parent
	}
	return append(chain, settings.Defaults()), nil
}

func (r *Repo) checkMove(ctx context.Context, id, parentID string) error {
	if parentID == "" {
		return nil
	}
	if parentID == id {
		return errs.New(ErrGroupCycle, errs.Params{"id": id})
	}
	// Walk up from the new parent: reaching id means id is an ancestor.
	chain, err := r.Chain(ctx, parentID)
	if err != nil {
		return err
	}
	for _, lvl := range chain {
		if lvl.ID == id {
			return errs.New(ErrGroupCycle, errs.Params{"id": id})
		}
	}
	return nil
}

func (r *Repo) requireGroup(ctx context.Context, id string) error {
	if id == "" {
		return nil
	}
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM groups WHERE id = ?`, id).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.New(ErrGroupNotFound, errs.Params{"id": id})
	}
	return err
}

func (r *Repo) timestamp() string {
	return r.now().UTC().Format(time.RFC3339Nano)
}

func validateGroup(in *GroupInput) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return errs.New(ErrNameRequired, nil)
	}
	return validateSettings(in.Settings)
}

func validateHost(in *HostInput) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Hostname = strings.TrimSpace(in.Hostname)
	if in.Hostname == "" {
		return errs.New(ErrHostnameRequired, nil)
	}
	if !ValidHostname(in.Hostname) {
		return errs.New(ErrHostnameInvalid, errs.Params{"hostname": in.Hostname})
	}
	if in.Name == "" {
		in.Name = in.Hostname
	}
	return validateSettings(in.Settings)
}

func validateSettings(s settings.Settings) error {
	if s.Port != nil && (*s.Port < 1 || *s.Port > 65535) {
		return errs.New(ErrPortInvalid, errs.Params{"port": *s.Port})
	}
	if s.User != nil && !ValidUser(*s.User) {
		return errs.New(ErrUserInvalid, errs.Params{"user": *s.User})
	}
	return nil
}

// ValidHostname accepts host names, IPv4 and IPv6 addresses (with a zone):
// no spaces, no shell characters and no leading '-', so a host name can
// never become an option of ssh or a command in a ProxyCommand.
func ValidHostname(h string) bool {
	if h == "" || h[0] == '-' {
		return false
	}
	for _, r := range h {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_' || r == ':' || r == '%' || r == '[' || r == ']':
		case r > 127 && unicode.IsLetter(r): // internationalized names
		default:
			return false
		}
	}
	return true
}

// ValidUser accepts user names, including DOMAIN\user and user@domain,
// without shell characters, spaces or a leading '-' (as OpenSSH does).
func ValidUser(u string) bool {
	if u == "" {
		return true
	}
	if u[0] == '-' {
		return false
	}
	for _, r := range u {
		if r < 32 || r == 127 || unicode.IsSpace(r) || strings.ContainsRune("'\"`$;&|<>(){}*?!#~", r) {
			return false
		}
	}
	return true
}

func affectedOne(res sql.Result, err error, notFound errs.Code, id string) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errs.New(notFound, errs.Params{"id": id})
	}
	return nil
}

func nullable(id string) any {
	if id == "" {
		return nil
	}
	return id
}

// newID returns a time-ordered UUIDv7, suitable for future sync.
func newID() string {
	return uuid.Must(uuid.NewV7()).String()
}

func parseTags(raw string) []string {
	var tags []string
	if json.Unmarshal([]byte(raw), &tags) != nil || tags == nil {
		return []string{}
	}
	return tags
}

// normalizeTags trims, drops empty and duplicate tags (case-insensitive)
// and sorts them.
func normalizeTags(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(t), "#"))
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}
