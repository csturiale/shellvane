//go:build !windows

package sshengine

import (
	"io"
	"net"
	"os"

	"golang.org/x/crypto/ssh/agent"
)

func openAgent() (agent.ExtendedAgent, io.Closer, AgentInfo, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil, AgentInfo{}, ErrNoAgent
	}
	c, err := net.Dial("unix", sock)
	if err != nil {
		return nil, nil, AgentInfo{}, err
	}
	return agent.NewClient(c), c, AgentInfo{Kind: "ssh-auth-sock", Path: sock}, nil
}
