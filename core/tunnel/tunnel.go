package tunnel

import (
	"context"
	"net"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrInvalid    errs.Code = "tunnel.invalid"
	ErrBindFailed errs.Code = "tunnel.bindFailed"
	ErrNotFound   errs.Code = "tunnel.notFound"
)

// Kind of forwarding.
type Kind string

const (
	Local   Kind = "local"   // ssh -L: listen here, connect from the server
	Remote  Kind = "remote"  // ssh -R: listen on the server, connect from here
	Dynamic Kind = "dynamic" // ssh -D: SOCKS5 proxy here, connect from the server
)

// Spec describes what to forward.
type Spec struct {
	Kind     Kind   `json:"kind"`
	BindAddr string `json:"bindAddr"`
	BindPort int    `json:"bindPort"`
	DestHost string `json:"destHost"`
	DestPort int    `json:"destPort"`
}

// Validate checks ports and required fields.
func (s *Spec) Validate() error {
	if s.BindAddr == "" {
		s.BindAddr = "127.0.0.1"
	}
	bad := func(field string) error { return errs.New(ErrInvalid, errs.Params{"field": field}) }
	if s.BindPort < 0 || s.BindPort > 65535 {
		return bad("bindPort")
	}
	switch s.Kind {
	case Local, Remote:
		if s.DestHost == "" {
			return bad("destHost")
		}
		// A path is a Unix socket (e.g. /var/run/docker.sock): no port.
		if s.socket() {
			s.DestPort = 0
			break
		}
		if s.DestPort < 1 || s.DestPort > 65535 {
			return bad("destPort")
		}
	case Dynamic:
		s.DestHost, s.DestPort = "", 0
	default:
		return bad("kind")
	}
	return nil
}

func (s Spec) bind() string { return net.JoinHostPort(s.BindAddr, strconv.Itoa(s.BindPort)) }
func (s Spec) dest() string {
	if s.socket() {
		return s.DestHost
	}
	return net.JoinHostPort(s.DestHost, strconv.Itoa(s.DestPort))
}

// socket tells whether the destination is a Unix socket path.
func (s Spec) socket() bool { return strings.HasPrefix(s.DestHost, "/") }

// Running is a started tunnel.
type Running struct {
	// Addr is where the tunnel listens (useful with BindPort 0).
	Addr   string
	cancel context.CancelFunc
	done   chan error
	c      *counters
}

// Stats returns live counters.
func (r *Running) Stats() Stats { return r.c.snapshot() }

// Done delivers the reason the tunnel stopped (nil when stopped on purpose).
func (r *Running) Done() <-chan error { return r.done }

// Stop closes the listener and every forwarded connection.
func (r *Running) Stop() {
	r.cancel()
	<-r.done
}

// Listener opens the listening side.
type Listener func(network, addr string) (net.Listener, error)

// Start forwards through an SSH connection.
func Start(ctx context.Context, spec Spec, client *ssh.Client) (*Running, error) {
	switch spec.Kind {
	case Remote:
		var d net.Dialer
		return StartWith(ctx, spec, client.Listen, d.DialContext)
	default:
		return StartWith(ctx, spec, net.Listen, func(ctx context.Context, network, addr string) (net.Conn, error) {
			return client.DialContext(ctx, network, addr)
		})
	}
}

// StartWith forwards using the given listen and dial functions.
func StartWith(ctx context.Context, spec Spec, listen Listener, dial Dialer) (*Running, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	ln, err := listen("tcp", spec.bind())
	if err != nil {
		return nil, errs.Wrap(err, ErrBindFailed, errs.Params{"addr": spec.bind()})
	}
	pick := fixed(spec.dest())
	if spec.Kind == Dynamic {
		ln = socksListener{ln}
		pick = socksTarget
	}
	ctx, cancel := context.WithCancel(ctx)
	r := &Running{Addr: ln.Addr().String(), cancel: cancel, done: make(chan error, 1), c: &counters{}}
	go func() {
		r.done <- serve(ctx, ln, pick, dial, r.c)
		close(r.done)
	}()
	return r, nil
}
