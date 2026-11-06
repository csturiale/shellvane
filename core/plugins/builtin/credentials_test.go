package builtin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/plugins"
)

func TestVaultSignsAKeyAndReadsKV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "hvs.x" || r.Header.Get("X-Vault-Namespace") != "team" {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"errors":["permission denied"]}`))
			return
		}
		switch r.URL.Path {
		case "/v1/ssh-client-signer/sign/ops":
			var body map[string]string
			data, _ := io.ReadAll(r.Body)
			json.Unmarshal(data, &body)
			if body["valid_principals"] != "deploy" || !strings.HasPrefix(body["public_key"], "ssh-ed25519 ") {
				t.Errorf("sign body = %v", body)
			}
			w.Write([]byte(`{"data":{"signed_key":"ssh-ed25519-cert-v01@openssh.com AAAAfake"}}`))
		case "/v1/secret/data/ssh/db.internal":
			w.Write([]byte(`{"data":{"data":{"username":"admin","password":"pw!"}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	s := map[string]string{"url": srv.URL, "token": "hvs.x", "namespace": "team", "role": "ops"}
	cred, err := HashiCorpVault{}.Credentials(context.Background(), s, plugins.Target{User: "deploy", Hostname: "web"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ssh.ParseRawPrivateKey([]byte(cred.PrivateKey)); err != nil || !strings.HasPrefix(cred.Certificate, "ssh-ed25519-cert") {
		t.Fatalf("cred = %+v, key error %v", cred, err)
	}
	s["kvPath"] = "secret/data/ssh/{{host}}"
	cred, err = HashiCorpVault{}.Credentials(context.Background(), s, plugins.Target{Hostname: "db.internal"})
	if err != nil || cred.Password != "pw!" || cred.User != "admin" {
		t.Fatalf("kv = %+v, %v", cred, err)
	}
	s["token"] = "wrong"
	if _, err := (HashiCorpVault{}).Credentials(context.Background(), s, plugins.Target{Hostname: "db.internal"}); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("denied: %v", err)
	}
}

func TestTeleportReadsTshConfig(t *testing.T) {
	var calls []string
	tsh := func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if args[0] == "status" {
			return []byte("> Profile URL: https://tp:443"), nil
		}
		return []byte(`# Common flags for all prod hosts
Host *.prod tp.example.org
    UserKnownHostsFile "/home/u/.tsh/known_hosts"
    IdentityFile "/home/u/.tsh/keys/tp.example.org/alice"
    CertificateFile "/home/u/.tsh/keys/tp.example.org/alice-ssh/prod-cert.pub"

# Flags for all prod hosts except the proxy
Host *.prod !tp.example.org
    Port 3022
    ProxyCommand "/usr/local/bin/tsh" proxy ssh --cluster=prod --proxy=tp.example.org:443 %r@%h:%p
`), nil
	}
	files := map[string]string{
		"/home/u/.tsh/keys/tp.example.org/alice":                   "-----BEGIN KEY-----",
		"/home/u/.tsh/keys/tp.example.org/alice-ssh/prod-cert.pub": "ssh-rsa-cert-v01@openssh.com AAAA\n",
	}
	read := func(p string) ([]byte, error) { return []byte(files[p]), nil }
	cred, err := Teleport{Cmd: tsh, ReadFile: read}.Credentials(context.Background(), map[string]string{"proxy": "tp.example.org:443", "cluster": "prod", "user": "root"}, plugins.Target{})
	if err != nil {
		t.Fatal(err)
	}
	if cred.PrivateKey != "-----BEGIN KEY-----" || cred.Certificate != "ssh-rsa-cert-v01@openssh.com AAAA" || cred.Port != 3022 || cred.User != "root" ||
		cred.ProxyCommand != `"/usr/local/bin/tsh" proxy ssh --cluster=prod --proxy=tp.example.org:443 %r@%h:%p` {
		t.Fatalf("cred = %+v", cred)
	}
	if calls[1] != "tsh config --proxy=tp.example.org:443 --cluster prod" {
		t.Errorf("calls = %v", calls)
	}
	failing := func(context.Context, string, ...string) ([]byte, error) { return nil, io.EOF }
	if _, err := (Teleport{Cmd: failing}).Credentials(context.Background(), map[string]string{"proxy": "tp:443"}, plugins.Target{}); err == nil || !strings.Contains(err.Error(), "tsh login") {
		t.Fatalf("not logged in: %v", err)
	}
}

func TestBoundaryAuthorizesASession(t *testing.T) {
	var calls []string
	b := Boundary{Cmd: fakeRun(`{"item":{"authorization_token":"at_ABC123def456","credentials":[{"secret":{"decoded":{"username":"ubuntu","password":"s3"}}}]}}`, &calls)}
	cred, err := b.Credentials(context.Background(), map[string]string{"target": "ttcp_1234567890", "addr": "https://bd.example.org:9200"}, plugins.Target{})
	if err != nil {
		t.Fatal(err)
	}
	if cred.User != "ubuntu" || cred.Password != "s3" ||
		cred.ProxyCommand != "boundary connect -addr https://bd.example.org:9200 -authz-token at_ABC123def456 -exec nc -- {{boundary.ip}} {{boundary.port}}" {
		t.Fatalf("cred = %+v", cred)
	}
	if !strings.Contains(calls[0], "targets authorize-session -id ttcp_1234567890 -format json -addr https://bd.example.org:9200") {
		t.Errorf("call = %s", calls[0])
	}
	if _, err := b.Credentials(context.Background(), map[string]string{"target": "x; rm -rf /"}, plugins.Target{}); err == nil {
		t.Fatal("an unsafe target was accepted")
	}
}
