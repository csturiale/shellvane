// Package snippets stores reusable commands. A snippet body may contain
// variables ({{name}} or {{name:a|b}}) that the UI asks for before sending.
package snippets

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrNameRequired errs.Code = "snippets.nameRequired"
	ErrBodyRequired errs.Code = "snippets.bodyRequired"
	ErrNotFound     errs.Code = "snippets.notFound"
)

// Snippet is a saved command.
type Snippet struct {
	ID     string `json:"id"`
	Folder string `json:"folder"`
	Name   string `json:"name"`
	Body   string `json:"body"`
	// Run presses Enter after the body is sent.
	Run bool `json:"run"`
}

// Input holds the editable fields.
type Input struct {
	Folder string `json:"folder"`
	Name   string `json:"name"`
	Body   string `json:"body"`
	Run    bool   `json:"run"`
}

// Repo stores snippets.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// New returns a Repo over a migrated database.
func New(db *sql.DB) *Repo { return &Repo{db: db, now: time.Now} }

// List returns all snippets sorted by folder and name.
func (r *Repo) List(ctx context.Context) ([]Snippet, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, folder, name, body, run FROM snippets ORDER BY folder COLLATE NOCASE, name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Snippet{}
	for rows.Next() {
		var s Snippet
		if err := rows.Scan(&s.ID, &s.Folder, &s.Name, &s.Body, &s.Run); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Create adds a snippet.
func (r *Repo) Create(ctx context.Context, in Input) (Snippet, error) {
	if err := validate(&in); err != nil {
		return Snippet{}, err
	}
	s := Snippet{ID: uuid.Must(uuid.NewV7()).String(), Folder: in.Folder, Name: in.Name, Body: in.Body, Run: in.Run}
	now := r.timestamp()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO snippets (id, folder, name, body, run, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Folder, s.Name, s.Body, s.Run, now, now)
	return s, err
}

// Update replaces a snippet's fields.
func (r *Repo) Update(ctx context.Context, id string, in Input) (Snippet, error) {
	if err := validate(&in); err != nil {
		return Snippet{}, err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE snippets SET folder = ?, name = ?, body = ?, run = ?, updated_at = ? WHERE id = ?`,
		in.Folder, in.Name, in.Body, in.Run, r.timestamp(), id)
	if err != nil {
		return Snippet{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Snippet{}, errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return Snippet{ID: id, Folder: in.Folder, Name: in.Name, Body: in.Body, Run: in.Run}, nil
}

// Delete removes a snippet.
func (r *Repo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM snippets WHERE id = ?`, id)
	return err
}

func (r *Repo) timestamp() string { return r.now().UTC().Format(time.RFC3339Nano) }

func validate(in *Input) error {
	in.Name = strings.TrimSpace(in.Name)
	in.Folder = strings.Trim(strings.TrimSpace(in.Folder), "/")
	if in.Name == "" {
		return errs.New(ErrNameRequired, nil)
	}
	if strings.TrimSpace(in.Body) == "" {
		return errs.New(ErrBodyRequired, nil)
	}
	return nil
}
