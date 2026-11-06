// Command shellvane-cli is the command-line companion of Shellvane. It reads
// the same database as the app and drives the running app through its
// control channel (connect, open, tunnels).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/csturiale/shellvane/app"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/i18n"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/tunnel"
	"github.com/csturiale/shellvane/core/version"
)

func main() {
	os.Exit(run(os.Args, os.Stdout, os.Stderr))
}

type cli struct {
	tr       *i18n.Translator
	cmd      string
	out, err io.Writer
	dbPath   string
}

func run(args []string, stdout, stderr io.Writer) int {
	c := &cli{tr: i18n.New(i18n.FromEnv()), cmd: filepath.Base(args[0]), out: stdout, err: stderr}
	if len(args) < 2 {
		fmt.Fprintln(stderr, c.t("cli.usage", nil))
		return 2
	}
	path, err := store.DefaultPath()
	if err != nil {
		return c.fail(err)
	}
	c.dbPath = path
	rest := args[2:]
	switch args[1] {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, c.t("cli.version", map[string]any{"Name": c.t("app.name", nil), "Version": version.Version}))
	case "help", "--help", "-h":
		fmt.Fprintln(stdout, c.t("cli.usage", nil))
	case "list", "ls":
		return c.list()
	case "connect", "open":
		if len(rest) == 0 {
			fmt.Fprintln(stderr, c.t("cli.usage", nil))
			return 2
		}
		if rest[0] == "--ssh" && len(rest) > 1 {
			return c.systemSSH(rest[1])
		}
		return c.control(app.ControlRequest{Cmd: "open", Target: strings.Join(rest, " ")})
	case "tunnel":
		return c.tunnel(rest)
	case "import":
		return c.importSSHConfig(rest)
	default:
		fmt.Fprintln(stderr, c.t("cli.unknownCommand", map[string]any{"Command": args[1]}))
		fmt.Fprintln(stderr, c.t("cli.usage", nil))
		return 2
	}
	return 0
}

func (c *cli) t(id string, data map[string]any) string {
	if data == nil {
		data = map[string]any{}
	}
	data["Command"] = c.cmd
	return c.tr.T(id, data)
}

// fail prints an error: coded errors are translated by their code.
func (c *cli) fail(err error) int {
	if code := errs.CodeOf(err); code != "" {
		var e *errs.Error
		params := map[string]any{}
		if errors.As(err, &e) {
			for k, v := range e.Params {
				params[k] = v
			}
		}
		msg := c.t("cli.errors."+string(code), params)
		if msg != "" && msg != "cli.errors."+string(code) {
			fmt.Fprintln(c.err, msg)
			return 1
		}
	}
	fmt.Fprintln(c.err, c.t("cli.error", map[string]any{"Error": err.Error()}))
	return 1
}

func (c *cli) open() (*hosts.Repo, func(), error) {
	db, err := store.Open(context.Background(), c.dbPath)
	if err != nil {
		return nil, nil, err
	}
	return hosts.New(db), func() { db.Close() }, nil
}

// list prints hosts as "group/path/name  user@host:port".
func (c *cli) list() int {
	repo, done, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	defer done()
	ctx := context.Background()
	inv, err := repo.List(ctx)
	if err != nil {
		return c.fail(err)
	}
	groups := map[string]hosts.Group{}
	for _, g := range inv.Groups {
		groups[g.ID] = g
	}
	path := func(id string) string {
		var parts []string
		for seen := 0; id != "" && seen < 64; seen++ {
			g := groups[id]
			parts = append([]string{g.Name}, parts...)
			id = g.ParentID
		}
		return strings.Join(parts, "/")
	}
	type row struct{ name, addr string }
	var rows []row
	for _, h := range inv.Hosts {
		res, err := repo.Resolve(ctx, h)
		if err != nil {
			return c.fail(err)
		}
		name := h.Name
		if p := path(h.GroupID); p != "" {
			name = p + "/" + name
		}
		addr := h.Hostname
		if res.Settings.Port != nil && *res.Settings.Port != 22 {
			addr += ":" + strconv.Itoa(*res.Settings.Port)
		}
		if res.Settings.User != nil && *res.Settings.User != "" {
			addr = *res.Settings.User + "@" + addr
		}
		rows = append(rows, row{name, addr})
	}
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].name) < strings.ToLower(rows[j].name) })
	width := 0
	for _, r := range rows {
		width = max(width, len(r.name))
	}
	for _, r := range rows {
		fmt.Fprintf(c.out, "%-*s  %s\n", width, r.name, r.addr)
	}
	return 0
}

// control sends a command to the running app.
func (c *cli) control(req app.ControlRequest) int {
	resp, err := app.SendControl(filepath.Dir(c.dbPath), req)
	if err != nil {
		return c.fail(err)
	}
	if !resp.OK && resp.Error != nil {
		return c.fail(errs.New(resp.Error.Code, errs.Params(resp.Error.Params)))
	}
	return 0
}

func (c *cli) tunnel(args []string) int {
	if len(args) == 0 || args[0] == "list" {
		resp, err := app.SendControl(filepath.Dir(c.dbPath), app.ControlRequest{Cmd: "tunnels"})
		if err != nil {
			return c.fail(err)
		}
		for _, t := range resp.Tunnels {
			fmt.Fprintf(c.out, "%-24s %-12s %s\n", t.Profile.Name, t.Status.State, t.Status.Addr)
		}
		return 0
	}
	if len(args) < 2 || (args[0] != "up" && args[0] != "down") {
		fmt.Fprintln(c.err, c.t("cli.usage", nil))
		return 2
	}
	cmd := "tunnelUp"
	if args[0] == "down" {
		cmd = "tunnelDown"
	}
	return c.control(app.ControlRequest{Cmd: cmd, Name: strings.Join(args[1:], " ")})
}

// importSSHConfig imports ~/.ssh/config (or the given file) into a group.
func (c *cli) importSSHConfig(args []string) int {
	if len(args) == 0 || args[0] != "ssh-config" {
		fmt.Fprintln(c.err, c.t("cli.usage", nil))
		return 2
	}
	path := ""
	if len(args) > 1 {
		path = args[1]
	} else {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".ssh", "config")
	}
	db, err := store.Open(context.Background(), c.dbPath)
	if err != nil {
		return c.fail(err)
	}
	defer db.Close()
	svc := app.NewImportService(db, hosts.New(db), tunnel.NewRepo(db), filepath.Join(filepath.Dir(c.dbPath), "backups"))
	res, err := svc.Preview("openssh", "file", path)
	if err != nil {
		return c.fail(err)
	}
	sum, err := svc.Import(context.Background(), app.ImportRequest{Hosts: res.Hosts, Group: c.t("cli.importGroup", nil)})
	if err != nil {
		return c.fail(err)
	}
	fmt.Fprintln(c.out, c.t("cli.imported", map[string]any{"Hosts": sum.Hosts, "Skipped": sum.Skipped, "Tunnels": sum.Tunnels, "Warnings": len(res.Warnings)}))
	return 0
}

// systemSSH runs the system's ssh for a saved host in this terminal, with
// its port, user, jump hosts and ProxyCommand, and Shellvane's agent when it
// runs (so vault keys work).
func (c *cli) systemSSH(target string) int {
	repo, done, err := c.open()
	if err != nil {
		return c.fail(err)
	}
	defer done()
	args, err := app.SSHArgs(context.Background(), repo, target)
	if err != nil {
		return c.fail(err)
	}
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, c.out, c.err
	cmd.Env = os.Environ()
	if sock := filepath.Join(filepath.Dir(c.dbPath), "agent.sock"); fileExists(sock) {
		cmd.Env = append(cmd.Env, "SSH_AUTH_SOCK="+sock)
	}
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		return c.fail(err)
	}
	return 0
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
