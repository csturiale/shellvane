package tunnel

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// echoServer answers every line with "echo: <line>".
func echoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					io.WriteString(c, "echo: "+line)
				}
			}()
		}
	}()
	return ln.Addr().String()
}

func splitAddr(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return host, p
}

var plainDial Dialer = func(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, addr)
}

func roundTrip(t *testing.T, c net.Conn, msg string) string {
	t.Helper()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	io.WriteString(c, msg+"\n")
	line, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(line)
}

func TestFixedForward(t *testing.T) {
	host, port := splitAddr(t, echoServer(t))
	r, err := StartWith(context.Background(), Spec{Kind: Local, BindAddr: "127.0.0.1", DestHost: host, DestPort: port}, net.Listen, plainDial)
	if err != nil {
		t.Fatal(err)
	}
	c, err := net.Dial("tcp", r.Addr)
	if err != nil {
		t.Fatal(err)
	}
	if got := roundTrip(t, c, "hello"); got != "echo: hello" {
		t.Fatalf("got %q", got)
	}
	// The client can read the echo before the forwarder counts either direction.
	st := r.Stats()
	for deadline := time.Now().Add(2 * time.Second); (st.BytesIn != 12 || st.BytesOut != 6) && time.Now().Before(deadline); st = r.Stats() {
		time.Sleep(5 * time.Millisecond)
	}
	if st.Active != 1 || st.Total != 1 || st.BytesOut != 6 || st.BytesIn != 12 {
		t.Errorf("stats = %+v", st)
	}
	c.Close()

	r.Stop()
	if _, err := net.Dial("tcp", r.Addr); err == nil {
		t.Fatal("listener should be closed after Stop")
	}
	if err, ok := <-r.Done(); ok && err != nil {
		t.Fatalf("stopping on purpose is not an error: %v", err)
	}
}

func TestSocks5(t *testing.T) {
	echo := echoServer(t)
	r, err := StartWith(context.Background(), Spec{Kind: Dynamic}, net.Listen, plainDial)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()

	c, err := net.Dial("tcp", r.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	// Greeting with "no authentication".
	c.Write([]byte{5, 1, 0})
	resp := make([]byte, 2)
	io.ReadFull(c, resp)
	if resp[0] != 5 || resp[1] != 0 {
		t.Fatalf("greeting reply %v", resp)
	}
	// CONNECT by domain name.
	_, port := splitAddr(t, echo)
	req := []byte{5, 1, 0, 3, byte(len("localhost"))}
	req = append(req, "localhost"...)
	req = binary.BigEndian.AppendUint16(req, uint16(port))
	c.Write(req)
	reply := make([]byte, 10)
	io.ReadFull(c, reply)
	if reply[1] != 0 {
		t.Fatalf("connect reply %v", reply)
	}
	if got := roundTrip(t, c, "via socks"); got != "echo: via socks" {
		t.Fatalf("got %q", got)
	}
}

func TestSocksRefusedTarget(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	_, closedPort := splitAddr(t, ln.Addr().String())
	ln.Close()
	r, err := StartWith(context.Background(), Spec{Kind: Dynamic}, net.Listen, plainDial)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	c, _ := net.Dial("tcp", r.Addr)
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	c.Write([]byte{5, 1, 0})
	io.ReadFull(c, make([]byte, 2))
	c.Write(binary.BigEndian.AppendUint16([]byte{5, 1, 0, 1, 127, 0, 0, 1}, uint16(closedPort)))
	reply := make([]byte, 10)
	io.ReadFull(c, reply)
	if reply[1] != 5 {
		t.Fatalf("expected 'connection refused' reply, got %v", reply)
	}
	if r.Stats().Errors != 1 {
		t.Errorf("errors = %d", r.Stats().Errors)
	}
}

func TestValidationAndBindFailure(t *testing.T) {
	_, err := StartWith(context.Background(), Spec{Kind: Local, DestPort: 80}, net.Listen, plainDial)
	if errs.CodeOf(err) != ErrInvalid {
		t.Fatalf("err = %v", err)
	}
	busy, _ := net.Listen("tcp", "127.0.0.1:0")
	defer busy.Close()
	_, p := splitAddr(t, busy.Addr().String())
	_, err = StartWith(context.Background(), Spec{Kind: Dynamic, BindPort: p}, net.Listen, plainDial)
	if errs.CodeOf(err) != ErrBindFailed {
		t.Fatalf("err = %v", err)
	}
}

func TestForwardToUnixSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix sockets")
	}
	path := filepath.Join(t.TempDir(), "app.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		line, _ := bufio.NewReader(c).ReadString('\n')
		fmt.Fprintf(c, "socket: %s", line)
	}()
	spec := Spec{Kind: Local, BindAddr: "127.0.0.1", DestHost: path, DestPort: 99}
	r, err := StartWith(context.Background(), spec, net.Listen, plainDial)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	c, err := net.Dial("tcp", r.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	fmt.Fprintln(c, "ping")
	got, _ := bufio.NewReader(c).ReadString('\n')
	if got != "socket: ping\n" {
		t.Fatalf("got %q", got)
	}
}
