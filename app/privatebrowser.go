package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Browsers for sign-in pages. Tools that open the browser with Python's
// webbrowser module (gcloud, az, aws sso) read the BROWSER variable, so the
// login terminal can choose the browser and a private (incognito) window.
const (
	BrowserAuto   = ""       // a private-capable browser when private, else the system's
	BrowserCustom = "custom" // the user's own command, %s for the link
)

type browserInfo struct {
	id      string
	macApp  string   // under /Applications
	winPath string   // under Program Files or LocalAppData
	linux   []string // commands on the PATH
	private string   // flag of a private window; "" when there is none
}

// browsers in the order tried by BrowserAuto.
var browsers = []browserInfo{
	{"chrome", "Google Chrome", `Google\Chrome\Application\chrome.exe`, []string{"google-chrome", "google-chrome-stable"}, "--incognito"},
	{"edge", "Microsoft Edge", `Microsoft\Edge\Application\msedge.exe`, []string{"microsoft-edge", "microsoft-edge-stable"}, "--inprivate"},
	{"brave", "Brave Browser", `BraveSoftware\Brave-Browser\Application\brave.exe`, []string{"brave-browser", "brave"}, "--incognito"},
	{"chromium", "Chromium", "", []string{"chromium", "chromium-browser"}, "--incognito"},
	{"firefox", "Firefox", `Mozilla Firefox\firefox.exe`, []string{"firefox"}, "-private-window"},
	{"safari", "Safari", "", nil, ""},
}

// BrowserChoice is how the login terminal opens sign-in pages.
type BrowserChoice struct {
	// ID is BrowserAuto, BrowserCustom or a browser of the list.
	ID string `json:"id,omitempty"`
	// Command is the custom command (BrowserCustom), %s for the link.
	Command string `json:"command,omitempty"`
	// Private opens a private (incognito) window where the browser can.
	Private bool `json:"private,omitempty"`
}

// browserEnv is the BROWSER value for c, or "" to leave the system's
// browser.
func browserEnv(c BrowserChoice) string {
	exists := func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}
	return browserCommand(c, runtime.GOOS, exists, exec.LookPath, os.Getenv)
}

// browserCommand builds the command; exists, lookPath and getenv are
// replaced in tests. Python reads BROWSER as a list separated by
// os.pathsep (":" on macOS and Linux, ";" on Windows) and splits each
// entry like a shell, replacing %s with the link: the commands contain
// neither separator and use forward slashes.
func browserCommand(c BrowserChoice, goos string, exists func(string) bool, lookPath func(string) (string, error), getenv func(string) string) string {
	switch c.ID {
	case BrowserCustom:
		cmd := strings.TrimSpace(c.Command)
		if cmd != "" && !strings.Contains(cmd, "%s") {
			cmd += " %s"
		}
		return cmd
	case BrowserAuto:
		if !c.Private {
			return "" // the system's browser
		}
		for _, b := range browsers {
			if b.private == "" {
				continue
			}
			if cmd := browserLaunch(b, true, goos, exists, lookPath, getenv); cmd != "" {
				return cmd
			}
		}
		return ""
	}
	for _, b := range browsers {
		if b.id == c.ID {
			if cmd := browserLaunch(b, c.Private, goos, exists, lookPath, getenv); cmd != "" {
				return cmd
			}
		}
	}
	// Not installed here: fall back to the automatic choice.
	return browserCommand(BrowserChoice{Private: c.Private}, goos, exists, lookPath, getenv)
}

// browserLaunch is the command that opens b on goos, "" when it is not installed.
func browserLaunch(b browserInfo, private bool, goos string, exists func(string) bool, lookPath func(string) (string, error), getenv func(string) string) string {
	flag := ""
	if private && b.private != "" {
		flag = " " + b.private
	}
	switch goos {
	case "darwin":
		if b.macApp == "" {
			return ""
		}
		home := getenv("HOME")
		if exists("/Applications/"+b.macApp+".app") || (home != "" && exists(filepath.Join(home, "Applications", b.macApp+".app"))) ||
			(b.id == "safari" && exists("/System/Applications/Safari.app")) {
			if flag == "" {
				return `open -a "` + b.macApp + `" %s`
			}
			return `open -na "` + b.macApp + `" --args` + flag + ` %s`
		}
	case "windows":
		if b.winPath == "" {
			return ""
		}
		for _, v := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
			d := getenv(v)
			if d == "" {
				continue
			}
			if p := d + `\` + b.winPath; exists(p) {
				return `"` + strings.ReplaceAll(p, `\`, "/") + `"` + flag + ` %s`
			}
		}
	default:
		for _, name := range b.linux {
			if p, err := lookPath(name); err == nil && !strings.Contains(p, ":") {
				return p + flag + " %s"
			}
		}
	}
	return ""
}
