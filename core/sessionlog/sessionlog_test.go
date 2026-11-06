package sessionlog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

func TestLoggerFormats(t *testing.T) {
	dir := t.TempDir()
	for format, want := range map[string]string{
		FormatRaw:         "\x1b[31mred\x1b[0m\r\n50%\rok",
		FormatText:        "red\nok\n",
		FormatTimestamped: "2026-01-02 03:04:05.000 red\n2026-01-02 03:04:05.000 ok\n",
	} {
		l, err := NewLogger(filepath.Join(dir, format, "s.log"), format)
		if err != nil {
			t.Fatal(err)
		}
		l.now = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
		l.Write([]byte("\x1b[31mred\x1b[0m\r\n50%"))
		l.Write([]byte("\rok"))
		l.Close()
		got, _ := os.ReadFile(l.Path)
		if format == FormatTimestamped {
			// The last, unterminated line is written without a time.
			want = "2026-01-02 03:04:05.000 red\nok\n"
		}
		if string(got) != want {
			t.Errorf("%s: %q, want %q", format, got, want)
		}
	}
	if _, err := NewLogger(filepath.Join(dir, "x.log"), "html"); errs.CodeOf(err) != ErrFormat {
		t.Fatal("unknown format accepted")
	}
}

func TestRecorderKeepsUTF8Whole(t *testing.T) {
	dir := t.TempDir()
	r, err := NewRecorder(filepath.Join(dir, "a.cast"), "web-1", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	euro := []byte("€") // 3 bytes, split across two frames
	r.Write(append([]byte("price "), euro[:2]...))
	r.Write(append(euro[2:], '\n'))
	r.Resize(100, 30)
	r.Close()
	data, _ := os.ReadFile(r.Path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var h Header
	json.Unmarshal([]byte(lines[0]), &h)
	if h.Version != 2 || h.Width != 80 || h.Title != "web-1" {
		t.Fatalf("header %+v", h)
	}
	var text string
	for _, l := range lines[1:] {
		var ev []any
		json.Unmarshal([]byte(l), &ev)
		if ev[1] == "o" {
			text += ev[2].(string)
		}
		if ev[1] == "r" && ev[2] != "100x30" {
			t.Fatalf("resize %v", ev)
		}
	}
	if text != "price €\n" {
		t.Fatalf("text %q", text)
	}

	files, err := List(dir)
	if err != nil || len(files) != 1 || files[0].Kind != "recording" || files[0].Title != "web-1" {
		t.Fatalf("list %+v, %v", files, err)
	}
	if _, err := Read(dir, "/etc/passwd"); errs.CodeOf(err) != ErrNotFound {
		t.Fatal("read outside the folder")
	}
	if got, _ := Read(dir, r.Path); !strings.HasPrefix(got, "{") {
		t.Fatal("read")
	}
	if err := Delete(dir, r.Path); err != nil {
		t.Fatal(err)
	}
}

func TestFileName(t *testing.T) {
	got := FileName("prod/web:1", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), ".log")
	if got != "prod_web_1_2026-01-02_03-04-05.log" {
		t.Fatal(got)
	}
}
