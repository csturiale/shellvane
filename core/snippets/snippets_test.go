package snippets

import (
	"context"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
)

func TestCRUD(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := New(db)

	if _, err := r.Create(ctx, Input{Name: " ", Body: "x"}); errs.CodeOf(err) != ErrNameRequired {
		t.Fatalf("err = %v", err)
	}
	if _, err := r.Create(ctx, Input{Name: "x", Body: "  "}); errs.CodeOf(err) != ErrBodyRequired {
		t.Fatalf("err = %v", err)
	}

	logs, err := r.Create(ctx, Input{Folder: "/docker/", Name: "logs", Body: "docker logs -f {{container}}", Run: true})
	if err != nil {
		t.Fatal(err)
	}
	if logs.Folder != "docker" {
		t.Errorf("folder should be trimmed: %q", logs.Folder)
	}
	r.Create(ctx, Input{Name: "disk", Body: "df -h", Run: true})

	list, _ := r.List(ctx)
	if len(list) != 2 || list[0].Name != "disk" || list[1].Name != "logs" {
		t.Fatalf("list = %+v", list)
	}

	if _, err := r.Update(ctx, logs.ID, Input{Folder: "docker", Name: "logs", Body: "docker logs {{container}}"}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Update(ctx, "missing", Input{Name: "a", Body: "b"}); errs.CodeOf(err) != ErrNotFound {
		t.Fatalf("err = %v", err)
	}
	if err := r.Delete(ctx, logs.ID); err != nil {
		t.Fatal(err)
	}
	list, _ = r.List(ctx)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
}
