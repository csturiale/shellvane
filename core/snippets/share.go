package snippets

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrImportInvalid is returned for a file with no snippet in it.
const ErrImportInvalid errs.Code = "snippets.importInvalid"

// FileKind marks a Shellvane snippets file.
const FileKind = "shellvane-snippets"

type file struct {
	Kind     string  `json:"kind"`
	Version  int     `json:"version"`
	Snippets []Input `json:"snippets"`
}

// ImportResult tells what an import did.
type ImportResult struct {
	Added int `json:"added"`
	// Skipped counts snippets identical to one already saved.
	Skipped int `json:"skipped"`
}

// Export renders snippets as a Shellvane snippets file (JSON).
func Export(list []Snippet) string {
	f := file{Kind: FileKind, Version: 1, Snippets: []Input{}}
	for _, s := range list {
		f.Snippets = append(f.Snippets, Input{Folder: s.Folder, Name: s.Name, Body: s.Body, Run: s.Run})
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	return string(b) + "\n"
}

// Parse reads a Shellvane snippets file or, for any other text, a shell
// script or list of commands: each command line is a snippet, named by the
// comment just above it (or by the command itself) and filed in a folder
// named after the file. A line ending in a backslash continues on the next.
func Parse(fileName, text string) ([]Input, error) {
	text = strings.TrimPrefix(text, "\ufeff")
	if t := strings.TrimSpace(text); strings.HasPrefix(t, "{") {
		var f file
		if err := json.Unmarshal([]byte(t), &f); err != nil || f.Kind != FileKind || f.Version != 1 {
			return nil, errs.New(ErrImportInvalid, nil)
		}
		var out []Input
		for _, s := range f.Snippets {
			if err := validate(&s); err == nil {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil, errs.New(ErrImportInvalid, nil)
		}
		return out, nil
	}

	folder := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(fileName, `\`, "/")), filepath.Ext(fileName))
	if folder == "." || folder == "/" {
		folder = ""
	}
	var out []Input
	var name string
	var cmd []string
	flush := func() {
		body := strings.Join(cmd, "\n")
		cmd = nil
		if strings.TrimSpace(body) == "" {
			return
		}
		n := name
		if n == "" {
			n = strings.TrimSpace(body)
			if i := strings.IndexByte(n, '\n'); i >= 0 {
				n = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(n[:i]), `\`))
			}
			if r := []rune(n); len(r) > 60 {
				n = string(r[:59]) + "…"
			}
		}
		out = append(out, Input{Folder: folder, Name: n, Body: body, Run: true})
		name = ""
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case len(cmd) > 0:
			cmd = append(cmd, line)
			if !strings.HasSuffix(trimmed, `\`) {
				flush()
			}
		case trimmed == "" || strings.HasPrefix(trimmed, "#!"):
			name = ""
		case strings.HasPrefix(trimmed, "#"):
			name = strings.TrimSpace(strings.Trim(trimmed, "# \t"))
		default:
			cmd = append(cmd, strings.TrimRight(line, " \t"))
			if !strings.HasSuffix(trimmed, `\`) {
				flush()
			}
		}
	}
	flush()
	if len(out) == 0 {
		return nil, errs.New(ErrImportInvalid, nil)
	}
	return out, nil
}

// Import saves the snippets that are not already saved (same folder, name
// and body).
func (r *Repo) Import(ctx context.Context, in []Input) (ImportResult, error) {
	var res ImportResult
	have, err := r.List(ctx)
	if err != nil {
		return res, err
	}
	seen := map[[3]string]bool{}
	for _, s := range have {
		seen[[3]string{s.Folder, s.Name, s.Body}] = true
	}
	for _, s := range in {
		if err := validate(&s); err != nil {
			return res, err
		}
		k := [3]string{s.Folder, s.Name, s.Body}
		if seen[k] {
			res.Skipped++
			continue
		}
		if _, err := r.Create(ctx, s); err != nil {
			return res, err
		}
		seen[k] = true
		res.Added++
	}
	return res, nil
}
