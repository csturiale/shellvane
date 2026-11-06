package vault

import (
	"context"
	"errors"
	"testing"
)

type memStore map[string]string

func (m memStore) Get(a string) (string, error) {
	s, ok := m[a]
	if !ok {
		return "", errors.New("not found")
	}
	return s, nil
}
func (m memStore) Set(a, s string) error { m[a] = s; return nil }
func (m memStore) Delete(a string) error { delete(m, a); return nil }

func TestDeviceUnlock(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	keychain := memStore{}
	wantCode(t, v.EnableDeviceUnlock(ctx, keychain), ErrLocked)
	if err := v.Setup(ctx, []byte("correct horse battery")); err != nil {
		t.Fatal(err)
	}
	id, err := v.Put(ctx, "password", []byte("s3cret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.EnableDeviceUnlock(ctx, keychain); err != nil {
		t.Fatal(err)
	}
	if on, _ := v.HasDeviceUnlock(ctx); !on || len(keychain) != 1 {
		t.Fatalf("enabled = %v, keychain = %v", on, keychain)
	}

	// The master password changes: the keychain still opens the vault.
	if err := v.ChangePassword(ctx, []byte("correct horse battery"), []byte("another long password")); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	if err := v.UnlockWithDevice(ctx, keychain); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Get(ctx, id); err != nil || string(got) != "s3cret" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	// Another computer's keychain does not have the secret.
	v.Lock()
	wantCode(t, v.UnlockWithDevice(ctx, memStore{}), ErrDeviceSecretGone)
	for k := range keychain {
		keychain[k] = "AAAA" + keychain[k][4:]
	}
	wantCode(t, v.UnlockWithDevice(ctx, keychain), ErrDeviceSecretGone)
	if state(t, v) != StateLocked {
		t.Fatal("a wrong secret must leave the vault locked")
	}

	if err := v.DisableDeviceUnlock(ctx, keychain); err != nil {
		t.Fatal(err)
	}
	if on, _ := v.HasDeviceUnlock(ctx); on || len(keychain) != 0 {
		t.Fatalf("still enabled: %v, %v", on, keychain)
	}
	wantCode(t, v.UnlockWithDevice(ctx, keychain), ErrNoDeviceUnlock)
}
