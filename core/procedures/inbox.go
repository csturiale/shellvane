package procedures

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
)

// Pages sent from a browser ("Send to Shellvane" bookmarklet) or pasted in
// the panel are kept as Markdown files in the inbox, a folder source of
// their own.

// ErrLink: a shellvane://procedure link that is not one Shellvane made.
const ErrLink errs.Code = "procedures.link"

// Marker starts the Markdown sent by the bookmarklet. A link that reads
// the clipboard keeps nothing that does not start with it, so whatever
// else is on the clipboard (a password…) is never saved.
const Marker = "<!-- shellvane-procedure -->"

// maxInbox is how many received pages are kept; the oldest go first.
const maxInbox = 200

// InboxDir is the folder of received pages.
func (l Library) InboxDir() string { return filepath.Join(l.Dir, "inbox") }

// Inbox is the source of received pages.
func (l Library) Inbox() Source { return Source{Kind: "folder", Folder: l.InboxDir()} }

var unsafeName = regexp.MustCompile(`[^\p{L}\p{N}._-]+`)

// SaveInbox keeps a page as a procedure and returns it. The page's address
// is written in a comment at the top, so it can be found again.
func (l Library) SaveInbox(title, source, markdown string) (Doc, error) {
	markdown = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(markdown), Marker))
	if markdown == "" {
		return Doc{}, errs.New(ErrLink, nil)
	}
	if len(markdown) > maxDocBytes {
		return Doc{}, errs.New(ErrTooLarge, errs.Params{"path": title})
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "page"
	}
	dir := l.InboxDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Doc{}, err
	}
	base := strings.Trim(unsafeName.ReplaceAllString(title, "-"), "-.")
	if r := []rune(base); len(r) > 60 {
		base = string(r[:60])
	}
	if base == "" {
		base = "page"
	}
	name := base + ".md"
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, name)); os.IsNotExist(err) {
			break
		}
		name = fmt.Sprintf("%s-%d.md", base, i)
	}
	var b strings.Builder
	if u, err := url.Parse(source); err == nil && (u.Scheme == "https" || u.Scheme == "http") {
		fmt.Fprintf(&b, "<!-- source: %s -->\n", strings.ReplaceAll(u.String(), "--", "%2D%2D"))
	}
	if !strings.HasPrefix(markdown, "# ") {
		fmt.Fprintf(&b, "# %s\n\n", strings.ReplaceAll(title, "\n", " "))
	}
	b.WriteString(markdown)
	b.WriteString("\n")
	if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o600); err != nil {
		return Doc{}, err
	}
	l.pruneInbox()
	return Doc{Path: name, Title: title}, nil
}

// pruneInbox removes the oldest pages beyond maxInbox.
func (l Library) pruneInbox() {
	entries, err := os.ReadDir(l.InboxDir())
	if err != nil || len(entries) <= maxInbox {
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		a, _ := entries[i].Info()
		b, _ := entries[j].Info()
		return a != nil && b != nil && a.ModTime().Before(b.ModTime())
	})
	for _, e := range entries[:len(entries)-maxInbox] {
		os.Remove(filepath.Join(l.InboxDir(), e.Name()))
	}
}

// DeleteInbox removes a received page.
func (l Library) DeleteInbox(path string) error {
	p, err := inside(l.InboxDir(), path)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// IsLink tells a shellvane://procedure link.
func IsLink(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && strings.EqualFold(u.Scheme, "shellvane") && strings.EqualFold(u.Host, "procedure")
}

// Page is what a shellvane://procedure link carries.
type Page struct {
	Title    string
	Source   string
	Markdown string
}

// ParseLink reads a link made by the bookmarklet: the Markdown is in the
// link (md, base64url) or, for a long page, on the clipboard (clip=1),
// read with clipboard. Either way it must start with Marker.
func ParseLink(raw string, clipboard func() (string, bool)) (Page, error) {
	u, err := url.Parse(raw)
	if err != nil || !IsLink(raw) {
		return Page{}, errs.New(ErrLink, nil)
	}
	q := u.Query()
	p := Page{Title: q.Get("title"), Source: q.Get("url")}
	var text string
	switch {
	case q.Get("md") != "":
		enc := strings.TrimRight(q.Get("md"), "=")
		b, err := base64.RawURLEncoding.DecodeString(enc)
		if err != nil {
			if b, err = base64.RawStdEncoding.DecodeString(enc); err != nil {
				return Page{}, errs.Wrap(err, ErrLink, nil)
			}
		}
		text = string(b)
	case q.Get("clip") == "1" && clipboard != nil:
		t, ok := clipboard()
		if !ok {
			return Page{}, errs.New(ErrLink, nil)
		}
		text = t
	default:
		return Page{}, errs.New(ErrLink, nil)
	}
	if !strings.HasPrefix(strings.TrimSpace(text), Marker) {
		return Page{}, errs.New(ErrLink, nil)
	}
	p.Markdown = text
	return p, nil
}
