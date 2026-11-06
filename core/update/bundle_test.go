package update

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

// makeZip writes a zip with the given files (name → content); a content
// starting with "->" is a symbolic link.
func makeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, body := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if len(body) > 2 && body[:2] == "->" {
			h.SetMode(os.ModeSymlink | 0o777)
			body = body[2:]
		} else {
			h.SetMode(0o755)
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		fw.Write([]byte(body))
	}
	w.Close()
	f.Close()
}

func TestInstallBundle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("macOS bundles")
	}
	dir := t.TempDir()
	app := filepath.Join(dir, "Shellvane.app")
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "MacOS", "shellvane"), []byte("old"), 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "Info.plist"), []byte("old"), 0o644)

	pkg := filepath.Join(dir, "update.zip")
	makeZip(t, pkg, map[string]string{
		"shellvane.app/Contents/Info.plist":        "new plist",
		"shellvane.app/Contents/MacOS/shellvane":   "new binary",
		"shellvane.app/Contents/Resources/current": "->../MacOS/shellvane",
	})
	if err := InstallBundle(pkg, app); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(app, "Contents", "MacOS", "shellvane")
	if b, _ := os.ReadFile(bin); string(b) != "new binary" {
		t.Fatalf("binary = %q", b)
	}
	if fi, _ := os.Stat(bin); fi.Mode().Perm()&0o100 == 0 {
		t.Fatal("binary is not executable")
	}
	if link, _ := os.Readlink(filepath.Join(app, "Contents", "Resources", "current")); link != "../MacOS/shellvane" {
		t.Fatalf("link = %q", link)
	}
	if b, _ := os.ReadFile(filepath.Join(app+".old", "Contents", "MacOS", "shellvane")); string(b) != "old" {
		t.Fatal("old bundle not kept")
	}
	CleanupBundle(app)
	if _, err := os.Stat(app + ".old"); !os.IsNotExist(err) {
		t.Fatal("old bundle not removed")
	}
	if AppBundle(bin) != app || AppBundle("/usr/local/bin/shellvane") != "" {
		t.Fatal("AppBundle")
	}
	if !Translocated("/private/var/folders/x/AppTranslocation/ABC/d/Shellvane.app") || Translocated(app) {
		t.Fatal("Translocated")
	}
}

func TestInstallBundleRefusesBadPackages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("macOS bundles")
	}
	dir := t.TempDir()
	app := filepath.Join(dir, "Shellvane.app")
	os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755)
	os.WriteFile(filepath.Join(app, "Contents", "MacOS", "shellvane"), []byte("old"), 0o755)
	for name, files := range map[string]map[string]string{
		"slip.zip":     {"../evil": "x"},
		"link.zip":     {"t.app/Contents/Info.plist": "p", "t.app/Contents/MacOS/t": "b", "t.app/x": "->/etc/passwd"},
		"nobundle.zip": {"readme.txt": "hi"},
	} {
		pkg := filepath.Join(dir, name)
		makeZip(t, pkg, files)
		if err := InstallBundle(pkg, app); errs.CodeOf(err) != ErrInstall {
			t.Errorf("%s: err = %v", name, err)
		}
		if b, _ := os.ReadFile(filepath.Join(app, "Contents", "MacOS", "shellvane")); string(b) != "old" {
			t.Fatalf("%s: old bundle damaged", name)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); err == nil {
		t.Fatal("zip slip wrote outside")
	}
}
