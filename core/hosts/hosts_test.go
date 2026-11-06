package hosts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/store"
)

func newRepo(t *testing.T) *Repo {
	t.Helper()
	db, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db)
}

func ptr[T any](v T) *T { return &v }

func mustGroup(t *testing.T, r *Repo, parent, name string) Group {
	t.Helper()
	g, err := r.CreateGroup(context.Background(), GroupInput{ParentID: parent, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func wantCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	if got := errs.CodeOf(err); err == nil || got != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

func TestCreateAndListTree(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)

	prod := mustGroup(t, r, "", "Production")
	web := mustGroup(t, r, prod.ID, "web")
	h, err := r.CreateHost(ctx, HostInput{GroupID: web.ID, Name: " web-1 ", Hostname: "10.0.0.1", Settings: settings.Settings{Port: ptr(2222)}})
	if err != nil {
		t.Fatal(err)
	}
	if h.Name != "web-1" {
		t.Errorf("name should be trimmed, got %q", h.Name)
	}
	if _, err := r.CreateHost(ctx, HostInput{Hostname: "bastion.example.org"}); err != nil {
		t.Fatal(err)
	}

	inv, err := r.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Groups) != 2 || len(inv.Hosts) != 2 {
		t.Fatalf("got %d groups and %d hosts", len(inv.Groups), len(inv.Hosts))
	}
	// Sorted by name, case-insensitive: "bastion..." before "web-1".
	if inv.Hosts[0].Name != "bastion.example.org" || inv.Hosts[0].GroupID != "" {
		t.Errorf("host without name should default to hostname at root: %+v", inv.Hosts[0])
	}
	if got := inv.Hosts[1]; got.GroupID != web.ID || *got.Settings.Port != 2222 {
		t.Errorf("unexpected host: %+v", got)
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)

	_, err := r.CreateGroup(ctx, GroupInput{Name: "  "})
	wantCode(t, err, ErrNameRequired)

	_, err = r.CreateHost(ctx, HostInput{Name: "x"})
	wantCode(t, err, ErrHostnameRequired)

	_, err = r.CreateHost(ctx, HostInput{Hostname: "root@server"})
	wantCode(t, err, ErrHostnameInvalid)

	_, err = r.CreateHost(ctx, HostInput{Hostname: "server", Settings: settings.Settings{Port: ptr(70000)}})
	wantCode(t, err, ErrPortInvalid)

	_, err = r.CreateHost(ctx, HostInput{Hostname: "server", GroupID: "missing"})
	wantCode(t, err, ErrGroupNotFound)
}

func TestMoveGroupRejectsCycles(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	a := mustGroup(t, r, "", "a")
	b := mustGroup(t, r, a.ID, "b")
	c := mustGroup(t, r, b.ID, "c")

	wantCode(t, r.MoveGroup(ctx, a.ID, a.ID), ErrGroupCycle)
	wantCode(t, r.MoveGroup(ctx, a.ID, c.ID), ErrGroupCycle)

	if err := r.MoveGroup(ctx, c.ID, ""); err != nil {
		t.Fatalf("moving to root: %v", err)
	}
	if err := r.MoveGroup(ctx, a.ID, c.ID); err != nil {
		t.Fatalf("a under c is valid once c left the subtree: %v", err)
	}
	wantCode(t, r.MoveGroup(ctx, "missing", ""), ErrGroupNotFound)
}

func TestDeleteGroupCascades(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	prod := mustGroup(t, r, "", "prod")
	web := mustGroup(t, r, prod.ID, "web")
	other := mustGroup(t, r, "", "other")
	for _, g := range []string{prod.ID, web.ID, web.ID, other.ID} {
		if _, err := r.CreateHost(ctx, HostInput{GroupID: g, Hostname: "h"}); err != nil {
			t.Fatal(err)
		}
	}

	c, err := r.Contents(ctx, prod.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c != (GroupContents{Groups: 1, Hosts: 3}) {
		t.Fatalf("contents = %+v, want 1 group and 3 hosts", c)
	}

	if err := r.DeleteGroup(ctx, prod.ID); err != nil {
		t.Fatal(err)
	}
	inv, _ := r.List(ctx)
	if len(inv.Groups) != 1 || len(inv.Hosts) != 1 || inv.Hosts[0].GroupID != other.ID {
		t.Fatalf("only the other group and its host should remain: %+v", inv)
	}

	_, err = r.Contents(ctx, prod.ID)
	wantCode(t, err, ErrGroupNotFound)
	wantCode(t, r.DeleteGroup(ctx, prod.ID), ErrGroupNotFound)
}

func TestUpdateAndMoveHost(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	g := mustGroup(t, r, "", "g")
	h, err := r.CreateHost(ctx, HostInput{Hostname: "old"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := r.UpdateHost(ctx, h.ID, HostInput{Name: "db", Hostname: "db.internal", Notes: "primary"}); err != nil {
		t.Fatal(err)
	}
	if err := r.MoveHost(ctx, h.ID, g.ID); err != nil {
		t.Fatal(err)
	}
	inv, _ := r.List(ctx)
	if got := inv.Hosts[0]; got.Name != "db" || got.Hostname != "db.internal" || got.GroupID != g.ID || got.Notes != "primary" {
		t.Fatalf("unexpected host after update: %+v", got)
	}

	wantCode(t, r.MoveHost(ctx, h.ID, "missing"), ErrGroupNotFound)
	if err := r.DeleteHost(ctx, h.ID); err != nil {
		t.Fatal(err)
	}
	wantCode(t, r.DeleteHost(ctx, h.ID), ErrHostNotFound)
}

func TestChainResolvesGroupSettings(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	prod, err := r.CreateGroup(ctx, GroupInput{Name: "prod", Settings: settings.Settings{User: ptr("deploy"), Port: ptr(2200)}})
	if err != nil {
		t.Fatal(err)
	}
	web, err := r.CreateGroup(ctx, GroupInput{ParentID: prod.ID, Name: "web", Settings: settings.Settings{Port: ptr(2201)}})
	if err != nil {
		t.Fatal(err)
	}

	chain, err := r.Chain(ctx, web.ID)
	if err != nil {
		t.Fatal(err)
	}
	host := settings.Level{ID: "host"}
	res := settings.Resolve(append([]settings.Level{host}, chain...)...)

	if *res.Settings.User != "deploy" || res.Source["user"] != prod.ID {
		t.Errorf("user should be inherited from prod: %+v", res.Source)
	}
	if *res.Settings.Port != 2201 || res.Source["port"] != web.ID {
		t.Errorf("port should come from web: %+v", res.Source)
	}

	_, err = r.Chain(ctx, "missing")
	if !errors.Is(err, errs.New(ErrGroupNotFound, nil)) {
		t.Errorf("missing group: %v", err)
	}
}

func TestHostAndResolve(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	g, err := r.CreateGroup(ctx, GroupInput{Name: "g", Settings: settings.Settings{User: ptr("deploy"), Port: ptr(2200)}})
	if err != nil {
		t.Fatal(err)
	}
	h, err := r.CreateHost(ctx, HostInput{GroupID: g.ID, Hostname: "web", Settings: settings.Settings{Port: ptr(2222)}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.Host(ctx, h.ID)
	if err != nil || got.Hostname != "web" || got.GroupID != g.ID {
		t.Fatalf("Host = %+v, %v", got, err)
	}
	res, err := r.Resolve(ctx, got)
	if err != nil {
		t.Fatal(err)
	}
	if *res.Settings.Port != 2222 || *res.Settings.User != "deploy" || *res.Settings.TermType != "xterm-256color" {
		t.Errorf("resolved = %+v", res.Settings)
	}

	_, err = r.Host(ctx, "missing")
	wantCode(t, err, ErrHostNotFound)
}

func TestLabel(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	prod, _ := r.CreateGroup(ctx, GroupInput{Name: "Production", Color: "red"})
	web, _ := r.CreateGroup(ctx, GroupInput{ParentID: prod.ID, Name: "web"})
	db, _ := r.CreateGroup(ctx, GroupInput{ParentID: prod.ID, Name: "db", Color: "blue"})

	cases := []struct {
		group string
		want  GroupLabel
	}{
		{web.ID, GroupLabel{Path: []string{"Production", "web"}, Color: "red"}},
		{db.ID, GroupLabel{Path: []string{"Production", "db"}, Color: "blue"}},
		{"", GroupLabel{}},
	}
	for _, c := range cases {
		got, err := r.Label(ctx, c.group)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Join(got.Path, "/") != strings.Join(c.want.Path, "/") || got.Color != c.want.Color {
			t.Errorf("Label(%q) = %+v, want %+v", c.group, got, c.want)
		}
	}
}

func TestTagsFavoritesRecentAndDuplicate(t *testing.T) {
	ctx := context.Background()
	r := newRepo(t)
	h, err := r.CreateHost(ctx, HostInput{Name: "web", Hostname: "10.0.0.1", Tags: []string{" #prod", "web", "PROD", ""}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(h.Tags, ",") != "prod,web" {
		t.Fatalf("tags %v", h.Tags)
	}
	if err := r.SetFavorite(ctx, h.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := r.Touch(ctx, h.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := r.Host(ctx, h.ID)
	if !got.Favorite || got.LastUsed == "" || len(got.Tags) != 2 {
		t.Fatalf("host %+v", got)
	}
	cp, err := r.DuplicateHost(ctx, h.ID, " (copy)")
	if err != nil || cp.Name != "web (copy)" || cp.ID == h.ID || strings.Join(cp.Tags, ",") != "prod,web" || cp.Favorite {
		t.Fatalf("copy %+v, %v", cp, err)
	}
	inv, _ := r.List(ctx)
	if len(inv.Hosts) != 2 || !inv.Hosts[0].Favorite {
		t.Fatalf("inventory %+v", inv.Hosts)
	}
}

func TestHostnameAndUserValidation(t *testing.T) {
	for _, h := range []string{"web-1.example.com", "10.0.0.1", "fe80::1%en0", "[::1]", "bücher.example"} {
		if !ValidHostname(h) {
			t.Errorf("%q refused", h)
		}
	}
	for _, h := range []string{"-oProxyCommand=sh", "a;b", "$(id)", "a`id`", "a|b", "a b", "a&b"} {
		if ValidHostname(h) {
			t.Errorf("%q accepted", h)
		}
	}
	for _, u := range []string{"root", "user@corp.example", `CORP\user`, "first.last"} {
		if !ValidUser(u) {
			t.Errorf("user %q refused", u)
		}
	}
	for _, u := range []string{"-oX", "$(id)", "a;b", "a b", "a`b`"} {
		if ValidUser(u) {
			t.Errorf("user %q accepted", u)
		}
	}
}
