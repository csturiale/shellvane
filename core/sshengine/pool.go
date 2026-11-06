package sshengine

import (
	"context"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Pool shares one SSH connection among the sessions to the same target, like
// OpenSSH's ControlMaster: a split pane opens a new channel instead of a new
// login. A connection is closed when its last user releases it.
type Pool struct {
	mu      sync.Mutex
	entries map[string]*poolEntry
}

type poolEntry struct {
	ready  chan struct{} // closed once the dial finished
	client *ssh.Client
	err    error
	refs   int
}

// NewPool returns an empty pool.
func NewPool() *Pool {
	return &Pool{entries: map[string]*poolEntry{}}
}

// Key identifies a shareable connection.
func (t Target) Key() string { return t.String() }

// Acquire returns a connection for key, dialing it if needed. Concurrent
// callers for the same key wait for a single dial. release must be called
// exactly once when the caller no longer uses the connection.
func (p *Pool) Acquire(ctx context.Context, key string, dial func(context.Context) (*ssh.Client, error)) (*ssh.Client, func(), error) {
	p.mu.Lock()
	e, ok := p.entries[key]
	if !ok {
		e = &poolEntry{ready: make(chan struct{})}
		p.entries[key] = e
	}
	e.refs++
	p.mu.Unlock()

	if !ok {
		e.client, e.err = dial(ctx)
		if e.err != nil {
			p.mu.Lock()
			delete(p.entries, key)
			p.mu.Unlock()
		} else {
			go p.watch(key, e)
		}
		close(e.ready)
	} else {
		select {
		case <-e.ready:
		case <-ctx.Done():
			p.release(key, e)
			return nil, nil, ctx.Err()
		}
	}
	if e.err != nil {
		return nil, nil, e.err
	}
	var once sync.Once
	return e.client, func() { once.Do(func() { p.release(key, e) }) }, nil
}

// Size reports how many connections are open; for tests and diagnostics.
func (p *Pool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entries)
}

func (p *Pool) release(key string, e *poolEntry) {
	p.mu.Lock()
	e.refs--
	last := e.refs == 0
	if last && p.entries[key] == e {
		delete(p.entries, key)
	}
	p.mu.Unlock()
	if last && e.client != nil {
		e.client.Close()
	}
}

// watch forgets a connection as soon as it dies, so the next Acquire dials
// again instead of handing out a dead client.
func (p *Pool) watch(key string, e *poolEntry) {
	e.client.Wait()
	p.mu.Lock()
	if p.entries[key] == e {
		delete(p.entries, key)
	}
	p.mu.Unlock()
}

// KeepAlive sends OpenSSH keepalive requests every interval until the
// connection closes, and closes it after maxMissed unanswered requests.
func KeepAlive(client *ssh.Client, interval time.Duration, maxMissed int) {
	if interval <= 0 {
		return
	}
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		missed := 0
		for range t.C {
			done := make(chan error, 1)
			go func() {
				_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					return // connection closed
				}
				missed = 0
			case <-time.After(interval):
				missed++
				if missed >= maxMissed {
					client.Close()
					return
				}
			}
		}
	}()
}
