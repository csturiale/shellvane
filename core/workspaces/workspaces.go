// Package workspaces stores named arrangements of tabs, splits and sessions,
// and the arrangement that was open when the app quit.
package workspaces

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrNameRequired errs.Code = "workspaces.nameRequired"
	ErrInvalid      errs.Code = "workspaces.invalid"
	ErrNotFound     errs.Code = "workspaces.notFound"
)

// lastID is the reserved row holding the session to restore at startup.
const lastID = "last"

// Workspace is a saved arrangement. Layout is JSON owned by the UI.
type Workspace struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Layout    string `json:"layout"`
	UpdatedAt string `json:"updatedAt"`
}

// Repo stores workspaces.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// New returns a Repo over a migrated database.
func New(db *sql.DB) *Repo { return &Repo{db: db, now: time.Now} }

// List returns the named workspaces sorted by name.
func (r *Repo) List(ctx context.Context) ([]Workspace, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, layout, updated_at FROM workspaces WHERE id <> ? ORDER BY name COLLATE NOCASE`, lastID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Workspace{}
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.Name, &w.Layout, &w.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Save stores layout under name, replacing a workspace with the same name.
func (r *Repo) Save(ctx context.Context, name, layout string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, errs.New(ErrNameRequired, nil)
	}
	if !json.Valid([]byte(layout)) {
		return Workspace{}, errs.New(ErrInvalid, nil)
	}
	w := Workspace{ID: uuid.NewString(), Name: name, Layout: layout, UpdatedAt: r.now().UTC().Format(time.RFC3339)}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO workspaces (id, name, layout, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (name) DO UPDATE SET layout = excluded.layout, updated_at = excluded.updated_at
		RETURNING id`, w.ID, w.Name, w.Layout, w.UpdatedAt).Scan(&w.ID)
	return w, err
}

// Delete removes a workspace.
func (r *Repo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ? AND id <> ?`, id, lastID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return nil
}

// SaveLast remembers the open session; an empty layout forgets it.
func (r *Repo) SaveLast(ctx context.Context, layout string) error {
	if layout == "" {
		_, err := r.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, lastID)
		return err
	}
	if !json.Valid([]byte(layout)) {
		return errs.New(ErrInvalid, nil)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO workspaces (id, name, layout, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET layout = excluded.layout, updated_at = excluded.updated_at`,
		lastID, "\x00last", layout, r.now().UTC().Format(time.RFC3339))
	return err
}

// Last returns the session saved by SaveLast, or "".
func (r *Repo) Last(ctx context.Context) (string, error) {
	var layout string
	err := r.db.QueryRowContext(ctx, `SELECT layout FROM workspaces WHERE id = ?`, lastID).Scan(&layout)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return layout, err
}
