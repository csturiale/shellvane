package app

import (
	"testing"

	"github.com/csturiale/shellvane/core/hosts"
)

func TestRecentHosts(t *testing.T) {
	list := []hosts.Host{
		{ID: "a", Name: "a", LastUsed: "2026-10-01T10:00:00Z"},
		{ID: "b", Name: "b"},
		{ID: "c", Name: "c", LastUsed: "2026-10-03T10:00:00Z"},
		{ID: "d", Name: "d", LastUsed: "2026-10-02T10:00:00Z"},
	}
	got := recentHosts(list, 2)
	if len(got) != 2 || got[0].ID != "c" || got[1].ID != "d" {
		t.Fatalf("recent = %+v", got)
	}
	if len(recentHosts(nil, 8)) != 0 {
		t.Fatal("empty")
	}
}
