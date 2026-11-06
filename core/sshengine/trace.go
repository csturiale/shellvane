package sshengine

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// Steps recorded in a Trace. They are codes; the UI translates them.
const (
	StepDial          = "dial"
	StepConnected     = "connected"
	StepServerVersion = "serverVersion"
	StepAlgorithms    = "algorithms"
	StepHostKey       = "hostKey"
	StepAuthMethods   = "authMethods"
	StepKeyOffered    = "keyOffered"
	StepAuthFailed    = "authFailed"
	StepBanner        = "banner"
	StepAuthenticated = "authenticated"
	StepError         = "error"
)

// TraceEvent is one step of a connection handshake.
type TraceEvent struct {
	// Ms is the time since the start of the trace.
	Ms     int64             `json:"ms"`
	Step   string            `json:"step"`
	Detail map[string]string `json:"detail"`
}

// Trace records how a connection was set up: transport, server version,
// negotiated algorithms, host key, authentication methods offered and
// tried, and why it failed. It never records secrets. A nil Trace records
// nothing.
type Trace struct {
	mu     sync.Mutex
	start  time.Time
	events []TraceEvent
}

// NewTrace starts a trace now.
func NewTrace() *Trace { return &Trace{start: time.Now()} }

// Add records a step with key/value pairs.
func (t *Trace) Add(step string, kv ...string) {
	if t == nil {
		return
	}
	d := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		d[kv[i]] = kv[i+1]
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, TraceEvent{Ms: time.Since(t.start).Milliseconds(), Step: step, Detail: d})
}

// Events returns a copy of the steps recorded so far.
func (t *Trace) Events() []TraceEvent {
	if t == nil {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]TraceEvent(nil), t.events...)
}

// authObserver records what the server allows and which methods failed;
// it never chooses a method itself.
func (t *Trace) authObserver() ssh.ClientAuthCallback {
	if t == nil {
		return nil
	}
	first, tried, failed := true, 0, 0
	return func(ctx *ssh.ClientAuthContext) (ssh.AuthMethod, error) {
		if first {
			first = false
			a := ctx.Algorithms
			t.Add(StepServerVersion, "version", string(ctx.Metadata.ServerVersion()))
			t.Add(StepAlgorithms,
				"kex", a.KeyExchange, "hostKey", a.HostKey,
				"cipher", a.Write.Cipher, "mac", macOrAEAD(a.Write.MAC))
		}
		for _, m := range ctx.TriedMethods[tried:] {
			if m != "none" { // the probe for the allowed methods
				failed++
				t.Add(StepAuthFailed, "method", m)
			}
		}
		tried = len(ctx.TriedMethods)
		t.Add(StepAuthMethods, "allowed", strings.Join(ctx.AllowedMethods, ","), "attempt", strconv.Itoa(failed+1))
		return nil, nil
	}
}

// offered records the keys the publickey method will offer, in order.
func (t *Trace) offered(keys func() ([]ssh.Signer, error)) func() ([]ssh.Signer, error) {
	if t == nil {
		return keys
	}
	return func() ([]ssh.Signer, error) {
		signers, err := keys()
		for _, s := range signers {
			t.Add(StepKeyOffered, "type", s.PublicKey().Type(), "fingerprint", ssh.FingerprintSHA256(s.PublicKey()))
		}
		return signers, err
	}
}

func macOrAEAD(mac string) string {
	if mac == "" {
		return "(aead)"
	}
	return mac
}
