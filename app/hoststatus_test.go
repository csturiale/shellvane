package app

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/store"
)

func TestHostStatus(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := hosts.New(db)
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	open := ln.Addr().(*net.TCPAddr).Port
	closed, _ := net.Listen("tcp", "127.0.0.1:0")
	closedPort := closed.Addr().(*net.TCPAddr).Port
	closed.Close()
	up, _ := repo.CreateHost(ctx, hosts.HostInput{Name: "up", Hostname: "127.0.0.1", Settings: settings.Settings{Port: &open}})
	down, _ := repo.CreateHost(ctx, hosts.HostInput{Name: "down", Hostname: "127.0.0.1", Settings: settings.Settings{Port: &closedPort}})
	via, _ := repo.CreateHost(ctx, hosts.HostInput{Name: "via", Hostname: "10.0.0.1", Settings: settings.Settings{JumpChain: []string{up.ID}}})
	got, err := NewHostService(repo).Status(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]HostStatus{}
	for _, s := range got {
		states[s.HostID] = s
	}
	if states[up.ID].State != "up" || states[up.ID].LatencyMs < 1 || states[down.ID].State != "down" || states[via.ID].State != "unknown" {
		t.Fatalf("status = %+v", got)
	}
	if one, _ := NewHostService(repo).Status(ctx, []string{down.ID}); len(one) != 1 {
		t.Fatalf("filtered = %+v", one)
	}
	if err := NewHostService(repo).Wake(ctx, up.ID); err == nil {
		t.Fatal("wake without a MAC address should fail")
	}
}
