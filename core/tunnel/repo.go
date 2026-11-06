package tunnel

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
)

// Profile is a saved tunnel.
type Profile struct {
	ID     string `json:"id"`
	HostID string `json:"hostId"`
	Name   string `json:"name"`
	Spec
	// Autostart starts the tunnel when the application opens.
	Autostart bool `json:"autostart"`
}

// Input holds the editable fields of a profile.
type Input struct {
	HostID    string `json:"hostId"`
	Name      string `json:"name"`
	Spec      Spec   `json:"spec"`
	Autostart bool   `json:"autostart"`
}

// Repo stores tunnel profiles.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// NewRepo returns a Repo over a migrated database.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db, now: time.Now} }

const columns = `id, host_id, name, kind, bind_addr, bind_port, dest_host, dest_port, autostart`

func scan(row interface{ Scan(...any) error }) (Profile, error) {
	var p Profile
	err := row.Scan(&p.ID, &p.HostID, &p.Name, &p.Kind, &p.BindAddr, &p.BindPort, &p.DestHost, &p.DestPort, &p.Autostart)
	return p, err
}

// List returns all profiles sorted by name.
func (r *Repo) List(ctx context.Context) ([]Profile, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM tunnels ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Profile{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get returns one profile.
func (r *Repo) Get(ctx context.Context, id string) (Profile, error) {
	p, err := scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM tunnels WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return p, err
}

// Create saves a new profile.
func (r *Repo) Create(ctx context.Context, in Input) (Profile, error) {
	if err := validate(&in); err != nil {
		return Profile{}, err
	}
	p := Profile{ID: uuid.Must(uuid.NewV7()).String(), HostID: in.HostID, Name: in.Name, Spec: in.Spec, Autostart: in.Autostart}
	now := r.timestamp()
	_, err := r.db.ExecContext(ctx, `INSERT INTO tunnels (`+columns+`, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.HostID, p.Name, p.Kind, p.BindAddr, p.BindPort, p.DestHost, p.DestPort, p.Autostart, now, now)
	return p, err
}

// Update replaces a profile.
func (r *Repo) Update(ctx context.Context, id string, in Input) (Profile, error) {
	if err := validate(&in); err != nil {
		return Profile{}, err
	}
	res, err := r.db.ExecContext(ctx, `UPDATE tunnels SET host_id = ?, name = ?, kind = ?, bind_addr = ?, bind_port = ?, dest_host = ?, dest_port = ?, autostart = ?, updated_at = ? WHERE id = ?`,
		in.HostID, in.Name, in.Spec.Kind, in.Spec.BindAddr, in.Spec.BindPort, in.Spec.DestHost, in.Spec.DestPort, in.Autostart, r.timestamp(), id)
	if err != nil {
		return Profile{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Profile{}, errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return Profile{ID: id, HostID: in.HostID, Name: in.Name, Spec: in.Spec, Autostart: in.Autostart}, nil
}

// Delete removes a profile.
func (r *Repo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM tunnels WHERE id = ?`, id)
	return err
}

func (r *Repo) timestamp() string { return r.now().UTC().Format(time.RFC3339Nano) }

func validate(in *Input) error {
	in.Name = strings.TrimSpace(in.Name)
	if in.HostID == "" {
		return errs.New(ErrInvalid, errs.Params{"field": "hostId"})
	}
	if err := in.Spec.Validate(); err != nil {
		return err
	}
	if in.Name == "" {
		switch in.Spec.Kind {
		case Dynamic:
			in.Name = "SOCKS " + in.Spec.bind()
		default:
			in.Name = in.Spec.bind() + " → " + in.Spec.dest()
		}
	}
	return nil
}
