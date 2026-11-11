package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/environments"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/shellenv"
	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/tunnel"
	"github.com/csturiale/shellvane/core/workspaces"
)

// EnvironmentService manages environments; the frontend runs their steps
// with the session, tunnel and workspace services.
type EnvironmentService struct {
	repo       *environments.Repo
	hosts      *hosts.Repo
	tunnels    *tunnel.Repo
	workspaces *workspaces.Repo

	// Set by EnableEnvironmentLogin: login secrets from credential
	// plugins and gcloud profiles.
	sessions *SessionService
	configs  *plugins.Configs
	dataDir  string
}

// EnableEnvironmentLogin lets environments read their login secret from a
// credential plugin and keep gcloud profiles under dataDir. It is not a
// method so that Wails does not expose it as a binding.
func EnableEnvironmentLogin(s *EnvironmentService, sessions *SessionService, configs *plugins.Configs, dataDir string) {
	s.sessions, s.configs, s.dataDir = sessions, configs, dataDir
}

// NewEnvironmentService returns a service; the other repositories resolve
// names on export and import.
func NewEnvironmentService(repo *environments.Repo, h *hosts.Repo, t *tunnel.Repo, w *workspaces.Repo) *EnvironmentService {
	return &EnvironmentService{repo: repo, hosts: h, tunnels: t, workspaces: w}
}

// List returns every environment.
func (s *EnvironmentService) List(ctx context.Context) ([]environments.Environment, error) {
	return s.repo.List(ctx)
}

// Save creates (empty ID) or updates an environment.
func (s *EnvironmentService) Save(ctx context.Context, e environments.Environment) (environments.Environment, error) {
	return s.repo.Save(ctx, e)
}

// Delete removes an environment.
func (s *EnvironmentService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// LoginCheck is the result of a local login check.
type LoginCheck struct {
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// CheckLogin runs a check command on this computer (e.g.
// `gcloud auth print-access-token`): a zero exit status means logged in.
// {{variables}} are expanded first; gcloud is the environment's gcloud
// profile ("" for gcloud's own configuration).
func (s *EnvironmentService) CheckLogin(ctx context.Context, command string, values map[string]string, gcloud string) LoginCheck {
	command = strings.TrimSpace(environments.Expand(command, values))
	if command == "" {
		return LoginCheck{OK: true}
	}
	var env []string
	if dir, err := gcloudDir(s.dataDir, gcloud); err != nil {
		return LoginCheck{Detail: err.Error()}
	} else if dir != "" {
		env = append(env, "CLOUDSDK_CONFIG="+dir)
	}
	if detail, ok := runInUserShell(ctx, command, env...); !ok {
		return LoginCheck{Detail: detail}
	}
	return LoginCheck{OK: true}
}

// environmentCommandTimeout bounds a check or preparation command.
const environmentCommandTimeout = 2 * time.Minute

// runInUserShell runs a command the way the user's terminal would: in
// their shell, reading its login and interactive files (where gcloud, aws
// or kubectl are often put on the PATH). It reports whether the command
// succeeded and, if not, the last line it printed as the reason.
func runInUserShell(ctx context.Context, command string, env ...string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, environmentCommandTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, shellenv.UserShell(), "-ilc", command)
	}
	cmd.Env = append(append(os.Environ(), "SHELLVANE_ENVIRONMENT=1"), env...)
	// Startup files may leave background processes (ssh-agent, gitstatusd)
	// holding the output: once the shell exits, stop waiting for them.
	cmd.WaitDelay = 2 * time.Second
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if err == nil || errors.Is(err, exec.ErrWaitDelay) {
		return "", true
	}
	if ctx.Err() != nil {
		return "timeout", false
	}
	detail := strings.TrimSpace(out.String())
	if i := strings.LastIndex(detail, "\n"); i >= 0 {
		detail = strings.TrimSpace(detail[i+1:])
	}
	if detail == "" {
		detail = err.Error()
	}
	if len(detail) > 300 {
		detail = detail[:300]
	}
	return detail, false
}

// Export renders an environment as YAML, with names instead of IDs.
func (s *EnvironmentService) Export(ctx context.Context, id string) (string, error) {
	e, err := s.repo.Get(ctx, id)
	if err != nil {
		return "", err
	}
	return environments.Export(e, func(kind, id string) string { return s.name(ctx, kind, id) })
}

// Import saves an exported environment, linking hosts, tunnels and
// workspaces by name.
func (s *EnvironmentService) Import(ctx context.Context, text string) (environments.Environment, error) {
	e, err := environments.Import(text, func(kind, name string) string { return s.idOf(ctx, kind, name) })
	if err != nil {
		return environments.Environment{}, err
	}
	return s.repo.Save(ctx, e)
}

func (s *EnvironmentService) name(ctx context.Context, kind, id string) string {
	if id == "" {
		return ""
	}
	switch kind {
	case "host":
		if h, err := s.hosts.Host(ctx, id); err == nil {
			return h.Name
		}
	case "tunnel":
		if t, err := s.tunnels.Get(ctx, id); err == nil {
			return t.Name
		}
	case "workspace":
		if list, err := s.workspaces.List(ctx); err == nil {
			for _, w := range list {
				if w.ID == id {
					return w.Name
				}
			}
		}
	case "plugin":
		if s.configs != nil {
			if c, err := s.configs.Get(ctx, id); err == nil {
				return c.Name
			}
		}
	}
	return ""
}

func (s *EnvironmentService) idOf(ctx context.Context, kind, name string) string {
	if name == "" {
		return ""
	}
	switch kind {
	case "host":
		if inv, err := s.hosts.List(ctx); err == nil {
			for _, h := range inv.Hosts {
				if strings.EqualFold(h.Name, name) {
					return h.ID
				}
			}
		}
	case "tunnel":
		if list, err := s.tunnels.List(ctx); err == nil {
			for _, t := range list {
				if strings.EqualFold(t.Name, name) {
					return t.ID
				}
			}
		}
	case "workspace":
		if list, err := s.workspaces.List(ctx); err == nil {
			for _, w := range list {
				if strings.EqualFold(w.Name, name) {
					return w.ID
				}
			}
		}
	case "plugin":
		if s.configs != nil {
			if list, err := s.configs.List(ctx); err == nil {
				for _, c := range list {
					if strings.EqualFold(c.Name, name) {
						return c.ID
					}
				}
			}
		}
	}
	return ""
}

// Error codes of environment logins.
const (
	ErrNoLoginSecret    errs.Code = "environments.noLoginSecret"
	ErrLoginSecretEmpty errs.Code = "environments.loginSecretEmpty"
	ErrClipboard        errs.Code = "environments.clipboard"
)

// How long a copied login secret stays on the clipboard: by default, and
// the bounds of the setting.
const (
	clipboardSeconds    = 45
	clipboardSecondsMin = 10
	clipboardSecondsMax = 600
)

// CopiedSecret tells the frontend what was copied (never the secret).
type CopiedSecret struct {
	User    string `json:"user"`
	Seconds int    `json:"seconds"`
}

// CopyLoginSecret reads the password of the environment's login secret
// from its credential plugin (signing in to it, with MFA, when needed)
// and puts it on the clipboard for the sign-in page the login opens. The
// clipboard is cleared after seconds (0: the default) unless something
// else was copied meanwhile. The password never reaches the frontend.
func (s *EnvironmentService) CopyLoginSecret(ctx context.Context, envID string, values map[string]string, seconds int) (CopiedSecret, error) {
	e, err := s.repo.Get(ctx, envID)
	if err != nil {
		return CopiedSecret{}, err
	}
	ref := e.Login.Secret
	if ref.Source == "" || s.sessions == nil {
		return CopiedSecret{}, errs.New(ErrNoLoginSecret, nil)
	}
	s.sessions.mu.Lock()
	broker := s.sessions.broker
	s.sessions.mu.Unlock()
	if broker == nil {
		return CopiedSecret{}, errs.New(ErrNoLoginSecret, nil)
	}
	expand := func(v string) string { return strings.TrimSpace(environments.Expand(v, values)) }
	t := plugins.Target{Name: e.Name, Hostname: expand(ref.Address), User: expand(ref.User), Safe: expand(ref.Safe), Purpose: "login of environment " + e.Name}
	ht := resolvedHost{name: e.Name, target: sshengine.Target{Host: t.Hostname, User: t.User}}
	cred, err := s.sessions.askBroker(ctx, broker, ref.Source, t, ht, "environment:"+e.ID)
	if err != nil {
		return CopiedSecret{}, err
	}
	if cred.Password == "" {
		return CopiedSecret{}, errs.New(ErrLoginSecretEmpty, errs.Params{"user": firstOf(cred.User, t.User)})
	}
	if seconds <= 0 {
		seconds = clipboardSeconds
	}
	seconds = min(max(seconds, clipboardSecondsMin), clipboardSecondsMax)
	if err := copyForAWhile(cred.Password, time.Duration(seconds)*time.Second); err != nil {
		return CopiedSecret{}, err
	}
	return CopiedSecret{User: firstOf(cred.User, t.User), Seconds: seconds}, nil
}

// copyForAWhile puts text on the clipboard and clears it after d if it is
// still there; only a hash is kept meanwhile.
func copyForAWhile(text string, d time.Duration) error {
	app := application.Get()
	if app == nil || app.Clipboard == nil || !app.Clipboard.SetText(text) {
		return errs.New(ErrClipboard, nil)
	}
	sum := sha256.Sum256([]byte(text))
	time.AfterFunc(d, func() {
		if now, ok := app.Clipboard.Text(); ok && sha256.Sum256([]byte(now)) == sum {
			app.Clipboard.SetText("")
		}
	})
	return nil
}

// firstOf returns the first non-empty value.
func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// GcloudProfiles lists the gcloud profiles already used, for suggestions.
func (s *EnvironmentService) GcloudProfiles() []string {
	out := []string{}
	if s.dataDir == "" {
		return out
	}
	entries, _ := os.ReadDir(filepath.Join(s.dataDir, "gcloud"))
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// gcloudDir is the configuration folder of a gcloud profile, created on
// first use; "" for gcloud's own configuration.
func gcloudDir(dataDir, profile string) (string, error) {
	p := environments.GcloudProfile(profile)
	if p == "" || dataDir == "" {
		return "", nil
	}
	dir := filepath.Join(dataDir, "gcloud", p)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}
