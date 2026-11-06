package app

import (
	"context"
	"sync"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/monitor"
)

// ErrMonitorFailed reports that the server's figures could not be read.
const ErrMonitorFailed errs.Code = "monitor.failed"

// MonitorService samples servers' load, CPU, memory and disk over their
// (pooled) SSH connection.
type MonitorService struct {
	sessions *SessionService

	mu   sync.Mutex
	prev map[string]*monitor.Stats // last sample per host, for CPU use
}

// NewMonitorService returns a sampler.
func NewMonitorService(sessions *SessionService) *MonitorService {
	return &MonitorService{sessions: sessions, prev: map[string]*monitor.Stats{}}
}

// Sample reads the host's figures now. CPU use is measured since the
// previous sample of the same host (unknown on the first one).
func (s *MonitorService) Sample(ctx context.Context, hostID string) (monitor.Stats, error) {
	ht, err := s.sessions.hostTarget(ctx, hostID)
	if err != nil {
		return monitor.Stats{}, err
	}
	client, release, err := s.sessions.connect(ctx, ht, "monitor:"+hostID)
	if err != nil {
		return monitor.Stats{}, err
	}
	defer release()
	sess, err := client.NewSession()
	if err != nil {
		return monitor.Stats{}, errs.Wrap(err, ErrMonitorFailed, nil)
	}
	defer sess.Close()
	out, err := sess.Output(monitor.Script)
	if err != nil && len(out) == 0 {
		return monitor.Stats{}, errs.Wrap(err, ErrMonitorFailed, nil)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := monitor.Parse(string(out), s.prev[hostID])
	s.prev[hostID] = &st
	return st, nil
}
