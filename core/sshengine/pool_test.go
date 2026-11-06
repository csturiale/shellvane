package sshengine

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/knownhosts"
)

func TestPoolSharesOneConnection(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	srv := startServer(t, "s3cret")
	pool := NewPool()
	var dials atomic.Int32
	dial := func(ctx context.Context) (*ssh.Client, error) {
		dials.Add(1)
		p := &fakePrompter{passwords: []string{"s3cret"}, decision: knownhosts.AcceptOnce}
		return Dial(ctx, srv.target(t), options(t, p))
	}

	// Three concurrent users of the same target: one dial.
	var wg sync.WaitGroup
	clients := make([]*ssh.Client, 3)
	releases := make([]func(), 3)
	for i := range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, rel, err := pool.Acquire(context.Background(), "k", dial)
			if err != nil {
				t.Error(err)
				return
			}
			clients[i], releases[i] = c, rel
		}()
	}
	wg.Wait()
	if dials.Load() != 1 || clients[0] != clients[1] || clients[1] != clients[2] {
		t.Fatalf("dials = %d; clients must be shared", dials.Load())
	}

	// Every user can open its own shell on the shared connection.
	for i := range 3 {
		sh, err := StartShell(clients[i], releases[i], "xterm", 80, 24, false)
		if err != nil {
			t.Fatal(err)
		}
		if i < 2 {
			sh.Close()
		} else {
			defer sh.Close()
		}
	}
	if pool.Size() != 1 {
		t.Fatalf("connection closed while still in use (size %d)", pool.Size())
	}
	releases[2]() // releasing twice is harmless: the shell's Close releases again
	if pool.Size() != 0 {
		t.Fatalf("last release must close the connection (size %d)", pool.Size())
	}
}

func TestPoolDialErrorIsNotCached(t *testing.T) {
	pool := NewPool()
	fail := func(context.Context) (*ssh.Client, error) { return nil, errors.New("boom") }
	if _, _, err := pool.Acquire(context.Background(), "k", fail); err == nil {
		t.Fatal("expected the dial error")
	}
	if pool.Size() != 0 {
		t.Fatal("failed dials must not stay in the pool")
	}
}

func TestPoolForgetsDeadConnections(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	srv := startServer(t, "s3cret")
	pool := NewPool()
	dial := func(ctx context.Context) (*ssh.Client, error) {
		p := &fakePrompter{passwords: []string{"s3cret"}, decision: knownhosts.AcceptOnce}
		return Dial(ctx, srv.target(t), options(t, p))
	}
	c, release, err := pool.Acquire(context.Background(), "k", dial)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	c.Close() // the network drops
	deadline := time.Now().Add(2 * time.Second)
	for pool.Size() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if pool.Size() != 0 {
		t.Fatal("a dead connection must leave the pool")
	}
}
