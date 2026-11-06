package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadsChunksAndNames(t *testing.T) {
	dir := t.TempDir()
	s := NewDownloadsService(dir)
	id, err := s.Begin("../../etc/report.txt")
	if err != nil {
		t.Fatal(err)
	}
	s.Append(id, []byte("hello "))
	s.Append(id, []byte("world"))
	path, err := s.Finish(id)
	if err != nil || path != filepath.Join(dir, "report.txt") {
		t.Fatalf("Finish = %q, %v", path, err)
	}
	if data, _ := os.ReadFile(path); string(data) != "hello world" {
		t.Fatalf("content %q", data)
	}

	// The same name again: a new file next to it, never a replacement.
	id2, _ := s.Begin("report.txt")
	p2, _ := s.Finish(id2)
	if p2 != filepath.Join(dir, "report (1).txt") {
		t.Fatalf("second = %q", p2)
	}
	if data, _ := os.ReadFile(path); string(data) != "hello world" {
		t.Fatal("the first file was replaced")
	}

	id3, _ := s.Begin(`C:\temp\..\a<b>.bin`)
	s.Append(id3, []byte("partial"))
	s.Cancel(id3)
	if _, err := os.Stat(filepath.Join(dir, "a_b_.bin")); !os.IsNotExist(err) {
		t.Fatal("a cancelled file must be removed")
	}
	if err := s.Append(id3, nil); err == nil {
		t.Fatal("append after cancel")
	}
	for in, want := range map[string]string{"": "download", "..": "download", "a\x00b": "a_b", ".hidden": "hidden"} {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
}
