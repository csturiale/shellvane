package sshengine

import (
	"errors"
	"io"

	"golang.org/x/crypto/ssh/agent"
)

// ErrNoAgent means no SSH agent is reachable.
var ErrNoAgent = errors.New("no ssh agent available")

// AgentInfo tells which agent was found, for diagnostics.
type AgentInfo struct {
	Kind string // "ssh-auth-sock", "openssh-pipe" or "pageant"
	Path string
}

// OpenAgent connects to the user's SSH agent. On Unix it uses SSH_AUTH_SOCK;
// on Windows it also tries the OpenSSH agent pipe and Pageant. The closer
// ends the connection; it is never nil when err is nil.
func OpenAgent() (agent.ExtendedAgent, io.Closer, AgentInfo, error) {
	return openAgent()
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
