package app

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/automation"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/serialport"
	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/telnet"
)

// ErrInvalidTelnet is returned for a telnet:// spec without a host.
const ErrInvalidTelnet errs.Code = "telnet.invalid"

// Protocols of panes that are not SSH.
const (
	ProtocolTelnet = "telnet"
	ProtocolSerial = "serial"
)

// SerialPorts lists the serial ports of this computer.
func (s *SessionService) SerialPorts() []string { return serialport.Ports() }

// openStream opens telnet:// and serial:// specs; ok is false for other
// specs (SSH targets).
func (s *SessionService) openStream(spec string, cols, rows int) (info SessionInfo, ok bool, err error) {
	switch {
	case strings.HasPrefix(spec, "telnet://"):
		u, perr := url.Parse(spec)
		if perr != nil || u.Hostname() == "" {
			return SessionInfo{}, true, errs.New(ErrInvalidTelnet, errs.Params{"spec": spec})
		}
		port := 23
		if p := u.Port(); p != "" {
			n, cerr := strconv.Atoi(p)
			if cerr != nil || n < 1 || n > 65535 {
				return SessionInfo{}, true, errs.New(ErrInvalidTelnet, errs.Params{"spec": spec})
			}
			port = n
		}
		host := u.Hostname()
		target := sshengine.Target{Host: host, Port: port, User: u.User.Username()}
		title := "telnet " + host
		if port != 23 {
			title += " " + strconv.Itoa(port)
		}
		info := s.openProcess(title, target, ProtocolTelnet, cols, rows, func(cols, rows int) (terminalProcess, error) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			return telnet.Dial(ctx, telnet.Options{Host: host, Port: port, Cols: cols, Rows: rows})
		})
		return info, true, nil
	case strings.HasPrefix(spec, "serial://"):
		st, perr := serialport.Parse(spec)
		if perr != nil {
			return SessionInfo{}, true, perr
		}
		target := sshengine.Target{Host: st.Port}
		info := s.openProcess(st.Label(), target, ProtocolSerial, cols, rows, func(int, int) (terminalProcess, error) {
			return serialport.Open(st)
		})
		return info, true, nil
	}
	return SessionInfo{}, false, nil
}

// openProcess starts a pane whose terminal is a plain byte stream (Telnet,
// a serial line) rather than an SSH shell.
func (s *SessionService) openProcess(title string, target sshengine.Target, protocol string, cols, rows int, start func(cols, rows int) (terminalProcess, error)) SessionInfo {
	sessCtx, cancel := context.WithCancel(context.Background())
	sess := &session{id: uuid.NewString(), ctx: sessCtx, cancel: cancel, ready: make(chan struct{}), flow: newFlowGate(),
		cols: cols, rows: rows, capture: &capture{name: title}}
	highlights, active := splitRules(s.loadRules(sessCtx, resolvedHost{}))
	sess.auto = &automationState{
		host:      title,
		matcher:   automation.NewMatcher(active),
		responses: map[string][]time.Time{},
		watchers:  map[int]func([]byte){},
	}
	s.mu.Lock()
	s.sessions[sess.id] = sess
	s.mu.Unlock()

	s.emit(EventSessionState, SessionState{ID: sess.id, State: StateConnecting})
	go func() {
		defer close(sess.ready)
		sess.sizeMu.Lock()
		c, r := sess.cols, sess.rows
		sess.sizeMu.Unlock()
		p, err := start(c, r)
		if err != nil {
			sess.err = err
			payload := errs.ToPayload(err)
			s.emit(EventSessionState, SessionState{ID: sess.id, State: StateFailed, Error: &payload})
			s.forget(sess.id)
			return
		}
		sess.shell = p
		s.emit(EventSessionState, SessionState{ID: sess.id, State: StateConnected})
	}()
	return SessionInfo{
		ID: sess.id, Title: title, Target: target, Protocol: protocol,
		Group: []string{}, Via: []string{}, Highlights: highlights,
	}
}
