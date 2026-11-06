package tunnel

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
)

// SOCKS5 (RFC 1928), CONNECT only, without authentication: the listener is
// meant to be bound to localhost, like ssh -D.

const socksVersion = 5

var errSocks = errors.New("socks: unsupported request")

type socksConn struct {
	net.Conn
}

// socksTarget performs the handshake and returns the requested address.
func socksTarget(conn net.Conn) (string, error) {
	s, ok := conn.(*socksConn)
	if !ok {
		return "", errSocks
	}
	return s.handshake()
}

func (s *socksConn) handshake() (string, error) {
	// Greeting: VER, NMETHODS, METHODS...
	head := make([]byte, 2)
	if _, err := io.ReadFull(s, head); err != nil {
		return "", err
	}
	if head[0] != socksVersion {
		return "", errSocks
	}
	methods := make([]byte, head[1])
	if _, err := io.ReadFull(s, methods); err != nil {
		return "", err
	}
	noAuth := false
	for _, m := range methods {
		noAuth = noAuth || m == 0
	}
	if !noAuth {
		s.Write([]byte{socksVersion, 0xff})
		return "", errSocks
	}
	if _, err := s.Write([]byte{socksVersion, 0}); err != nil {
		return "", err
	}

	// Request: VER, CMD, RSV, ATYP, DST.ADDR, DST.PORT
	req := make([]byte, 4)
	if _, err := io.ReadFull(s, req); err != nil {
		return "", err
	}
	if req[0] != socksVersion || req[1] != 1 { // CONNECT
		s.reply(7) // command not supported
		return "", errSocks
	}
	var host string
	switch req[3] {
	case 1: // IPv4
		ip := make([]byte, 4)
		if _, err := io.ReadFull(s, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	case 3: // domain
		n := make([]byte, 1)
		if _, err := io.ReadFull(s, n); err != nil {
			return "", err
		}
		name := make([]byte, n[0])
		if _, err := io.ReadFull(s, name); err != nil {
			return "", err
		}
		host = string(name)
	case 4: // IPv6
		ip := make([]byte, 16)
		if _, err := io.ReadFull(s, ip); err != nil {
			return "", err
		}
		host = net.IP(ip).String()
	default:
		s.reply(8) // address type not supported
		return "", errSocks
	}
	port := make([]byte, 2)
	if _, err := io.ReadFull(s, port); err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(int(binary.BigEndian.Uint16(port)))), nil
}

func (s *socksConn) replySuccess() error { return s.reply(0) }

func (s *socksConn) replyFailure() { s.reply(5) } // connection refused

func (s *socksConn) reply(code byte) error {
	// Bound address is not meaningful through SSH: report 0.0.0.0:0.
	_, err := s.Write([]byte{socksVersion, code, 0, 1, 0, 0, 0, 0, 0, 0})
	return err
}

// socksListener wraps accepted connections for the handshake.
type socksListener struct{ net.Listener }

func (l socksListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &socksConn{Conn: c}, nil
}

func (l socksListener) String() string { return fmt.Sprintf("socks on %s", l.Addr()) }
