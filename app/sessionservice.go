package app

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/audit"
	"github.com/csturiale/shellvane/core/automation"
	"github.com/csturiale/shellvane/core/diagnostics"
	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/keys"
	"github.com/csturiale/shellvane/core/knownhosts"
	"github.com/csturiale/shellvane/core/kube"
	"github.com/csturiale/shellvane/core/otp"
	"github.com/csturiale/shellvane/core/pkcs11"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/vault"
)

// Events emitted to the frontend.
const (
	EventSessionState  = "session:state"
	EventSessionPrompt = "session:prompt"
)

// defaultKeepAlive applies to quick connections, which have no settings.
const defaultKeepAlive = 30 * time.Second

// ErrSessionNotFound is returned for unknown or closed session IDs.
const ErrSessionNotFound errs.Code = "session.notFound"

// Session states.
const (
	StateConnecting = "connecting"
	StateConnected  = "connected"
	StateClosed     = "closed"
	StateFailed     = "failed"
)

// SessionInfo identifies a new session.
// It carries what the UI needs to tell panes apart.
type SessionInfo struct {
	ID     string           `json:"id"`
	Title  string           `json:"title"`
	Target sshengine.Target `json:"target"`
	// Group is the path of the host's group from the root; empty for quick
	// connections. Color is the nearest group color.
	Group []string `json:"group"`
	Color string   `json:"color"`
	// TerminalTheme and FontSize override the app preference when set.
	TerminalTheme string `json:"terminalTheme"`
	FontSize      int    `json:"fontSize"`
	// Via lists the jump hosts, first hop first.
	Via []string `json:"via"`
	// Highlights color matching output in the terminal.
	Highlights []Highlight `json:"highlights"`
	// Multiplexer is "tmux" or "screen" when the shell runs in a named
	// session that a reconnection attaches to again.
	Multiplexer   string `json:"multiplexer"`
	AutoReconnect bool   `json:"autoReconnect"`
	// Local is a shell of this computer.
	Local bool `json:"local"`
	// Protocol is "telnet" or "serial" for panes that are not SSH; empty
	// for SSH and local shells.
	Protocol string `json:"protocol"`
	// Logging and Recording start with the session (host settings).
	Logging   bool `json:"logging"`
	Recording bool `json:"recording"`
	// Kube is the pane's kubectl context; nil when the pane has none and
	// cannot switch without reopening.
	Kube *KubeInfo `json:"kube,omitempty"`
}

// OpenOptions tune how a saved host's session starts.
type OpenOptions struct {
	// Persist names the multiplexer session; the same name reattaches to it
	// (after a disconnection or when a workspace is restored).
	Persist string `json:"persist"`
	// Resume skips the startup commands: they already ran in the
	// multiplexer session being reattached.
	Resume bool `json:"resume"`
	// Kube overrides the host's kubectl context for this pane.
	Kube *kube.Selection `json:"kube,omitempty"`
	// Script runs after the host's startup script (environments).
	Script []string `json:"script,omitempty"`
}

// SessionState is emitted whenever a session changes state.
type SessionState struct {
	ID       string        `json:"id"`
	State    string        `json:"state"`
	Error    *errs.Payload `json:"error,omitempty"`
	ExitCode int           `json:"exitCode"`
	// Lost is true when a connected session ended because the connection
	// dropped, not because the shell exited or the user closed it.
	Lost bool `json:"lost"`
}

// Prompt asks the user for something the connection needs.
type Prompt struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	// "password", "passphrase", "hostKey" or "vaultUnlock" (to use a saved
	// credential while the vault is locked).
	Kind    string                 `json:"kind"`
	Target  sshengine.Target       `json:"target"`
	Attempt int                    `json:"attempt"`
	HostKey *knownhosts.UnknownKey `json:"hostKey,omitempty"`
	Key     *sshengine.KeyInfo     `json:"key,omitempty"`
	// Challenge holds keyboard-interactive questions; Autofilled marks the
	// ones answered from the vault (their answers are not sent to the UI).
	Challenge  *sshengine.Challenge   `json:"challenge,omitempty"`
	Autofilled []bool                 `json:"autofilled,omitempty"`
	Sign       *sshengine.SignRequest `json:"sign,omitempty"`
	// CanSave is true when the answer can be saved in the (unlocked) vault.
	CanSave bool `json:"canSave"`
}

// PromptAnswer is the user's reply. Decision applies to host keys:
// "reject", "once" or "save".
type PromptAnswer struct {
	Value    string `json:"value"`
	Decision string `json:"decision"`
	Cancel   bool   `json:"cancel"`
	// Save stores the password or passphrase in the vault after a
	// successful login.
	Save bool `json:"save"`
	// Answers to a keyboard-interactive challenge, one per question.
	Answers []string `json:"answers"`
}

// StreamOpen is the first frame a terminal stream sends.
type StreamOpen struct {
	SessionID string `json:"sessionId"`
	// Replay asks for the recent output first (a terminal moved to a new
	// window).
	Replay bool `json:"replay"`
}

// Emitter sends an event to the frontend.
type Emitter func(name string, data any)

// SessionService opens SSH shells and pipes them through the "terminal" stream.
type SessionService struct {
	// unlockMu: the panes of an environment ask to open the vault one at a time.
	unlockMu sync.Mutex

	hosts *hosts.Repo
	known *knownhosts.Store
	vault *VaultService
	keys  *keys.Repo
	pool  *sshengine.Pool
	// broker gives logins from credential plugins (nil: none).
	broker Broker
	emit   Emitter
	rules  *automation.Repo
	audit  *audit.Log

	dataDir string // kubectl files of local panes, session logs and recordings

	handshakes *diagnostics.Log // recent connection attempts, for diagnostics

	mu        sync.Mutex
	sessions  map[string]*session
	forwarded map[*ssh.Client]bool // connections that already serve the agent
	x11       map[*ssh.Client]*sshengine.X11
	prompts   map[string]chan PromptAnswer

	openWindow func(url string)
	detached   map[string]detachedTab // tabs waiting for their new window
}

type session struct {
	id     string
	ctx    context.Context
	cancel context.CancelFunc
	ready  chan struct{} // closed when connected or failed
	shell  terminalProcess
	err    error
	local  bool
	client *ssh.Client // SSH sessions: runs kubectl on the server
	kube   kubeState

	flow    *flowGate
	auto    *automationState // nil when no rule applies and no script runs
	capture *capture

	sizeMu     sync.Mutex
	cols, rows int // latest size requested before the shell started

	out outputHub
	// handover is set while the terminal moves to another window: the
	// stream that ends then does not close the session.
	handover atomic.Bool
}

// NewSessionService returns a service; emit delivers events to the frontend.
func NewSessionService(repo *hosts.Repo, known *knownhosts.Store, vaultSvc *VaultService, keyRepo *keys.Repo, emit Emitter) *SessionService {
	return &SessionService{
		hosts:    repo,
		known:    known,
		vault:    vaultSvc,
		keys:     keyRepo,
		emit:     emit,
		pool:     sshengine.NewPool(),
		sessions: map[string]*session{},
		prompts:  map[string]chan PromptAnswer{},

		handshakes: diagnostics.NewLog(50),
	}
}

// OpenHost starts connecting to a saved host. Progress arrives as events.
func (s *SessionService) OpenHost(ctx context.Context, hostID string, cols, rows int, opts OpenOptions) (SessionInfo, error) {
	ht, err := s.hostTarget(ctx, hostID)
	if err != nil {
		return SessionInfo{}, err
	}
	sel := ht.kube
	if opts.Kube != nil {
		sel = *opts.Kube
	}
	var kubeSel *kube.Selection
	if !sel.Empty() && opts.Persist != "" {
		kubeSel = &sel
		ht.kubeName = sessionName(opts.Persist)
	}
	if len(opts.Script) > 0 && !opts.Resume {
		ht.startup = append(append([]string{}, ht.startup...), opts.Script...)
	}
	l := shellLaunch{name: opts.Persist, kube: kubeSel, dir: ht.initialDir, env: ht.env, command: ht.remoteCommand}
	if opts.Persist != "" {
		l.mux = ht.multiplexer
		if ht.multiplexer != "" && opts.Resume {
			ht.startup = nil
		}
	}
	ht.command = remoteCommand(l)
	info := s.open(ht, cols, rows)
	if err := s.hosts.Touch(ctx, hostID); err != nil {
		log.Printf("host %s: %v", hostID, err)
	}
	if ht.command != "" && ht.multiplexer != "" {
		info.Multiplexer = ht.multiplexer
	}
	if kubeSel != nil {
		info.Kube = &KubeInfo{Context: sel.Context, Namespace: sel.Namespace, Live: true}
		s.withSession(info.ID, func(sess *session) { sess.kube = kubeState{name: ht.kubeName, sel: sel} })
	}
	info.AutoReconnect = ht.autoReconnect
	info.Logging, info.Recording = ht.logSession, ht.recordSession
	info.Group, info.Color = ht.label.Path, ht.label.Color
	info.TerminalTheme, info.FontSize = ht.terminalTheme, ht.fontSize
	info.Via = []string{}
	for _, id := range ht.jumpChain {
		if hop, err := s.hostTarget(ctx, id); err == nil {
			info.Via = append(info.Via, hop.name)
		}
	}
	return info, nil
}

// resolvedHost is a saved host with its effective connection settings.
type resolvedHost struct {
	target             sshengine.Target
	name               string
	term               string
	keepAlive          time.Duration
	startup            []string
	identityID         string // key from the vault, tried first
	identitiesOnly     bool
	authOrder          []string
	id                 string // empty for quick connections
	jumpChain          []string
	proxy              string
	proxyCommand       string
	cloud              sshengine.Cloud
	kerberos           *sshengine.Kerberos
	passwordCommand    string
	certificateCommand string
	preConnectCommand  string
	credentialPlugin   string              // plugin configuration brokering the login
	brokered           *plugins.Credential // what it gave for this connection
	forwardAgent       bool
	multiplexer        string
	autoReconnect      bool
	command            string // run instead of the login shell
	initialDir         string
	env                []string
	remoteCommand      string
	mac                string
	wakeBroadcast      string
	wakeOnConnect      bool
	knock              string
	forwardX11         bool
	encoding           string
	logSession         bool
	logFormat          string
	recordSession      bool
	kube               kube.Selection
	kubeName           string // the pane's file on the server
	groupID            string
	terminalTheme      string
	fontSize           int
	label              hosts.GroupLabel
}

func (s *SessionService) hostTarget(ctx context.Context, hostID string) (resolvedHost, error) {
	h, err := s.hosts.Host(ctx, hostID)
	if err != nil {
		return resolvedHost{}, err
	}
	res, err := s.hosts.Resolve(ctx, h)
	if err != nil {
		return resolvedHost{}, err
	}
	t := sshengine.Target{Host: h.Hostname, Port: *res.Settings.Port, User: sshengine.DefaultUser()}
	if res.Settings.User != nil {
		t.User = *res.Settings.User
	}
	label, err := s.hosts.Label(ctx, h.GroupID)
	if err != nil {
		return resolvedHost{}, err
	}
	return resolvedHost{
		target:             t,
		name:               h.Name,
		term:               *res.Settings.TermType,
		keepAlive:          time.Duration(*res.Settings.KeepAliveSeconds) * time.Second,
		startup:            res.Settings.StartupCommands,
		identityID:         deref(res.Settings.IdentityID),
		identitiesOnly:     res.Settings.IdentitiesOnly != nil && *res.Settings.IdentitiesOnly,
		authOrder:          res.Settings.AuthMethods,
		kerberos:           kerberosOf(res.Settings),
		id:                 h.ID,
		groupID:            h.GroupID,
		passwordCommand:    deref(res.Settings.PasswordCommand),
		certificateCommand: deref(res.Settings.CertificateCommand),
		preConnectCommand:  deref(res.Settings.PreConnectCommand),
		credentialPlugin:   deref(res.Settings.CredentialPlugin),
		forwardAgent:       res.Settings.ForwardAgent != nil && *res.Settings.ForwardAgent,
		multiplexer:        deref(res.Settings.Multiplexer),
		encoding:           deref(res.Settings.Encoding),
		logSession:         res.Settings.LogSession != nil && *res.Settings.LogSession,
		logFormat:          deref(res.Settings.LogFormat),
		recordSession:      res.Settings.RecordSession != nil && *res.Settings.RecordSession,
		kube:               kube.Selection{Context: deref(res.Settings.KubeContext), Namespace: deref(res.Settings.KubeNamespace)},
		autoReconnect:      res.Settings.AutoReconnect == nil || *res.Settings.AutoReconnect,
		jumpChain:          res.Settings.JumpChain,
		proxy:              deref(res.Settings.Proxy),
		proxyCommand:       deref(res.Settings.ProxyCommand),
		cloud: sshengine.Cloud{Kind: deref(res.Settings.Transport), Target: deref(res.Settings.CloudTarget), Region: deref(res.Settings.CloudRegion),
			Account: deref(res.Settings.CloudAccount), Bastion: deref(res.Settings.CloudBastion), Group: deref(res.Settings.CloudGroup)},
		terminalTheme: deref(res.Settings.TerminalThemeID),
		fontSize:      derefInt(res.Settings.FontSize),
		label:         label,
		initialDir:    deref(res.Settings.InitialDirectory),
		env:           res.Settings.Environment,
		remoteCommand: deref(res.Settings.RemoteCommand),
		mac:           deref(res.Settings.MACAddress),
		wakeBroadcast: deref(res.Settings.WakeBroadcast),
		wakeOnConnect: res.Settings.WakeOnConnect != nil && *res.Settings.WakeOnConnect,
		knock:         deref(res.Settings.KnockSequence),
		forwardX11:    res.Settings.ForwardX11 != nil && *res.Settings.ForwardX11,
	}, nil
}

// ErrJumpLoop reports a jump chain that contains the target or a hop twice.
const ErrJumpLoop errs.Code = "ssh.jumpLoop"

// connect returns a pooled connection to the host, dialing with interactive
// prompts attributed to owner (a session, tunnel or SFTP ID) when needed.
//
// The target's jump chain describes the whole path (first hop first); the
// hops' own chains are not followed. Each hop is a pooled connection too, so
// hosts behind the same bastion share one login to it. A proxy or
// ProxyCommand applies to the first network connection.
func (s *SessionService) connect(ctx context.Context, ht resolvedHost, owner string) (*ssh.Client, func(), error) {
	seen := map[string]bool{ht.target.Key(): true}
	var hops []resolvedHost
	for _, id := range ht.jumpChain {
		hop, err := s.hostTarget(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if seen[hop.target.Key()] {
			return nil, nil, errs.New(ErrJumpLoop, errs.Params{"host": hop.name})
		}
		seen[hop.target.Key()] = true
		hops = append(hops, hop)
	}

	// Brokers (Vault, Teleport, Boundary) give the login, and may change
	// the user, the port and how the first hop is reached.
	if err := s.applyBrokers(ctx, &ht, owner); err != nil {
		return nil, nil, err
	}
	for i := range hops {
		if err := s.applyBrokers(ctx, &hops[i], owner); err != nil {
			return nil, nil, err
		}
	}

	var releases []func()
	releaseAll := func() {
		for i := len(releases) - 1; i >= 0; i-- {
			releases[i]()
		}
	}
	first := ht
	if len(hops) > 0 {
		first = hops[0]
	}
	dial, err := firstDialer(first)
	if err != nil {
		return nil, nil, err
	}
	// Wake-on-LAN and port knocking only make sense on the direct hop.
	if first.proxy == "" && first.proxyCommand == "" {
		if err := prepareConnection(ctx, first); err != nil {
			return nil, nil, err
		}
	}
	key := ""
	var client *ssh.Client
	for _, hop := range append(hops, ht) {
		key += hop.target.Key() + "|"
		c, release, err := s.dialPooled(ctx, hop, key, dial, owner)
		if err != nil {
			releaseAll()
			return nil, nil, err
		}
		releases = append(releases, release)
		client, dial = c, c.DialContext // the next hop goes through this one
	}
	var once sync.Once
	return client, func() { once.Do(releaseAll) }, nil
}

// kerberosOf returns the Kerberos options of a host, nil when it is off.
func kerberosOf(s settings.Settings) *sshengine.Kerberos {
	if s.Kerberos == nil || !*s.Kerberos {
		return nil
	}
	return &sshengine.Kerberos{Realm: deref(s.KerberosRealm), Principal: deref(s.KerberosPrincipal)}
}

// firstDialer opens the first network connection: a cloud access service,
// ProxyCommand, proxy or direct TCP.
func firstDialer(first resolvedHost) (sshengine.DialFunc, error) {
	switch {
	case first.cloud.Kind != "":
		return sshengine.CloudDialer(first.cloud, first.target)
	case first.proxyCommand != "":
		return sshengine.ProxyCommandDialer(first.proxyCommand, first.target), nil
	case first.proxy != "":
		return sshengine.ProxyDialer(first.proxy, nil)
	default:
		return nil, nil
	}
}

func (s *SessionService) dialPooled(ctx context.Context, ht resolvedHost, key string, dial sshengine.DialFunc, owner string) (*ssh.Client, func(), error) {
	return s.pool.Acquire(ctx, key, func(ctx context.Context) (*ssh.Client, error) {
		pr := &prompter{svc: s, sessionID: owner, host: ht, tried: map[string]bool{}}
		trace := sshengine.NewTrace()
		opts := sshengine.Options{
			KnownHosts: s.known, Prompter: pr, Dialer: dial, Trace: trace,
			IdentitiesOnly: ht.identitiesOnly, AuthOrder: ht.authOrder, Kerberos: ht.kerberos,
		}
		started := time.Now()
		if ht.identityID != "" {
			opts.Identity = pr.identity(ctx, ht.identityID, ht.target)
		}
		if ht.preConnectCommand != "" {
			if err := preConnect(ctx, ht, opts.Identity); err != nil {
				return nil, err
			}
		}
		if opts.Identity != nil && ht.certificateCommand != "" {
			opts.Identity = pr.certify(ctx, opts.Identity, ht)
		}
		if signer, err := brokeredSigner(ht.brokered); err != nil {
			return nil, err
		} else if signer != nil {
			opts.Identity = signer // the broker's key replaces the host's
		}
		c, err := sshengine.Dial(ctx, ht.target, opts)
		pr.closeTokens()
		s.handshakes.Add(diagnostics.Handshake{
			ID: uuid.NewString(), HostID: ht.id, Name: ht.name,
			Address: net.JoinHostPort(ht.target.Host, strconv.Itoa(ht.target.Port)), User: ht.target.User,
			Started: started, OK: err == nil, Code: string(errs.CodeOf(err)), Events: trace.Events(),
		})
		if err == nil {
			sshengine.KeepAlive(c, ht.keepAlive, 3)
			pr.commit(ctx)
		}
		return c, err
	})
}

// OpenTarget starts connecting to "user@host:port" (quick connect), or
// opens "telnet://host[:port]" or "serial://PORT?baud=N".
func (s *SessionService) OpenTarget(_ context.Context, spec string, cols, rows int) (SessionInfo, error) {
	if info, ok, err := s.openStream(strings.TrimSpace(spec), cols, rows); ok {
		return info, err
	}
	t, err := sshengine.ParseTarget(spec)
	if err != nil {
		return SessionInfo{}, err
	}
	if t.User == "" {
		t.User = sshengine.DefaultUser()
	}
	info := s.open(resolvedHost{target: t, name: t.String(), term: "xterm-256color", keepAlive: defaultKeepAlive}, cols, rows)
	info.AutoReconnect = true
	return info, nil
}

func (s *SessionService) open(ht resolvedHost, cols, rows int) SessionInfo {
	t, title, term, startup := ht.target, ht.name, ht.term, ht.startup
	ctx, cancel := context.WithCancel(context.Background())
	sess := &session{id: uuid.NewString(), ctx: ctx, cancel: cancel, ready: make(chan struct{}), flow: newFlowGate(), cols: cols, rows: rows}
	sess.capture = &capture{name: ht.name}
	highlights, active := splitRules(s.loadRules(ctx, ht))
	sess.auto = &automationState{
		host:      ht.name,
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
		// Sessions to the same target share one connection: a split pane
		// opens a channel instead of logging in again.
		client, release, err := s.connect(ctx, ht, sess.id)
		if err == nil {
			sess.sizeMu.Lock()
			cols, rows := sess.cols, sess.rows
			sess.sizeMu.Unlock()
			if ht.forwardAgent {
				s.serveAgent(client, sess.id)
			}
			var sh *sshengine.Shell
			enc, encErr := charset(ht.encoding)
			if encErr != nil {
				log.Printf("session %s: %v", sess.id, encErr) // UTF-8 then
			}
			var x11 *sshengine.X11
			if ht.forwardX11 {
				x11 = s.x11For(client)
			}
			sh, err = sshengine.StartSession(client, release, sshengine.SessionOptions{
				Term: term, Cols: cols, Rows: rows, ForwardAgent: ht.forwardAgent, Command: ht.command, X11: x11,
			})
			if err != nil {
				release()
			} else {
				sess.shell, sess.client = withTrzsz(withCharset(sh, enc), cols), client
				if ht.logSession {
					if _, err := s.StartLogging(sess.id, ht.logFormat); err != nil {
						log.Printf("session %s: log: %v", sess.id, err)
					}
				}
				if ht.recordSession {
					if _, err := s.StartRecording(sess.id); err != nil {
						log.Printf("session %s: recording: %v", sess.id, err)
					}
				}
				go s.runScript(sess, startup)
			}
		}
		if err != nil {
			sess.err = err
			p := errs.ToPayload(err)
			s.emit(EventSessionState, SessionState{ID: sess.id, State: StateFailed, Error: &p})
			s.forget(sess.id)
			return
		}
		s.emit(EventSessionState, SessionState{ID: sess.id, State: StateConnected})
	}()
	return SessionInfo{ID: sess.id, Title: title, Target: t, Group: []string{}, Via: []string{}, Highlights: highlights}
}

// Answer replies to a prompt.
func (s *SessionService) Answer(promptID string, answer PromptAnswer) error {
	s.mu.Lock()
	ch, ok := s.prompts[promptID]
	delete(s.prompts, promptID)
	s.mu.Unlock()
	if !ok {
		return errs.New(ErrSessionNotFound, errs.Params{"id": promptID})
	}
	ch <- answer
	return nil
}

// count is the number of open sessions.
func (s *SessionService) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

// Pause stops reading terminal output until Resume. The frontend calls it
// when xterm.js has too much unrendered data queued.
func (s *SessionService) Pause(id string) error {
	sess, err := s.get(id)
	if err != nil {
		return err
	}
	sess.flow.pause()
	return nil
}

// Resume restarts reading terminal output.
func (s *SessionService) Resume(id string) error {
	sess, err := s.get(id)
	if err != nil {
		return err
	}
	sess.flow.open()
	return nil
}

// Resize changes the terminal size.
func (s *SessionService) Resize(id string, cols, rows int) error {
	sess, err := s.get(id)
	if err != nil {
		return err
	}
	select {
	case <-sess.ready:
	default:
		// Not connected yet: the PTY starts with the latest size.
		sess.sizeMu.Lock()
		sess.cols, sess.rows = cols, rows
		sess.sizeMu.Unlock()
		return nil
	}
	if sess.shell == nil {
		return nil
	}
	sess.sizeMu.Lock()
	sess.cols, sess.rows = cols, rows
	sess.sizeMu.Unlock()
	sess.capture.resize(cols, rows)
	return sess.shell.Resize(cols, rows)
}

// Close ends a session, connected or not.
func (s *SessionService) Close(id string) error {
	sess, err := s.get(id)
	if err != nil {
		return nil // already gone
	}
	sess.cancel()
	sess.flow.open() // a paused reader must see the shell close
	<-sess.ready
	if sess.shell != nil {
		sess.shell.Close()
	}
	s.forget(id)
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

// TerminalStream returns the handler for the "terminal" stream. It is not a
// method so that Wails does not expose it as a binding.
func TerminalStream(s *SessionService) application.StreamHandler {
	return s.handleStream
}

// handleStream serves one terminal: the first frame names the session, then
// frames are raw terminal bytes in both directions.
func (s *SessionService) handleStream(c *application.StreamConn) {
	defer c.Close()
	var hello StreamOpen
	if err := c.ReceiveJSON(&hello); err != nil {
		return
	}
	sess, err := s.get(hello.SessionID)
	if err != nil {
		return
	}
	select {
	case <-sess.ready:
	case <-c.Context().Done():
		s.Close(sess.id)
		return
	}
	if sess.shell == nil {
		return // failed; the state event carries the error
	}

	// Output: one pump per session reads the shell; blocking Send applies
	// backpressure to the SSH channel when the frontend falls behind.
	id := sess.out.attach(c.Send, hello.Replay, func() { s.pump(sess) })
	if id < 0 {
		return
	}
	done := sess.out.doneCh()

	// Input.
	go func() {
		for {
			frame, err := c.Receive()
			if err != nil {
				return
			}
			if sess.auto != nil {
				sess.auto.typed()
			}
			if _, err := sess.shell.Write(frame); err != nil {
				return
			}
		}
	}()

	code := -1
	streamGone := false
	select {
	case <-done:
		code = sess.shell.Wait() // the remote side ended: collect the exit status
	case <-c.Context().Done():
		streamGone = true
		// The terminal went away: moving to another window, a dropped
		// stream, or a window closed or reloaded.
	}
	sess.out.release(id)
	if streamGone && sess.ctx.Err() == nil {
		if sess.handover.CompareAndSwap(true, false) {
			return // a new window attaches to the session
		}
		// The stream dropped without the pane being closed (Close ends the
		// session first): its terminal opens it again, so the session and
		// its output wait a little before being given up.
		go func() {
			select {
			case <-time.After(reattachGrace):
			case <-sess.ctx.Done():
				return
			}
			if _, err := s.get(sess.id); err != nil || sess.out.attachedSince(id) {
				return // ended meanwhile, or its terminal is back
			}
			s.endStream(sess, -1, false)
		}()
		return
	}
	// No exit status and not closed on purpose: the connection dropped.
	lost := !sess.local && code == -1 && sess.ctx.Err() == nil && c.Context().Err() == nil
	s.endStream(sess, code, lost)
}

// reattachGrace is how long a session whose stream dropped waits for its
// terminal to open the stream again.
const reattachGrace = 30 * time.Second

// endStream ends a session and tells the frontend.
func (s *SessionService) endStream(sess *session, code int, lost bool) {
	sess.shell.Close()
	s.forget(sess.id)
	s.emit(EventSessionState, SessionState{ID: sess.id, State: StateClosed, ExitCode: code, Lost: lost})
}

// pump reads the shell until it ends and hands the output to the hub.
func (s *SessionService) pump(sess *session) {
	defer sess.out.finish()
	buf := make([]byte, 32*1024)
	for {
		if !sess.flow.wait(sess.ctx) {
			return
		}
		n, err := sess.shell.Read(buf)
		if n > 0 {
			frame := make([]byte, n)
			copy(frame, buf[:n])
			s.observe(sess, frame)
			sess.capture.write(frame)
			sess.out.deliver(frame)
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				log.Printf("session %s: read: %v", sess.id, err)
			}
			return
		}
	}
}

func (s *SessionService) get(id string) (*session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return nil, errs.New(ErrSessionNotFound, errs.Params{"id": id})
	}
	return sess, nil
}

func (s *SessionService) forget(id string) {
	s.mu.Lock()
	sess := s.sessions[id]
	delete(s.sessions, id)
	s.mu.Unlock()
	if sess != nil && sess.kube.file != "" {
		os.Remove(sess.kube.file)
	}
	if sess != nil && sess.capture != nil {
		sess.capture.close()
	}
}

// ask emits a prompt and waits for the answer or the session's end.
func (s *SessionService) ask(ctx context.Context, p Prompt) (PromptAnswer, error) {
	p.ID = uuid.NewString()
	ch := make(chan PromptAnswer, 1)
	s.mu.Lock()
	s.prompts[p.ID] = ch
	s.mu.Unlock()
	s.emit(EventSessionPrompt, p)

	select {
	case a := <-ch:
		if a.Cancel {
			return a, errs.New(sshengine.ErrCancelled, nil)
		}
		return a, nil
	case <-ctx.Done():
		s.mu.Lock()
		delete(s.prompts, p.ID)
		s.mu.Unlock()
		return PromptAnswer{}, errs.Wrap(ctx.Err(), sshengine.ErrCancelled, nil)
	}
}

type prompter struct {
	svc       *SessionService
	sessionID string
	host      resolvedHost

	tried   map[string]bool // scopes whose saved credential was already used
	pending []pendingSave   // answers to save once the login succeeds
	tokens  []*pkcs11.Signer
}

type pendingSave struct {
	scope, label string
	secret       []byte
}

func (p *prompter) Password(ctx context.Context, t sshengine.Target, attempt int) (string, error) {
	if b := p.host.brokered; b != nil && b.Password != "" && !p.tried["broker"] {
		p.tried["broker"] = true
		return b.Password, nil
	}
	if p.host.passwordCommand != "" && !p.tried["command"] {
		p.tried["command"] = true
		if out, err := runCredentialCommand(ctx, p.host.passwordCommand, targetEnv(t)); err == nil && out != "" {
			return out, nil
		}
	}
	scope := vault.PasswordScope(t.User, t.Host, t.Port)
	if secret, ok := p.saved(ctx, scope, t); ok {
		return string(secret), nil
	}
	a, err := p.svc.ask(ctx, Prompt{SessionID: p.sessionID, Kind: "password", Target: t, Attempt: attempt, CanSave: p.canSave(ctx)})
	if err == nil && a.Save {
		p.pending = append(p.pending, pendingSave{scope: scope, label: t.String(), secret: []byte(a.Value)})
	}
	return a.Value, err
}

// Challenge answers keyboard-interactive rounds: one-time codes from the
// host's TOTP seed and passwords from the vault or a password command are
// filled in; anything left is asked to the user.
func (p *prompter) Challenge(ctx context.Context, t sshengine.Target, c sshengine.Challenge) ([]string, error) {
	answers := make([]string, len(c.Questions))
	auto := make([]bool, len(c.Questions))
	pending := false
	for i, q := range c.Questions {
		switch {
		case otp.LooksLikeOTPPrompt(q):
			if code, ok := p.totpCode(ctx); ok {
				answers[i], auto[i] = code, true
				continue
			}
		case c.Round == 1 && looksLikePassword(q):
			if pw, ok := p.knownPassword(ctx, t); ok {
				answers[i], auto[i] = pw, true
				continue
			}
		}
		pending = true
	}
	if !pending {
		return answers, nil
	}
	a, err := p.svc.ask(ctx, Prompt{SessionID: p.sessionID, Kind: "challenge", Target: t, Challenge: &c, Autofilled: auto, Attempt: c.Round})
	if err != nil {
		return nil, err
	}
	for i := range answers {
		if !auto[i] && i < len(a.Answers) {
			answers[i] = a.Answers[i]
		}
	}
	return answers, nil
}

func looksLikePassword(q string) bool {
	q = strings.ToLower(q)
	return strings.Contains(q, "password") && !otp.LooksLikeOTPPrompt(q)
}

// knownPassword returns the password from the host's password command or
// the vault, without asking the user.
func (p *prompter) knownPassword(ctx context.Context, t sshengine.Target) (string, bool) {
	if b := p.host.brokered; b != nil && b.Password != "" && !p.tried["broker"] {
		p.tried["broker"] = true
		return b.Password, true
	}
	if p.host.passwordCommand != "" && !p.tried["command"] {
		p.tried["command"] = true
		if out, err := runCredentialCommand(ctx, p.host.passwordCommand, targetEnv(t)); err == nil && out != "" {
			return out, true
		}
	}
	if secret, ok := p.saved(ctx, vault.PasswordScope(t.User, t.Host, t.Port), t); ok {
		return string(secret), true
	}
	return "", false
}

// totpCode computes the current code from the host's seed in the vault,
// once per connection (a rejected code must not be retried).
func (p *prompter) totpCode(ctx context.Context) (string, bool) {
	if p.host.id == "" {
		return "", false
	}
	seed, ok := p.saved(ctx, TOTPScope(p.host.id), p.host.target)
	if !ok {
		return "", false
	}
	defer vault.Zero(seed)
	cfg, err := otp.Parse(string(seed))
	if err != nil {
		return "", false
	}
	return cfg.Code(time.Now()), true
}

// certify asks the host's certificate command for a short-lived certificate
// of signer's public key. On failure the plain key is used.
func (p *prompter) certify(ctx context.Context, signer ssh.Signer, ht resolvedHost) ssh.Signer {
	pub := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
	env := targetEnv(ht.target)
	env["SHELLVANE_PUBLIC_KEY"] = pub
	out, err := runCredentialCommand(ctx, ht.certificateCommand, env)
	if err != nil {
		log.Printf("certificate command for %s: %v", ht.name, err)
		return signer
	}
	cert, err := keys.ParseCertificate(out)
	if err != nil {
		log.Printf("certificate command for %s: %v", ht.name, err)
		return signer
	}
	cs, err := keys.CertSigner(cert, signer)
	if err != nil {
		return signer
	}
	return cs
}

func (p *prompter) Passphrase(ctx context.Context, key sshengine.KeyInfo, attempt int) (string, error) {
	scope := vault.PassphraseScope(key.Fingerprint)
	if secret, ok := p.saved(ctx, scope, sshengine.Target{}); ok {
		return string(secret), nil
	}
	a, err := p.svc.ask(ctx, Prompt{SessionID: p.sessionID, Kind: "passphrase", Key: &key, Attempt: attempt, CanSave: p.canSave(ctx)})
	if err == nil && a.Save {
		p.pending = append(p.pending, pendingSave{scope: scope, label: key.Path, secret: []byte(a.Value)})
	}
	return a.Value, err
}

// saved returns the vault's answer for scope, once per connection: if it
// is wrong, the next attempt asks the user. A locked vault is unlocked first.
func (p *prompter) saved(ctx context.Context, scope string, t sshengine.Target) ([]byte, bool) {
	v := p.svc.vault
	if v == nil || p.tried[scope] {
		return nil, false
	}
	p.tried[scope] = true
	if has, err := v.v.HasCredential(ctx, scope); err != nil || !has {
		return nil, false
	}
	if st, _ := v.v.State(ctx); st == vault.StateLocked {
		if !p.unlock(ctx, t) {
			return nil, false
		}
	}
	secret, found, err := v.v.Credential(ctx, scope)
	if err != nil || !found {
		return nil, false
	}
	v.touch()
	return secret, true
}

// unlock opens the vault: with the system keychain (Touch ID, Windows
// Hello) when enabled, else by asking the master password; cancelling
// falls back to a prompt.
//
// The panes of an environment ask one at a time: the next one finds the
// vault open (by the keychain, Touch ID or the master password typed for
// the first) and asks nothing. An answer to a prompt that arrives once
// the vault was opened elsewhere (the UI answers pending prompts when the
// vault opens) counts as unlocked.
func (p *prompter) unlock(ctx context.Context, t sshengine.Target) bool {
	p.svc.unlockMu.Lock()
	defer p.svc.unlockMu.Unlock()
	open := func() bool {
		st, _ := p.svc.vault.Status(ctx)
		return st.State == vault.StateUnlocked
	}
	if open() {
		return true
	}
	if has, _ := p.svc.vault.HasKeychainUnlock(ctx); has && p.svc.vault.UnlockWithKeychain(ctx) == nil {
		return true
	}
	for attempt := 1; attempt <= 3; attempt++ {
		a, err := p.svc.ask(ctx, Prompt{SessionID: p.sessionID, Kind: "vaultUnlock", Target: t, Attempt: attempt})
		if err != nil {
			return false
		}
		if open() || p.svc.vault.Unlock(ctx, a.Value) == nil {
			return true
		}
	}
	return false
}

// identity loads the host's key from the vault, unlocking it if needed. A
// missing key or a refused unlock is not fatal: other methods still apply.
func (p *prompter) identity(ctx context.Context, keyID string, t sshengine.Target) ssh.Signer {
	// A hardware key needs its token and PIN, not the vault.
	if s, isToken := p.tokenIdentity(ctx, keyID); isToken {
		return s
	}
	if p.svc.keys == nil || p.svc.vault == nil {
		return nil
	}
	if st, _ := p.svc.vault.v.State(ctx); st == vault.StateLocked && !p.unlock(ctx, t) {
		return nil
	}
	signer, err := p.svc.keys.Signer(ctx, keyID)
	if err != nil {
		log.Printf("identity %s: %v", keyID, err)
		return nil
	}
	p.svc.vault.touch()
	return signer
}

func (p *prompter) canSave(ctx context.Context) bool {
	if p.svc.vault == nil {
		return false
	}
	st, err := p.svc.vault.v.State(ctx)
	return err == nil && st == vault.StateUnlocked
}

// commit saves the answers the user asked to keep, after a successful login.
func (p *prompter) commit(ctx context.Context) {
	for _, s := range p.pending {
		if err := p.svc.vault.v.SaveCredential(ctx, s.scope, s.label, s.secret); err != nil {
			log.Printf("save credential %s: %v", s.label, err)
		}
		vault.Zero(s.secret)
	}
	p.pending = nil
}

func (p *prompter) ConfirmHostKey(ctx context.Context, key knownhosts.UnknownKey) (knownhosts.Decision, error) {
	a, err := p.svc.ask(ctx, Prompt{SessionID: p.sessionID, Kind: "hostKey", HostKey: &key,
		Target: sshengine.Target{Host: key.Host, Port: key.Port}})
	if err != nil {
		return knownhosts.Reject, err
	}
	switch a.Decision {
	case "save":
		return knownhosts.AcceptSave, nil
	case "once":
		return knownhosts.AcceptOnce, nil
	default:
		return knownhosts.Reject, nil
	}
}
