//go:build windows

package sshengine

import (
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
	"github.com/davidmz/go-pageant"
	"golang.org/x/crypto/ssh/agent"
)

// openSSHPipe is where the Windows OpenSSH agent service listens.
const openSSHPipe = `\\.\pipe\openssh-ssh-agent`

func openAgent() (agent.ExtendedAgent, io.Closer, AgentInfo, error) {
	timeout := 2 * time.Second

	// An explicit SSH_AUTH_SOCK wins: a named pipe (OpenSSH, 1Password,
	// gpg-agent bridges) or a Unix socket (supported since Windows 10).
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		var c net.Conn
		var err error
		if strings.HasPrefix(sock, `\\.\pipe\`) {
			c, err = winio.DialPipe(sock, &timeout)
		} else {
			c, err = net.DialTimeout("unix", sock, timeout)
		}
		if err == nil {
			return agent.NewClient(c), c, AgentInfo{Kind: "ssh-auth-sock", Path: sock}, nil
		}
	}

	if c, err := winio.DialPipe(openSSHPipe, &timeout); err == nil {
		return agent.NewClient(c), c, AgentInfo{Kind: "openssh-pipe", Path: openSSHPipe}, nil
	}

	// Pageant (PuTTY, KeePass KeeAgent, Gpg4win) answers WM_COPYDATA messages.
	if pageant.Available() {
		a, ok := pageant.New().(agent.ExtendedAgent)
		if !ok {
			return nil, nil, AgentInfo{}, ErrNoAgent
		}
		return a, nopCloser{}, AgentInfo{Kind: "pageant"}, nil
	}
	return nil, nil, AgentInfo{}, ErrNoAgent
}
