package update

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
)

// AppBundle returns the macOS .app bundle containing exe
// (X.app/Contents/MacOS/x), or "" when exe is not in one.
func AppBundle(exe string) string {
	macos := filepath.Dir(exe)
	contents := filepath.Dir(macos)
	app := filepath.Dir(contents)
	if filepath.Base(macos) == "MacOS" && filepath.Base(contents) == "Contents" && strings.HasSuffix(app, ".app") {
		return app
	}
	return ""
}

// Translocated tells whether macOS runs the app from a read-only random
// copy (App Translocation, for apps started from where they were
// downloaded): it cannot update itself there.
func Translocated(app string) bool {
	return strings.Contains(app, "/AppTranslocation/")
}

// Writable tells whether files can be created next to path.
func Writable(path string) bool {
	f, err := os.CreateTemp(filepath.Dir(path), ".shellvane-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return true
}

// InstallBundle replaces the bundle app with the one inside the verified
// zip file: it is extracted next to app (same volume), checked, then the
// two are swapped by renaming. The old bundle is kept as app+".old" until
// CleanupBundle; any failure leaves the old bundle in place.
func InstallBundle(zipFile, app string) error {
	tmp, err := os.MkdirTemp(filepath.Dir(app), ".shellvane-update-*")
	if err != nil {
		return errs.Wrap(err, ErrInstall, nil)
	}
	defer os.RemoveAll(tmp)
	if err := unzip(zipFile, tmp); err != nil {
		return errs.Wrap(err, ErrInstall, nil)
	}
	newApp, err := findApp(tmp)
	if err != nil {
		return err
	}
	old := app + ".old"
	os.RemoveAll(old)
	if err := os.Rename(app, old); err != nil {
		return errs.Wrap(err, ErrInstall, nil)
	}
	if err := os.Rename(newApp, app); err != nil {
		os.Rename(old, app) // put the working version back
		return errs.Wrap(err, ErrInstall, nil)
	}
	return nil
}

// CleanupBundle removes the previous bundle left by InstallBundle.
func CleanupBundle(app string) {
	os.RemoveAll(app + ".old")
}

// findApp returns the single .app at the top of dir, checked to contain an
// executable and an Info.plist.
func findApp(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", errs.Wrap(err, ErrInstall, nil)
	}
	for _, e := range entries {
		if !e.IsDir() || !strings.HasSuffix(e.Name(), ".app") {
			continue
		}
		app := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(app, "Contents", "Info.plist")); err != nil {
			break
		}
		bins, err := os.ReadDir(filepath.Join(app, "Contents", "MacOS"))
		if err != nil || len(bins) == 0 {
			break
		}
		return app, nil
	}
	return "", errs.New(ErrInstall, errs.Params{"reason": "no app bundle in the package"})
}

// unzip extracts a zip into dir, keeping file modes and symbolic links and
// refusing paths that would leave dir.
func unzip(file, dir string) error {
	r, err := zip.OpenReader(file)
	if err != nil {
		return err
	}
	defer r.Close()
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, f := range r.File {
		target := filepath.Join(root, filepath.FromSlash(f.Name))
		if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
			return errs.New(ErrInstall, errs.Params{"reason": "unsafe path " + f.Name})
		}
		mode := f.Mode()
		switch {
		case f.FileInfo().IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			rc, err := f.Open()
			if err != nil {
				return err
			}
			link, err := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if err != nil {
				return err
			}
			// A link must stay inside the bundle too.
			dest := filepath.Join(filepath.Dir(target), string(link))
			if filepath.IsAbs(string(link)) || !strings.HasPrefix(dest, root+string(filepath.Separator)) {
				return errs.New(ErrInstall, errs.Params{"reason": "unsafe link " + f.Name})
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(string(link), target); err != nil {
				return err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			rc, err := f.Open()
			if err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm()|0o600)
			if err != nil {
				rc.Close()
				return err
			}
			_, err = io.Copy(out, io.LimitReader(rc, maxPackage))
			rc.Close()
			if cerr := out.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
