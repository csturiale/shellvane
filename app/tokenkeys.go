package app

import (
	"context"
	"log"
	"os"
	"runtime"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/keys"
	"github.com/csturiale/shellvane/core/pkcs11"
	"github.com/csturiale/shellvane/core/sshengine"
)

// TokenSupport tells the frontend whether hardware keys work in this build
// and which PKCS#11 modules are installed in their usual places.
type TokenSupport struct {
	Supported bool     `json:"supported"`
	Modules   []string `json:"modules"`
}

// TokenSupport lists the PKCS#11 modules found on this computer.
func (s *KeyService) TokenSupport() TokenSupport {
	out := TokenSupport{Supported: pkcs11.Supported, Modules: []string{}}
	for _, m := range pkcs11.CommonModules[runtime.GOOS] {
		if fi, err := os.Stat(m); err == nil && !fi.IsDir() {
			out.Modules = append(out.Modules, m)
		}
	}
	return out
}

// Tokens lists the tokens a module sees (plug the key in first).
func (s *KeyService) Tokens(module string) ([]pkcs11.Token, error) {
	return pkcs11.Tokens(module)
}

// TokenKeys lists the keys of a token.
func (s *KeyService) TokenKeys(module, serial string) ([]pkcs11.Key, error) {
	return pkcs11.Keys(module, serial)
}

// AddTokenKey records a key of a token, to assign it to hosts like any
// other key. Its private part stays on the token.
func (s *KeyService) AddTokenKey(ctx context.Context, name, module string, token pkcs11.Token, key pkcs11.Key) (keys.Key, error) {
	ref := keys.TokenRef{Module: module, Serial: token.Serial, Label: token.Label, KeyID: key.ID}
	return s.repo.AddToken(ctx, name, ref, key.PublicKey)
}

// tokenIdentity loads a hardware key for a connection; isToken is false
// for keys kept in the vault. The PIN is asked (or read from the vault)
// when the token first signs.
func (p *prompter) tokenIdentity(ctx context.Context, keyID string) (signer ssh.Signer, isToken bool) {
	if p.svc.keys == nil {
		return nil, false
	}
	ref, pub, cert, ok, err := p.svc.keys.TokenOf(ctx, keyID)
	if err != nil || !ok {
		return nil, false
	}
	fingerprint := ""
	if k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pub)); err == nil {
		fingerprint = ssh.FingerprintSHA256(k)
	}
	info := sshengine.KeyInfo{Path: ref.Label, Fingerprint: fingerprint}
	ts, err := pkcs11.NewSigner(ref.Module, ref.Serial, ref.KeyID, pub, func(attempt int) (string, error) {
		return p.tokenPIN(ctx, ref, info, attempt)
	})
	if err != nil {
		log.Printf("hardware key %s: %v", keyID, err)
		return nil, true
	}
	p.tokens = append(p.tokens, ts)
	s, err := ssh.NewSignerFromSigner(ts)
	if err != nil {
		log.Printf("hardware key %s: %v", keyID, err)
		return nil, true
	}
	return keys.WithCertificate(s, cert), true
}

// tokenPIN reads the token's PIN from the vault once, then asks.
func (p *prompter) tokenPIN(ctx context.Context, ref keys.TokenRef, info sshengine.KeyInfo, attempt int) (string, error) {
	scope := "pin:" + ref.Serial
	if secret, ok := p.saved(ctx, scope, sshengine.Target{}); ok {
		return string(secret), nil
	}
	a, err := p.svc.ask(ctx, Prompt{SessionID: p.sessionID, Kind: "pin", Key: &info, Attempt: attempt, CanSave: p.canSave(ctx)})
	if err == nil && a.Save {
		p.pending = append(p.pending, pendingSave{scope: scope, label: ref.Label, secret: []byte(a.Value)})
	}
	return a.Value, err
}

// closeTokens logs out of the tokens used to log in: the connection no
// longer needs them.
func (p *prompter) closeTokens() {
	for _, t := range p.tokens {
		t.Close()
	}
	p.tokens = nil
}
