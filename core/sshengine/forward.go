package sshengine

import (
	"errors"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// ErrSignRefused is returned to the remote side when the user refuses to sign.
var ErrSignRefused = errors.New("signature refused by the user")

// SignRequest describes a remote request to use a forwarded key.
type SignRequest struct {
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	// Local is true for a program on this computer using Shellvane's agent.
	Local bool `json:"local"`
}

// SignDecision is the user's answer to a SignRequest.
type SignDecision int

const (
	SignRefuse  SignDecision = iota
	SignOnce                 // this signature only
	SignSession              // every signature with this key on this connection
)

// ConfirmingAgent forwards an agent to a server but asks before every
// signature (a compromised server could otherwise log in elsewhere as the
// user). Adding, removing and locking keys from the remote side is refused.
type ConfirmingAgent struct {
	agent.ExtendedAgent
	confirm func(SignRequest) SignDecision

	mu      sync.Mutex
	allowed map[string]bool // fingerprints allowed for the session
}

// NewConfirmingAgent wraps ag; confirm is called for each signature request.
func NewConfirmingAgent(ag agent.ExtendedAgent, confirm func(SignRequest) SignDecision) *ConfirmingAgent {
	return &ConfirmingAgent{ExtendedAgent: ag, confirm: confirm, allowed: map[string]bool{}}
}

func (c *ConfirmingAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	if err := c.check(key); err != nil {
		return nil, err
	}
	return c.ExtendedAgent.Sign(key, data)
}

func (c *ConfirmingAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	if err := c.check(key); err != nil {
		return nil, err
	}
	return c.ExtendedAgent.SignWithFlags(key, data, flags)
}

func (c *ConfirmingAgent) check(key ssh.PublicKey) error {
	fp := ssh.FingerprintSHA256(key)
	c.mu.Lock()
	ok := c.allowed[fp]
	c.mu.Unlock()
	if ok {
		return nil
	}
	req := SignRequest{Fingerprint: fp}
	if keys, err := c.ExtendedAgent.List(); err == nil {
		for _, k := range keys {
			if ssh.FingerprintSHA256(k) == fp {
				req.Comment = k.Comment
			}
		}
	}
	switch c.confirm(req) {
	case SignSession:
		c.mu.Lock()
		c.allowed[fp] = true
		c.mu.Unlock()
		return nil
	case SignOnce:
		return nil
	default:
		return ErrSignRefused
	}
}

func (c *ConfirmingAgent) Add(agent.AddedKey) error   { return ErrSignRefused }
func (c *ConfirmingAgent) Remove(ssh.PublicKey) error { return ErrSignRefused }
func (c *ConfirmingAgent) RemoveAll() error           { return ErrSignRefused }
func (c *ConfirmingAgent) Lock([]byte) error          { return ErrSignRefused }
func (c *ConfirmingAgent) Unlock([]byte) error        { return ErrSignRefused }
func (c *ConfirmingAgent) Extension(string, []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}

// ServeAgentForwarding answers the server's agent channels on client with ag.
// Call it once per connection, before requesting forwarding on a session.
func ServeAgentForwarding(client *ssh.Client, ag agent.Agent) error {
	return agent.ForwardToAgent(client, ag)
}

// RequestAgentForwarding asks the server to forward the agent for a session.
func RequestAgentForwarding(session *ssh.Session) error {
	return agent.RequestAgentForwarding(session)
}
