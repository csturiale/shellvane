// Package environments stores environments: everything needed to work on
// a system (cloud login, jump host, commands, tunnels, kubectl contexts,
// layout), started in order with one click.
package environments

import (
	"context"
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrNameRequired errs.Code = "environments.nameRequired"
	ErrNotFound     errs.Code = "environments.notFound"
	ErrDuplicate    errs.Code = "environments.duplicate"
	ErrInvalid      errs.Code = "environments.invalid"
)

// Variable is asked when the environment starts; {{name}} is replaced in
// every command.
type Variable struct {
	Name    string   `json:"name" yaml:"name"`
	Default string   `json:"default" yaml:"default,omitempty"`
	Choices []string `json:"choices" yaml:"choices,omitempty"`
}

// LocalLogin checks a login on this computer and logs in when the check
// fails, e.g. `gcloud auth print-access-token` / `gcloud auth login`.
type LocalLogin struct {
	Check string `json:"check" yaml:"check,omitempty"`
	Login string `json:"login" yaml:"login,omitempty"`
	// Prepare runs on this computer once logged in, without a terminal,
	// e.g. `gcloud container clusters get-credentials …` so that the
	// kubectl context exists locally.
	Prepare string `json:"prepare" yaml:"prepare,omitempty"`
}

// Remote is the jump host and what runs there.
type Remote struct {
	HostID string `json:"hostId" yaml:"host,omitempty"`
	// Terminal opens a pane; without it the connection serves the tunnels.
	Terminal bool `json:"terminal" yaml:"terminal"`
	// Script runs after the host's own startup script.
	Script        []string `json:"script" yaml:"script,omitempty"`
	KubeContext   string   `json:"kubeContext" yaml:"kubeContext,omitempty"`
	KubeNamespace string   `json:"kubeNamespace" yaml:"kubeNamespace,omitempty"`
}

// LocalKube opens local terminals whose kubectl context reaches the
// cluster through a tunnel of the environment.
type LocalKube struct {
	Context   string `json:"context" yaml:"context,omitempty"`
	Namespace string `json:"namespace" yaml:"namespace,omitempty"`
	// TunnelID is a dynamic (SOCKS) tunnel used as proxy-url, or a local
	// forward to the API server that replaces its address.
	TunnelID string `json:"tunnelId" yaml:"tunnel,omitempty"`
	// Terminals is how many local panes to open (0 = none).
	Terminals int `json:"terminals" yaml:"terminals"`
}

// Environment is a saved environment.
type Environment struct {
	ID          string     `json:"id" yaml:"-"`
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description,omitempty"`
	Variables   []Variable `json:"variables" yaml:"variables,omitempty"`
	Login       LocalLogin `json:"login" yaml:"login,omitempty"`
	Remote      Remote     `json:"remote" yaml:"remote,omitempty"`
	TunnelIDs   []string   `json:"tunnelIds" yaml:"tunnels,omitempty"`
	LocalKube   LocalKube  `json:"localKube" yaml:"localKube,omitempty"`
	// WorkspaceID opens a saved workspace at the end.
	WorkspaceID string `json:"workspaceId" yaml:"workspace,omitempty"`
}

// Repo stores environments.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// New returns a Repo over a migrated database.
func New(db *sql.DB) *Repo { return &Repo{db: db, now: time.Now} }

// List returns every environment sorted by name.
func (r *Repo) List(ctx context.Context) ([]Environment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, data FROM environments ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Environment{}
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var e Environment
		if err := json.Unmarshal([]byte(data), &e); err != nil {
			continue
		}
		e.ID = id
		out = append(out, normalize(e))
	}
	return out, rows.Err()
}

// Get returns one environment.
func (r *Repo) Get(ctx context.Context, id string) (Environment, error) {
	var data string
	if err := r.db.QueryRowContext(ctx, `SELECT data FROM environments WHERE id = ?`, id).Scan(&data); err != nil {
		return Environment{}, errs.New(ErrNotFound, errs.Params{"id": id})
	}
	var e Environment
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		return Environment{}, errs.Wrap(err, ErrInvalid, nil)
	}
	e.ID = id
	return normalize(e), nil
}

// Save creates (empty ID) or replaces an environment.
func (r *Repo) Save(ctx context.Context, e Environment) (Environment, error) {
	e = normalize(e)
	if e.Name == "" {
		return Environment{}, errs.New(ErrNameRequired, nil)
	}
	if e.ID == "" {
		e.ID = uuid.NewString()
	}
	data, _ := json.Marshal(e)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO environments (id, name, data, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, data = excluded.data, updated_at = excluded.updated_at`,
		e.ID, e.Name, string(data), r.now().UTC().Format(time.RFC3339))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return Environment{}, errs.New(ErrDuplicate, errs.Params{"name": e.Name})
	}
	return e, err
}

// Delete removes an environment.
func (r *Repo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM environments WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return nil
}

// Export renders an environment as YAML to share. Host, tunnel and
// workspace references are replaced by their names (resolve maps an ID to
// a name), since IDs mean nothing on another computer.
func Export(e Environment, resolve func(kind, id string) string) (string, error) {
	x := e
	x.Remote.HostID = resolve("host", e.Remote.HostID)
	x.TunnelIDs = nil
	for _, id := range e.TunnelIDs {
		x.TunnelIDs = append(x.TunnelIDs, resolve("tunnel", id))
	}
	x.LocalKube.TunnelID = resolve("tunnel", e.LocalKube.TunnelID)
	x.WorkspaceID = resolve("workspace", e.WorkspaceID)
	b, err := yaml.Marshal(x)
	return "# Shellvane environment\n" + string(b), err
}

// Import reads an exported environment; lookup maps names back to IDs
// ("" when nothing has that name).
func Import(text string, lookup func(kind, name string) string) (Environment, error) {
	var e Environment
	if err := yaml.Unmarshal([]byte(text), &e); err != nil {
		return Environment{}, errs.Wrap(err, ErrInvalid, nil)
	}
	e.Remote.HostID = lookup("host", e.Remote.HostID)
	var tunnels []string
	for _, n := range e.TunnelIDs {
		if id := lookup("tunnel", n); id != "" {
			tunnels = append(tunnels, id)
		}
	}
	e.TunnelIDs = tunnels
	e.LocalKube.TunnelID = lookup("tunnel", e.LocalKube.TunnelID)
	e.WorkspaceID = lookup("workspace", e.WorkspaceID)
	e.ID = ""
	e = normalize(e)
	if e.Name == "" {
		return Environment{}, errs.New(ErrNameRequired, nil)
	}
	return e, nil
}

func normalize(e Environment) Environment {
	e.Name = strings.TrimSpace(e.Name)
	if e.Variables == nil {
		e.Variables = []Variable{}
	}
	if e.TunnelIDs == nil {
		e.TunnelIDs = []string{}
	}
	if e.Remote.Script == nil {
		e.Remote.Script = []string{}
	}
	return e
}

var varRef = regexp.MustCompile(`\{\{\s*([\w.-]+)\s*\}\}`)

// Expand replaces {{name}} with the values; unknown names stay.
func Expand(s string, values map[string]string) string {
	return varRef.ReplaceAllStringFunc(s, func(m string) string {
		name := varRef.FindStringSubmatch(m)[1]
		if v, ok := values[name]; ok {
			return v
		}
		return m
	})
}
