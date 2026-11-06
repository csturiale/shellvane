package sshengine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
	"github.com/jcmturner/gokrb5/v8/crypto"
	"github.com/jcmturner/gokrb5/v8/gssapi"
	"github.com/jcmturner/gokrb5/v8/iana/flags"
	"github.com/jcmturner/gokrb5/v8/iana/keyusage"
	"github.com/jcmturner/gokrb5/v8/messages"
	"github.com/jcmturner/gokrb5/v8/spnego"
	"github.com/jcmturner/gokrb5/v8/types"

	"github.com/csturiale/shellvane/core/errs"
)

// AuthGSSAPI is Kerberos single sign-on (RFC 4462), as in Active Directory.
const AuthGSSAPI = "gssapi-with-mic"

// ErrKerberos: no Kerberos ticket could be obtained or used.
const ErrKerberos errs.Code = "ssh.kerberos"

// Kerberos configures gssapi-with-mic: the realm (empty: the ticket's, the
// configuration's default, or the host's domain in upper case) and the
// principal (empty: the user).
type Kerberos struct {
	Realm     string
	Principal string
}

// gssClient is a Kerberos GSS-API initiator for golang.org/x/crypto/ssh. It
// uses the ticket of the system's credentials cache (kinit, a domain
// login) and, when there is none, asks for the password and gets one.
type gssClient struct {
	ctx    context.Context
	t      Target
	opts   Kerberos
	prompt Prompter
	key    types.EncryptionKey // the ticket's session key
	micKey types.EncryptionKey // the key MICs are signed with
	// acceptorSubkey: the server named a subkey in its AP-REP.
	acceptorSubkey bool
	done           bool
}

func (g *gssClient) InitSecContext(target string, token []byte, _ bool) ([]byte, bool, error) {
	if token != nil {
		// The server's AP-REP (mutual authentication): it proves the server
		// holds the host's key, and may name the subkey MICs are signed with.
		return nil, false, g.acceptReply(token)
	}
	host := strings.TrimPrefix(target, "host@")
	cl, err := g.client(host)
	if err != nil {
		return nil, false, err
	}
	tkt, key, err := cl.GetServiceTicket("host/" + host)
	if err != nil {
		return nil, false, errs.Wrap(err, ErrKerberos, errs.Params{"detail": krbDetail(err)})
	}
	// Mutual authentication and integrity (OpenSSH requires both); the
	// ticket is not delegated to the server.
	tok, err := spnego.NewKRB5TokenAPREQ(cl, tkt, key, []int{gssapi.ContextFlagMutual, gssapi.ContextFlagInteg}, []int{flags.APOptionMutualRequired})
	if err != nil {
		return nil, false, errs.Wrap(err, ErrKerberos, errs.Params{"detail": krbDetail(err)})
	}
	out, err := tok.Marshal()
	if err != nil {
		return nil, false, errs.Wrap(err, ErrKerberos, errs.Params{"detail": krbDetail(err)})
	}
	g.key, g.micKey = key, key
	return out, true, nil
}

// acceptReply checks the server's AP-REP and takes its subkey, if any.
func (g *gssClient) acceptReply(token []byte) error {
	var tok spnego.KRB5Token
	if err := tok.Unmarshal(token); err != nil || !tok.IsAPRep() {
		return errs.New(ErrKerberos, errs.Params{"detail": "the server's answer is not an AP-REP"})
	}
	plain, err := crypto.DecryptEncPart(tok.APRep.EncPart, g.key, keyusage.AP_REP_ENCPART)
	if err != nil {
		return errs.Wrap(err, ErrKerberos, errs.Params{"detail": "the server could not prove its identity"})
	}
	var part messages.EncAPRepPart
	if err := part.Unmarshal(plain); err != nil {
		return errs.Wrap(err, ErrKerberos, errs.Params{"detail": krbDetail(err)})
	}
	if part.Subkey.KeyValue != nil {
		g.micKey, g.acceptorSubkey = part.Subkey, true
	}
	g.done = true
	return nil
}

func (g *gssClient) GetMIC(micField []byte) ([]byte, error) {
	mic := gssapi.MICToken{Payload: micField}
	if g.acceptorSubkey {
		mic.Flags = gssapi.MICTokenFlagAcceptorSubkey
	}
	if err := mic.SetChecksum(g.micKey, keyusage.GSSAPI_INITIATOR_SIGN); err != nil {
		return nil, errs.Wrap(err, ErrKerberos, errs.Params{"detail": err.Error()})
	}
	return mic.Marshal()
}

func (g *gssClient) DeleteSecContext() error { return nil }

// client returns a Kerberos client with a ticket-granting ticket: from the
// credentials cache, or by logging in with the password.
func (g *gssClient) client(host string) (*client.Client, error) {
	cfg := krbConfig()
	if cc, err := credentials.LoadCCache(ccachePath()); err == nil {
		if cl, err := client.NewFromCCache(cc, cfg, client.DisablePAFXFAST(true)); err == nil {
			return cl, nil
		}
	}
	realm := g.opts.Realm
	if realm == "" {
		realm = cfg.LibDefaults.DefaultRealm
	}
	principal := g.opts.Principal
	if principal == "" {
		principal = g.t.User
	}
	if user, r, ok := strings.Cut(principal, "@"); ok {
		principal, realm = user, r
	}
	if realm == "" {
		if _, domain, ok := strings.Cut(host, "."); ok {
			realm = domain
		}
	}
	realm = strings.ToUpper(realm)
	if realm == "" || principal == "" {
		return nil, errs.New(ErrKerberos, errs.Params{"detail": "no realm"})
	}
	if cfg.LibDefaults.DefaultRealm == "" {
		cfg.LibDefaults.DefaultRealm = realm
	}
	answers, err := g.prompt.Challenge(g.ctx, g.t, Challenge{Name: "Kerberos", Questions: []string{fmt.Sprintf("Password for %s@%s:", principal, realm)}, Echo: []bool{false}, Round: 1})
	if err != nil || len(answers) != 1 {
		return nil, errs.New(ErrKerberos, errs.Params{"detail": "cancelled"})
	}
	cl := client.NewWithPassword(principal, realm, answers[0], cfg, client.DisablePAFXFAST(true))
	if err := cl.Login(); err != nil {
		return nil, errs.Wrap(err, ErrKerberos, errs.Params{"detail": krbDetail(err)})
	}
	return cl, nil
}

// krbConfig reads krb5.conf ($KRB5_CONFIG, /etc/krb5.conf); without one,
// KDCs are found in DNS (_kerberos SRV records, as Active Directory has).
func krbConfig() *config.Config {
	for _, p := range []string{os.Getenv("KRB5_CONFIG"), "/etc/krb5.conf", "/Library/Preferences/edu.mit.Kerberos"} {
		if p == "" {
			continue
		}
		if cfg, err := config.Load(p); err == nil {
			return cfg
		}
	}
	cfg := config.New()
	cfg.LibDefaults.DNSLookupKDC = true
	cfg.LibDefaults.DNSLookupRealm = true
	return cfg
}

// ccachePath is the FILE credentials cache: $KRB5CCNAME or the default
// /tmp/krb5cc_<uid>. Other cache types (macOS API:, KCM:) cannot be read:
// the password is asked instead.
func ccachePath() string {
	if v := os.Getenv("KRB5CCNAME"); v != "" {
		if strings.HasPrefix(v, "FILE:") {
			return strings.TrimPrefix(v, "FILE:")
		}
		if !strings.Contains(v, ":") {
			return v
		}
		return ""
	}
	return fmt.Sprintf("/tmp/krb5cc_%d", os.Getuid())
}

// krbDetail is the useful part of a gokrb5 error.
func krbDetail(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, " < "); i >= 0 {
		s = s[i+3:]
	}
	if len(s) > 300 {
		s = s[:300]
	}
	return s
}
