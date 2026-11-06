// Package knownhosts verifies SSH host keys against known_hosts files.
//
// Shellvane keeps its own known_hosts file and also reads the user's
// ~/.ssh/known_hosts, without ever writing to it. Unknown keys are confirmed
// by the user (trust on first use); a changed key is always refused.
package knownhosts

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"golang.org/x/crypto/ssh"
	xknownhosts "golang.org/x/crypto/ssh/knownhosts"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrHostKeyChanged  errs.Code = "ssh.hostKeyChanged"
	ErrHostKeyRejected errs.Code = "ssh.hostKeyRejected"
)

// Decision is the user's answer to an unknown host key.
type Decision int

const (
	Reject     Decision = iota
	AcceptOnce          // trust for this connection only
	AcceptSave          // trust and remember
)

// UnknownKey describes a host key seen for the first time.
type UnknownKey struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	KeyType     string `json:"keyType"`
	Fingerprint string `json:"fingerprint"` // SHA256:...
}

// Confirm asks the user whether to trust an unknown key.
type Confirm func(ctx context.Context, key UnknownKey) (Decision, error)

// Store checks keys against the app file and, read-only, the user's files.
type Store struct {
	appFile   string
	userFiles []string
	mu        sync.Mutex // serializes appends to appFile
}

// New returns a Store that writes to appFile and also trusts userFiles.
// Missing files are fine.
func New(appFile string, userFiles ...string) *Store {
	return &Store{appFile: appFile, userFiles: userFiles}
}

// DefaultUserFiles returns ~/.ssh/known_hosts when the home directory is known.
func DefaultUserFiles() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{filepath.Join(home, ".ssh", "known_hosts")}
}

// Callback returns an ssh.HostKeyCallback that asks confirm for unknown keys.
func (s *Store) Callback(ctx context.Context, confirm Confirm) (ssh.HostKeyCallback, error) {
	check, err := s.checker()
	if err != nil {
		return nil, err
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := check(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *xknownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}
		host, port := splitHostPort(hostname)
		if len(keyErr.Want) > 0 {
			// Known host, different key: possible man-in-the-middle.
			return errs.New(ErrHostKeyChanged, errs.Params{
				"host":        host,
				"port":        port,
				"fingerprint": ssh.FingerprintSHA256(key),
				"expected":    ssh.FingerprintSHA256(keyErr.Want[0].Key),
			})
		}
		decision, err := confirm(ctx, UnknownKey{
			Host: host, Port: port, KeyType: key.Type(), Fingerprint: ssh.FingerprintSHA256(key),
		})
		if err != nil {
			return err
		}
		switch decision {
		case AcceptSave:
			return s.add(hostname, key)
		case AcceptOnce:
			return nil
		default:
			return errs.New(ErrHostKeyRejected, errs.Params{"host": host, "port": port})
		}
	}, nil
}

// Algorithms returns the host key algorithms already known for host:port, so
// the server is asked for a key type we can verify instead of whatever it
// prefers. Empty means no preference.
func (s *Store) Algorithms(host string, port int) []string {
	check, err := s.checker()
	if err != nil {
		return nil
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	// Checking a throwaway key reports every known key for the host.
	err = check(addr, &net.TCPAddr{IP: net.IPv4zero, Port: port}, probeKey)
	var keyErr *xknownhosts.KeyError
	if !errors.As(err, &keyErr) {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, k := range keyErr.Want {
		for _, alg := range algorithmsFor(k.Key.Type()) {
			if !seen[alg] {
				seen[alg] = true
				out = append(out, alg)
			}
		}
	}
	return out
}

// probeKey is a random key no host can have; checking it lists the known keys.
var probeKey = func() ssh.PublicKey {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		panic(err)
	}
	return k
}()

func (s *Store) checker() (ssh.HostKeyCallback, error) {
	var files []string
	for _, f := range append([]string{s.appFile}, s.userFiles...) {
		if _, err := os.Stat(f); err == nil {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return func(string, net.Addr, ssh.PublicKey) error {
			return &xknownhosts.KeyError{}
		}, nil
	}
	return xknownhosts.New(files...)
}

func (s *Store) add(hostname string, key ssh.PublicKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.appFile), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.appFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	line := xknownhosts.Line([]string{xknownhosts.Normalize(hostname)}, key)
	if _, err := fmt.Fprintln(f, line); err != nil {
		return err
	}
	return f.Sync()
}

func splitHostPort(hostname string) (string, int) {
	host, portStr, err := net.SplitHostPort(hostname)
	if err != nil {
		return hostname, 22
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}

// algorithmsFor maps a key type to the signature algorithms that verify it.
func algorithmsFor(keyType string) []string {
	switch keyType {
	case ssh.KeyAlgoRSA:
		return []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA}
	case ssh.CertAlgoRSAv01:
		return []string{ssh.CertAlgoRSASHA512v01, ssh.CertAlgoRSASHA256v01, ssh.CertAlgoRSAv01}
	default:
		return []string{keyType}
	}
}
