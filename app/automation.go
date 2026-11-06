package app

import (
	"context"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/audit"
	"github.com/csturiale/shellvane/core/automation"
	"github.com/csturiale/shellvane/core/errs"
)

// EventSessionTrigger reports a notify rule, an automatic response or a
// failed startup script.
const EventSessionTrigger = "session:trigger"

// TriggerEvent is emitted when automation acts on a session.
type TriggerEvent struct {
	SessionID string `json:"sessionId"`
	// Kind is "notify", "respond", "respondSkipped" or "script".
	Kind  string        `json:"kind"`
	Rule  string        `json:"rule"`
	Line  string        `json:"line"`
	Sound bool          `json:"sound"`
	Error *errs.Payload `json:"error,omitempty"`
}

// Highlight is a highlight rule sent to the terminal, which colors matches
// without changing the bytes.
type Highlight struct {
	Pattern    string `json:"pattern"`
	IgnoreCase bool   `json:"ignoreCase"`
	Color      string `json:"color"`
}

// respondLimit bounds automatic responses per rule and session: hostile
// output repeating a prompt must not make the app type forever.
const (
	respondLimit  = 3
	respondWindow = time.Minute
)

// automationState is the per-session part of the rules engine.
type automationState struct {
	host    string // name, for notifications and the audit log
	matchMu sync.Mutex
	matcher *automation.Matcher

	mu        sync.Mutex
	broadcast bool                   // responders are off while broadcasting
	responses map[string][]time.Time // rule ID -> recent responses
	watchers  map[int]func([]byte)
	nextWatch int
}

// EnableAutomation turns on rules and the audit log for s. It is not a
// method so that Wails does not expose it as a binding.
func EnableAutomation(s *SessionService, rules *automation.Repo, log *audit.Log) {
	s.rules, s.audit = rules, log
}

// SetBroadcast tells which sessions receive broadcast input: automatic
// responses are suspended there.
func (s *SessionService) SetBroadcast(ids []string, on bool) {
	for _, id := range ids {
		if sess, err := s.get(id); err == nil && sess.auto != nil {
			sess.auto.mu.Lock()
			sess.auto.broadcast = on
			sess.auto.mu.Unlock()
		}
	}
}

// loadRules returns the rules that apply to a saved host (or the global
// ones for a quick connection).
func (s *SessionService) loadRules(ctx context.Context, ht resolvedHost) []automation.Rule {
	if s.rules == nil {
		return nil
	}
	all, err := s.rules.List(ctx)
	if err != nil {
		log.Printf("rules: %v", err)
		return nil
	}
	var groups []string
	if ht.id != "" && s.hosts != nil {
		if chain, err := s.hosts.Chain(ctx, ht.groupID); err == nil {
			for _, lvl := range chain {
				groups = append(groups, lvl.ID)
			}
		}
	}
	return automation.Applicable(all, ht.id, groups)
}

// splitRules separates highlights (applied by the terminal) from the rules
// evaluated here.
func splitRules(rules []automation.Rule) (highlights []Highlight, active []automation.Rule) {
	highlights = []Highlight{}
	for _, r := range rules {
		if r.Action == automation.ActionHighlight {
			highlights = append(highlights, Highlight{Pattern: r.Pattern, IgnoreCase: r.IgnoreCase, Color: r.Color})
		} else {
			active = append(active, r)
		}
	}
	return highlights, active
}

// observe passes terminal output to the rules and script watchers.
func (s *SessionService) observe(sess *session, frame []byte) {
	a := sess.auto
	if a == nil {
		return
	}
	a.mu.Lock()
	for _, w := range a.watchers {
		w(frame)
	}
	a.mu.Unlock()
	a.matchMu.Lock()
	matches := a.matcher.Feed(frame)
	a.matchMu.Unlock()
	for _, m := range matches {
		switch m.Rule.Action {
		case automation.ActionNotify:
			s.emit(EventSessionTrigger, TriggerEvent{SessionID: sess.id, Kind: "notify", Rule: m.Rule.Name, Line: m.Line, Sound: m.Rule.Sound})
		case automation.ActionRespond:
			s.respond(sess, *m.Rule, m.Line)
		}
	}
}

func (s *SessionService) respond(sess *session, r automation.Rule, line string) {
	a := sess.auto
	a.mu.Lock()
	now := time.Now()
	recent := a.responses[r.ID][:0]
	for _, t := range a.responses[r.ID] {
		if now.Sub(t) < respondWindow {
			recent = append(recent, t)
		}
	}
	allowed := !a.broadcast && len(recent) < respondLimit
	if allowed {
		recent = append(recent, now)
	}
	a.responses[r.ID] = recent
	a.mu.Unlock()
	if !allowed {
		s.emit(EventSessionTrigger, TriggerEvent{SessionID: sess.id, Kind: "respondSkipped", Rule: r.Name, Line: line})
		return
	}
	go func() {
		ctx := context.Background()
		text, detail := r.Response, "typed "+quoteShort(r.Response)
		if r.SecretScope != "" {
			secret, err := s.secret(ctx, r.SecretScope)
			if err != nil {
				p := errs.ToPayload(errs.Wrap(err, automation.ErrSecretMissing, errs.Params{"scope": r.SecretScope}))
				s.emit(EventSessionTrigger, TriggerEvent{SessionID: sess.id, Kind: "respondSkipped", Rule: r.Name, Line: line, Error: &p})
				return
			}
			text, detail = secret, "sent vault credential "+r.SecretScope
		}
		if r.Enter {
			text += "\r"
		}
		if _, err := sess.shell.Write([]byte(text)); err != nil {
			return
		}
		if s.audit != nil {
			if err := s.audit.Add(ctx, "trigger.respond", a.host, r.Name, detail+" after "+quoteShort(line)); err != nil {
				log.Printf("audit: %v", err)
			}
		}
		s.emit(EventSessionTrigger, TriggerEvent{SessionID: sess.id, Kind: "respond", Rule: r.Name, Line: line})
	}()
}

// secret reads a vault credential; the vault must be unlocked.
func (s *SessionService) secret(ctx context.Context, scope string) (string, error) {
	if s.vault == nil {
		return "", errs.New(automation.ErrSecretMissing, errs.Params{"scope": scope})
	}
	b, found, err := s.vault.v.Credential(ctx, scope)
	if err != nil {
		return "", err
	}
	if !found {
		return "", errs.New(automation.ErrSecretMissing, errs.Params{"scope": scope})
	}
	return string(b), nil
}

func quoteShort(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 80 {
		s = s[:80] + "…"
	}
	return `"` + s + `"`
}

// typed tells the rules that the user is typing on the current line.
func (a *automationState) typed() {
	a.matchMu.Lock()
	a.matcher.Typed()
	a.matchMu.Unlock()
}

// watch calls fn with every output frame until the returned stop is called.
func (a *automationState) watch(fn func([]byte)) (stop func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := a.nextWatch
	a.nextWatch++
	a.watchers[id] = fn
	return func() {
		a.mu.Lock()
		delete(a.watchers, id)
		a.mu.Unlock()
	}
}

// scriptTerminal runs a startup script against a session. Expectations
// look at the output since the previous keystrokes.
type scriptTerminal struct {
	svc  *SessionService
	sess *session

	mu     sync.Mutex
	recent []byte // output since the last Type, bounded
	signal chan struct{}
}

const scriptBuffer = 64 * 1024

func newScriptTerminal(svc *SessionService, sess *session) (*scriptTerminal, func()) {
	t := &scriptTerminal{svc: svc, sess: sess, signal: make(chan struct{}, 1)}
	stop := sess.auto.watch(func(b []byte) {
		t.mu.Lock()
		t.recent = append(t.recent, b...)
		if len(t.recent) > scriptBuffer {
			t.recent = t.recent[len(t.recent)-scriptBuffer:]
		}
		t.mu.Unlock()
		select {
		case t.signal <- struct{}{}:
		default:
		}
	})
	return t, stop
}

func (t *scriptTerminal) Type(text string) error {
	t.mu.Lock()
	t.recent = t.recent[:0]
	t.mu.Unlock()
	_, err := t.sess.shell.Write([]byte(text))
	return err
}

func (t *scriptTerminal) Expect(ctx context.Context, re *regexp.Regexp, timeout time.Duration) error {
	m := automation.NewMatcher([]automation.Rule{{Name: "expect", Pattern: re.String(), Enabled: true}})
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	seen := 0
	for {
		t.mu.Lock()
		if seen > len(t.recent) {
			seen = 0 // reset by Type
		}
		chunk := append([]byte(nil), t.recent[seen:]...)
		seen = len(t.recent)
		t.mu.Unlock()
		if len(m.Feed(chunk)) > 0 {
			return nil
		}
		select {
		case <-t.signal:
		case <-deadline.C:
			return context.DeadlineExceeded
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (t *scriptTerminal) Secret(ctx context.Context, scope string) (string, error) {
	return t.svc.secret(ctx, scope)
}

// runScript types the startup script; failures are reported, the shell
// stays usable.
func (s *SessionService) runScript(sess *session, lines []string) {
	steps, err := automation.ParseScript(lines)
	if err == nil && len(steps) == 0 {
		return
	}
	if err == nil {
		term, stop := newScriptTerminal(s, sess)
		defer stop()
		err = automation.Run(sess.ctx, term, steps)
	}
	if err != nil && sess.ctx.Err() == nil {
		p := errs.ToPayload(err)
		s.emit(EventSessionTrigger, TriggerEvent{SessionID: sess.id, Kind: "script", Error: &p})
	}
}
