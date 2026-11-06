// Package telnet is a Telnet client (RFC 854) for terminal panes: it
// negotiates echo, suppress-go-ahead, the terminal type (RFC 1091) and the
// window size (RFC 1073), and refuses every other option.
package telnet

import (
	"bufio"
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes.
const (
	ErrDial errs.Code = "telnet.dialFailed"
)

// Protocol bytes.
const (
	se   = 240
	sb   = 250
	will = 251
	wont = 252
	do   = 253
	dont = 254
	iac  = 255

	optEcho  = 1
	optSGA   = 3
	optTType = 24
	optNAWS  = 31

	ttypeIs   = 0
	ttypeSend = 1
)

// Options configure Dial.
type Options struct {
	Host       string
	Port       int
	Term       string // sent as the terminal type; "xterm-256color" when empty
	Cols, Rows int
	Timeout    time.Duration
}

// Conn is a Telnet session: Read returns what the server prints, Write
// sends keystrokes.
type Conn struct {
	conn net.Conn
	r    *bufio.Reader
	term string

	wmu        sync.Mutex
	cols, rows int
	naws       bool // the server asked for the window size

	// Parser state, kept between reads.
	state  int
	verb   byte
	sub    []byte
	lastCR bool
	done   chan struct{}
	once   sync.Once
}

const (
	stData = iota
	stIAC
	stVerb
	stSub
	stSubIAC
)

// Dial connects to the server.
func Dial(ctx context.Context, o Options) (*Conn, error) {
	if o.Port == 0 {
		o.Port = 23
	}
	if o.Term == "" {
		o.Term = "xterm-256color"
	}
	if o.Timeout == 0 {
		o.Timeout = 15 * time.Second
	}
	addr := net.JoinHostPort(o.Host, strconv.Itoa(o.Port))
	d := net.Dialer{Timeout: o.Timeout}
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, errs.Wrap(err, ErrDial, errs.Params{"host": o.Host, "port": o.Port})
	}
	return newConn(c, o), nil
}

func newConn(c net.Conn, o Options) *Conn {
	t := &Conn{conn: c, r: bufio.NewReader(c), term: o.Term, cols: o.Cols, rows: o.Rows, done: make(chan struct{})}
	if t.term == "" {
		t.term = "xterm-256color"
	}
	// Offer what a terminal needs; servers that do not care just refuse.
	t.send(iac, will, optNAWS, iac, will, optTType, iac, do, optSGA)
	return t
}

// Read returns the next data from the server, without protocol bytes.
func (t *Conn) Read(p []byte) (int, error) {
	for {
		n := 0
		for n < len(p) {
			if n > 0 && t.r.Buffered() == 0 {
				break // return what we have rather than block
			}
			b, err := t.r.ReadByte()
			if err != nil {
				t.finish(err)
				if n > 0 {
					return n, nil
				}
				return 0, err
			}
			if d, ok := t.feed(b); ok {
				p[n] = d
				n++
			}
		}
		if n > 0 {
			return n, nil
		}
	}
}

// feed advances the parser; ok is true when b is data.
func (t *Conn) feed(b byte) (byte, bool) {
	switch t.state {
	case stData:
		if b == iac {
			t.state = stIAC
			return 0, false
		}
		// NVT: CR is followed by NUL (a bare CR) or LF.
		if t.lastCR && b == 0 {
			t.lastCR = false
			return 0, false
		}
		t.lastCR = b == '\r'
		return b, true
	case stIAC:
		t.state = stData
		switch b {
		case iac:
			return iac, true
		case will, wont, do, dont:
			t.verb, t.state = b, stVerb
		case sb:
			t.sub, t.state = t.sub[:0], stSub
		}
		return 0, false
	case stVerb:
		t.state = stData
		t.negotiate(t.verb, b)
		return 0, false
	case stSub:
		if b == iac {
			t.state = stSubIAC
		} else if len(t.sub) < 1024 {
			t.sub = append(t.sub, b)
		}
		return 0, false
	case stSubIAC:
		switch b {
		case se:
			t.state = stData
			t.subnegotiation(t.sub)
		case iac:
			t.state = stSub
			t.sub = append(t.sub, iac)
		default:
			t.state = stData
		}
		return 0, false
	}
	return 0, false
}

func (t *Conn) negotiate(verb, opt byte) {
	switch verb {
	case do:
		switch opt {
		case optNAWS:
			t.wmu.Lock()
			t.naws = true
			t.wmu.Unlock()
			t.sendSize()
		case optTType, optSGA:
			// already offered with WILL
		default:
			t.send(iac, wont, opt)
		}
	case will:
		switch opt {
		case optEcho, optSGA:
			t.send(iac, do, opt)
		default:
			t.send(iac, dont, opt)
		}
	case dont:
		if opt == optNAWS {
			t.wmu.Lock()
			t.naws = false
			t.wmu.Unlock()
		}
	}
}

func (t *Conn) subnegotiation(sub []byte) {
	if len(sub) >= 2 && sub[0] == optTType && sub[1] == ttypeSend {
		msg := []byte{iac, sb, optTType, ttypeIs}
		msg = append(msg, t.term...)
		t.send(append(msg, iac, se)...)
	}
}

// Write sends keystrokes, escaping IAC and turning a bare CR into CR NUL.
func (t *Conn) Write(p []byte) (int, error) {
	out := make([]byte, 0, len(p)+8)
	for _, b := range p {
		switch b {
		case iac:
			out = append(out, iac, iac)
		case '\r':
			out = append(out, '\r', 0)
		default:
			out = append(out, b)
		}
	}
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if _, err := t.conn.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Resize tells the server the new window size, once it asked for it.
func (t *Conn) Resize(cols, rows int) error {
	t.wmu.Lock()
	t.cols, t.rows = cols, rows
	t.wmu.Unlock()
	t.sendSize()
	return nil
}

func (t *Conn) sendSize() {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if !t.naws || t.cols <= 0 || t.rows <= 0 {
		return
	}
	msg := []byte{iac, sb, optNAWS}
	for _, v := range []int{t.cols, t.rows} {
		hi, lo := byte(v>>8), byte(v)
		for _, b := range []byte{hi, lo} {
			msg = append(msg, b)
			if b == iac {
				msg = append(msg, iac)
			}
		}
	}
	t.conn.Write(append(msg, iac, se))
}

func (t *Conn) send(b ...byte) {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	t.conn.Write(b)
}

func (t *Conn) finish(error) {
	t.once.Do(func() { close(t.done) })
}

// Wait blocks until the server closes the connection; Telnet has no exit
// status, so it returns 0.
func (t *Conn) Wait() int {
	<-t.done
	return 0
}

// Close ends the session.
func (t *Conn) Close() error {
	err := t.conn.Close()
	t.finish(io.EOF)
	return err
}
