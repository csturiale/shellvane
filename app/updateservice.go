package app

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/update"
	"github.com/csturiale/shellvane/core/version"
)

// UpdateInfo is the result of a check.
type UpdateInfo struct {
	Current   string            `json:"current"`
	Latest    string            `json:"latest"`
	Available bool              `json:"available"`
	Notes     map[string]string `json:"notes"`
	Page      string            `json:"page"`
	// CanInstall is true when this platform's package installs in place;
	// otherwise the user downloads it from Page.
	CanInstall bool `json:"canInstall"`
	// Enabled is false in builds without the update key.
	Enabled bool `json:"enabled"`
	// NoRelease is true when no release is published yet: nothing to
	// update to, not an error.
	NoRelease bool `json:"noRelease"`
	// Blocked tells why an installable package cannot be installed here
	// ("notPublished": the release has no package for this system yet):
	// "translocated" (a macOS app run from where it was downloaded) or
	// "readOnly" (the app folder is not writable).
	Blocked string `json:"blocked"`
}

// UpdateService checks for new versions and installs them after verifying
// the release signature and the package checksum.
type UpdateService struct {
	checker *update.Checker
	latest  *update.Manifest
}

// NewUpdateService uses the compiled-in manifest URL and key; it removes
// the previous version left by an update.
func NewUpdateService() *UpdateService {
	if exe, err := update.Executable(); err == nil {
		update.Cleanup(exe)
		if app := update.AppBundle(exe); app != "" {
			update.CleanupBundle(app)
		}
	}
	return &UpdateService{checker: update.NewChecker()}
}

// Check fetches and verifies the latest manifest.
func (s *UpdateService) Check(ctx context.Context) (UpdateInfo, error) {
	info := UpdateInfo{Current: version.Version, Enabled: s.checker.PublicKey != ""}
	if !info.Enabled {
		return info, nil
	}
	m, err := s.checker.Latest(ctx)
	if errs.CodeOf(err) == update.ErrNoRelease {
		info.NoRelease = true
		return info, nil
	}
	if err != nil {
		return info, err
	}
	s.latest = &m
	info.Latest, info.Notes, info.Page = m.Version, m.Notes, m.Page
	info.Available = update.Newer(m.Version, version.Version)
	if a, ok := m.Assets[update.Platform()]; ok {
		_, info.Blocked, info.CanInstall = updateTarget(a)
	} else if info.Available {
		// The release is out but this system's package is not in it yet
		// (published a little after the others): the next check finds it.
		info.Blocked = "notPublished"
	}
	return info, nil
}

// Install downloads, verifies and installs the version found by Check;
// Restart then starts it.
func (s *UpdateService) Install(ctx context.Context) error {
	if s.latest == nil {
		return errs.New(update.ErrNoPackage, nil)
	}
	a, ok := s.latest.Assets[update.Platform()]
	if !ok {
		return errs.New(update.ErrNoPackage, nil)
	}
	dest, blocked, can := updateTarget(a)
	switch {
	case blocked == "translocated":
		return errs.New(update.ErrTranslocated, nil)
	case blocked == "readOnly":
		return errs.New(update.ErrReadOnly, nil)
	case !can:
		return errs.New(update.ErrManualInstall, errs.Params{"page": s.latest.Page})
	}
	// Download next to the target so the final rename stays on one volume.
	file, err := s.checker.Download(ctx, a, filepath.Dir(dest))
	if err != nil {
		return err
	}
	if a.Kind == "bundle" {
		err = update.InstallBundle(file, dest)
		os.Remove(file)
		return err
	}
	if err := update.Install(file, dest); err != nil {
		os.Remove(file)
		return err
	}
	return nil
}

// Restart starts the installed version and quits this one.
func (s *UpdateService) Restart() error {
	exe, err := update.Executable()
	if err != nil {
		return errs.Wrap(err, update.ErrInstall, nil)
	}
	// After a bundle swap the executable keeps its path inside the new .app.
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), RestartEnv+"=1")
	if err := cmd.Start(); err != nil {
		return errs.Wrap(err, update.ErrInstall, nil)
	}
	if a := application.Get(); a != nil {
		a.Quit()
	}
	return nil
}

// updateTarget returns what an asset replaces on this installation: the
// executable for a "binary", the .app for a "bundle". blocked names why it
// cannot be replaced ("translocated", "readOnly"); can is false for assets
// the user installs by hand ("page") or that do not fit how Shellvane runs.
func updateTarget(a update.Asset) (dest, blocked string, can bool) {
	exe, err := update.Executable()
	if err != nil {
		return "", "", false
	}
	app := update.AppBundle(exe)
	switch {
	case a.Kind == "binary" && app == "":
		dest = exe
	case a.Kind == "bundle" && app != "":
		if update.Translocated(app) {
			return app, "translocated", false
		}
		dest = app
	default:
		return "", "", false
	}
	if !update.Writable(dest) {
		return dest, "readOnly", false
	}
	return dest, "", true
}
