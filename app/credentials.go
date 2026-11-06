package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/sshengine"
)

// ErrCredentialCommand reports a failing password or certificate command.
const ErrCredentialCommand errs.Code = "credentials.commandFailed"

// ErrPreConnect is returned when the command run before connecting fails.
const ErrPreConnect errs.Code = "credentials.preConnectFailed"

// credentialCommandTimeout bounds password-manager and signer commands,
// which may wait for the user (e.g. a biometric prompt).
const credentialCommandTimeout = 2 * time.Minute

// TOTPScope is the vault scope of a host's TOTP seed.
func TOTPScope(hostID string) string { return "totp:" + hostID }

// targetEnv describes the destination to a credential command.
func targetEnv(t sshengine.Target) map[string]string {
	return map[string]string{"SHELLVANE_HOST": t.Host, "SHELLVANE_PORT": strconv.Itoa(t.Port), "SHELLVANE_USER": t.User}
}

// runCredentialCommand runs a user-configured command through the shell and
// returns its trimmed stdout. env adds variables; SHELLVANE_PUBLIC_KEY is also
// written to a file named by SHELLVANE_PUBLIC_KEY_FILE.
func runCredentialCommand(ctx context.Context, command string, env map[string]string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, credentialCommandTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", command)
	}
	cmd.Env = os.Environ()
	if pub, ok := env["SHELLVANE_PUBLIC_KEY"]; ok {
		dir, err := os.MkdirTemp("", "shellvane-key-")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(dir)
		file := filepath.Join(dir, "id.pub")
		if err := os.WriteFile(file, []byte(pub+"\n"), 0o600); err != nil {
			return "", err
		}
		cmd.Env = append(cmd.Env, "SHELLVANE_PUBLIC_KEY_FILE="+file)
	}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.WaitDelay = 2 * time.Second // a background child must not hold the command up
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrWaitDelay) {
		err = nil
	}
	if err != nil {
		detail := ""
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			detail = lastLine(string(exit.Stderr))
		}
		return "", errs.Wrap(err, ErrCredentialCommand, errs.Params{"detail": detail})
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

// serveAgent forwards the user's agent on a connection, once, asking
// before every signature. Prompts go to the session that enabled it.
func (s *SessionService) serveAgent(client *ssh.Client, sessionID string) {
	s.mu.Lock()
	if s.forwarded == nil {
		s.forwarded = map[*ssh.Client]bool{}
	}
	done := s.forwarded[client]
	s.forwarded[client] = true
	s.mu.Unlock()
	if done {
		return
	}
	ag, closer, _, err := sshengine.OpenAgent()
	if err != nil {
		return
	}
	confirming := sshengine.NewConfirmingAgent(ag, func(r sshengine.SignRequest) sshengine.SignDecision {
		a, err := s.ask(context.Background(), Prompt{SessionID: sessionID, Kind: "agentSign", Sign: &r})
		if err != nil {
			return sshengine.SignRefuse
		}
		switch a.Decision {
		case "session":
			return sshengine.SignSession
		case "once":
			return sshengine.SignOnce
		}
		return sshengine.SignRefuse
	})
	if err := sshengine.ServeAgentForwarding(client, confirming); err != nil {
		closer.Close()
		return
	}
	go func() {
		client.Wait()
		closer.Close()
		s.mu.Lock()
		delete(s.forwarded, client)
		s.mu.Unlock()
	}()
}

// x11For returns the X11 forwarder of a connection: one per connection,
// since its X channels arrive on the connection, not on a session.
func (s *SessionService) x11For(client *ssh.Client) *sshengine.X11 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.x11 == nil {
		s.x11 = map[*ssh.Client]*sshengine.X11{}
	}
	x, ok := s.x11[client]
	if !ok {
		x = sshengine.NewX11(os.Getenv("DISPLAY"))
		s.x11[client] = x
		go func() {
			client.Wait()
			s.mu.Lock()
			delete(s.x11, client)
			s.mu.Unlock()
		}()
	}
	return x
}

// preConnect runs the host's command before connecting, with the host's
// public key when it has one (EC2 Instance Connect pushes it for 60 s).
func preConnect(ctx context.Context, ht resolvedHost, identity ssh.Signer) error {
	env := targetEnv(ht.target)
	env["SHELLVANE_HOST_NAME"] = ht.name
	if identity != nil {
		env["SHELLVANE_PUBLIC_KEY"] = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(identity.PublicKey())))
	}
	if _, err := runCredentialCommand(ctx, ht.preConnectCommand, env); err != nil {
		return errs.Wrap(err, ErrPreConnect, errs.Params{"name": ht.name, "detail": errs.ToPayload(err).Params["detail"]})
	}
	return nil
}

// lastLine is the last non-empty line of a command's error output.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
