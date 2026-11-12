package app

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/procedures"
	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/sshurl"
	"github.com/csturiale/shellvane/core/tunnel"
)

// EventOpen asks the UI to open a host, a quick connection or files.
const EventOpen = "app:open"

// Error codes of the control channel.
const (
	ErrNotRunning   errs.Code = "control.notRunning"
	ErrOpenNotFound errs.Code = "control.notFound"
	ErrBadCommand   errs.Code = "control.badCommand"
)

// OpenRequest is what the UI opens (payload of EventOpen).
type OpenRequest struct {
	Kind   string `json:"kind"` // "host", "target" or "files"
	HostID string `json:"hostId"`
	Name   string `json:"name"`
	Spec   string `json:"spec"` // user@host:port for "target"
	Path   string `json:"path"` // folder for "files"
}

// ControlRequest is a command sent by the CLI to the running app.
type ControlRequest struct {
	Cmd    string `json:"cmd"`    // "ping", "open", "tunnelUp", "tunnelDown", "tunnels"
	Target string `json:"target"` // host name, ID, user@host or ssh:// / sftp:// URL
	Name   string `json:"name"`   // tunnel name
}

// ControlResponse answers a ControlRequest.
type ControlResponse struct {
	OK      bool          `json:"ok"`
	Error   *errs.Payload `json:"error,omitempty"`
	Tunnels []TunnelView  `json:"tunnels,omitempty"`
}

// Control serves commands from the CLI and links opened while the app
// runs, on a local socket (named pipe on Windows) only the user can open.
type Control struct {
	hosts   *hosts.Repo
	tunnels *TunnelService
	emit    Emitter
	// Show brings the window to the front; set by main.
	Show func()
	// Procedure receives shellvane://procedure links (pages sent from the
	// browser); nil ignores them.
	Procedure func(link string)
	ln        net.Listener

	mu      sync.Mutex
	ready   bool          // the UI listens for EventOpen
	pending []OpenRequest // opens received before the UI was ready
}

// Ready is called by the UI once it listens for EventOpen; it returns
// what was asked before (a link that launched the app).
func (c *Control) Ready() []OpenRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ready = true
	out := c.pending
	c.pending = nil
	if out == nil {
		out = []OpenRequest{}
	}
	return out
}

// OpenAll opens the links among args (command line of a launch).
func (c *Control) OpenAll(ctx context.Context, args []string) {
	for _, a := range args {
		if procedures.IsLink(a) {
			if c.Procedure != nil {
				c.Show()
				c.Procedure(a)
			}
			continue
		}
		if sshurl.Is(a) {
			if err := c.Open(ctx, a); err != nil {
				log.Printf("open %s: %v", a, err)
			}
		}
	}
}

// NewControl returns a control channel; Listen starts it.
func NewControl(repo *hosts.Repo, tunnels *TunnelService, emit Emitter) *Control {
	return &Control{hosts: repo, tunnels: tunnels, emit: emit, Show: func() {}}
}

// Listen starts serving in dataDir.
func (c *Control) Listen(dataDir string) error {
	ln, err := listenLocal(controlPath(dataDir))
	if err != nil {
		return err
	}
	c.ln = ln
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go c.serve(conn)
		}
	}()
	return nil
}

// Close stops serving.
func (c *Control) Close() {
	if c.ln != nil {
		c.ln.Close()
	}
}

func (c *Control) serve(conn net.Conn) {
	defer conn.Close()
	var req ControlRequest
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
		return
	}
	resp := c.handle(context.Background(), req)
	json.NewEncoder(conn).Encode(resp)
}

func (c *Control) handle(ctx context.Context, req ControlRequest) ControlResponse {
	fail := func(err error) ControlResponse {
		p := errs.ToPayload(err)
		return ControlResponse{Error: &p}
	}
	switch req.Cmd {
	case "ping":
	case "open":
		if err := c.Open(ctx, req.Target); err != nil {
			return fail(err)
		}
	case "tunnels":
		list, err := c.tunnels.List(ctx)
		if err != nil {
			return fail(err)
		}
		return ControlResponse{OK: true, Tunnels: list}
	case "tunnelUp", "tunnelDown":
		list, err := c.tunnels.List(ctx)
		if err != nil {
			return fail(err)
		}
		for _, t := range list {
			if strings.EqualFold(t.Profile.Name, req.Name) || t.Profile.ID == req.Name {
				if req.Cmd == "tunnelUp" {
					if err := c.tunnels.Start(ctx, t.Profile.ID); err != nil {
						return fail(err)
					}
				} else {
					c.tunnels.Stop(t.Profile.ID)
				}
				return ControlResponse{OK: true}
			}
		}
		return fail(errs.New(tunnel.ErrNotFound, errs.Params{"id": req.Name}))
	default:
		return fail(errs.New(ErrBadCommand, errs.Params{"cmd": req.Cmd}))
	}
	return ControlResponse{OK: true}
}

// Open shows the window and opens a target: a saved host (by name or
// ID), an ssh:// or sftp:// link, or user@host[:port].
func (c *Control) Open(ctx context.Context, target string) error {
	req, err := ResolveOpen(ctx, c.hosts, target)
	if err != nil {
		return err
	}
	c.Show()
	c.mu.Lock()
	if !c.ready {
		c.pending = append(c.pending, req)
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()
	c.emit(EventOpen, req)
	return nil
}

// ResolveOpen turns a target into what the UI opens. Links and addresses
// use a saved host with the same address, port and user when one exists,
// so its credentials, jump hosts and options apply.
func ResolveOpen(ctx context.Context, repo *hosts.Repo, target string) (OpenRequest, error) {
	target = strings.TrimSpace(target)
	inv, err := repo.List(ctx)
	if err != nil {
		return OpenRequest{}, err
	}
	for _, h := range inv.Hosts {
		if h.ID == target || strings.EqualFold(h.Name, target) {
			return OpenRequest{Kind: "host", HostID: h.ID, Name: h.Name}, nil
		}
	}
	var link sshurl.Link
	if sshurl.Is(target) {
		if link, err = sshurl.Parse(target); err != nil {
			return OpenRequest{}, err
		}
	} else if t, err := sshengine.ParseTarget(target); err == nil && strings.ContainsAny(target, "@.:") {
		link = sshurl.Link{Scheme: "ssh", User: t.User, Host: t.Host, Port: t.Port}
		if link.Port == 22 {
			link.Port = 0
		}
	} else {
		return OpenRequest{}, errs.New(ErrOpenNotFound, errs.Params{"target": target})
	}
	for _, h := range inv.Hosts {
		if !strings.EqualFold(h.Hostname, link.Host) {
			continue
		}
		res, err := repo.Resolve(ctx, h)
		if err != nil {
			return OpenRequest{}, err
		}
		port := 22
		if res.Settings.Port != nil {
			port = *res.Settings.Port
		}
		want := link.Port
		if want == 0 {
			want = 22
		}
		if port != want || (link.User != "" && deref(res.Settings.User) != link.User) {
			continue
		}
		if link.Scheme == "sftp" {
			return OpenRequest{Kind: "files", HostID: h.ID, Name: h.Name, Path: link.Path}, nil
		}
		return OpenRequest{Kind: "host", HostID: h.ID, Name: h.Name}, nil
	}
	return OpenRequest{Kind: "target", Spec: link.Spec(), Name: link.Spec()}, nil
}

// SendControl sends a command to the running app in dataDir.
func SendControl(dataDir string, req ControlRequest) (ControlResponse, error) {
	conn, err := dialLocal(controlPath(dataDir))
	if err != nil {
		return ControlResponse{}, errs.Wrap(err, ErrNotRunning, nil)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return ControlResponse{}, err
	}
	var resp ControlResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return ControlResponse{}, errs.Wrap(err, ErrNotRunning, nil)
	}
	return resp, nil
}

// ControlService is the UI's side of the control channel.
type ControlService struct{ c *Control }

// NewControlService exposes c's pending opens to the UI.
func NewControlService(c *Control) *ControlService { return &ControlService{c: c} }

// Ready tells that the UI listens for EventOpen and returns the opens that
// arrived before (a link that launched the app).
func (s *ControlService) Ready() []OpenRequest { return s.c.Ready() }

// RestartEnv marks a process started by an update to replace this one.
const RestartEnv = "SHELLVANE_RESTARTED"

// WaitForPreviousInstance waits (up to timeout) until the instance being
// replaced has closed its control channel, so the single-instance check
// of the new one does not hand over to it.
func WaitForPreviousInstance(dataDir string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := dialLocal(controlPath(dataDir))
		if err != nil {
			return
		}
		conn.Close()
		time.Sleep(200 * time.Millisecond)
	}
}
