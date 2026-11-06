package keys

import (
	"context"
	"crypto"
	"crypto/rand"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/vault"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "keys", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fingerprint(t *testing.T, k any) string {
	t.Helper()
	s, err := ssh.NewSignerFromKey(k)
	if err != nil {
		t.Fatal(err)
	}
	return ssh.FingerprintSHA256(s.PublicKey())
}

func TestParsePPK(t *testing.T) {
	cases := []struct {
		file, pass, fp, comment string
	}{
		// Fingerprints printed by puttygen when the fixtures were made.
		{"plain-v3.ppk", "", "SHA256:dStmDvL+O2hqXxBPztfaM3ChApbQQJ0mRWdqunt4EQw", "test-v3"},
		{"plain-v2.ppk", "", "SHA256:Uhcik5+tyRIop2gqbN3CiE1LDVhzzeFvpkzc4zXANwc", "test-v2"},
		{"enc-v3.ppk", "segreta", "SHA256:hDrP/od0ntDrDBZj43Aiq/juxY9tv/PEhbsgONfu83I", "test-rsa"},
	}
	for _, c := range cases {
		k, comment, err := Parse(fixture(t, c.file), []byte(c.pass))
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		if got := fingerprint(t, k); got != c.fp || comment != c.comment {
			t.Errorf("%s: fingerprint %s comment %q", c.file, got, comment)
		}
	}
	_, _, err := Parse(fixture(t, "enc-v3.ppk"), nil)
	if errs.CodeOf(err) != ErrPassphraseRequired {
		t.Errorf("missing passphrase: %v", err)
	}
	_, _, err = Parse(fixture(t, "enc-v3.ppk"), []byte("wrong"))
	if errs.CodeOf(err) != ErrWrongPassphrase {
		t.Errorf("wrong passphrase: %v", err)
	}
}

func TestGenerateAndParseOpenSSH(t *testing.T) {
	for _, c := range []struct {
		kind string
		bits int
	}{{"ed25519", 0}, {"ecdsa", 384}, {"rsa", 3072}} {
		k, err := Generate(c.kind, c.bits)
		if err != nil {
			t.Fatal(c.kind, err)
		}
		block, err := ssh.MarshalPrivateKeyWithPassphrase(k, "c", []byte("pw"))
		if err != nil {
			t.Fatal(err)
		}
		data := pem.EncodeToMemory(block)
		if _, _, err := Parse(data, nil); errs.CodeOf(err) != ErrPassphraseRequired {
			t.Errorf("%s: %v", c.kind, err)
		}
		if _, _, err := Parse(data, []byte("nope")); errs.CodeOf(err) != ErrWrongPassphrase {
			t.Errorf("%s: %v", c.kind, err)
		}
		back, _, err := Parse(data, []byte("pw"))
		if err != nil || fingerprint(t, back) != fingerprint(t, k) {
			t.Errorf("%s: round trip failed: %v", c.kind, err)
		}
		if bitsOf(k) == 0 {
			t.Errorf("%s: bits unknown", c.kind)
		}
	}
	if _, err := Generate("dsa", 1024); errs.CodeOf(err) != ErrUnsupported {
		t.Errorf("dsa must be refused: %v", err)
	}
}

func TestRepo(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v := vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1})
	if err := v.Setup(ctx, []byte("correct horse")); err != nil {
		t.Fatal(err)
	}
	r := NewRepo(db, v)

	priv, _ := Generate("ed25519", 0)
	k, err := r.Add(ctx, "laptop", priv, "me@laptop")
	if err != nil {
		t.Fatal(err)
	}
	if k.Type != "ssh-ed25519" || k.Bits != 256 || k.PublicKey[:12] != "ssh-ed25519 " {
		t.Errorf("key = %+v", k)
	}
	if _, err := r.Add(ctx, "again", priv, ""); errs.CodeOf(err) != ErrDuplicate {
		t.Errorf("duplicate: %v", err)
	}

	v.Lock()
	if list, _ := r.List(ctx); len(list) != 1 {
		t.Fatal("public data must be listable while locked")
	}
	if _, err := r.Signer(ctx, k.ID); errs.CodeOf(err) != vault.ErrLocked {
		t.Errorf("signer while locked: %v", err)
	}
	v.Unlock(ctx, []byte("correct horse"))
	s, err := r.Signer(ctx, k.ID)
	if err != nil || ssh.FingerprintSHA256(s.PublicKey()) != k.Fingerprint {
		t.Fatalf("signer = %v", err)
	}

	if err := r.Delete(ctx, k.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM secrets`).Scan(&n)
	if n != 0 {
		t.Fatal("private key left in the vault")
	}
}

func TestCertificates(t *testing.T) {
	ctx := context.Background()
	db, _ := store.Open(ctx, ":memory:")
	defer db.Close()
	v := vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1})
	v.Setup(ctx, []byte("correct horse"))
	r := NewRepo(db, v)
	priv, _ := Generate("ed25519", 0)
	k, _ := r.Add(ctx, "work", priv, "")
	other, _ := Generate("ed25519", 0)

	caKey, _ := Generate("ed25519", 0)
	ca, _ := ssh.NewSignerFromSigner(caKey)
	certify := func(key any, validBefore uint64) string {
		s, _ := ssh.NewSignerFromSigner(key.(interface {
			Public() crypto.PublicKey
			Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error)
		}))
		cert := &ssh.Certificate{Key: s.PublicKey(), CertType: ssh.UserCert, KeyId: "me", ValidPrincipals: []string{"deploy"},
			ValidAfter: 0, ValidBefore: validBefore}
		if err := cert.SignCert(rand.Reader, ca); err != nil {
			t.Fatal(err)
		}
		return string(ssh.MarshalAuthorizedKey(cert))
	}

	if err := r.SetCertificate(ctx, k.ID, certify(other, ssh.CertTimeInfinity)); errs.CodeOf(err) != ErrCertificateKey {
		t.Fatalf("a certificate for another key must be refused: %v", err)
	}
	if err := r.SetCertificate(ctx, k.ID, "ssh-ed25519 AAAA nope"); errs.CodeOf(err) != ErrCertificate {
		t.Fatalf("garbage: %v", err)
	}
	if err := r.SetCertificate(ctx, k.ID, certify(priv, ssh.CertTimeInfinity)); err != nil {
		t.Fatal(err)
	}
	list, _ := r.List(ctx)
	if len(list[0].CertPrincipals) != 1 || list[0].CertPrincipals[0] != "deploy" || list[0].CertValidBefore != "" {
		t.Errorf("cert info = %+v", list[0])
	}
	s, err := r.Signer(ctx, k.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.PublicKey().(*ssh.Certificate); !ok {
		t.Fatal("signer should present the certificate")
	}

	// An expired certificate falls back to the plain key.
	r.SetCertificate(ctx, k.ID, certify(priv, uint64(time.Now().Add(-time.Hour).Unix())))
	s, _ = r.Signer(ctx, k.ID)
	if _, ok := s.PublicKey().(*ssh.Certificate); ok {
		t.Fatal("an expired certificate must not be offered")
	}
}
