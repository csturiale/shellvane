package sshengine

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/knownhosts"
	"github.com/csturiale/shellvane/core/tunnel"
)

func TestParseTarget(t *testing.T) {
	cases := []struct {
		in   string
		want Target
		ok   bool
	}{
		{"example.org", Target{Host: "example.org", Port: 22}, true},
		{"root@example.org", Target{Host: "example.org", Port: 22, User: "root"}, true},
		{"deploy@10.0.0.1:2222", Target{Host: "10.0.0.1", Port: 2222, User: "deploy"}, true},
		{"me@[::1]:2200", Target{Host: "::1", Port: 2200, User: "me"}, true},
		{"a@b@host", Target{Host: "host", Port: 22, User: "a@b"}, true},
		{"host:99999", Target{}, false},
		{"user@", Target{}, false},
		{"bad host", Target{}, false},
	}
	for _, c := range cases {
		got, err := ParseTarget(c.in)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("ParseTarget(%q) = %+v, %v; want %+v ok=%v", c.in, got, err, c.want, c.ok)
		}
	}
}

// testServer is a minimal SSH server: password auth and an echo "shell".
type testServer struct {
	addr     string
	hostKey  ssh.PublicKey
	password string

	mu      sync.Mutex
	resizes [][2]int
}

func startServer(t *testing.T, password string, authorized ...ssh.PublicKey) *testServer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	srv := &testServer{hostKey: signer.PublicKey(), password: password}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(_ ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
			if string(p) == password {
				return nil, nil
			}
			return nil, io.EOF
		},
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			for _, k := range authorized {
				if bytes.Equal(k.Marshal(), key.Marshal()) {
					return nil, nil
				}
			}
			return nil, io.EOF
		},
	}
	cfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	srv.addr = ln.Addr().String()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.serve(conn, cfg)
		}
	}()
	return srv
}

func (s *testServer) serve(conn net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(conn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() == "direct-tcpip" {
			go forwardChannel(nc)
			continue
		}
		ch, requests, err := nc.Accept()
		if err != nil {
			continue
		}
		go func() {
			for req := range requests {
				switch req.Type {
				case "pty-req", "shell":
					req.Reply(true, nil)
					if req.Type == "shell" {
						go func() {
							io.Copy(ch, ch) // echo
							ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
							ch.Close()
						}()
					}
				case "window-change":
					cols := binary.BigEndian.Uint32(req.Payload[0:4])
					rows := binary.BigEndian.Uint32(req.Payload[4:8])
					s.mu.Lock()
					s.resizes = append(s.resizes, [2]int{int(cols), int(rows)})
					s.mu.Unlock()
				default:
					req.Reply(false, nil)
				}
			}
		}()
	}
}

func (s *testServer) target(t *testing.T) Target {
	host, port, _ := net.SplitHostPort(s.addr)
	tg, err := ParseTarget("tester@" + net.JoinHostPort(host, port))
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

type fakePrompter struct {
	passwords []string
	decision  knownhosts.Decision
	asked     int
	hostKeys  []knownhosts.UnknownKey

	passphrases     []string
	passphraseAsked int
	keyInfo         KeyInfo

	answers    [][]string
	challenges []Challenge
}

func (p *fakePrompter) Password(_ context.Context, _ Target, attempt int) (string, error) {
	p.asked++
	if attempt > len(p.passwords) {
		return "", errs.New(ErrCancelled, nil)
	}
	return p.passwords[attempt-1], nil
}

func (p *fakePrompter) Passphrase(_ context.Context, k KeyInfo, attempt int) (string, error) {
	p.passphraseAsked++
	p.keyInfo = k
	if attempt > len(p.passphrases) {
		return "", errs.New(ErrCancelled, nil)
	}
	return p.passphrases[attempt-1], nil
}

func (p *fakePrompter) Challenge(_ context.Context, _ Target, c Challenge) ([]string, error) {
	p.challenges = append(p.challenges, c)
	if len(p.answers) == 0 {
		return nil, errs.New(ErrCancelled, nil)
	}
	a := p.answers[0]
	p.answers = p.answers[1:]
	return a, nil
}

func (p *fakePrompter) ConfirmHostKey(_ context.Context, k knownhosts.UnknownKey) (knownhosts.Decision, error) {
	p.hostKeys = append(p.hostKeys, k)
	return p.decision, nil
}

func options(t *testing.T, p Prompter) Options {
	return Options{
		KnownHosts:    knownhosts.New(filepath.Join(t.TempDir(), "known_hosts")),
		Prompter:      p,
		IdentityFiles: []string{}, // never pick up the developer's keys
	}
}

func TestDialPasswordRetryAndShell(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	srv := startServer(t, "s3cret")
	p := &fakePrompter{passwords: []string{"wrong", "s3cret"}, decision: knownhosts.AcceptSave}

	client, err := Dial(context.Background(), srv.target(t), options(t, p))
	if err != nil {
		t.Fatal(err)
	}
	if p.asked != 2 {
		t.Errorf("password asked %d times, want 2", p.asked)
	}
	if len(p.hostKeys) != 1 || p.hostKeys[0].Fingerprint != ssh.FingerprintSHA256(srv.hostKey) {
		t.Errorf("unexpected host key prompts: %+v", p.hostKeys)
	}

	shell, err := StartShell(client, nil, "xterm-256color", 80, 24, false)
	if err != nil {
		t.Fatal(err)
	}
	defer shell.Close()

	if _, err := shell.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	var got bytes.Buffer
	deadline := time.Now().Add(5 * time.Second)
	for !bytes.Contains(got.Bytes(), []byte("hello")) && time.Now().Before(deadline) {
		n, err := shell.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			break
		}
	}
	if !bytes.Contains(got.Bytes(), []byte("hello")) {
		t.Fatalf("echo not received, got %q", got.String())
	}

	if err := shell.Resize(120, 40); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if len(srv.resizes) != 1 || srv.resizes[0] != [2]int{120, 40} {
		t.Errorf("resizes = %v, want [[120 40]]", srv.resizes)
	}
}

func TestDialAuthFailure(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	srv := startServer(t, "s3cret")
	p := &fakePrompter{passwords: []string{"a", "b", "c"}, decision: knownhosts.AcceptOnce}

	opts := options(t, p)
	opts.Trace = NewTrace()
	_, err := Dial(context.Background(), srv.target(t), opts)
	if errs.CodeOf(err) != ErrAuthFailed {
		t.Fatalf("err = %v, want %s", err, ErrAuthFailed)
	}
	steps := map[string]map[string]string{}
	for _, e := range opts.Trace.Events() {
		steps[e.Step] = e.Detail
	}
	for _, want := range []string{StepDial, StepConnected, StepServerVersion, StepAlgorithms, StepHostKey, StepAuthMethods, StepAuthFailed, StepError} {
		if _, ok := steps[want]; !ok {
			t.Errorf("trace lacks %s: %+v", want, opts.Trace.Events())
		}
	}
	if steps[StepAlgorithms]["kex"] == "" || steps[StepError]["code"] != string(ErrAuthFailed) {
		t.Errorf("trace details: %+v", steps)
	}
	if _, ok := steps[StepAuthenticated]; ok {
		t.Error("failed dial traced as authenticated")
	}
}

func TestDialRejectedHostKey(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	srv := startServer(t, "s3cret")
	p := &fakePrompter{passwords: []string{"s3cret"}, decision: knownhosts.Reject}

	_, err := Dial(context.Background(), srv.target(t), options(t, p))
	if errs.CodeOf(err) != knownhosts.ErrHostKeyRejected {
		t.Fatalf("err = %v, want %s", err, knownhosts.ErrHostKeyRejected)
	}
	if p.asked != 0 {
		t.Error("no password must be sent to an untrusted host")
	}
}

func TestDialUnreachable(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close() // nothing listens there now
	tg, _ := ParseTarget("u@" + addr)

	_, err := Dial(context.Background(), tg, options(t, &fakePrompter{}))
	if errs.CodeOf(err) != ErrDialFailed {
		t.Fatalf("err = %v, want %s", err, ErrDialFailed)
	}
}

// encryptedKey writes a passphrase-protected OpenSSH key and returns its path.
func encryptedKey(t *testing.T, passphrase string) (string, ssh.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "test", []byte(passphrase))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	sshPub, _ := ssh.NewPublicKey(pub)
	return path, sshPub
}

func TestEncryptedKeyAsksPassphraseOnlyWhenAccepted(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	path, pub := encryptedKey(t, "open sesame")
	srv := startServer(t, "s3cret", pub)
	p := &fakePrompter{passphrases: []string{"wrong", "open sesame"}, decision: knownhosts.AcceptOnce}
	opts := options(t, p)
	opts.IdentityFiles = []string{path}

	client, err := Dial(context.Background(), srv.target(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if p.passphraseAsked != 2 || p.asked != 0 {
		t.Fatalf("passphrase asked %d times, password %d; want 2 and 0", p.passphraseAsked, p.asked)
	}
	if p.keyInfo.Path != path || p.keyInfo.Fingerprint != ssh.FingerprintSHA256(pub) {
		t.Errorf("key info = %+v", p.keyInfo)
	}
}

func TestEncryptedKeyNotAcceptedIsNeverUnlocked(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	path, _ := encryptedKey(t, "open sesame")
	srv := startServer(t, "s3cret") // the key is not authorized
	p := &fakePrompter{passwords: []string{"s3cret"}, decision: knownhosts.AcceptOnce}
	opts := options(t, p)
	opts.IdentityFiles = []string{path}

	client, err := Dial(context.Background(), srv.target(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if p.passphraseAsked != 0 || p.asked != 1 {
		t.Fatalf("passphrase asked %d times, password %d; want 0 and 1", p.passphraseAsked, p.asked)
	}
}

// Like OpenSSH: after three wrong passphrases the key is skipped and the
// next method (password) is tried.
func TestEncryptedKeyWrongPassphraseFallsBackToPassword(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	path, pub := encryptedKey(t, "open sesame")
	srv := startServer(t, "s3cret", pub)
	p := &fakePrompter{passphrases: []string{"a", "b", "c"}, passwords: []string{"s3cret"}, decision: knownhosts.AcceptOnce}
	opts := options(t, p)
	opts.IdentityFiles = []string{path}

	client, err := Dial(context.Background(), srv.target(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if p.passphraseAsked != 3 || p.asked != 1 {
		t.Fatalf("passphrase asked %d times, password %d; want 3 and 1", p.passphraseAsked, p.asked)
	}
}

func TestDialWithAgent(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(priv)
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: priv}); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(t.TempDir(), "agent.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Skip("unix sockets not available:", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go agent.ServeAgent(keyring, c)
		}
	}()
	t.Setenv("SSH_AUTH_SOCK", sock)

	srv := startServer(t, "unused", signer.PublicKey())
	p := &fakePrompter{decision: knownhosts.AcceptOnce}
	client, err := Dial(context.Background(), srv.target(t), options(t, p))
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if p.asked != 0 {
		t.Fatal("the agent key should log in without a password")
	}
	if _, _, info, err := OpenAgent(); err != nil || info.Kind != "ssh-auth-sock" {
		t.Fatalf("OpenAgent = %+v, %v", info, err)
	}
}

// forwardChannel serves a direct-tcpip request like an SSH server acting as
// a jump host.
func forwardChannel(nc ssh.NewChannel) {
	var req struct {
		Host       string
		Port       uint32
		OriginHost string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(nc.ExtraData(), &req); err != nil {
		nc.Reject(ssh.ConnectionFailed, "bad request")
		return
	}
	conn, err := net.Dial("tcp", net.JoinHostPort(req.Host, strconv.Itoa(int(req.Port))))
	if err != nil {
		nc.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	ch, reqs, err := nc.Accept()
	if err != nil {
		conn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	go func() { io.Copy(ch, conn); ch.CloseWrite() }()
	io.Copy(conn, ch)
	conn.Close()
	ch.Close()
}

func TestDialThroughJumpHost(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	jump := startServer(t, "jump-pw")
	final := startServer(t, "final-pw")

	jumpClient, err := Dial(context.Background(), jump.target(t), options(t, &fakePrompter{passwords: []string{"jump-pw"}, decision: knownhosts.AcceptOnce}))
	if err != nil {
		t.Fatal(err)
	}
	defer jumpClient.Close()

	opts := options(t, &fakePrompter{passwords: []string{"final-pw"}, decision: knownhosts.AcceptOnce})
	opts.Dialer = jumpClient.DialContext
	client, err := Dial(context.Background(), final.target(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	sh, err := StartShell(client, nil, "xterm", 80, 24, false)
	if err != nil {
		t.Fatal(err)
	}
	sh.Close()
}

func TestDialThroughProxies(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	srv := startServer(t, "pw")

	// SOCKS5, served by the tunnel package's SOCKS implementation.
	socks, err := tunnel.StartWith(context.Background(), tunnel.Spec{Kind: tunnel.Dynamic}, net.Listen,
		func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		})
	if err != nil {
		t.Fatal(err)
	}
	defer socks.Stop()

	// HTTP CONNECT proxy.
	httpLn, _ := net.Listen("tcp", "127.0.0.1:0")
	defer httpLn.Close()
	go func() {
		for {
			c, err := httpLn.Accept()
			if err != nil {
				return
			}
			go func() {
				br := bufio.NewReader(c)
				req, err := http.ReadRequest(br)
				if err != nil || req.Method != http.MethodConnect || req.Header.Get("Proxy-Authorization") == "" {
					io.WriteString(c, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")
					c.Close()
					return
				}
				up, err := net.Dial("tcp", req.Host)
				if err != nil {
					c.Close()
					return
				}
				io.WriteString(c, "HTTP/1.1 200 Connection established\r\n\r\n")
				go io.Copy(up, br)
				io.Copy(c, up)
				c.Close()
			}()
		}
	}()

	for _, proxyURL := range []string{"socks5://" + socks.Addr, "http://user:secret@" + httpLn.Addr().String()} {
		dial, err := ProxyDialer(proxyURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		opts := options(t, &fakePrompter{passwords: []string{"pw"}, decision: knownhosts.AcceptOnce})
		opts.Dialer = dial
		client, err := Dial(context.Background(), srv.target(t), opts)
		if err != nil {
			t.Fatalf("%s: %v", proxyURL, err)
		}
		client.Close()
	}

	dial, _ := ProxyDialer("http://"+httpLn.Addr().String(), nil) // no credentials: 407
	opts := options(t, &fakePrompter{})
	opts.Dialer = dial
	if _, err := Dial(context.Background(), srv.target(t), opts); err == nil {
		t.Fatal("a refused CONNECT must fail")
	}
	if _, err := ProxyDialer("ftp://x", nil); errs.CodeOf(err) != ErrProxy {
		t.Fatalf("unsupported scheme: %v", err)
	}
}

func TestProxyCommand(t *testing.T) {
	if got := expandProxyCommand("ssh -W %h:%p %r@bastion 100%%", Target{Host: "db", Port: 5432, User: "me"}); got != "ssh -W db:5432 me@bastion 100%" {
		t.Fatalf("expanded %q", got)
	}
	nc, err := exec.LookPath("nc")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("nc not available")
	}
	t.Setenv("SSH_AUTH_SOCK", "")
	srv := startServer(t, "pw")
	tg := srv.target(t)
	opts := options(t, &fakePrompter{passwords: []string{"pw"}, decision: knownhosts.AcceptOnce})
	opts.Dialer = ProxyCommandDialer(nc+" %h %p", tg)
	client, err := Dial(context.Background(), tg, opts)
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
}

func TestKeyboardInteractive2FA(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	hostSigner, _ := ssh.NewSignerFromKey(priv)
	cfg := &ssh.ServerConfig{
		KeyboardInteractiveCallback: func(_ ssh.ConnMetadata, ask ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			a, err := ask("", "Two-step login", []string{"Password: ", "Verification code: "}, []bool{false, true})
			if err != nil || len(a) != 2 || a[0] != "pw" || a[1] != "123456" {
				return nil, io.EOF
			}
			return nil, nil
		},
	}
	cfg.AddHostKey(hostSigner)
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				_, chans, reqs, err := ssh.NewServerConn(c, cfg)
				if err == nil {
					go ssh.DiscardRequests(reqs)
					for nc := range chans {
						nc.Reject(ssh.Prohibited, "")
					}
				}
			}()
		}
	}()
	tg, _ := ParseTarget("u@" + ln.Addr().String())
	p := &fakePrompter{decision: knownhosts.AcceptOnce, answers: [][]string{{"pw", "000000"}, {"pw", "123456"}}}
	client, err := Dial(context.Background(), tg, options(t, p))
	if err != nil {
		t.Fatal(err)
	}
	client.Close()
	if len(p.challenges) != 2 || p.challenges[0].Instruction != "Two-step login" || p.challenges[1].Round != 2 || !p.challenges[0].Echo[1] {
		t.Fatalf("challenges = %+v", p.challenges)
	}
}

func TestNoDefaultsNeverAsksForAPassword(t *testing.T) {
	srv := startServer(t, "s3cret")
	p := &fakePrompter{passwords: []string{"s3cret"}, decision: knownhosts.AcceptOnce}
	opts := options(t, p)
	opts.NoDefaults = true
	if _, err := Dial(context.Background(), srv.target(t), opts); errs.CodeOf(err) != ErrAuthFailed {
		t.Fatalf("err = %v, want %s", err, ErrAuthFailed)
	}
	if p.asked != 0 {
		t.Errorf("password asked %d times", p.asked)
	}
}
