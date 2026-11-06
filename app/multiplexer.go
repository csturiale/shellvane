package app

import (
	"fmt"
	"strings"

	"github.com/csturiale/shellvane/core/kube"
)

// shellLaunch describes what runs on the server instead of the plain login shell.
type shellLaunch struct {
	mux     string // "tmux" or "screen": attach to the named session
	name    string // tmux/screen session and kubectl file name
	kube    *kube.Selection
	dir     string   // initial directory; "~/x" is under $HOME
	env     []string // NAME=value
	command string   // runs instead of the interactive shell
}

// remoteCommand returns the command run instead of the login shell, or ""
// when the plain shell will do. It sets up the pane's kubectl context,
// exports the environment, changes directory, then runs the command or
// attaches to the named tmux or screen session (created if needed; the
// login shell when the multiplexer is missing). It goes through /bin/sh so
// that it works whatever the user's shell is (fish, csh...).
func remoteCommand(l shellLaunch) string {
	if l.mux != "tmux" && l.mux != "screen" {
		l.mux = ""
	}
	if l.mux == "" && l.kube == nil && l.dir == "" && len(l.env) == 0 && l.command == "" {
		return ""
	}
	name := sessionName(l.name)
	var script strings.Builder
	if l.kube != nil {
		script.WriteString(kube.RemoteSetup(name, *l.kube))
	}
	for _, kv := range l.env {
		if k, v, ok := envPair(kv); ok {
			fmt.Fprintf(&script, "export %s=%s; ", k, kube.ShellQuote(v))
		}
	}
	if l.dir != "" {
		fmt.Fprintf(&script, "cd %s 2>/dev/null; ", shellPath(l.dir))
	}
	run := ""
	if l.command != "" {
		run = " " + kube.ShellQuote(l.command)
	}
	switch l.mux {
	case "tmux":
		if l.kube != nil {
			// -e (tmux 3.2+) gives a new session the pane's KUBECONFIG;
			// an existing session keeps its own.
			fmt.Fprintf(&script, `command -v tmux >/dev/null 2>&1 && { tmux new-session -A -s %s -e "KUBECONFIG=$KUBECONFIG"%s 2>/dev/null || tmux new-session -A -s %s%s; exit; }; `, name, run, name, run)
		} else {
			fmt.Fprintf(&script, `command -v tmux >/dev/null 2>&1 && exec tmux new-session -A -s %s%s; `, name, run)
		}
	case "screen":
		screenRun := ""
		if l.command != "" {
			screenRun = " /bin/sh -c" + run
		}
		fmt.Fprintf(&script, `command -v screen >/dev/null 2>&1 && exec screen -D -R -S %s%s; `, name, screenRun)
	}
	if l.command != "" {
		script.WriteString(`exec "${SHELL:-/bin/sh}" -l -c` + run)
	} else {
		script.WriteString(`exec "${SHELL:-/bin/sh}" -l`)
	}
	return "/bin/sh -c " + kube.ShellQuote(script.String())
}

// envPair splits NAME=value; names are letters, digits and '_', not
// starting with a digit.
func envPair(kv string) (string, string, bool) {
	k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
	if !ok || k == "" || (k[0] >= '0' && k[0] <= '9') {
		return "", "", false
	}
	for _, r := range k {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return "", "", false
		}
	}
	return k, v, true
}

// shellPath quotes a directory for sh, keeping a leading ~ as $HOME.
func shellPath(dir string) string {
	switch {
	case dir == "~":
		return `"$HOME"`
	case strings.HasPrefix(dir, "~/"):
		return `"$HOME"/` + kube.ShellQuote(dir[2:])
	}
	return kube.ShellQuote(dir)
}

// sessionName keeps letters, digits, '-' and '_' so the name needs no
// quoting; tmux also rejects '.' and ':'.
func sessionName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "shellvane"
	}
	return b.String()
}
