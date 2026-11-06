package app

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestOutputHubHandsOverBetweenStreams(t *testing.T) {
	var h outputHub
	var first, second []string
	started := make(chan struct{})
	h.deliver([]byte("before any window "))
	id := h.attach(func(b []byte) error { first = append(first, string(b)); return nil }, false, func() { close(started) })
	<-started
	h.deliver([]byte("one "))
	h.release(id) // the window closes while the terminal moves

	h.deliver([]byte("two "))
	// The old window's stream may still fail a send: the frame is kept.
	h.sink = func([]byte) error { return errors.New("closed") }
	h.deliver([]byte("three "))

	h.attach(func(b []byte) error { second = append(second, string(b)); return nil }, true, func() { t.Fatal("pump started twice") })
	h.deliver([]byte("four"))
	if strings.Join(first, "") != "before any window one " {
		t.Errorf("first window got %q", first)
	}
	if got := strings.Join(second, ""); got != "before any window one two three four" {
		t.Errorf("second window got %q (replay plus new output)", got)
	}

	// Without replay, a reconnecting stream gets only what it missed.
	var third []string
	h.sink = nil
	h.deliver([]byte("five"))
	h.attach(func(b []byte) error { third = append(third, string(b)); return nil }, false, nil)
	if strings.Join(third, "") != "five" {
		t.Errorf("reattach got %q", third)
	}

	h.finish()
	h.finish() // twice is fine
	select {
	case <-h.doneCh():
	case <-time.After(time.Second):
		t.Fatal("done not closed")
	}
}

func TestOutputHubBoundsKeptOutput(t *testing.T) {
	var h outputHub
	chunk := make([]byte, 1<<20)
	for i := 0; i < 10; i++ {
		h.deliver(chunk)
	}
	if h.pendingBytes > pendingMax || len(h.ring) != replaySize {
		t.Fatalf("pending %d bytes, ring %d", h.pendingBytes, len(h.ring))
	}
}

func TestOutputHubAttachedSince(t *testing.T) {
	var h outputHub
	id := h.attach(func([]byte) error { return nil }, false, func() {})
	if h.attachedSince(id) {
		t.Fatal("the same stream is not a new one")
	}
	h.release(id)
	if h.attachedSince(id) {
		t.Fatal("nothing attached")
	}
	h.attach(func([]byte) error { return nil }, false, nil)
	if !h.attachedSince(id) {
		t.Fatal("a new stream is attached")
	}
}
