// Package diagnostics keeps recent connection handshakes and builds the
// anonymized support bundle.
package diagnostics

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/anonymize"
	"github.com/csturiale/shellvane/core/sshengine"
)

// Handshake is one connection attempt.
type Handshake struct {
	ID      string                 `json:"id"`
	HostID  string                 `json:"hostId"`
	Name    string                 `json:"name"`
	Address string                 `json:"address"` // host:port
	User    string                 `json:"user"`
	Started time.Time              `json:"started"`
	OK      bool                   `json:"ok"`
	Code    string                 `json:"code"` // error code when it failed
	Events  []sshengine.TraceEvent `json:"events"`
}

// Log keeps the most recent handshakes.
type Log struct {
	mu    sync.Mutex
	max   int
	items []Handshake
}

// NewLog keeps at most max handshakes.
func NewLog(max int) *Log { return &Log{max: max} }

// Add records a handshake, dropping the oldest beyond the limit.
func (l *Log) Add(h Handshake) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = append([]Handshake{h}, l.items...)
	if len(l.items) > l.max {
		l.items = l.items[:l.max]
	}
}

// List returns the handshakes, newest first.
func (l *Log) List() []Handshake {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Handshake(nil), l.items...)
}

// Get returns one handshake.
func (l *Log) Get(id string) (Handshake, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, h := range l.items {
		if h.ID == id {
			return h, true
		}
	}
	return Handshake{}, false
}

// Format writes a handshake as a plain-text report for support, with
// names, users and addresses replaced by pseudonyms.
func Format(h Handshake, a *anonymize.Anonymizer) string {
	name := a.Name(anonymize.KindHost, h.Name)
	user := a.Name(anonymize.KindUser, h.User)
	addr := a.Host(h.Address)
	var b strings.Builder
	outcome := "OK"
	if !h.OK {
		outcome = "FAILED " + h.Code
	}
	fmt.Fprintf(&b, "Connection %s (%s@%s) at %s: %s\n", name, user, addr, h.Started.UTC().Format(time.RFC3339), outcome)
	for _, e := range h.Events {
		keys := make([]string, 0, len(e.Detail))
		for k := range e.Detail {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			v := e.Detail[k]
			switch k {
			case "user":
				v = a.Name(anonymize.KindUser, v)
			case "address", "remote":
				v = a.Host(v)
			default:
				v = a.Text(v)
			}
			parts = append(parts, fmt.Sprintf("%s=%q", k, v))
		}
		fmt.Fprintf(&b, "  +%5dms %-14s %s\n", e.Ms, e.Step, strings.Join(parts, " "))
	}
	return b.String()
}
