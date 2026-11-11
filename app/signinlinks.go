package app

import (
	"log"
	"net/url"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Sign-in links printed in a terminal (e.g. by `gcloud auth login
// --no-launch-browser`, `az login --use-device-code`, `kubelogin`,
// `vault login -method=oidc`) can be opened by Shellvane in the browser
// chosen for sign-in pages, for tools that do not read BROWSER.

var (
	linkPattern = regexp.MustCompile(`https://[^\s"'<>\x60\x1b]+`)
	ansiPattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)
)

// maxOpenedLinks bounds how many links one terminal opens.
const maxOpenedLinks = 5

// isSignInLink tells a sign-in page (OAuth, OpenID Connect, SAML, device
// code) from other links: its query names a client or its path signs in.
func isSignInLink(raw string) bool {
	u, err := url.Parse(strings.TrimRight(raw, ".,;)]"))
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || host == "127.0.0.1" {
		return false
	}
	q := u.Query()
	if q.Has("client_id") || q.Has("code_challenge") || q.Has("SAMLRequest") || q.Has("user_code") {
		return true
	}
	path := strings.ToLower(u.Path)
	for _, p := range []string{"/oauth", "/authorize", "/device", "/signin", "/sign-in", "/login", "/sso", "/saml", "/auth"} {
		if strings.Contains(path, p) {
			return true
		}
	}
	return false
}

// linkWatcher finds sign-in links in a terminal's output and opens each
// once.
type linkWatcher struct {
	mu      sync.Mutex
	partial []byte
	seen    map[string]bool
	open    func(string)
}

func newLinkWatcher(open func(string)) *linkWatcher {
	return &linkWatcher{seen: map[string]bool{}, open: open}
}

// feed reads output; a link is looked for once its line is complete.
func (w *linkWatcher) feed(frame []byte) {
	w.mu.Lock()
	w.partial = append(w.partial, frame...)
	var lines []string
	for {
		i := strings.IndexAny(string(w.partial), "\r\n")
		if i < 0 {
			break
		}
		lines = append(lines, string(w.partial[:i]))
		w.partial = w.partial[i+1:]
	}
	if len(w.partial) > 16*1024 {
		w.partial = w.partial[len(w.partial)-16*1024:]
	}
	var found []string
	for _, line := range lines {
		for _, l := range linkPattern.FindAllString(ansiPattern.ReplaceAllString(line, ""), -1) {
			l = strings.TrimRight(l, ".,;)]")
			if !w.seen[l] && len(w.seen) < maxOpenedLinks && isSignInLink(l) {
				w.seen[l] = true
				found = append(found, l)
			}
		}
	}
	w.mu.Unlock()
	for _, l := range found {
		w.open(l)
	}
}

// openLink opens a link with the chosen browser, or the system's.
func openLink(c BrowserChoice, link string) {
	if cmd := browserEnv(c); cmd != "" {
		args := splitCommand(cmd)
		for i := range args {
			args[i] = strings.ReplaceAll(args[i], "%s", link)
		}
		if len(args) > 0 {
			p := exec.Command(args[0], args[1:]...)
			if err := p.Start(); err == nil {
				go p.Wait()
				return
			}
			log.Printf("open sign-in link: %v", args[0])
		}
	}
	if app := application.Get(); app != nil && app.Browser != nil {
		app.Browser.OpenURL(link)
	}
}

// splitCommand splits a command line like Python's shlex (as BROWSER is
// read): spaces separate words, quotes group them.
func splitCommand(s string) []string {
	var out []string
	var cur strings.Builder
	quote := rune(0)
	inWord := false
	for _, r := range s {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote != 0:
			cur.WriteRune(r)
		case r == '"' || r == '\'':
			quote, inWord = r, true
		case r == ' ' || r == '\t':
			if inWord {
				out = append(out, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		out = append(out, cur.String())
	}
	return out
}
