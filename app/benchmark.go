package app

import (
	"context"
	"io"
	"os"
	"strconv"
	"sync"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/sshengine"
)

// OpenBenchmark opens a pane that measures the terminal: a session that
// prints megabytes of numbered lines (like `seq`) as fast as the stream
// takes them, then ends. It needs no shell or server, so the same test
// runs on every system; the frontend times it from the first byte to the
// last one drawn.
func (s *SessionService) OpenBenchmark(_ context.Context, cols, rows, megabytes int) (SessionInfo, error) {
	if megabytes < 1 || megabytes > 64 {
		megabytes = 8
	}
	sessCtx, cancel := context.WithCancel(context.Background())
	sess := &session{id: uuid.NewString(), ctx: sessCtx, cancel: cancel, ready: make(chan struct{}), flow: newFlowGate(),
		cols: cols, rows: rows, local: true, capture: &capture{name: "benchmark"}}
	sess.shell = &benchProcess{left: megabytes << 20}
	s.mu.Lock()
	s.sessions[sess.id] = sess
	s.mu.Unlock()
	close(sess.ready)
	s.emit(EventSessionState, SessionState{ID: sess.id, State: StateConnected})
	host, _ := os.Hostname()
	return SessionInfo{ID: sess.id, Title: "benchmark " + strconv.Itoa(megabytes) + " MB", Target: sshengine.Target{User: "shellvane", Host: host},
		Group: []string{}, Via: []string{}, Local: true}, nil
}

// benchProcess prints "1\r\n2\r\n…" until left bytes are out.
type benchProcess struct {
	mu   sync.Mutex
	left int
	n    int
	tail []byte // part of a line that did not fit the last Read
}

func (b *benchProcess) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.left <= 0 && len(b.tail) == 0 {
		return 0, io.EOF
	}
	w := copy(p, b.tail)
	b.tail = b.tail[w:]
	for w < len(p) && b.left > 0 {
		b.n++
		line := strconv.AppendInt(nil, int64(b.n), 10)
		line = append(line, '\r', '\n')
		if len(line) > b.left {
			line = line[:b.left]
		}
		b.left -= len(line)
		c := copy(p[w:], line)
		w += c
		if c < len(line) {
			b.tail = append(b.tail, line[c:]...)
		}
	}
	return w, nil
}

func (b *benchProcess) Write(p []byte) (int, error) { return len(p), nil }
func (b *benchProcess) Resize(int, int) error       { return nil }
func (b *benchProcess) Wait() int                   { return 0 }
func (b *benchProcess) Close() error                { b.mu.Lock(); b.left, b.tail = 0, nil; b.mu.Unlock(); return nil }
