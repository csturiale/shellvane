package app

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/files"
)

func TestLocalTransferEmitsProgress(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o644)

	var mu sync.Mutex
	var states []string
	svc := NewFilesService(nil, func(name string, data any) {
		if st, ok := data.(TransferStatus); ok && name == EventTransfer {
			mu.Lock()
			states = append(states, st.State)
			mu.Unlock()
		}
	})
	id, err := svc.Transfer(LocalFS, filepath.Join(src, "a.txt"), LocalFS, dst, files.Options{})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if list := svc.Transfers(); len(list) == 1 && list[0].State == TransferDone {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, _ := os.ReadFile(filepath.Join(dst, "a.txt"))
	if string(got) != "hello" {
		t.Fatalf("file not copied (transfer %s)", id)
	}
	mu.Lock()
	defer mu.Unlock()
	if states[0] != TransferQueued || states[len(states)-1] != TransferDone {
		t.Fatalf("states = %v", states)
	}
	svc.ClearFinished()
	if len(svc.Transfers()) != 0 {
		t.Fatal("finished transfers should be cleared")
	}
}

func TestUnknownFileSystem(t *testing.T) {
	svc := NewFilesService(nil, func(string, any) {})
	if _, err := svc.List("nope", "/"); err == nil {
		t.Fatal("unknown file system must fail")
	}
}

func TestChmodService(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file systems have no Unix permission bits")
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "a.sh")
	os.WriteFile(f, []byte("x"), 0o644)
	svc := NewFilesService(nil, func(string, any) {})
	if err := svc.Chmod(LocalFS, f, 0o755, false); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(f); fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v", fi.Mode().Perm())
	}
	if err := svc.Chmod(LocalFS, f, 0o17777, false); err == nil {
		t.Fatal("modes above 07777 must be rejected")
	}
}
