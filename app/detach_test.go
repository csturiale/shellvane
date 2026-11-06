package app

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

func TestDetachHandsTheTabToOneWindow(t *testing.T) {
	svc := NewSessionService(nil, nil, nil, nil, func(string, any) {})
	EnableLocal(svc, t.TempDir())
	var opened string
	EnableDetach(svc, func(u string) { opened = u })

	info, err := svc.OpenLocal(context.Background(), 80, 24, LocalOptions{})
	if err != nil {
		t.Skip("no local shell:", err)
	}
	defer svc.Close(info.ID)
	if err := svc.Detach(`{"tab":1}`, []string{"missing"}); errs.CodeOf(err) != ErrSessionNotFound {
		t.Fatalf("unknown session: %v", err)
	}
	if err := svc.Detach(`{"tab":1}`, []string{info.ID}); err != nil {
		t.Fatal(err)
	}
	sess, _ := svc.get(info.ID)
	if !sess.handover.Load() {
		t.Fatal("the session must survive its old stream")
	}
	if !strings.HasPrefix(opened, "/?detached=") {
		t.Fatalf("opened %q", opened)
	}
	u, _ := url.Parse(opened)
	token := u.Query().Get("detached")
	payload, err := svc.TakeDetached(token)
	if err != nil || payload != `{"tab":1}` {
		t.Fatalf("TakeDetached = %q, %v", payload, err)
	}
	if _, err := svc.TakeDetached(token); errs.CodeOf(err) != ErrDetachExpired {
		t.Fatalf("second take: %v", err)
	}
}

func TestReattachHandsTheTabToTheMainWindow(t *testing.T) {
	var events []any
	svc := NewSessionService(nil, nil, nil, nil, func(name string, data any) {
		if name == EventSessionReturn {
			events = append(events, data)
		}
	})
	EnableLocal(svc, t.TempDir())
	info, err := svc.OpenLocal(context.Background(), 80, 24, LocalOptions{})
	if err != nil {
		t.Skip("no local shell:", err)
	}
	defer svc.Close(info.ID)
	if err := svc.Reattach(`{"tab":2}`, []string{info.ID}); err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %v", events)
	}
	payload, err := svc.TakeDetached(events[0].(string))
	if err != nil || payload != `{"tab":2}` {
		t.Fatalf("TakeDetached = %q, %v", payload, err)
	}
}
