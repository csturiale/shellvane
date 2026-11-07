package builtin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

func TestCyberArkCredentialProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/AIMWebService/api/Accounts" || q.Get("AppID") != "Shellvane" || q.Get("Safe") != "Linux" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"ErrorCode":"APPAP306E","ErrorMsg":"not authorized"}`))
			return
		}
		switch {
		case q.Get("Address") == "web" && q.Get("UserName") == "deploy":
			w.Write([]byte(`{"Content":"pw!","UserName":"deploy","Address":"web"}`))
		case q.Get("Object") == "db-root":
			w.Write([]byte(`{"Content":"-----BEGIN OPENSSH PRIVATE KEY-----\nAAA\n-----END OPENSSH PRIVATE KEY-----","UserName":"root"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"ErrorCode":"APPAP004E","ErrorMsg":"Password object matching query not found"}`))
		}
	}))
	defer srv.Close()
	s := map[string]string{"url": srv.URL + "/", "appId": "Shellvane", "safe": "Linux"}
	cred, err := CyberArk{}.Credentials(context.Background(), s, plugins.Target{Hostname: "web", User: "deploy"})
	if err != nil || cred.Password != "pw!" || cred.User != "deploy" {
		t.Fatalf("by address = %+v, %v", cred, err)
	}
	s["object"] = "{{host}}-{{user}}"
	cred, err = CyberArk{}.Credentials(context.Background(), s, plugins.Target{Hostname: "db", User: "root"})
	if err != nil || !strings.Contains(cred.PrivateKey, "PRIVATE KEY") || cred.Password != "" {
		t.Fatalf("by object = %+v, %v", cred, err)
	}
	_, err = CyberArk{}.Credentials(context.Background(), s, plugins.Target{Hostname: "nope", User: "x"})
	if errs.CodeOf(err) != plugins.ErrCommand || !strings.Contains(err.Error(), "APPAP004E") {
		t.Fatalf("missing = %v", err)
	}
	s["clientCert"] = "/does/not/exist.pem"
	if _, err := (CyberArk{}).Credentials(context.Background(), s, plugins.Target{Hostname: "web"}); errs.CodeOf(err) != plugins.ErrSetting {
		t.Fatalf("bad certificate: %v", err)
	}
}

// fakePVWA answers logons (RADIUS challenge when the password is
// "secret" and the method RADIUS), account searches and retrieves.
type fakePVWA struct {
	logons   int
	tokens   map[string]bool
	searches int
}

func (f *fakePVWA) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.HasSuffix(r.URL.Path, "/Logon") {
			var b map[string]any
			json.Unmarshal(body, &b)
			radius := strings.Contains(r.URL.Path, "/RADIUS/")
			switch {
			case b["username"] != "alice":
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"ErrorCode":"ITATS004E","ErrorMessage":"Authentication failure"}`))
				return
			case radius && b["password"] == "secret":
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"ErrorCode":"ITATS542I","ErrorMessage":"Enter the code from your authenticator"}`))
				return
			case radius && b["password"] != "123456" && b["password"] != "secret,123456":
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"ErrorCode":"ITATS004E","ErrorMessage":"Authentication failure"}`))
				return
			case !radius && b["password"] != "secret":
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"ErrorCode":"ITATS004E","ErrorMessage":"Authentication failure"}`))
				return
			}
			f.logons++
			tok := "tok" + string(rune('0'+f.logons))
			f.tokens[tok] = true
			w.Write([]byte(`"` + tok + `"`))
			return
		}
		if !f.tokens[r.Header.Get("Authorization")] {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"ErrorCode":"PASWS006E","ErrorMessage":"Your session has expired"}`))
			return
		}
		switch r.URL.Path {
		case "/PasswordVault/API/Accounts":
			f.searches++
			if r.URL.Query().Get("filter") != "safeName eq Linux" {
				t.Errorf("filter = %q", r.URL.Query().Get("filter"))
			}
			w.Write([]byte(`{"value":[{"id":"1_1","userName":"other","address":"web"},{"id":"1_2","userName":"deploy","address":"WEB"}],"count":2}`))
		case "/PasswordVault/API/Accounts/1_2/Password/Retrieve":
			if !strings.Contains(string(body), "SSH to web") {
				t.Errorf("reason = %s", body)
			}
			w.Write([]byte(`"pw!"`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func resetPVWA() {
	pvwaSessions.Lock()
	pvwaSessions.m = map[string]pvwaSession{}
	pvwaSessions.Unlock()
}

func TestCyberArkPVWAKeepsTheSession(t *testing.T) {
	resetPVWA()
	f := &fakePVWA{tokens: map[string]bool{}}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	clock := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	ca := CyberArk{Now: func() time.Time { return clock }}
	s := map[string]string{"url": srv.URL, "username": "alice", "password": "secret", "authMethod": "LDAP", "safe": "Linux", "reason": "SSH to {{host}}"}
	web := plugins.Target{Hostname: "web", User: "deploy"}
	for i := 0; i < 3; i++ {
		cred, err := ca.Credentials(context.Background(), s, web)
		if err != nil || cred.Password != "pw!" || cred.User != "deploy" {
			t.Fatalf("pvwa = %+v, %v", cred, err)
		}
		clock = clock.Add(5 * time.Minute)
	}
	if f.logons != 1 {
		t.Fatalf("logons = %d, want one session for three connections", f.logons)
	}
	// The PVWA ended the session: Shellvane signs in again by itself.
	f.tokens = map[string]bool{}
	if _, err := ca.Credentials(context.Background(), s, web); err != nil || f.logons != 2 {
		t.Fatalf("expired session: %v, logons %d", err, f.logons)
	}
	// Unused for longer than pvwaIdle: a new session.
	clock = clock.Add(pvwaIdle + time.Minute)
	if _, err := ca.Credentials(context.Background(), s, web); err != nil || f.logons != 3 {
		t.Fatalf("idle session: %v, logons %d", err, f.logons)
	}
	if _, err := ca.Credentials(context.Background(), s, plugins.Target{Hostname: "db", User: "root"}); err == nil || !strings.Contains(err.Error(), "root@db") {
		t.Fatalf("no account: %v", err)
	}
	resetPVWA()
	s["password"] = "wrong"
	if _, err := ca.Credentials(context.Background(), s, web); err == nil || !strings.Contains(err.Error(), "Authentication failure") {
		t.Fatalf("bad login: %v", err)
	}
	s["authMethod"] = "../x"
	if _, err := ca.Credentials(context.Background(), s, web); errs.CodeOf(err) != plugins.ErrSetting {
		t.Fatalf("bad method: %v", err)
	}
	delete(s, "password")
	if _, err := ca.Credentials(context.Background(), s, web); errs.CodeOf(err) != plugins.ErrSetting {
		t.Fatalf("no password: %v", err)
	}
}

func TestCyberArkPVWARadiusCode(t *testing.T) {
	resetPVWA()
	f := &fakePVWA{tokens: map[string]bool{}}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()
	s := map[string]string{"url": srv.URL, "username": "alice", "password": "secret", "authMethod": "RADIUS", "safe": "Linux", "reason": "SSH to {{host}}"}
	web := plugins.Target{Hostname: "web", User: "deploy"}
	// In the background nobody can type the code: PVWA's question is the error.
	if _, err := (CyberArk{}).Credentials(context.Background(), s, web); err == nil || !strings.Contains(err.Error(), "authenticator") {
		t.Fatalf("no asker: %v", err)
	}
	var asked []string
	ctx := plugins.WithAsker(context.Background(), func(_ context.Context, title, q string, secret bool) (string, error) {
		asked = append(asked, title+": "+q)
		if !secret {
			t.Error("the code must be hidden")
		}
		return "123456", nil
	})
	cred, err := CyberArk{}.Credentials(ctx, s, web)
	if err != nil || cred.Password != "pw!" || len(asked) != 1 || asked[0] != "CyberArk: Enter the code from your authenticator" {
		t.Fatalf("challenge = %+v, %v, asked %v", cred, err, asked)
	}
	// Append mode: password and code in one logon.
	resetPVWA()
	s["otpMode"] = "append"
	asked = nil
	if _, err := (CyberArk{}).Credentials(ctx, s, web); err != nil || len(asked) != 1 {
		t.Fatalf("append = %v, asked %v", err, asked)
	}
}

func TestConjurReadsVariables(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.EscapedPath() {
		case "/authn/myorg/host%2Fshellvane/authenticate":
			body, _ := io.ReadAll(r.Body)
			if string(body) != "key1" || r.Header.Get("Accept-Encoding") != "base64" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Write([]byte("dG9rZW4="))
		case "/secrets/myorg/variable/ssh%2Fweb%2Fpassword", "/secrets/myorg/variable/ssh%2Fweb%2Fusername":
			if r.Header.Get("Authorization") != `Token token="dG9rZW4="` {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if strings.HasSuffix(r.URL.EscapedPath(), "username") {
				w.Write([]byte("deploy\n"))
			} else {
				w.Write([]byte("pw!"))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":{"code":"not_found","message":"CONJ00076E Variable myorg:variable:x is empty or not found."}}`))
		}
	}))
	defer srv.Close()
	s := map[string]string{"url": srv.URL, "account": "myorg", "login": "host/shellvane", "apiKey": "key1",
		"passwordVariable": "ssh/{{host}}/password", "userVariable": "ssh/{{host}}/username"}
	cred, err := Conjur{}.Credentials(context.Background(), s, plugins.Target{Hostname: "web"})
	if err != nil || cred.Password != "pw!" || cred.User != "deploy" {
		t.Fatalf("conjur = %+v, %v", cred, err)
	}
	if _, err := (Conjur{}).Credentials(context.Background(), s, plugins.Target{Hostname: "db"}); err == nil || !strings.Contains(err.Error(), "CONJ00076E") {
		t.Fatalf("missing variable: %v", err)
	}
	s["apiKey"] = "wrong"
	if _, err := (Conjur{}).Credentials(context.Background(), s, plugins.Target{Hostname: "web"}); errs.CodeOf(err) != plugins.ErrCommand {
		t.Fatalf("bad key: %v", err)
	}
	s["authenticator"] = "../admin"
	if _, err := (Conjur{}).Credentials(context.Background(), s, plugins.Target{Hostname: "web"}); errs.CodeOf(err) != plugins.ErrSetting {
		t.Fatalf("bad authenticator: %v", err)
	}
	delete(s, "authenticator")
	delete(s, "passwordVariable")
	if _, err := (Conjur{}).Credentials(context.Background(), s, plugins.Target{Hostname: "web"}); errs.CodeOf(err) != plugins.ErrSetting {
		t.Fatalf("no variable: %v", err)
	}
}

func TestCyberArkPVWABrowserSignIn(t *testing.T) {
	resetPVWA()
	logons := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/PasswordVault/API/auth/SAML/Logon":
			r.ParseForm()
			if r.Form.Get("SAMLResponse") != "PHNhbWw+" || r.Form.Get("apiUse") != "true" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			logons++
			w.Write([]byte(`"saml-token-123456"`))
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "saml-token-123456" && auth != "web-session-token-abcdef" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/PasswordVault/API/Accounts":
			w.Write([]byte(`{"value":[{"id":"7_1","userName":"deploy","address":"web"}]}`))
		case "/PasswordVault/API/Accounts/7_1/Password/Retrieve":
			w.Write([]byte(`"pw!"`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	web := plugins.Target{Hostname: "web", User: "deploy"}
	s := map[string]string{"url": srv.URL, "authMethod": "SAML"}

	// Without a window (a background refresh) there is no sign-in.
	if _, err := (CyberArk{}).Credentials(context.Background(), s, web); errs.CodeOf(err) != plugins.ErrUnsupported {
		t.Fatalf("no window: %v", err)
	}

	var opened []string
	signIn := func(reports ...map[string]string) plugins.WebLogin {
		return func(_ context.Context, title, u, probe string, accept func(map[string]string) bool) error {
			opened = append(opened, u)
			if !strings.Contains(probe, "__SHELLVANE_RETURN__") {
				t.Error("the probe must report through __SHELLVANE_RETURN__")
			}
			for _, r := range reports {
				if accept(r) {
					return nil
				}
			}
			return errs.New(plugins.ErrCommand, errs.Params{"command": "sign-in", "detail": "timed out"})
		}
	}
	// The SAML response, read before it reaches the PVWA.
	ctx := plugins.WithWebLogin(context.Background(), signIn(map[string]string{"SAMLResponse": "PHNhbWw+"}))
	cred, err := CyberArk{}.Credentials(ctx, s, web)
	if err != nil || cred.Password != "pw!" || logons != 1 || opened[0] != srv.URL+"/PasswordVault/v10/logon/saml" {
		t.Fatalf("saml = %+v, %v, logons %d, opened %v", cred, err, logons, opened)
	}
	// Kept: the next connection does not open the window again.
	if _, err := (CyberArk{}).Credentials(ctx, s, web); err != nil || len(opened) != 1 {
		t.Fatalf("reuse: %v, opened %v", err, opened)
	}

	// OpenID Connect: the web session's token, found among the storage values.
	resetPVWA()
	s = map[string]string{"url": srv.URL, "authMethod": "OIDC", "webLoginUrl": srv.URL + "/PasswordVault/v10/logon/oidc?app=x"}
	ctx = plugins.WithWebLogin(context.Background(), signIn(
		map[string]string{"l:lang": "en-US-language-setting"},
		map[string]string{"l:lang": "en-US-language-setting", "s:session": `{"user":"alice-the-admin","token":"web-session-token-abcdef"}`},
	))
	if cred, err := (CyberArk{}).Credentials(ctx, s, web); err != nil || cred.Password != "pw!" {
		t.Fatalf("oidc = %+v, %v", cred, err)
	}

	// Closed window or no session found: an error, nothing kept.
	resetPVWA()
	ctx = plugins.WithWebLogin(context.Background(), signIn(map[string]string{"l:x": "not-a-token-at-all-really"}))
	if _, err := (CyberArk{}).Credentials(ctx, s, web); err == nil {
		t.Fatal("no session must fail")
	}
	s["webLoginUrl"] = "javascript:alert(1)"
	if _, err := (CyberArk{}).Credentials(ctx, s, web); errs.CodeOf(err) != plugins.ErrSetting {
		t.Fatalf("bad url: %v", err)
	}
}

func TestTokenCandidates(t *testing.T) {
	got := tokenCandidates(map[string]string{
		"SAMLResponse": "ignored-saml-response-value",
		"s:a":          `"quoted-token-0123456789"`,
		"s:b":          `{"x":{"token":"nested-token-0123456789"},"short":"no"}`,
		"c:c":          "plain cookie value with spaces",
	})
	want := []string{"quoted-token-0123456789", "nested-token-0123456789"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("candidates = %v", got)
	}
}
