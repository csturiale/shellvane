// Package runbooks runs sequences of commands on several hosts through
// exec channels: each step may depend on the previous one's exit status
// or output, capture its output for later steps, and wait for the user's
// confirmation. Runs can be previewed (dry run) and are audited.
package runbooks

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrNameRequired  errs.Code = "runbooks.nameRequired"
	ErrNoSteps       errs.Code = "runbooks.noSteps"
	ErrNoTargets     errs.Code = "runbooks.noTargets"
	ErrBadCondition  errs.Code = "runbooks.badCondition"
	ErrNotFound      errs.Code = "runbooks.notFound"
	ErrDuplicate     errs.Code = "runbooks.duplicate"
	ErrInvalid       errs.Code = "runbooks.invalid"
	ErrStepTimeout   errs.Code = "runbooks.timeout"
	ErrRunNotRunning errs.Code = "runbooks.notRunning"
	ErrCancelled     errs.Code = "runbooks.cancelled"
)

// When a step runs, after the previous one.
const (
	WhenAlways  = ""
	WhenSuccess = "success" // previous step exited 0
	WhenFailure = "failure" // previous step failed
	WhenMatches = "matches" // previous step's output matches Pattern
)

// Step is one command.
type Step struct {
	// Name identifies the step; {{steps.<name>.output}} refers to it.
	Name    string `json:"name" yaml:"name"`
	Command string `json:"command" yaml:"command"`
	When    string `json:"when" yaml:"when,omitempty"`
	Pattern string `json:"pattern" yaml:"pattern,omitempty"`
	// ContinueOnError runs the next steps even if this one fails.
	ContinueOnError bool `json:"continueOnError" yaml:"continueOnError,omitempty"`
	// Confirm pauses before the step until the user continues.
	Confirm bool `json:"confirm" yaml:"confirm,omitempty"`
	// TimeoutSec bounds the step (0 = DefaultTimeout).
	TimeoutSec int `json:"timeoutSec" yaml:"timeoutSec,omitempty"`
}

// Variable is asked when the runbook starts.
type Variable struct {
	Name    string   `json:"name" yaml:"name"`
	Default string   `json:"default" yaml:"default,omitempty"`
	Choices []string `json:"choices" yaml:"choices,omitempty"`
}

// Targets select hosts: explicit hosts, every host of groups (and their
// subgroups) and every host with one of the tags.
type Targets struct {
	HostIDs  []string `json:"hostIds" yaml:"hosts,omitempty"`
	GroupIDs []string `json:"groupIds" yaml:"groups,omitempty"`
	Tags     []string `json:"tags" yaml:"tags,omitempty"`
}

// Runbook is a saved runbook.
type Runbook struct {
	ID          string     `json:"id" yaml:"-"`
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description,omitempty"`
	Variables   []Variable `json:"variables" yaml:"variables,omitempty"`
	Steps       []Step     `json:"steps" yaml:"steps"`
	Targets     Targets    `json:"targets" yaml:"targets"`
	// Parallel runs the hosts at the same time (at most Concurrency);
	// otherwise one host after the other.
	Parallel    bool `json:"parallel" yaml:"parallel,omitempty"`
	Concurrency int  `json:"concurrency" yaml:"concurrency,omitempty"`
	// StopOnHostFailure stops the other hosts when one fails (sequence).
	StopOnHostFailure bool `json:"stopOnHostFailure" yaml:"stopOnHostFailure,omitempty"`
}

// DefaultTimeout bounds a step without its own timeout.
const DefaultTimeout = 10 * time.Minute

// DefaultConcurrency is used for parallel runs without a limit.
const DefaultConcurrency = 8

// Validate checks a runbook before it is saved or run.
func (r Runbook) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errs.New(ErrNameRequired, nil)
	}
	if len(r.Steps) == 0 {
		return errs.New(ErrNoSteps, nil)
	}
	for i, s := range r.Steps {
		switch s.When {
		case WhenAlways, WhenSuccess, WhenFailure:
		case WhenMatches:
			if _, err := regexp.Compile(s.Pattern); err != nil || s.Pattern == "" {
				return errs.New(ErrBadCondition, errs.Params{"step": i + 1})
			}
		default:
			return errs.New(ErrBadCondition, errs.Params{"step": i + 1})
		}
	}
	return nil
}

// Host is a resolved target.
type Host struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Result of one step on one host.
type Result struct {
	ExitCode int     `json:"exitCode"`
	Output   string  `json:"output"` // stdout and stderr, bounded
	Duration float64 `json:"duration"`
	Error    string  `json:"error,omitempty"` // error code: connection, timeout, cancelled
}

// Executor runs a command on a host.
type Executor interface {
	Exec(ctx context.Context, host Host, command string) Result
}

// Step states in a run.
const (
	StatePending   = "pending"
	StateWaiting   = "waiting" // for confirmation
	StateRunning   = "running"
	StateDone      = "done"   // exit 0
	StateFailed    = "failed" // non-zero exit or error
	StateSkipped   = "skipped"
	StateCancelled = "cancelled"
)

// Cell is a step on a host in a run, as shown in the results grid.
type Cell struct {
	State   string  `json:"state"`
	Command string  `json:"command"` // resolved
	Result  *Result `json:"result,omitempty"`
}

// Expand replaces {{name}} with values and {{steps.<name>.output}} with the
// trimmed output of an earlier step on the same host.
func Expand(text string, values map[string]string, outputs map[string]string) string {
	return varRef.ReplaceAllStringFunc(text, func(m string) string {
		name := varRef.FindStringSubmatch(m)[1]
		if step, ok := strings.CutPrefix(name, "steps."); ok {
			if out, ok := outputs[strings.TrimSuffix(step, ".output")]; ok {
				return strings.TrimSpace(out)
			}
			return m
		}
		if v, ok := values[name]; ok {
			return v
		}
		return m
	})
}

var varRef = regexp.MustCompile(`\{\{\s*([\w.-]+)\s*\}\}`)

// Plan is the dry run: the resolved commands per host, before outputs
// are known ({{steps...}} stay as written).
func Plan(r Runbook, hosts []Host, values map[string]string) map[string][]string {
	out := map[string][]string{}
	for _, h := range hosts {
		v := withHost(values, h)
		for _, s := range r.Steps {
			out[h.ID] = append(out[h.ID], Expand(s.Command, v, nil))
		}
	}
	return out
}

func withHost(values map[string]string, h Host) map[string]string {
	v := map[string]string{"host": h.Name}
	for k, x := range values {
		v[k] = x
	}
	return v
}

// Run is a running runbook.
type Run struct {
	ID      string
	Runbook Runbook
	Hosts   []Host
	Values  map[string]string

	mu      sync.Mutex
	cells   map[string][]Cell // host ID -> one cell per step
	confirm map[string]chan bool
	cancel  context.CancelFunc
	done    chan struct{}
	onCell  func(hostID string, step int, c Cell)
}

// Start runs the runbook in the background; onCell reports every change.
func Start(ctx context.Context, id string, r Runbook, hosts []Host, values map[string]string, x Executor, onCell func(string, int, Cell)) *Run {
	ctx, cancel := context.WithCancel(ctx)
	run := &Run{ID: id, Runbook: r, Hosts: hosts, Values: values, cells: map[string][]Cell{}, confirm: map[string]chan bool{},
		cancel: cancel, done: make(chan struct{}), onCell: onCell}
	for _, h := range hosts {
		cells := make([]Cell, len(r.Steps))
		for i := range cells {
			cells[i].State = StatePending
		}
		run.cells[h.ID] = cells
	}
	go func() {
		defer close(run.done)
		defer cancel()
		limit := 1
		if r.Parallel {
			limit = r.Concurrency
			if limit <= 0 {
				limit = DefaultConcurrency
			}
		}
		sem := make(chan struct{}, limit)
		var wg sync.WaitGroup
		for _, h := range hosts {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
			}
			if ctx.Err() != nil {
				run.cancelRest(h.ID)
				continue
			}
			wg.Add(1)
			go func(h Host) {
				defer wg.Done()
				defer func() { <-sem }()
				if !run.host(ctx, h, x) && !r.Parallel && r.StopOnHostFailure {
					cancel()
				}
			}(h)
			if !r.Parallel {
				wg.Wait() // one host at a time
			}
		}
		wg.Wait()
	}()
	return run
}

// host runs the steps on one host; false when a step failed.
func (run *Run) host(ctx context.Context, h Host, x Executor) bool {
	values := withHost(run.Values, h)
	outputs := map[string]string{}
	var prev *Result
	ok := true
	stopped := false
	for i, s := range run.Runbook.Steps {
		command := Expand(s.Command, values, outputs)
		if stopped || ctx.Err() != nil {
			run.set(h.ID, i, Cell{State: StateCancelled, Command: command})
			continue
		}
		if !shouldRun(s, prev) {
			run.set(h.ID, i, Cell{State: StateSkipped, Command: command})
			continue
		}
		if s.Confirm {
			run.set(h.ID, i, Cell{State: StateWaiting, Command: command})
			if !run.wait(ctx, h.ID, i) {
				run.set(h.ID, i, Cell{State: StateCancelled, Command: command})
				stopped = true
				continue
			}
		}
		run.set(h.ID, i, Cell{State: StateRunning, Command: command})
		timeout := DefaultTimeout
		if s.TimeoutSec > 0 {
			timeout = time.Duration(s.TimeoutSec) * time.Second
		}
		sctx, cancel := context.WithTimeout(ctx, timeout)
		res := x.Exec(sctx, h, command)
		if sctx.Err() == context.DeadlineExceeded && res.Error == "" {
			res.Error = string(ErrStepTimeout)
		}
		cancel()
		prev = &res
		if s.Name != "" {
			outputs[s.Name] = res.Output
		}
		state := StateDone
		if res.ExitCode != 0 || res.Error != "" {
			state = StateFailed
			if !s.ContinueOnError {
				ok, stopped = false, true
			}
		}
		run.set(h.ID, i, Cell{State: state, Command: command, Result: &res})
	}
	return ok
}

func shouldRun(s Step, prev *Result) bool {
	if prev == nil {
		return s.When == WhenAlways || s.When == WhenSuccess
	}
	failed := prev.ExitCode != 0 || prev.Error != ""
	switch s.When {
	case WhenSuccess:
		return !failed
	case WhenFailure:
		return failed
	case WhenMatches:
		re, err := regexp.Compile(s.Pattern)
		return err == nil && re.MatchString(prev.Output)
	}
	return true
}

func (run *Run) set(hostID string, step int, c Cell) {
	run.mu.Lock()
	run.cells[hostID][step] = c
	run.mu.Unlock()
	if run.onCell != nil {
		run.onCell(hostID, step, c)
	}
}

func (run *Run) wait(ctx context.Context, hostID string, step int) bool {
	ch := make(chan bool, 1)
	run.mu.Lock()
	run.confirm[key(hostID, step)] = ch
	run.mu.Unlock()
	select {
	case ok := <-ch:
		return ok
	case <-ctx.Done():
		return false
	}
}

// Confirm continues (or, with ok false, stops) a step waiting on a host;
// hostID "" answers every host waiting at that step.
func (run *Run) Confirm(hostID string, step int, ok bool) bool {
	run.mu.Lock()
	defer run.mu.Unlock()
	answered := false
	for k, ch := range run.confirm {
		if k == key(hostID, step) || (hostID == "" && strings.HasSuffix(k, "#"+itoa(step))) {
			ch <- ok
			delete(run.confirm, k)
			answered = true
		}
	}
	return answered
}

func (run *Run) cancelRest(hostID string) {
	for i := range run.Runbook.Steps {
		run.set(hostID, i, Cell{State: StateCancelled})
	}
}

// Cancel stops the run; running commands are interrupted.
func (run *Run) Cancel() { run.cancel() }

// Done is closed when every host has finished.
func (run *Run) Done() <-chan struct{} { return run.done }

// Cells returns a copy of the grid.
func (run *Run) Cells() map[string][]Cell {
	run.mu.Lock()
	defer run.mu.Unlock()
	out := map[string][]Cell{}
	for k, v := range run.cells {
		out[k] = append([]Cell(nil), v...)
	}
	return out
}

func key(hostID string, step int) string { return hostID + "#" + itoa(step) }

func itoa(n int) string {
	b := []byte{}
	if n == 0 {
		return "0"
	}
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
