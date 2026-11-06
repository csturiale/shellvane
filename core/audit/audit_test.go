package audit

import (
	"context"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/store"
)

func TestLogAndExport(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	l := New(db)
	l.Add(ctx, "trigger.respond", "web-1", "sudo", "sent a vault credential")
	l.Add(ctx, "trigger.respond", "web-2", "yes", `typed "y"`)
	list, err := l.List(ctx, 10)
	if err != nil || len(list) != 2 || list[0].Host != "web-2" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	csv, _ := Export(list, "csv")
	if !strings.HasPrefix(csv, "at,kind,host,subject,detail\n") || !strings.Contains(csv, `"typed ""y"""`) {
		t.Fatalf("csv = %q", csv)
	}
	js, _ := Export(list, "json")
	if !strings.Contains(js, `"subject": "sudo"`) {
		t.Fatalf("json = %s", js)
	}
	l.Clear(ctx)
	if list, _ := l.List(ctx, 0); len(list) != 0 {
		t.Fatal("not cleared")
	}
}
