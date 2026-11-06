// Package themes stores custom terminal color schemes and imports them
// from other terminals' formats.
package themes

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes.
const (
	ErrNotFound  errs.Code = "themes.notFound"
	ErrDuplicate errs.Code = "themes.duplicate"
	ErrInvalid   errs.Code = "themes.invalid"
	ErrFormat    errs.Code = "themes.unknownFormat"
)

// ColorKeys are the colors of a scheme, named like xterm.js's ITheme.
var ColorKeys = []string{
	"background", "foreground", "cursor", "cursorAccent", "selectionBackground",
	"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
	"brightBlack", "brightRed", "brightGreen", "brightYellow", "brightBlue", "brightMagenta", "brightCyan", "brightWhite",
}

// required colors; the others have defaults.
var required = []string{"background", "foreground"}

// Theme is a terminal color scheme.
type Theme struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Dark   bool              `json:"dark"`
	Colors map[string]string `json:"colors"` // ColorKeys → #rrggbb
}

var hexColor = regexp.MustCompile(`^#[0-9a-f]{6}$`)

// Normalize checks a theme, lower-cases colors, fills missing ANSI
// colors from the foreground/background and works out Dark.
func (t *Theme) Normalize() error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return errs.New(ErrInvalid, errs.Params{"field": "name"})
	}
	colors := map[string]string{}
	for _, k := range ColorKeys {
		v, ok := t.Colors[k]
		if !ok || v == "" {
			continue
		}
		c, err := NormalizeColor(v)
		if err != nil {
			return errs.New(ErrInvalid, errs.Params{"field": k})
		}
		colors[k] = c
	}
	for _, k := range required {
		if colors[k] == "" {
			return errs.New(ErrInvalid, errs.Params{"field": k})
		}
	}
	if colors["cursor"] == "" {
		colors["cursor"] = colors["foreground"]
	}
	for i, k := range ColorKeys[5:13] {
		if colors[k] == "" {
			colors[k] = defaultANSI[i]
		}
		bright := ColorKeys[13+i]
		if colors[bright] == "" {
			colors[bright] = colors[k]
		}
	}
	t.Colors = colors
	t.Dark = luminance(colors["background"]) < 0.4
	return nil
}

var defaultANSI = []string{"#000000", "#cd3131", "#0dbc79", "#e5e510", "#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5"}

// NormalizeColor accepts #rgb, #rrggbb, #rrggbbaa, rrggbb, 0xrrggbb and
// rgb(r, g, b), and returns #rrggbb.
func NormalizeColor(v string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(v), `"'`)))
	if strings.HasPrefix(s, "rgb(") && strings.HasSuffix(s, ")") {
		parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(s, "rgb("), ")"), ",")
		if len(parts) == 3 {
			var rgb [3]int
			for i, p := range parts {
				n, err := strconv.Atoi(strings.TrimSpace(p))
				if err != nil || n < 0 || n > 255 {
					return "", fmt.Errorf("bad color %q", v)
				}
				rgb[i] = n
			}
			return fmt.Sprintf("#%02x%02x%02x", rgb[0], rgb[1], rgb[2]), nil
		}
	}
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "#")
	switch len(s) {
	case 3:
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	case 8:
		s = s[:6]
	}
	if !hexColor.MatchString("#" + s) {
		return "", fmt.Errorf("bad color %q", v)
	}
	return "#" + s, nil
}

func luminance(hex string) float64 {
	r, _ := strconv.ParseUint(hex[1:3], 16, 8)
	g, _ := strconv.ParseUint(hex[3:5], 16, 8)
	b, _ := strconv.ParseUint(hex[5:7], 16, 8)
	return (0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)) / 255
}

// Repo stores custom themes.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// NewRepo returns a Repo over a migrated database.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db, now: time.Now} }

// List returns the custom themes by name.
func (r *Repo) List(ctx context.Context) ([]Theme, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, data FROM themes ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Theme{}
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var t Theme
		if json.Unmarshal([]byte(data), &t) == nil {
			t.ID = id
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

// Save creates (empty ID) or replaces a theme.
func (r *Repo) Save(ctx context.Context, t Theme) (Theme, error) {
	if err := t.Normalize(); err != nil {
		return Theme{}, err
	}
	if t.ID == "" {
		t.ID = "custom-" + uuid.NewString()
	}
	data, _ := json.Marshal(t)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO themes (id, name, data, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, data = excluded.data, updated_at = excluded.updated_at`,
		t.ID, t.Name, string(data), r.now().UTC().Format(time.RFC3339))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return Theme{}, errs.New(ErrDuplicate, errs.Params{"name": t.Name})
	}
	return t, err
}

// Delete removes a theme.
func (r *Repo) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM themes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return nil
}

// FreeName returns name, or "name (2)", "name (3)"... not used yet.
func (r *Repo) FreeName(ctx context.Context, name string) (string, error) {
	list, err := r.List(ctx)
	if err != nil {
		return "", err
	}
	used := map[string]bool{}
	for _, t := range list {
		used[strings.ToLower(t.Name)] = true
	}
	candidate := name
	for i := 2; used[strings.ToLower(candidate)]; i++ {
		candidate = fmt.Sprintf("%s (%d)", name, i)
	}
	return candidate, nil
}
