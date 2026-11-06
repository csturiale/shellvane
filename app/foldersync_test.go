package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSyncFolders(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(src, "docs", "img"), 0o700)
	os.WriteFile(filepath.Join(src, "docs", "img", "a.txt"), []byte("a"), 0o600)
	os.WriteFile(filepath.Join(src, "readme.md"), []byte("new readme"), 0o600)
	os.WriteFile(filepath.Join(dst, "readme.md"), []byte("old"), 0o600)
	past := time.Now().Add(-time.Hour)
	os.Chtimes(filepath.Join(dst, "readme.md"), past, past)
	os.WriteFile(filepath.Join(dst, "leftover.log"), []byte("x"), 0o600)

	svc := NewFilesService(NewSessionService(nil, nil, nil, nil, func(string, any) {}), func(string, any) {})
	diffs, err := svc.Compare(context.Background(), LocalFS, src, LocalFS, dst)
	if err != nil || len(diffs) != 3 {
		t.Fatalf("Compare = %+v, %v", diffs, err)
	}
	n, err := svc.SyncFolders(LocalFS, src, LocalFS, dst, diffs, true)
	if err != nil || n != 2 {
		t.Fatalf("SyncFolders = %d, %v", n, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		done := true
		for _, tr := range svc.Transfers() {
			if tr.State != TransferDone {
				done = false
			}
		}
		if done {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "readme.md")); string(b) != "new readme" {
		t.Errorf("readme = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dst, "docs", "img", "a.txt")); err != nil {
		t.Errorf("folder not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "leftover.log")); !os.IsNotExist(err) {
		t.Error("extra file not removed")
	}
	if again, _ := svc.Compare(context.Background(), LocalFS, src, LocalFS, dst); len(again) != 0 {
		t.Errorf("still different: %+v", again)
	}
}
