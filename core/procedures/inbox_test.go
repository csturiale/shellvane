package procedures

import (
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

func TestInbox(t *testing.T) {
	l := Library{Dir: t.TempDir()}
	// Before the first page the folder does not exist: the inbox is empty.
	if docs, err := l.List(l.Inbox()); err != nil || len(docs) != 0 {
		t.Fatalf("empty inbox = %v, %v", docs, err)
	}
	if _, err := l.List(Source{Kind: "folder", Folder: filepath.Join(l.Dir, "missing")}); err == nil {
		t.Fatal("another missing folder is still an error")
	}
	d, err := l.SaveInbox("Restart / web (prod)", "https://wiki.example.com/x?a=1", Marker+"\n\nIntro\n\n```sh\nls\n```")
	if err != nil || d.Path != "Restart-web-prod.md" {
		t.Fatalf("save = %+v, %v", d, err)
	}
	text, err := l.Read(l.Inbox(), d.Path)
	if err != nil || !strings.HasPrefix(text, "<!-- source: https://wiki.example.com/x?a=1 -->\n# Restart / web (prod)\n\nIntro") || strings.Contains(text, Marker) {
		t.Fatalf("text = %q, %v", text, err)
	}
	d2, _ := l.SaveInbox("Restart / web (prod)", "javascript:alert(1)", "# Own title\n")
	if d2.Path != "Restart-web-prod-2.md" {
		t.Fatalf("second = %+v", d2)
	}
	if text, _ := l.Read(l.Inbox(), d2.Path); strings.Contains(text, "source:") || !strings.HasPrefix(text, "# Own title") {
		t.Fatalf("second text = %q", text)
	}
	if docs, _ := l.List(l.Inbox()); len(docs) != 2 {
		t.Fatalf("docs = %+v", docs)
	}
	if err := l.DeleteInbox(d2.Path); err != nil {
		t.Fatal(err)
	}
	if err := l.DeleteInbox("../x.md"); err == nil {
		t.Fatal("deleted outside the inbox")
	}
	if _, err := l.SaveInbox("x", "", Marker); errs.CodeOf(err) != ErrLink {
		t.Fatalf("empty page: %v", err)
	}
	for i := 0; i < maxInbox+5; i++ {
		l.SaveInbox("p", "", "text")
	}
	if entries, _ := os.ReadDir(l.InboxDir()); len(entries) != maxInbox {
		t.Fatalf("kept %d pages", len(entries))
	}
	_ = filepath.Join
}

func TestParseLink(t *testing.T) {
	md := Marker + "\n# Hello\n\n```sh\necho ü\n```\n"
	link := "shellvane://procedure?title=" + url.QueryEscape("Hello") + "&url=" + url.QueryEscape("https://wiki/x") + "&md=" + base64.RawURLEncoding.EncodeToString([]byte(md))
	p, err := ParseLink(link, nil)
	if err != nil || p.Title != "Hello" || p.Source != "https://wiki/x" || p.Markdown != md {
		t.Fatalf("inline = %+v, %v", p, err)
	}
	clip := func(s string) func() (string, bool) { return func() (string, bool) { return s, true } }
	if p, err := ParseLink("shellvane://procedure?title=T&clip=1", clip(md)); err != nil || p.Markdown != md {
		t.Fatalf("clipboard = %+v, %v", p, err)
	}
	// Anything else on the clipboard is never taken.
	if _, err := ParseLink("shellvane://procedure?clip=1", clip("my password")); errs.CodeOf(err) != ErrLink {
		t.Fatalf("clipboard without marker: %v", err)
	}
	if _, err := ParseLink("shellvane://procedure?md="+base64.RawURLEncoding.EncodeToString([]byte("no marker")), nil); errs.CodeOf(err) != ErrLink {
		t.Fatalf("link without marker: %v", err)
	}
	if IsLink("ssh://host") || !IsLink("SHELLVANE://procedure?x") {
		t.Fatal("IsLink")
	}
}
