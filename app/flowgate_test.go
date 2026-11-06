package app

import (
	"context"
	"testing"
	"time"
)

func TestFlowGate(t *testing.T) {
	g := newFlowGate()
	if !g.wait(context.Background()) {
		t.Fatal("an open gate must not block")
	}

	g.pause()
	g.pause() // idempotent
	passed := make(chan bool)
	go func() { passed <- g.wait(context.Background()) }()
	select {
	case <-passed:
		t.Fatal("a paused gate must block")
	case <-time.After(50 * time.Millisecond):
	}

	g.open()
	g.open() // idempotent
	if !<-passed {
		t.Fatal("wait should succeed after open")
	}

	g.pause()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if g.wait(ctx) {
		t.Fatal("wait must give up when the context ends")
	}
}
