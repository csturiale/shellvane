//go:build cgo

package pkcs11

import (
	"crypto/rand"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
)

const softhsm = "/usr/lib/softhsm/libsofthsm2.so"

// softToken makes a SoftHSM token with an RSA and an EC key.
func softToken(t *testing.T) string {
	t.Helper()
	if _, err := os.Stat(softhsm); err != nil {
		t.Skip("SoftHSM not installed")
	}
	for _, tool := range []string{"softhsm2-util", "pkcs11-tool"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not installed")
		}
	}
	dir := t.TempDir()
	conf := filepath.Join(dir, "softhsm2.conf")
	os.MkdirAll(filepath.Join(dir, "tokens"), 0o700)
	os.WriteFile(conf, []byte("directories.tokendir = "+filepath.Join(dir, "tokens")+"\nobjectstore.backend = file\n"), 0o600)
	t.Setenv("SOFTHSM2_CONF", conf)
	run := func(name string, args ...string) {
		if out, err := exec.Command(name, args...).CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	run("softhsm2-util", "--init-token", "--free", "--label", "shellvane-test", "--pin", "123456", "--so-pin", "654321")
	run("pkcs11-tool", "--module", softhsm, "--login", "--pin", "123456", "--keypairgen", "--key-type", "rsa:2048", "--id", "01", "--label", "rsa key")
	run("pkcs11-tool", "--module", softhsm, "--login", "--pin", "123456", "--keypairgen", "--key-type", "EC:prime256v1", "--id", "02", "--label", "ec key")
	return dir
}

func TestTokenKeysAndSignatures(t *testing.T) {
	softToken(t)
	tokens, err := Tokens(softhsm)
	if err != nil {
		t.Fatal(err)
	}
	var serial string
	for _, tk := range tokens {
		if tk.Label == "shellvane-test" {
			serial = tk.Serial
			if !tk.NeedsPIN {
				t.Error("the token needs a PIN")
			}
		}
	}
	if serial == "" {
		t.Fatalf("token not found in %+v", tokens)
	}
	keys, err := Keys(softhsm, serial)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Key{}
	for _, k := range keys {
		byID[k.ID] = k
	}
	if byID["01"].Type != "ssh-rsa" || byID["02"].Type != "ecdsa-sha2-nistp256" || byID["01"].Label != "rsa key" {
		t.Fatalf("keys = %+v", keys)
	}

	data := []byte("session id and request to sign")
	for _, id := range []string{"01", "02"} {
		asked := 0
		signer, err := NewSigner(softhsm, serial, id, byID[id].PublicKey, func(attempt int) (string, error) {
			asked++
			if attempt == 1 {
				return "000000", nil // a wrong PIN first: asked again
			}
			return "123456", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		sshSigner, err := ssh.NewSignerFromSigner(signer)
		if err != nil {
			t.Fatal(err)
		}
		algos := []string{""}
		if id == "01" {
			algos = []string{ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSASHA512}
		}
		for _, algo := range algos {
			var sig *ssh.Signature
			if algo == "" {
				sig, err = sshSigner.Sign(rand.Reader, data)
			} else {
				sig, err = sshSigner.(ssh.AlgorithmSigner).SignWithAlgorithm(rand.Reader, data, algo)
			}
			if err != nil {
				t.Fatalf("key %s %s: %v", id, algo, err)
			}
			if err := sshSigner.PublicKey().Verify(data, sig); err != nil {
				t.Fatalf("key %s %s: signature does not verify: %v", id, algo, err)
			}
		}
		if asked != 2 {
			t.Errorf("key %s: PIN asked %d times, want 2 (once wrong, then right; then kept)", id, asked)
		}
		signer.Close()
	}

	// Three wrong PINs give up (the token would lock after more).
	signer, _ := NewSigner(softhsm, serial, "02", byID["02"].PublicKey, func(int) (string, error) { return "999999", nil })
	if _, err := signer.Sign(rand.Reader, make([]byte, 32), nil); errs.CodeOf(err) != ErrWrongPIN {
		t.Fatalf("wrong PINs: %v", err)
	}
	if _, err := Keys(softhsm, "no-such-serial"); errs.CodeOf(err) != ErrNoToken {
		t.Fatalf("missing token: %v", err)
	}
	if _, err := Tokens("/nonexistent/module.so"); errs.CodeOf(err) != ErrModule {
		t.Fatalf("missing module: %v", err)
	}
}
