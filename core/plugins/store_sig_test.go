package plugins

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/update"
)

func TestOfficialStoreNeedsSignature(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	index := `{"plugins":[]}`
	sig := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sig") {
			if sig == "" {
				http.NotFound(w, r)
				return
			}
			w.Write([]byte(sig))
			return
		}
		w.Write([]byte(index))
	}))
	defer srv.Close()
	url := strings.Replace(srv.URL, "http://127.0.0.1", "http://localhost", 1) + "/plugins/index.json"
	officialStore = url
	verifyIndex = func(data, s []byte) bool {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(s)))
		return err == nil && ed25519.Verify(pub, data, raw)
	}
	defer func() { officialStore, verifyIndex = OfficialStore, update.Verify }()

	if _, err := FetchIndex(context.Background(), url); errs.CodeOf(err) != ErrStoreSignature {
		t.Fatalf("unsigned index: %v", err)
	}
	sig = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(index)))
	if _, err := FetchIndex(context.Background(), url); err != nil {
		t.Fatalf("signed index: %v", err)
	}
	index = `{"plugins":[{"id":"evil"}]}` // changed after signing
	if _, err := FetchIndex(context.Background(), url); errs.CodeOf(err) != ErrStoreSignature {
		t.Fatalf("tampered index: %v", err)
	}
}
