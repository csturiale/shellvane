package hosts

import (
	"context"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/settings"
)

func TestTemplateRoundTrip(t *testing.T) {
	ctx := context.Background()
	src := newRepo(t)
	aws, err := src.CreateGroup(ctx, GroupInput{Name: "AWS", Color: "#ff9900", Settings: settings.Settings{
		User: ptr("ec2-user"), ProxyCommand: ptr("aws ssm start-session --target %h"),
		IdentityID: ptr("key-1"), KnockSequence: ptr("7000,8000"), JumpChain: []string{"h1"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	prod, err := src.CreateGroup(ctx, GroupInput{ParentID: aws.ID, Name: "prod", Settings: settings.Settings{Port: ptr(2222)}})
	if err != nil {
		t.Fatal(err)
	}
	mustGroup(t, src, "", "Home")
	if _, err := src.CreateHost(ctx, HostInput{GroupID: prod.ID, Name: "web", Hostname: "10.0.0.1"}); err != nil {
		t.Fatal(err)
	}
	inv, _ := src.List(ctx)

	tpl, err := ExportTemplate(inv, aws.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	text := MarshalTemplate(tpl)
	for _, leak := range []string{"key-1", "7000", "h1", "10.0.0.1", "web", "Home"} {
		if strings.Contains(text, leak) {
			t.Errorf("template contains %q:\n%s", leak, text)
		}
	}

	dst := newRepo(t)
	existing := mustGroup(t, dst, "", "aws") // same name: reused, settings kept
	parsed, err := ParseTemplate(text)
	if err != nil {
		t.Fatal(err)
	}
	res, err := dst.ImportTemplate(ctx, "", parsed)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 || res.Reused != 1 {
		t.Fatalf("result = %+v, want 1 created and 1 reused", res)
	}
	got, _ := dst.List(ctx)
	if len(got.Groups) != 2 || len(got.Hosts) != 0 {
		t.Fatalf("groups = %+v, hosts = %d", got.Groups, len(got.Hosts))
	}
	for _, g := range got.Groups {
		switch g.Name {
		case "aws":
			if g.Settings.User != nil {
				t.Error("a reused group must keep its settings")
			}
		case "prod":
			if g.ParentID != existing.ID || g.Settings.Port == nil || *g.Settings.Port != 2222 {
				t.Errorf("prod = %+v", g)
			}
		}
	}

	// Into a fresh place everything is created, settings included.
	fresh := newRepo(t)
	if res, err := fresh.ImportTemplate(ctx, "", parsed); err != nil || res.Created != 2 {
		t.Fatalf("import = %+v, %v", res, err)
	}
	inv2, _ := fresh.List(ctx)
	for _, g := range inv2.Groups {
		if g.Name == "AWS" && (g.Color != "#ff9900" || *g.Settings.User != "ec2-user" || g.Settings.IdentityID != nil) {
			t.Errorf("AWS = %+v", g)
		}
	}

	all, _ := ExportTemplate(inv, "", false)
	if len(all.Groups) != 2 {
		t.Errorf("full export has %d top groups, want 2", len(all.Groups))
	}
}

func TestParseTemplateRejectsOtherText(t *testing.T) {
	for _, s := range []string{"", "{}", `{"kind":"shellvane-group-template","version":2,"groups":[{"name":"a"}]}`,
		`{"kind":"shellvane-group-template","version":1,"groups":[{"name":" "}]}`, "name: x"} {
		if _, err := ParseTemplate(s); errs.CodeOf(err) != ErrTemplateInvalid {
			t.Errorf("ParseTemplate(%q) = %v", s, err)
		}
	}
	if _, err := ExportTemplate(Inventory{}, "missing", false); errs.CodeOf(err) != ErrGroupNotFound {
		t.Errorf("export of a missing group = %v", err)
	}
}

func TestExportWithHosts(t *testing.T) {
	ctx := context.Background()
	src := newRepo(t)
	dc := mustGroup(t, src, "", "DC")
	bastion, err := src.CreateHost(ctx, HostInput{GroupID: dc.ID, Name: "bastion", Hostname: "bastion.example.org", Notes: "root pw is hunter2", Tags: []string{"edge"},
		Settings: settings.Settings{IdentityID: ptr("key-1"), User: ptr("ops")}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.CreateHost(ctx, HostInput{GroupID: dc.ID, Name: "db", Hostname: "10.0.0.5", Settings: settings.Settings{JumpChain: []string{bastion.ID}}}); err != nil {
		t.Fatal(err)
	}
	inv, _ := src.List(ctx)
	tpl, err := ExportTemplate(inv, dc.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	text := MarshalTemplate(tpl)
	for _, leak := range []string{"hunter2", "key-1", bastion.ID} {
		if strings.Contains(text, leak) {
			t.Errorf("export contains %q", leak)
		}
	}
	parsed, err := ParseTemplate(text)
	if err != nil {
		t.Fatal(err)
	}

	dst := newRepo(t)
	res, err := dst.ImportTemplate(ctx, "", parsed)
	if err != nil || res.Created != 1 || res.HostsCreated != 2 {
		t.Fatalf("import = %+v, %v", res, err)
	}
	got, _ := dst.List(ctx)
	ids := map[string]Host{}
	for _, h := range got.Hosts {
		ids[h.Name] = h
	}
	db, b := ids["db"], ids["bastion"]
	if len(db.Settings.JumpChain) != 1 || db.Settings.JumpChain[0] != b.ID {
		t.Errorf("db jump chain = %v, want [%s]", db.Settings.JumpChain, b.ID)
	}
	if b.Notes != "" || b.Settings.IdentityID != nil || *b.Settings.User != "ops" || len(b.Tags) != 1 {
		t.Errorf("bastion = %+v", b)
	}
	// Importing again adds nothing.
	if res, err := dst.ImportTemplate(ctx, "", parsed); err != nil || res.HostsCreated != 0 || res.HostsSkipped != 2 || res.Reused != 1 {
		t.Fatalf("second import = %+v, %v", res, err)
	}
}
