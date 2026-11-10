package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/automation"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/kube"
	"github.com/csturiale/shellvane/core/localshell"
	"github.com/csturiale/shellvane/core/netproxy"
	"github.com/csturiale/shellvane/core/shellenv"
	"github.com/csturiale/shellvane/core/sshengine"
)

// ErrNoLocalShell reports that no shell was found on this computer.
const ErrNoLocalShell errs.Code = "local.noShell"

// kubectlTimeout bounds kubectl calls (listing contexts, reading one).
const kubectlTimeout = 20 * time.Second

// terminalProcess is what a pane talks to: an SSH shell or a local one.
type terminalProcess interface {
	io.ReadWriter
	Resize(cols, rows int) error
	Wait() int
	Close() error
}

// KubeInfo is a pane's kubectl context. Live means it can be switched
// without reopening the pane.
type KubeInfo struct {
	Context   string `json:"context"`
	Namespace string `json:"namespace"`
	Live      bool   `json:"live"`
}

// kubeState is the pane's kubectl file: a path for local shells, a name
// under ~/.kube/shellvane on servers.
type kubeState struct {
	name string // remote
	file string // local
	sel  kube.Selection
}

// LocalOptions describe a local terminal.
type LocalOptions struct {
	// Program is a Programs ID; "" is the first one (the login shell).
	Program string `json:"program"`
	// Dir is the starting directory; "" is the home directory.
	Dir string `json:"dir"`
	// Persist names the pane (its kubectl file).
	Persist string          `json:"persist"`
	Kube    *kube.Selection `json:"kube,omitempty"`
	// Script is typed after the shell starts (e.g. a login command).
	Script []string `json:"script,omitempty"`

	// program runs instead of a shell (Mosh); env is added to it.
	program *localshell.Program
	env     []string
}

// EnableLocal sets where local panes keep their kubectl files. It is not a
// method so that Wails does not expose it as a binding.
func EnableLocal(s *SessionService, dataDir string) {
	s.dataDir = dataDir
}

// LocalShells lists the shells of this computer.
func (s *SessionService) LocalShells() []localshell.Program {
	list := localshell.Programs()
	if list == nil {
		list = []localshell.Program{}
	}
	return list
}

// OpenLocal starts a shell of this computer. Every local pane gets its own
// KUBECONFIG file, so its kubectl context can be switched at any time.
func (s *SessionService) OpenLocal(ctx context.Context, cols, rows int, opts LocalOptions) (SessionInfo, error) {
	programs := localshell.Programs()
	if len(programs) == 0 {
		return SessionInfo{}, errs.New(ErrNoLocalShell, nil)
	}
	program := programs[0]
	for _, p := range programs {
		if p.ID == opts.Program {
			program = p
		}
	}
	if opts.program != nil {
		program = *opts.program
	}
	name := opts.Persist
	if name == "" {
		name = uuid.NewString()
	}
	sel := kube.Selection{}
	if opts.Kube != nil {
		sel = *opts.Kube
	}
	file := filepath.Join(s.kubeDir(), kube.SafeName(name)+".yaml")
	env := append([]string(nil), opts.env...)
	if err := s.writeLocalKube(ctx, file, sel); err == nil {
		env = append(env, "KUBECONFIG="+kube.Env(file, kube.Original()))
	} else {
		file = ""
	}

	sessCtx, cancel := context.WithCancel(context.Background())
	sess := &session{id: uuid.NewString(), ctx: sessCtx, cancel: cancel, ready: make(chan struct{}), flow: newFlowGate(),
		cols: cols, rows: rows, local: true, kube: kubeState{file: file, sel: sel}, capture: &capture{name: program.Name}}
	highlights, active := splitRules(s.loadRules(ctx, resolvedHost{}))
	sess.auto = &automationState{
		host:      program.Name,
		matcher:   automation.NewMatcher(active),
		responses: map[string][]time.Time{},
		watchers:  map[int]func([]byte){},
	}
	s.mu.Lock()
	s.sessions[sess.id] = sess
	s.mu.Unlock()

	s.emit(EventSessionState, SessionState{ID: sess.id, State: StateConnecting})
	go func() {
		defer close(sess.ready)
		sh, err := localshell.Start(localshell.Options{Program: program, Dir: opts.Dir, Env: env, Cols: cols, Rows: rows})
		if err != nil {
			sess.err = err
			p := errs.ToPayload(err)
			s.emit(EventSessionState, SessionState{ID: sess.id, State: StateFailed, Error: &p})
			s.forget(sess.id)
			return
		}
		sess.shell = sh
		s.emit(EventSessionState, SessionState{ID: sess.id, State: StateConnected})
		if len(opts.Script) > 0 {
			go s.runScript(sess, opts.Script)
		}
	}()

	host, _ := os.Hostname()
	who := "me"
	if u, err := user.Current(); err == nil {
		who = filepath.Base(u.Username) // Windows: DOMAIN\user
	}
	info := SessionInfo{
		ID: sess.id, Title: program.Name, Target: sshengine.Target{User: who, Host: host},
		Group: []string{}, Via: []string{}, Highlights: highlights, Local: true,
	}
	if file != "" {
		info.Kube = &KubeInfo{Context: sel.Context, Namespace: sel.Namespace, Live: true}
	}
	return info, nil
}

// KubeContexts lists the kubectl contexts where the pane runs: this
// computer or the server.
func (s *SessionService) KubeContexts(ctx context.Context, id string) ([]string, error) {
	sess, err := s.get(id)
	if err != nil {
		return nil, err
	}
	out, err := s.kubectl(ctx, sess, "config", "get-contexts", "-o", "name")
	if err != nil {
		return nil, err
	}
	return kube.ParseContexts(out), nil
}

// SetKube switches the pane's kubectl context and namespace by rewriting
// its file; programs started afterwards use the new context.
func (s *SessionService) SetKube(ctx context.Context, id string, sel kube.Selection) error {
	sess, err := s.get(id)
	if err != nil {
		return err
	}
	switch {
	case sess.local && sess.kube.file != "":
		err = s.writeLocalKube(ctx, sess.kube.file, sel)
	case !sess.local && sess.kube.name != "" && sess.client != nil:
		_, err = runRemote(ctx, sess, kube.RemoteWrite(sess.kube.name, sel))
	default:
		return errs.New(kube.ErrOverlayDisabled, nil)
	}
	if err == nil {
		s.withSession(id, func(sess *session) { sess.kube.sel = sel })
	}
	return err
}

func (s *SessionService) kubeDir() string {
	dir := s.dataDir
	if dir == "" {
		dir, _ = os.UserConfigDir()
		dir = filepath.Join(dir, "Shellvane")
	}
	return filepath.Join(dir, "kube")
}

// writeLocalKube writes a local pane's file; a namespace needs the
// context's cluster and user from the user's kubeconfig.
func (s *SessionService) writeLocalKube(ctx context.Context, file string, sel kube.Selection) error {
	var def kube.ContextDef
	if (sel.Namespace != "" || sel.Route != nil) && sel.Context != "" {
		var err error
		def, err = kube.Lookup(ctx, localKubectl, sel.Context)
		if err == nil && sel.Route != nil {
			def, err = kube.LookupCluster(ctx, localKubectl, def)
		}
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	return os.WriteFile(file, kube.Overlay(sel, def), 0o600)
}

// kubectl runs kubectl where the session runs, with the user's own
// kubeconfig (never the pane's file).
func (s *SessionService) kubectl(ctx context.Context, sess *session, args ...string) ([]byte, error) {
	if sess.local {
		return localKubectl(ctx, args...)
	}
	if sess.client == nil {
		return nil, errs.New(ErrSessionNotFound, errs.Params{"id": sess.id})
	}
	cmd := "kubectl"
	for _, a := range args {
		cmd += " " + kube.ShellQuote(a)
	}
	return runRemote(ctx, sess, `KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}" `+cmd)
}

func localKubectl(ctx context.Context, args ...string) ([]byte, error) {
	// At startup, restored panes ask before the login shell's PATH (where
	// kubectl usually is) has been read.
	shellenv.Wait(ctx)
	ctx, cancel := context.WithTimeout(ctx, kubectlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Env = append(append(os.Environ(), netproxy.ChildEnv()...), "KUBECONFIG="+kube.Original())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errs.Wrap(err, kube.ErrNotInstalled, nil)
		}
		detail := firstLine(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, errs.Wrap(err, kube.ErrKubectl, errs.Params{"detail": detail})
	}
	return out, nil
}

// runRemote runs a command on the session's server, on its own channel.
func runRemote(ctx context.Context, sess *session, command string) ([]byte, error) {
	ch, err := sess.client.NewSession()
	if err != nil {
		return nil, errs.Wrap(err, kube.ErrKubectl, nil)
	}
	defer ch.Close()
	var stdout, stderr bytes.Buffer
	ch.Stdout, ch.Stderr = &stdout, &stderr
	done := make(chan error, 1)
	go func() { done <- ch.Run(command) }()
	select {
	case err = <-done:
	case <-time.After(kubectlTimeout):
		err = context.DeadlineExceeded
	case <-ctx.Done():
		err = ctx.Err()
	}
	if err != nil {
		detail := firstLine(stderr.String())
		if notFound(detail) {
			return nil, errs.Wrap(err, kube.ErrNotInstalled, nil)
		}
		return nil, errs.Wrap(err, kube.ErrKubectl, errs.Params{"detail": detail})
	}
	return stdout.Bytes(), nil
}

// notFound recognizes a shell's "command not found" for kubectl.
func notFound(detail string) bool {
	d := strings.ToLower(detail)
	return strings.Contains(d, "kubectl") && (strings.Contains(d, "not found") || strings.Contains(d, "no such file"))
}

func firstLine(s string) string {
	s = string(bytes.TrimSpace([]byte(s)))
	if i := bytes.IndexByte([]byte(s), '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// withSession changes a session under the service lock.
func (s *SessionService) withSession(id string, fn func(*session)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[id]; ok {
		fn(sess)
	}
}
