package sshengine

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Cloud access services that reach a host without a direct connection.
const (
	CloudAwsSsm       = "awsSsm"
	CloudAzureBastion = "azureBastion"
	CloudGcpIap       = "gcpIap"
)

// Error codes of cloud transports.
const (
	// ErrCloudTool: the vendor's command-line tool is not installed.
	ErrCloudTool errs.Code = "ssh.cloudTool"
	// ErrCloudSetting: a required field is missing or not valid.
	ErrCloudSetting errs.Code = "ssh.cloudSetting"
	// ErrCloudFailed: the service refused or the tunnel did not open.
	ErrCloudFailed errs.Code = "ssh.cloudFailed"
)

// Cloud says how to reach a host through a cloud access service.
type Cloud struct {
	Kind    string // CloudAwsSsm, CloudAzureBastion or CloudGcpIap
	Target  string // instance ID, VM resource ID or instance name
	Region  string // AWS region or GCP zone
	Account string // AWS profile, Azure subscription or GCP project
	Bastion string // Azure Bastion name
	Group   string // Azure resource group of the bastion
}

// lookPath finds a program; tests replace it.
var lookPath = defaultLookPath

func defaultLookPath(name string) (string, error) { return exec.LookPath(name) }

// CloudDialer opens the first connection through the service with the
// vendor's CLI, run directly (no shell). AWS SSM and GCP IAP speak over the
// program's stdin and stdout; Azure Bastion opens a local tunnel port.
func CloudDialer(c Cloud, t Target) (DialFunc, error) {
	if c.Target == "" {
		c.Target = t.Host
	}
	port := strconv.Itoa(t.Port)
	for _, v := range []string{c.Target, c.Region, c.Account, c.Bastion, c.Group} {
		if !cloudValue(v) {
			return nil, errs.New(ErrCloudSetting, errs.Params{"value": v})
		}
	}
	switch c.Kind {
	case CloudAwsSsm:
		if err := needTools("aws", "session-manager-plugin"); err != nil {
			return nil, err
		}
		args := []string{"ssm", "start-session", "--target", c.Target, "--document-name", "AWS-StartSSHSession", "--parameters", "portNumber=" + port}
		args = appendOpt(args, "--region", c.Region)
		args = appendOpt(args, "--profile", c.Account)
		return stdioDialer("aws", args), nil
	case CloudGcpIap:
		if err := needTools("gcloud"); err != nil {
			return nil, err
		}
		args := []string{"compute", "start-iap-tunnel", c.Target, port, "--listen-on-stdin"}
		args = appendOpt(args, "--zone", c.Region)
		args = appendOpt(args, "--project", c.Account)
		return stdioDialer("gcloud", args), nil
	case CloudAzureBastion:
		if err := needTools("az"); err != nil {
			return nil, err
		}
		if c.Bastion == "" || c.Group == "" || !strings.HasPrefix(strings.ToLower(c.Target), "/subscriptions/") {
			return nil, errs.New(ErrCloudSetting, errs.Params{"value": c.Target})
		}
		return bastionDialer(c, port), nil
	}
	return nil, errs.New(ErrCloudSetting, errs.Params{"value": c.Kind})
}

// cloudValue: identifiers, names, regions and resource IDs only; nothing a
// program would read as an option.
func cloudValue(v string) bool {
	if v == "" {
		return true
	}
	if v[0] == '-' {
		return false
	}
	for _, r := range v {
		if r <= ' ' || r == 127 || strings.ContainsRune("'\"`$;&|<>(){}*?!\\", r) {
			return false
		}
	}
	return true
}

func needTools(names ...string) error {
	for _, n := range names {
		if _, err := lookPath(n); err != nil {
			return errs.New(ErrCloudTool, errs.Params{"tool": n})
		}
	}
	return nil
}

func appendOpt(args []string, flag, v string) []string {
	if v == "" {
		return args
	}
	return append(args, flag, v)
}

// stdioDialer runs the program and speaks SSH over its stdin and stdout.
func stdioDialer(name string, args []string) DialFunc {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		cmd := exec.Command(name, args...)
		stderr := &lastLines{done: make(chan struct{})}
		errR, errW, err := os.Pipe()
		if err != nil {
			return nil, err
		}
		cmd.Stderr = errW
		go func() { io.Copy(stderr, errR); errR.Close(); close(stderr.done) }()
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return nil, errs.Wrap(err, ErrCloudFailed, errs.Params{"tool": name, "detail": err.Error()})
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, errs.Wrap(err, ErrCloudFailed, errs.Params{"tool": name, "detail": err.Error()})
		}
		err = cmd.Start()
		errW.Close() // the child has its copy
		if err != nil {
			return nil, errs.Wrap(err, ErrCloudFailed, errs.Params{"tool": name, "detail": err.Error()})
		}
		return &commandConn{cmd: cmd, w: stdin, r: &failureReader{r: stdout, tool: name, stderr: stderr}}, nil
	}
}

// bastionWait bounds how long the Azure tunnel may take to open.
var bastionWait = 90 * time.Second

// bastionDialer opens `az network bastion tunnel` on a free local port,
// waits for it, and connects there; closing the connection stops it.
func bastionDialer(c Cloud, port string) DialFunc {
	return func(ctx context.Context, _, _ string) (net.Conn, error) {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		local := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
		l.Close()
		args := []string{"network", "bastion", "tunnel", "--name", c.Bastion, "--resource-group", c.Group,
			"--target-resource-id", c.Target, "--resource-port", port, "--port", local}
		args = appendOpt(args, "--subscription", c.Account)
		cmd := exec.Command("az", args...)
		var stderr lastLines
		cmd.Stderr = &stderr
		if err := cmd.Start(); err != nil {
			return nil, errs.Wrap(err, ErrCloudFailed, errs.Params{"tool": "az", "detail": err.Error()})
		}
		exited := make(chan struct{})
		go func() { cmd.Wait(); close(exited) }()
		stop := func() { cmd.Process.Kill(); <-exited }
		deadline := time.Now().Add(bastionWait)
		for {
			conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", local), time.Second)
			if err == nil {
				return &tunnelConn{Conn: conn, stop: stop}, nil
			}
			select {
			case <-exited:
				return nil, errs.New(ErrCloudFailed, errs.Params{"tool": "az", "detail": stderr.last()})
			case <-ctx.Done():
				stop()
				return nil, ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
			if time.Now().After(deadline) {
				stop()
				return nil, errs.New(ErrCloudFailed, errs.Params{"tool": "az", "detail": "timeout"})
			}
		}
	}
}

// tunnelConn stops the tunnel's process when the connection closes.
type tunnelConn struct {
	net.Conn
	stop func()
	once sync.Once
}

func (c *tunnelConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.stop)
	return err
}

// failureReader turns the end of a CLI that never spoke SSH into an error
// carrying what it printed (an expired login, a missing permission).
type failureReader struct {
	r      io.ReadCloser
	tool   string
	stderr *lastLines
	got    bool
}

func (f *failureReader) Close() error { return f.r.Close() }

func (f *failureReader) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if n > 0 {
		f.got = true
	}
	if err != nil && !f.got {
		// The program ended without speaking SSH: what it printed says why.
		select {
		case <-f.stderr.done:
		case <-time.After(2 * time.Second):
		}
		if line := f.stderr.last(); line != "" {
			return n, errs.New(ErrCloudFailed, errs.Params{"tool": f.tool, "detail": line})
		}
	}
	return n, err
}

// lastLines keeps the end of a program's error output.
type lastLines struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	done chan struct{} // closed when the output ended (stdio transports)
}

func (l *lastLines) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(p)
	if l.buf.Len() > 8192 {
		b := l.buf.Bytes()[l.buf.Len()-4096:]
		l.buf.Reset()
		l.buf.Write(b)
	}
	return len(p), nil
}

func (l *lastLines) last() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	lines := strings.Split(strings.TrimSpace(l.buf.String()), "\n")
	line := strings.TrimSpace(lines[len(lines)-1])
	if len(line) > 300 {
		line = line[:300]
	}
	return line
}

// String describes the transport, for logs and the handshake view.
func (c Cloud) String() string { return fmt.Sprintf("%s %s", c.Kind, c.Target) }
