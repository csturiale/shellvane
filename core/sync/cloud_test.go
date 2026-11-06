package sync

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCloudFolders(t *testing.T) {
	home := t.TempDir()
	mk := func(parts ...string) string {
		p := filepath.Join(append([]string{home}, parts...)...)
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
		return p
	}
	icloud := mk("Library", "Mobile Documents", "com~apple~CloudDocs")
	gd := mk("Library", "CloudStorage", "GoogleDrive-me@example.org", "Il mio Drive")
	mk("Library", "CloudStorage", "GoogleDrive-me@example.org", "Drive condivisi")
	mk("Library", "CloudStorage", "GoogleDrive-me@example.org", ".shortcut-targets-by-id")
	dropbox := mk("Library", "CloudStorage", "Dropbox")
	onedrive := mk("Library", "CloudStorage", "OneDrive-Personal")

	got := cloudFolders("darwin", home, func(string) string { return "" }, nil)
	want := []CloudFolder{{"icloud", icloud}, {"googleDrive", gd}, {"dropbox", dropbox}, {"oneDrive", onedrive}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("darwin =\n%v\nwant\n%v", got, want)
	}

	// Windows: Google Drive is a drive letter; OneDrive comes from the environment.
	win := t.TempDir()
	g := filepath.Join(t.TempDir(), "G")
	os.MkdirAll(filepath.Join(g, "My Drive"), 0o700)
	os.MkdirAll(filepath.Join(g, ".shortcut-targets-by-id"), 0o700)
	one := filepath.Join(win, "OneDrive - Acme")
	os.MkdirAll(one, 0o700)
	os.MkdirAll(filepath.Join(win, "iCloudDrive"), 0o700)
	got = cloudFolders("windows", win, func(k string) string {
		if k == "OneDriveCommercial" {
			return one
		}
		return ""
	}, []string{g})
	want = []CloudFolder{{"icloud", filepath.Join(win, "iCloudDrive")}, {"googleDrive", filepath.Join(g, "My Drive")}, {"oneDrive", one}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("windows =\n%v\nwant\n%v", got, want)
	}
	if got := cloudFolders("linux", t.TempDir(), func(string) string { return "" }, nil); len(got) != 0 {
		t.Fatalf("nothing installed: %v", got)
	}
}
