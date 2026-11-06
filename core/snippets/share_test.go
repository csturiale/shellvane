package snippets

import (
	"context"
	"reflect"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
)

func TestParseShellFile(t *testing.T) {
	text := "#!/bin/bash\r\n# Disk usage\r\ndf -h\r\n\r\ndocker ps \\\r\n  --all\r\n## Tail the log ##\njournalctl -fu nginx\n"
	got, err := Parse(`C:\ops\server-cheats.sh`, text)
	if err != nil {
		t.Fatal(err)
	}
	want := []Input{
		{Folder: "server-cheats", Name: "Disk usage", Body: "df -h", Run: true},
		{Folder: "server-cheats", Name: "docker ps", Body: "docker ps \\\n  --all", Run: true},
		{Folder: "server-cheats", Name: "Tail the log", Body: "journalctl -fu nginx", Run: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse =\n%+v\nwant\n%+v", got, want)
	}
	if _, err := Parse("empty.txt", "# only a comment\n\n"); errs.CodeOf(err) != ErrImportInvalid {
		t.Errorf("empty file: %v", err)
	}
	if _, err := Parse("x.json", `{"kind":"other"}`); errs.CodeOf(err) != ErrImportInvalid {
		t.Errorf("foreign JSON: %v", err)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := New(db)
	if _, err := r.Create(ctx, Input{Folder: "k8s", Name: "pods", Body: "kubectl get pods -A", Run: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create(ctx, Input{Name: "type only", Body: "sudo -i"}); err != nil {
		t.Fatal(err)
	}
	list, _ := r.List(ctx)
	in, err := Parse("shellvane-snippets.json", Export(list))
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.Import(ctx, in)
	if err != nil || res.Added != 0 || res.Skipped != 2 {
		t.Fatalf("re-import = %+v, %v; want everything skipped", res, err)
	}
	in[1].Body = "sudo -s"
	if res, _ := r.Import(ctx, in); res.Added != 1 || res.Skipped != 1 {
		t.Fatalf("import = %+v", res)
	}
	if all, _ := r.List(ctx); len(all) != 3 {
		t.Fatalf("snippets = %d, want 3", len(all))
	}
}
