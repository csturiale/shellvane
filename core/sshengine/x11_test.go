package sshengine

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func setupPacket(order binary.ByteOrder, b byte, name string, data []byte) []byte {
	p := make([]byte, 12)
	p[0] = b
	order.PutUint16(p[2:4], 11)
	order.PutUint16(p[6:8], uint16(len(name)))
	order.PutUint16(p[8:10], uint16(len(data)))
	p = append(p, padded([]byte(name))...)
	return append(p, padded(data)...)
}

func TestX11Rewrite(t *testing.T) {
	x := &X11{fake: []byte("0123456789abcdef"), real: []byte("REALCOOKIE123456")}
	for _, c := range []struct {
		order binary.ByteOrder
		b     byte
	}{{binary.LittleEndian, 'l'}, {binary.BigEndian, 'B'}} {
		in := setupPacket(c.order, c.b, x11Proto, x.fake)
		got, err := readSetup(bytes.NewReader(append(in, []byte("more data")...)))
		if err != nil || !bytes.Equal(got, in) {
			t.Fatalf("readSetup = %x, %v", got, err)
		}
		out, err := x.rewrite(got)
		if err != nil {
			t.Fatal(err)
		}
		if want := setupPacket(c.order, c.b, x11Proto, x.real); !bytes.Equal(out, want) {
			t.Fatalf("rewrite = %x, want %x", out, want)
		}
	}
	if _, err := x.rewrite(setupPacket(binary.LittleEndian, 'l', x11Proto, []byte("guessed-cookie!!"))); err == nil {
		t.Fatal("a wrong cookie must be refused")
	}
	x.real = nil
	out, _ := x.rewrite(setupPacket(binary.LittleEndian, 'l', x11Proto, x.fake))
	if len(out) != 12 || out[6] != 0 || out[8] != 0 {
		t.Fatalf("no real cookie = %x", out)
	}
}

func TestDisplayParts(t *testing.T) {
	for in, want := range map[string]struct {
		host string
		n    int
	}{":0": {"", 0}, ":1.0": {"", 1}, "localhost:10.0": {"localhost", 10}, "/private/tmp/com.apple.launchd.x/org.xquartz:0": {"/private/tmp/com.apple.launchd.x/org.xquartz", 0}} {
		h, n, ok := displayParts(in)
		if !ok || h != want.host || n != want.n {
			t.Errorf("displayParts(%q) = %q %d %v", in, h, n, ok)
		}
	}
	if displayScreen("localhost:10.2") != 2 || displayScreen(":0") != 0 {
		t.Error("screen")
	}
}
