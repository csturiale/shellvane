package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html"
	"net"
	"net/http"
	"runtime"
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
const samlFormHook = `(function () {
  if (location.href.indexOf(__SHELLVANE_RETURN__) === 0) return;
  function grab(form) {
    var s = form && form.querySelector && form.querySelector('input[name="SAMLResponse"]');
    if (!s || !s.value) return false;
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

	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			http.NotFound(w, r)
			return
		}
		r.ParseForm()
		values := map[string]string{}
		for k, v := range r.Form {
			if len(v) > 0 {
				values[k] = v[0]
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if accept(values) {
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
	script := "var __SHELLVANE_RETURN__ = " + string(ret) + ";\n" + probe
	startHook := "var __SHELLVANE_RETURN__ = " + string(ret) + ";\n" + samlFormHook
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
				hooked = addDocumentStartScript(win, startHook)
			}
			win.ExecJS(script)
		}
	}
}
