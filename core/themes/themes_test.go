package themes

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
)

func parseFile(t *testing.T, name string) []Theme {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "themes", name))
	if err != nil {
		t.Fatal(err)
	}
	out, err := Parse(name, data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	for i := range out {
		if err := out[i].Normalize(); err != nil {
			t.Fatalf("%s: normalize: %v", name, err)
		}
	}
	return out
}

func TestParseFormats(t *testing.T) {
	cases := []struct {
		file, name string
		dark       bool
		colors     map[string]string
	}{
		{"Snazzy.itermcolors", "Snazzy", true, map[string]string{"background": "#282a36", "foreground": "#f1f0f0", "red": "#ff5c5c", "brightRed": "#ff5c5c"}},
		{"ocean.yaml", "Ocean", true, map[string]string{"background": "#2b303b", "foreground": "#c0c5ce", "red": "#bf616a", "brightBlack": "#65737e", "brightWhite": "#eff1f5"}},
		{"alacritty.toml", "alacritty", true, map[string]string{"background": "#282a36", "red": "#ff5555", "magenta": "#ff79c6", "brightRed": "#ff6e6e", "cursor": "#f8f8f2"}},
		{"kitty.conf", "kitty", true, map[string]string{"background": "#282828", "red": "#cc241d", "brightRed": "#fb4934"}},
		{"ghostty", "ghostty", true, map[string]string{"background": "#1d1f21", "black": "#1d1f21", "red": "#cc6666", "cursor": "#aeafad"}},
	}
	for _, c := range cases {
		got := parseFile(t, c.file)
		if len(got) != 1 || got[0].Name != c.name || got[0].Dark != c.dark {
			t.Errorf("%s: %+v", c.file, got)
			continue
		}
		for k, v := range c.colors {
			if got[0].Colors[k] != v {
				t.Errorf("%s: %s = %q, want %q", c.file, k, got[0].Colors[k], v)
			}
		}
		if len(got[0].Colors) < 19 {
			t.Errorf("%s: missing colors: %v", c.file, got[0].Colors)
		}
	}
	wt := parseFile(t, "settings.json")
	if len(wt) != 2 || wt[0].Name != "Campbell" || wt[0].Colors["magenta"] != "#881798" || wt[0].Colors["brightMagenta"] != "#b4009e" || wt[1].Dark {
		t.Errorf("windows terminal: %+v", wt)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	if _, err := Parse("x.json", []byte(`{"hello": 1}`)); errs.CodeOf(err) != ErrFormat {
		t.Errorf("err = %v", err)
	}
	if _, err := Parse("notes.txt", []byte("just some text")); errs.CodeOf(err) != ErrFormat {
		t.Errorf("err = %v", err)
	}
}

func TestNormalizeColor(t *testing.T) {
	for in, want := range map[string]string{"#ABC": "#aabbcc", "0x112233": "#112233", "112233ff": "#112233", "rgb(1, 2, 255)": "#0102ff", `"#010203"`: "#010203"} {
		if got, err := NormalizeColor(in); err != nil || got != want {
			t.Errorf("%q → %q, %v", in, got, err)
		}
	}
	if _, err := NormalizeColor("red"); err == nil {
		t.Error("named colors are not supported")
	}
}

func TestRepo(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := NewRepo(db)
	saved, err := r.Save(ctx, Theme{Name: "Mine", Colors: map[string]string{"background": "#fff", "foreground": "#000"}})
	if err != nil || saved.ID == "" || saved.Dark || saved.Colors["cursor"] != "#000000" {
		t.Fatalf("Save = %+v, %v", saved, err)
	}
	if _, err := r.Save(ctx, Theme{Name: "mine", Colors: map[string]string{"background": "#fff", "foreground": "#000"}}); errs.CodeOf(err) != ErrDuplicate {
		t.Errorf("duplicate: %v", err)
	}
	if _, err := r.Save(ctx, Theme{Name: "x", Colors: map[string]string{"background": "#fff"}}); errs.CodeOf(err) != ErrInvalid {
		t.Errorf("missing foreground: %v", err)
	}
	if n, _ := r.FreeName(ctx, "Mine"); n != "Mine (2)" {
		t.Errorf("FreeName = %q", n)
	}
	if err := r.Delete(ctx, saved.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.List(ctx); len(list) != 0 {
		t.Errorf("left: %+v", list)
	}
}
