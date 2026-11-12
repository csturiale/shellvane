package app

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/tunnel"
)

func TestResolveOpenAndControl(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := hosts.New(db)
	port, user := 2222, "deploy"
	web, _ := repo.CreateHost(ctx, hosts.HostInput{Name: "Web", Hostname: "web.example.com", Settings: settings.Settings{Port: &port, User: &user}})

	cases := map[string]OpenRequest{
		"web":                                 {Kind: "host", HostID: web.ID, Name: "Web"},
		"ssh://deploy@web.example.com:2222":   {Kind: "host", HostID: web.ID, Name: "Web"},
		"sftp://web.example.com:2222/var/log": {Kind: "files", HostID: web.ID, Name: "Web", Path: "/var/log"},
		"ssh://other@web.example.com:2222":    {Kind: "target", Spec: "other@web.example.com:2222", Name: "other@web.example.com:2222"},
		"root@10.0.0.5":                       {Kind: "target", Spec: "root@10.0.0.5", Name: "root@10.0.0.5"},
	}
	for in, want := range cases {
		got, err := ResolveOpen(ctx, repo, in)
		if err != nil || got != want {
			t.Errorf("ResolveOpen(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	if _, err := ResolveOpen(ctx, repo, "nothing"); errs.CodeOf(err) != ErrOpenNotFound {
		t.Errorf("unknown name: %v", err)
	}

	if runtime.GOOS == "windows" {
		return // the pipe name is global
	}
	if _, err := SendControl(dir, ControlRequest{Cmd: "ping"}); errs.CodeOf(err) != ErrNotRunning {
		t.Fatalf("no app: %v", err)
	}
	var opened []any
	sessions := NewSessionService(repo, nil, nil, nil, func(string, any) {})
	c := NewControl(repo, NewTunnelService(tunnel.NewRepo(db), sessions, func(string, any) {}), func(name string, data any) { opened = append(opened, data) })
	shown := false
	c.Show = func() { shown = true }
	if got := c.Ready(); len(got) != 0 {
		t.Fatalf("pending = %+v", got)
	}
	if err := c.Listen(dir); err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	resp, err := SendControl(dir, ControlRequest{Cmd: "open", Target: "web"})
	if err != nil || !resp.OK || !shown || len(opened) != 1 {
		t.Fatalf("open = %+v, %v (shown %v, %v)", resp, err, shown, opened)
	}
	resp, _ = SendControl(dir, ControlRequest{Cmd: "tunnelUp", Name: "missing"})
	if resp.OK || resp.Error == nil || resp.Error.Code != tunnel.ErrNotFound {
		t.Fatalf("missing tunnel = %+v", resp)
	}
}

func TestControlSendsProcedureLinks(t *testing.T) {
	c := NewControl(nil, nil, func(string, any) {})
	var got []string
	shown := 0
	c.Show = func() { shown++ }
	c.Procedure = func(l string) { got = append(got, l) }
	c.OpenAll(context.Background(), []string{"--flag", "shellvane://procedure?md=x", "https://example.com"})
	if len(got) != 1 || got[0] != "shellvane://procedure?md=x" || shown != 1 {
		t.Fatalf("got %v, shown %d", got, shown)
	}
}
