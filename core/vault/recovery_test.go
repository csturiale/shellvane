package vault

import (
	"context"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

func TestRecoveryKey(t *testing.T) {
	ctx := context.Background()
	v := newVault(t)
	if err := v.Setup(ctx, []byte("first password")); err != nil {
		t.Fatal(err)
	}
	id, _ := v.Put(ctx, "password", []byte("hunter2"))
	if has, _ := v.HasRecoveryKey(ctx); has {
		t.Fatal("no recovery key yet")
	}
	code, err := v.CreateRecoveryKey(ctx)
	if err != nil || len(strings.Split(code, "-")) != 8 {
		t.Fatalf("code = %q, %v", code, err)
	}
	v.Lock()
	if _, err := v.CreateRecoveryKey(ctx); errs.CodeOf(err) != ErrLocked {
		t.Fatalf("locked: %v", err)
	}
	if err := v.Recover(ctx, "AAAA-BBBB", []byte("new password")); errs.CodeOf(err) != ErrWrongRecoveryKey {
		t.Fatalf("wrong key: %v", err)
	}
	// Spaces and lower case are tolerated.
	if err := v.Recover(ctx, " "+strings.ToLower(strings.ReplaceAll(code, "-", " "))+" ", []byte("new password")); err != nil {
		t.Fatal(err)
	}
	if got, err := v.Get(ctx, id); err != nil || string(got) != "hunter2" {
		t.Fatalf("secret after recovery = %q, %v", got, err)
	}
	v.Lock()
	if err := v.Unlock(ctx, []byte("first password")); errs.CodeOf(err) != ErrWrongPassword {
		t.Fatalf("old password still works: %v", err)
	}
	if err := v.Unlock(ctx, []byte("new password")); err != nil {
		t.Fatal(err)
	}
	if err := v.RemoveRecoveryKey(ctx); err != nil {
		t.Fatal(err)
	}
	if err := v.Recover(ctx, code, []byte("other password")); errs.CodeOf(err) != ErrNoRecoveryKey {
		t.Fatalf("removed key: %v", err)
	}
}
