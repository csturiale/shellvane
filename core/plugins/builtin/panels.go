package builtin

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

// Names that may be put in a command: no spaces, quotes or shell syntax.
var (
	containerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)
	unitName      = regexp.MustCompile(`^[A-Za-z0-9@_.:\\-]{1,255}$`)
)

// run executes a command on the host; a non-zero exit is an error with
// the output's last line as detail.
func run(ctx context.Context, exec plugins.Exec, command, tool string) (string, error) {
	if exec == nil {
		return "", errs.New(plugins.ErrUnsupported, nil)
	}
	out, code, err := exec(ctx, command)
	if err != nil {
		return "", errs.Wrap(err, plugins.ErrCommand, errs.Params{"command": tool, "detail": err.Error()})
	}
	if code != 0 {
		detail := lastLine(out)
		if code == 127 {
			detail = tool + ": not found"
		}
		return "", errs.New(plugins.ErrCommand, errs.Params{"command": tool, "detail": detail})
	}
	return out, nil
}

func badItem(item string) error {
	return errs.New(plugins.ErrSetting, errs.Params{"key": item})
}

// Docker shows the containers of the host, with logs, a shell and
// start/stop/restart.
type Docker struct{ plugins.Base }

// Manifest describes the plugin.
func (Docker) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "docker", Name: "Docker", Version: "1", Description: "plugins.docker.description", BuiltIn: true,
		Capabilities: []string{plugins.CapPanel, plugins.CapActions},
	}
}

// Panel lists the containers, running ones first.
func (Docker) Panel(ctx context.Context, _ map[string]string, _ plugins.Target, exec plugins.Exec) (plugins.Panel, error) {
	out, err := run(ctx, exec, `docker ps -a --format '{{json .}}'`, "docker")
	if err != nil {
		return plugins.Panel{}, err
	}
	type row struct{ Names, Image, State, Status, Ports string }
	var rows []row
	for _, line := range strings.Split(out, "\n") {
		var r row
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &r) == nil && r.Names != "" {
			rows = append(rows, r)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return (rows[i].State == "running") && rows[j].State != "running" })
	t := plugins.Table{
		Title:   "plugins.docker.containers",
		Columns: []string{"plugins.docker.name", "plugins.docker.image", "plugins.docker.state", "plugins.docker.status", "plugins.docker.ports"},
		Actions: []plugins.Action{
			{ID: "logs", Label: "plugins.docker.logs"},
			{ID: "shell", Label: "plugins.docker.shell"},
			{ID: "start", Label: "plugins.docker.start"},
			{ID: "restart", Label: "plugins.docker.restart", Confirm: true},
			{ID: "stop", Label: "plugins.docker.stop", Confirm: true},
		},
	}
	for _, r := range rows {
		t.Rows = append(t.Rows, []string{r.Names, r.Image, r.State, r.Status, r.Ports})
	}
	return plugins.Panel{Title: "Docker", Tables: []plugins.Table{t}}, nil
}

// Run acts on a container.
func (Docker) Run(ctx context.Context, _ map[string]string, _ plugins.Target, action, item string, exec plugins.Exec) (plugins.Result, error) {
	if !containerName.MatchString(item) {
		return plugins.Result{}, badItem(item)
	}
	switch action {
	case "logs":
		return plugins.Result{Command: "docker logs -f --tail 200 " + item}, nil
	case "shell":
		return plugins.Result{Command: "docker exec -it " + item + " sh -c 'command -v bash >/dev/null && exec bash || exec sh'"}, nil
	case "start", "stop", "restart":
		out, err := run(ctx, exec, "docker "+action+" "+item, "docker")
		return plugins.Result{Output: strings.TrimSpace(out)}, err
	}
	return plugins.Result{}, errs.New(plugins.ErrUnsupported, nil)
}

// Systemd shows the failed and running services, with status, journal
// and restart (typed in a terminal, where sudo can ask its password).
type Systemd struct{ plugins.Base }

// Manifest describes the plugin.
func (Systemd) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "systemd", Name: "systemd", Version: "1", Description: "plugins.systemd.description", BuiltIn: true,
		Capabilities: []string{plugins.CapPanel, plugins.CapActions},
	}
}

// Panel lists the failed services, then the running ones.
func (Systemd) Panel(ctx context.Context, _ map[string]string, _ plugins.Target, exec plugins.Exec) (plugins.Panel, error) {
	out, err := run(ctx, exec, "systemctl list-units --type=service --all --no-legend --no-pager --plain", "systemctl")
	if err != nil {
		return plugins.Panel{}, err
	}
	actions := []plugins.Action{
		{ID: "status", Label: "plugins.systemd.status"},
		{ID: "journal", Label: "plugins.systemd.journal"},
		{ID: "restart", Label: "plugins.systemd.restart", Confirm: true},
		{ID: "stop", Label: "plugins.systemd.stop", Confirm: true},
		{ID: "start", Label: "plugins.systemd.start"},
	}
	columns := []string{"plugins.systemd.unit", "plugins.systemd.active", "plugins.systemd.sub", "plugins.systemd.descriptionColumn"}
	failed := plugins.Table{Title: "plugins.systemd.failed", Columns: columns, Actions: actions}
	running := plugins.Table{Title: "plugins.systemd.running", Columns: columns, Actions: actions}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		unit, active, sub := f[0], f[2], f[3]
		desc := strings.Join(f[4:], " ")
		r := []string{unit, active, sub, desc}
		switch {
		case active == "failed":
			failed.Rows = append(failed.Rows, r)
		case sub == "running":
			running.Rows = append(running.Rows, r)
		}
	}
	return plugins.Panel{Title: "systemd", Tables: []plugins.Table{failed, running}}, nil
}

// Run acts on a unit.
func (Systemd) Run(_ context.Context, _ map[string]string, _ plugins.Target, action, item string, _ plugins.Exec) (plugins.Result, error) {
	if !unitName.MatchString(item) {
		return plugins.Result{}, badItem(item)
	}
	switch action {
	case "status":
		return plugins.Result{Command: "systemctl status --no-pager -l " + item}, nil
	case "journal":
		return plugins.Result{Command: "journalctl -u " + item + " -n 200 -f"}, nil
	case "start", "stop", "restart":
		return plugins.Result{Command: "sudo systemctl " + action + " " + item + " && systemctl status --no-pager " + item}, nil
	}
	return plugins.Result{}, errs.New(plugins.ErrUnsupported, nil)
}

// Logs shows the host's recent errors from the journal (or syslog) and
// follows the log in a terminal.
type Logs struct{ plugins.Base }

// Manifest describes the plugin.
func (Logs) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "logs", Name: "Logs", Version: "1", Description: "plugins.logs.description", BuiltIn: true,
		Capabilities: []string{plugins.CapPanel, plugins.CapActions},
	}
}

const recentErrors = `if command -v journalctl >/dev/null 2>&1; then journalctl -p err -n 150 --no-pager -o short-iso 2>&1; ` +
	`elif [ -r /var/log/syslog ]; then grep -iE 'error|fail|crit' /var/log/syslog | tail -n 150; ` +
	`elif [ -r /var/log/messages ]; then grep -iE 'error|fail|crit' /var/log/messages | tail -n 150; fi`

// Panel shows the recent errors.
func (Logs) Panel(ctx context.Context, _ map[string]string, _ plugins.Target, exec plugins.Exec) (plugins.Panel, error) {
	out, err := run(ctx, exec, recentErrors, "journalctl")
	if err != nil {
		return plugins.Panel{}, err
	}
	return plugins.Panel{Title: "plugins.logs.recent", Text: out}, nil
}

// Actions offers following the log.
func (Logs) Actions(context.Context, map[string]string, plugins.Target) ([]plugins.Action, error) {
	return []plugins.Action{{ID: "follow", Label: "plugins.logs.follow"}}, nil
}

// Run follows the log in a terminal.
func (Logs) Run(_ context.Context, _ map[string]string, _ plugins.Target, action, _ string, _ plugins.Exec) (plugins.Result, error) {
	if action != "follow" {
		return plugins.Result{}, errs.New(plugins.ErrUnsupported, nil)
	}
	return plugins.Result{Command: "journalctl -f -n 100 2>/dev/null || tail -F /var/log/syslog /var/log/messages 2>/dev/null"}, nil
}
