// Package audit records what automation did on the user's behalf.
package audit

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"time"
)

// Entry is one audited action.
type Entry struct {
	ID      int64  `json:"id"`
	At      string `json:"at"`   // RFC 3339, UTC
	Kind    string `json:"kind"` // e.g. "trigger.respond"
	Host    string `json:"host"`
	Subject string `json:"subject"` // rule or runbook name
	Detail  string `json:"detail"`  // never a secret
}

// Log stores entries.
type Log struct {
	db  *sql.DB
	now func() time.Time
}

// New returns a Log over a migrated database.
func New(db *sql.DB) *Log { return &Log{db: db, now: time.Now} }

// Add records an entry.
func (l *Log) Add(ctx context.Context, kind, host, subject, detail string) error {
	_, err := l.db.ExecContext(ctx, `INSERT INTO audit_log (at, kind, host, subject, detail) VALUES (?, ?, ?, ?, ?)`,
		l.now().UTC().Format(time.RFC3339Nano), kind, host, subject, detail)
	return err
}

// List returns the newest entries first.
func (l *Log) List(ctx context.Context, limit int) ([]Entry, error) {
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
	rows, err := l.db.QueryContext(ctx, `SELECT id, at, kind, host, subject, detail FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.At, &e.Kind, &e.Host, &e.Subject, &e.Detail); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Clear deletes every entry.
func (l *Log) Clear(ctx context.Context) error {
	_, err := l.db.ExecContext(ctx, `DELETE FROM audit_log`)
	return err
}

// Export renders entries as "csv" or "json".
func Export(entries []Entry, format string) (string, error) {
	if format == "json" {
		b, err := json.MarshalIndent(entries, "", "  ")
		return string(b), err
	}
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Write([]string{"at", "kind", "host", "subject", "detail"})
	for _, e := range entries {
		w.Write([]string{e.At, e.Kind, e.Host, e.Subject, e.Detail})
	}
	w.Flush()
	return buf.String(), w.Error()
}
