package automation

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes of startup scripts.
const (
	ErrScriptSyntax  errs.Code = "automation.scriptSyntax"
	ErrExpectTimeout errs.Code = "automation.expectTimeout"
	ErrSecretMissing errs.Code = "automation.secretMissing"
)

// DefaultExpectTimeout applies until a "timeout" step changes it.
const DefaultExpectTimeout = 15 * time.Second

// Step is one line of a startup script:
//
//	command text          types the line and presses Enter (as before)
//	send <text>           the same, explicitly
//	expect <regex>        waits until the output matches
//	timeout <seconds>     sets the timeout of the following expects
//	wait <seconds>        pauses
//	sendSecret <scope>    types a vault credential and presses Enter
//	# comment             ignored
type Step struct {
	Kind    string // send, expect, wait, sendSecret
	Text    string // send: the text; sendSecret: the scope
	Pattern *regexp.Regexp
	Delay   time.Duration // wait; expect: its timeout
}

// ParseScript turns startup lines into steps. Plain lines are commands, so
// lists written before scripts existed keep working.
func ParseScript(lines []string) ([]Step, error) {
	timeout := DefaultExpectTimeout
	var steps []Step
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		word, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		bad := errs.Params{"line": i + 1, "text": line}
		switch word {
		case "send":
			steps = append(steps, Step{Kind: "send", Text: rest})
		case "sendSecret":
			if rest == "" {
				return nil, errs.New(ErrScriptSyntax, bad)
			}
			steps = append(steps, Step{Kind: "sendSecret", Text: rest})
		case "expect":
			re, err := regexp.Compile(rest)
			if rest == "" || err != nil {
				return nil, errs.New(ErrScriptSyntax, bad)
			}
			steps = append(steps, Step{Kind: "expect", Pattern: re, Delay: timeout})
		case "wait", "timeout":
			secs, err := strconv.ParseFloat(rest, 64)
			if err != nil || secs < 0 || secs > 3600 {
				if word == "wait" && err != nil {
					// "wait" alone (or "wait $pid") is the shell builtin.
					steps = append(steps, Step{Kind: "send", Text: line})
					continue
				}
				return nil, errs.New(ErrScriptSyntax, bad)
			}
			d := time.Duration(secs * float64(time.Second))
			if word == "timeout" {
				timeout = d
			} else {
				steps = append(steps, Step{Kind: "wait", Delay: d})
			}
		default:
			steps = append(steps, Step{Kind: "send", Text: line})
		}
	}
	return steps, nil
}

// Terminal is what a script drives.
type Terminal interface {
	// Type writes keystrokes.
	Type(text string) error
	// Expect blocks until output written after the call matches re.
	Expect(ctx context.Context, re *regexp.Regexp, timeout time.Duration) error
	// Secret returns a vault credential.
	Secret(ctx context.Context, scope string) (string, error)
}

// Run executes the steps in order and stops at the first failure.
func Run(ctx context.Context, t Terminal, steps []Step) error {
	for i, s := range steps {
		var err error
		switch s.Kind {
		case "send":
			err = t.Type(s.Text + "\r")
		case "sendSecret":
			var secret string
			if secret, err = t.Secret(ctx, s.Text); err == nil {
				err = t.Type(secret + "\r")
			} else {
				err = errs.Wrap(err, ErrSecretMissing, errs.Params{"scope": s.Text})
			}
		case "wait":
			select {
			case <-time.After(s.Delay):
			case <-ctx.Done():
				err = ctx.Err()
			}
		case "expect":
			// The pattern may already be on screen when the next step
			// starts: expectations look at output that follows the
			// previous step, which the caller arranges by starting to
			// watch before typing.
			if err = t.Expect(ctx, s.Pattern, s.Delay); err != nil && ctx.Err() == nil {
				err = errs.Wrap(err, ErrExpectTimeout, errs.Params{"pattern": s.Pattern.String(), "step": i + 1})
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// HasExpect reports whether the script needs to watch the output.
func HasExpect(steps []Step) bool {
	for _, s := range steps {
		if s.Kind == "expect" {
			return true
		}
	}
	return false
}
