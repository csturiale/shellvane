package app

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/keys"
	"github.com/csturiale/shellvane/core/sshengine"
)

// ErrAgentFailed reports that the agent could not listen.
const ErrAgentFailed errs.Code = "keys.agentFailed"

// AgentStatus tells whether Shellvane's agent runs and where.
type AgentStatus struct {
	Running bool   `json:"running"`
	Path    string `json:"path"` // value for SSH_AUTH_SOCK
}

// agentServer is Shellvane's SSH agent: it offers the vault's keys to other
// programs (ssh, git, scp...) and asks the user before every signature.
type agentServer struct {
	mu sync.Mutex
	ln net.Listener
}

// StartAgent starts Shellvane's agent; programs use it through SSH_AUTH_SOCK.
func (s *KeyService) StartAgent(ctx context.Context) (AgentStatus, error) {
	s.agent.mu.Lock()
	defer s.agent.mu.Unlock()
	if s.agent.ln != nil {
		return AgentStatus{Running: true, Path: agentPath(s.sessions.dataDir)}, nil
	}
	path := agentPath(s.sessions.dataDir)
	ln, err := listenLocal(path)
	if err != nil {
		return AgentStatus{}, errs.Wrap(err, ErrAgentFailed, errs.Params{"path": path})
	}
	s.agent.ln = ln
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// "Allow for this connection" lasts as long as the program's connection.
			ag := sshengine.NewConfirmingAgent(&vaultAgent{repo: s.repo}, s.confirmSign)
			go func() {
				defer conn.Close()
				agent.ServeAgent(ag, conn)
			}()
		}
	}()
	return AgentStatus{Running: true, Path: path}, nil
}

// StopAgent stops Shellvane's agent.
func (s *KeyService) StopAgent() {
	s.agent.mu.Lock()
	defer s.agent.mu.Unlock()
	if s.agent.ln != nil {
		s.agent.ln.Close()
		s.agent.ln = nil
		if !isPipe(agentPath(s.sessions.dataDir)) {
			os.Remove(agentPath(s.sessions.dataDir))
		}
	}
}

// AgentStatus reports whether the agent runs.
func (s *KeyService) AgentStatus() AgentStatus {
	s.agent.mu.Lock()
	defer s.agent.mu.Unlock()
	return AgentStatus{Running: s.agent.ln != nil, Path: agentPath(s.sessions.dataDir)}
}

// askSign asks the user whether a program may sign with a key.
func (s *KeyService) askSign(r sshengine.SignRequest) sshengine.SignDecision {
	r.Local = true
	a, err := s.sessions.ask(context.Background(), Prompt{Kind: "agentSign", Sign: &r})
	if err != nil {
		return sshengine.SignRefuse
	}
	switch a.Decision {
	case "session":
		return sshengine.SignSession
	case "once":
		return sshengine.SignOnce
	}
	return sshengine.SignRefuse
}

// vaultAgent lists and signs with the vault's keys. While the vault is
// locked it offers no key.
type vaultAgent struct {
	repo *keys.Repo
}

var errAgentReadOnly = errors.New("Shellvane's agent does not accept changes")

func (a *vaultAgent) List() ([]*agent.Key, error) {
	list, err := a.repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	var out []*agent.Key
	for _, k := range list {
		if _, err := a.repo.Signer(context.Background(), k.ID); err != nil {
			continue // locked vault or unreadable key
		}
		pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(k.PublicKey))
		if err != nil {
			continue
		}
		out = append(out, &agent.Key{Format: pub.Type(), Blob: pub.Marshal(), Comment: k.Name})
	}
	return out, nil
}

func (a *vaultAgent) signer(key ssh.PublicKey) (ssh.Signer, error) {
	list, err := a.repo.List(context.Background())
	if err != nil {
		return nil, err
	}
	fp := ssh.FingerprintSHA256(key)
	for _, k := range list {
		if k.Fingerprint == fp {
			s, err := a.repo.Signer(context.Background(), k.ID)
			if err != nil {
				return nil, err
			}
			// A key with a certificate signs as the plain key here.
			if c, ok := s.(interface{ Signer() ssh.Signer }); ok {
				return c.Signer(), nil
			}
			return s, nil
		}
	}
	return nil, errors.New("key not found")
}

func (a *vaultAgent) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return a.SignWithFlags(key, data, 0)
}

func (a *vaultAgent) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	s, err := a.signer(key)
	if err != nil {
		return nil, err
	}
	if as, ok := s.(ssh.AlgorithmSigner); ok && key.Type() == ssh.KeyAlgoRSA {
		switch {
		case flags&agent.SignatureFlagRsaSha512 != 0:
			return as.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA512)
		case flags&agent.SignatureFlagRsaSha256 != 0:
			return as.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA256)
		}
	}
	return s.Sign(rand.Reader, data)
}

func (a *vaultAgent) Signers() ([]ssh.Signer, error) { return nil, errAgentReadOnly }
func (a *vaultAgent) Add(agent.AddedKey) error       { return errAgentReadOnly }
func (a *vaultAgent) Remove(ssh.PublicKey) error     { return errAgentReadOnly }
func (a *vaultAgent) RemoveAll() error               { return errAgentReadOnly }
func (a *vaultAgent) Lock([]byte) error              { return errAgentReadOnly }
func (a *vaultAgent) Unlock([]byte) error            { return errAgentReadOnly }
func (a *vaultAgent) Extension(string, []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}
