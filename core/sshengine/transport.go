package sshengine

import (
	"bufio"
	"context"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/net/proxy"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrProxy reports a proxy or ProxyCommand failure.
const ErrProxy errs.Code = "ssh.proxyFailed"

// DialFunc opens the transport to an SSH server: a TCP connection, a channel
// through a jump host, a proxy tunnel or a ProxyCommand's stdio.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

func directDialer(timeout time.Duration) DialFunc {
	d := net.Dialer{Timeout: timeout}
	return d.DialContext
}

// ProxyDialer returns a dialer through a SOCKS5 or HTTP CONNECT proxy given
// as a URL: socks5://[user:pass@]host:port or http://[user:pass@]host:port.
// next opens the connection to the proxy itself (direct when nil).
func ProxyDialer(rawURL string, next DialFunc) (DialFunc, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return nil, errs.New(ErrProxy, errs.Params{"proxy": rawURL})
	}
	if next == nil {
		next = directDialer(15 * time.Second)
	}
	switch u.Scheme {
	case "socks5", "socks5h":
		var auth *proxy.Auth
		if u.User != nil {
			pass, _ := u.User.Password()
			auth = &proxy.Auth{User: u.User.Username(), Password: pass}
		}
		d, err := proxy.SOCKS5("tcp", u.Host, auth, contextDialer(next))
		if err != nil {
			return nil, errs.Wrap(err, ErrProxy, errs.Params{"proxy": u.Redacted()})
		}
		cd := d.(proxy.ContextDialer)
		return cd.DialContext, nil
	case "http":
		return func(ctx context.Context, network, addr string) (net.Conn, error) {
			return httpConnect(ctx, u, next, addr)
		}, nil
	default:
		return nil, errs.New(ErrProxy, errs.Params{"proxy": u.Redacted()})
	}
}

type contextDialer DialFunc

func (d contextDialer) Dial(network, addr string) (net.Conn, error) {
	return d(context.Background(), network, addr)
}

func (d contextDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return d(ctx, network, addr)
}

func httpConnect(ctx context.Context, u *url.URL, next DialFunc, addr string) (net.Conn, error) {
	conn, err := next(ctx, "tcp", u.Host)
	if err != nil {
		return nil, errs.Wrap(err, ErrProxy, errs.Params{"proxy": u.Redacted()})
	}
	req := "CONNECT " + addr + " HTTP/1.1\r\nHost: " + addr + "\r\n"
	if u.User != nil {
		pass, _ := u.User.Password()
		token := base64.StdEncoding.EncodeToString([]byte(u.User.Username() + ":" + pass))
		req += "Proxy-Authorization: Basic " + token + "\r\n"
	}
	if _, err := io.WriteString(conn, req+"\r\n"); err != nil {
		conn.Close()
		return nil, errs.Wrap(err, ErrProxy, errs.Params{"proxy": u.Redacted()})
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, &http.Request{Method: http.MethodConnect})
	if err != nil {
		conn.Close()
		return nil, errs.Wrap(err, ErrProxy, errs.Params{"proxy": u.Redacted()})
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, errs.New(ErrProxy, errs.Params{"proxy": u.Redacted(), "status": resp.Status})
	}
	if br.Buffered() > 0 {
		return &bufferedConn{Conn: conn, r: br}, nil
	}
	return conn, nil
}

type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// ProxyCommandDialer runs command (OpenSSH ProxyCommand syntax: %h host,
// %p port, %r user, %% literal) and speaks SSH over its stdin and stdout.
func ProxyCommandDialer(command string, t Target) DialFunc {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		// The command goes to a shell: a host or user name with shell
		// characters would run commands (as OpenSSH, refuse them).
		if (strings.Contains(command, "%h") && !shellSafe(t.Host)) || (strings.Contains(command, "%r") && !shellSafe(t.User)) {
			return nil, errs.New(ErrProxy, errs.Params{"proxy": command})
		}
		expanded := expandProxyCommand(command, t)
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/C", expanded)
		} else {
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", expanded)
		}
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, errs.Wrap(err, ErrProxy, errs.Params{"proxy": expanded})
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, errs.Wrap(err, ErrProxy, errs.Params{"proxy": expanded})
		}
		if err := cmd.Start(); err != nil {
			return nil, errs.Wrap(err, ErrProxy, errs.Params{"proxy": expanded})
		}
		return &commandConn{cmd: cmd, w: stdin, r: stdout}, nil
	}
}

// shellSafe tells whether a value can be put in a shell command as it is.
func shellSafe(s string) bool {
	if s == "" || s[0] == '-' {
		return s == ""
	}
	for _, r := range s {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune(".-_:%[]@+=,/", r) || r > 127 && unicode.IsLetter(r)
		if !ok {
			return false
		}
	}
	return true
}

func expandProxyCommand(command string, t Target) string {
	var b strings.Builder
	for i := 0; i < len(command); i++ {
		if command[i] != '%' || i+1 == len(command) {
			b.WriteByte(command[i])
			continue
		}
		i++
		switch command[i] {
		case 'h':
			b.WriteString(t.Host)
		case 'p':
			b.WriteString(strconv.Itoa(t.Port))
		case 'r':
			b.WriteString(t.User)
		case '%':
			b.WriteByte('%')
		default:
			b.WriteByte('%')
			b.WriteByte(command[i])
		}
	}
	return b.String()
}

// commandConn is a net.Conn over a process's stdio.
type commandConn struct {
	cmd *exec.Cmd
	w   io.WriteCloser
	r   io.ReadCloser

	closeOnce sync.Once
}

func (c *commandConn) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c *commandConn) Write(p []byte) (int, error) { return c.w.Write(p) }

// Close may be called by several goroutines of the SSH transport.
func (c *commandConn) Close() error {
	c.closeOnce.Do(func() {
		c.w.Close()
		c.r.Close()
		if c.cmd.Process != nil {
			c.cmd.Process.Kill()
		}
		c.cmd.Wait()
	})
	return nil
}
func (c *commandConn) LocalAddr() net.Addr              { return pipeAddr{} }
func (c *commandConn) RemoteAddr() net.Addr             { return pipeAddr{} }
func (c *commandConn) SetDeadline(time.Time) error      { return nil }
func (c *commandConn) SetReadDeadline(time.Time) error  { return nil }
func (c *commandConn) SetWriteDeadline(time.Time) error { return nil }

type pipeAddr struct{}

func (pipeAddr) Network() string { return "pipe" }
func (pipeAddr) String() string  { return "proxycommand" }
