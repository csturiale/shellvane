// Package sshengine opens SSH connections and interactive shells.
//
// Authentication is tried in the order OpenSSH uses by default: the SSH agent,
// unencrypted default identity files, then a password asked to the user.
// Host keys are verified by core/knownhosts.
package sshengine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/knownhosts"
)

// Error codes returned by this package.
const (
	ErrInvalidTarget errs.Code = "ssh.invalidTarget"
	ErrDialFailed    errs.Code = "ssh.dialFailed"
	ErrAuthFailed    errs.Code = "ssh.authFailed"
	ErrShellFailed   errs.Code = "ssh.shellFailed"
	ErrCancelled     errs.Code = "ssh.cancelled"
	ErrPassphrase    errs.Code = "ssh.passphraseFailed"
)

// Target is where to connect.
type Target struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	User string `json:"user"`
}

func (t Target) addr() string { return net.JoinHostPort(t.Host, strconv.Itoa(t.Port)) }

// ParseTarget parses "user@host:port"; user and port are optional.
func ParseTarget(spec string) (Target, error) {
	spec = strings.TrimSpace(spec)
	t := Target{Port: 22}
	if at := strings.LastIndex(spec, "@"); at >= 0 {
		t.User, spec = spec[:at], spec[at+1:]
	}
	host := spec
	if h, p, err := net.SplitHostPort(spec); err == nil {
		port, err := strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return Target{}, errs.New(ErrInvalidTarget, errs.Params{"target": spec})
		}
		host, t.Port = h, port
	}
	t.Host = strings.Trim(host, "[]")
	if t.Host == "" || strings.ContainsAny(t.Host, " /") {
		return Target{}, errs.New(ErrInvalidTarget, errs.Params{"target": spec})
	}
	return t, nil
}

// DefaultUser is the local user name, used when a host has none (like ssh).
func DefaultUser() string {
	if u, err := user.Current(); err == nil {
		// On Windows the name is DOMAIN\user.
		name := u.Username
		if i := strings.LastIndex(name, `\`); i >= 0 {
			name = name[i+1:]
		}
		return name
	}
	return os.Getenv("USER")
}

// Prompter asks the user for what the connection needs.
type Prompter interface {
	Password(ctx context.Context, t Target, attempt int) (string, error)
	Passphrase(ctx context.Context, key KeyInfo, attempt int) (string, error)
	ConfirmHostKey(ctx context.Context, key knownhosts.UnknownKey) (knownhosts.Decision, error)
	// Challenge answers keyboard-interactive questions (2FA, OTP, password
	// prompts of PAM); echo[i] tells whether answer i may be shown.
	Challenge(ctx context.Context, t Target, c Challenge) ([]string, error)
}

// Challenge is one keyboard-interactive round.
type Challenge struct {
	Name        string   `json:"name"`
	Instruction string   `json:"instruction"`
	Questions   []string `json:"questions"`
	Echo        []bool   `json:"echo"`
	Round       int      `json:"round"`
}

// KeyInfo identifies an encrypted private key.
type KeyInfo struct {
	Path        string `json:"path"`
	Fingerprint string `json:"fingerprint"`
}

// Options configure Dial.
type Options struct {
	KnownHosts     *knownhosts.Store
	Prompter       Prompter
	ConnectTimeout time.Duration
	// IdentityFiles defaults to ~/.ssh/id_ed25519, id_ecdsa and id_rsa.
	IdentityFiles []string
	// Identity is the key chosen for the host; it is tried first.
	Identity ssh.Signer
	// Dialer opens the transport; direct TCP when nil. Jump hosts, proxies
	// and ProxyCommand plug in here.
	Dialer DialFunc
	// Trace, when set, records the handshake for diagnostics.
	Trace *Trace
	// NoDefaults authenticates with Identity only: no agent, default key
	// files, keyboard-interactive or password (to verify a key).
	NoDefaults bool
	// IdentitiesOnly offers Identity alone among the keys (OpenSSH's
	// IdentitiesOnly): the agent's keys and the default key files are not
	// tried, so servers with a low MaxAuthTries do not refuse early.
	IdentitiesOnly bool
	// AuthOrder lists the methods to use, in order of preference (OpenSSH's
	// PreferredAuthentications): AuthPublicKey, AuthKeyboardInteractive,
	// AuthPassword. Unknown names are ignored; empty means all three in
	// that order.
	AuthOrder []string
	// Kerberos, when set, tries gssapi-with-mic first (single sign-on with
	// the system's Kerberos ticket, or the password to get one).
	Kerberos *Kerberos
}

// Authentication method names (RFC 4252).
const (
	AuthPublicKey           = "publickey"
	AuthKeyboardInteractive = "keyboard-interactive"
	AuthPassword            = "password"
)

// DefaultAuthOrder is OpenSSH's order.
var DefaultAuthOrder = []string{AuthPublicKey, AuthKeyboardInteractive, AuthPassword}

// NormalizeAuthOrder keeps the known methods of order, once each; it
// returns DefaultAuthOrder when none is left.
func NormalizeAuthOrder(order []string) []string {
	var out []string
	for _, m := range order {
		m = strings.TrimSpace(m)
		if slices.Contains(DefaultAuthOrder, m) && !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		return slices.Clone(DefaultAuthOrder)
	}
	return out
}

// Dial connects and authenticates.
func Dial(ctx context.Context, t Target, opts Options) (*ssh.Client, error) {
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = 15 * time.Second
	}
	tr := opts.Trace
	hostKeyCallback, err := opts.KnownHosts.Callback(ctx, opts.Prompter.ConfirmHostKey)
	if err != nil {
		return nil, err
	}
	checkKey := hostKeyCallback
	hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		tr.Add(StepHostKey, "type", key.Type(), "fingerprint", ssh.FingerprintSHA256(key))
		return checkKey(hostname, remote, key)
	}

	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			c.Close()
		}
	}()
	auth, agentConn := authMethods(ctx, t, opts)
	if agentConn != nil {
		closers = append(closers, agentConn)
	}

	cfg := &ssh.ClientConfig{
		User:              t.User,
		Auth:              auth,
		HostKeyCallback:   hostKeyCallback,
		HostKeyAlgorithms: opts.KnownHosts.Algorithms(t.Host, t.Port),
		Timeout:           opts.ConnectTimeout,
		ClientVersion:     "SSH-2.0-Shellvane",
		AuthCallback:      tr.authObserver(),
		BannerCallback: func(msg string) error {
			tr.Add(StepBanner, "text", strings.TrimSpace(msg))
			return nil
		},
	}

	dial := opts.Dialer
	if dial == nil {
		dial = directDialer(opts.ConnectTimeout)
	}
	via := "direct"
	if opts.Dialer != nil {
		via = "custom"
	}
	tr.Add(StepDial, "address", t.addr(), "user", t.User, "via", via)
	conn, err := dial(ctx, "tcp", t.addr())
	if err != nil {
		tr.Add(StepError, "message", err.Error())
		return nil, errs.Wrap(err, ErrDialFailed, errs.Params{"host": t.Host, "port": t.Port})
	}
	tr.Add(StepConnected, "remote", conn.RemoteAddr().String())

	// The handshake has no context: close the socket if ctx ends first.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	c, chans, reqs, err := ssh.NewClientConn(conn, t.addr(), cfg)
	if err != nil {
		conn.Close()
		cerr := classify(ctx, err, t)
		tr.Add(StepError, "message", err.Error(), "code", string(errs.CodeOf(cerr)))
		return nil, cerr
	}
	tr.Add(StepAuthenticated)
	return ssh.NewClient(c, chans, reqs), nil
}

func classify(ctx context.Context, err error, t Target) error {
	var coded *errs.Error
	switch {
	case errors.As(err, &coded):
		return coded // from the host key callback or the prompter
	case ctx.Err() != nil:
		return errs.Wrap(ctx.Err(), ErrCancelled, nil)
	case strings.Contains(err.Error(), "unable to authenticate"):
		return errs.Wrap(err, ErrAuthFailed, errs.Params{"user": t.User, "host": t.Host})
	default:
		return errs.Wrap(err, ErrDialFailed, errs.Params{"host": t.Host, "port": t.Port})
	}
}

// authMethods builds the methods in the order chosen for the host. All keys
// go in a single publickey method, because the client never tries a second
// method with the same name after the first one fails: the host's key comes
// first, then the agent's, then the default key files, each offered once.
func authMethods(ctx context.Context, t Target, opts Options) ([]ssh.AuthMethod, io.Closer) {
	if opts.NoDefaults {
		if opts.Identity == nil {
			return nil, nil
		}
		return []ssh.AuthMethod{ssh.PublicKeys(opts.Identity)}, nil
	}

	var agentConn io.Closer
	var agentSigners func() ([]ssh.Signer, error)
	var files []ssh.Signer
	if !opts.IdentitiesOnly || opts.Identity == nil {
		if ag, closer, _, err := OpenAgent(); err == nil {
			agentConn, agentSigners = closer, ag.Signers
		}
		files = fileSigners(ctx, opts)
	}
	keys := func() ([]ssh.Signer, error) {
		var all []ssh.Signer
		if opts.Identity != nil {
			all = append(all, opts.Identity)
		}
		if agentSigners != nil {
			if s, err := agentSigners(); err == nil {
				all = append(all, s...)
			}
		}
		return uniqueSigners(append(all, files...)), nil
	}

	var methods []ssh.AuthMethod
	if opts.Kerberos != nil && opts.Prompter != nil {
		methods = append(methods, ssh.GSSAPIWithMICAuthMethod(&gssClient{ctx: ctx, t: t, opts: *opts.Kerberos, prompt: opts.Prompter}, t.Host))
	}
	for _, m := range NormalizeAuthOrder(opts.AuthOrder) {
		switch m {
		case AuthPublicKey:
			methods = append(methods, ssh.PublicKeysCallback(opts.Trace.offered(keys)))
		case AuthKeyboardInteractive:
			round := 0
			methods = append(methods, ssh.RetryableAuthMethod(ssh.KeyboardInteractive(
				func(name, instruction string, questions []string, echos []bool) ([]string, error) {
					if len(questions) == 0 {
						return nil, nil // informational round (banner): nothing to answer
					}
					round++
					return opts.Prompter.Challenge(ctx, t, Challenge{Name: name, Instruction: instruction, Questions: questions, Echo: echos, Round: round})
				}), 6))
		case AuthPassword:
			attempt := 0
			methods = append(methods, ssh.RetryableAuthMethod(ssh.PasswordCallback(func() (string, error) {
				attempt++
				return opts.Prompter.Password(ctx, t, attempt)
			}), 3))
		}
	}

	if agentConn == nil {
		return methods, nil
	}
	return methods, agentConn
}

// fileSigners loads the key files; encrypted ones ask for their passphrase
// only when the server accepts the key.
func fileSigners(ctx context.Context, opts Options) []ssh.Signer {
	files := opts.IdentityFiles
	if files == nil {
		files = defaultIdentityFiles()
	}
	var signers []ssh.Signer
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		s, err := ssh.ParsePrivateKey(data)
		var missing *ssh.PassphraseMissingError
		switch {
		case err == nil:
			signers = append(signers, s)
		case errors.As(err, &missing) && missing.PublicKey != nil:
			signers = append(signers, &lazySigner{
				ctx: ctx, pem: data, pub: missing.PublicKey, prompter: opts.Prompter,
				info: KeyInfo{Path: f, Fingerprint: ssh.FingerprintSHA256(missing.PublicKey)},
			})
		}
	}
	return signers
}

// uniqueSigners drops keys already in the list (the agent often holds the
// same key as a file), keeping the first.
func uniqueSigners(signers []ssh.Signer) []ssh.Signer {
	seen := map[string]bool{}
	out := signers[:0:0]
	for _, s := range signers {
		k := string(s.PublicKey().Marshal())
		if !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	return out
}

func defaultIdentityFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var files []string
	for _, name := range []string{"id_ed25519", "id_ecdsa", "id_rsa"} {
		files = append(files, filepath.Join(home, ".ssh", name))
	}
	return files
}

// Shell is an interactive shell with a pseudo-terminal.
type Shell struct {
	client  *ssh.Client
	release func() // returns a pooled connection; nil closes the client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader

	closeOnce sync.Once
}

// StartShell requests a PTY of the given size and starts the login shell.
// release is called on Close for a pooled connection; with nil, Close closes
// the client.
//
// forwardAgent asks the server to forward the agent; the caller must have
// called ServeAgentForwarding on client first.
func StartShell(client *ssh.Client, release func(), term string, cols, rows int, forwardAgent bool) (*Shell, error) {
	return StartCommand(client, release, term, cols, rows, forwardAgent, "")
}

// StartCommand is StartShell running command (through the user's shell, in
// the PTY) instead of the login shell; an empty command starts the shell.
func StartCommand(client *ssh.Client, release func(), term string, cols, rows int, forwardAgent bool, command string) (*Shell, error) {
	return StartSession(client, release, SessionOptions{Term: term, Cols: cols, Rows: rows, ForwardAgent: forwardAgent, Command: command})
}

// SessionOptions configure StartSession.
type SessionOptions struct {
	Term         string
	Cols, Rows   int
	ForwardAgent bool
	Command      string // "" starts the login shell
	// X11, when set, forwards X11 programs to the local display.
	X11 *X11
}

// StartSession starts a shell or command in a PTY.
func StartSession(client *ssh.Client, release func(), o SessionOptions) (*Shell, error) {
	term, cols, rows, forwardAgent, command := o.Term, o.Cols, o.Rows, o.ForwardAgent, o.Command
	session, err := client.NewSession()
	if err != nil {
		return nil, errs.Wrap(err, ErrShellFailed, nil)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, errs.Wrap(err, ErrShellFailed, nil)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, errs.Wrap(err, ErrShellFailed, nil)
	}
	if forwardAgent {
		// Best effort: the server may refuse, the shell still works.
		RequestAgentForwarding(session)
	}
	if o.X11 != nil {
		o.X11.Request(client, session) // best effort, like the agent
	}
	// With a PTY the server merges stderr into stdout.
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 115200, ssh.TTY_OP_OSPEED: 115200}
	if err := session.RequestPty(term, rows, cols, modes); err != nil {
		session.Close()
		return nil, errs.Wrap(err, ErrShellFailed, nil)
	}
	start := session.Shell
	if command != "" {
		start = func() error { return session.Start(command) }
	}
	if err := start(); err != nil {
		session.Close()
		return nil, errs.Wrap(err, ErrShellFailed, nil)
	}
	return &Shell{client: client, release: release, session: session, stdin: stdin, stdout: stdout}, nil
}

// Read reads terminal output.
func (s *Shell) Read(p []byte) (int, error) { return s.stdout.Read(p) }

// Write sends keystrokes.
func (s *Shell) Write(p []byte) (int, error) { return s.stdin.Write(p) }

// Resize changes the PTY size.
func (s *Shell) Resize(cols, rows int) error {
	return s.session.WindowChange(rows, cols)
}

// Wait blocks until the shell exits and returns its exit status, or -1 when
// the connection dropped.
func (s *Shell) Wait() int {
	err := s.session.Wait()
	var exit *ssh.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitStatus()
	default:
		return -1
	}
}

// Close ends the shell and the connection.
func (s *Shell) Close() error {
	var err error
	s.closeOnce.Do(func() {
		s.session.Close()
		if s.release != nil {
			s.release()
		} else {
			err = s.client.Close()
		}
	})
	return err
}

// String describes the target for logs.
func (t Target) String() string { return fmt.Sprintf("%s@%s", t.User, t.addr()) }
