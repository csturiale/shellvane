package sshengine

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/csturiale/shellvane/core/knownhosts"
)

func plainKey(t *testing.T) (string, ssh.Signer) {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	block, err := ssh.MarshalPrivateKey(priv, "test")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	signer, _ := ssh.NewSignerFromKey(priv)
	return path, signer
}

func newSigner(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s, priv
}

// startAgent serves keys on a fresh SSH_AUTH_SOCK.
func startAgent(t *testing.T, keys ...ed25519.PrivateKey) {
	keyring := agent.NewKeyring()
	for _, k := range keys {
		if err := keyring.Add(agent.AddedKey{PrivateKey: k}); err != nil {
			t.Fatal(err)
		}
	}
	sock := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("unix sockets not available:", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, c)
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", sock)
}

// The host's key and the agent's are refused: the key file is still tried
// (the client skips a second publickey method once one has failed).
func TestTriesEveryKeyInTurn(t *testing.T) {
	_, agentKey := newSigner(t)
	startAgent(t, agentKey)
	path, good := plainKey(t)
	identity, _ := newSigner(t)
	srv := startServer(t, "s3cret", good.PublicKey())
	p := &fakePrompter{decision: knownhosts.AcceptOnce}
	opts := options(t, p)
	opts.Identity = identity
	opts.IdentityFiles = []string{path, path} // the duplicate is offered once
	opts.Trace = NewTrace()

	client, err := Dial(context.Background(), srv.target(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if p.asked != 0 {
		t.Fatal("the key file should log in without a password")
	}
	var offered []string
	for _, e := range opts.Trace.Events() {
		if e.Step == StepKeyOffered {
			offered = append(offered, e.Detail["fingerprint"])
		}
	}
	want := []string{ssh.FingerprintSHA256(identity.PublicKey()), ssh.FingerprintSHA256(must(ssh.NewSignerFromKey(agentKey)).PublicKey()), ssh.FingerprintSHA256(good.PublicKey())}
	if !slices.Equal(offered, want) {
		t.Errorf("offered %v, want %v", offered, want)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func TestIdentitiesOnlySkipsOtherKeys(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	path, good := plainKey(t)
	identity, _ := newSigner(t)
	srv := startServer(t, "s3cret", good.PublicKey())
	p := &fakePrompter{passwords: []string{"s3cret"}, decision: knownhosts.AcceptOnce}
	opts := options(t, p)
	opts.Identity = identity
	opts.IdentityFiles = []string{path}
	opts.IdentitiesOnly = true

	client, err := Dial(context.Background(), srv.target(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if p.asked != 1 {
		t.Fatalf("password asked %d times, want 1: the key file must not be offered", p.asked)
	}
}

func TestAuthOrderPrefersPassword(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	path, good := plainKey(t)
	srv := startServer(t, "s3cret", good.PublicKey())

	p := &fakePrompter{passwords: []string{"s3cret"}, decision: knownhosts.AcceptOnce}
	opts := options(t, p)
	opts.IdentityFiles = []string{path}
	opts.AuthOrder = []string{AuthPassword, AuthPublicKey}
	client, err := Dial(context.Background(), srv.target(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if p.asked != 1 {
		t.Fatalf("password asked %d times, want 1", p.asked)
	}

	// Without publickey in the list the key is never used.
	p = &fakePrompter{decision: knownhosts.AcceptOnce}
	opts = options(t, p)
	opts.IdentityFiles = []string{path}
	opts.AuthOrder = []string{AuthPassword}
	if _, err := Dial(context.Background(), srv.target(t), opts); err == nil {
		t.Fatal("logged in although publickey is not allowed")
	}
}

func TestNormalizeAuthOrder(t *testing.T) {
	cases := []struct{ in, want []string }{
		{nil, DefaultAuthOrder},
		{[]string{"bogus"}, DefaultAuthOrder},
		{[]string{"password", " publickey", "password"}, []string{"password", "publickey"}},
	}
	for _, c := range cases {
		if got := NormalizeAuthOrder(c.in); !slices.Equal(got, c.want) {
			t.Errorf("NormalizeAuthOrder(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
