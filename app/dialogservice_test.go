package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStartDir(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "client.pem")
	os.WriteFile(file, []byte("x"), 0o600)
	home, _ := os.UserHomeDir()
	for in, want := range map[string]string{
		"":                               "",
		dir:                              dir,
		file:                             dir, // a file: its folder
		filepath.Join(dir, "new-folder"): dir, // not there yet: its parent
		filepath.Join(dir, "a", "b"):     "",  // nothing near it exists
		"~":                              home,
	} {
		if got := startDir(in); got != want {
			t.Errorf("startDir(%q) = %q, want %q", in, got, want)
		}
	}
}
