// Package sessionlog writes terminal sessions to disk: logs (raw bytes,
// plain text or timestamped text) and asciinema v2 recordings.
package sessionlog

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/termtext"
)

// Error codes returned by this package.
const (
	ErrCreate errs.Code = "sessionlog.createFailed"
	ErrFormat errs.Code = "sessionlog.badFormat"
)

// Log formats.
const (
	FormatRaw         = "raw"         // bytes as received, escape sequences included
	FormatText        = "text"        // plain text lines
	FormatTimestamped = "timestamped" // plain text lines prefixed with the time
)

// FileName is a safe file name for a session started at t.
func FileName(name string, t time.Time, ext string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r < 32:
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	base := strings.TrimSpace(b.String())
	if base == "" {
		base = "session"
	}
	return base + "_" + t.Format("2006-01-02_15-04-05") + ext
}

func create(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, errs.Wrap(err, ErrCreate, errs.Params{"path": path})
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, errs.Wrap(err, ErrCreate, errs.Params{"path": path})
	}
	return f, nil
}

// Logger writes a session log.
type Logger struct {
	mu     sync.Mutex
	f      *os.File
	w      *bufio.Writer
	format string
	text   termtext.Cleaner
	now    func() time.Time
	Path   string
}

// NewLogger creates (or appends to) the log at path.
func NewLogger(path, format string) (*Logger, error) {
	switch format {
	case FormatRaw, FormatText, FormatTimestamped:
	case "":
		format = FormatText
	default:
		return nil, errs.New(ErrFormat, errs.Params{"format": format})
	}
	f, err := create(path)
	if err != nil {
		return nil, err
	}
	return &Logger{f: f, w: bufio.NewWriter(f), format: format, now: time.Now, Path: path}, nil
}

// Write logs output.
func (l *Logger) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.format == FormatRaw {
		return l.w.Write(p)
	}
	l.text.Feed(p, func(line []byte) {
		if l.format == FormatTimestamped {
			l.w.WriteString(l.now().Format("2006-01-02 15:04:05.000 "))
		}
		l.w.Write(line)
		l.w.WriteByte('\n')
	})
	return len(p), nil
}

// Close flushes the log, including an unterminated last line.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.format != FormatRaw {
		if rest := l.text.Partial(); len(rest) > 0 {
			l.w.Write(rest)
			l.w.WriteByte('\n')
		}
	}
	l.w.Flush()
	return l.f.Close()
}

// Recorder writes an asciinema v2 recording: a JSON header, then one
// [seconds, "o", text] event per output and [seconds, "r", "WxH"] on resize.
type Recorder struct {
	mu      sync.Mutex
	f       *os.File
	w       *bufio.Writer
	start   time.Time
	pending []byte // an incomplete UTF-8 sequence at the end of a frame
	now     func() time.Time
	Path    string
}

// Header is the first line of a recording.
type Header struct {
	Version   int               `json:"version"`
	Width     int               `json:"width"`
	Height    int               `json:"height"`
	Timestamp int64             `json:"timestamp"`
	Title     string            `json:"title,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
}

// NewRecorder starts a recording at path.
func NewRecorder(path, title string, cols, rows int) (*Recorder, error) {
	f, err := create(path)
	if err != nil {
		return nil, err
	}
	r := &Recorder{f: f, w: bufio.NewWriter(f), now: time.Now, Path: path}
	r.start = r.now()
	h, _ := json.Marshal(Header{Version: 2, Width: cols, Height: rows, Timestamp: r.start.Unix(), Title: title,
		Env: map[string]string{"TERM": "xterm-256color"}})
	r.w.Write(h)
	r.w.WriteByte('\n')
	return r, nil
}

// Write records output.
func (r *Recorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	data := append(r.pending, p...)
	// Keep a UTF-8 sequence cut between frames for the next event.
	cut := len(data)
	for i := len(data) - 1; i >= 0 && i >= len(data)-utf8.UTFMax; i-- {
		if utf8.RuneStart(data[i]) {
			if !utf8.FullRune(data[i:]) {
				cut = i
			}
			break
		}
	}
	r.pending = append([]byte(nil), data[cut:]...)
	if cut > 0 {
		r.event("o", strings.ToValidUTF8(string(data[:cut]), "�"))
	}
	return len(p), nil
}

// Resize records a new terminal size.
func (r *Recorder) Resize(cols, rows int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.event("r", itoa(cols)+"x"+itoa(rows))
}

func (r *Recorder) event(kind, data string) {
	b, _ := json.Marshal([]any{float64(r.now().Sub(r.start).Microseconds()) / 1e6, kind, data})
	r.w.Write(b)
	r.w.WriteByte('\n')
}

// Close ends the recording.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.pending) > 0 {
		r.event("o", strings.ToValidUTF8(string(r.pending), "�"))
	}
	r.w.Flush()
	return r.f.Close()
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
