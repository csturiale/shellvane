package keys

import (
	"context"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/vault"
)

func TestTokenKeys(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v := vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1})
	r := NewRepo(db, v)

	// The public half of a key that would live on the token.
	priv, _ := Generate("ecdsa", 256)
	signer, _ := ssh.NewSignerFromSigner(priv)
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	ref := TokenRef{Module: "/usr/lib/opensc-pkcs11.so", Serial: "0123", Label: "YubiKey PIV", KeyID: "01"}

	// The vault is not even set up: nothing secret is stored.
	k, err := r.AddToken(ctx, "yubikey", ref, line)
	if err != nil {
		t.Fatal(err)
	}
	if k.Type != "ecdsa-sha2-nistp256" || k.Bits != 256 || k.Token == nil || *k.Token != ref {
		t.Fatalf("key = %+v", k)
	}
	if _, err := r.AddToken(ctx, "again", ref, line); errs.CodeOf(err) != ErrDuplicate {
		t.Fatalf("duplicate: %v", err)
	}
	list, _ := r.List(ctx)
	if len(list) != 1 || list[0].Token == nil || list[0].Token.Serial != "0123" {
		t.Fatalf("list = %+v", list)
	}
	got, pub, _, ok, err := r.TokenOf(ctx, k.ID)
	if err != nil || !ok || got != ref || pub != k.PublicKey {
		t.Fatalf("TokenOf = %+v %q %v %v", got, pub, ok, err)
	}
	if _, err := r.Signer(ctx, k.ID); errs.CodeOf(err) != ErrOnToken {
		t.Fatalf("Signer on a token key: %v", err)
	}
	if _, _, err := r.Private(ctx, k.ID); errs.CodeOf(err) != ErrOnToken {
		t.Fatalf("Private on a token key: %v", err)
	}
	if err := r.Delete(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.List(ctx); len(list) != 0 {
		t.Fatal("not deleted")
	}
}
