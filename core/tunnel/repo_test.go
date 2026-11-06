package tunnel

import (
	"context"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/store"
)

func TestRepo(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	h, err := hosts.New(db).CreateHost(ctx, hosts.HostInput{Hostname: "db.internal"})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRepo(db)

	_, err = r.Create(ctx, Input{HostID: h.ID, Spec: Spec{Kind: Local, BindPort: 5432}})
	if errs.CodeOf(err) != ErrInvalid {
		t.Fatalf("missing destination must be rejected: %v", err)
	}
	pg, err := r.Create(ctx, Input{HostID: h.ID, Spec: Spec{Kind: Local, BindPort: 15432, DestHost: "localhost", DestPort: 5432}, Autostart: true})
	if err != nil {
		t.Fatal(err)
	}
	if pg.Name != "127.0.0.1:15432 → localhost:5432" || pg.BindAddr != "127.0.0.1" {
		t.Errorf("defaults not applied: %+v", pg)
	}
	socks, _ := r.Create(ctx, Input{HostID: h.ID, Name: "proxy", Spec: Spec{Kind: Dynamic, BindPort: 1080, DestHost: "ignored", DestPort: 9}})

	got, err := r.Get(ctx, socks.ID)
	if err != nil || got.DestHost != "" || got.Kind != Dynamic {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	list, _ := r.List(ctx)
	if len(list) != 2 || !list[0].Autostart {
		t.Fatalf("list = %+v", list)
	}

	// Deleting the host deletes its tunnels.
	if err := hosts.New(db).DeleteHost(ctx, h.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := r.List(ctx); len(list) != 0 {
		t.Fatalf("tunnels should be deleted with their host: %+v", list)
	}
}
