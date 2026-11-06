package netprobe

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

func TestMagicPacketAndWake(t *testing.T) {
	mac, err := ParseMAC("AA-BB-CC-DD-EE-FF")
	if err != nil {
		t.Fatal(err)
	}
	p := MagicPacket(mac)
	if len(p) != 102 || !bytes.Equal(p[:6], []byte{255, 255, 255, 255, 255, 255}) || !bytes.Equal(p[96:], mac) {
		t.Fatalf("packet = %x", p)
	}
	if _, err := ParseMAC("nope"); errs.CodeOf(err) != ErrBadMAC {
		t.Fatalf("bad mac: %v", err)
	}
	// A listener on loopback stands in for the broadcast address.
	ln, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if err := Wake("aa:bb:cc:dd:ee:ff", ln.LocalAddr().String()); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 200)
	ln.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := ln.ReadFrom(buf)
	if err != nil || !bytes.Equal(buf[:n], p) {
		t.Fatalf("received %x, %v", buf[:n], err)
	}
}

func TestKnockAndProbe(t *testing.T) {
	steps, err := ParseKnock("7000, 8000/udp,9000/TCP")
	if err != nil || len(steps) != 3 || !steps[1].UDP || steps[2].UDP {
		t.Fatalf("ParseKnock = %+v, %v", steps, err)
	}
	for _, bad := range []string{"", "70000", "80/icmp", "x"} {
		if _, err := ParseKnock(bad); errs.CodeOf(err) != ErrBadKnock {
			t.Errorf("ParseKnock(%q) = %v", bad, err)
		}
	}
	// The knock reaches a TCP and a UDP listener in order.
	tcp, _ := net.Listen("tcp", "127.0.0.1:0")
	defer tcp.Close()
	udp, _ := net.ListenPacket("udp", "127.0.0.1:0")
	defer udp.Close()
	got := make(chan string, 2)
	go func() {
		if c, err := tcp.Accept(); err == nil {
			got <- "tcp"
			c.Close()
		}
	}()
	go func() {
		buf := make([]byte, 10)
		if _, _, err := udp.ReadFrom(buf); err == nil {
			got <- "udp"
		}
	}()
	tp := tcp.Addr().(*net.TCPAddr).Port
	up := udp.LocalAddr().(*net.UDPAddr).Port
	steps = []KnockStep{{Port: tp}, {Port: up, UDP: true}}
	if err := Knock(context.Background(), "127.0.0.1", steps, 50*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"tcp", "udp"} {
		select {
		case g := <-got:
			if g != want {
				t.Fatalf("knock order: got %s, want %s", g, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no %s knock", want)
		}
	}
	if d, err := Probe(context.Background(), "127.0.0.1:"+strconv.Itoa(tp), time.Second); err != nil || d < 0 {
		t.Fatalf("Probe = %v, %v", d, err)
	}
}
