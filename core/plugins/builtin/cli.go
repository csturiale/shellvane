// Package builtin holds the plugins Shellvane ships: inventories, brokers,
// panels and actions built on the vendors' CLIs and APIs.
package builtin

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

// cliTimeout bounds a vendor CLI call (a cloud listing can be slow).
const cliTimeout = 90 * time.Second

// Runner runs a command on this computer and returns its stdout; tests
// replace it.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// System runs real commands.
func System(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, cliTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		detail := lastLine(stderr.String())
		var notFound *exec.Error
		if errors.As(err, &notFound) {
			detail = name + ": not found"
		}
		return nil, errs.Wrap(err, plugins.ErrCommand, errs.Params{"command": name, "detail": detail})
	}
	return out, nil
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// appendIf adds a flag and its value when the value is set.
func appendIf(args []string, flag, value string) []string {
	if v := strings.TrimSpace(value); v != "" {
		return append(args, flag, v)
	}
	return args
}

// expandHome turns a leading ~ into the home folder.
func expandHome(p string) string {
	p = strings.TrimSpace(p)
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func or(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// All returns the built-in plugins, running their tools on this computer.
func All() []plugins.Plugin {
	return []plugins.Plugin{
		AWS{Cmd: System}, GCP{Cmd: System}, Azure{Cmd: System},
		Kubernetes{Cmd: System}, Ansible{Cmd: System}, Terraform{Cmd: System},
		Proxmox{}, NetBox{},
		Docker{}, Systemd{}, Logs{},
		HashiCorpVault{}, CyberArk{}, Conjur{}, Teleport{Cmd: System}, Boundary{Cmd: System},
	}
}
