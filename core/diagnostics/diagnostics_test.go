package diagnostics

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/anonymize"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/sshengine"
)

func ptr[T any](v T) *T { return &v }

func TestBundleHasNoRealNamesOrSecrets(t *testing.T) {
	inv := hosts.Inventory{
		Groups: []hosts.Group{{ID: "g1", Name: "Customers ACME", Settings: settings.Settings{User: ptr("deploy")}}},
		Hosts: []hosts.Host{
			{ID: "h1", GroupID: "g1", Name: "acme-db", Hostname: "db.acme.example", Tags: []string{"acme"}, Notes: "root pw in 1password",
				Settings: settings.Settings{Port: ptr(2222), PasswordCommand: ptr("op read op://acme/db"), JumpChain: []string{"h2"}}},
			{ID: "h2", Name: "bastion", Hostname: "203.0.113.7"},
		},
	}
	hs := Handshake{ID: "x", Name: "acme-db", Address: "db.acme.example:2222", User: "deploy", Started: time.Now(), Code: "ssh.authFailed",
		Events: []sshengine.TraceEvent{
			{Step: "dial", Detail: map[string]string{"address": "db.acme.example:2222", "user": "deploy"}},
			{Step: "error", Detail: map[string]string{"message": "ssh: unable to authenticate deploy@db.acme.example via 203.0.113.7"}},
		}}
	files := Bundle(BundleInput{Version: "1.0", Inventory: inv, Handshakes: []Handshake{hs},
		Preferences: map[string]string{"terminal.fontSize": "14", "quick.history": `["deploy@db.acme.example"]`}}, anonymize.New())
	var all strings.Builder
	for _, f := range files {
		all.WriteString(f.Content)
	}
	text := all.String()
	for _, secret := range []string{"acme", "ACME", "deploy", "203.0.113.7", "bastion", "1password", "op read"} {
		if strings.Contains(text, secret) {
			t.Errorf("bundle contains %q:\n%s", secret, text)
		}
	}
	for _, want := range []string{"host-01", "user-01", "group-01", `"port": 2222`, `"passwordCommand": "(set)"`, "terminal.fontSize", "ssh.authFailed"} {
		if !strings.Contains(text, want) {
			t.Errorf("bundle lacks %q:\n%s", want, text)
		}
	}
	data, err := Zip(files)
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(r.File) != len(files) {
		t.Fatalf("zip: %v, %d files", err, len(r.File))
	}
}

func TestLogKeepsNewestFirst(t *testing.T) {
	l := NewLog(2)
	l.Add(Handshake{ID: "1"})
	l.Add(Handshake{ID: "2"})
	l.Add(Handshake{ID: "3"})
	got := l.List()
	if len(got) != 2 || got[0].ID != "3" || got[1].ID != "2" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := l.Get("1"); ok {
		t.Fatal("oldest kept")
	}
}
