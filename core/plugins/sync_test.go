package plugins

import (
	"context"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/store"
)

func TestConfigsAndSync(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := hosts.New(db)
	configs := NewConfigs(db)

	if _, err := configs.Save(ctx, Config{PluginID: "aws-ec2"}); errs.CodeOf(err) != ErrNameRequired {
		t.Fatalf("no name: %v", err)
	}
	cfg, err := configs.Save(ctx, Config{PluginID: "aws-ec2", Name: "AWS prod", Settings: map[string]string{"region": "eu-west-1"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := configs.Save(ctx, Config{PluginID: "gcp-compute", Name: "aws PROD"}); errs.CodeOf(err) != ErrDuplicate {
		t.Fatalf("duplicate: %v", err)
	}

	first := []Host{
		{Key: "i-1", Name: "web", Hostname: "3.3.3.3", User: "ec2-user", Group: []string{"eu-west-1"}, Tags: []string{"env=prod"}},
		{Key: "i-2", Name: "db", Hostname: "10.0.0.2", Port: 2222, Group: []string{"eu-west-1", "data"}},
		{Key: "i-3", Name: "bad", Hostname: "has space"},
	}
	res, err := Sync(ctx, repo, configs, cfg, first)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 2 || res.Skipped != 1 || res.GroupID == "" {
		t.Fatalf("first sync = %+v", res)
	}
	cfg, _ = configs.Get(ctx, cfg.ID)
	if cfg.GroupID != res.GroupID || cfg.LastSync == "" || cfg.Settings["region"] != "eu-west-1" {
		t.Fatalf("config = %+v", cfg)
	}
	known, _ := configs.Hosts(ctx, cfg.ID)
	web, _ := repo.Host(ctx, known["i-1"])
	if web.Hostname != "3.3.3.3" || *web.Settings.User != "ec2-user" {
		t.Fatalf("web = %+v", web)
	}
	// The user marks it favorite and sets a key: a refresh keeps them.
	key := "key-1"
	web.Settings.IdentityID = &key
	repo.UpdateHost(ctx, web.ID, hosts.HostInput{GroupID: web.GroupID, Name: web.Name, Hostname: web.Hostname, Settings: web.Settings, Tags: web.Tags, Favorite: true})

	second := []Host{{Key: "i-1", Name: "web-renamed", Hostname: "3.3.3.4", Group: []string{"eu-west-1"}}}
	res, err = Sync(ctx, repo, configs, cfg, second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Updated != 1 || res.Removed != 1 || res.Added != 0 {
		t.Fatalf("second sync = %+v", res)
	}
	web, _ = repo.Host(ctx, web.ID)
	if web.Name != "web-renamed" || web.Hostname != "3.3.3.4" || !web.Favorite || *web.Settings.IdentityID != "key-1" {
		t.Fatalf("web after refresh = %+v", web)
	}
	inv, _ := repo.List(ctx)
	names := map[string]bool{}
	for _, g := range inv.Groups {
		names[g.Name] = true
	}
	if names["data"] || !names["eu-west-1"] || !names["AWS prod"] {
		t.Fatalf("groups = %v (empty 'data' must be pruned)", names)
	}

	if err := configs.Delete(ctx, cfg.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := configs.List(ctx); len(list) != 0 {
		t.Fatalf("list after delete = %v", list)
	}
	if _, err := repo.Host(ctx, web.ID); err != nil {
		t.Fatal("deleting the configuration must keep its hosts")
	}
}
