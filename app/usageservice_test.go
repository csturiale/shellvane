package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUsageSendsOnlyCleanCounters(t *testing.T) {
	var got UsagePayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	old := UsageURL
	UsageURL = srv.URL
	defer func() { UsageURL = old }()

	s := NewUsageService()
	err := s.Send(context.Background(), UsageReport{Days: 7,
		Features: map[string]int{"session.ssh": 12, "Bad Key": 3, "split": 0, "web-01.prod": 2},
		Tools:    map[string]int{"kubectl": 40, "/usr/bin/secret --token=x": 1}})
	if err != nil {
		t.Fatal(err)
	}
	if got.V != 1 || got.Days != 7 || got.OS == "" || got.Version == "" {
		t.Fatalf("payload %+v", got)
	}
	if len(got.Features) != 2 || got.Features["session.ssh"] != 12 || got.Features["web-01.prod"] != 2 {
		t.Fatalf("features %+v", got.Features)
	}
	if len(got.Tools) != 1 || got.Tools["kubectl"] != 40 {
		t.Fatalf("tools %+v", got.Tools)
	}
}
