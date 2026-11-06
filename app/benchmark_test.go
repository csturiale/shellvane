package app

import (
	"io"
	"strings"
	"testing"
)

func TestBenchProcessPrintsExactly(t *testing.T) {
	b := &benchProcess{left: 1 << 20}
	data, err := io.ReadAll(struct{ io.Reader }{b})
	if err != nil || len(data) != 1<<20 {
		t.Fatalf("read %d bytes, %v", len(data), err)
	}
	if !strings.HasPrefix(string(data), "1\r\n2\r\n3\r\n") {
		t.Fatalf("start = %q", data[:12])
	}
}
