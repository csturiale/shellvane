package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/store"
)

func TestKeyUsage(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := hosts.New(db)
	key := "k1"
	g, _ := repo.CreateGroup(ctx, hosts.GroupInput{Name: "Prod", Settings: settings.Settings{IdentityID: &key}})
	inherits, _ := repo.CreateHost(ctx, hosts.HostInput{GroupID: g.ID, Name: "web", Hostname: "web"})
	direct, _ := repo.CreateHost(ctx, hosts.HostInput{Name: "db", Hostname: "db", Settings: settings.Settings{IdentityID: &key}})
	other := "k2"
	repo.CreateHost(ctx, hosts.HostInput{Name: "x", Hostname: "x", Settings: settings.Settings{IdentityID: &other}})
	svc := NewKeyService(nil, NewSessionService(repo, nil, nil, nil, func(string, any) {}))
	usage, err := svc.Usage(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]KeyUsage{}
	for _, u := range usage {
		got[u.Kind+":"+u.ID] = u
	}
	if len(usage) != 3 || got["group:"+g.ID].Name != "Prod" || !got["host:"+inherits.ID].Inherited || got["host:"+direct.ID].Inherited {
		t.Fatalf("usage = %+v", usage)
	}
}

func TestRemoveCommandKeepsOtherKeys(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs /bin/sh")
	}
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".ssh"), 0o700)
	keys := "ssh-ed25519 AAAA keep\nssh-ed25519 BBBB it's old\nssh-rsa CCCC keep2\n"
	os.WriteFile(filepath.Join(home, ".ssh", "authorized_keys"), []byte(keys), 0o600)
	cmd := exec.Command("/bin/sh", "-c", removeCommand("ssh-ed25519 BBBB it's old"))
	cmd.Env = append(os.Environ(), "HOME="+home)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	data, _ := os.ReadFile(filepath.Join(home, ".ssh", "authorized_keys"))
	if string(data) != "ssh-ed25519 AAAA keep\nssh-rsa CCCC keep2\n" {
		t.Fatalf("authorized_keys = %q", data)
	}
}

func TestSlept(t *testing.T) {
	prev := time.Now()
	if slept(prev, prev.Add(15*time.Second), 15*time.Second) {
		t.Error("a normal tick is not a sleep")
	}
	// A wall clock 10 minutes ahead of the monotonic one: the machine slept.
	wallJump := time.Unix(0, prev.Add(10*time.Minute).UnixNano())
	if !slept(prev.Round(0), wallJump, 15*time.Second) {
		t.Error("a 10 minute gap is a sleep")
	}
}
