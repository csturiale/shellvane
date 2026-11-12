package app

import (
	"errors"
	"testing"
)

func TestBrowserCommand(t *testing.T) {
	has := func(paths ...string) func(string) bool {
		return func(p string) bool {
			for _, x := range paths {
				if x == p {
					return true
				}
			}
			return false
		}
	}
	env := func(k string) string {
		return map[string]string{"HOME": "/Users/a", "ProgramFiles": `C:\Program Files`, "ProgramFiles(x86)": `C:\Program Files (x86)`}[k]
	}
	none := func(string) (string, error) { return "", errors.New("no") }
	mac := has("/Users/a/Applications/Firefox.app", "/Applications/Microsoft Edge.app", "/System/Applications/Safari.app")

	for _, c := range []struct {
		choice BrowserChoice
		goos   string
		exists func(string) bool
		want   string
	}{
		{BrowserChoice{Private: true}, "darwin", mac, `open -na "Microsoft Edge" --args --inprivate %s`},
		{BrowserChoice{}, "darwin", mac, ""},
		{BrowserChoice{ID: "firefox", Private: true}, "darwin", mac, `open -na "Firefox" --args -private-window %s`},
		{BrowserChoice{ID: "firefox"}, "darwin", mac, `open -a "Firefox" %s`},
		{BrowserChoice{ID: "safari", Private: true}, "darwin", mac, `open -a "Safari" %s`},
		{BrowserChoice{ID: "chrome", Private: true}, "darwin", mac, `open -na "Microsoft Edge" --args --inprivate %s`}, // not installed: automatic
		{BrowserChoice{ID: BrowserCustom, Command: "/opt/bin/work-browser --profile=sso"}, "darwin", mac, "/opt/bin/work-browser --profile=sso %s"},
		{BrowserChoice{ID: BrowserCustom, Command: "x %s --new"}, "darwin", mac, "x %s --new"},
		{BrowserChoice{Private: true}, "windows", has(`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`), `"C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" --inprivate %s`},
		{BrowserChoice{Private: true}, "darwin", has(), ""},
	} {
		if got := browserCommand(c.choice, c.goos, c.exists, none, env, func() string { return "" }); got != c.want {
			t.Errorf("%s %+v = %q, want %q", c.goos, c.choice, got, c.want)
		}
	}
	look := func(c string) (string, error) {
		if c == "firefox" {
			return "/usr/bin/firefox", nil
		}
		return "", errors.New("no")
	}
	if got := browserCommand(BrowserChoice{Private: true}, "linux", has(), look, env, func() string { return "" }); got != "/usr/bin/firefox -private-window %s" {
		t.Errorf("Linux = %q", got)
	}
}

func TestDefaultBrowser(t *testing.T) {
	plist := `{"LSHandlers":[{"LSHandlerURLScheme":"http","LSHandlerRoleAll":"com.google.chrome"},{"LSHandlerURLScheme":"https","LSHandlerRoleAll":"com.brave.browser"}]}`
	run := func(out string) func(string, ...string) (string, error) {
		return func(string, ...string) (string, error) { return out, nil }
	}
	if got := defaultBrowser("darwin", run(plist)); got != "brave" {
		t.Errorf("macOS = %q", got)
	}
	reg := "\r\nHKEY_CURRENT_USER\\...\\UserChoice\r\n    ProgId    REG_SZ    FirefoxURL-308046B0AF4A39CB\r\n"
	if got := defaultBrowser("windows", run(reg)); got != "firefox" {
		t.Errorf("Windows = %q", got)
	}
	if got := defaultBrowser("linux", run("brave-browser.desktop\n")); got != "brave" {
		t.Errorf("Linux = %q", got)
	}
	if got := defaultBrowser("linux", run("opera.desktop\n")); got != "" {
		t.Errorf("unknown = %q", got)
	}

	// Automatic and private: the default browser (Brave) rather than the
	// first of the list (Chrome); Safari as default falls back to the list.
	mac := func(p string) bool {
		return p == "/Applications/Google Chrome.app" || p == "/Applications/Brave Browser.app" || p == "/System/Applications/Safari.app"
	}
	none := func(string) (string, error) { return "", errors.New("no") }
	env := func(string) string { return "" }
	if got := browserCommand(BrowserChoice{Private: true}, "darwin", mac, none, env, func() string { return "brave" }); got != `open -na "Brave Browser" --args --incognito %s` {
		t.Errorf("default Brave = %q", got)
	}
	if got := browserCommand(BrowserChoice{Private: true}, "darwin", mac, none, env, func() string { return "safari" }); got != `open -na "Google Chrome" --args --incognito %s` {
		t.Errorf("default Safari = %q", got)
	}
}
