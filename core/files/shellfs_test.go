package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

// shRunner runs commands with the local shell, as a server would.
type shRunner struct{}

func (shRunner) Run(cmd string) ([]byte, error) {
	c := exec.Command("/bin/sh", "-c", cmd)
	var stderr bytes.Buffer
	c.Stderr = &stderr
	out, err := c.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out, &ExitError{Status: exit.ExitCode(), Stderr: stderr.String()}
	}
	return out, err
}

func (shRunner) Stream(cmd string) (io.ReadCloser, error) {
	c := exec.Command("/bin/sh", "-c", cmd)
	out, err := c.StdoutPipe()
	if err != nil {
		return nil, err
	}
	return out, c.Start()
}

type sink struct {
	io.WriteCloser
	c *exec.Cmd
}

func (s sink) Close() error {
	s.WriteCloser.Close()
	return s.c.Wait()
}

func (shRunner) Sink(cmd string) (io.WriteCloser, error) {
	c := exec.Command("/bin/sh", "-c", cmd)
	in, err := c.StdinPipe()
	if err != nil {
		return nil, err
	}
	return sink{in, c}, c.Start()
}

func TestShellFS(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("needs GNU find")
	}
	s := NewShellFS(shRunner{})
	if err := s.Probe(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "it's.txt"), []byte("hello"))
	os.Mkdir(filepath.Join(dir, "sub"), 0o750)
	os.Symlink("sub", filepath.Join(dir, "link"))

	list, err := s.List(dir)
	if err != nil || len(list) != 3 || list[0].Name != "link" || !list[0].IsDir || list[0].Target != "sub" || list[1].Name != "sub" {
		t.Fatalf("list %+v %v", list, err)
	}
	f := list[2]
	if f.Name != "it's.txt" || f.Size != 5 || f.Perm != 0o644 || f.Mode != "-rw-r--r--" || f.UID != os.Getuid() {
		t.Fatalf("file %+v", f)
	}
	if e, err := s.Stat(filepath.Join(dir, "sub")); err != nil || !e.IsDir || e.Perm != 0o750 {
		t.Fatalf("stat %+v %v", e, err)
	}
	if _, err := s.Stat(filepath.Join(dir, "nope")); errs.CodeOf(err) != ErrNotFound {
		t.Fatalf("missing: %v", err)
	}
	if err := s.Mkdir(filepath.Join(dir, "sub")); errs.CodeOf(err) != ErrExists {
		t.Fatalf("mkdir existing: %v", err)
	}

	// Copies through the shell, both ways, with resume.
	dst := t.TempDir()
	if err := Copy(context.Background(), s, dir, Local{}, dst, nil); err != nil {
		t.Fatal(err)
	}
	back := filepath.Join(dst, filepath.Base(dir))
	if b, _ := os.ReadFile(filepath.Join(back, "it's.txt")); string(b) != "hello" {
		t.Fatalf("copied %q", b)
	}
	write(t, filepath.Join(dir, "big.txt"), []byte("0123456789"))
	write(t, filepath.Join(dst, "big.txt"), []byte("0123"))
	if err := CopyWith(context.Background(), s, filepath.Join(dir, "big.txt"), Local{}, dst, Options{Conflict: ConflictResume}, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "big.txt")); string(b) != "0123456789" {
		t.Fatalf("resumed %q", b)
	}
	up := t.TempDir()
	if err := Copy(context.Background(), Local{}, filepath.Join(dst, "big.txt"), s, up, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(up, "big.txt")); string(b) != "0123456789" {
		t.Fatalf("uploaded %q", b)
	}
	if err := s.Rename(filepath.Join(up, "big.txt"), filepath.Join(dir, "big.txt")); errs.CodeOf(err) != ErrExists {
		t.Fatalf("rename over: %v", err)
	}
	if err := s.Chmod(filepath.Join(up, "big.txt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(up); err != nil {
		t.Fatal(err)
	}
}
