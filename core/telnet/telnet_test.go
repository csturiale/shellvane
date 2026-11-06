package telnet

import (
	"bytes"
	"context"
	"io"
	"net"
	"strconv"
	"testing"
	"time"
)

// A fake server: asks for the terminal type and size, prints a banner with
// an escaped IAC, and records what the client sends.
func TestNegotiationAndData(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	got := make(chan []byte, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		c.Write([]byte{iac, do, optNAWS, iac, do, optTType, iac, sb, optTType, ttypeSend, iac, se, iac, will, optEcho, iac, do, 99})
		c.Write([]byte("login: \xff\xff\r\x00ok\r\n"))
		c.SetReadDeadline(time.Now().Add(2 * time.Second))
		var buf bytes.Buffer
		tmp := make([]byte, 256)
		for !bytes.Contains(buf.Bytes(), []byte("root\r\x00")) || !bytes.Contains(buf.Bytes(), []byte{iac, sb, optNAWS, 0, 100}) {
			n, err := c.Read(tmp)
			if err != nil {
				break
			}
			buf.Write(tmp[:n])
		}
		got <- buf.Bytes()
	}()

	_, port, _ := net.SplitHostPort(ln.Addr().String())
	p, _ := strconv.Atoi(port)
	conn, err := Dial(context.Background(), Options{Host: "127.0.0.1", Port: p, Cols: 80, Rows: 24, Term: "vt100"})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	want := "login: \xff\rok\r\n"
	var out bytes.Buffer
	buf := make([]byte, 64)
	for out.Len() < len(want) {
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		out.Write(buf[:n])
	}
	if out.String() != want {
		t.Fatalf("data = %q, want %q", out.String(), want)
	}
	conn.Resize(100, 30)
	conn.Write([]byte("root\r"))

	sent := <-got
	for _, w := range [][]byte{
		{iac, will, optNAWS},
		{iac, sb, optNAWS, 0, 80, 0, 24, iac, se},
		{iac, sb, optNAWS, 0, 100, 0, 30, iac, se},
		append(append([]byte{iac, sb, optTType, ttypeIs}, "vt100"...), iac, se),
		{iac, do, optEcho},
		{iac, wont, 99},
		[]byte("root\r\x00"),
	} {
		if !bytes.Contains(sent, w) {
			t.Errorf("client did not send %v; sent %v", w, sent)
		}
	}
}

func TestWaitEndsWhenServerCloses(t *testing.T) {
	a, b := net.Pipe()
	go io.Copy(io.Discard, b) // drain the client's offers
	conn := newConn(a, Options{})
	go func() { time.Sleep(20 * time.Millisecond); b.Close() }()
	go io.Copy(io.Discard, conn)
	done := make(chan int)
	go func() { done <- conn.Wait() }()
	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("Wait = %d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return")
	}
}

func TestDialFailure(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().(*net.TCPAddr)
	ln.Close()
	if _, err := Dial(context.Background(), Options{Host: "127.0.0.1", Port: addr.Port, Timeout: time.Second}); err == nil {
		t.Fatal("dial to a closed port succeeded")
	}
}
