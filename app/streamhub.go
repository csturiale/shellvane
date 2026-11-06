package app

import (
	"sync"
)

// replaySize is how much recent output a session keeps, so a terminal
// moved to another window shows its screen and scrollback again.
const replaySize = 512 << 10

// pendingMax bounds the output kept while no window shows the session.
const pendingMax = 4 << 20

// outputHub delivers a session's output to the stream currently attached,
// one at a time. A single pump reads the shell for the session's whole
// life, so a terminal can move between windows without losing output.
type outputHub struct {
	mu           sync.Mutex
	sink         func([]byte) error
	sinkID       int
	nextID       int
	pending      [][]byte // output no stream received
	pendingBytes int
	ring         []byte // the last replaySize bytes of output
	started      bool
	done         chan struct{} // closed when the shell's output ends
}

func (h *outputHub) doneCh() chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.done == nil {
		h.done = make(chan struct{})
	}
	return h.done
}

// deliver sends a frame to the attached stream or keeps it for the next.
func (h *outputHub) deliver(frame []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ring = append(h.ring, frame...)
	if over := len(h.ring) - replaySize; over > 0 {
		h.ring = append([]byte(nil), h.ring[over:]...)
	}
	if h.sink != nil {
		if h.sink(frame) == nil {
			return
		}
		h.sink = nil // the window went away: keep the output for the next one
	}
	h.pending = append(h.pending, frame)
	h.pendingBytes += len(frame)
	for h.pendingBytes > pendingMax && len(h.pending) > 1 {
		h.pendingBytes -= len(h.pending[0])
		h.pending = h.pending[1:]
	}
}

// attach makes send the output's destination, first sending what it
// missed: the recent output when replay is set (a terminal opened in a new
// window), else only the output no stream received. start runs once, to
// launch the pump. It returns the attachment's id, or -1 if send failed.
func (h *outputHub) attach(send func([]byte) error, replay bool, start func()) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	var backlog [][]byte
	if replay {
		if len(h.ring) > 0 {
			backlog = [][]byte{append([]byte(nil), h.ring...)}
		}
	} else {
		backlog = h.pending
	}
	h.pending, h.pendingBytes = nil, 0
	for _, f := range backlog {
		if send(f) != nil {
			return -1
		}
	}
	h.nextID++
	h.sink, h.sinkID = send, h.nextID
	if !h.started {
		h.started = true
		go start()
	}
	return h.sinkID
}

// release detaches the stream id, if it is still the current one.
func (h *outputHub) release(id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sinkID == id {
		h.sink = nil
	}
}

// attachedSince reports whether a stream other than id is attached now.
func (h *outputHub) attachedSince(id int) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sink != nil && h.sinkID != id
}

// finish marks the end of the output.
func (h *outputHub) finish() {
	ch := h.doneCh()
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-ch:
	default:
		close(ch)
	}
}
