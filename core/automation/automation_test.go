package automation

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/store"
)

func names(ms []Match) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Rule.Name+"="+m.Line)
	}
	return out
}

func TestMatcherLinesEscapesAndPrompts(t *testing.T) {
	m := NewMatcher([]Rule{
		{Name: "panic", Pattern: `panic:`, Enabled: true},
		{Name: "sudo", Pattern: `\[sudo\] password for \w+: $`, Enabled: true},
		{Name: "oom", Pattern: `out of memory`, IgnoreCase: true, Enabled: true},
	})
	// Colors and an OSC title are removed before matching.
	got := names(m.Feed([]byte("\x1b]0;title\x07\x1b[31mpan")))
	got = append(got, names(m.Feed([]byte("ic:\x1b[0m boom\r\nok\n")))...)
	if strings.Join(got, ",") != "panic=panic: boom" {
		t.Fatalf("got %v", got)
	}
	// The prompt has no newline yet; it fires once.
	if got := names(m.Feed([]byte("[sudo] password for ci: "))); len(got) != 1 {
		t.Fatalf("prompt: %v", got)
	}
	if got := m.Feed([]byte("\x1b[K")); len(got) != 0 {
		t.Fatalf("fired twice: %v", names(got))
	}
	m.Feed([]byte("\r\n"))
	// A progress bar rewrites the line with \r: only the final text counts.
	got = names(m.Feed([]byte("10%\r50%\rOut Of Memory\n")))
	if strings.Join(got, ",") != "oom=Out Of Memory" {
		t.Fatalf("got %v", got)
	}
}

func TestMatcherIgnoresTypedLines(t *testing.T) {
	m := NewMatcher([]Rule{{Name: "panic", Pattern: `panic:`, Enabled: true}})
	m.Feed([]byte("$ "))
	m.Typed()
	if got := m.Feed([]byte("echo panic: x\r\n")); len(got) != 0 {
		t.Fatalf("the command echo fired: %v", names(got))
	}
	if got := m.Feed([]byte("panic: x\r\n")); len(got) != 1 {
		t.Fatal("the command output must fire")
	}
}

func TestMatcherBoundsLongLines(t *testing.T) {
	m := NewMatcher([]Rule{{Name: "end", Pattern: `END`, Enabled: true}})
	if got := m.Feed([]byte(strings.Repeat("x", MaxLine+10) + "END\n")); len(got) != 0 {
		t.Fatal("text past the limit must be ignored")
	}
	if !NewMatcher([]Rule{{Name: "bad", Pattern: `(`}}).Empty() {
		t.Fatal("invalid rules are skipped")
	}
}

func TestValidateAndApplicable(t *testing.T) {
	respond := Rule{Name: "r", Pattern: "x", Action: ActionRespond, Response: "y", ScopeKind: ScopeGlobal}
	if errs.CodeOf(respond.Validate()) != ErrRespondScope {
		t.Fatal("global responders must be refused")
	}
	respond.ScopeKind, respond.ScopeID = ScopeHost, "h1"
	if err := respond.Validate(); err != nil {
		t.Fatal(err)
	}
	if errs.CodeOf((Rule{Name: "n", Pattern: "(", Action: ActionNotify, ScopeKind: ScopeGlobal}).Validate()) != ErrBadPattern {
		t.Fatal("pattern")
	}
	rules := []Rule{
		{Name: "g", ScopeKind: ScopeGlobal, Enabled: true},
		{Name: "off", ScopeKind: ScopeGlobal},
		{Name: "grp", ScopeKind: ScopeGroup, ScopeID: "prod", Enabled: true},
		{Name: "host", ScopeKind: ScopeHost, ScopeID: "h1", Enabled: true},
		{Name: "other", ScopeKind: ScopeHost, ScopeID: "h2", Enabled: true},
	}
	var got []string
	for _, r := range Applicable(rules, "h1", []string{"root", "prod"}) {
		got = append(got, r.Name)
	}
	if strings.Join(got, ",") != "g,grp,host" {
		t.Fatalf("applicable = %v", got)
	}
	if n := len(Applicable(rules, "", nil)); n != 1 {
		t.Fatalf("quick connections get global rules only: %d", n)
	}
}

func TestRepo(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	r := NewRepo(db)
	x, err := r.Save(ctx, Rule{Name: " Errors ", Pattern: "ERROR", Action: ActionHighlight, Color: "red", ScopeKind: ScopeGlobal, ScopeID: "ignored", Enabled: true})
	if err != nil || x.ID == "" || x.Name != "Errors" || x.ScopeID != "" {
		t.Fatalf("save = %+v, %v", x, err)
	}
	x.Color = "orange"
	if _, err := r.Save(ctx, x); err != nil {
		t.Fatal(err)
	}
	list, _ := r.List(ctx)
	if len(list) != 1 || list[0].Color != "orange" || !list[0].Enabled {
		t.Fatalf("list = %+v", list)
	}
	if err := r.Delete(ctx, x.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Save(ctx, x); errs.CodeOf(err) != ErrNotFound {
		t.Fatalf("update of a deleted rule: %v", err)
	}
}

func TestParseScript(t *testing.T) {
	steps, err := ParseScript([]string{"# login", "cd /srv", "expect \\$ $", "timeout 2", "expect ready", "wait 0.5", "wait", "sendSecret db", "send exit"})
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, s := range steps {
		kinds = append(kinds, s.Kind)
	}
	if strings.Join(kinds, ",") != "send,expect,expect,wait,send,sendSecret,send" {
		t.Fatalf("kinds = %v", kinds)
	}
	if steps[1].Delay != DefaultExpectTimeout || steps[2].Delay != 2*time.Second || steps[3].Delay != 500*time.Millisecond {
		t.Fatalf("delays: %v %v %v", steps[1].Delay, steps[2].Delay, steps[3].Delay)
	}
	if _, err := ParseScript([]string{"ls", "expect ("}); errs.CodeOf(err) != ErrScriptSyntax {
		t.Fatalf("syntax: %v", err)
	}
}

type fakeTerm struct {
	typed   []string
	expects int
	fail    bool
}

func (f *fakeTerm) Type(s string) error { f.typed = append(f.typed, s); return nil }
func (f *fakeTerm) Expect(context.Context, *regexp.Regexp, time.Duration) error {
	f.expects++
	if f.fail {
		return errors.New("timeout")
	}
	return nil
}
func (f *fakeTerm) Secret(_ context.Context, scope string) (string, error) {
	if scope == "db" {
		return "s3cret", nil
	}
	return "", errors.New("missing")
}

func TestRun(t *testing.T) {
	steps, _ := ParseScript([]string{"expect login", "sendSecret db", "ls"})
	term := &fakeTerm{}
	if err := Run(context.Background(), term, steps); err != nil {
		t.Fatal(err)
	}
	if strings.Join(term.typed, "|") != "s3cret\r|ls\r" || term.expects != 1 {
		t.Fatalf("typed %q", term.typed)
	}
	term = &fakeTerm{fail: true}
	if err := Run(context.Background(), term, steps); errs.CodeOf(err) != ErrExpectTimeout || len(term.typed) != 0 {
		t.Fatalf("expect failure: %v %q", err, term.typed)
	}
	steps, _ = ParseScript([]string{"sendSecret nope"})
	if err := Run(context.Background(), &fakeTerm{}, steps); errs.CodeOf(err) != ErrSecretMissing {
		t.Fatalf("secret: %v", err)
	}
}
