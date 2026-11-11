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
		if got := browserCommand(c.choice, c.goos, c.exists, none, env); got != c.want {
			t.Errorf("%s %+v = %q, want %q", c.goos, c.choice, got, c.want)
		}
	}
	look := func(c string) (string, error) {
		if c == "firefox" {
			return "/usr/bin/firefox", nil
		}
		return "", errors.New("no")
	}
	if got := browserCommand(BrowserChoice{Private: true}, "linux", has(), look, env); got != "/usr/bin/firefox -private-window %s" {
		t.Errorf("Linux = %q", got)
	}
}
