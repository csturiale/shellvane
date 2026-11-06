package main

import (
	"bytes"
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/app"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/store"
)

func TestCLI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELLVANE_DATA_DIR", dir)
	t.Setenv("LANG", "en_US.UTF-8")
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(dir, "shellvane.db"))
	if err != nil {
		t.Fatal(err)
	}
	repo := hosts.New(db)
	port, user, cmd := 2222, "ops", "nc %h %p"
	g, _ := repo.CreateGroup(ctx, hosts.GroupInput{Name: "Prod", Settings: settings.Settings{User: &user}})
	jump, _ := repo.CreateHost(ctx, hosts.HostInput{GroupID: g.ID, Name: "bastion", Hostname: "bastion.example.com"})
	repo.CreateHost(ctx, hosts.HostInput{GroupID: g.ID, Name: "db", Hostname: "10.0.0.5", Settings: settings.Settings{Port: &port, JumpChain: []string{jump.ID}}})
	repo.CreateHost(ctx, hosts.HostInput{Name: "edge", Hostname: "edge.example.com", Settings: settings.Settings{ProxyCommand: &cmd}})

	var out, errOut bytes.Buffer
	if code := run([]string{"shellvane-cli", "list"}, &out, &errOut); code != 0 {
		t.Fatalf("list: %d %s", code, errOut.String())
	}
	want := "edge          edge.example.com\nProd/bastion  ops@bastion.example.com\nProd/db       ops@10.0.0.5:2222\n"
	if out.String() != want {
		t.Errorf("list =\n%s\nwant\n%s", out.String(), want)
	}

	args, err := app.SSHArgs(ctx, repo, "db")
	if err != nil || !reflect.DeepEqual(args, []string{"-p", "2222", "-J", "ops@bastion.example.com", "ops@10.0.0.5"}) {
		t.Errorf("sshArgs(db) = %v, %v", args, err)
	}
	if args, _ := app.SSHArgs(ctx, repo, "edge"); !reflect.DeepEqual(args, []string{"-o", "ProxyCommand=nc %h %p", "edge.example.com"}) {
		t.Errorf("sshArgs(edge) = %v", args)
	}
	db.Close()

	errOut.Reset()
	if code := run([]string{"shellvane-cli", "connect", "db"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "not running") {
		t.Errorf("connect without app: %d %q", code, errOut.String())
	}
	if code := run([]string{"shellvane-cli", "frobnicate"}, &out, &errOut); code != 2 {
		t.Errorf("unknown command: %d", code)
	}
}
