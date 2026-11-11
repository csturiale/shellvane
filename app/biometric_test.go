package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/biometric"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/vault"
)

type memSecrets map[string]string

func (m memSecrets) Get(a string) (string, error) {
	if s, ok := m[a]; ok {
		return s, nil
	}
	return "", errors.New("not found")
}
func (m memSecrets) Set(a, s string) error { m[a] = s; return nil }
func (m memSecrets) Delete(a string) error { delete(m, a); return nil }

type fakeVerifier struct {
	answer error
	asked  []string
}

func (f *fakeVerifier) Kind() string                   { return biometric.KindTouchID }
func (f *fakeVerifier) Available(context.Context) bool { return true }
func (f *fakeVerifier) Verify(_ context.Context, reason string) error {
	f.asked = append(f.asked, reason)
	return f.answer
}

func TestBiometricGuardsTheKeychainUnlock(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vs := NewVaultService(vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1}), func(string, any) {})
	vs.secrets = memSecrets{}
	bio := &fakeVerifier{}
	EnableBiometric(vs, bio, func() string { return "unlock Shellvane's vault" })

	if err := vs.Setup(ctx, "master password"); err != nil {
		t.Fatal(err)
	}
	if err := vs.SetBiometric(ctx, true); errs.CodeOf(err) != vault.ErrNoDeviceUnlock {
		t.Fatalf("biometric without the keychain unlock: %v", err)
	}
	if err := vs.EnableKeychainUnlock(ctx); err != nil {
		t.Fatal(err)
	}
	bio.answer = errs.New(biometric.ErrCancelled, nil)
	if err := vs.SetBiometric(ctx, true); errs.CodeOf(err) != biometric.ErrCancelled {
		t.Fatalf("turning it on must verify first: %v", err)
	}
	if st := vs.Biometric(ctx); st.Required || !st.Available || st.Kind != biometric.KindTouchID {
		t.Fatalf("status = %+v", st)
	}
	bio.answer = nil
	if err := vs.SetBiometric(ctx, true); err != nil {
		t.Fatal(err)
	}

	vs.Lock(ctx)
	bio.answer = errs.New(biometric.ErrCancelled, nil)
	if err := vs.UnlockWithKeychain(ctx); errs.CodeOf(err) != biometric.ErrCancelled {
		t.Fatalf("cancelled Touch ID: %v", err)
	}
	if st, _ := vs.Status(ctx); st.State != vault.StateLocked {
		t.Fatal("a cancelled verification must leave the vault locked")
	}
	bio.answer = nil
	if err := vs.UnlockWithKeychain(ctx); err != nil {
		t.Fatal(err)
	}
	if st, _ := vs.Status(ctx); st.State != vault.StateUnlocked {
		t.Fatal("not unlocked")
	}
	if len(bio.asked) != 4 || bio.asked[0] != "unlock Shellvane's vault" {
		t.Fatalf("asked %q", bio.asked)
	}

	// Off again: the keychain opens the vault by itself.
	if err := vs.SetBiometric(ctx, false); err != nil {
		t.Fatal(err)
	}
	vs.Lock(ctx)
	if err := vs.UnlockWithKeychain(ctx); err != nil || len(bio.asked) != 4 {
		t.Fatalf("unlock without biometric: %v, asked %d", err, len(bio.asked))
	}
}

// notFoundSecrets answers like the system keychain without the secret.
type notFoundSecrets struct{ memSecrets }

func (n notFoundSecrets) Get(a string) (string, error) {
	if s, ok := n.memSecrets[a]; ok {
		return s, nil
	}
	return "", errs.New("keychain.notFound", nil)
}

func TestKeychainUnlockAsksOnceAndStopsWhenTheSecretIsGone(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vs := NewVaultService(vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1}), func(string, any) {})
	secrets := notFoundSecrets{memSecrets{}}
	vs.secrets = secrets
	bio := &fakeVerifier{}
	EnableBiometric(vs, bio, func() string { return "unlock" })
	if err := vs.Setup(ctx, "master password"); err != nil {
		t.Fatal(err)
	}
	if err := vs.EnableKeychainUnlock(ctx); err != nil {
		t.Fatal(err)
	}
	if err := vs.SetBiometric(ctx, true); err != nil {
		t.Fatal(err)
	}

	// Many parts of the app ask together: Touch ID is asked once.
	vs.Lock(ctx)
	bio.asked = nil
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() { done <- vs.UnlockWithKeychain(ctx) }()
	}
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if len(bio.asked) != 1 {
		t.Fatalf("Touch ID asked %d times", len(bio.asked))
	}

	// The keychain lost the secret: no Touch ID for nothing, and the
	// keychain unlock is turned off until the master password turns it on.
	vs.Lock(ctx)
	for k := range secrets.memSecrets {
		delete(secrets.memSecrets, k)
	}
	bio.asked = nil
	if err := vs.UnlockWithKeychain(ctx); errs.CodeOf(err) != vault.ErrDeviceSecretGone {
		t.Fatalf("gone: %v", err)
	}
	if len(bio.asked) != 0 {
		t.Fatal("Touch ID was asked for a secret that is gone")
	}
	if has, _ := vs.HasKeychainUnlock(ctx); has {
		t.Fatal("the keychain unlock must be turned off")
	}
	if st := vs.Biometric(ctx); st.Required {
		t.Fatal("Touch ID must be turned off with it")
	}

	// The master password turns both on again, with a new secret.
	if err := vs.Unlock(ctx, "master password"); err != nil {
		t.Fatal(err)
	}
	if has, _ := vs.HasKeychainUnlock(ctx); !has || len(secrets.memSecrets) != 1 {
		t.Fatal("the master password must turn the keychain unlock on again")
	}
	if st := vs.Biometric(ctx); !st.Required {
		t.Fatal("Touch ID must be required again")
	}
	vs.Lock(ctx)
	if err := vs.UnlockWithKeychain(ctx); err != nil {
		t.Fatalf("keychain unlock after the relink: %v", err)
	}
}

func TestActivityPostponesTheAutoLock(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	vs := NewVaultService(vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1}), func(string, any) {})
	vs.activity() // locked vault, no timer: nothing to do
	if err := vs.Setup(ctx, "master password"); err != nil {
		t.Fatal(err)
	}
	vs.mu.Lock()
	vs.autoLock = time.Hour
	vs.mu.Unlock()
	vs.touch()

	// Within activityEvery of the last restart nothing changes; later the
	// timer starts again.
	vs.mu.Lock()
	first := vs.lastActivity
	vs.mu.Unlock()
	vs.activity()
	vs.mu.Lock()
	if !vs.lastActivity.Equal(first) {
		t.Fatal("activity must be rate limited")
	}
	vs.lastActivity = first.Add(-time.Minute)
	vs.mu.Unlock()
	vs.activity()
	vs.mu.Lock()
	restarted := vs.lastActivity.After(first)
	vs.mu.Unlock()
	if !restarted {
		t.Fatal("activity must restart the auto-lock timer")
	}

	vs.Lock(ctx)
	vs.mu.Lock()
	if vs.timer != nil {
		t.Fatal("locking must drop the timer")
	}
	vs.mu.Unlock()
	vs.activity() // must not start a timer on a locked vault
	vs.mu.Lock()
	defer vs.mu.Unlock()
	if vs.timer != nil {
		t.Fatal("no timer on a locked vault")
	}
}
