package files

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/pkg/sftp"

	"github.com/csturiale/shellvane/core/errs"
)

// remoteFS serves the local disk over an in-memory SFTP pipe. SFTP paths use
// forward slashes, so tests pass Windows paths through filepath.ToSlash.
func remoteFS(t *testing.T) *Remote {
	t.Helper()
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	server, err := sftp.NewServer(struct {
		io.Reader
		io.WriteCloser
	}{sr, sw})
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve()
	client, err := sftp.NewClientPipe(cr, cw)
	if err != nil {
		t.Fatal(err)
	}
	// The server does not close its side on EOF: do it, or Close waits forever.
	t.Cleanup(func() { sw.Close(); client.Close(); server.Close() })
	return NewRemote(client)
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListSortsDirectoriesFirst(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "b.txt"), []byte("b"))
	write(t, filepath.Join(dir, "A.txt"), []byte("a"))
	os.Mkdir(filepath.Join(dir, "zeta"), 0o755)
	for name, fsys := range map[string]FS{"local": Local{}, "remote": remoteFS(t)} {
		list, err := fsys.List(dir)
		if err != nil {
			t.Fatal(name, err)
		}
		got := []string{}
		for _, e := range list {
			got = append(got, e.Name)
		}
		if len(got) != 3 || got[0] != "zeta" || got[1] != "A.txt" || got[2] != "b.txt" || !list[0].IsDir {
			t.Errorf("%s: list = %v", name, got)
		}
	}
}

func TestUploadAndDownloadDirectory(t *testing.T) {
	src, mid, back := t.TempDir(), t.TempDir(), t.TempDir()
	big := bytes.Repeat([]byte("x"), 700*1024)
	write(t, filepath.Join(src, "project", "main.go"), []byte("package main"))
	write(t, filepath.Join(src, "project", "assets", "big.bin"), big)
	remote := remoteFS(t)

	var last, total int64
	progress := func(done, tot int64, _ string) { last, total = done, tot }
	if err := Copy(context.Background(), Local{}, filepath.Join(src, "project"), remote, filepath.ToSlash(mid), progress); err != nil {
		t.Fatal(err)
	}
	if total != int64(len(big))+12 || last != total {
		t.Errorf("progress %d/%d", last, total)
	}
	if err := Copy(context.Background(), remote, filepath.ToSlash(filepath.Join(mid, "project")), Local{}, back, nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(back, "project", "assets", "big.bin"))
	if err != nil || !bytes.Equal(got, big) {
		t.Fatalf("round trip lost data: %v", err)
	}
}

func TestCancelledCopyLeavesNoPartialFile(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	write(t, filepath.Join(src, "f.bin"), bytes.Repeat([]byte("y"), 1024*1024))
	ctx, cancel := context.WithCancel(context.Background())
	err := Copy(ctx, Local{}, filepath.Join(src, "f.bin"), Local{}, dst, func(done, total int64, _ string) {
		if done > 0 {
			cancel()
		}
	})
	if errs.CodeOf(err) != ErrCancelled {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "f.bin")); !os.IsNotExist(err) {
		t.Fatal("partial file left behind")
	}
}

func TestOperationsAndErrors(t *testing.T) {
	dir := t.TempDir()
	for name, fsys := range map[string]FS{"local": Local{}, "remote": remoteFS(t)} {
		base := filepath.Join(dir, name)
		os.Mkdir(base, 0o755)
		d := fsys.Join(base, "new")
		if err := fsys.Mkdir(d); err != nil {
			t.Fatal(name, err)
		}
		if err := fsys.Mkdir(d); errs.CodeOf(err) == "" {
			t.Errorf("%s: mkdir twice should fail", name)
		}
		write(t, filepath.Join(d, "a.txt"), []byte("a"))
		write(t, filepath.Join(d, "b.txt"), []byte("b"))
		if err := fsys.Rename(fsys.Join(d, "a.txt"), fsys.Join(d, "b.txt")); errs.CodeOf(err) != ErrExists {
			t.Errorf("%s: rename over an existing file must fail, got %v", name, err)
		}
		if err := fsys.Rename(fsys.Join(d, "a.txt"), fsys.Join(d, "c.txt")); err != nil {
			t.Error(name, err)
		}
		if err := fsys.Remove(d); err != nil {
			t.Error(name, err)
		}
		if _, err := fsys.List(d); errs.CodeOf(err) != ErrNotFound {
			t.Errorf("%s: list of a removed dir = %v", name, err)
		}
	}
}

func TestRemoteToRemoteCopy(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	data := bytes.Repeat([]byte("z"), 300*1024)
	write(t, filepath.Join(src, "dir", "f.bin"), data)
	// Two independent SFTP servers stand in for two hosts.
	a, b := remoteFS(t), remoteFS(t)
	if err := Copy(context.Background(), a, filepath.ToSlash(filepath.Join(src, "dir")), b, filepath.ToSlash(dst), nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "dir", "f.bin"))
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("copy between servers failed: %v", err)
	}
}

func TestChmod(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file systems have no Unix permission bits")
	}
	dir := t.TempDir()
	for name, fsys := range map[string]FS{"local": Local{}, "remote": remoteFS(t)} {
		base := filepath.Join(dir, name)
		write(t, filepath.Join(base, "sub", "run.sh"), []byte("#!/bin/sh"))
		file := filepath.Join(base, "sub", "run.sh")

		if err := fsys.Chmod(file, 0o750); err != nil {
			t.Fatal(name, err)
		}
		if e, _ := fsys.Stat(file); e.Perm != 0o750 || e.Mode != "-rwxr-x---" {
			t.Errorf("%s: perm = %o (%s)", name, e.Perm, e.Mode)
		}

		if err := ChmodAll(fsys, base, 0o700, true); err != nil {
			t.Fatal(name, err)
		}
		for _, p := range []string{base, filepath.Join(base, "sub"), file} {
			if e, _ := fsys.Stat(p); e.Perm != 0o700 {
				t.Errorf("%s: %s perm = %o after recursive chmod", name, p, e.Perm)
			}
		}
		if err := fsys.Chmod(filepath.Join(base, "missing"), 0o644); errs.CodeOf(err) != ErrNotFound {
			t.Errorf("%s: chmod of a missing file = %v", name, err)
		}
	}
}

func TestUnixPermRoundTrip(t *testing.T) {
	for _, p := range []uint32{0o644, 0o755, 0o4755, 0o2775, 0o1777, 0o7777} {
		if got := unixPerm(goMode(p)); got != p {
			t.Errorf("%o -> %o", p, got)
		}
	}
}

func TestCopyConflicts(t *testing.T) {
	ctx := context.Background()
	src, dst := t.TempDir(), t.TempDir()
	write(t, filepath.Join(src, "a.txt"), []byte("new content"))
	write(t, filepath.Join(dst, "a.txt"), []byte("old"))
	read := func(name string) string {
		b, _ := os.ReadFile(filepath.Join(dst, name))
		return string(b)
	}
	from := filepath.Join(src, "a.txt")

	CopyWith(ctx, Local{}, from, Local{}, dst, Options{Conflict: ConflictSkip}, nil)
	if read("a.txt") != "old" {
		t.Fatal("skip overwrote")
	}
	CopyWith(ctx, Local{}, from, Local{}, dst, Options{Conflict: ConflictRename}, nil)
	CopyWith(ctx, Local{}, from, Local{}, dst, Options{Conflict: ConflictRename}, nil)
	if read("a (1).txt") != "new content" || read("a (2).txt") != "new content" {
		t.Fatal("rename")
	}
	// The destination is newer: "newer" keeps it.
	future := time.Now().Add(time.Hour)
	os.Chtimes(filepath.Join(dst, "a.txt"), future, future)
	CopyWith(ctx, Local{}, from, Local{}, dst, Options{Conflict: ConflictNewer}, nil)
	if read("a.txt") != "old" {
		t.Fatal("newer copied an older file")
	}
	// Resume: "new" is the start of the source, the rest is appended.
	write(t, filepath.Join(dst, "a.txt"), []byte("new"))
	var last int64
	CopyWith(ctx, Local{}, from, Local{}, dst, Options{Conflict: ConflictResume}, func(done, _ int64, _ string) { last = done })
	if read("a.txt") != "new content" || last != 11 {
		t.Fatalf("resume: %q, progress %d", read("a.txt"), last)
	}
	CopyWith(ctx, Local{}, from, Local{}, dst, Options{}, nil)
	if read("a.txt") != "new content" {
		t.Fatal("overwrite")
	}
}

func TestLinksAndOwners(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need privileges on Windows")
	}
	dir := t.TempDir()
	write(t, filepath.Join(dir, "target.txt"), []byte("x"))
	for name, fsys := range map[string]FS{"local": Local{}, "remote": remoteFS(t)} {
		link := filepath.Join(dir, name+"-link")
		if err := fsys.Symlink("target.txt", link); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got, err := fsys.ReadLink(link); err != nil || got != "target.txt" {
			t.Fatalf("%s: readlink %q %v", name, got, err)
		}
		list, _ := fsys.List(dir)
		for _, e := range list {
			if e.Name == name+"-link" && (!e.IsLink || e.Target != "target.txt") {
				t.Fatalf("%s: entry %+v", name, e)
			}
			if e.Name == "target.txt" && e.UID != os.Getuid() {
				t.Fatalf("%s: uid %d", name, e.UID)
			}
		}
		if err := fsys.Chown(filepath.Join(dir, "target.txt"), os.Getuid(), os.Getgid()); err != nil {
			t.Fatalf("%s: chown %v", name, err)
		}
	}
}

// evilFS lists, in every folder, entries named to escape the destination.
type evilFS struct{ Local }

func (e evilFS) List(dir string) ([]Entry, error) {
	list, err := e.Local.List(dir)
	if err != nil {
		return nil, err
	}
	payload := filepath.Join(dir, "payload")
	for _, name := range []string{"../escaped", "..", "a/../../escaped2", `..\escaped3`} {
		list = append(list, Entry{Name: name, Path: payload})
	}
	return list, nil
}

func TestCopyIgnoresNamesThatEscape(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src", "dir")
	os.MkdirAll(src, 0o755)
	write(t, filepath.Join(src, "payload"), []byte("x"))
	dst := filepath.Join(root, "out", "dest")
	os.MkdirAll(dst, 0o755)
	if err := Copy(context.Background(), evilFS{}, src, Local{}, dst, nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(root, "out", "escaped"), filepath.Join(root, "out", "escaped2"), filepath.Join(dst, "escaped")} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s written outside the destination", p)
		}
	}
	if _, err := os.Stat(filepath.Join(dst, "dir", "payload")); err != nil {
		t.Errorf("normal file not copied: %v", err)
	}
}
