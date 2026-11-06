package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/audit"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/runbooks"
)

// Events of the runbook service.
const (
	EventRunbookCell = "runbook:cell"
	EventRunbookDone = "runbook:done"
)

// maxOutput bounds the output kept per step and host.
const maxOutput = 256 << 10

// RunbookCell is a change in a run's results grid.
type RunbookCell struct {
	RunID  string        `json:"runId"`
	HostID string        `json:"hostId"`
	Step   int           `json:"step"`
	Cell   runbooks.Cell `json:"cell"`
}

// RunbookRun describes a started run.
type RunbookRun struct {
	ID      string           `json:"id"`
	Runbook runbooks.Runbook `json:"runbook"`
	Hosts   []runbooks.Host  `json:"hosts"`
}

// RunbookDone is emitted when every host has finished.
type RunbookDone struct {
	RunID string `json:"runId"`
}

// RunbookPreview is a dry run: the hosts and the resolved commands.
type RunbookPreview struct {
	Hosts    []runbooks.Host     `json:"hosts"`
	Commands map[string][]string `json:"commands"`
}

// RunbookService stores runbooks and runs them on hosts.
type RunbookService struct {
	repo     *runbooks.Repo
	hosts    *hosts.Repo
	sessions *SessionService
	audit    *audit.Log
	emit     Emitter

	mu   sync.Mutex
	runs map[string]*runbooks.Run
}

// NewRunbookService returns a service; commands go through sessions' pool.
func NewRunbookService(repo *runbooks.Repo, h *hosts.Repo, sessions *SessionService, log *audit.Log, emit Emitter) *RunbookService {
	return &RunbookService{repo: repo, hosts: h, sessions: sessions, audit: log, emit: emit, runs: map[string]*runbooks.Run{}}
}

// List returns the saved runbooks.
func (s *RunbookService) List(ctx context.Context) ([]runbooks.Runbook, error) {
	return s.repo.List(ctx)
}

// Save creates (empty ID) or updates a runbook.
func (s *RunbookService) Save(ctx context.Context, rb runbooks.Runbook) (runbooks.Runbook, error) {
	return s.repo.Save(ctx, rb)
}

// Delete removes a runbook.
func (s *RunbookService) Delete(ctx context.Context, id string) error { return s.repo.Delete(ctx, id) }

// Targets resolves hosts, groups (with subgroups) and tags to hosts.
func (s *RunbookService) Targets(ctx context.Context, t runbooks.Targets) ([]runbooks.Host, error) {
	inv, err := s.hosts.List(ctx)
	if err != nil {
		return nil, err
	}
	groups := map[string]bool{}
	for _, g := range t.GroupIDs {
		groups[g] = true
	}
	for grew := true; grew; {
		grew = false
		for _, g := range inv.Groups {
			if groups[g.ParentID] && !groups[g.ID] {
				groups[g.ID], grew = true, true
			}
		}
	}
	ids := map[string]bool{}
	for _, id := range t.HostIDs {
		ids[id] = true
	}
	tags := map[string]bool{}
	for _, tag := range t.Tags {
		tags[strings.ToLower(tag)] = true
	}
	out := []runbooks.Host{}
	for _, h := range inv.Hosts {
		match := ids[h.ID] || (h.GroupID != "" && groups[h.GroupID])
		for _, tag := range h.Tags {
			match = match || tags[strings.ToLower(tag)]
		}
		if match {
			out = append(out, runbooks.Host{ID: h.ID, Name: h.Name})
		}
	}
	return out, nil
}

// Preview resolves the hosts and the commands without running anything.
func (s *RunbookService) Preview(ctx context.Context, rb runbooks.Runbook, values map[string]string) (RunbookPreview, error) {
	if err := rb.Validate(); err != nil {
		return RunbookPreview{}, err
	}
	hs, err := s.Targets(ctx, rb.Targets)
	if err != nil {
		return RunbookPreview{}, err
	}
	return RunbookPreview{Hosts: hs, Commands: runbooks.Plan(rb, hs, values)}, nil
}

// Start runs a runbook (saved or not). Progress arrives as events.
func (s *RunbookService) Start(ctx context.Context, rb runbooks.Runbook, values map[string]string) (RunbookRun, error) {
	if err := rb.Validate(); err != nil {
		return RunbookRun{}, err
	}
	hs, err := s.Targets(ctx, rb.Targets)
	if err != nil {
		return RunbookRun{}, err
	}
	if len(hs) == 0 {
		return RunbookRun{}, errs.New(runbooks.ErrNoTargets, nil)
	}
	id := uuid.NewString()
	x := &sshExecutor{svc: s, runID: id, runbook: rb.Name}
	run := runbooks.Start(context.Background(), id, rb, hs, values, x, func(hostID string, step int, c runbooks.Cell) {
		s.emit(EventRunbookCell, RunbookCell{RunID: id, HostID: hostID, Step: step, Cell: c})
	})
	s.mu.Lock()
	s.runs[id] = run
	s.mu.Unlock()
	go func() {
		<-run.Done()
		s.emit(EventRunbookDone, RunbookDone{RunID: id})
	}()
	return RunbookRun{ID: id, Runbook: rb, Hosts: hs}, nil
}

// Confirm answers a step waiting for confirmation (hostID "" = all hosts).
func (s *RunbookService) Confirm(runID, hostID string, step int, ok bool) error {
	run, err := s.run(runID)
	if err != nil {
		return err
	}
	run.Confirm(hostID, step, ok)
	return nil
}

// Cancel stops a run.
func (s *RunbookService) Cancel(runID string) error {
	run, err := s.run(runID)
	if err != nil {
		return err
	}
	run.Cancel()
	return nil
}

// Cells returns the current grid of a run, by host ID.
func (s *RunbookService) Cells(runID string) (map[string][]runbooks.Cell, error) {
	run, err := s.run(runID)
	if err != nil {
		return nil, err
	}
	return run.Cells(), nil
}

// ExportResults renders a run's results as "csv" or "json".
func (s *RunbookService) ExportResults(runID, format string) (string, error) {
	run, err := s.run(runID)
	if err != nil {
		return "", err
	}
	return runbooks.ExportResults(run.Runbook, run.Hosts, run.Cells(), format)
}

// ExportYAML renders a saved runbook to share.
func (s *RunbookService) ExportYAML(ctx context.Context, id string) (string, error) {
	rb, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	inv, _ := s.hosts.List(ctx)
	return runbooks.ExportYAML(rb, func(kind, id string) string { return nameIn(inv, kind, id) })
}

// ImportYAML saves an exported runbook, linking hosts and groups by name.
func (s *RunbookService) ImportYAML(ctx context.Context, text string) (runbooks.Runbook, error) {
	inv, _ := s.hosts.List(ctx)
	rb, err := runbooks.ImportYAML(text, func(kind, name string) string { return idIn(inv, kind, name) })
	if err != nil {
		return runbooks.Runbook{}, err
	}
	return s.repo.Save(ctx, rb)
}

func nameIn(inv hosts.Inventory, kind, id string) string {
	if kind == "host" {
		for _, h := range inv.Hosts {
			if h.ID == id {
				return h.Name
			}
		}
	}
	for _, g := range inv.Groups {
		if g.ID == id {
			return g.Name
		}
	}
	return ""
}

func idIn(inv hosts.Inventory, kind, name string) string {
	if kind == "host" {
		for _, h := range inv.Hosts {
			if strings.EqualFold(h.Name, name) {
				return h.ID
			}
		}
		return ""
	}
	for _, g := range inv.Groups {
		if strings.EqualFold(g.Name, name) {
			return g.ID
		}
	}
	return ""
}

func (s *RunbookService) run(id string) (*runbooks.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.runs[id]; ok {
		return r, nil
	}
	return nil, errs.New(runbooks.ErrRunNotRunning, errs.Params{"id": id})
}

// sshExecutor runs a step on its own exec channel of the host's pooled
// connection and records it in the audit log.
type sshExecutor struct {
	svc     *RunbookService
	runID   string
	runbook string
}

// limitedBuffer keeps the first n bytes.
type limitedBuffer struct {
	bytes.Buffer
	n int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.n - b.Len(); room > 0 {
		if len(p) > room {
			b.Buffer.Write(p[:room])
		} else {
			b.Buffer.Write(p)
		}
	}
	return len(p), nil
}

func (x *sshExecutor) Exec(ctx context.Context, h runbooks.Host, command string) runbooks.Result {
	start := time.Now()
	res := x.exec(ctx, h, command)
	res.Duration = time.Since(start).Seconds()
	if x.svc.audit != nil {
		detail := fmt.Sprintf("exit %d: %s", res.ExitCode, quoteShort(command))
		if res.Error != "" {
			detail = res.Error + ": " + quoteShort(command)
		}
		if err := x.svc.audit.Add(context.Background(), "runbook.exec", h.Name, x.runbook, detail); err != nil {
			log.Printf("audit: %v", err)
		}
	}
	return res
}

func (x *sshExecutor) exec(ctx context.Context, h runbooks.Host, command string) runbooks.Result {
	ht, err := x.svc.sessions.hostTarget(ctx, h.ID)
	if err != nil {
		return runbooks.Result{ExitCode: -1, Error: string(errs.ToPayload(err).Code)}
	}
	client, release, err := x.svc.sessions.connect(ctx, ht, "runbook:"+x.runID)
	if err != nil {
		return runbooks.Result{ExitCode: -1, Error: string(errs.ToPayload(err).Code)}
	}
	defer release()
	sess, err := client.NewSession()
	if err != nil {
		return runbooks.Result{ExitCode: -1, Error: err.Error()}
	}
	defer sess.Close()
	out := &limitedBuffer{n: maxOutput}
	sess.Stdout, sess.Stderr = out, out
	done := make(chan error, 1)
	go func() { done <- sess.Run(command) }()
	select {
	case err = <-done:
	case <-ctx.Done():
		sess.Signal(ssh.SIGTERM)
		sess.Close()
		return runbooks.Result{ExitCode: -1, Output: out.String(), Error: string(runbooks.ErrCancelled)}
	}
	var exit *ssh.ExitError
	switch {
	case err == nil:
		return runbooks.Result{Output: out.String()}
	case errors.As(err, &exit):
		return runbooks.Result{ExitCode: exit.ExitStatus(), Output: out.String()}
	default:
		return runbooks.Result{ExitCode: -1, Output: out.String(), Error: err.Error()}
	}
}
