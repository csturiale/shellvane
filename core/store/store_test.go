package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenMigratesAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "shellvane.db")

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	// Reopening must not re-apply migrations.
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	all, err := migrations()
	if err != nil {
		t.Fatal(err)
	}
	var count, latest int
	if err := db.QueryRow(`SELECT COUNT(*), MAX(version) FROM schema_migrations`).Scan(&count, &latest); err != nil {
		t.Fatal(err)
	}
	if count != len(all) || latest != all[len(all)-1].version {
		t.Fatalf("schema_migrations has %d rows (latest %d), want %d", count, latest, len(all))
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	db, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var on int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&on); err != nil {
		t.Fatal(err)
	}
	if on != 1 {
		t.Fatal("foreign keys must be enabled")
	}
}

func TestBackup(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "shellvane.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	path, err := Backup(ctx, db, filepath.Join(t.TempDir(), "backups"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Backup(ctx, db, filepath.Dir(path))
	if err != nil || second == path {
		t.Fatalf("back-to-back backups must not collide: %q %v", second, err)
	}
	copyDB, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("backup is not a usable database: %v", err)
	}
	copyDB.Close()
}

func TestPortableDir(t *testing.T) {
	dir := t.TempDir()
	if got := portableDir(dir); got != "" {
		t.Fatalf("no marker: %q", got)
	}
	os.WriteFile(filepath.Join(dir, PortableMarker), nil, 0o600)
	if got := portableDir(dir); got != filepath.Join(dir, "ShellvaneData") {
		t.Fatalf("marker: %q", got)
	}
	// macOS bundle: the marker is next to Shellvane.app.
	macos := filepath.Join(dir, "Shellvane.app", "Contents", "MacOS")
	os.MkdirAll(macos, 0o700)
	if got := portableDir(macos); got != filepath.Join(dir, "ShellvaneData") {
		t.Fatalf("bundle: %q", got)
	}
	t.Setenv("SHELLVANE_DATA_DIR", filepath.Join(dir, "x"))
	if p, _ := DefaultPath(); p != filepath.Join(dir, "x", "shellvane.db") {
		t.Fatalf("env: %q", p)
	}
}
