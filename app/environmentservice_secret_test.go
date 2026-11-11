package app

import (
	"context"
	"testing"

	"github.com/csturiale/shellvane/core/environments"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/store"
)

// The login secret is asked to the plugin configuration with the
// environment's address, user and safe, variables expanded; an account
// without a password is an error, and nothing is copied outside the app.
func TestCopyLoginSecretAsksThePlugin(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := environments.New(db)
	e, err := repo.Save(ctx, environments.Environment{Name: "prod", Login: environments.LocalLogin{
		Login:  "gcloud auth login --update-adc",
		Secret: environments.LoginSecret{Source: "cfg-1", Address: "accounts.google.com", User: "{{who}}@example.com", Safe: "GCP"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var got plugins.Target
	var ref string
	sessions := &SessionService{}
	EnableBrokers(sessions, func(_ context.Context, r string, t plugins.Target) (plugins.Credential, error) {
		ref, got = r, t
		return plugins.Credential{User: t.User}, nil
	})
	s := NewEnvironmentService(repo, nil, nil, nil)
	EnableEnvironmentLogin(s, sessions, plugins.NewConfigs(db), t.TempDir())

	_, err = s.CopyLoginSecret(ctx, e.ID, map[string]string{"who": "alice-admin"}, 0)
	if errs.CodeOf(err) != ErrLoginSecretEmpty {
		t.Fatalf("an account without a password: %v", err)
	}
	if ref != "cfg-1" || got.Hostname != "accounts.google.com" || got.User != "alice-admin@example.com" || got.Safe != "GCP" || got.Purpose != "login of environment prod" {
		t.Fatalf("asked %q %+v", ref, got)
	}

	// Without a source there is nothing to copy.
	e.Login.Secret = environments.LoginSecret{}
	if _, err := repo.Save(ctx, e); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CopyLoginSecret(ctx, e.ID, nil, 30); errs.CodeOf(err) != ErrNoLoginSecret {
		t.Fatalf("no source: %v", err)
	}
}
