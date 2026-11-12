package plugins

import (
	"context"

	"github.com/csturiale/shellvane/core/errs"
)

// Asker asks the user a question while a credential plugin works, e.g. a
// one-time code its server wants; secret hides the answer as it is typed.
type Asker func(ctx context.Context, title, question string, secret bool) (string, error)

type askerKey struct{}

// WithAsker lets the plugins called with ctx ask the user.
func WithAsker(ctx context.Context, a Asker) context.Context {
	return context.WithValue(ctx, askerKey{}, a)
}

// Ask asks the user through the ctx's Asker; without one (a refresh in
// the background) it fails with ErrUnsupported.
func Ask(ctx context.Context, title, question string, secret bool) (string, error) {
	a, _ := ctx.Value(askerKey{}).(Asker)
	if a == nil {
		return "", errs.New(ErrUnsupported, nil)
	}
	return a(ctx, title, question, secret)
}

// WebLogin opens url in a window of the app, for a sign-in that happens
// in a browser (SAML, OpenID Connect, MFA). probe is JavaScript run in the
// window's pages again and again; to report what it found it navigates to
// the address in the variable __SHELLVANE_RETURN__ with the values in the
// query string. accept checks them (e.g. with the server): true ends the
// sign-in and closes the window, false sends the page back to keep going.
type WebLogin func(ctx context.Context, title, url, probe string, accept func(values map[string]string) bool) error

type webLoginKey struct{}

// WithWebLogin lets the plugins called with ctx open a sign-in window.
func WithWebLogin(ctx context.Context, w WebLogin) context.Context {
	return context.WithValue(ctx, webLoginKey{}, w)
}

// OpenWebLogin runs a browser sign-in through the ctx's WebLogin; without
// one (no window can open) it fails with ErrUnsupported.
func OpenWebLogin(ctx context.Context, title, url, probe string, accept func(map[string]string) bool) error {
	w, _ := ctx.Value(webLoginKey{}).(WebLogin)
	if w == nil {
		return errs.New(ErrUnsupported, nil)
	}
	return w(ctx, title, url, probe, accept)
}

// BrowserLogin opens url in the user's own browser (browser: an id such
// as "safari" or "edge", "" for the system's default), for a sign-in that a
// window of the app cannot do (an identity provider's conditional access
// that only trusts the managed browsers). The page cannot be probed from
// the app: what it finds comes back from the "Send to Shellvane"
// bookmarklet, run by the user once signed in, and goes through accept as
// for WebLogin; true ends the sign-in.
type BrowserLogin func(ctx context.Context, title, url, browser string, accept func(values map[string]string) bool) error

type browserLoginKey struct{}

// WithBrowserLogin lets the plugins called with ctx sign in in the browser.
func WithBrowserLogin(ctx context.Context, b BrowserLogin) context.Context {
	return context.WithValue(ctx, browserLoginKey{}, b)
}

// OpenBrowserLogin runs a sign-in in the user's browser through the ctx's
// BrowserLogin; without one it fails with ErrUnsupported.
func OpenBrowserLogin(ctx context.Context, title, url, browser string, accept func(map[string]string) bool) error {
	b, _ := ctx.Value(browserLoginKey{}).(BrowserLogin)
	if b == nil {
		return errs.New(ErrUnsupported, nil)
	}
	return b(ctx, title, url, browser, accept)
}
