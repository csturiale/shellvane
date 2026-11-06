// Package tunnel implements SSH port forwarding: local (-L), remote (-R) and
// dynamic SOCKS5 (-D). The forwarding loop only needs a listener and a dial
// function, so it does not depend on SSH and is tested without it.
package tunnel

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
)

// Dialer opens the far side of a forwarded connection.
type Dialer func(ctx context.Context, network, addr string) (net.Conn, error)

// Stats are live counters of a running tunnel.
type Stats struct {
	Active   int64 `json:"active"`   // open connections
	Total    int64 `json:"total"`    // connections accepted so far
	BytesIn  int64 `json:"bytesIn"`  // from the far side to the client
	BytesOut int64 `json:"bytesOut"` // from the client to the far side
	Errors   int64 `json:"errors"`   // connections that could not be forwarded
}

type counters struct {
	active, total, in, out, errors atomic.Int64
}

func (c *counters) snapshot() Stats {
	return Stats{Active: c.active.Load(), Total: c.total.Load(), BytesIn: c.in.Load(), BytesOut: c.out.Load(), Errors: c.errors.Load()}
}

// target returns the address to dial for an accepted connection; for SOCKS it
// performs the handshake on conn first.
type target func(conn net.Conn) (string, error)

// serve accepts on ln until ctx ends or ln fails, forwarding each connection
// to the address chosen by pick through dial.
func serve(ctx context.Context, ln net.Listener, pick target, dial Dialer, c *counters) error {
	stop := context.AfterFunc(ctx, func() { ln.Close() })
	defer stop()
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		c.total.Add(1)
		wg.Add(1)
		go func() {
			defer wg.Done()
			forward(ctx, conn, pick, dial, c)
		}()
	}
}

func forward(ctx context.Context, conn net.Conn, pick target, dial Dialer, c *counters) {
	defer conn.Close()
	addr, err := pick(conn)
	if err != nil {
		c.errors.Add(1)
		return
	}
	network := "tcp"
	if strings.HasPrefix(addr, "/") {
		network = "unix" // a Unix socket on the other side
	}
	remote, err := dial(ctx, network, addr)
	if err != nil {
		c.errors.Add(1)
		if r, ok := conn.(interface{ replyFailure() }); ok {
			r.replyFailure()
		}
		return
	}
	defer remote.Close()
	if s, ok := conn.(*socksConn); ok {
		if err := s.replySuccess(); err != nil {
			return
		}
	}
	c.active.Add(1)
	defer c.active.Add(-1)

	// Close both ends when either side finishes or the tunnel stops.
	stop := context.AfterFunc(ctx, func() { conn.Close(); remote.Close() })
	defer stop()
	done := make(chan struct{}, 2)
	go func() { pipe(remote, conn, &c.out); done <- struct{}{} }()
	go func() { pipe(conn, remote, &c.in); done <- struct{}{} }()
	<-done
	conn.Close()
	remote.Close()
	<-done
}

func pipe(dst io.Writer, src io.Reader, n *atomic.Int64) {
	buf := make([]byte, 32*1024)
	for {
		r, err := src.Read(buf)
		if r > 0 {
			w, werr := dst.Write(buf[:r])
			n.Add(int64(w))
			if werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// fixed always forwards to the same address (-L and -R).
func fixed(addr string) target {
	return func(net.Conn) (string, error) { return addr, nil }
}
