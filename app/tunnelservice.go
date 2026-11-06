package app

import (
	"context"
	"errors"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/tunnel"
)

// EventTunnelState is emitted on state changes and, while running, once a
// second with fresh statistics.
const EventTunnelState = "tunnel:state"

// Tunnel states.
const (
	TunnelStopped  = "stopped"
	TunnelStarting = "starting"
	TunnelRunning  = "running"
	TunnelFailed   = "failed"
	// TunnelReconnecting waits before opening the tunnel again after its
	// connection dropped.
	TunnelReconnecting = "reconnecting"
)

// TunnelStatus is the live state of a profile.
type TunnelStatus struct {
	ID    string        `json:"id"`
	State string        `json:"state"`
	Addr  string        `json:"addr"`
	Stats tunnel.Stats  `json:"stats"`
	Error *errs.Payload `json:"error,omitempty"`
	// Attempt counts reconnections while reconnecting.
	Attempt int `json:"attempt"`
}

// TunnelView is a profile with its status.
type TunnelView struct {
	Profile tunnel.Profile `json:"profile"`
	Status  TunnelStatus   `json:"status"`
	// Temporary forwards are not saved and end when stopped.
	Temporary bool `json:"temporary"`
}

// TunnelService manages saved tunnels and runs them over pooled connections.
type TunnelService struct {
	repo     *tunnel.Repo
	sessions *SessionService
	emit     Emitter

	mu        sync.Mutex
	running   map[string]*activeTunnel
	temporary map[string]tunnel.Profile // forwards of detected ports
}

type activeTunnel struct {
	cancel context.CancelFunc
	status TunnelStatus
	run    *tunnel.Running
}

// NewTunnelService returns a service; connections come from sessions' pool.
func NewTunnelService(repo *tunnel.Repo, sessions *SessionService, emit Emitter) *TunnelService {
	return &TunnelService{repo: repo, sessions: sessions, emit: emit, running: map[string]*activeTunnel{}, temporary: map[string]tunnel.Profile{}}
}

// List returns every profile with its status.
func (s *TunnelService) List(ctx context.Context) ([]TunnelView, error) {
	profiles, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TunnelView, len(profiles))
	for i, p := range profiles {
		out[i] = TunnelView{Profile: p, Status: s.status(p.ID)}
	}
	s.mu.Lock()
	var temp []tunnel.Profile
	for _, p := range s.temporary {
		temp = append(temp, p)
	}
	s.mu.Unlock()
	sort.Slice(temp, func(i, j int) bool { return temp[i].Name < temp[j].Name })
	for _, p := range temp {
		out = append(out, TunnelView{Profile: p, Status: s.status(p.ID), Temporary: true})
	}
	return out, nil
}

// Create saves a profile.
func (s *TunnelService) Create(ctx context.Context, in tunnel.Input) (tunnel.Profile, error) {
	return s.repo.Create(ctx, in)
}

// Update edits a profile; a running tunnel keeps its old settings until restarted.
func (s *TunnelService) Update(ctx context.Context, id string, in tunnel.Input) (tunnel.Profile, error) {
	return s.repo.Update(ctx, id, in)
}

// Delete stops and removes a profile.
func (s *TunnelService) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	_, temporary := s.temporary[id]
	s.mu.Unlock()
	s.Stop(id)
	if temporary {
		return nil
	}
	return s.repo.Delete(ctx, id)
}

// ListeningPorts returns the TCP ports listening on a host, read with ss,
// netstat or /proc over the host's (pooled) connection.
func (s *TunnelService) ListeningPorts(ctx context.Context, hostID string) ([]tunnel.RemotePort, error) {
	ht, err := s.sessions.hostTarget(ctx, hostID)
	if err != nil {
		return nil, err
	}
	client, release, err := s.sessions.connect(ctx, ht, "ports:"+hostID)
	if err != nil {
		return nil, err
	}
	defer release()
	sess, err := client.NewSession()
	if err != nil {
		return nil, errs.Wrap(err, ErrPortsFailed, nil)
	}
	defer sess.Close()
	out, err := sess.Output(tunnel.ListeningScript)
	var exit *ssh.ExitError
	if err != nil && !(errors.As(err, &exit) && len(out) > 0) {
		return nil, errs.Wrap(err, ErrPortsFailed, nil)
	}
	return tunnel.ParseListening(string(out)), nil
}

// ErrPortsFailed reports that the listening ports could not be read.
const ErrPortsFailed errs.Code = "tunnel.portsFailed"

// Forward opens a temporary local forward to a port listening on a host,
// on the same local port when it is free (any free port otherwise). It is
// not saved and ends when stopped.
func (s *TunnelService) Forward(ctx context.Context, hostID string, remote tunnel.RemotePort) (tunnel.Profile, error) {
	h, err := s.sessions.hosts.Host(ctx, hostID)
	if err != nil {
		return tunnel.Profile{}, err
	}
	s.mu.Lock()
	for _, p := range s.temporary {
		if p.HostID == hostID && p.DestPort == remote.Port {
			s.mu.Unlock()
			return p, nil // already forwarded
		}
	}
	s.mu.Unlock()
	local := remote.Port
	if ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(local))); err != nil {
		local = 0
	} else {
		ln.Close()
	}
	p := tunnel.Profile{
		ID: "tmp-" + uuid.NewString(), HostID: hostID, Name: h.Name + ":" + strconv.Itoa(remote.Port),
		Spec: tunnel.Spec{Kind: tunnel.Local, BindAddr: "127.0.0.1", BindPort: local, DestHost: tunnel.DestHost(remote.Addr), DestPort: remote.Port},
	}
	if err := p.Spec.Validate(); err != nil {
		return tunnel.Profile{}, err
	}
	s.mu.Lock()
	s.temporary[p.ID] = p
	s.mu.Unlock()
	if err := s.Start(ctx, p.ID); err != nil {
		s.Stop(p.ID)
		return tunnel.Profile{}, err
	}
	return p, nil
}

// Start connects to the profile's host and opens the tunnel. Progress and
// errors arrive as events.
func (s *TunnelService) Start(ctx context.Context, id string) error {
	s.mu.Lock()
	p, temporary := s.temporary[id]
	s.mu.Unlock()
	if !temporary {
		var err error
		if p, err = s.repo.Get(ctx, id); err != nil {
			return err
		}
	}
	ht, err := s.sessions.hostTarget(ctx, p.HostID)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if a, ok := s.running[id]; ok && a.status.State != TunnelFailed {
		s.mu.Unlock()
		return nil // already starting or running
	}
	runCtx, cancel := context.WithCancel(context.Background())
	a := &activeTunnel{cancel: cancel, status: TunnelStatus{ID: id, State: TunnelStarting}}
	s.running[id] = a
	s.mu.Unlock()
	s.emit(EventTunnelState, a.status)

	go s.run(runCtx, a, p, ht)
	return nil
}

// run keeps the tunnel open: when the SSH connection drops it reconnects
// after 1, 2, 4… seconds (at most 30) until it works again or the tunnel
// is stopped. Failures another attempt cannot fix end it.
func (s *TunnelService) run(ctx context.Context, a *activeTunnel, p tunnel.Profile, ht resolvedHost) {
	for attempt := 0; ; attempt++ {
		ran, err := s.runOnce(ctx, a, p, ht)
		if ctx.Err() != nil {
			return
		}
		if ran {
			attempt = 0 // it worked: start counting again
		}
		if !retryable(err) || (!ran && attempt == 0 && errs.CodeOf(err) != ErrConnectionLost) {
			s.fail(a, err)
			return
		}
		delay := tunnelRetryMin << min(attempt, 5)
		if delay > tunnelRetryMax {
			delay = tunnelRetryMax
		}
		pl := errs.ToPayload(errs.New(ErrConnectionLost, nil))
		s.update(a, func(st *TunnelStatus) {
			st.State, st.Error, st.Attempt = TunnelReconnecting, &pl, attempt+1
		}, nil)
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return
		}
	}
}

// Reconnection pauses of tunnels.
const (
	tunnelRetryMin = time.Second
	tunnelRetryMax = 30 * time.Second
)

// retryable tells failures that may go away (network) from those that
// will not (authentication, host key, address in use).
func retryable(err error) bool {
	switch errs.CodeOf(err) {
	case "ssh.authFailed", "ssh.hostKeyChanged", "ssh.hostKeyRejected", "ssh.cancelled", "ssh.passphraseFailed",
		"ssh.jumpLoop", tunnel.ErrBindFailed, tunnel.ErrInvalid, "hosts.notFound":
		return false
	}
	return err != nil
}

// runOnce connects, opens the tunnel and serves it until the connection
// drops; ran tells whether it was running at some point.
func (s *TunnelService) runOnce(ctx context.Context, a *activeTunnel, p tunnel.Profile, ht resolvedHost) (ran bool, err error) {
	client, release, err := s.sessions.connect(ctx, ht, "tunnel:"+p.ID)
	if err != nil {
		return false, err
	}
	defer release()
	run, err := tunnel.Start(ctx, p.Spec, client)
	if err != nil {
		return false, err
	}
	s.update(a, func(st *TunnelStatus) { st.State, st.Addr, st.Error, st.Attempt = TunnelRunning, run.Addr, nil, 0 }, run)

	// Stop with the tunnel, the profile, or the SSH connection.
	dropped := make(chan struct{})
	go func() { client.Wait(); close(dropped) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			run.Stop()
			return true, ctx.Err()
		case err := <-run.Done():
			if err != nil {
				return true, errs.Wrap(err, tunnel.ErrBindFailed, errs.Params{"addr": run.Addr})
			}
			return true, nil
		case <-dropped:
			run.Stop()
			return true, errs.New(ErrConnectionLost, nil)
		case <-tick.C:
			s.update(a, func(st *TunnelStatus) { st.Stats = run.Stats() }, run)
		}
	}
}

// ErrConnectionLost reports that the SSH connection of a tunnel dropped.
const ErrConnectionLost errs.Code = "tunnel.connectionLost"

// Stop closes a running tunnel.
func (s *TunnelService) Stop(id string) {
	s.mu.Lock()
	a, ok := s.running[id]
	delete(s.running, id)
	delete(s.temporary, id)
	s.mu.Unlock()
	if ok {
		a.cancel()
		s.emit(EventTunnelState, TunnelStatus{ID: id, State: TunnelStopped})
	}
}

// StartAutostart starts the profiles marked to open with the application.
func (s *TunnelService) StartAutostart(ctx context.Context) error {
	profiles, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	for _, p := range profiles {
		if p.Autostart {
			if err := s.Start(ctx, p.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *TunnelService) status(id string) TunnelStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.running[id]; ok {
		return a.status
	}
	return TunnelStatus{ID: id, State: TunnelStopped}
}

func (s *TunnelService) update(a *activeTunnel, change func(*TunnelStatus), run *tunnel.Running) {
	s.mu.Lock()
	a.run = run
	change(&a.status)
	st := a.status
	s.mu.Unlock()
	s.emit(EventTunnelState, st)
}

func (s *TunnelService) fail(a *activeTunnel, err error) {
	p := errs.ToPayload(err)
	s.update(a, func(st *TunnelStatus) { st.State, st.Error = TunnelFailed, &p }, nil)
}

// waitRunning waits until a tunnel listens and returns its address.
func (s *TunnelService) waitRunning(ctx context.Context, id string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		st := s.status(id)
		switch st.State {
		case TunnelRunning:
			return st.Addr, nil
		case TunnelFailed, TunnelStopped:
			if st.Error != nil {
				return "", errs.New(st.Error.Code, errs.Params(st.Error.Params))
			}
			return "", errs.New(tunnel.ErrBindFailed, errs.Params{"addr": id})
		}
		select {
		case <-ctx.Done():
			return "", errs.Wrap(ctx.Err(), ErrConnectionLost, nil)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// addrPort returns the port of "host:port" (0 when there is none).
func addrPort(addr string) int {
	_, p, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(p)
	return n
}
