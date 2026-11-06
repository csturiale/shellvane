package plugins

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/update"
	"github.com/csturiale/shellvane/core/version"
)

// Error codes of packages and of the store.
const (
	ErrPackage  errs.Code = "plugins.package"
	ErrStore    errs.Code = "plugins.store"
	ErrChecksum errs.Code = "plugins.checksum"
	ErrBuiltIn  errs.Code = "plugins.builtIn" // an external plugin may not take a built-in's ID
)

// Limits of a package.
const (
	maxPackage = 64 << 20  // download or zip size
	maxFiles   = 2000      // entries in a zip
	maxUnzip   = 256 << 20 // total extracted size
)

// Install copies a plugin (a folder or a .zip holding plugin.json, at its
// root or in one folder) into root/<id>, replacing an older version, and
// returns its manifest.
func Install(src, root string) (Manifest, error) {
	tmp, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		if err := os.MkdirAll(root, 0o755); err != nil {
			return Manifest{}, err
		}
		if tmp, err = os.MkdirTemp(root, ".install-"); err != nil {
			return Manifest{}, err
		}
	}
	defer os.RemoveAll(tmp)
	if strings.EqualFold(filepath.Ext(src), ".zip") {
		err = unzip(src, tmp)
	} else {
		err = copyDir(src, tmp)
	}
	if err != nil {
		return Manifest{}, errs.Wrap(err, ErrPackage, errs.Params{"detail": err.Error()})
	}
	dir := tmp
	if _, err := os.Stat(filepath.Join(dir, ManifestFile)); err != nil {
		// A single top-level folder (how zips are usually made).
		entries, _ := os.ReadDir(dir)
		var dirs []string
		for _, e := range entries {
			if e.IsDir() && e.Name() != "__MACOSX" {
				dirs = append(dirs, e.Name())
			}
		}
		if len(dirs) == 1 {
			dir = filepath.Join(dir, dirs[0])
		}
	}
	m, err := ReadManifest(dir)
	if err != nil {
		return Manifest{}, err
	}
	dest := filepath.Join(root, m.ID)
	old := filepath.Join(root, "."+m.ID+".old")
	os.RemoveAll(old)
	if _, err := os.Stat(dest); err == nil {
		if err := os.Rename(dest, old); err != nil {
			return Manifest{}, err
		}
	}
	if err := os.Rename(dir, dest); err != nil {
		os.Rename(old, dest)
		return Manifest{}, err
	}
	os.RemoveAll(old)
	return m, nil
}

// ReadManifestOf reads the manifest of a folder or .zip without installing
// it (to check its ID first).
func ReadManifestOf(src string) (Manifest, error) {
	if !strings.EqualFold(filepath.Ext(src), ".zip") {
		return ReadManifest(src)
	}
	tmp, err := os.MkdirTemp("", "shellvane-plugin-")
	if err != nil {
		return Manifest{}, err
	}
	defer os.RemoveAll(tmp)
	if err := unzip(src, tmp); err != nil {
		return Manifest{}, errs.Wrap(err, ErrPackage, errs.Params{"detail": err.Error()})
	}
	if m, err := ReadManifest(tmp); err == nil {
		return m, nil
	}
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if e.IsDir() && e.Name() != "__MACOSX" {
			return ReadManifest(filepath.Join(tmp, e.Name()))
		}
	}
	return ReadManifest(tmp)
}

// Uninstall removes root/<id>.
func Uninstall(root, id string) error {
	if !validID.MatchString(id) {
		return errs.New(ErrManifest, errs.Params{"detail": "id"})
	}
	return os.RemoveAll(filepath.Join(root, id))
}

func unzip(src, dest string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	if len(zr.File) > maxFiles {
		return errs.New(ErrPackage, errs.Params{"detail": "too many files"})
	}
	var total int64
	for _, f := range zr.File {
		name := path.Clean(strings.ReplaceAll(f.Name, `\`, "/"))
		if name == "." || strings.HasPrefix(name, "../") || path.IsAbs(name) || strings.Contains(name, "/../") {
			return errs.New(ErrPackage, errs.Params{"detail": "unsafe path " + f.Name})
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if f.Mode()&fs.ModeSymlink != 0 {
			continue // links could point outside the folder
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if f.Mode()&0o111 != 0 {
			mode = 0o755
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, maxUnzip-total+1))
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
		if total += n; total > maxUnzip {
			return errs.New(ErrPackage, errs.Params{"detail": "too large"})
		}
	}
	return nil
}

func copyDir(src, dest string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// StoreEntry is a plugin listed by a store index.
type StoreEntry struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	Homepage     string   `json:"homepage"`
	Capabilities []string `json:"capabilities"`
	Requires     []string `json:"requires"`
	// URL of the .zip and its SHA-256, which the download must match.
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// StoreIndex is the JSON a store publishes.
type StoreIndex struct {
	Plugins []StoreEntry `json:"plugins"`
}

var storeClient = &http.Client{Timeout: 60 * time.Second}

// OfficialStore is Shellvane's plugin store: its index must carry the
// project's signature (index.json.sig), so whoever controls the website
// alone cannot make the app install other packages.
const OfficialStore = version.SiteURL + "/plugins/index.json"

// ErrStoreSignature: the official store's index is not signed by the project.
const ErrStoreSignature errs.Code = "plugins.storeSignature"

var officialStore = OfficialStore

// verifyIndex checks an index's signature; other stores, added by the
// person, may be unsigned.
var verifyIndex = update.Verify

// FetchIndex reads a store index (https only, except for localhost).
func FetchIndex(ctx context.Context, url string) (StoreIndex, error) {
	body, err := fetch(ctx, url, 4<<20)
	if err != nil {
		return StoreIndex{}, err
	}
	if strings.TrimRight(url, "/") == officialStore {
		sig, err := fetch(ctx, url+".sig", 4096)
		if err != nil || !verifyIndex(body, sig) {
			return StoreIndex{}, errs.New(ErrStoreSignature, nil)
		}
	}
	var idx StoreIndex
	if err := json.Unmarshal(body, &idx); err != nil {
		return StoreIndex{}, errs.Wrap(err, ErrStore, errs.Params{"detail": err.Error()})
	}
	return idx, nil
}

// Download fetches a store package into a temporary .zip, checking its
// SHA-256; the caller removes the file.
func Download(ctx context.Context, e StoreEntry) (string, error) {
	if len(e.SHA256) != 64 {
		return "", errs.New(ErrChecksum, errs.Params{"id": e.ID})
	}
	body, err := fetch(ctx, e.URL, maxPackage)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), e.SHA256) {
		return "", errs.New(ErrChecksum, errs.Params{"id": e.ID})
	}
	f, err := os.CreateTemp("", "shellvane-plugin-*.zip")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(body); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func fetch(ctx context.Context, url string, limit int64) ([]byte, error) {
	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://127.0.0.1") && !strings.HasPrefix(url, "http://localhost") {
		return nil, errs.New(ErrStore, errs.Params{"detail": "https required"})
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errs.Wrap(err, ErrStore, errs.Params{"detail": err.Error()})
	}
	req.Header.Set("User-Agent", version.UserAgent())
	resp, err := storeClient.Do(req)
	if err != nil {
		return nil, errs.Wrap(err, ErrStore, errs.Params{"detail": err.Error()})
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errs.New(ErrStore, errs.Params{"detail": resp.Status})
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, errs.Wrap(err, ErrStore, errs.Params{"detail": err.Error()})
	}
	if int64(len(body)) > limit {
		return nil, errs.New(ErrStore, errs.Params{"detail": "too large"})
	}
	return body, nil
}
