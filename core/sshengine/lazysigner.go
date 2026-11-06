package sshengine

import (
	"context"
	"crypto/x509"
	"errors"
	"io"
	"sync"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
)

// maxPassphraseAttempts matches OpenSSH's default number of prompts.
const maxPassphraseAttempts = 3

// lazySigner wraps an encrypted key and asks for its passphrase only when a
// signature is needed. The SSH client first asks the server whether it would
// accept the public key, so the user is prompted only for a key that can log
// in, as with OpenSSH.
type lazySigner struct {
	ctx      context.Context
	pem      []byte
	pub      ssh.PublicKey
	info     KeyInfo
	prompter Prompter

	mu     sync.Mutex
	signer ssh.Signer
}

func (l *lazySigner) PublicKey() ssh.PublicKey { return l.pub }

func (l *lazySigner) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	s, err := l.unlock()
	if err != nil {
		return nil, err
	}
	return s.Sign(rand, data)
}

// Algorithms lets RSA keys use SHA-2 signatures, as an unwrapped signer would.
func (l *lazySigner) Algorithms() []string {
	if l.pub.Type() == ssh.KeyAlgoRSA {
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	}
	return []string{l.pub.Type()}
}

func (l *lazySigner) SignWithAlgorithm(rand io.Reader, data []byte, algorithm string) (*ssh.Signature, error) {
	s, err := l.unlock()
	if err != nil {
		return nil, err
	}
	if as, ok := s.(ssh.AlgorithmSigner); ok {
		return as.SignWithAlgorithm(rand, data, algorithm)
	}
	return s.Sign(rand, data)
}

func (l *lazySigner) unlock() (ssh.Signer, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.signer != nil {
		return l.signer, nil
	}
	for attempt := 1; attempt <= maxPassphraseAttempts; attempt++ {
		pass, err := l.prompter.Passphrase(l.ctx, l.info, attempt)
		if err != nil {
			return nil, err
		}
		s, err := ssh.ParsePrivateKeyWithPassphrase(l.pem, []byte(pass))
		if err == nil {
			l.signer = s
			return s, nil
		}
		// Both OpenSSH and PEM keys report a wrong passphrase this way.
		if !errors.Is(err, x509.IncorrectPasswordError) {
			return nil, errs.Wrap(err, ErrPassphrase, errs.Params{"path": l.info.Path})
		}
	}
	return nil, errs.New(ErrPassphrase, errs.Params{"path": l.info.Path})
}
