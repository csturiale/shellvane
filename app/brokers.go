package app

import (
	"context"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/vault"
)

// ErrBroker: the credential plugin of a host failed; the detail says why.
const ErrBroker errs.Code = "session.broker"

// Broker returns the login a plugin configuration gives for a target.
type Broker func(ctx context.Context, configID string, t plugins.Target) (plugins.Credential, error)

// EnableBrokers lets hosts get their login from credential plugins. It is
// not a method so that Wails does not expose it.
func EnableBrokers(s *SessionService, b Broker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.broker = b
}

// applyBrokers asks the host's credential plugin, if any, and applies its
// user, port and proxy command.
func (s *SessionService) applyBrokers(ctx context.Context, ht *resolvedHost, owner string) error {
	if ht.credentialPlugin == "" {
		return nil
	}
	s.mu.Lock()
	broker := s.broker
	s.mu.Unlock()
	if broker == nil {
		return errs.New(plugins.ErrUnsupported, nil)
	}
	t := plugins.Target{HostID: ht.id, Name: ht.name, Hostname: ht.target.Host, Port: ht.target.Port, User: ht.target.User}
	cred, err := s.askBroker(ctx, broker, ht.credentialPlugin, t, *ht, owner)
	if err != nil {
		p := errs.ToPayload(err)
		detail, _ := p.Params["detail"].(string)
		if detail == "" {
			detail = string(p.Code)
		}
		return errs.Wrap(err, ErrBroker, errs.Params{"host": ht.name, "detail": detail})
	}
	if cred.User != "" {
		ht.target.User = cred.User
	}
	if cred.Port > 0 {
		ht.target.Port = cred.Port
	}
	if cred.ProxyCommand != "" {
		ht.proxyCommand = cred.ProxyCommand
		ht.proxy = ""
	}
	ht.brokered = &cred
	return nil
}

// askBroker asks a credential plugin for t. The plugin may need the user,
// e.g. for a one-time code (asked like a keyboard-interactive question of
// owner's connection) or a sign-in in a browser window (SAML, OpenID
// Connect); a plugin token kept in the locked vault opens it first.
func (s *SessionService) askBroker(ctx context.Context, broker Broker, ref string, t plugins.Target, ht resolvedHost, owner string) (plugins.Credential, error) {
	ctx = plugins.WithAsker(ctx, func(ctx context.Context, title, question string, secret bool) (string, error) {
		a, err := s.ask(ctx, Prompt{SessionID: owner, Kind: "challenge", Target: ht.target, Attempt: 1,
			Challenge: &sshengine.Challenge{Name: title, Questions: []string{question}, Echo: []bool{!secret}, Round: 1}, Autofilled: []bool{false}})
		if err != nil {
			return "", err
		}
		if len(a.Answers) == 0 {
			return "", errs.New(sshengine.ErrCancelled, nil)
		}
		return a.Answers[0], nil
	})
	ctx = plugins.WithWebLogin(ctx, WebLoginWindow)
	cred, err := broker(ctx, ref, t)
	if errs.CodeOf(err) == vault.ErrLocked && s.vault != nil {
		pr := &prompter{svc: s, sessionID: owner, host: ht, tried: map[string]bool{}}
		if pr.unlock(ctx, ht.target) {
			cred, err = broker(ctx, ref, t)
		}
	}
	return cred, err
}

// brokeredSigner is the broker's key, with its certificate when given.
func brokeredSigner(cred *plugins.Credential) (ssh.Signer, error) {
	if cred == nil || cred.PrivateKey == "" {
		return nil, nil
	}
	signer, err := ssh.ParsePrivateKey([]byte(cred.PrivateKey))
	if err != nil {
		return nil, errs.Wrap(err, ErrBroker, errs.Params{"detail": "private key"})
	}
	if cred.Certificate == "" {
		return signer, nil
	}
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(cred.Certificate))
	if err != nil {
		return nil, errs.Wrap(err, ErrBroker, errs.Params{"detail": "certificate"})
	}
	cert, ok := pub.(*ssh.Certificate)
	if !ok {
		return nil, errs.New(ErrBroker, errs.Params{"detail": "certificate"})
	}
	return ssh.NewCertSigner(cert, signer)
}

// PluginBroker gives logins from the plugin configurations of s. It is a
// function, not a method, so that secrets never become a Wails binding.
func PluginBroker(s *PluginService) Broker {
	return func(ctx context.Context, configID string, t plugins.Target) (plugins.Credential, error) {
		cfg, err := s.configs.Get(ctx, configID)
		if err != nil {
			return plugins.Credential{}, err
		}
		p, err := s.installedPlugin(ctx, cfg.PluginID)
		if err != nil {
			return plugins.Credential{}, err
		}
		settings, err := s.settings(ctx, p, cfg)
		if err != nil {
			return plugins.Credential{}, err
		}
		return p.Credentials(ctx, settings, t)
	}
}

// RouteBrokers sends a reference that starts with one of the prefixes (for
// example "team:") to that prefix's broker, and any other to fallback.
func RouteBrokers(fallback Broker, byPrefix map[string]Broker) Broker {
	return func(ctx context.Context, ref string, t plugins.Target) (plugins.Credential, error) {
		for prefix, b := range byPrefix {
			if strings.HasPrefix(ref, prefix) {
				return b(ctx, ref, t)
			}
		}
		return fallback(ctx, ref, t)
	}
}
