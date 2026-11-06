package sync

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/snippets"
	"github.com/csturiale/shellvane/core/store"
)

var testKDF = KDF{Time: 1, Memory: 1024, Threads: 1}

type device struct {
	db    *sql.DB
	hosts *hosts.Repo
	s     *Syncer
	state *State
}

func newDevice(t *testing.T, folder, id string) *device {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	key, err := OpenKey(folder, "correct horse", testKDF)
	if err != nil {
		t.Fatal(err)
	}
	return &device{db: db, hosts: hosts.New(db), s: &Syncer{DB: db, Folder: folder, Key: key, Name: id}, state: &State{Device: id}}
}

func (d *device) sync(t *testing.T) Report {
	t.Helper()
	r, err := d.s.Sync(context.Background(), d.state)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func names(t *testing.T, d *device) map[string]string {
	inv, err := d.hosts.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, h := range inv.Hosts {
		out[h.Name] = h.Hostname
	}
	return out
}

func TestSyncTwoDevices(t *testing.T) {
	ctx := context.Background()
	folder := t.TempDir()
	a, b := newDevice(t, folder, "laptop"), newDevice(t, folder, "desktop")

	g, _ := a.hosts.CreateGroup(ctx, hosts.GroupInput{Name: "Prod"})
	web, _ := a.hosts.CreateHost(ctx, hosts.HostInput{GroupID: g.ID, Name: "web", Hostname: "web.example.com"})
	snippets.New(a.db).Create(ctx, snippets.Input{Name: "uptime", Body: "uptime"})
	a.sync(t)
	if r := b.sync(t); r.Updated != 3 || r.Devices != 1 {
		t.Fatalf("first pull = %+v", r)
	}
	if got := names(t, b); got["web"] != "web.example.com" {
		t.Fatalf("b hosts = %v", got)
	}

	// b edits, a deletes another host meanwhile; both changes travel.
	time.Sleep(5 * time.Millisecond)
	db1, _ := a.hosts.CreateHost(ctx, hosts.HostInput{Name: "db", Hostname: "db.example.com"})
	a.sync(t)
	b.sync(t)
	time.Sleep(5 * time.Millisecond)
	if _, err := b.hosts.UpdateHost(ctx, web.ID, hosts.HostInput{GroupID: g.ID, Name: "web", Hostname: "web2.example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := a.hosts.DeleteHost(ctx, db1.ID); err != nil {
		t.Fatal(err)
	}
	b.sync(t)
	a.sync(t)
	b.sync(t)
	for name, d := range map[string]*device{"a": a, "b": b} {
		got := names(t, d)
		if got["web"] != "web2.example.com" || got["db"] != "" || len(got) != 1 {
			t.Errorf("%s hosts = %v", name, got)
		}
	}

	// Deleting a group on one device removes its hosts everywhere.
	if err := b.hosts.DeleteGroup(ctx, g.ID); err != nil {
		t.Fatal(err)
	}
	b.sync(t)
	a.sync(t)
	if got := names(t, a); len(got) != 0 {
		t.Errorf("a after group deletion = %v", got)
	}
}

func TestWrongPassphrase(t *testing.T) {
	folder := t.TempDir()
	if _, err := OpenKey(folder, "right", testKDF); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenKey(folder, "wrong", testKDF); errs.CodeOf(err) != ErrWrongPassphrase {
		t.Fatalf("err = %v", err)
	}
}

func TestAfter(t *testing.T) {
	if !after("2026-10-31T16:00:00.1Z", "2026-10-31T16:00:00Z") || after("2026-10-31T16:00:00Z", "2026-10-31T16:00:00.1Z") {
		t.Fatal("precision must not matter")
	}
}
