package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/diagnostics"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/store"
)

func TestDiagnosticsReportAndBundle(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "shellvane.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	repo := hosts.New(db)
	svc := NewSessionService(repo, nil, nil, nil, func(string, any) {})
	svc.handshakes.Add(diagnostics.Handshake{ID: "a", Name: "prod-db", Address: "10.1.2.3:22", User: "admin", Started: time.Now(),
		Events: []sshengine.TraceEvent{{Step: sshengine.StepDial, Detail: map[string]string{"address": "10.1.2.3:22", "user": "admin"}}}})
	d := NewDiagnosticsService(svc, repo, t.TempDir())
	report, err := d.HandshakeReport("a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(report, "prod-db") || strings.Contains(report, "admin") || strings.Contains(report, "10.1.2.3") {
		t.Fatalf("report not anonymized:\n%s", report)
	}
	if _, err := d.HandshakeReport("missing"); err == nil {
		t.Fatal("expected an error")
	}
	path, err := d.SaveBundle(ctx, map[string]string{"theme": "dark"})
	if err != nil || !strings.HasSuffix(path, ".zip") {
		t.Fatalf("SaveBundle = %q, %v", path, err)
	}
}
