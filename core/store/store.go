// Package store opens the application database and keeps its schema current.
//
// The database is SQLite (pure Go driver, no CGO). Migrations are embedded SQL
// files named NNNN_description.sql and applied in order, each in its own
// transaction. Applied versions are recorded in schema_migrations.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Open opens (creating if needed) the database at path and migrates it.
// Use ":memory:" for tests.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create data directory: %w", err)
		}
	}
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer; a single connection also keeps ":memory:"
	// databases from being split across connections.
	db.SetMaxOpenConns(1)

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// DefaultPath returns the per-user database location, e.g.
// ~/.config/Shellvane/shellvane.db on Linux.
func DefaultPath() (string, error) {
	if dir := os.Getenv("SHELLVANE_DATA_DIR"); dir != "" {
		return filepath.Join(dir, "shellvane.db"), nil
	}
	if dir := PortableDir(); dir != "" {
		return filepath.Join(dir, "shellvane.db"), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Shellvane", "shellvane.db"), nil
}

// PortableMarker, next to the executable (or to the macOS .app bundle),
// makes Shellvane keep its data in a ShellvaneData folder beside it, e.g. on a
// USB stick.
const PortableMarker = "shellvane.portable"

// PortableDir returns the data folder of a portable installation, or "".
func PortableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return portableDir(filepath.Dir(exe))
}

func portableDir(exeDir string) string {
	candidates := []string{exeDir}
	// Shellvane.app/Contents/MacOS/shellvane: the marker sits next to the bundle.
	if filepath.Base(exeDir) == "MacOS" && filepath.Base(filepath.Dir(exeDir)) == "Contents" {
		candidates = append(candidates, filepath.Dir(filepath.Dir(filepath.Dir(exeDir))))
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, PortableMarker)); err == nil {
			return filepath.Join(dir, "ShellvaneData")
		}
	}
	return ""
}

type migration struct {
	version int
	name    string
}

func migrations() ([]migration, error) {
	files, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return nil, err
	}
	out := make([]migration, 0, len(files))
	for _, f := range files {
		base := filepath.Base(f)
		num, _, ok := strings.Cut(base, "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil {
			return nil, fmt.Errorf("migration %s: name must start with NNNN_", base)
		}
		out = append(out, migration{version: v, name: f})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	var current int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return err
	}

	all, err := migrations()
	if err != nil {
		return err
	}
	for _, m := range all {
		if m.version <= current {
			continue
		}
		body, err := migrationFS.ReadFile(m.name)
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply %s: %w", m.name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, m.version); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Backup writes a consistent copy of the database into dir and returns its
// path. It is taken before risky operations such as imports.
func Backup(ctx context.Context, db *sql.DB, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	base := filepath.Join(dir, "shellvane-"+time.Now().Format("20060102-150405.000"))
	path := base + ".db"
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = fmt.Sprintf("%s-%d.db", base, i)
	}
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("backup: %w", err)
	}
	return path, nil
}
