package hosts

import (
	"context"
	"testing"

	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/store"
)

func TestMirrorTemplateUpdatesInPlace(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := New(db)
	str := func(s string) *string { return &s }
	root, _ := r.CreateGroup(ctx, GroupInput{Name: "Ops (team)"})
	shared := TemplateGroup{Name: "Ops", Color: "red", Settings: settings.Settings{User: str("deploy")},
		Hosts: []TemplateHost{{Name: "bastion", Hostname: "10.0.0.1"}},
		Groups: []TemplateGroup{{Name: "web", Jumps: []string{"bastion"}, Hosts: []TemplateHost{
			{Name: "web1", Hostname: "10.0.1.1", Tags: []string{"prod"}, Settings: settings.Settings{CredentialPlugin: str("team:t1:i1")}},
			{Name: "web2", Hostname: "10.0.1.2"},
		}}}}
	res, err := r.MirrorTemplate(ctx, root.ID, shared)
	if err != nil || res.Added != 3 {
		t.Fatalf("first mirror = %+v, %v", res, err)
	}
	inv, _ := r.List(ctx)
	byName := map[string]Host{}
	for _, h := range inv.Hosts {
		byName[h.Name] = h
	}
	var web Group
	for _, g := range inv.Groups {
		if g.Name == "web" {
			web = g
		}
		if g.ID == root.ID && (g.Name != "Ops (team)" || g.Color != "red" || *g.Settings.User != "deploy") {
			t.Fatalf("root = %+v", g)
		}
	}
	if len(web.Settings.JumpChain) != 1 || web.Settings.JumpChain[0] != byName["bastion"].ID {
		t.Fatalf("web jumps = %v", web.Settings.JumpChain)
	}
	if *byName["web1"].Settings.CredentialPlugin != "team:t1:i1" {
		t.Fatal("team login lost")
	}
	// Local touches: a key and a favorite on web1.
	web1 := byName["web1"]
	web1.Settings.IdentityID = str("key-1")
	r.UpdateHost(ctx, web1.ID, HostInput{GroupID: web1.GroupID, Name: web1.Name, Hostname: web1.Hostname, Tags: web1.Tags, Settings: web1.Settings, Favorite: true})

	// The team changes web1's address and drops web2.
	shared.Groups[0].Hosts = []TemplateHost{{Name: "web1", Hostname: "10.0.1.10", Settings: settings.Settings{CredentialPlugin: str("team:t1:i1")}}}
	res, err = r.MirrorTemplate(ctx, root.ID, shared)
	if err != nil || res.Removed != 1 || res.Updated != 2 {
		t.Fatalf("second mirror = %+v, %v", res, err)
	}
	h, err := r.Host(ctx, web1.ID)
	if err != nil || h.Hostname != "10.0.1.10" || !h.Favorite || h.Settings.IdentityID == nil || *h.Settings.IdentityID != "key-1" {
		t.Fatalf("web1 after mirror = %+v, %v", h, err)
	}
	if _, err := r.Host(ctx, byName["web2"].ID); err == nil {
		t.Fatal("web2 should be gone")
	}

	// Export keeps team logins of the given team only.
	inv, _ = r.List(ctx)
	tpl, err := ExportTemplateKeeping(inv, root.ID, true, func(ref string) bool { return ref == "team:t1:i1" })
	if err != nil || tpl.Groups[0].Groups[0].Hosts[0].Settings.CredentialPlugin == nil {
		t.Fatalf("export = %+v, %v", tpl, err)
	}
	if tpl.Groups[0].Groups[0].Hosts[0].Settings.IdentityID != nil {
		t.Fatal("a local key was exported")
	}
}
