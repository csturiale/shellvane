package app

import (
	"context"
	"os"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/keys"
	"github.com/csturiale/shellvane/core/sshengine"
)

// ErrDeployFailed reports that the public key could not be installed.
const ErrDeployFailed errs.Code = "keys.deployFailed"

// KeyService manages SSH keys. Private keys never reach the frontend.
type KeyService struct {
	repo     *keys.Repo
	sessions *SessionService
	agent    agentServer
	// confirmSign asks before the agent signs; tests replace it.
	confirmSign func(sshengine.SignRequest) sshengine.SignDecision
}

// NewKeyService returns a service over repo; Deploy uses sessions' pool.
func NewKeyService(repo *keys.Repo, sessions *SessionService) *KeyService {
	s := &KeyService{repo: repo, sessions: sessions}
	s.confirmSign = s.askSign
	return s
}

// List returns all keys (public data only).
func (s *KeyService) List(ctx context.Context) ([]keys.Key, error) { return s.repo.List(ctx) }

// Generate creates and stores a new key; kind is ed25519, rsa or ecdsa.
func (s *KeyService) Generate(ctx context.Context, name, kind string, bits int, comment string) (keys.Key, error) {
	priv, err := keys.Generate(kind, bits)
	if err != nil {
		return keys.Key{}, err
	}
	return s.repo.Add(ctx, name, priv, strings.TrimSpace(comment))
}

// Import stores a pasted private key (OpenSSH, PEM or PuTTY .ppk).
func (s *KeyService) Import(ctx context.Context, name, data, passphrase string) (keys.Key, error) {
	priv, comment, err := keys.Parse([]byte(data), []byte(passphrase))
	if err != nil {
		return keys.Key{}, err
	}
	return s.repo.Add(ctx, name, priv, comment)
}

// ImportFile stores a private key read from a file.
func (s *KeyService) ImportFile(ctx context.Context, name, path, passphrase string) (keys.Key, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return keys.Key{}, errs.Wrap(err, ErrImportRead, errs.Params{"path": path})
	}
	return s.Import(ctx, name, string(data), passphrase)
}

// SetCertificate attaches an OpenSSH certificate (the "-cert.pub" line) to a
// key; an empty line removes it.
func (s *KeyService) SetCertificate(ctx context.Context, id, certLine string) error {
	return s.repo.SetCertificate(ctx, id, certLine)
}

// Rename changes a key's name.
func (s *KeyService) Rename(ctx context.Context, id, name string) error {
	return s.repo.Rename(ctx, id, name)
}

// Delete removes a key.
func (s *KeyService) Delete(ctx context.Context, id string) error { return s.repo.Delete(ctx, id) }

// Deploy appends the public key to ~/.ssh/authorized_keys on a host, like
// ssh-copy-id, unless it is already there.
func (s *KeyService) Deploy(ctx context.Context, keyID, hostID string) error {
	var line string
	list, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	for _, k := range list {
		if k.ID == keyID {
			line = k.PublicKey
		}
	}
	if line == "" {
		return errs.New(keys.ErrNotFound, errs.Params{"id": keyID})
	}
	ht, err := s.sessions.hostTarget(ctx, hostID)
	if err != nil {
		return err
	}
	client, release, err := s.sessions.connect(ctx, ht, "deploy:"+keyID)
	if err != nil {
		return err
	}
	defer release()
	sess, err := client.NewSession()
	if err != nil {
		return errs.Wrap(err, ErrDeployFailed, errs.Params{"host": ht.name})
	}
	defer sess.Close()
	if out, err := sess.CombinedOutput(deployCommand(line)); err != nil {
		return errs.Wrap(err, ErrDeployFailed, errs.Params{"host": ht.name, "output": strings.TrimSpace(string(out))})
	}
	return nil
}

// deployCommand is a POSIX shell command that adds line once.
func deployCommand(line string) string {
	q := shellQuote(line)
	return "umask 077; mkdir -p ~/.ssh && touch ~/.ssh/authorized_keys && " +
		"(grep -qxF " + q + " ~/.ssh/authorized_keys || printf '%s\\n' " + q + " >> ~/.ssh/authorized_keys)"
}

// shellQuote wraps s in single quotes for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
