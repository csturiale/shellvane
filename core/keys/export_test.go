package keys

import (
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestExportRoundTrip(t *testing.T) {
	for _, kind := range []struct {
		name string
		bits int
	}{{"ed25519", 0}, {"rsa", 3072}, {"ecdsa", 256}, {"ecdsa", 521}} {
		priv, err := Generate(kind.name, kind.bits)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := ssh.NewSignerFromSigner(priv)
		for _, format := range []string{FormatOpenSSH, FormatPPK} {
			for _, pass := range []string{"", "s3cret pass"} {
				text, err := Export(priv, "me@laptop", format, pass)
				if err != nil {
					t.Fatalf("%s/%s: %v", kind.name, format, err)
				}
				if format == FormatPPK && !strings.HasPrefix(text, "PuTTY-User-Key-File-3: ") {
					t.Fatalf("not a PPK v3:\n%s", text)
				}
				back, comment, err := Parse([]byte(text), []byte(pass))
				if err != nil {
					t.Fatalf("%s/%s/%q: parse back: %v\n%s", kind.name, format, pass, err, text)
				}
				got, _ := ssh.NewSignerFromSigner(back)
				if ssh.FingerprintSHA256(got.PublicKey()) != ssh.FingerprintSHA256(want.PublicKey()) {
					t.Errorf("%s/%s: different key", kind.name, format)
				}
				if format == FormatPPK && comment != "me@laptop" {
					t.Errorf("comment = %q", comment)
				}
				if pass != "" {
					if _, _, err := Parse([]byte(text), []byte("wrong")); err == nil {
						t.Errorf("%s/%s: wrong passphrase accepted", kind.name, format)
					}
				}
			}
		}
	}
}
