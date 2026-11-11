package builtin

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

// expandTarget fills {{host}}, {{user}} and {{name}} in a setting.
func expandTarget(s string, t plugins.Target) string {
	return strings.NewReplacer("{{host}}", t.Hostname, "{{user}}", t.User, "{{name}}", t.Name).Replace(strings.TrimSpace(s))
}

// secretOf turns a secret read from a vault into a credential: a PEM
// private key or a password.
func secretOf(value, user string) plugins.Credential {
	if strings.Contains(value, "-----BEGIN ") && strings.Contains(value, "PRIVATE KEY-----") {
		return plugins.Credential{PrivateKey: value, User: user}
	}
	return plugins.Credential{Password: value, User: user}
}

// pamCall is one JSON (or plain text) request to a PAM's REST API.
type pamCall struct {
	ctx     context.Context
	c       *http.Client
	command string // in errors: "cyberark", "conjur"
}

// do sends the request and returns the body of a 2xx answer; detail
// reads the error message out of any other.
func (p pamCall) do(method, u string, body []byte, header http.Header, detail func([]byte) string) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		rd = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(p.ctx, method, u, rd)
	if err != nil {
		return nil, errs.Wrap(err, plugins.ErrSetting, errs.Params{"key": "url"})
	}
	for k, v := range header {
		req.Header[k] = v
	}
	resp, err := p.c.Do(req)
	if err != nil {
		return nil, errs.Wrap(err, plugins.ErrCommand, errs.Params{"command": p.command, "detail": err.Error()})
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		msg := resp.Status
		if d := detail(data); d != "" {
			msg = d
		}
		return nil, errs.New(plugins.ErrCommand, errs.Params{"command": p.command, "detail": msg})
	}
	return data, nil
}

// CyberArk reads an account's password or SSH key from CyberArk
// Privileged Access Manager: through the Central Credential Provider with
// an application ID (and a client certificate where required), or through
// the PVWA REST API with the user's own CyberArk login.
type CyberArk struct {
	plugins.Base
	Client func(fingerprint string) *http.Client
	Now    func() time.Time // tests
}

// Manifest describes the plugin.
func (CyberArk) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "cyberark", Name: "CyberArk", Version: "1", Description: "plugins.cyberark.description", BuiltIn: true,
		Capabilities: []string{plugins.CapCredentials},
		Settings: []plugins.Setting{
			{Key: "url", Label: "plugins.settings.url", Placeholder: "https://cyberark.example.org", Required: true},
			{Key: "safe", Label: "plugins.settings.cyberarkSafe", Placeholder: "Linux-Servers"},
			{Key: "appId", Label: "plugins.settings.cyberarkAppId", Placeholder: "Shellvane"},
			{Key: "object", Label: "plugins.settings.cyberarkObject", Placeholder: "{{host}}-{{user}}"},
			{Key: "clientCert", Label: "plugins.settings.clientCert", Placeholder: "~/.cyberark/client.pem"},
			{Key: "clientKey", Label: "plugins.settings.clientKey", Placeholder: "~/.cyberark/client.key"},
			{Key: "username", Label: "plugins.settings.cyberarkUser"},
			{Key: "password", Label: "plugins.settings.cyberarkPassword", Secret: true},
			{Key: "authMethod", Label: "plugins.settings.cyberarkAuth", Placeholder: "SAML"},
			{Key: "webLoginUrl", Label: "plugins.settings.cyberarkWebLogin", Placeholder: "https://cyberark.example.org/PasswordVault/v10/logon/saml"},
			{Key: "otpMode", Label: "plugins.settings.cyberarkOtpMode", Placeholder: "challenge"},
			{Key: "otpDelimiter", Label: "plugins.settings.cyberarkOtpDelimiter", Placeholder: ","},
			{Key: "reason", Label: "plugins.settings.cyberarkReason", Placeholder: "SSH to {{host}}"},
			{Key: "fingerprint", Label: "plugins.settings.fingerprint"},
		},
	}
}

// cyberarkError reads ErrorMsg (CCP) or ErrorMessage (PVWA).
func cyberarkError(data []byte) string {
	var e struct {
		Code    string `json:"ErrorCode"`
		Msg     string `json:"ErrorMsg"`
		Message string `json:"ErrorMessage"`
	}
	if json.Unmarshal(data, &e) != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimSpace(e.Code + " " + or(e.Msg, e.Message)))
}

// Credentials asks the Credential Provider when appId is set, the PVWA
// with username and password otherwise.
func (p CyberArk) Credentials(ctx context.Context, s map[string]string, t plugins.Target) (plugins.Credential, error) {
	if err := plugins.Require(s, "url"); err != nil {
		return plugins.Credential{}, err
	}
	c, err := p.client(s)
	if err != nil {
		return plugins.Credential{}, err
	}
	call := pamCall{ctx: ctx, c: c, command: "cyberark"}
	if strings.TrimSpace(s["appId"]) != "" {
		return p.ccp(call, s, t)
	}
	if web := strings.ToLower(strings.TrimSpace(s["authMethod"])); web == "saml" || web == "oidc" {
		return p.pvwa(call, s, t) // the sign-in happens in a browser window
	}
	if err := plugins.Require(s, "username", "password"); err != nil {
		return plugins.Credential{}, err
	}
	return p.pvwa(call, s, t)
}

// client is the HTTP client, with the client certificate if one is set.
func (p CyberArk) client(s map[string]string) (*http.Client, error) {
	mk := p.Client
	if mk == nil {
		mk = client
	}
	c := mk(s["fingerprint"])
	if c.Timeout > 0 && c.Timeout < 2*time.Minute {
		// A logon waits while the user approves an authenticator push.
		c = &http.Client{Transport: c.Transport, Timeout: 2 * time.Minute, Jar: c.Jar}
	}
	cert := strings.TrimSpace(s["clientCert"])
	if cert == "" {
		return c, nil
	}
	pair, err := tls.LoadX509KeyPair(expandHome(cert), expandHome(or(s["clientKey"], cert)))
	if err != nil {
		return nil, errs.Wrap(err, plugins.ErrSetting, errs.Params{"key": "clientCert"})
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		return c, nil
	}
	tr = tr.Clone()
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{}
	}
	tr.TLSClientConfig.Certificates = []tls.Certificate{pair}
	return &http.Client{Transport: tr, Timeout: c.Timeout}, nil
}

// ccp asks the Central Credential Provider for the account: by Object
// when set, else by the host's address and user.
func (p CyberArk) ccp(call pamCall, s map[string]string, t plugins.Target) (plugins.Credential, error) {
	q := url.Values{"AppID": {strings.TrimSpace(s["appId"])}}
	if safe := strings.TrimSpace(or(t.Safe, s["safe"])); safe != "" {
		q.Set("Safe", safe)
	}
	if obj := expandTarget(s["object"], t); obj != "" {
		q.Set("Object", obj)
	} else {
		q.Set("Address", t.Hostname)
		if t.User != "" {
			q.Set("UserName", t.User)
		}
	}
	data, err := call.do(http.MethodGet, trimURL(s["url"])+"/AIMWebService/api/Accounts?"+q.Encode(), nil, http.Header{"Accept": {"application/json"}}, cyberarkError)
	if err != nil {
		return plugins.Credential{}, err
	}
	var res struct {
		Content  string `json:"Content"`
		UserName string `json:"UserName"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return plugins.Credential{}, badJSON(err, "cyberark")
	}
	if res.Content == "" {
		return plugins.Credential{}, errs.New(plugins.ErrResponse, errs.Params{"command": "cyberark"})
	}
	return secretOf(res.Content, res.UserName), nil
}

// pvwaSessions keeps PVWA session tokens in memory, so that MFA (an
// authenticator push, a RADIUS code) is asked once and not at every
// connection. A token is dropped when the PVWA refuses it or after
// pvwaIdle without use (PVWA's own default timeout is 20 minutes).
var pvwaSessions = struct {
	sync.Mutex
	m map[string]pvwaSession
}{m: map[string]pvwaSession{}}

type pvwaSession struct {
	token string
	used  time.Time
}

const pvwaIdle = 15 * time.Minute

// pvwaChallenge is PVWA's answer to a RADIUS logon that wants a code.
const pvwaChallenge = "ITATS542I"

func pvwaKey(s map[string]string) string {
	return trimURL(s["url"]) + "\x00" + strings.ToLower(strings.TrimSpace(s["username"])) + "\x00" + strings.ToLower(or(s["authMethod"], "CyberArk"))
}

func pvwaCached(key string, now time.Time) string {
	pvwaSessions.Lock()
	defer pvwaSessions.Unlock()
	sess, ok := pvwaSessions.m[key]
	if !ok || now.Sub(sess.used) > pvwaIdle {
		delete(pvwaSessions.m, key)
		return ""
	}
	sess.used = now
	pvwaSessions.m[key] = sess
	return sess.token
}

func pvwaStore(key, token string, now time.Time) {
	pvwaSessions.Lock()
	defer pvwaSessions.Unlock()
	if token == "" {
		delete(pvwaSessions.m, key)
		return
	}
	pvwaSessions.m[key] = pvwaSession{token: token, used: now}
}

// pvwaLogon signs in: an authenticator push is approved on the phone
// while the request waits; a RADIUS challenge asks the user for the code
// (or appends it to the password, otpMode "append").
func (p CyberArk) pvwaLogon(call pamCall, s map[string]string, base, method string) (string, error) {
	jsonHeader := http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json"}}
	logon := func(password string) ([]byte, error) {
		body, _ := json.Marshal(map[string]any{"username": strings.TrimSpace(s["username"]), "password": password, "concurrentSession": true})
		return call.do(http.MethodPost, base+"/auth/"+url.PathEscape(method)+"/Logon", body, jsonHeader, cyberarkError)
	}
	password := s["password"]
	if strings.EqualFold(strings.TrimSpace(s["otpMode"]), "append") {
		code, err := plugins.Ask(call.ctx, "CyberArk", "One-time code for "+strings.TrimSpace(s["username"]), true)
		if err != nil {
			return "", err
		}
		password += or(s["otpDelimiter"], ",") + strings.TrimSpace(code)
	}
	data, err := logon(password)
	for round := 0; err != nil && round < 3 && strings.Contains(err.Error(), pvwaChallenge); round++ {
		question := "Code for " + strings.TrimSpace(s["username"])
		if d, _ := errs.ToPayload(err).Params["detail"].(string); d != "" {
			question = strings.TrimSpace(strings.TrimPrefix(d, pvwaChallenge))
		}
		code, aerr := plugins.Ask(call.ctx, "CyberArk", question, true)
		if aerr != nil {
			return "", err // no way to ask (a background refresh): keep PVWA's message
		}
		data, err = logon(strings.TrimSpace(code))
	}
	if err != nil {
		return "", err
	}
	var token string
	if err := json.Unmarshal(data, &token); err != nil || token == "" {
		return "", badJSON(err, "cyberark logon")
	}
	return token, nil
}

// pvwaProbe runs in the pages of the browser sign-in. It reports the
// SAML response on its way to the PVWA (read while the identity
// provider's form posts it; navigating away stops that post, and when
// the response cannot be used the app posts it on to __action, so the
// PVWA web sign-in goes on),
// and, once the PVWA web app is open, the values it keeps in the
// browser's storage and cookies: one of them is the session token, which
// pvwaWebLogon finds by trying them on the API. A set already reported is
// not sent again.
const pvwaProbe = `(function () {
  if (location.href.indexOf(__SHELLVANE_RETURN__) === 0) return; // the app's own pages
  var out = {};
  var saml = document.querySelector('input[name="SAMLResponse"]');
  if (saml && saml.value) {
    out.SAMLResponse = saml.value;
    var form = saml.form;
    if (form) {
      out.__action = form.action || '';
      var relay = form.querySelector('input[name="RelayState"]');
      if (relay) out.RelayState = relay.value;
    }
  } else {
    var p = location.pathname.toLowerCase();
    if (p.indexOf('/passwordvault/') < 0 || p.indexOf('logon') >= 0 || p.indexOf('login') >= 0) return;
    [sessionStorage, localStorage].forEach(function (st, n) {
      try {
        for (var i = 0; i < st.length; i++) {
          var k = st.key(i), v = st.getItem(k) || '';
          if (k !== '__shellvane_sent' && v.length >= 16 && v.length < 8192) out[(n ? 'l:' : 's:') + k] = v;
        }
      } catch (e) {}
    });
    document.cookie.split(';').forEach(function (c) {
      var i = c.indexOf('=');
      var v = i > 0 ? decodeURIComponent(c.slice(i + 1).trim()) : '';
      if (v.length >= 16 && v.length < 8192) out['c:' + c.slice(0, i).trim()] = v;
    });
  }
  var keys = Object.keys(out).sort();
  if (!keys.length) return;
  var q = keys.map(function (k) { return encodeURIComponent(k) + '=' + encodeURIComponent(out[k]); }).join('&');
  try {
    if (sessionStorage.getItem('__shellvane_sent') === q) return;
    sessionStorage.setItem('__shellvane_sent', q);
  } catch (e) {}
  if (window.__shellvaneSent === q) return;
  window.__shellvaneSent = q;
  location.href = __SHELLVANE_RETURN__ + '?' + q;
})();`

// tokenCandidates are the reported values that could be a session token:
// the values themselves, a JSON string's content, a JSON object's string
// fields (one level down too).
func tokenCandidates(values map[string]string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(v string) {
		v = strings.TrimSpace(v)
		if len(v) >= 16 && len(v) < 8192 && !strings.ContainsAny(v, " \t\n{}[]") && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	var walk func(any, int)
	walk = func(x any, depth int) {
		switch t := x.(type) {
		case string:
			add(t)
		case map[string]any:
			if depth < 2 {
				for _, v := range t {
					walk(v, depth+1)
				}
			}
		}
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys) // stable order: tests, and fewer calls on retries
	for _, k := range keys {
		if k == "SAMLResponse" {
			continue
		}
		var x any
		if json.Unmarshal([]byte(values[k]), &x) == nil {
			walk(x, 0)
		} else {
			add(values[k])
		}
	}
	return out
}

// pvwaWebLogon signs in through the PVWA's SAML or OpenID Connect page in
// a window of the app (Entra ID, Okta… with their MFA): the SAML response
// is exchanged for a token on the API, or the token of the web session is
// taken over once the API accepts it.
func (p CyberArk) pvwaWebLogon(call pamCall, s map[string]string, base, method string) (string, error) {
	start := strings.TrimSpace(s["webLoginUrl"])
	if start == "" {
		start = trimURL(s["url"]) + "/PasswordVault/v10/logon/" + strings.ToLower(method)
	}
	if !strings.HasPrefix(strings.ToLower(start), "https://") && !strings.HasPrefix(strings.ToLower(start), "http://") {
		return "", errs.New(plugins.ErrSetting, errs.Params{"key": "webLoginUrl"})
	}
	var token string
	var seen []string // what the window reported, for the error: names only, no values
	accept := func(values map[string]string) bool {
		seen = seen[:0]
		for k := range values {
			if !strings.HasPrefix(k, "__") {
				seen = append(seen, k)
			}
		}
		sort.Strings(seen)
		if r := values["SAMLResponse"]; r != "" {
			form := url.Values{"concurrentSession": {"true"}, "apiUse": {"true"}, "SAMLResponse": {r}}
			data, err := call.do(http.MethodPost, base+"/auth/SAML/Logon", []byte(form.Encode()),
				http.Header{"Content-Type": {"application/x-www-form-urlencoded"}, "Accept": {"application/json"}}, cyberarkError)
			var t string
			if err == nil && json.Unmarshal(data, &t) == nil && t != "" {
				token = t
				return true
			}
			if err != nil {
				seen = append(seen, "(SAML exchange: "+err.Error()+")")
			}
			return false
		}
		for _, c := range tokenCandidates(values) {
			if p.pvwaWorks(call, base, c) {
				token = c
				return true
			}
		}
		return false
	}
	err := plugins.OpenWebLogin(call.ctx, "CyberArk", start, pvwaProbe, accept)
	if token != "" {
		return token, nil
	}
	if err != nil && errs.CodeOf(err) == plugins.ErrUnsupported {
		return "", err
	}
	detail := "the browser sign-in gave no PVWA session"
	if len(seen) > 0 {
		detail += " (seen: " + strings.Join(seen, ", ") + ")"
	} else if err != nil {
		detail += " (" + err.Error() + ")"
	}
	return "", errs.New(plugins.ErrCommand, errs.Params{"command": "cyberark", "detail": detail})
}

// pvwaWorks tells whether the API accepts token.
func (p CyberArk) pvwaWorks(call pamCall, base, token string) bool {
	_, err := call.do(http.MethodGet, base+"/Accounts?limit=1", nil, http.Header{"Accept": {"application/json"}, "Authorization": {token}}, cyberarkError)
	return err == nil
}

// pvwa uses the user's PVWA session (signing in when there is none),
// finds the account of the host and retrieves its secret.
func (p CyberArk) pvwa(call pamCall, s map[string]string, t plugins.Target) (plugins.Credential, error) {
	base := trimURL(s["url"]) + "/PasswordVault/API"
	method := or(s["authMethod"], "CyberArk")
	switch strings.ToLower(method) {
	case "cyberark", "ldap", "radius", "windows", "saml", "oidc":
	default:
		return plugins.Credential{}, errs.New(plugins.ErrSetting, errs.Params{"key": "authMethod"})
	}
	now := p.Now
	if now == nil {
		now = time.Now
	}
	key := pvwaKey(s)
	token := pvwaCached(key, now())
	fresh := false
	for {
		if token == "" {
			var err error
			logon := p.pvwaLogon
			if web := strings.ToLower(method); web == "saml" || web == "oidc" {
				logon = p.pvwaWebLogon
			}
			if token, err = logon(call, s, base, method); err != nil {
				return plugins.Credential{}, err
			}
			fresh = true
			pvwaStore(key, token, now())
		}
		cred, err := p.retrieve(call, s, t, base, token)
		if err == nil || fresh || errs.CodeOf(err) != errPVWAExpired {
			return cred, err
		}
		pvwaStore(key, "", now()) // the session expired on the PVWA: sign in again
		token = ""
	}
}

// defaultReason is the reason logged by the PVWA when none is set.
func defaultReason(t plugins.Target) string {
	if t.Purpose != "" {
		return "Shellvane: " + t.Purpose
	}
	return "Shellvane: SSH to " + t.Hostname
}

// errPVWAExpired marks a 401 on a kept session.
var errPVWAExpired = errs.Code("plugins.pvwaExpired")

func (p CyberArk) retrieve(call pamCall, s map[string]string, t plugins.Target, base, token string) (plugins.Credential, error) {
	auth := http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json"}, "Authorization": {token}}
	expired := func(err error) error {
		if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "PASWS006E") || strings.Contains(err.Error(), "PASWS013E") {
			return errs.Wrap(err, errPVWAExpired, nil)
		}
		return err
	}
	q := url.Values{"search": {strings.TrimSpace(t.Hostname + " " + t.User)}}
	if safe := strings.TrimSpace(or(t.Safe, s["safe"])); safe != "" {
		q.Set("filter", "safeName eq "+safe)
	}
	data, err := call.do(http.MethodGet, base+"/Accounts?"+q.Encode(), nil, auth, cyberarkError)
	if err != nil {
		return plugins.Credential{}, expired(err)
	}
	var list struct {
		Value []struct {
			ID       string `json:"id"`
			UserName string `json:"userName"`
			Address  string `json:"address"`
		} `json:"value"`
	}
	if err := json.Unmarshal(data, &list); err != nil {
		return plugins.Credential{}, badJSON(err, "cyberark accounts")
	}
	id, user := "", ""
	for _, a := range list.Value {
		if strings.EqualFold(a.Address, t.Hostname) && (t.User == "" || a.UserName == t.User) {
			id, user = a.ID, a.UserName
			break
		}
	}
	if id == "" {
		who := t.Hostname
		if t.User != "" {
			who = t.User + "@" + t.Hostname
		}
		return plugins.Credential{}, errs.New(plugins.ErrCommand, errs.Params{"command": "cyberark", "detail": "no account for " + who})
	}
	body, _ := json.Marshal(map[string]string{"reason": or(expandTarget(s["reason"], t), defaultReason(t))})
	data, err = call.do(http.MethodPost, base+"/Accounts/"+url.PathEscape(id)+"/Password/Retrieve", body, auth, cyberarkError)
	if err != nil {
		return plugins.Credential{}, expired(err)
	}
	var secret string
	if err := json.Unmarshal(data, &secret); err != nil || secret == "" {
		return plugins.Credential{}, badJSON(err, "cyberark retrieve")
	}
	return secretOf(secret, user), nil
}

// Conjur reads a host's password or SSH key from CyberArk Conjur (Conjur
// Open Source, Conjur Enterprise / Secrets Manager Self-Hosted, Conjur
// Cloud) with a host or user API key.
type Conjur struct {
	plugins.Base
	Client func(fingerprint string) *http.Client
}

// Manifest describes the plugin.
func (Conjur) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "conjur", Name: "CyberArk Conjur", Version: "1", Description: "plugins.conjur.description", BuiltIn: true,
		Capabilities: []string{plugins.CapCredentials},
		Settings: []plugins.Setting{
			{Key: "url", Label: "plugins.settings.url", Placeholder: "https://conjur.example.org", Required: true},
			{Key: "account", Label: "plugins.settings.conjurAccount", Placeholder: "myorg", Required: true},
			{Key: "login", Label: "plugins.settings.conjurLogin", Placeholder: "host/shellvane/laptop", Required: true},
			{Key: "apiKey", Label: "plugins.settings.conjurApiKey", Secret: true, Required: true},
			{Key: "authenticator", Label: "plugins.settings.conjurAuthenticator", Placeholder: "authn"},
			{Key: "passwordVariable", Label: "plugins.settings.conjurPassword", Placeholder: "ssh/{{host}}/password"},
			{Key: "keyVariable", Label: "plugins.settings.conjurKey", Placeholder: "ssh/{{host}}/private-key"},
			{Key: "userVariable", Label: "plugins.settings.conjurUser", Placeholder: "ssh/{{host}}/username"},
			{Key: "fingerprint", Label: "plugins.settings.fingerprint"},
		},
	}
}

// conjurError reads Conjur's {"error":{"message":…}}.
func conjurError(data []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &e) != nil {
		return ""
	}
	return e.Error.Message
}

// Credentials authenticates with the API key, then reads the variables
// set for the host: the private key, the password, the user.
func (p Conjur) Credentials(ctx context.Context, s map[string]string, t plugins.Target) (plugins.Credential, error) {
	if err := plugins.Require(s, "url", "account", "login", "apiKey"); err != nil {
		return plugins.Credential{}, err
	}
	if strings.TrimSpace(s["passwordVariable"]) == "" && strings.TrimSpace(s["keyVariable"]) == "" {
		return plugins.Credential{}, errs.New(plugins.ErrSetting, errs.Params{"key": "passwordVariable"})
	}
	authn := strings.Trim(or(s["authenticator"], "authn"), "/")
	if strings.Contains(authn, "..") || !strings.HasPrefix(authn, "authn") {
		return plugins.Credential{}, errs.New(plugins.ErrSetting, errs.Params{"key": "authenticator"})
	}
	mk := p.Client
	if mk == nil {
		mk = client
	}
	call := pamCall{ctx: ctx, c: mk(s["fingerprint"]), command: "conjur"}
	base := trimURL(s["url"])
	account := url.PathEscape(strings.TrimSpace(s["account"]))
	data, err := call.do(http.MethodPost, base+"/"+authn+"/"+account+"/"+url.PathEscape(strings.TrimSpace(s["login"]))+"/authenticate",
		[]byte(strings.TrimSpace(s["apiKey"])), http.Header{"Accept-Encoding": {"base64"}, "Content-Type": {"text/plain"}}, conjurError)
	if err != nil {
		return plugins.Credential{}, err
	}
	token := strings.TrimSpace(string(data))
	if strings.HasPrefix(token, "{") { // a server that ignored Accept-Encoding
		token = base64.StdEncoding.EncodeToString(data)
	}
	auth := http.Header{"Authorization": {`Token token="` + token + `"`}}
	read := func(key string) (string, error) {
		id := expandTarget(s[key], t)
		if id == "" {
			return "", nil
		}
		v, err := call.do(http.MethodGet, base+"/secrets/"+account+"/variable/"+url.PathEscape(id), nil, auth, conjurError)
		return string(v), err
	}
	var cred plugins.Credential
	if cred.PrivateKey, err = read("keyVariable"); err != nil {
		return plugins.Credential{}, err
	}
	if cred.PrivateKey == "" {
		if cred.Password, err = read("passwordVariable"); err != nil {
			return plugins.Credential{}, err
		}
	}
	if cred.User, err = read("userVariable"); err != nil {
		return plugins.Credential{}, err
	}
	cred.User = strings.TrimSpace(cred.User)
	if cred.Password == "" && cred.PrivateKey == "" {
		return plugins.Credential{}, errs.New(plugins.ErrResponse, errs.Params{"command": "conjur"})
	}
	return cred, nil
}
