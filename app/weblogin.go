package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html"
	"log"
	"net"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/sshengine"
)

// signInUserAgentSuffix ends the sign-in window's user agent on macOS,
// after "AppleWebKit/605.1.15 (KHTML, like Gecko)", as Safari's does.
const signInUserAgentSuffix = "Version/18.5 Safari/605.1.15"

// samlFormHook runs at the start of every page of a sign-in window: a form
// carrying a SAML response (the identity provider sending it on) goes to
// the app's address instead, which uses it or posts it on to where it was
// going (__action). Not on the app's own pages, which post it on.
//
// With fields (__SHELLVANE_SAML_FIELDS__, plugins.WithSAMLFields) the form
// keeps its way: the fields are added and the page posts it itself.
const samlFormHook = `(function () {
  if (location.href.indexOf(__SHELLVANE_RETURN__) === 0) return;
  window.__shellvaneHooked = true;
  function grab(form) {
    var s = form && form.querySelector && form.querySelector('input[name="SAMLResponse"]');
    if (!s || !s.value) return false;
    if (__SHELLVANE_SAML_FIELDS__) {
      for (var k in __SHELLVANE_SAML_FIELDS__) {
        if (form.querySelector('input[name="' + k + '"]')) continue;
        var i = document.createElement('input');
        i.type = 'hidden';
        i.name = k;
        i.value = __SHELLVANE_SAML_FIELDS__[k];
        form.appendChild(i);
      }
      return false;
    }
    var q = 'SAMLResponse=' + encodeURIComponent(s.value) + '&__action=' + encodeURIComponent(form.action || '');
    var relay = form.querySelector('input[name="RelayState"]');
    if (relay) q += '&RelayState=' + encodeURIComponent(relay.value);
    location.href = __SHELLVANE_RETURN__ + '?' + q;
    return true;
  }
  var submit = HTMLFormElement.prototype.submit;
  HTMLFormElement.prototype.submit = function () {
    if (!grab(this)) return submit.apply(this, arguments);
  };
  document.addEventListener('submit', function (e) {
    if (grab(e.target)) e.preventDefault();
  }, true);
})();`

// startScript is the script run at the start of every page of a sign-in
// window: the SAML form hook, with the app's address and the fields to
// add, and the caller's probe, run once the page is there and then every
// 300 ms. The probe cannot go through the window's ExecJS: Wails queues
// that until its own runtime has loaded, which never happens on another
// site's pages.
func startScript(ret string, fields map[string]string, probe string) string {
	f := []byte("null")
	if len(fields) > 0 {
		f, _ = json.Marshal(fields)
	}
	s := "var __SHELLVANE_RETURN__ = " + ret + ";\nvar __SHELLVANE_SAML_FIELDS__ = " + string(f) + ";\n" + samlFormHook
	if strings.TrimSpace(probe) != "" {
		s += "\n(function () {\n  function probe() {\n    try {\n" + probe + "\n    } catch (e) {}\n  }\n" +
			"  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', probe); else probe();\n" +
			"  setInterval(probe, 300);\n})();"
	}
	return s
}

// pageValues is what a page that is only a token, or a service's JSON error,
// reports: {"token": …} or {"__pvwaError": "code message"}; nil for any
// other page.
func pageValues(text string) map[string]string {
	t := strings.TrimSpace(text)
	if len(t) < 8192 && tokenPage.MatchString(t) {
		return map[string]string{"token": strings.Trim(t, `"`)}
	}
	if strings.HasPrefix(t, "{") && strings.Contains(t, `"ErrorCode"`) {
		var e struct{ ErrorCode, ErrorMessage string }
		if json.Unmarshal([]byte(t), &e) == nil && e.ErrorCode != "" {
			return map[string]string{"__pvwaError": strings.TrimSpace(e.ErrorCode + " " + e.ErrorMessage)}
		}
	}
	return nil
}

var tokenPage = regexp.MustCompile(`^"?[A-Za-z0-9+/=._-]{40,}"?$`)

// signInRedirectPage is a page that only goes to startURL.
func signInRedirectPage(startURL string) string {
	u, _ := json.Marshal(startURL)
	return `<!doctype html><meta charset="utf-8"><script>location.replace(` + string(u) + `)</script>`
}

// webLoginTimeout bounds a browser sign-in.
const webLoginTimeout = 5 * time.Minute

// WebLoginWindow is plugins.WebLogin with a window of the app: the page
// signs in as in a browser; a server on 127.0.0.1, at a random path, gets
// what the probe finds. Nothing is kept once the sign-in ends.
func WebLoginWindow(ctx context.Context, title, startURL, probe string, accept func(map[string]string) bool) error {
	a := application.Get()
	if a == nil {
		return errs.New(plugins.ErrUnsupported, nil)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return errs.Wrap(err, plugins.ErrCommand, errs.Params{"command": "sign-in", "detail": err.Error()})
	}
	secret := make([]byte, 16)
	rand.Read(secret)
	path := "/" + hex.EncodeToString(secret)
	back := "http://" + ln.Addr().String() + path

	ctx, cancel := context.WithTimeout(ctx, webLoginTimeout)
	defer cancel()
	done := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(done) }) }
	var mu sync.Mutex // accept runs for one report at a time

	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		r.ParseForm()
		values := map[string]string{}
		var keys []string
		for k, v := range r.Form {
			if len(v) > 0 {
				values[k] = v[0]
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		log.Printf("sign-in window %q reported: %s (names only)", title, strings.Join(keys, ", "))
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		mu.Lock()
		ok := accept(values)
		mu.Unlock()
		if ok {
			w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>` + html.EscapeString(title) + `</title><body style="font:15px system-ui;padding:40px">` + html.EscapeString(title) + ` ✓</body>`))
			finish()
			return
		}
		// Refused for good (the caller has the reason): the window closes.
		if values[plugins.WebLoginStop] != "" {
			w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>` + html.EscapeString(title) + `</title>`))
			finish()
			return
		}
		// A form the probe stopped (a SAML response on its way): posted on to
		// where it was going, so the sign-in in the page goes on.
		if action := values["__action"]; strings.HasPrefix(action, "https://") || strings.HasPrefix(action, "http://") {
			var b strings.Builder
			b.WriteString(`<!doctype html><form method="POST" action="` + html.EscapeString(action) + `">`)
			for k, v := range values {
				if !strings.HasPrefix(k, "__") {
					b.WriteString(`<input type="hidden" name="` + html.EscapeString(k) + `" value="` + html.EscapeString(v) + `">`)
				}
			}
			b.WriteString(`</form><script>document.forms[0].submit()</script>`)
			w.Write([]byte(b.String()))
			return
		}
		// Not signed in yet: back to the page, which goes on.
		w.Write([]byte(`<!doctype html><script>history.back()</script>`))
	})}
	go srv.Serve(ln)
	defer srv.Close()

	ret, _ := json.Marshal(back)
	startHook := startScript(string(ret), plugins.SAMLFields(ctx), probe)
	opts := application.WebviewWindowOptions{
		Title:  title,
		Width:  520,
		Height: 720,
		URL:    startURL,
		// macOS: WKWebView's user agent ends with the app's name (by
		// default "wails.io"), which identity providers do not know as a
		// browser; it ends like Safari's instead, which this WebKit is.
		Mac: application.MacWindow{WebviewPreferences: application.MacWebviewPreferences{ApplicationNameForUserAgent: signInUserAgentSuffix}},
	}
	if runtime.GOOS == "windows" {
		// WebView2 runs a window's JS at the start of every document
		// (AddScriptToExecuteOnDocumentCreated) only for a window opened on
		// its own HTML: a page that goes straight to the sign-in.
		opts.URL = ""
		opts.HTML = signInRedirectPage(startURL)
		opts.JS = startHook
	}
	win := a.Window.NewWithOptions(opts)
	closed := make(chan struct{})
	var closeOnce sync.Once
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		closeOnce.Do(func() { close(closed) })
	})
	defer func() {
		select {
		case <-closed:
		default:
			win.Close()
		}
	}()
	win.Show()
	win.Focus()

	lastPage := ""
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	// An identity provider's page posts its SAML response as soon as it
	// loads, faster than the probe's ticks: a script at the start of every
	// page catches the post instead (where the web view allows it).
	hooked := runtime.GOOS == "windows" // set with the window's options there
	for {
		select {
		case <-done:
			return nil
		case <-closed:
			return errs.New(sshengine.ErrCancelled, nil)
		case <-ctx.Done():
			if strings.Contains(ctx.Err().Error(), "deadline") {
				return errs.New(plugins.ErrCommand, errs.Params{"command": "sign-in", "detail": "timed out"})
			}
			return errs.Wrap(ctx.Err(), sshengine.ErrCancelled, nil)
		case <-tick.C:
			if !hooked {
				if hooked = addDocumentStartScript(win, startHook); hooked {
					log.Printf("sign-in window %q: start script installed", title)
				}
			}
			// A page that is only a token (or a service's error), read
			// natively where possible: it does not depend on the probe.
			select {
			case <-closed:
				return errs.New(sshengine.ErrCancelled, nil) // never read a closed window
			default:
			}
			if _, text, ok := readPage(win); ok && text != lastPage {
				lastPage = text
				if values := pageValues(text); values != nil {
					log.Printf("sign-in window %q: page read, it holds a token or an error", title)
					mu.Lock()
					accepted := accept(values)
					mu.Unlock()
					if accepted || values[plugins.WebLoginStop] != "" {
						return nil
					}
				}
			}
		}
	}
}
