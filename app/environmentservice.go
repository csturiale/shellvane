package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/environments"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/shellenv"
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
// {{variables}} are expanded first.
func (s *EnvironmentService) CheckLogin(ctx context.Context, command string, values map[string]string) LoginCheck {
	command = strings.TrimSpace(environments.Expand(command, values))
	if command == "" {
		return LoginCheck{OK: true}
	}
	if detail, ok := runInUserShell(ctx, command); !ok {
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
func runInUserShell(ctx context.Context, command string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, environmentCommandTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, shellenv.UserShell(), "-ilc", command)
	}
	cmd.Env = append(os.Environ(), "SHELLVANE_ENVIRONMENT=1")
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
	}
	return ""
}
