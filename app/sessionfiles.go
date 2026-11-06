package app

import (
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/sessionlog"
)

// ErrEncoding reports an unknown character encoding.
const ErrEncoding errs.Code = "session.badEncoding"

// capture holds a session's log and recording.
type capture struct {
	mu       sync.Mutex
	name     string // for file names
	logger   *sessionlog.Logger
	recorder *sessionlog.Recorder
}

func (c *capture) write(p []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logger != nil {
		c.logger.Write(p)
	}
	if c.recorder != nil {
		c.recorder.Write(p)
	}
}

func (c *capture) resize(cols, rows int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recorder != nil {
		c.recorder.Resize(cols, rows)
	}
}

func (c *capture) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logger != nil {
		c.logger.Close()
		c.logger = nil
	}
	if c.recorder != nil {
		c.recorder.Close()
		c.recorder = nil
	}
}

// filesDir holds session logs (logs/) and recordings (recordings/).
func (s *SessionService) filesDir() string {
	return filepath.Join(filepath.Dir(s.kubeDir()), "sessions")
}

// StartLogging writes the session's output to a new log; format is "raw",
// "text" or "timestamped". It returns the file path.
func (s *SessionService) StartLogging(id, format string) (string, error) {
	sess, err := s.get(id)
	if err != nil {
		return "", err
	}
	c := sess.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logger != nil {
		return c.logger.Path, nil
	}
	now := time.Now()
	path := filepath.Join(s.filesDir(), "logs", now.Format("2006-01-02"), sessionlog.FileName(c.name, now, ".log"))
	l, err := sessionlog.NewLogger(path, format)
	if err != nil {
		return "", err
	}
	c.logger = l
	return path, nil
}

// StopLogging closes the session's log.
func (s *SessionService) StopLogging(id string) error {
	sess, err := s.get(id)
	if err != nil {
		return err
	}
	c := sess.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logger != nil {
		c.logger.Close()
		c.logger = nil
	}
	return nil
}

// StartRecording records the session in asciinema v2 format; it returns
// the file path.
func (s *SessionService) StartRecording(id string) (string, error) {
	sess, err := s.get(id)
	if err != nil {
		return "", err
	}
	sess.sizeMu.Lock()
	cols, rows := sess.cols, sess.rows
	sess.sizeMu.Unlock()
	c := sess.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recorder != nil {
		return c.recorder.Path, nil
	}
	now := time.Now()
	path := filepath.Join(s.filesDir(), "recordings", sessionlog.FileName(c.name, now, ".cast"))
	r, err := sessionlog.NewRecorder(path, c.name, cols, rows)
	if err != nil {
		return "", err
	}
	c.recorder = r
	return path, nil
}

// StopRecording ends the session's recording.
func (s *SessionService) StopRecording(id string) error {
	sess, err := s.get(id)
	if err != nil {
		return err
	}
	c := sess.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recorder != nil {
		c.recorder.Close()
		c.recorder = nil
	}
	return nil
}

// SessionFiles lists saved logs and recordings, newest first.
func (s *SessionService) SessionFiles() ([]sessionlog.File, error) {
	return sessionlog.List(s.filesDir())
}

// ReadSessionFile returns a log or recording (for the viewer and player).
func (s *SessionService) ReadSessionFile(path string) (string, error) {
	return sessionlog.Read(s.filesDir(), path)
}

// DeleteSessionFile removes a log or recording.
func (s *SessionService) DeleteSessionFile(path string) error {
	return sessionlog.Delete(s.filesDir(), path)
}

// SessionFilesDir is the folder of logs and recordings, to show the user.
func (s *SessionService) SessionFilesDir() string {
	return s.filesDir()
}

// charset returns the encoding of a host that does not use UTF-8, or nil.
func charset(name string) (encoding.Encoding, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" || n == "utf-8" || n == "utf8" {
		return nil, nil
	}
	enc, err := htmlindex.Get(n)
	if err != nil {
		return nil, errs.New(ErrEncoding, errs.Params{"encoding": name})
	}
	return enc, nil
}

// encodedShell converts a shell's output to UTF-8 and the keystrokes to
// the host's encoding.
type encodedShell struct {
	terminalProcess
	r   io.Reader
	enc *encoding.Encoder
}

func withCharset(t terminalProcess, enc encoding.Encoding) terminalProcess {
	if enc == nil {
		return t
	}
	return &encodedShell{terminalProcess: t, r: transform.NewReader(t, enc.NewDecoder()), enc: enc.NewEncoder()}
}

func (e *encodedShell) Read(p []byte) (int, error) { return e.r.Read(p) }

func (e *encodedShell) Write(p []byte) (int, error) {
	// Keystrokes are short: encode rune by rune, so a character the host
	// cannot show becomes '?' (the encoders would send SUB, 0x1A).
	var out []byte
	for _, r := range string(p) {
		b, err := e.enc.Bytes([]byte(string(r)))
		if err != nil || (len(b) == 1 && b[0] == 0x1a && r != 0x1a) {
			b = []byte{'?'}
		}
		out = append(out, b...)
	}
	if _, err := e.terminalProcess.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}
