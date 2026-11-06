package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// The check runs in the user's shell, which may put the tool on the PATH
// (like ~/.zshrc does for the Google Cloud SDK).
func TestCheckLoginUsesTheUserShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cmd /C")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "sdk")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "fakecloud"), []byte("#!/bin/sh\n[ \"$1\" = ok ] && exit 0\necho 'not logged in: run fakecloud login' >&2\nexit 1\n"), 0o755)
	shell := filepath.Join(dir, "zsh")
	os.WriteFile(shell, []byte("#!/bin/sh\n# -ilc command\nPATH=\""+bin+":$PATH\"\nexport PATH\nshift\nexec /bin/sh -c \"$1\"\n"), 0o755)
	t.Setenv("SHELL", shell)

	s := &EnvironmentService{}
	if r := s.CheckLogin(context.Background(), "fakecloud ok", nil); !r.OK {
		t.Fatalf("check through the user's shell failed: %+v", r)
	}
	r := s.CheckLogin(context.Background(), "fakecloud {{state}}", map[string]string{"state": "expired"})
	if r.OK || r.Detail != "not logged in: run fakecloud login" {
		t.Fatalf("failing check = %+v", r)
	}
}

// A shell whose startup files leave a background process holding the
// output (ssh-agent, gitstatusd…) must not make the check hang.
func TestCheckLoginDoesNotWaitForBackgroundProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cmd /C")
	}
	dir := t.TempDir()
	shell := filepath.Join(dir, "zsh")
	os.WriteFile(shell, []byte("#!/bin/sh\nsleep 30 &\nshift\nexec /bin/sh -c \"$1\"\n"), 0o755)
	t.Setenv("SHELL", shell)
	done := make(chan LoginCheck, 1)
	go func() { done <- (&EnvironmentService{}).CheckLogin(context.Background(), "true", nil) }()
	select {
	case r := <-done:
		if !r.OK {
			t.Fatalf("check = %+v", r)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the check waited for a background process of the shell")
	}
}
