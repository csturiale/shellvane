package knownhosts

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
)

func newKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

var addr = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 2222}

func answer(d Decision, asked *int) Confirm {
	return func(context.Context, UnknownKey) (Decision, error) {
		*asked++
		return d, nil
	}
}

func TestTrustOnFirstUseThenRemember(t *testing.T) {
	ctx := context.Background()
	s := New(filepath.Join(t.TempDir(), "known_hosts"))
	key := newKey(t)
	asked := 0

	cb, err := s.Callback(ctx, answer(AcceptSave, &asked))
	if err != nil {
		t.Fatal(err)
	}
	if err := cb("example.org:2222", addr, key); err != nil {
		t.Fatalf("first connection: %v", err)
	}

	// A fresh callback reads the saved key and does not ask again.
	cb, _ = s.Callback(ctx, answer(Reject, &asked))
	if err := cb("example.org:2222", addr, key); err != nil {
		t.Fatalf("second connection: %v", err)
	}
	if asked != 1 {
		t.Fatalf("asked %d times, want 1", asked)
	}
	if algs := s.Algorithms("example.org", 2222); len(algs) != 1 || algs[0] != ssh.KeyAlgoED25519 {
		t.Errorf("algorithms = %v", algs)
	}
	if algs := s.Algorithms("other.org", 22); len(algs) != 0 {
		t.Errorf("unknown host should have no preference, got %v", algs)
	}
}

func TestChangedKeyIsRefusedWithoutAsking(t *testing.T) {
	ctx := context.Background()
	s := New(filepath.Join(t.TempDir(), "known_hosts"))
	asked := 0
	cb, _ := s.Callback(ctx, answer(AcceptSave, &asked))
	if err := cb("example.org:22", addr, newKey(t)); err != nil {
		t.Fatal(err)
	}

	cb, _ = s.Callback(ctx, answer(AcceptSave, &asked))
	err := cb("example.org:22", addr, newKey(t))
	if errs.CodeOf(err) != ErrHostKeyChanged {
		t.Fatalf("err = %v, want %s", err, ErrHostKeyChanged)
	}
	if asked != 1 {
		t.Fatalf("a changed key must not be offered for confirmation (asked %d)", asked)
	}
}

func TestRejectAndAcceptOnce(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "known_hosts")
	s := New(file)
	asked := 0

	cb, _ := s.Callback(ctx, answer(Reject, &asked))
	if err := cb("example.org:22", addr, newKey(t)); errs.CodeOf(err) != ErrHostKeyRejected {
		t.Fatalf("err = %v, want %s", err, ErrHostKeyRejected)
	}

	cb, _ = s.Callback(ctx, answer(AcceptOnce, &asked))
	if err := cb("example.org:22", addr, newKey(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("accept once must not write the file")
	}
}

func TestUserFileIsTrustedButNeverWritten(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user_known_hosts")
	key := newKey(t)
	line := "example.org " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + "\n"
	if err := os.WriteFile(user, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(dir, "app_known_hosts"), user)
	asked := 0
	cb, _ := s.Callback(context.Background(), answer(AcceptSave, &asked))
	if err := cb("example.org:22", addr, key); err != nil {
		t.Fatal(err)
	}
	if err := cb("new.org:22", addr, newKey(t)); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(user)
	if string(data) != line {
		t.Fatal("user known_hosts was modified")
	}
}
