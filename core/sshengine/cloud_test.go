package sshengine

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// fakeCLI puts a script named name first on PATH.
func fakeCLI(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCloudSettingsAndTools(t *testing.T) {
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	defer func() { lookPath = defaultLookPath }()
	if _, err := CloudDialer(Cloud{Kind: CloudAwsSsm, Target: "i-123"}, Target{Host: "h", Port: 22}); errs.CodeOf(err) != ErrCloudTool {
		t.Fatalf("missing aws: %v", err)
	}
	lookPath = func(n string) (string, error) { return "/bin/" + n, nil }
	for _, c := range []Cloud{{Kind: CloudAwsSsm, Target: "-oX"}, {Kind: CloudGcpIap, Target: "vm", Account: "p;rm"}, {Kind: CloudAzureBastion, Target: "vm", Bastion: "b", Group: "g"}, {Kind: "other"}} {
		if _, err := CloudDialer(c, Target{Host: "h", Port: 22}); errs.CodeOf(err) != ErrCloudSetting {
			t.Errorf("%+v: %v", c, err)
		}
	}
}

func TestCloudStdioTransport(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	// A fake aws that echoes, after checking its arguments.
	fakeCLI(t, "aws", `[ "$*" = "ssm start-session --target i-0abc --document-name AWS-StartSSHSession --parameters portNumber=22 --region eu-south-1 --profile prod" ] || { echo "bad args: $*" >&2; exit 1; }
exec cat`)
	fakeCLI(t, "session-manager-plugin", "")
	dial, err := CloudDialer(Cloud{Kind: CloudAwsSsm, Target: "i-0abc", Region: "eu-south-1", Account: "prod"}, Target{Host: "web", Port: 22})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dial(context.Background(), "tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	conn.Write([]byte("SSH-2.0-test\r\n"))
	buf := make([]byte, 14)
	if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "SSH-2.0-test\r\n" {
		t.Fatalf("echo = %q, %v", buf, err)
	}
	conn.Close()

	// A CLI that fails says why.
	fakeCLI(t, "gcloud", `echo "ERROR: (gcloud) You do not currently have an active account selected." >&2; exit 1`)
	dial, _ = CloudDialer(Cloud{Kind: CloudGcpIap, Target: "vm-1", Region: "europe-west8-a"}, Target{Host: "vm-1", Port: 22})
	conn, _ = dial(context.Background(), "tcp", "")
	_, err = conn.Read(make([]byte, 10))
	var e *errs.Error
	if !errors.As(err, &e) || e.Code != ErrCloudFailed || !strings.Contains(e.Params["detail"].(string), "active account") {
		t.Fatalf("failure = %v", err)
	}
}

func TestCloudAzureBastionTunnel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}
	// A fake az that serves the tunnel port given with --port (nc -l).
	if _, err := os.Stat("/bin/nc"); err != nil {
		if _, err := os.Stat("/usr/bin/nc"); err != nil {
			t.Skip("nc not installed")
		}
	}
	fakeCLI(t, "az", `while [ "$1" != "--port" ]; do shift; done; port=$2
exec python3 -c "
import socket,sys
s=socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1); s.bind(('127.0.0.1', int(sys.argv[1]))); s.listen(1)
c,_=s.accept(); c.sendall(b'SSH-2.0-bastion\r\n'); c.recv(10)
" $port`)
	bastionWait = 10 * time.Second
	dial, err := CloudDialer(Cloud{Kind: CloudAzureBastion, Target: "/subscriptions/1/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm", Bastion: "bastion", Group: "rg"}, Target{Host: "vm", Port: 22})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := dial(context.Background(), "tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	buf := make([]byte, 17)
	if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "SSH-2.0-bastion\r\n" {
		t.Fatalf("tunnel = %q, %v", buf, err)
	}
	var _ net.Conn = conn
}
