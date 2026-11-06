package importers

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/csturiale/shellvane/core/tunnel"
)

func TestOpenSSH(t *testing.T) {
	res, err := OpenSSH(filepath.Join("..", "..", "testdata", "importers", "openssh", "config"))
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]Host{}
	for _, h := range res.Hosts {
		byName[h.Name] = h
	}
	names := []string{}
	for _, h := range res.Hosts {
		names = append(names, h.Name)
	}
	if want := []string{"ci", "bastion", "jump", "db-prod", "quoted name"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("hosts = %v, want %v", names, want)
	}

	b := byName["jump"]
	if b.Hostname != "bastion.example.org" || b.User != "deploy" || b.Port != 2200 {
		t.Errorf("alias should share the block's settings: %+v", b)
	}
	if !reflect.DeepEqual(b.Tunnels, []tunnel.Spec{{Kind: tunnel.Dynamic, BindAddr: "127.0.0.1", BindPort: 1080}}) {
		t.Errorf("dynamic forward = %+v", b.Tunnels)
	}

	db := byName["db-prod"]
	if len(db.ProxyJump) != 1 || db.ProxyJump[0] != "bastion" {
		t.Errorf("ProxyJump = %v", db.ProxyJump)
	}
	if db.Hostname != "10.0.2.15" || db.Port != 0 {
		t.Errorf("db-prod = %+v", db)
	}
	wantTunnels := []tunnel.Spec{
		{Kind: tunnel.Local, BindAddr: "127.0.0.1", BindPort: 15432, DestHost: "localhost", DestPort: 5432},
		{Kind: tunnel.Local, BindAddr: "127.0.0.1", BindPort: 16379, DestHost: "::1", DestPort: 6379},
	}
	if !reflect.DeepEqual(db.Tunnels, wantTunnels) {
		t.Errorf("local forwards = %+v", db.Tunnels)
	}
	if ci := byName["ci"]; len(ci.Tunnels) != 1 || ci.Tunnels[0].Kind != tunnel.Remote {
		t.Errorf("included file not parsed: %+v", ci)
	}
	if q := byName["quoted name"]; q.Hostname != "quoted.example.org" {
		t.Errorf("quoted alias = %+v", q)
	}
	if q := byName["quoted name"]; !reflect.DeepEqual(q.Environment, []string{"APP_ENV=prod", "GREETING=hello world"}) || q.RemoteCommand != "tmux attach" {
		t.Errorf("SetEnv/RemoteCommand = %+v", q)
	}
	if q := byName["quoted name"]; !reflect.DeepEqual(q.AuthMethods, []string{"keyboard-interactive", "publickey"}) || !q.IdentitiesOnly {
		t.Errorf("PreferredAuthentications/IdentitiesOnly = %+v", q)
	}

	codes := map[string]int{}
	for _, w := range res.Warnings {
		codes[w.Code]++
	}
	// Host *, web-*, Match, IdentityFile (bastion block).
	if codes["patternSkipped"] != 2 || codes["matchSkipped"] != 1 || codes["directiveSkipped"] != 1 {
		t.Errorf("warnings = %+v", res.Warnings)
	}
}

func TestOpenSSHIncludeLoop(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config")
	os.WriteFile(cfg, []byte("Include config\nHost a\n"), 0o600)
	res, err := OpenSSH(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) == 0 || res.Warnings[0].Code != "includeLoop" {
		t.Fatalf("warnings = %+v", res.Warnings)
	}
}

func FuzzSplitDirective(f *testing.F) {
	f.Add("Host a b")
	f.Add(`LocalForward="8080 localhost:80"`)
	f.Fuzz(func(t *testing.T, line string) {
		splitDirective(line) // must not panic
		splitBind(line)
	})
}
