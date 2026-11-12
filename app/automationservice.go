package app

import (
	"context"
	"strings"

	"github.com/csturiale/shellvane/core/audit"
	"github.com/csturiale/shellvane/core/automation"
)

// AutomationService manages output rules and the audit log.
type AutomationService struct {
	rules *automation.Repo
	audit *audit.Log
}

// NewAutomationService returns a service over the rules and the audit log.
func NewAutomationService(rules *automation.Repo, log *audit.Log) *AutomationService {
	return &AutomationService{rules: rules, audit: log}
}

// Rules lists every rule; they apply to sessions opened afterwards.
func (s *AutomationService) Rules(ctx context.Context) ([]automation.Rule, error) {
	return s.rules.List(ctx)
}

// SaveRule creates (empty ID) or updates a rule.
func (s *AutomationService) SaveRule(ctx context.Context, r automation.Rule) (automation.Rule, error) {
	return s.rules.Save(ctx, r)
}

// DeleteRule removes a rule.
func (s *AutomationService) DeleteRule(ctx context.Context, id string) error {
	return s.rules.Delete(ctx, id)
}

// Audit returns the newest audit entries first.
func (s *AutomationService) Audit(ctx context.Context, limit int) ([]audit.Entry, error) {
	return s.audit.List(ctx, limit)
}

// RecordProcedure adds to the audit log a command of a procedure sent to a
// terminal: action is "run" (Enter pressed) or "paste"; target names the
// terminal (host or environment), procedure the document. The command is
// kept on one line and shortened.
func (s *AutomationService) RecordProcedure(ctx context.Context, action, target, procedure, command string) error {
	if action != "run" {
		action = "paste" // only these two
	}
	return s.audit.Add(ctx, "procedure."+action, target, procedure, oneLine(command, 500))
}

// oneLine joins a command's lines with " ⏎ " and keeps at most max runes.
func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(strings.TrimSpace(s), "\n", " ⏎ ")), " ")
	if r := []rune(s); len(r) > max {
		return string(r[:max]) + "…"
	}
	return s
}

// ExportAudit renders the whole audit log as "csv" or "json".
func (s *AutomationService) ExportAudit(ctx context.Context, format string) (string, error) {
	entries, err := s.audit.List(ctx, 0)
	if err != nil {
		return "", err
	}
	return audit.Export(entries, format)
}

// ClearAudit deletes the audit log.
func (s *AutomationService) ClearAudit(ctx context.Context) error {
	return s.audit.Clear(ctx)
}
