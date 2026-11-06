// Package automation evaluates rules on terminal output (highlight, notify,
// respond) and runs startup scripts (send, expect, wait, sendSecret).
package automation

import (
	"context"
	"database/sql"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrNameRequired  errs.Code = "automation.nameRequired"
	ErrBadPattern    errs.Code = "automation.badPattern"
	ErrBadAction     errs.Code = "automation.badAction"
	ErrRespondScope  errs.Code = "automation.respondScope"
	ErrResponseEmpty errs.Code = "automation.responseRequired"
	ErrNotFound      errs.Code = "automation.notFound"
)

// Actions a rule can take.
const (
	ActionHighlight = "highlight"
	ActionNotify    = "notify"
	ActionRespond   = "respond"
)

// Scopes a rule applies to.
const (
	ScopeGlobal = "global"
	ScopeGroup  = "group" // the group and its subgroups
	ScopeHost   = "host"
)

// Rule reacts to terminal output matching Pattern (RE2 syntax).
type Rule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Pattern    string `json:"pattern"`
	IgnoreCase bool   `json:"ignoreCase"`
	Action     string `json:"action"`
	// Color is a highlight's color name.
	Color string `json:"color"`
	// Response is typed when a respond rule matches; SecretScope sends a
	// vault credential instead. Enter presses Enter after it.
	Response    string `json:"response"`
	SecretScope string `json:"secretScope"`
	Enter       bool   `json:"enter"`
	// Sound plays with a notification.
	Sound     bool   `json:"sound"`
	ScopeKind string `json:"scopeKind"`
	ScopeID   string `json:"scopeId"`
	Enabled   bool   `json:"enabled"`
}

// Validate checks a rule before it is saved.
func (r Rule) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errs.New(ErrNameRequired, nil)
	}
	if _, err := r.compile(); err != nil {
		return errs.Wrap(err, ErrBadPattern, errs.Params{"pattern": r.Pattern})
	}
	switch r.Action {
	case ActionHighlight, ActionNotify:
	case ActionRespond:
		// Typing into a terminal on output is only allowed where the user
		// asked for it, never everywhere.
		if r.ScopeKind != ScopeHost && r.ScopeKind != ScopeGroup {
			return errs.New(ErrRespondScope, nil)
		}
		if r.Response == "" && r.SecretScope == "" {
			return errs.New(ErrResponseEmpty, nil)
		}
	default:
		return errs.New(ErrBadAction, errs.Params{"action": r.Action})
	}
	switch r.ScopeKind {
	case ScopeGlobal, ScopeGroup, ScopeHost:
	default:
		return errs.New(ErrBadAction, errs.Params{"action": r.ScopeKind})
	}
	return nil
}

func (r Rule) compile() (*regexp.Regexp, error) {
	if r.Pattern == "" {
		return nil, errs.New(ErrBadPattern, nil)
	}
	p := r.Pattern
	if r.IgnoreCase {
		p = "(?i)" + p
	}
	return regexp.Compile(p)
}

// Applicable returns the enabled rules for a host in the given groups
// (any order); hostID "" (quick connections) gets the global rules only.
func Applicable(rules []Rule, hostID string, groupIDs []string) []Rule {
	groups := map[string]bool{}
	for _, g := range groupIDs {
		groups[g] = true
	}
	var out []Rule
	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		switch {
		case r.ScopeKind == ScopeGlobal,
			r.ScopeKind == ScopeHost && hostID != "" && r.ScopeID == hostID,
			r.ScopeKind == ScopeGroup && groups[r.ScopeID]:
			out = append(out, r)
		}
	}
	return out
}

// Repo stores rules.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// NewRepo returns a Repo over a migrated database.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db, now: time.Now} }

// List returns all rules sorted by name.
func (r *Repo) List(ctx context.Context) ([]Rule, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, pattern, ignore_case, action, color, response, secret_scope, enter, sound, scope_kind, scope_id, enabled
		FROM rules ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var x Rule
		if err := rows.Scan(&x.ID, &x.Name, &x.Pattern, &x.IgnoreCase, &x.Action, &x.Color, &x.Response, &x.SecretScope,
			&x.Enter, &x.Sound, &x.ScopeKind, &x.ScopeID, &x.Enabled); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// Save creates the rule (empty ID) or replaces it.
func (r *Repo) Save(ctx context.Context, x Rule) (Rule, error) {
	x.Name = strings.TrimSpace(x.Name)
	if x.ScopeKind == ScopeGlobal {
		x.ScopeID = ""
	}
	if err := x.Validate(); err != nil {
		return Rule{}, err
	}
	now := r.now().UTC().Format(time.RFC3339)
	if x.ID == "" {
		x.ID = uuid.NewString()
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO rules (id, name, pattern, ignore_case, action, color, response, secret_scope, enter, sound, scope_kind, scope_id, enabled, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			x.ID, x.Name, x.Pattern, x.IgnoreCase, x.Action, x.Color, x.Response, x.SecretScope, x.Enter, x.Sound, x.ScopeKind, x.ScopeID, x.Enabled, now, now)
		return x, err
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE rules SET name = ?, pattern = ?, ignore_case = ?, action = ?, color = ?, response = ?, secret_scope = ?,
			enter = ?, sound = ?, scope_kind = ?, scope_id = ?, enabled = ?, updated_at = ?
		WHERE id = ?`,
		x.Name, x.Pattern, x.IgnoreCase, x.Action, x.Color, x.Response, x.SecretScope, x.Enter, x.Sound, x.ScopeKind, x.ScopeID, x.Enabled, now, x.ID)
	if err != nil {
		return Rule{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Rule{}, errs.New(ErrNotFound, errs.Params{"id": x.ID})
	}
	return x, nil
}

// Delete removes a rule.
func (r *Repo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return nil
}
