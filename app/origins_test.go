package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRejectForeignOrigins(t *testing.T) {
	h := RejectForeignOrigins(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	cases := []struct {
		path, host, origin string
		want               int
	}{
		{"/wails/runtime", "wails.localhost", "http://wails.localhost", http.StatusTeapot},
		{"/wails/runtime", "wails.localhost", "https://wails.localhost", http.StatusTeapot},
		{"/wails/runtime", "wails", "wails://wails", http.StatusTeapot},
		{"/wails/runtime", "wails", "", http.StatusTeapot},
		{"/wails/runtime", "localhost:9245", "http://localhost:9245", http.StatusTeapot}, // dev server
		{"/wails/runtime", "wails.localhost", "https://login.microsoftonline.com", http.StatusForbidden},
		{"/wails/stream/x", "wails", "https://pvwa.example.org", http.StatusForbidden},
		{"/index.html", "wails.localhost", "https://login.microsoftonline.com", http.StatusTeapot},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodPost, "http://"+c.host+c.path, nil)
		if c.origin != "" {
			r.Header.Set("Origin", c.origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("%s from %q: %d, want %d", c.path, c.origin, w.Code, c.want)
		}
	}
}
