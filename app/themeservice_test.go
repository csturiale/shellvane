package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/themes"
)

func TestThemeImportAndExport(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	svc := NewThemeService(themes.NewRepo(db))
	kitty := "background #101010\nforeground #eeeeee\ncolor5 #aa00aa\n"
	first, err := svc.Import(ctx, "Night.conf", kitty)
	if err != nil || len(first) != 1 || first[0].Name != "Night" {
		t.Fatalf("Import = %+v, %v", first, err)
	}
	again, err := svc.Import(ctx, "Night.conf", kitty)
	if err != nil || again[0].Name != "Night (2)" {
		t.Fatalf("second import = %+v, %v", again, err)
	}
	out, err := svc.Export(first[0])
	if err != nil || !strings.Contains(out, `"purple": "#aa00aa"`) || !strings.Contains(out, `"name": "Night"`) {
		t.Fatalf("Export = %s, %v", out, err)
	}
	back, err := themes.Parse("x.json", []byte(out))
	if err != nil || back[0].Colors["magenta"] != "#aa00aa" {
		t.Fatalf("round trip = %+v, %v", back, err)
	}
}
