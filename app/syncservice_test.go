package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/sync"
	"github.com/csturiale/shellvane/core/vault"
)

func TestSyncService(t *testing.T) {
	ctx := context.Background()
	folder := t.TempDir()
	sync.DefaultKDF = sync.KDF{Time: 1, Memory: 1024, Threads: 1}
	setup := func(name string) (*SyncService, *hosts.Repo, *vault.Vault) {
		dir := t.TempDir()
		db, err := store.Open(ctx, filepath.Join(dir, "t.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		v := vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1})
		if err := v.Setup(ctx, []byte("password!")); err != nil {
			t.Fatal(err)
		}
		return NewSyncService(db, v, dir, func(string, any) {}), hosts.New(db), v
	}
	a, ha, _ := setup("a")
	b, hb, vb := setup("b")
	ha.CreateHost(ctx, hosts.HostInput{Name: "web", Hostname: "web.example.com"})
	if _, err := a.SyncNow(ctx); errs.CodeOf(err) != sync.ErrNotConfigured {
		t.Fatalf("not configured: %v", err)
	}
	if _, err := a.Setup(ctx, SyncSetup{Kind: SyncFolder, Folder: folder, Passphrase: "short"}); errs.CodeOf(err) != vault.ErrWeakPassword {
		t.Fatalf("weak: %v", err)
	}
	if _, err := a.Setup(ctx, SyncSetup{Kind: SyncFolder, Folder: folder, Passphrase: "shared secret"}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Setup(ctx, SyncSetup{Kind: SyncFolder, Folder: folder, Passphrase: "not the same"}); errs.CodeOf(err) != sync.ErrWrongPassphrase {
		t.Fatalf("wrong passphrase: %v", err)
	}
	rep, err := b.Setup(ctx, SyncSetup{Kind: SyncFolder, Folder: folder, Passphrase: "shared secret"})
	if err != nil || rep.Updated != 1 {
		t.Fatalf("b setup = %+v, %v", rep, err)
	}
	if inv, _ := hb.List(ctx); len(inv.Hosts) != 1 || inv.Hosts[0].Name != "web" {
		t.Fatalf("b hosts = %+v", inv.Hosts)
	}
	st := b.Status(ctx)
	if !st.Configured || len(st.Devices) != 1 || st.LastSync == "" {
		t.Fatalf("status = %+v", st)
	}
	vb.Lock()
	if _, err := b.SyncNow(ctx); errs.CodeOf(err) != vault.ErrLocked {
		t.Fatalf("locked vault: %v", err)
	}
	vb.Unlock(ctx, []byte("password!"))
	if err := b.Disable(ctx); err != nil || b.Status(ctx).Configured {
		t.Fatalf("disable: %v", err)
	}
}
