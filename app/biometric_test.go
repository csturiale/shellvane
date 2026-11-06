package app

import (
	"context"
	"errors"
	"testing"

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
