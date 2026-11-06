package app

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/kube"
)

func TestMultiplexerCommand(t *testing.T) {
	cmd := remoteCommand(shellLaunch{mux: "tmux", name: "shellvane-web.1; rm -rf ~", kube: nil})
	if !strings.Contains(cmd, "tmux new-session -A -s shellvane-web1rm-rf;") {
		t.Fatalf("name not sanitized: %s", cmd)
	}
	if !strings.HasPrefix(cmd, "/bin/sh -c '") || strings.Count(cmd, "'") != 2 {
		t.Fatalf("quoting: %s", cmd)
	}
	if !strings.Contains(remoteCommand(shellLaunch{mux: "screen", name: "x", kube: nil}), "screen -D -R -S x") {
		t.Fatal("screen")
	}
	if remoteCommand(shellLaunch{mux: "zellij", name: "x", kube: nil}) != "" {
		t.Fatal("unknown multiplexers are ignored")
	}
	cmd = remoteCommand(shellLaunch{mux: "", name: "p1", kube: &kube.Selection{Context: "prod"}})
	if !strings.Contains(cmd, "shellvane/p1.yaml") || !strings.HasSuffix(cmd, `exec "${SHELL:-/bin/sh}" -l'`) {
		t.Fatalf("kube without multiplexer: %s", cmd)
	}
	if !strings.Contains(remoteCommand(shellLaunch{mux: "tmux", name: "p1", kube: &kube.Selection{Context: "prod"}}), "-e \"KUBECONFIG=$KUBECONFIG\"") {
		t.Fatal("tmux must receive the pane's KUBECONFIG")
	}
	if sessionName("..") != "shellvane" {
		t.Fatal("empty name")
	}
}

// The generated command must be valid shell, kube setup included.
func TestRemoteCommandRuns(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	home := t.TempDir()
	cmd := remoteCommand(shellLaunch{mux: "", name: "p1", kube: &kube.Selection{Context: "it's"}})
	// Replace the final login shell by a probe.
	cmd = strings.Replace(cmd, `exec "${SHELL:-/bin/sh}" -l`, `printf %s "$KUBECONFIG"`, 1)
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
	out, err := c.CombinedOutput()
	if err != nil || !strings.HasPrefix(string(out), home+"/.kube/shellvane/p1.yaml:") {
		t.Fatalf("%v: %s", err, out)
	}
}

func TestRemoteCommandOptions(t *testing.T) {
	cmd := remoteCommand(shellLaunch{dir: "~/src/my app", env: []string{"APP_ENV=prod it's", "1BAD=x", "bad-name=y", "EMPTY="}, command: "htop"})
	for _, want := range []string{`export APP_ENV='prod it'\''s'`, `export EMPTY=''`, `cd "$HOME"/'src/my app'`, `-l -c 'htop'`} {
		if !strings.Contains(unquote(t, cmd), want) {
			t.Errorf("missing %q in %s", want, unquote(t, cmd))
		}
	}
	if strings.Contains(cmd, "1BAD") || strings.Contains(cmd, "bad-name") {
		t.Errorf("invalid names exported: %s", cmd)
	}
	if !strings.Contains(unquote(t, remoteCommand(shellLaunch{mux: "tmux", name: "p", command: "top"})), "tmux new-session -A -s p 'top'") {
		t.Error("tmux with command")
	}
	if remoteCommand(shellLaunch{name: "p"}) != "" {
		t.Error("nothing to do should keep the plain shell")
	}
}

// unquote runs the outer /bin/sh -c argument back through sh to get the script.
func unquote(t *testing.T, cmd string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs /bin/sh")
	}
	out, err := exec.Command("/bin/sh", "-c", "printf %s "+strings.TrimPrefix(cmd, "/bin/sh -c ")).Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
