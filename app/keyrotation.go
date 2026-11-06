package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/keys"
	"github.com/csturiale/shellvane/core/knownhosts"
	"github.com/csturiale/shellvane/core/sshengine"
)

// Error codes of key rotation and export.
const (
	ErrKeyVerifyFailed errs.Code = "keys.verifyFailed"
	ErrKeyNotUsed      errs.Code = "keys.notUsed"
)

// KeyUsage is a host or group whose identity is the key.
type KeyUsage struct {
	Kind string `json:"kind"` // "host" or "group"
	ID   string `json:"id"`
	Name string `json:"name"`
	// Inherited is true for a host that gets the key from its group.
	Inherited bool `json:"inherited"`
}

// Usage lists the groups that set the key and the hosts that use it,
// directly or through their groups.
func (s *KeyService) Usage(ctx context.Context, keyID string) ([]KeyUsage, error) {
	inv, err := s.sessions.hosts.List(ctx)
	if err != nil {
		return nil, err
	}
	out := []KeyUsage{}
	for _, g := range inv.Groups {
		if deref(g.Settings.IdentityID) == keyID {
			out = append(out, KeyUsage{Kind: "group", ID: g.ID, Name: g.Name})
		}
	}
	for _, h := range inv.Hosts {
		res, err := s.sessions.hosts.Resolve(ctx, h)
		if err != nil {
			return nil, err
		}
		if deref(res.Settings.IdentityID) == keyID {
			out = append(out, KeyUsage{Kind: "host", ID: h.ID, Name: h.Name, Inherited: res.Source["identityId"] != h.ID})
		}
	}
	return out, nil
}

// Export returns the private key as OpenSSH or PPK text, encrypted with
// the passphrase when one is given. The vault must be unlocked.
func (s *KeyService) Export(ctx context.Context, keyID, format, passphrase string) (string, error) {
	priv, comment, err := s.repo.Private(ctx, keyID)
	if err != nil {
		return "", err
	}
	return keys.Export(priv, comment, format, passphrase)
}

// RotationStep is the outcome of a rotation on one host.
type RotationStep struct {
	HostID   string        `json:"hostId"`
	Name     string        `json:"name"`
	Deployed bool          `json:"deployed"`
	Verified bool          `json:"verified"`
	Removed  bool          `json:"removed"` // old key removed from authorized_keys
	Error    *errs.Payload `json:"error,omitempty"`
}

// RotationResult reports a rotation.
type RotationResult struct {
	NewKey keys.Key       `json:"newKey"`
	Hosts  []RotationStep `json:"hosts"`
	// Switched is true when hosts and groups now use the new key: only
	// when it was verified on every host.
	Switched bool `json:"switched"`
}

// Rotate replaces a key: it generates a key of the same type, installs it
// on every host that uses the old one, checks that each host accepts it
// alone, then makes the hosts and groups use it and, if removeOld, removes
// the old key from the hosts' authorized_keys. The old key stays in
// Shellvane until deleted.
func (s *KeyService) Rotate(ctx context.Context, keyID string, removeOld bool) (RotationResult, error) {
	var res RotationResult
	usage, err := s.Usage(ctx, keyID)
	if err != nil {
		return res, err
	}
	var hostIDs []KeyUsage
	for _, u := range usage {
		if u.Kind == "host" {
			hostIDs = append(hostIDs, u)
		}
	}
	if len(hostIDs) == 0 {
		return res, errs.New(ErrKeyNotUsed, nil)
	}
	old, err := s.find(ctx, keyID)
	if err != nil {
		return res, err
	}
	kind, bits := keyKind(old)
	priv, err := keys.Generate(kind, bits)
	if err != nil {
		return res, err
	}
	res.NewKey, err = s.repo.Add(ctx, old.Name+" ("+time.Now().Format("2006-01-02")+")", priv, old.Comment)
	if err != nil {
		return res, err
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		return res, err
	}
	all := true
	for _, u := range hostIDs {
		step := RotationStep{HostID: u.ID, Name: u.Name}
		fail := func(err error) {
			p := errs.ToPayload(err)
			step.Error, all = &p, false
		}
		if err := s.Deploy(ctx, res.NewKey.ID, u.ID); err != nil {
			fail(err)
		} else if step.Deployed = true; true {
			if err := s.sessions.verifyKey(ctx, u.ID, signer); err != nil {
				fail(err)
			} else {
				step.Verified = true
			}
		}
		res.Hosts = append(res.Hosts, step)
	}
	if !all {
		return res, nil
	}
	if err := s.switchIdentity(ctx, keyID, res.NewKey.ID); err != nil {
		return res, err
	}
	res.Switched = true
	if removeOld {
		for i, step := range res.Hosts {
			if err := s.runOnHost(ctx, step.HostID, removeCommand(old.PublicKey)); err != nil {
				p := errs.ToPayload(err)
				res.Hosts[i].Error = &p
			} else {
				res.Hosts[i].Removed = true
			}
		}
	}
	return res, nil
}

func (s *KeyService) find(ctx context.Context, id string) (keys.Key, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return keys.Key{}, err
	}
	for _, k := range list {
		if k.ID == id {
			return k, nil
		}
	}
	return keys.Key{}, errs.New(keys.ErrNotFound, errs.Params{"id": id})
}

// keyKind maps a key type to Generate's arguments.
func keyKind(k keys.Key) (string, int) {
	switch {
	case k.Type == ssh.KeyAlgoED25519:
		return "ed25519", 0
	case k.Type == ssh.KeyAlgoRSA:
		return "rsa", max(k.Bits, 3072)
	case strings.HasPrefix(k.Type, "ecdsa-sha2-nistp"):
		return "ecdsa", k.Bits
	}
	return "ed25519", 0
}

// switchIdentity makes groups and hosts that set the old key use the new one.
func (s *KeyService) switchIdentity(ctx context.Context, oldID, newID string) error {
	repo := s.sessions.hosts
	inv, err := repo.List(ctx)
	if err != nil {
		return err
	}
	for _, g := range inv.Groups {
		if deref(g.Settings.IdentityID) == oldID {
			st := g.Settings
			st.IdentityID = &newID
			if _, err := repo.UpdateGroup(ctx, g.ID, hosts.GroupInput{ParentID: g.ParentID, Name: g.Name, Color: g.Color, Settings: st}); err != nil {
				return err
			}
		}
	}
	for _, h := range inv.Hosts {
		if deref(h.Settings.IdentityID) == oldID {
			st := h.Settings
			st.IdentityID = &newID
			if _, err := repo.UpdateHost(ctx, h.ID, hosts.HostInput{GroupID: h.GroupID, Name: h.Name, Hostname: h.Hostname, Notes: h.Notes, Settings: st, Tags: h.Tags, Favorite: h.Favorite}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *KeyService) runOnHost(ctx context.Context, hostID, command string) error {
	ht, err := s.sessions.hostTarget(ctx, hostID)
	if err != nil {
		return err
	}
	client, release, err := s.sessions.connect(ctx, ht, "keys:"+hostID)
	if err != nil {
		return err
	}
	defer release()
	sess, err := client.NewSession()
	if err != nil {
		return errs.Wrap(err, ErrDeployFailed, errs.Params{"host": ht.name})
	}
	defer sess.Close()
	if out, err := sess.CombinedOutput(command); err != nil {
		return errs.Wrap(err, ErrDeployFailed, errs.Params{"host": ht.name, "output": strings.TrimSpace(string(out))})
	}
	return nil
}

// removeCommand removes a line from authorized_keys, keeping the others.
func removeCommand(line string) string {
	q := shellQuote(line)
	return "f=~/.ssh/authorized_keys; [ -f \"$f\" ] || exit 0; umask 077; " +
		"grep -vxF " + q + " \"$f\" > \"$f.shellvane\"; mv \"$f.shellvane\" \"$f\""
}

// verifyKey logs in to a host with only signer (no agent, no other key,
// no password), through the host's jump chain, to prove it is accepted.
func (s *SessionService) verifyKey(ctx context.Context, hostID string, signer ssh.Signer) error {
	ht, err := s.hostTarget(ctx, hostID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	dial := sshengine.DialFunc(nil)
	if n := len(ht.jumpChain); n > 0 {
		parent, err := s.hostTarget(ctx, ht.jumpChain[n-1])
		if err != nil {
			return err
		}
		parent.jumpChain = ht.jumpChain[:n-1]
		client, release, err := s.connect(ctx, parent, "verify:"+hostID)
		if err != nil {
			return err
		}
		defer release()
		dial = client.DialContext
	} else if dial, err = firstDialer(ht); err != nil {
		return err
	}
	client, err := sshengine.Dial(ctx, ht.target, sshengine.Options{
		KnownHosts: s.known,
		Prompter:   noPrompts{},
		Dialer:     dial,
		Identity:   signer,
		NoDefaults: true,
	})
	if err == nil {
		client.Close()
	}
	if err != nil {
		return errs.Wrap(err, ErrKeyVerifyFailed, errs.Params{"host": ht.name, "detail": fmt.Sprint(errs.CodeOf(err))})
	}
	return nil
}

// noPrompts refuses everything: verification must not ask the user.
type noPrompts struct{}

func (noPrompts) Password(context.Context, sshengine.Target, int) (string, error) {
	return "", errs.New(sshengine.ErrCancelled, nil)
}

func (noPrompts) Passphrase(context.Context, sshengine.KeyInfo, int) (string, error) {
	return "", errs.New(sshengine.ErrCancelled, nil)
}

func (noPrompts) ConfirmHostKey(context.Context, knownhosts.UnknownKey) (knownhosts.Decision, error) {
	return knownhosts.Reject, nil
}

func (noPrompts) Challenge(context.Context, sshengine.Target, sshengine.Challenge) ([]string, error) {
	return nil, errs.New(sshengine.ErrCancelled, nil)
}
