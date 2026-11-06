package sshengine

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// x11Proto is the only X authorization protocol forwarded.
const x11Proto = "MIT-MAGIC-COOKIE-1"

// X11 forwards X11 programs started on the server to the local display,
// like ssh -X: the server gets a random cookie, and each X connection it
// opens back must present it; Shellvane then swaps it for the display's
// real cookie (from xauth), so the real one never leaves this computer.
type X11 struct {
	Display string // local DISPLAY (":0", "localhost:10.0", a socket path)
	fake    []byte
	real    []byte // nil when the display needs no cookie
	once    sync.Once
}

// NewX11 prepares forwarding to display; "" uses the platform default.
func NewX11(display string) *X11 {
	if display == "" && runtime.GOOS == "windows" {
		display = "localhost:0" // VcXsrv, X410, Xming
	}
	x := &X11{Display: display, fake: make([]byte, 16)}
	rand.Read(x.fake)
	x.real = xauthCookie(display)
	return x
}

// Request asks the server to forward X11 for a session (before the shell
// starts) and serves the X channels of the session's connection.
func (x *X11) Request(client *ssh.Client, session *ssh.Session) error {
	if x.Display == "" {
		return errors.New("no local X display")
	}
	x.once.Do(func() { go x.serve(client.HandleChannelOpen("x11")) })
	payload := ssh.Marshal(struct {
		Single bool
		Proto  string
		Cookie string
		Screen uint32
	}{false, x11Proto, hex.EncodeToString(x.fake), uint32(displayScreen(x.Display))})
	ok, err := session.SendRequest("x11-req", true, payload)
	if err == nil && !ok {
		err = errors.New("X11 forwarding refused by the server")
	}
	return err
}

func (x *X11) serve(chans <-chan ssh.NewChannel) {
	if chans == nil {
		return // already served by another session of this connection
	}
	for nc := range chans {
		go x.handle(nc)
	}
}

func (x *X11) handle(nc ssh.NewChannel) {
	ch, reqs, err := nc.Accept()
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	defer ch.Close()
	setup, err := readSetup(ch)
	if err != nil {
		return
	}
	setup, err = x.rewrite(setup)
	if err != nil {
		return // wrong cookie: refused, as OpenSSH does
	}
	conn, err := dialDisplay(x.Display)
	if err != nil {
		return
	}
	defer conn.Close()
	if _, err := conn.Write(setup); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { io.Copy(conn, ch); done <- struct{}{} }()
	go func() { io.Copy(ch, conn); ch.CloseWrite(); done <- struct{}{} }()
	<-done
}

// readSetup reads the X connection setup request: a 12-byte header with
// the lengths of the authorization name and data, then both, padded to 4.
func readSetup(r io.Reader) ([]byte, error) {
	head := make([]byte, 12)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, err
	}
	order, err := byteOrder(head[0])
	if err != nil {
		return nil, err
	}
	n := pad4(int(order.Uint16(head[6:8]))) + pad4(int(order.Uint16(head[8:10])))
	rest := make([]byte, n)
	if _, err := io.ReadFull(r, rest); err != nil {
		return nil, err
	}
	return append(head, rest...), nil
}

// rewrite checks the fake cookie and puts the real one (or none) instead.
func (x *X11) rewrite(setup []byte) ([]byte, error) {
	order, err := byteOrder(setup[0])
	if err != nil {
		return nil, err
	}
	nameLen, dataLen := int(order.Uint16(setup[6:8])), int(order.Uint16(setup[8:10]))
	name := string(setup[12 : 12+nameLen])
	dataAt := 12 + pad4(nameLen)
	data := setup[dataAt : dataAt+dataLen]
	if name != x11Proto || !bytes.Equal(data, x.fake) {
		return nil, errors.New("bad X11 cookie")
	}
	out := append([]byte(nil), setup[:12]...)
	if x.real == nil {
		order.PutUint16(out[6:8], 0)
		order.PutUint16(out[8:10], 0)
		return out, nil
	}
	order.PutUint16(out[6:8], uint16(len(x11Proto)))
	order.PutUint16(out[8:10], uint16(len(x.real)))
	out = append(out, padded([]byte(x11Proto))...)
	return append(out, padded(x.real)...), nil
}

func byteOrder(b byte) (binary.ByteOrder, error) {
	switch b {
	case 'B':
		return binary.BigEndian, nil
	case 'l':
		return binary.LittleEndian, nil
	}
	return nil, fmt.Errorf("bad X11 byte order %q", b)
}

func pad4(n int) int { return (n + 3) &^ 3 }

func padded(b []byte) []byte {
	return append(append([]byte(nil), b...), make([]byte, pad4(len(b))-len(b))...)
}

// displayParts splits a DISPLAY into host (or socket path) and number.
func displayParts(display string) (host string, number int, ok bool) {
	i := strings.LastIndex(display, ":")
	if i < 0 {
		return "", 0, false
	}
	host = display[:i]
	num, _, _ := strings.Cut(display[i+1:], ".")
	n, err := strconv.Atoi(num)
	if err != nil {
		return "", 0, false
	}
	return host, n, true
}

func displayScreen(display string) int {
	i := strings.LastIndex(display, ":")
	if i < 0 {
		return 0
	}
	if _, screen, ok := strings.Cut(display[i+1:], "."); ok {
		n, _ := strconv.Atoi(screen)
		return n
	}
	return 0
}

// dialDisplay connects to the local X server.
func dialDisplay(display string) (net.Conn, error) {
	host, n, ok := displayParts(display)
	if !ok {
		return nil, fmt.Errorf("bad DISPLAY %q", display)
	}
	switch {
	case strings.HasPrefix(host, "/"):
		// macOS XQuartz: the socket path is the whole host part.
		return net.DialTimeout("unix", display, 5*time.Second)
	case host == "" || host == "unix":
		return net.DialTimeout("unix", filepath.Join("/tmp/.X11-unix", "X"+strconv.Itoa(n)), 5*time.Second)
	default:
		return net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(6000+n)), 5*time.Second)
	}
}

// xauthCookie reads the display's MIT-MAGIC-COOKIE-1 with xauth; nil when
// there is none (the X server then accepts local clients without one).
func xauthCookie(display string) []byte {
	if display == "" {
		return nil
	}
	path, err := exec.LookPath("xauth")
	if err != nil {
		return nil
	}
	out, err := exec.Command(path, "list", display).Output()
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && f[1] == x11Proto {
			if c, err := hex.DecodeString(f[2]); err == nil {
				return c
			}
		}
	}
	return nil
}
