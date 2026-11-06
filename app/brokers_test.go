package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/sshengine"
)

func TestBrokersChangeTheLogin(t *testing.T) {
	svc := NewSessionService(nil, nil, nil, nil, func(string, any) {})
	var asked plugins.Target
	EnableBrokers(svc, func(_ context.Context, id string, tg plugins.Target) (plugins.Credential, error) {
		if id != "cfg-1" {
			return plugins.Credential{}, errors.New("unknown")
		}
		asked = tg
		return plugins.Credential{Password: "brokered", User: "root", Port: 3022, ProxyCommand: "tsh proxy ssh %r@%h:%p"}, nil
	})
	ht := resolvedHost{id: "h1", name: "web", target: sshengine.Target{Host: "web.internal", Port: 22, User: "me"}, proxy: "socks5://x", credentialPlugin: "cfg-1"}
	if err := svc.applyBrokers(context.Background(), &ht, "s1"); err != nil {
		t.Fatal(err)
	}
	if asked.Hostname != "web.internal" || asked.User != "me" {
		t.Errorf("broker asked for %+v", asked)
	}
	if ht.target.User != "root" || ht.target.Port != 3022 || ht.proxyCommand != "tsh proxy ssh %r@%h:%p" || ht.proxy != "" {
		t.Fatalf("host = %+v", ht)
	}
	p := &prompter{svc: svc, host: ht, tried: map[string]bool{}}
	if pw, err := p.Password(context.Background(), ht.target, 1); err != nil || pw != "brokered" {
		t.Fatalf("password = %q, %v", pw, err)
	}

	ht.credentialPlugin = "missing"
	if err := svc.applyBrokers(context.Background(), &ht, "s1"); errs.CodeOf(err) != ErrBroker {
		t.Fatalf("failing broker: %v", err)
	}
}

func TestBrokeredSignerWithCertificate(t *testing.T) {
	_, caKey, _ := ed25519.GenerateKey(rand.Reader)
	ca, _ := ssh.NewSignerFromKey(caKey)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	sshPub, _ := ssh.NewPublicKey(pub)
	cert := &ssh.Certificate{Key: sshPub, CertType: ssh.UserCert, ValidPrincipals: []string{"root"}, ValidBefore: ssh.CertTimeInfinity}
	if err := cert.SignCert(rand.Reader, ca); err != nil {
		t.Fatal(err)
	}
	block, _ := ssh.MarshalPrivateKey(priv, "")
	signer, err := brokeredSigner(&plugins.Credential{PrivateKey: string(pem.EncodeToMemory(block)), Certificate: string(ssh.MarshalAuthorizedKey(cert))})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := signer.PublicKey().(*ssh.Certificate); !ok {
		t.Fatalf("signer key = %T, want a certificate", signer.PublicKey())
	}
	if s, err := brokeredSigner(&plugins.Credential{Password: "x"}); s != nil || err != nil {
		t.Fatal("a password-only credential has no signer")
	}
	if _, err := brokeredSigner(&plugins.Credential{PrivateKey: "junk"}); errs.CodeOf(err) != ErrBroker {
		t.Fatalf("bad key: %v", err)
	}
}
