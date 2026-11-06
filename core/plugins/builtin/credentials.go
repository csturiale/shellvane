package builtin

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

// Values put in Boundary's proxy command.
var (
	boundaryID    = regexp.MustCompile(`^[A-Za-z0-9_]{3,64}$`)
	boundaryToken = regexp.MustCompile(`^[A-Za-z0-9_.-]{8,}$`)
	boundaryAddr  = regexp.MustCompile(`^https?://[A-Za-z0-9.:\[\]-]+$`)
)

// HashiCorpVault brokers logins with Vault: an SSH certificate signed by
// its SSH secrets engine for a key made for each connection, or a
// password read from a KV secret.
type HashiCorpVault struct {
	plugins.Base
	Client func(fingerprint string) *http.Client
}

// Manifest describes the plugin.
func (HashiCorpVault) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "hashicorp-vault", Name: "HashiCorp Vault", Version: "1", Description: "plugins.vault.description", BuiltIn: true,
		Capabilities: []string{plugins.CapCredentials},
		Settings: []plugins.Setting{
			{Key: "url", Label: "plugins.settings.url", Placeholder: "https://vault.example.org:8200", Required: true},
			{Key: "token", Label: "plugins.settings.vaultToken", Secret: true, Required: true},
			{Key: "namespace", Label: "plugins.settings.vaultNamespace"},
			{Key: "mount", Label: "plugins.settings.vaultMount", Placeholder: "ssh-client-signer"},
			{Key: "role", Label: "plugins.settings.vaultRole", Placeholder: "admin"},
			{Key: "kvPath", Label: "plugins.settings.vaultKV", Placeholder: "secret/data/ssh/{{host}}"},
			{Key: "fingerprint", Label: "plugins.settings.fingerprint"},
		},
	}
}

// Credentials signs a new key, or reads the password when kvPath is set.
func (p HashiCorpVault) Credentials(ctx context.Context, s map[string]string, t plugins.Target) (plugins.Credential, error) {
	if err := plugins.Require(s, "url", "token"); err != nil {
		return plugins.Credential{}, err
	}
	mk := p.Client
	if mk == nil {
		mk = client
	}
	c := mk(s["fingerprint"])
	call := func(method, path string, body any, into any) error {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		url := trimURL(s["url"]) + "/v1/" + strings.TrimLeft(path, "/")
		req, err := http.NewRequestWithContext(ctx, method, url, rd)
		if err != nil {
			return errs.Wrap(err, plugins.ErrSetting, errs.Params{"key": "url"})
		}
		req.Header.Set("X-Vault-Token", strings.TrimSpace(s["token"]))
		if ns := strings.TrimSpace(s["namespace"]); ns != "" {
			req.Header.Set("X-Vault-Namespace", ns)
		}
		resp, err := c.Do(req)
		if err != nil {
			return errs.Wrap(err, plugins.ErrCommand, errs.Params{"command": "vault", "detail": err.Error()})
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode/100 != 2 {
			var e struct {
				Errors []string `json:"errors"`
			}
			json.Unmarshal(data, &e)
			detail := resp.Status
			if len(e.Errors) > 0 {
				detail = e.Errors[0]
			}
			return errs.New(plugins.ErrCommand, errs.Params{"command": "vault", "detail": detail})
		}
		if err := json.Unmarshal(data, into); err != nil {
			return badJSON(err, "vault")
		}
		return nil
	}

	if kv := strings.TrimSpace(s["kvPath"]); kv != "" {
		path := strings.NewReplacer("{{host}}", t.Hostname, "{{user}}", t.User, "{{name}}", t.Name).Replace(kv)
		var res struct {
			Data map[string]any `json:"data"`
		}
		if err := call(http.MethodGet, path, nil, &res); err != nil {
			return plugins.Credential{}, err
		}
		fields := res.Data
		if inner, ok := res.Data["data"].(map[string]any); ok {
			fields = inner // KV version 2
		}
		pw, _ := fields["password"].(string)
		if pw == "" {
			return plugins.Credential{}, errs.New(plugins.ErrResponse, errs.Params{"command": "vault " + path})
		}
		user, _ := fields["username"].(string)
		return plugins.Credential{Password: pw, User: user}, nil
	}

	if err := plugins.Require(s, "role"); err != nil {
		return plugins.Credential{}, err
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return plugins.Credential{}, err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return plugins.Credential{}, err
	}
	body := map[string]any{"public_key": string(ssh.MarshalAuthorizedKey(sshPub))}
	if t.User != "" {
		body["valid_principals"] = t.User
	}
	mount := or(strings.Trim(strings.TrimSpace(s["mount"]), "/"), "ssh-client-signer")
	var res struct {
		Data struct {
			SignedKey string `json:"signed_key"`
		} `json:"data"`
	}
	if err := call(http.MethodPost, mount+"/sign/"+strings.TrimSpace(s["role"]), body, &res); err != nil {
		return plugins.Credential{}, err
	}
	if res.Data.SignedKey == "" {
		return plugins.Credential{}, errs.New(plugins.ErrResponse, errs.Params{"command": "vault"})
	}
	block, err := ssh.MarshalPrivateKey(priv, "shellvane vault")
	if err != nil {
		return plugins.Credential{}, err
	}
	return plugins.Credential{PrivateKey: string(pem.EncodeToMemory(block)), Certificate: strings.TrimSpace(res.Data.SignedKey)}, nil
}

// Teleport logs in through a Teleport cluster with the certificates of
// `tsh login` and its proxy.
type Teleport struct {
	plugins.Base
	Cmd      Runner
	ReadFile func(string) ([]byte, error)
}

// Manifest describes the plugin.
func (Teleport) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "teleport", Name: "Teleport", Version: "1", Description: "plugins.teleport.description", BuiltIn: true,
		Capabilities: []string{plugins.CapCredentials}, Requires: []string{"tsh"},
		Settings: []plugins.Setting{
			{Key: "proxy", Label: "plugins.settings.teleportProxy", Placeholder: "teleport.example.org:443", Required: true},
			{Key: "cluster", Label: "plugins.settings.teleportCluster"},
			{Key: "user", Label: "plugins.settings.user", Placeholder: "root"},
		},
	}
}

// Credentials reads the key, certificate, node port and proxy command
// from `tsh config`; the user must have run `tsh login` (asked otherwise).
func (p Teleport) Credentials(ctx context.Context, s map[string]string, _ plugins.Target) (plugins.Credential, error) {
	if err := plugins.Require(s, "proxy"); err != nil {
		return plugins.Credential{}, err
	}
	proxy := "--proxy=" + strings.TrimSpace(s["proxy"])
	if _, err := p.Cmd(ctx, "tsh", "status", proxy); err != nil {
		return plugins.Credential{}, errs.New(plugins.ErrCommand, errs.Params{"command": "tsh", "detail": "tsh login " + proxy})
	}
	args := []string{"config", proxy}
	args = appendIf(args, "--cluster", s["cluster"])
	out, err := p.Cmd(ctx, "tsh", args...)
	if err != nil {
		return plugins.Credential{}, err
	}
	conf := parseSSHConfig(out)
	read := p.ReadFile
	if read == nil {
		read = os.ReadFile
	}
	key, err := read(expandHome(conf["identityfile"]))
	if err != nil {
		return plugins.Credential{}, errs.Wrap(err, plugins.ErrResponse, errs.Params{"command": "tsh config"})
	}
	cert, err := read(expandHome(conf["certificatefile"]))
	if err != nil {
		return plugins.Credential{}, errs.Wrap(err, plugins.ErrResponse, errs.Params{"command": "tsh config"})
	}
	port, _ := strconv.Atoi(conf["port"])
	return plugins.Credential{PrivateKey: string(key), Certificate: strings.TrimSpace(string(cert)), ProxyCommand: conf["proxycommand"], Port: port, User: s["user"]}, nil
}

// parseSSHConfig reads the first value of each option in OpenSSH config
// text, keys in lower case, quotes removed from single values.
func parseSSHConfig(out []byte) map[string]string {
	conf := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		key = strings.ToLower(key)
		value = strings.TrimSpace(value)
		if key == "host" || conf[key] != "" {
			continue
		}
		if key != "proxycommand" {
			value = strings.Trim(value, `"`)
		}
		conf[key] = value
	}
	return conf
}

// Boundary connects through a HashiCorp Boundary target: it authorizes a
// session, uses the credentials Boundary brokers and connects through
// `boundary connect`.
type Boundary struct {
	plugins.Base
	Cmd Runner
}

// Manifest describes the plugin.
func (Boundary) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "boundary", Name: "HashiCorp Boundary", Version: "1", Description: "plugins.boundary.description", BuiltIn: true,
		Capabilities: []string{plugins.CapCredentials}, Requires: []string{"boundary", "nc"},
		Settings: []plugins.Setting{
			{Key: "target", Label: "plugins.settings.boundaryTarget", Placeholder: "ttcp_1234567890", Required: true},
			{Key: "addr", Label: "plugins.settings.boundaryAddr", Placeholder: "https://boundary.example.org:9200"},
			{Key: "user", Label: "plugins.settings.user"},
		},
	}
}

// Credentials authorizes a session and returns its brokered credentials
// with the command that connects through it.
func (p Boundary) Credentials(ctx context.Context, s map[string]string, _ plugins.Target) (plugins.Credential, error) {
	if err := plugins.Require(s, "target"); err != nil {
		return plugins.Credential{}, err
	}
	target := strings.TrimSpace(s["target"])
	if !boundaryID.MatchString(target) {
		return plugins.Credential{}, errs.New(plugins.ErrSetting, errs.Params{"key": "target"})
	}
	args := appendIf([]string{"targets", "authorize-session", "-id", target, "-format", "json"}, "-addr", s["addr"])
	out, err := p.Cmd(ctx, "boundary", args...)
	if err != nil {
		return plugins.Credential{}, err
	}
	var res struct {
		Item struct {
			AuthorizationToken string `json:"authorization_token"`
			Credentials        []struct {
				Secret struct {
					Decoded struct {
						Username   string `json:"username"`
						Password   string `json:"password"`
						PrivateKey string `json:"private_key"`
					} `json:"decoded"`
				} `json:"secret"`
			} `json:"credentials"`
		} `json:"item"`
	}
	if err := json.Unmarshal(out, &res); err != nil || res.Item.AuthorizationToken == "" {
		return plugins.Credential{}, badJSON(err, "boundary")
	}
	if len(res.Item.AuthorizationToken) > 8192 || !boundaryToken.MatchString(res.Item.AuthorizationToken) {
		return plugins.Credential{}, errs.New(plugins.ErrResponse, errs.Params{"command": "boundary"})
	}
	cred := plugins.Credential{User: s["user"]}
	for _, c := range res.Item.Credentials {
		d := c.Secret.Decoded
		cred.User = or(d.Username, cred.User)
		cred.Password = or(d.Password, cred.Password)
		cred.PrivateKey = or(d.PrivateKey, cred.PrivateKey)
	}
	addr := ""
	if a := strings.TrimSpace(s["addr"]); a != "" && boundaryAddr.MatchString(a) {
		addr = " -addr " + a
	}
	cred.ProxyCommand = "boundary connect" + addr + " -authz-token " + res.Item.AuthorizationToken + " -exec nc -- {{boundary.ip}} {{boundary.port}}"
	return cred, nil
}
