// Package shellenv completes the PATH (and the proxy settings) of a desktop
// app with the ones of the user's login shell. Apps started from the macOS Finder or Dock (and from
// some Linux launchers) get a minimal PATH without /opt/homebrew/bin,
// ~/.local/bin and the like, so tools such as kubectl, gcloud or aws and
// their credential plugins are not found.
package shellenv

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const marker = "__SHELLVANE_PATH__"

// Timeout bounds the login shell run.
const Timeout = 15 * time.Second // heavy .zshrc files (oh-my-zsh, nvm) take a few seconds

// Imported are the variables, besides PATH, taken from the login shell
// when the app lacks them: proxies (corporate networks often set them only
// in ~/.zshrc) and the certificate bundles of TLS-inspecting proxies, so
// git, gcloud, kubectl and Shellvane itself reach the network like the
// user's terminal does.
var Imported = []string{
	"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy",
	"SSL_CERT_FILE", "SSL_CERT_DIR", "GIT_SSL_CAINFO", "CURL_CA_BUNDLE", "REQUESTS_CA_BUNDLE", "NODE_EXTRA_CA_CERTS",
}

// LoginPath returns the PATH set up by the user's login (and interactive)
// shell, or "" when it cannot be read.
func LoginPath(ctx context.Context) string {
	return LoginEnv(ctx)["PATH"]
}

// LoginEnv returns PATH and the Imported variables set by the user's login
// (and interactive) shell; empty when the shell cannot be run.
func LoginEnv(ctx context.Context) map[string]string {
	shell := UserShell()
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	// -i reads ~/.zshrc / ~/.bashrc too, where PATH is often extended; the
	// markers isolate the values from anything the rc files print.
	script := "printf '%s' " + marker + "; printf 'PATH=%s\\n' \"$PATH\""
	for _, name := range Imported {
		script += fmt.Sprintf("; if [ -n \"${%[1]s+x}\" ]; then printf '%[1]s=%%s\\n' \"$%[1]s\"; fi", name)
	}
	script += "; printf '%s' " + marker
	cmd := exec.CommandContext(ctx, shell, "-ilc", script)
	cmd.Stdin = nil
	cmd.Env = append(os.Environ(), "SHELLVANE_SHELLENV=1")
	// Background processes started by the startup files keep the output
	// open: once the shell exits, stop waiting for them.
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return map[string]string{}
	}
	return ParseEnv(Extract(out))
}

// ParseEnv reads NAME=value lines.
func ParseEnv(text string) map[string]string {
	env := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && k != "" {
			env[k] = strings.TrimSpace(v)
		}
	}
	return env
}

// UserShell is the user's login shell. Apps opened from the macOS Dock may
// not get $SHELL, so the account's shell is read from the system then:
// dscl on macOS, the passwd database elsewhere.
func UserShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	name := os.Getenv("USER")
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	if runtime.GOOS == "darwin" && name != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if out, err := exec.CommandContext(ctx, "/usr/bin/dscl", ".", "-read", "/Users/"+name, "UserShell").Output(); err == nil {
			if s := parseDscl(string(out)); s != "" {
				return s
			}
		}
		return "/bin/zsh" // macOS's default since 10.15
	}
	if data, err := os.ReadFile("/etc/passwd"); err == nil && name != "" {
		if s := passwdShell(string(data), name); s != "" {
			return s
		}
	}
	return "/bin/sh"
}

// parseDscl reads "UserShell: /bin/zsh".
func parseDscl(out string) string {
	_, v, ok := strings.Cut(out, "UserShell:")
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

// passwdShell finds the shell of name in /etc/passwd text.
func passwdShell(data, name string) string {
	for _, line := range strings.Split(data, "\n") {
		f := strings.Split(line, ":")
		if len(f) >= 7 && f[0] == name {
			return strings.TrimSpace(f[6])
		}
	}
	return ""
}

// Extract reads the PATH between the markers.
func Extract(out []byte) string {
	start := bytes.Index(out, []byte(marker))
	if start < 0 {
		return ""
	}
	rest := out[start+len(marker):]
	end := bytes.Index(rest, []byte(marker))
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(string(rest[:end]))
}

// Merge returns current followed by the entries of extra it lacks.
func Merge(current, extra string) string {
	sep := string(filepath.ListSeparator)
	seen := map[string]bool{}
	var out []string
	for _, list := range []string{current, extra} {
		for _, p := range strings.Split(list, sep) {
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return strings.Join(out, sep)
}

// Complete gives the process the login shell's PATH (its entries first, in
// its order, as in the user's terminal) and the Imported variables it does
// not have. It does nothing on Windows, where apps get the user's full
// environment.
func Complete(ctx context.Context) {
	defer finished.Do(func() { close(done) })
	if runtime.GOOS == "windows" {
		return
	}
	env := LoginEnv(ctx)
	if p := env["PATH"]; p != "" {
		os.Setenv("PATH", Merge(p, os.Getenv("PATH")))
	}
	for _, name := range Imported {
		if v, ok := env[name]; ok && v != "" {
			if _, set := os.LookupEnv(name); !set {
				os.Setenv(name, v)
			}
		}
	}
}

var (
	done     = make(chan struct{})
	finished sync.Once
)

// Wait waits, at most until ctx ends or Timeout passes, for Complete to
// finish: commands that need the network (git) run with the proxies set.
// It returns at once when Complete is not running.
func Wait(ctx context.Context) {
	if !started.Load() {
		return
	}
	t := time.NewTimer(Timeout)
	defer t.Stop()
	select {
	case <-done:
	case <-ctx.Done():
	case <-t.C:
	}
}

var started atomic.Bool

// Start runs Complete in the background; Wait waits for it.
func Start(ctx context.Context) {
	if started.Swap(true) {
		return
	}
	go Complete(ctx)
}
