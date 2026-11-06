package sshurl

import "testing"

func TestParse(t *testing.T) {
	cases := map[string]Link{
		"ssh://example.com":                          {Scheme: "ssh", Host: "example.com"},
		"ssh://alice@example.com:2222":               {Scheme: "ssh", User: "alice", Host: "example.com", Port: 2222},
		"SSH://bob;fingerprint=ssh-ed25519-AbC@h:22": {Scheme: "ssh", User: "bob", Host: "h"},
		"ssh://root@[2001:db8::1]:2200":              {Scheme: "ssh", User: "root", Host: "2001:db8::1", Port: 2200},
		"sftp://u@files.example.com/var/www/":        {Scheme: "sftp", User: "u", Host: "files.example.com", Path: "/var/www/"},
		"sftp://u@files.example.com/~/logs":          {Scheme: "sftp", User: "u", Host: "files.example.com", Path: "~/logs"},
		"sftp://files.example.com/":                  {Scheme: "sftp", Host: "files.example.com"},
	}
	for in, want := range cases {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"http://x", "ssh://", "ssh://h:99999", "ssh:/x", "ftp://h"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
	if s := (Link{User: "a", Host: "::1", Port: 2222}).Spec(); s != "a@[::1]:2222" {
		t.Errorf("Spec = %q", s)
	}
	if !Is(" SFTP://x") || Is("https://x") {
		t.Error("Is")
	}
}
