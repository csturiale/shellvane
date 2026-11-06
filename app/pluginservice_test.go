package app

import (
	"context"
	"errors"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/vault"
)

// tokenInventory needs a secret token and lists one host.
type tokenInventory struct{ plugins.Base }

func (tokenInventory) Manifest() plugins.Manifest {
	return plugins.Manifest{ID: "fake", Name: "Fake", Capabilities: []string{plugins.CapInventory}, Requires: []string{"no-such-tool-xyz"},
		Settings: []plugins.Setting{{Key: "url"}, {Key: "token", Secret: true}}}
}

func (tokenInventory) Inventory(_ context.Context, s map[string]string) ([]plugins.Host, error) {
	if s["token"] != "s3cret" || s["url"] != "https://x" {
		return nil, errors.New("bad settings")
	}
	return []plugins.Host{{Key: "a", Name: "alpha", Hostname: "10.0.0.1", Group: []string{"dc1"}}}, nil
}

func TestPluginServiceKeepsSecretsInTheVault(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	v := vault.New(db, vault.KDFParams{Time: 1, Memory: 1024, Threads: 1})
	if err := v.Setup(ctx, []byte("master-password")); err != nil {
		t.Fatal(err)
	}
	repo := hosts.New(db)
	svc := NewPluginService(plugins.NewRegistry(tokenInventory{}), plugins.NewConfigs(db), repo, v, "")

	list := svc.Plugins(ctx)
	if len(list) != 1 || len(list[0].Missing) != 1 {
		t.Fatalf("plugins = %+v", list)
	}
	if _, err := svc.SaveConfig(ctx, plugins.Config{PluginID: "fake", Name: "Early"}, nil); errs.CodeOf(err) != plugins.ErrNotInstalled {
		t.Fatalf("not installed: %v", err)
	}
	if err := svc.Install(ctx, "fake"); err != nil {
		t.Fatal(err)
	}
	cfg, err := svc.SaveConfig(ctx, plugins.Config{PluginID: "fake", Name: "Lab",
		Settings: map[string]string{"url": "https://x", "token": "leaked?"}}, map[string]string{"token": "s3cret"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Settings["token"]; ok || len(cfg.SecretsSet) != 1 {
		t.Fatalf("a secret reached the settings: %+v", cfg)
	}
	res, err := svc.Sync(ctx, cfg.ID)
	if err != nil || res.Added != 1 {
		t.Fatalf("sync = %+v, %v", res, err)
	}
	// Saving again without typing the token keeps it.
	if _, err := svc.SaveConfig(ctx, cfg.Config, map[string]string{"token": ""}); err != nil {
		t.Fatal(err)
	}
	if res, err := svc.Sync(ctx, cfg.ID); err != nil || res.Updated != 1 {
		t.Fatalf("second sync = %+v, %v", res, err)
	}
	v.Lock()
	if _, err := svc.Sync(ctx, cfg.ID); errs.CodeOf(err) != vault.ErrLocked {
		t.Fatalf("locked vault: %v", err)
	}
	if err := svc.Uninstall(ctx, "fake"); errs.CodeOf(err) != ErrPluginInUse {
		t.Fatalf("uninstall with a configuration: %v", err)
	}
	if err := svc.DeleteConfig(ctx, cfg.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Uninstall(ctx, "fake"); err != nil || svc.Plugins(ctx)[0].Installed {
		t.Fatalf("uninstall: %v", err)
	}
	if has, _ := v.HasCredential(ctx, plugins.SecretScope(cfg.ID, "token")); has {
		t.Fatal("the secret outlived its configuration")
	}
}
