// Package procedures reads Markdown procedures (operating instructions
// with commands) from a Git repository or a folder, so that their commands
// can be pasted or run in a terminal without leaving Shellvane. Files are
// only read: nothing is ever written back to the source.
package procedures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/sync"
)

// Error codes returned by this package.
const (
	ErrSource   errs.Code = "procedures.source"
	ErrNotFound errs.Code = "procedures.notFound"
	ErrTooLarge errs.Code = "procedures.tooLarge"
)

// Source is where procedures are kept.
type Source struct {
	// Kind is "git" (a repository, cloned under the data folder) or
	// "folder" (a folder of this computer).
	Kind string `json:"kind"`
	// URL and Branch of a repository ("" for its default branch).
	URL    string `json:"url,omitempty"`
	Branch string `json:"branch,omitempty"`
	// Folder is the local folder, or the folder inside the repository
	// that holds the procedures ("" for all of it).
	Folder string `json:"folder,omitempty"`
}

// Doc is a procedure: a Markdown file.
type Doc struct {
	// Path is relative to the source's folder, with forward slashes.
	Path  string `json:"path"`
	Title string `json:"title"`
}

// Limits on what is read.
const (
	maxDocs     = 2000
	maxDocBytes = 2 << 20
)

// Library keeps the clones of repositories under Dir.
type Library struct {
	Dir string
	// Git runs git; tests replace it.
	Git func(ctx context.Context, dir string, args ...string) (string, error)
}

func (l Library) git(ctx context.Context, dir string, args ...string) (string, error) {
	if l.Git != nil {
		return l.Git(ctx, dir, args...)
	}
	return sync.GitStore{}.Run(ctx, dir, args...)
}

// clone is the local clone of a repository source.
func (l Library) clone(src Source) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(src.URL) + "\x00" + strings.TrimSpace(src.Branch)))
	return filepath.Join(l.Dir, hex.EncodeToString(sum[:8]))
}

// Root is the folder that holds the source's procedures.
func (l Library) Root(src Source) (string, error) {
	switch src.Kind {
	case "folder":
		dir := strings.TrimSpace(src.Folder)
		if strings.HasPrefix(dir, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				dir = filepath.Join(home, dir[1:])
			}
		}
		if dir == "" || !filepath.IsAbs(dir) {
			return "", errs.New(ErrSource, errs.Params{"detail": "folder"})
		}
		return filepath.Clean(dir), nil
	case "git":
		if strings.TrimSpace(src.URL) == "" {
			return "", errs.New(ErrSource, errs.Params{"detail": "url"})
		}
		sub := filepath.Clean("/" + filepath.FromSlash(strings.TrimSpace(src.Folder)))
		return filepath.Join(l.clone(src), sub), nil
	}
	return "", errs.New(ErrSource, errs.Params{"detail": "kind"})
}

// Sync clones a repository the first time and brings it up to date
// afterwards (a shallow copy of the branch: local changes are dropped,
// since the clone is only read). A folder needs nothing.
func (l Library) Sync(ctx context.Context, src Source) error {
	if src.Kind != "git" {
		_, err := l.Root(src)
		return err
	}
	if _, err := l.Root(src); err != nil {
		return err
	}
	dir := l.clone(src)
	branch := strings.TrimSpace(src.Branch)
	if _, err := os.Stat(filepath.Join(dir, ".git")); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(l.Dir, 0o700); err != nil {
			return err
		}
		args := []string{"clone", "--quiet", "--depth", "1"}
		if branch != "" {
			args = append(args, "--branch", branch)
		}
		os.RemoveAll(dir) // a clone interrupted earlier
		_, err := l.git(ctx, l.Dir, append(args, "--", strings.TrimSpace(src.URL), dir)...)
		return err
	}
	ref := branch
	if ref == "" {
		ref = "HEAD"
	}
	if _, err := l.git(ctx, dir, "fetch", "--quiet", "--depth", "1", "origin", ref); err != nil {
		return err
	}
	_, err := l.git(ctx, dir, "reset", "--quiet", "--hard", "FETCH_HEAD")
	return err
}

// Synced is when a repository was last brought up to date (zero when it
// was never cloned); for a folder, now.
func (l Library) Synced(src Source) time.Time {
	if src.Kind != "git" {
		return time.Now()
	}
	st, err := os.Stat(filepath.Join(l.clone(src), ".git", "FETCH_HEAD"))
	if err != nil {
		st, err = os.Stat(filepath.Join(l.clone(src), ".git", "HEAD"))
	}
	if err != nil {
		return time.Time{}
	}
	return st.ModTime()
}

// List returns the procedures of a source, sorted by path.
func (l Library) List(src Source) ([]Doc, error) {
	root, err := l.Root(src)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root); err != nil {
		return nil, errs.Wrap(err, ErrNotFound, errs.Params{"path": root})
	}
	docs := []Doc{}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !isMarkdown(name) || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		docs = append(docs, Doc{Path: filepath.ToSlash(rel), Title: title(p, name)})
		if len(docs) >= maxDocs {
			return fs.SkipAll
		}
		return nil
	})
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs, err
}

// Read returns a procedure. The path must stay inside the source's folder,
// symbolic links included.
func (l Library) Read(src Source, path string) (string, error) {
	root, err := l.Root(src)
	if err != nil {
		return "", err
	}
	p, err := inside(root, path)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(p)
	if err != nil || st.IsDir() || !isMarkdown(p) {
		return "", errs.New(ErrNotFound, errs.Params{"path": path})
	}
	if st.Size() > maxDocBytes {
		return "", errs.New(ErrTooLarge, errs.Params{"path": path})
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", errs.Wrap(err, ErrNotFound, errs.Params{"path": path})
	}
	return string(b), nil
}

// inside resolves path under root, refusing anything that leads out of it.
func inside(root, path string) (string, error) {
	p := filepath.Join(root, filepath.FromSlash(path))
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", errs.Wrap(err, ErrNotFound, errs.Params{"path": path})
	}
	real, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", errs.Wrap(err, ErrNotFound, errs.Params{"path": path})
	}
	if rel, err := filepath.Rel(realRoot, real); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errs.New(ErrNotFound, errs.Params{"path": path})
	}
	return real, nil
}

func isMarkdown(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".markdown", ".mdx":
		return true
	}
	return false
}

// title is the first heading of a file, or its name.
func title(path, name string) string {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		buf := make([]byte, 4096)
		n, _ := f.Read(buf)
		for _, line := range strings.Split(string(buf[:n]), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "# ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}
