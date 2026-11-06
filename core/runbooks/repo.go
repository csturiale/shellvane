package runbooks

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/csturiale/shellvane/core/errs"
)

// Repo stores runbooks.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// NewRepo returns a Repo over a migrated database.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db, now: time.Now} }

// List returns every runbook by name.
func (r *Repo) List(ctx context.Context) ([]Runbook, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, data FROM runbooks ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Runbook{}
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var rb Runbook
		if json.Unmarshal([]byte(data), &rb) == nil {
			rb.ID = id
			out = append(out, normalize(rb))
		}
	}
	return out, rows.Err()
}

// Get returns one runbook.
func (r *Repo) Get(ctx context.Context, id string) (Runbook, error) {
	var data string
	if err := r.db.QueryRowContext(ctx, `SELECT data FROM runbooks WHERE id = ?`, id).Scan(&data); err != nil {
		return Runbook{}, errs.New(ErrNotFound, errs.Params{"id": id})
	}
	var rb Runbook
	if err := json.Unmarshal([]byte(data), &rb); err != nil {
		return Runbook{}, errs.Wrap(err, ErrInvalid, nil)
	}
	rb.ID = id
	return normalize(rb), nil
}

// Save creates (empty ID) or replaces a runbook.
func (r *Repo) Save(ctx context.Context, rb Runbook) (Runbook, error) {
	rb = normalize(rb)
	if err := rb.Validate(); err != nil {
		return Runbook{}, err
	}
	if rb.ID == "" {
		rb.ID = uuid.NewString()
	}
	data, _ := json.Marshal(rb)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO runbooks (id, name, data, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, data = excluded.data, updated_at = excluded.updated_at`,
		rb.ID, rb.Name, string(data), r.now().UTC().Format(time.RFC3339))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return Runbook{}, errs.New(ErrDuplicate, errs.Params{"name": rb.Name})
	}
	return rb, err
}

// Delete removes a runbook.
func (r *Repo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM runbooks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return nil
}

func normalize(rb Runbook) Runbook {
	rb.Name = strings.TrimSpace(rb.Name)
	if rb.Variables == nil {
		rb.Variables = []Variable{}
	}
	if rb.Steps == nil {
		rb.Steps = []Step{}
	}
	for _, p := range []*[]string{&rb.Targets.HostIDs, &rb.Targets.GroupIDs, &rb.Targets.Tags} {
		if *p == nil {
			*p = []string{}
		}
	}
	return rb
}

// ExportYAML renders a runbook to share; resolve maps host and group IDs to
// names, which mean something on another computer.
func ExportYAML(rb Runbook, resolve func(kind, id string) string) (string, error) {
	x := rb
	x.Targets = Targets{Tags: rb.Targets.Tags}
	for _, id := range rb.Targets.HostIDs {
		x.Targets.HostIDs = append(x.Targets.HostIDs, resolve("host", id))
	}
	for _, id := range rb.Targets.GroupIDs {
		x.Targets.GroupIDs = append(x.Targets.GroupIDs, resolve("group", id))
	}
	b, err := yaml.Marshal(x)
	return "# Shellvane runbook\n" + string(b), err
}

// ImportYAML reads an exported runbook; lookup maps names back to IDs.
func ImportYAML(text string, lookup func(kind, name string) string) (Runbook, error) {
	var rb Runbook
	if err := yaml.Unmarshal([]byte(text), &rb); err != nil {
		return Runbook{}, errs.Wrap(err, ErrInvalid, nil)
	}
	t := Targets{Tags: rb.Targets.Tags}
	for _, n := range rb.Targets.HostIDs {
		if id := lookup("host", n); id != "" {
			t.HostIDs = append(t.HostIDs, id)
		}
	}
	for _, n := range rb.Targets.GroupIDs {
		if id := lookup("group", n); id != "" {
			t.GroupIDs = append(t.GroupIDs, id)
		}
	}
	rb.Targets, rb.ID = t, ""
	rb = normalize(rb)
	return rb, rb.Validate()
}

// ExportResults renders a run's grid as "csv" or "json".
func ExportResults(r Runbook, hosts []Host, cells map[string][]Cell, format string) (string, error) {
	type row struct {
		Host     string  `json:"host"`
		Step     string  `json:"step"`
		Command  string  `json:"command"`
		State    string  `json:"state"`
		ExitCode int     `json:"exitCode"`
		Duration float64 `json:"duration"`
		Output   string  `json:"output"`
		Error    string  `json:"error,omitempty"`
	}
	var rows []row
	for _, h := range hosts {
		for i, c := range cells[h.ID] {
			name := r.Steps[i].Name
			if name == "" {
				name = strconv.Itoa(i + 1)
			}
			x := row{Host: h.Name, Step: name, Command: c.Command, State: c.State}
			if c.Result != nil {
				x.ExitCode, x.Duration, x.Output, x.Error = c.Result.ExitCode, c.Result.Duration, c.Result.Output, c.Result.Error
			}
			rows = append(rows, x)
		}
	}
	if format == "json" {
		b, err := json.MarshalIndent(rows, "", "  ")
		return string(b), err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write([]string{"host", "step", "command", "state", "exit_code", "duration_s", "output", "error"})
	for _, x := range rows {
		w.Write([]string{x.Host, x.Step, x.Command, x.State, strconv.Itoa(x.ExitCode), strconv.FormatFloat(x.Duration, 'f', 3, 64), x.Output, x.Error})
	}
	w.Flush()
	return buf.String(), w.Error()
}
