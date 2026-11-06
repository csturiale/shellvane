package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/snippets"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/themes"
)

// TestGalleryContentImports installs every snippet collection and theme of
// the site's gallery with the app's own importers, so a broken file never
// reaches the published (signed) index.
func TestGalleryContentImports(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "g.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	snips := NewSnippetService(snippets.New(db))
	themeSvc := NewThemeService(themes.NewRepo(db))
	dir := filepath.Join("..", "gallery")
	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.json"))
	if len(files) == 0 {
		t.Fatal("no gallery files found")
	}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(f)
		switch filepath.Base(filepath.Dir(f)) {
		case "snippets":
			res, err := snips.Import(ctx, name, string(raw))
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			if res.Added == 0 {
				t.Errorf("%s: nothing imported: %+v", name, res)
			}
			in, _ := snippets.Parse(name, string(raw))
			for _, s := range in {
				if strings.TrimSpace(s.Folder) == "" || strings.TrimSpace(s.Name) == "" {
					t.Errorf("%s: snippet without folder or name: %+v", name, s)
				}
			}
		case "themes":
			got, err := themeSvc.Import(ctx, name, string(raw))
			if err != nil || len(got) != 1 {
				t.Errorf("%s: %v, %v", name, got, err)
				continue
			}
			if n := len(got[0].Colors); n < 20 {
				t.Errorf("%s: only %d colors", name, n)
			}
		default:
			t.Errorf("unexpected gallery folder for %s", f)
		}
	}
}
