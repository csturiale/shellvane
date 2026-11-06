package app

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/runbooks"
	"github.com/csturiale/shellvane/core/store"
)

func TestRunbookTargets(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := hosts.New(db)
	prod, _ := repo.CreateGroup(ctx, hosts.GroupInput{Name: "prod"})
	eu, _ := repo.CreateGroup(ctx, hosts.GroupInput{Name: "eu", ParentID: prod.ID})
	repo.CreateHost(ctx, hosts.HostInput{Name: "web-eu", Hostname: "a", GroupID: eu.ID})
	repo.CreateHost(ctx, hosts.HostInput{Name: "db", Hostname: "b", Tags: []string{"Database"}})
	lone, _ := repo.CreateHost(ctx, hosts.HostInput{Name: "lone", Hostname: "c"})
	repo.CreateHost(ctx, hosts.HostInput{Name: "other", Hostname: "d"})

	svc := NewRunbookService(runbooks.NewRepo(db), repo, nil, nil, func(string, any) {})
	got, err := svc.Targets(ctx, runbooks.Targets{GroupIDs: []string{prod.ID}, Tags: []string{"database"}, HostIDs: []string{lone.ID}})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, h := range got {
		names = append(names, h.Name)
	}
	sort.Strings(names)
	if strings.Join(names, ",") != "db,lone,web-eu" {
		t.Fatalf("targets %v", names)
	}
	if _, err := svc.Start(ctx, runbooks.Runbook{Name: "x", Steps: []runbooks.Step{{Command: "true"}}}, nil); err == nil {
		t.Fatal("a runbook without hosts must not start")
	}
}
