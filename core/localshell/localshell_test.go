package localshell

import (
	"bytes"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProgramsFindsAShell(t *testing.T) {
	if len(Programs()) == 0 {
		t.Fatal("no shell found")
	}
}

func TestRunShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	s, err := Start(Options{
		Program: Program{Name: "sh", Path: "/bin/sh"},
		Env:     []string{"SHELLVANE_TEST=ok"},
		Cols:    100, Rows: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Write([]byte("echo \"v=$SHELLVANE_TEST cols=$(stty size | cut -d' ' -f2)\"; exit 3\n"))
	var out bytes.Buffer
	done := make(chan struct{})
	go func() {
		io.Copy(&out, s)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("shell did not exit")
	}
	if code := s.Wait(); code != 3 {
		t.Fatalf("exit code %d", code)
	}
	if !strings.Contains(out.String(), "v=ok cols=100") {
		t.Fatalf("output %q", out.String())
	}
}
