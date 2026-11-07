package app

import (
	"net/http"
	"net/url"
	"strings"
)

// RejectForeignOrigins is middleware for the app's asset server: calls to
// the runtime (/wails/…: bindings, events, streams) are refused when they
// come from a page of another site, such as the sign-in window's (Entra
// ID, the PVWA). Desktop builds of Wails do not check the origin of these
// calls, and a page could otherwise call the app's services. Requests
// without an Origin header (same-origin GETs) and those of the app's own
// pages pass.
func RejectForeignOrigins(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/wails/") && foreignOrigin(r) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// foreignOrigin reports an http(s) Origin of another host than the app's
// (wails.localhost on Windows, the wails:// scheme elsewhere).
func foreignOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" || o == "null" {
		return false
	}
	u, err := url.Parse(o)
	if err != nil {
		return true
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false // wails://, the app's own scheme
	}
	host := strings.ToLower(u.Hostname())
	return host != "wails.localhost" && host != "wails" && !strings.EqualFold(u.Host, r.Host)
}
