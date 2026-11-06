package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/tunnel"
)

func TestImportOpenSSH(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, "shellvane.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	hr, tr := hosts.New(db), tunnel.NewRepo(db)
	svc := NewImportService(db, hr, tr, filepath.Join(dir, "backups"))

	res, err := svc.Preview("openssh", "file", filepath.Join("..", "testdata", "importers", "openssh", "config"))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := svc.Import(ctx, ImportRequest{Hosts: res.Hosts, Group: "OpenSSH"})
	if err != nil {
		t.Fatal(err)
	}
	// "bastion" and "jump" are aliases of the same address and user: one is
	// a duplicate.
	if sum.Hosts != 4 || sum.Skipped != 1 || sum.Groups != 1 || sum.Tunnels != 4 || sum.Backup == "" {
		t.Fatalf("summary = %+v", sum)
	}

	// Importing again creates nothing new and reuses the group.
	again, err := svc.Import(ctx, ImportRequest{Hosts: res.Hosts, Group: "openssh"})
	if err != nil {
		t.Fatal(err)
	}
	if again.Hosts != 0 || again.Skipped != 5 || again.Groups != 0 {
		t.Fatalf("second import = %+v", again)
	}

	inv, _ := hr.List(ctx)
	if len(inv.Groups) != 1 || len(inv.Hosts) != 4 {
		t.Fatalf("inventory = %d groups, %d hosts", len(inv.Groups), len(inv.Hosts))
	}
	ids := map[string]string{}
	for _, h := range inv.Hosts {
		ids[h.Name] = h.ID
	}
	for _, h := range inv.Hosts {
		if h.Name == "db-prod" && (len(h.Settings.JumpChain) != 1 || h.Settings.JumpChain[0] != ids["bastion"]) {
			t.Errorf("db-prod should jump through bastion: %v", h.Settings.JumpChain)
		}
	}
}

func TestImportKiTTYFolders(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, _ := store.Open(ctx, filepath.Join(dir, "shellvane.db"))
	defer db.Close()
	hr := hosts.New(db)
	svc := NewImportService(db, hr, tunnel.NewRepo(db), filepath.Join(dir, "backups"))

	res, err := svc.Preview("kitty", "dir", filepath.Join("..", "testdata", "importers", "kitty", "Sessions"))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := svc.Import(ctx, ImportRequest{Hosts: res.Hosts})
	if err != nil {
		t.Fatal(err)
	}
	if sum.Groups != 2 || sum.Hosts != 1 {
		t.Fatalf("summary = %+v", sum)
	}
	inv, _ := hr.List(ctx)
	label, _ := hr.Label(ctx, inv.Hosts[0].GroupID)
	if len(label.Path) != 2 || label.Path[0] != "Production" || label.Path[1] != "Databases" {
		t.Fatalf("group path = %v", label.Path)
	}
}

func TestPreviewErrors(t *testing.T) {
	svc := NewImportService(nil, nil, nil, "")
	if _, err := svc.Preview("nope", "file", ""); err == nil {
		t.Fatal("unknown source must fail")
	}
	if _, err := svc.Preview("openssh", "file", "/does/not/exist"); err == nil {
		t.Fatal("missing file must fail")
	}
}

func TestImportedTunnelNames(t *testing.T) {
	cases := map[string]tunnel.Spec{
		"bastion SOCKS 1080":       {Kind: tunnel.Dynamic, BindPort: 1080},
		"bastion L 8080 → db:5432": {Kind: tunnel.Local, BindPort: 8080, DestHost: "db", DestPort: 5432},
		"bastion R 9000 → web:80":  {Kind: tunnel.Remote, BindPort: 9000, DestHost: "web", DestPort: 80},
	}
	for want, spec := range cases {
		if got := importedTunnelName("bastion", spec); got != want {
			t.Errorf("name = %q, want %q", got, want)
		}
	}
}
