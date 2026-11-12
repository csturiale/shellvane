package app

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/sshengine"
)

// SignInLinkPrefix starts the links of the "Send to Shellvane" sign-in
// bookmarklet: shellvane://signin?<what the page keeps>.
const SignInLinkPrefix = "shellvane://signin"

// browserLogins holds the sign-in waiting for the bookmarklet's link: one
// at a time, and links that come when none waits are dropped (a page
// cannot push a session the user did not ask for).
var browserLogins struct {
	mu      sync.Mutex
	pending chan map[string]string
}

// IsSignInLink tells whether link comes from the sign-in bookmarklet.
func IsSignInLink(link string) bool {
	return strings.HasPrefix(strings.ToLower(link), SignInLinkPrefix)
}

// ReceiveSignInLink hands a bookmarklet's link to the sign-in waiting for
// it; false when none waits.
func ReceiveSignInLink(link string) bool {
	values, ok := signInValues(link)
	if !ok {
		return false
	}
	browserLogins.mu.Lock()
	ch := browserLogins.pending
	browserLogins.mu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- values:
		return true
	default:
		return false // one is being checked already
	}
}

func signInValues(link string) (map[string]string, bool) {
	if !IsSignInLink(link) {
		return nil, false
	}
	q := ""
	if i := strings.IndexByte(link, '?'); i >= 0 {
		q = link[i+1:]
	}
	parsed, err := url.ParseQuery(q)
	if err != nil {
		return nil, false
	}
	values := map[string]string{}
	for k, v := range parsed {
		if len(v) > 0 && v[0] != "" {
			values[k] = v[0]
		}
	}
	return values, len(values) > 0
}

// openInBrowser opens a page in browser (an id of browsers, "" for the
// system's default); a variable for tests.
var openInBrowser = func(link, browser string) error {
	if browser != "" {
		if argv := browserArgv(browser, runtime.GOOS, link); argv != nil {
			return exec.Command(argv[0], argv[1:]...).Start()
		}
		return errs.New(plugins.ErrSetting, errs.Params{"key": "webLoginIn"})
	}
	a := application.Get()
	if a == nil {
		return errs.New(plugins.ErrUnsupported, nil)
	}
	return a.Browser.OpenURL(link)
}

// browserArgv is the command opening link in the browser with that id,
// nil when it is unknown or not installed.
func browserArgv(id, goos, link string) []string {
	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}
	for _, b := range browsers {
		if b.id == id {
			if cmd := browserLaunch(b, false, goos, exists, exec.LookPath, os.Getenv); cmd != "" {
				args := splitCommand(cmd) // split before the link goes in: it is never parsed
				for i := range args {
					if args[i] == "%s" {
						args[i] = link
					}
				}
				return args
			}
		}
	}
	return nil
}

// BrowserLoginLink is plugins.BrowserLogin: it opens startURL in the
// user's browser and waits for the bookmarklet's link (accept may refuse
// one, e.g. sent before the sign-in ended: the next one is waited for).
// Cancelling ctx (the user skips the password) ends the wait.
func BrowserLoginLink(ctx context.Context, title, startURL, browser string, accept func(map[string]string) bool) error {
	ch := make(chan map[string]string, 1)
	browserLogins.mu.Lock()
	browserLogins.pending = ch
	browserLogins.mu.Unlock()
	defer func() {
		browserLogins.mu.Lock()
		if browserLogins.pending == ch {
			browserLogins.pending = nil
		}
		browserLogins.mu.Unlock()
	}()
	if err := openInBrowser(startURL, browser); err != nil {
		return errs.Wrap(err, plugins.ErrCommand, errs.Params{"command": "sign-in", "detail": err.Error()})
	}
	ctx, cancel := context.WithTimeout(ctx, webLoginTimeout)
	defer cancel()
	for {
		select {
		case values := <-ch:
			if accept(values) {
				return nil
			}
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return errs.New(plugins.ErrCommand, errs.Params{"command": "sign-in", "detail": "timed out after " + webLoginTimeout.String() + " waiting for the bookmarklet"})
			}
			return errs.Wrap(ctx.Err(), sshengine.ErrCancelled, nil)
		}
	}
}
