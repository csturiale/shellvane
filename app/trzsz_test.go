package app

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/localshell"
)

func TestTrzszPassesTerminalBytes(t *testing.T) {
	inner := &fakeProcess{out: bytes.NewReader([]byte("hello \x1b[31mred\x1b[0m\r\n"))}
	p := withTrzsz(inner, 80)
	defer p.Close()
	got := make([]byte, 64)
	n, _ := io.ReadAtLeast(p, got, len("hello \x1b[31mred\x1b[0m\r\n"))
	if string(got[:n]) != "hello \x1b[31mred\x1b[0m\r\n" {
		t.Fatalf("output = %q", got[:n])
	}
	p.Write([]byte("ls -la\r"))
	deadline := time.Now().Add(2 * time.Second)
	for inner.written.String() != "ls -la\r" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if inner.written.String() != "ls -la\r" {
		t.Fatalf("input = %q", inner.written.String())
	}
}

// A real tsz (from trzsz-go) in a local terminal sends a file through the
// filter; the download folder is set instead of the system dialog.
func TestTrzszDownloadsAFile(t *testing.T) {
	tsz, err := exec.LookPath("tsz")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("tsz not installed")
	}
	src, dst := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(src, "report.txt"), bytes.Repeat([]byte("shellvane\n"), 5000), 0o644)
	sh, err := localshell.Start(localshell.Options{Program: localshell.Program{ID: "sh", Name: "sh", Path: "/bin/sh"}, Dir: src, Cols: 100, Rows: 30})
	if err != nil {
		t.Fatal(err)
	}
	p := withTrzsz(sh, 100).(*trzszProcess)
	defer p.Close()
	p.filter.SetDefaultDownloadPath(dst)
	go io.Copy(io.Discard, p) // the terminal reading
	p.Write([]byte(tsz + " -y report.txt\r"))
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(filepath.Join(dst, "report.txt")); err == nil && len(data) == 40000 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("file not downloaded")
}
