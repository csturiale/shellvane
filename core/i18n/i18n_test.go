package i18n

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"
)

func TestTranslate(t *testing.T) {
	it := New("it-IT")
	if got := it.Language().String(); got != "it" {
		t.Fatalf("language = %q, want it", got)
	}
	if got := it.T("menu.quit", nil); got != "Esci" {
		t.Errorf("menu.quit = %q, want Esci", got)
	}
	if got := it.N("tray.activeTunnels", 1, nil); got != "1 tunnel attivo" {
		t.Errorf("plural one = %q", got)
	}
	if got := it.N("tray.activeTunnels", 3, nil); got != "3 tunnel attivi" {
		t.Errorf("plural other = %q", got)
	}
}

func TestFallbacks(t *testing.T) {
	if got := New("ja-JP").Language().String(); got != "en" {
		t.Errorf("unsupported language should fall back to en, got %q", got)
	}
	if got := New().T("menu.quit", nil); got != "Quit" {
		t.Errorf("no preference should use English, got %q", got)
	}
	if got := New("en").T("does.not.exist", nil); got != "does.not.exist" {
		t.Errorf("unknown id should render as itself, got %q", got)
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "it_IT.UTF-8")
	if got := FromEnv(); got != "it-IT" {
		t.Errorf("FromEnv = %q, want it-IT", got)
	}
	t.Setenv("LC_ALL", "C")
	t.Setenv("LANG", "")
	if got := FromEnv(); got != "" {
		t.Errorf("FromEnv with C locale = %q, want empty", got)
	}
}

// Every locale must define exactly the keys of the source locale.
func TestLocalesHaveSameKeys(t *testing.T) {
	files, err := localeFiles()
	if err != nil {
		t.Fatal(err)
	}
	keysOf := func(name string) []string {
		data, err := localeFS.ReadFile("locales/" + name)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return slices.Sorted(maps.Keys(m))
	}

	source := keysOf(Source.String() + ".json")
	for _, f := range files {
		if keys := keysOf(f); !slices.Equal(keys, source) {
			t.Errorf("%s keys differ from source:\n got %v\nwant %v", f, keys, source)
		}
	}
}
