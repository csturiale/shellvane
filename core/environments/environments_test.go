package environments

import (
	"context"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
)

func TestRepoAndYAML(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := New(db)
	e, err := r.Save(ctx, Environment{
		Name:      " GKE prod ",
		Variables: []Variable{{Name: "project", Default: "acme-prod"}},
		Login:     LocalLogin{Check: "gcloud auth print-access-token", Login: "gcloud auth login"},
		Remote:    Remote{HostID: "h1", Terminal: true, Script: []string{"gcloud config set project {{project}}"}},
		TunnelIDs: []string{"t1"},
		LocalKube: LocalKube{Context: "gke_prod", TunnelID: "t1", Terminals: 1},
	})
	if err != nil || e.ID == "" || e.Name != "GKE prod" {
		t.Fatalf("save = %+v, %v", e, err)
	}
	if _, err := r.Save(ctx, Environment{Name: "gke PROD"}); errs.CodeOf(err) != ErrDuplicate {
		t.Fatalf("duplicate: %v", err)
	}
	got, _ := r.Get(ctx, e.ID)
	if got.Remote.Script[0] != "gcloud config set project {{project}}" || got.LocalKube.TunnelID != "t1" {
		t.Fatalf("get = %+v", got)
	}

	names := map[string]string{"h1": "bastion", "t1": "gke-socks"}
	text, err := Export(got, func(_, id string) string { return names[id] })
	if err != nil || !strings.Contains(text, "host: bastion") || !strings.Contains(text, "- gke-socks") {
		t.Fatalf("yaml:\n%s", text)
	}
	ids := map[string]string{"bastion": "h9", "gke-socks": "t9"}
	in, err := Import(text, func(_, n string) string { return ids[n] })
	if err != nil || in.ID != "" || in.Remote.HostID != "h9" || in.TunnelIDs[0] != "t9" || in.LocalKube.TunnelID != "t9" {
		t.Fatalf("import = %+v, %v", in, err)
	}
	if err := r.Delete(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := Import("name: [", nil); errs.CodeOf(err) != ErrInvalid {
		t.Fatal("bad yaml accepted")
	}
}

func TestExpand(t *testing.T) {
	got := Expand("gcloud --project {{project}} {{ zone }} {{other}}", map[string]string{"project": "p1", "zone": "z1"})
	if got != "gcloud --project p1 z1 {{other}}" {
		t.Fatal(got)
	}
}

// The login secret's plugin configuration travels by name, the gcloud
// profile as it is; the profile is cleaned for use as a folder name.
func TestLoginSecretAndGcloudProfile(t *testing.T) {
	e := normalize(Environment{Name: "prod", Gcloud: " ../prod admin ",
		Login: LocalLogin{Secret: LoginSecret{Source: "cfg-1", Address: "accounts.google.com", User: "alice@example.com"}}})
	if e.Gcloud != "prod-admin" {
		t.Fatalf("profile = %q", e.Gcloud)
	}
	text, err := Export(e, func(kind, id string) string {
		if kind == "plugin" && id == "cfg-1" {
			return "CyberArk prod"
		}
		return ""
	})
	if err != nil || !strings.Contains(text, "source: CyberArk prod") || !strings.Contains(text, "gcloud: prod-admin") {
		t.Fatalf("yaml:\n%s", text)
	}
	in, err := Import(text, func(kind, name string) string {
		if kind == "plugin" && name == "CyberArk prod" {
			return "cfg-9"
		}
		return ""
	})
	if err != nil || in.Login.Secret.Source != "cfg-9" || in.Login.Secret.User != "alice@example.com" || in.Gcloud != "prod-admin" {
		t.Fatalf("import = %+v, %v", in, err)
	}
	for in, want := range map[string]string{"": "", "dev": "dev", "a/b": "a-b", "...": ""} {
		if got := GcloudProfile(in); got != want {
			t.Errorf("GcloudProfile(%q) = %q, want %q", in, got, want)
		}
	}
}
