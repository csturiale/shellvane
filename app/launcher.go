package app

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/launch"
	"github.com/csturiale/shellvane/core/localshell"
	"github.com/csturiale/shellvane/core/tunnel"
)

// Clients tells which external clients are installed.
type Clients struct {
	Mosh bool `json:"mosh"`
	RDP  bool `json:"rdp"`
	VNC  bool `json:"vnc"`
}

// LaunchService opens hosts with external clients: Mosh in a pane, remote
// desktop and VNC viewers in their own windows.
type LaunchService struct {
	sessions *SessionService
	tunnels  *TunnelService
}

// NewLaunchService returns a launcher.
func NewLaunchService(sessions *SessionService, tunnels *TunnelService) *LaunchService {
	return &LaunchService{sessions: sessions, tunnels: tunnels}
}

// Clients reports the installed clients.
func (s *LaunchService) Clients() Clients {
	_, err := exec.LookPath("mosh")
	return Clients{Mosh: err == nil, RDP: launch.Available(launch.RDP), VNC: launch.Available(launch.VNC)}
}

// OpenMosh opens a pane running mosh to a saved host, with the host's
// port, user, jump hosts and ProxyCommand passed to its ssh, and
// Shellvane's agent when it runs.
func (s *LaunchService) OpenMosh(ctx context.Context, hostID string, cols, rows int) (SessionInfo, error) {
	path, err := exec.LookPath("mosh")
	if err != nil {
		return SessionInfo{}, errs.New(launch.ErrNoClient, errs.Params{"kind": "mosh"})
	}
	args, err := SSHArgs(ctx, s.sessions.hosts, hostID)
	if err != nil {
		return SessionInfo{}, err
	}
	dest := args[len(args)-1]
	quoted := []string{"ssh"}
	for _, a := range args[:len(args)-1] {
		quoted = append(quoted, shellQuote(a))
	}
	h, err := s.sessions.hosts.Host(ctx, hostID)
	if err != nil {
		return SessionInfo{}, err
	}
	program := localshell.Program{ID: "mosh", Name: "mosh " + h.Name, Path: path, Args: []string{"--ssh=" + strings.Join(quoted, " "), "--", dest}}
	var env []string
	if sock := agentPath(s.sessions.dataDir); fileExists(sock) {
		env = append(env, "SSH_AUTH_SOCK="+sock)
	}
	info, err := s.sessions.OpenLocal(ctx, cols, rows, LocalOptions{program: &program, env: env})
	if err != nil {
		return info, err
	}
	// The pane shows the remote host, not this computer.
	if ht, err := s.sessions.hostTarget(ctx, hostID); err == nil {
		info.Target = ht.target
		info.Group, info.Color = ht.label.Path, ht.label.Color
		info.Local = false
	}
	return info, nil
}

func fileExists(p string) bool {
	if isPipe(p) {
		return true
	}
	_, err := os.Stat(p)
	return err == nil
}

// RemoteDesktop opens the host's remote desktop (RDP or VNC, from its
// settings) with the system's client. Through SSH, a temporary forward to
// the port on the host is opened first and the client connects to it.
func (s *LaunchService) RemoteDesktop(ctx context.Context, hostID string) error {
	h, err := s.sessions.hosts.Host(ctx, hostID)
	if err != nil {
		return err
	}
	res, err := s.sessions.hosts.Resolve(ctx, h)
	if err != nil {
		return err
	}
	kind := deref(res.Settings.RemoteDesktop)
	if kind != launch.RDP && kind != launch.VNC {
		kind = launch.RDP
	}
	port := launch.DefaultPort(kind)
	if res.Settings.RemoteDesktopPort != nil && *res.Settings.RemoteDesktopPort > 0 {
		port = *res.Settings.RemoteDesktopPort
	}
	host := h.Hostname
	if res.Settings.RemoteDesktopViaSSH != nil && *res.Settings.RemoteDesktopViaSSH {
		p, err := s.tunnels.Forward(ctx, hostID, tunnel.RemotePort{Port: port, Addr: "127.0.0.1"})
		if err != nil {
			return err
		}
		addr, err := s.tunnels.waitRunning(ctx, p.ID)
		if err != nil {
			return err
		}
		host, port = "127.0.0.1", addrPort(addr)
	}
	c, err := launch.Find(kind, host, port)
	if err != nil {
		return err
	}
	return launch.Start(c)
}
