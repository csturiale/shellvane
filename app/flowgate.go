package app

import (
	"context"
	"sync"
)

// flowGate pauses the reader of a terminal while the frontend catches up.
// It is opened and closed by the frontend from its own buffer watermarks.
type flowGate struct {
	mu     sync.Mutex
	paused bool
	resume chan struct{} // closed when the gate reopens
}

func newFlowGate() *flowGate {
	return &flowGate{resume: make(chan struct{})}
}

func (g *flowGate) pause() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.paused {
		g.paused = true
		g.resume = make(chan struct{})
	}
}

func (g *flowGate) open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.paused {
		g.paused = false
		close(g.resume)
	}
}

// wait blocks while the gate is paused. It returns false if ctx ended.
func (g *flowGate) wait(ctx context.Context) bool {
	g.mu.Lock()
	paused, ch := g.paused, g.resume
	g.mu.Unlock()
	if !paused {
		return true
	}
	select {
	case <-ch:
		return true
	case <-ctx.Done():
		return false
	}
}
