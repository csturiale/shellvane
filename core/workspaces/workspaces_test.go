package workspaces

import (
	"context"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
)

func repo(t *testing.T) *Repo {
	t.Helper()
	db, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return New(db)
}

func TestSaveReplacesByName(t *testing.T) {
	ctx := context.Background()
	r := repo(t)
	a, err := r.Save(ctx, "Ops", `{"tabs":1}`)
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.Save(ctx, "ops", `{"tabs":2}`)
	if err != nil || b.ID != a.ID {
		t.Fatalf("same name must update: %v %s %s", err, a.ID, b.ID)
	}
	r.Save(ctx, "Dev", `{}`)
	list, _ := r.List(ctx)
	if len(list) != 2 || list[0].Name != "Dev" || list[1].Layout != `{"tabs":2}` {
		t.Fatalf("list = %+v", list)
	}
	if err := r.Delete(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(ctx, a.ID); errs.CodeOf(err) != ErrNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestValidation(t *testing.T) {
	ctx := context.Background()
	r := repo(t)
	if _, err := r.Save(ctx, " ", `{}`); errs.CodeOf(err) != ErrNameRequired {
		t.Fatal(err)
	}
	if _, err := r.Save(ctx, "x", `{`); errs.CodeOf(err) != ErrInvalid {
		t.Fatal(err)
	}
}

func TestLastIsHiddenAndReplaced(t *testing.T) {
	ctx := context.Background()
	r := repo(t)
	if got, err := r.Last(ctx); err != nil || got != "" {
		t.Fatalf("empty last = %q, %v", got, err)
	}
	r.SaveLast(ctx, `{"v":1}`)
	r.SaveLast(ctx, `{"v":2}`)
	if got, _ := r.Last(ctx); got != `{"v":2}` {
		t.Fatalf("last = %q", got)
	}
	if list, _ := r.List(ctx); len(list) != 0 {
		t.Fatalf("last listed: %+v", list)
	}
	if err := r.Delete(ctx, lastID); errs.CodeOf(err) != ErrNotFound {
		t.Fatal("last cannot be deleted as a workspace")
	}
	r.SaveLast(ctx, "")
	if got, _ := r.Last(ctx); got != "" {
		t.Fatalf("forgotten last = %q", got)
	}
}
