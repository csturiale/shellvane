package vault

import (
	"bytes"
	"context"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
)

// Small parameters keep tests fast; production uses DefaultKDF.
var testKDF = KDFParams{Time: 1, Memory: 1024, Threads: 1}

func newVault(t *testing.T) *Vault {
	t.Helper()
	db, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db, testKDF)
}

func wantCode(t *testing.T, err error, code errs.Code) {
	t.Helper()
	if errs.CodeOf(err) != code {
		t.Fatalf("err = %v, want %s", err, code)
	}
}

func state(t *testing.T, v *Vault) State {
	s, err := v.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestLifecycle(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	if state(t, v) != StateNotSetUp {
		t.Fatal("new vault should not be set up")
	}
	wantCode(t, v.Unlock(ctx, []byte("whatever1")), ErrNotSetUp)
	wantCode(t, v.Setup(ctx, []byte("short")), ErrWeakPassword)

	if err := v.Setup(ctx, []byte("correct horse")); err != nil {
		t.Fatal(err)
	}
	if state(t, v) != StateUnlocked {
		t.Fatal("setup should leave the vault unlocked")
	}
	wantCode(t, v.Setup(ctx, []byte("another one")), ErrAlreadySetUp)

	id, err := v.Put(ctx, "password", []byte("s3cret"))
	if err != nil {
		t.Fatal(err)
	}

	v.Lock()
	if state(t, v) != StateLocked {
		t.Fatal("vault should be locked")
	}
	_, err = v.Get(ctx, id)
	wantCode(t, err, ErrLocked)
	wantCode(t, v.Unlock(ctx, []byte("wrong password")), ErrWrongPassword)

	if err := v.Unlock(ctx, []byte("correct horse")); err != nil {
		t.Fatal(err)
	}
	got, err := v.Get(ctx, id)
	if err != nil || string(got) != "s3cret" {
		t.Fatalf("Get = %q, %v", got, err)
	}
}

func TestChangePasswordKeepsSecrets(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	if err := v.Setup(ctx, []byte("first password")); err != nil {
		t.Fatal(err)
	}
	id, _ := v.Put(ctx, "password", []byte("data"))

	wantCode(t, v.ChangePassword(ctx, []byte("not it at all"), []byte("second password")), ErrWrongPassword)
	if err := v.ChangePassword(ctx, []byte("first password"), []byte("second password")); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	wantCode(t, v.Unlock(ctx, []byte("first password")), ErrWrongPassword)
	if err := v.Unlock(ctx, []byte("second password")); err != nil {
		t.Fatal(err)
	}
	if got, _ := v.Get(ctx, id); string(got) != "data" {
		t.Fatalf("secret lost after password change: %q", got)
	}
}

func TestCiphertextsAreBoundToTheirRecord(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	if err := v.Setup(ctx, []byte("correct horse")); err != nil {
		t.Fatal(err)
	}
	a, _ := v.Put(ctx, "password", []byte("alpha"))
	b, _ := v.Put(ctx, "password", []byte("beta"))

	// Swap the ciphertexts in the database, as an attacker with file access could.
	if _, err := v.db.Exec(`UPDATE secrets SET ciphertext = (SELECT ciphertext FROM secrets WHERE id = ?) WHERE id = ?`, a, b); err != nil {
		t.Fatal(err)
	}
	_, err := v.Get(ctx, b)
	wantCode(t, err, ErrCorrupt)

	var raw []byte
	v.db.QueryRow(`SELECT ciphertext FROM secrets WHERE id = ?`, a).Scan(&raw)
	if bytes.Contains(raw, []byte("alpha")) {
		t.Fatal("plaintext found in the database")
	}
}

func TestCredentials(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	if err := v.Setup(ctx, []byte("correct horse")); err != nil {
		t.Fatal(err)
	}
	scope := PasswordScope("deploy", "10.0.0.1", 22)

	if _, found, err := v.Credential(ctx, scope); err != nil || found {
		t.Fatalf("no credential expected: found=%v err=%v", found, err)
	}
	if err := v.SaveCredential(ctx, scope, "deploy@10.0.0.1:22", []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := v.SaveCredential(ctx, scope, "deploy@10.0.0.1:22", []byte("two")); err != nil {
		t.Fatal(err)
	}
	got, found, err := v.Credential(ctx, scope)
	if err != nil || !found || string(got) != "two" {
		t.Fatalf("Credential = %q, %v, %v", got, found, err)
	}

	v.Lock()
	if has, _ := v.HasCredential(ctx, scope); !has {
		t.Fatal("metadata must be readable while locked")
	}
	list, _ := v.Credentials(ctx)
	if len(list) != 1 || list[0].Label != "deploy@10.0.0.1:22" {
		t.Fatalf("list = %+v", list)
	}
	_, _, err = v.Credential(ctx, scope)
	wantCode(t, err, ErrLocked)

	if err := v.DeleteCredential(ctx, scope); err != nil {
		t.Fatal(err)
	}
	if has, _ := v.HasCredential(ctx, scope); has {
		t.Fatal("credential should be gone")
	}
	var n int
	v.db.QueryRow(`SELECT COUNT(*) FROM secrets`).Scan(&n)
	if n != 0 {
		t.Fatalf("secret rows left behind: %d", n)
	}
}
