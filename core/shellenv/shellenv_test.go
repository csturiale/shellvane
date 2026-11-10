package shellenv

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExtractAndMerge(t *testing.T) {
	out := []byte("welcome banner\n" + marker + "/opt/homebrew/bin:/usr/bin" + marker + "\nbye")
	if got := Extract(out); got != "/opt/homebrew/bin:/usr/bin" {
		t.Fatalf("Extract = %q", got)
	}
	if Extract([]byte("no markers")) != "" {
		t.Fatal("Extract without markers")
	}
	sep := string(filepath.ListSeparator)
	got := Merge("/usr/bin"+sep+"/bin", "/opt/homebrew/bin"+sep+"/usr/bin")
	if want := "/usr/bin" + sep + "/bin" + sep + "/opt/homebrew/bin"; got != want {
		t.Fatalf("Merge = %q, want %q", got, want)
	}
}

func TestLoginPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no login shell")
	}
	dir := t.TempDir()
	sh := filepath.Join(dir, "fakesh")
	os.WriteFile(sh, []byte("#!/bin/sh\necho noise\nPATH=/extra/bin:$PATH\nexport HTTPS_PROXY=http://proxy.corp:8080\nexport NO_PROXY=.corp\nshift\neval \"$1\"\n"), 0o755)
	t.Setenv("SHELL", sh)
	t.Setenv("PATH", "/usr/bin:/bin")
	t.Setenv("HTTPS_PROXY", "")
	os.Unsetenv("HTTPS_PROXY")
	t.Setenv("NO_PROXY", "mine")
	p := LoginPath(context.Background())
	if p != "/extra/bin:/usr/bin:/bin" {
		t.Fatalf("LoginPath = %q", p)
	}
	Complete(context.Background())
	// The terminal's order: the shell's entries first, then the usual
	// tool folders that exist on this computer.
	want := Merge("/extra/bin:/usr/bin:/bin", strings.Join(existing(Fallback()), ":"))
	if os.Getenv("PATH") != want {
		t.Fatalf("PATH = %q", os.Getenv("PATH"))
	}
	// Proxies come from the shell, without replacing the app's own.
	if os.Getenv("HTTPS_PROXY") != "http://proxy.corp:8080" || os.Getenv("NO_PROXY") != "mine" {
		t.Fatalf("proxies = %q %q", os.Getenv("HTTPS_PROXY"), os.Getenv("NO_PROXY"))
	}
	Wait(context.Background()) // not started: returns at once
}

func TestUserShellParsers(t *testing.T) {
	if got := parseDscl("UserShell: /bin/zsh\n"); got != "/bin/zsh" {
		t.Fatalf("dscl = %q", got)
	}
	passwd := "root:x:0:0:root:/root:/bin/bash\nada:x:1000:1000::/home/ada:/usr/bin/fish\n"
	if got := passwdShell(passwd, "ada"); got != "/usr/bin/fish" {
		t.Fatalf("passwd = %q", got)
	}
	if passwdShell(passwd, "nobody") != "" {
		t.Fatal("unknown user")
	}
}
