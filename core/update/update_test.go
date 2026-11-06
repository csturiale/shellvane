package update

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"1.2.10", "v1.2.9", true}, {"1.2.9", "1.2.10", false}, {"1.0.0", "1.0.0", false},
		{"1.0.0", "1.0.0-rc1", true}, {"1.0.0-rc2", "1.0.0-rc1", true}, {"0.1.0", "dev", true}, {"dev", "0.1.0", false},
		{"2", "1.9.9", true},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestCheckDownloadInstall(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	payload := []byte("new shellvane binary")
	sum := sha256.Sum256(payload)
	var manifest []byte
	var sig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			w.Write(manifest)
		case "/manifest.json.sig":
			w.Write([]byte(sig))
		case "/pkg":
			w.Write(payload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	m := Manifest{Version: "9.9.9", Assets: map[string]Asset{Platform(): {URL: srv.URL + "/pkg", SHA256: hex.EncodeToString(sum[:]), Kind: "binary"}}}
	manifest, _ = json.Marshal(m)
	sig, _ = Sign(manifest, base64.StdEncoding.EncodeToString(priv))

	c := &Checker{Client: srv.Client(), URL: srv.URL + "/manifest.json", PublicKey: base64.StdEncoding.EncodeToString(pub)}
	got, err := c.Latest(context.Background())
	if err != nil || got.Version != "9.9.9" {
		t.Fatalf("Latest = %+v, %v", got, err)
	}

	// No release yet (or a private repository): not an error to show.
	if _, err := (&Checker{Client: srv.Client(), URL: srv.URL + "/none.json", PublicKey: c.PublicKey}).Latest(context.Background()); errs.CodeOf(err) != ErrNoRelease {
		t.Fatalf("missing manifest: %v", err)
	}

	// A different key, or a changed manifest, is refused.
	otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
	bad := *c
	bad.PublicKey = base64.StdEncoding.EncodeToString(otherPub)
	if _, err := bad.Latest(context.Background()); errs.CodeOf(err) != ErrSignature {
		t.Fatalf("wrong key: %v", err)
	}
	manifest = append(manifest, ' ')
	if _, err := c.Latest(context.Background()); errs.CodeOf(err) != ErrSignature {
		t.Fatalf("tampered manifest: %v", err)
	}
	if _, err := (&Checker{URL: c.URL}).Latest(context.Background()); errs.CodeOf(err) != ErrDisabled {
		t.Fatalf("no key: %v", err)
	}

	dir := t.TempDir()
	file, err := c.Download(context.Background(), m.Assets[Platform()], dir)
	if err != nil {
		t.Fatal(err)
	}
	broken := m.Assets[Platform()]
	broken.SHA256 = hex.EncodeToString(make([]byte, 32))
	if _, err := c.Download(context.Background(), broken, dir); errs.CodeOf(err) != ErrChecksum {
		t.Fatalf("bad checksum: %v", err)
	}

	exe := filepath.Join(dir, "shellvane")
	os.WriteFile(exe, []byte("old"), 0o755)
	if err := Install(file, exe); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(exe); string(data) != string(payload) {
		t.Fatalf("installed = %q", data)
	}
	if _, err := os.Stat(exe + ".old"); err != nil {
		t.Fatal("old version not kept")
	}
	Cleanup(exe)
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Fatal("old version not removed")
	}
}
