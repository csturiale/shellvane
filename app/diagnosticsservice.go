package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/anonymize"
	"github.com/csturiale/shellvane/core/diagnostics"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/version"
)

// Error codes of the diagnostics service.
const (
	ErrHandshakeNotFound errs.Code = "diagnostics.notFound"
	ErrBundleFailed      errs.Code = "diagnostics.bundleFailed"
)

// DiagnosticsService shows how connections were set up and builds the
// anonymized support bundle.
type DiagnosticsService struct {
	sessions *SessionService
	hosts    *hosts.Repo
	dir      string // where bundles are saved
}

// NewDiagnosticsService saves bundles in dataDir/support.
func NewDiagnosticsService(sessions *SessionService, repo *hosts.Repo, dataDir string) *DiagnosticsService {
	return &DiagnosticsService{sessions: sessions, hosts: repo, dir: filepath.Join(dataDir, "support")}
}

// Handshakes returns the recent connection attempts, newest first.
func (s *DiagnosticsService) Handshakes() []diagnostics.Handshake {
	return s.sessions.handshakes.List()
}

// HandshakeReport is one attempt as anonymized text, to copy for support.
func (s *DiagnosticsService) HandshakeReport(id string) (string, error) {
	h, ok := s.sessions.handshakes.Get(id)
	if !ok {
		return "", errs.New(ErrHandshakeNotFound, errs.Params{"id": id})
	}
	return diagnostics.Format(h, anonymize.New()), nil
}

// BundlePreview returns the files the bundle would contain. preferences
// are the UI settings.
func (s *DiagnosticsService) BundlePreview(ctx context.Context, preferences map[string]string) ([]diagnostics.File, error) {
	inv, err := s.hosts.List(ctx)
	if err != nil {
		return nil, err
	}
	return diagnostics.Bundle(diagnostics.BundleInput{
		Version: version.Version, Commit: version.Commit, OS: runtime.GOOS, Arch: runtime.GOARCH,
		Inventory: inv, Preferences: preferences, Handshakes: s.sessions.handshakes.List(),
		Counts: map[string]int{"groups": len(inv.Groups), "hosts": len(inv.Hosts), "sessions": s.sessions.count()},
	}, anonymize.New()), nil
}

// SaveBundle writes the bundle as a ZIP file and returns its path.
func (s *DiagnosticsService) SaveBundle(ctx context.Context, preferences map[string]string) (string, error) {
	files, err := s.BundlePreview(ctx, preferences)
	if err != nil {
		return "", err
	}
	data, err := diagnostics.Zip(files)
	if err != nil {
		return "", errs.Wrap(err, ErrBundleFailed, nil)
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return "", errs.Wrap(err, ErrBundleFailed, nil)
	}
	path := filepath.Join(s.dir, "shellvane-support-"+time.Now().Format("20060102-150405")+".zip")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", errs.Wrap(err, ErrBundleFailed, nil)
	}
	return path, nil
}

// RevealBundle shows a saved bundle in the system's file manager.
func (s *DiagnosticsService) RevealBundle(path string) error {
	app := application.Get()
	if app == nil || filepath.Dir(path) != s.dir {
		return errs.New(ErrBundleFailed, nil)
	}
	return app.Env.OpenFileManager(path, true)
}
