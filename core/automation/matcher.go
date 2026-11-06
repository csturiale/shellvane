package automation

import (
	"regexp"

	"github.com/csturiale/shellvane/core/termtext"
)

// MaxLine bounds the text kept for one line; longer lines are cut. Go
// regexps run in linear time, so a rule cannot stall the output.
const MaxLine = termtext.MaxLine

// Match is a rule that matched a line.
type Match struct {
	Rule *Rule
	Line string
}

// Matcher finds rules matching terminal output. Escape sequences are
// removed; a carriage return not followed by a newline starts the line
// again (progress bars). An unterminated line is checked too, because
// prompts (sudo, confirmations) do not end with a newline; each rule fires
// at most once per line. A line the user types into (the shell echoing a
// command) is not checked, so typing a pattern does not trigger it.
type Matcher struct {
	rules []compiled
	text  termtext.Cleaner
	fired map[int]bool
	typed bool
}

type compiled struct {
	rule *Rule
	re   *regexp.Regexp
}

// NewMatcher compiles the rules; invalid ones are skipped.
func NewMatcher(rules []Rule) *Matcher {
	m := &Matcher{fired: map[int]bool{}}
	m.text.OnReset = func() {
		m.typed = false
		clear(m.fired)
	}
	for i := range rules {
		if re, err := rules[i].compile(); err == nil {
			m.rules = append(m.rules, compiled{rule: &rules[i], re: re})
		}
	}
	return m
}

// Typed marks the current line as typed by the user.
func (m *Matcher) Typed() { m.typed = true }

// Empty reports whether no rule can ever match.
func (m *Matcher) Empty() bool { return len(m.rules) == 0 }

// Feed consumes output and returns the rules that matched.
func (m *Matcher) Feed(data []byte) []Match {
	if m.Empty() {
		return nil
	}
	var out []Match
	m.text.Feed(data, func(line []byte) { out = m.check(out, line) })
	if line := m.text.Partial(); len(line) > 0 {
		out = m.check(out, line) // the prompt case
	}
	return out
}

func (m *Matcher) check(out []Match, line []byte) []Match {
	if m.typed {
		return out
	}
	for i, c := range m.rules {
		if m.fired[i] {
			continue
		}
		if c.re.Match(line) {
			m.fired[i] = true
			out = append(out, Match{Rule: c.rule, Line: string(line)})
		}
	}
	return out
}
