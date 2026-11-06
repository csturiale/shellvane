package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/snippets"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/themes"
	"github.com/csturiale/shellvane/core/update"
)

func TestGalleryInstallsVerifiedItems(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	snippetFile := `{"kind":"shellvane-snippets","version":1,"snippets":[{"folder":"k8s","name":"pods","body":"kubectl get pods"}]}`
	themeFile := `{"name":"Dusk","background":"#101018","foreground":"#e0e0e0","black":"#000000","red":"#ff5555","green":"#50fa7b","yellow":"#f1fa8c","blue":"#6272a4","purple":"#bd93f9","cyan":"#8be9fd","white":"#ffffff","brightBlack":"#444444","brightRed":"#ff6e6e","brightGreen":"#69ff94","brightYellow":"#ffffa5","brightBlue":"#d6acff","brightPurple":"#ff92df","brightCyan":"#a4ffff","brightWhite":"#ffffff"}`
	sum := func(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
	idx := GalleryIndex{
		Snippets: []GallerySnippets{{File: "/gallery/files/snippets/k8s.json", Name: "k8s", SHA256: sum(snippetFile)}},
		Themes:   []GalleryTheme{{File: "/gallery/files/themes/dusk.json", Name: "Dusk", SHA256: sum(themeFile)}},
	}
	index, _ := json.Marshal(idx)
	served := map[string]string{
		"/gallery/index.json":              string(index),
		"/gallery/index.json.sig":          "signed",
		"/gallery/files/snippets/k8s.json": snippetFile,
		"/gallery/files/themes/dusk.json":  themeFile,
		"/gallery/files/themes/other.json": themeFile,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := served[r.URL.Path]; ok {
			w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	GalleryURL = srv.URL + "/gallery/index.json"
	galleryVerify = func(data, sig []byte) bool { return string(sig) == "signed" && string(data) == string(index) }
	defer func() { GalleryURL, galleryVerify = "https://termoraplus.com/gallery/index.json", update.Verify }()

	g := NewGalleryService(NewSnippetService(snippets.New(db)), NewThemeService(themes.NewRepo(db)))
	if got, err := g.Gallery(ctx); err != nil || len(got.Snippets) != 1 || len(got.Themes) != 1 {
		t.Fatalf("index = %+v, %v", got, err)
	}
	if res, err := g.Install(ctx, "snippets", "/gallery/files/snippets/k8s.json"); err != nil || res.Added != 1 {
		t.Fatalf("snippets = %+v, %v", res, err)
	}
	if res, err := g.Install(ctx, "themes", "/gallery/files/themes/dusk.json"); err != nil || res.Added != 1 || res.Names[0] != "Dusk" {
		t.Fatalf("theme = %+v, %v", res, err)
	}
	// Not in the index, or changed after signing: refused.
	if _, err := g.Install(ctx, "themes", "/gallery/files/themes/other.json"); errs.CodeOf(err) != ErrGalleryItem {
		t.Fatalf("unlisted file: %v", err)
	}
	served["/gallery/files/snippets/k8s.json"] = strings.Replace(snippetFile, "get pods", "delete pods --all", 1)
	if _, err := g.Install(ctx, "snippets", "/gallery/files/snippets/k8s.json"); errs.CodeOf(err) != ErrGalleryItem {
		t.Fatalf("changed file: %v", err)
	}
	served["/gallery/index.json.sig"] = "forged"
	if _, err := g.Gallery(ctx); errs.CodeOf(err) != ErrGallerySignature {
		t.Fatalf("unsigned index: %v", err)
	}
}
