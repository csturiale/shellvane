// Package update checks for, downloads and installs new versions. The
// release publishes manifest.json and manifest.json.sig (an ed25519
// signature of the manifest's exact bytes); the manifest lists each
// platform's package with its SHA-256, so verifying the signature and the
// hash proves the package came from the release key. Nothing is installed
// without both checks.
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/version"
)

// Error codes.
const (
	ErrDisabled      errs.Code = "update.disabled"
	ErrFetch         errs.Code = "update.fetchFailed"
	ErrSignature     errs.Code = "update.badSignature"
	ErrNoPackage     errs.Code = "update.noPackage"
	ErrChecksum      errs.Code = "update.badChecksum"
	ErrInstall       errs.Code = "update.installFailed"
	ErrManualInstall errs.Code = "update.manual"
	// ErrNoRelease: no release with a manifest is published yet (GitHub's
	// "latest" skips pre-releases, and a private repository answers 404).
	ErrNoRelease    errs.Code = "update.noRelease"
	ErrTranslocated errs.Code = "update.translocated"
	ErrReadOnly     errs.Code = "update.readOnly"
)

// releaseKey is the release signing key's public half (base64 ed25519),
// made with "shellvane-release keygen". It is not secret. While the file is
// empty, builds never update.
//
//go:embed release.pub
var releaseKey string

// PublicKey verifies manifests.
var PublicKey = strings.TrimSpace(releaseKey)

// ManifestURL is the manifest of the latest full release of the public
// repository ("latest" never points at a pre-release); the signature is at
// ManifestURL + ".sig". Withdrawing a release on GitHub (or marking an
// earlier one as latest) makes the apps see the previous one again.
var ManifestURL = "https://github.com/" + version.PublicRepo + "/releases/latest/download/manifest.json"

// maxManifest and maxPackage bound downloads.
const (
	maxManifest = 1 << 20
	maxPackage  = 512 << 20
)

// Asset is a platform's package.
type Asset struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	// Kind is "binary" (the executable itself, replaced in place),
	// "bundle" (a zipped macOS .app that replaces the running one) or
	// "page" (installed by the user from Manifest.Page).
	Kind string `json:"kind"`
}

// Manifest describes a release.
type Manifest struct {
	Version string            `json:"version"`
	Date    string            `json:"date"`
	Notes   map[string]string `json:"notes"`  // language → text
	Page    string            `json:"page"`   // release page
	Assets  map[string]Asset  `json:"assets"` // "linux-amd64", "windows-amd64", "darwin-arm64"...
}

// Platform is this build's key in Manifest.Assets.
func Platform() string { return runtime.GOOS + "-" + runtime.GOARCH }

// Checker fetches and verifies manifests.
type Checker struct {
	Client    *http.Client
	URL       string
	PublicKey string
}

// NewChecker uses the compiled-in URL and key.
func NewChecker() *Checker {
	return &Checker{Client: http.DefaultClient, URL: ManifestURL, PublicKey: PublicKey}
}

// Verify tells whether sigText (base64, as shellvane-release sign writes it)
// is the project's signature of data. Also used for the plugin store index.
func Verify(data, sigText []byte) bool {
	key, err := base64.StdEncoding.DecodeString(PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	return err == nil && ed25519.Verify(ed25519.PublicKey(key), data, sig)
}

// Latest fetches the manifest and verifies its signature.
func (c *Checker) Latest(ctx context.Context) (Manifest, error) {
	if c.PublicKey == "" {
		return Manifest{}, errs.New(ErrDisabled, nil)
	}
	key, err := base64.StdEncoding.DecodeString(c.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return Manifest{}, errs.New(ErrDisabled, nil)
	}
	body, err := c.get(ctx, c.URL, maxManifest)
	var coded *errs.Error
	if errors.As(err, &coded) && coded.Code == ErrFetch && coded.Params["status"] == http.StatusNotFound {
		return Manifest{}, errs.New(ErrNoRelease, nil)
	}
	if err != nil {
		return Manifest{}, err
	}
	sigText, err := c.get(ctx, c.URL+".sig", 4096)
	if err != nil {
		return Manifest{}, err
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	if err != nil || !ed25519.Verify(ed25519.PublicKey(key), body, sig) {
		return Manifest{}, errs.New(ErrSignature, nil)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, errs.Wrap(err, ErrSignature, nil)
	}
	return m, nil
}

func (c *Checker) get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errs.Wrap(err, ErrFetch, nil)
	}
	req.Header.Set("User-Agent", version.UserAgent())
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, errs.Wrap(err, ErrFetch, nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errs.New(ErrFetch, errs.Params{"status": resp.StatusCode})
	}
	return io.ReadAll(io.LimitReader(resp.Body, limit))
}

// Download fetches a package into dir and checks its SHA-256 against the
// (signed) manifest. It returns the file's path.
func (c *Checker) Download(ctx context.Context, a Asset, dir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", errs.Wrap(err, ErrFetch, nil)
	}
	req.Header.Set("User-Agent", version.UserAgent())
	resp, err := c.Client.Do(req)
	if err != nil {
		return "", errs.Wrap(err, ErrFetch, nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errs.New(ErrFetch, errs.Params{"status": resp.StatusCode})
	}
	f, err := os.CreateTemp(dir, "shellvane-update-*")
	if err != nil {
		return "", errs.Wrap(err, ErrInstall, nil)
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, maxPackage))
	f.Close()
	if err != nil {
		os.Remove(f.Name())
		return "", errs.Wrap(err, ErrFetch, nil)
	}
	if !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), a.SHA256) {
		os.Remove(f.Name())
		return "", errs.New(ErrChecksum, nil)
	}
	return f.Name(), nil
}

// Install replaces the executable exe with the verified file: the old one
// is kept as exe+".old" (Windows cannot delete a running program) and
// removed by Cleanup at the next start.
func Install(file, exe string) error {
	if err := os.Chmod(file, 0o755); err != nil {
		return errs.Wrap(err, ErrInstall, nil)
	}
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return errs.Wrap(err, ErrInstall, nil)
	}
	if err := moveFile(file, exe); err != nil {
		os.Rename(old, exe) // put the working version back
		return errs.Wrap(err, ErrInstall, nil)
	}
	return nil
}

// moveFile renames, or copies when file is on another volume.
func moveFile(from, to string) error {
	if os.Rename(from, to) == nil {
		return nil
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(from)
}

// Cleanup removes the previous version left by Install.
func Cleanup(exe string) {
	os.Remove(exe + ".old")
}

// Newer tells whether version a is newer than b ("1.2.10" > "v1.2.9").
// Pre-release suffixes ("-rc1") sort before the release; "dev" is never
// newer and is older than everything.
func Newer(a, b string) bool {
	return compare(a, b) > 0
}

func compare(a, b string) int {
	pa, sa, okA := parse(a)
	pb, sb, okB := parse(b)
	switch {
	case !okA && !okB:
		return 0
	case !okA:
		return -1
	case !okB:
		return 1
	}
	for i := range 3 {
		if pa[i] != pb[i] {
			if pa[i] > pb[i] {
				return 1
			}
			return -1
		}
	}
	switch {
	case sa == sb:
		return 0
	case sa == "":
		return 1
	case sb == "":
		return -1
	case sa > sb:
		return 1
	}
	return -1
}

func parse(v string) ([3]int, string, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	core, suffix, _ := strings.Cut(v, "-")
	parts := strings.Split(core, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return out, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, "", false
		}
		out[i] = n
	}
	return out, suffix, true
}

// Sign signs a manifest with a base64 ed25519 private key (release tool).
func Sign(manifest []byte, privateKey string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKey))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("invalid private key")
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(key), manifest)), nil
}

// FileSHA256 returns the hex SHA-256 of a file (release tool).
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Executable is the running program, symlinks resolved. In an AppImage it
// is the AppImage file itself (the program runs from a read-only mount).
func Executable() (string, error) {
	if appImage := os.Getenv("APPIMAGE"); appImage != "" {
		return appImage, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return exe, nil
}
