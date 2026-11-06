package app

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/netprobe"
)

// HostStatus is whether a host's SSH port answers.
type HostStatus struct {
	HostID string `json:"hostId"`
	// State is "up", "down" or "unknown" (reached through a jump host or
	// a proxy, so not probed directly).
	State     string `json:"state"`
	LatencyMs int    `json:"latencyMs"`
}

const probeTimeout = 2500 * time.Millisecond

// Status probes the SSH port of the hosts (all when ids is empty), a few
// at a time.
func (s *HostService) Status(ctx context.Context, ids []string) ([]HostStatus, error) {
	inv, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := []HostStatus{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 16)
	for _, h := range inv.Hosts {
		if len(ids) > 0 && !want[h.ID] {
			continue
		}
		res, err := s.repo.Resolve(ctx, h)
		if err != nil {
			return nil, err
		}
		st := HostStatus{HostID: h.ID, State: "unknown"}
		if len(res.Settings.JumpChain) > 0 || deref(res.Settings.Proxy) != "" || deref(res.Settings.ProxyCommand) != "" || deref(res.Settings.Transport) != "" {
			mu.Lock() // probes started for earlier hosts append too
			out = append(out, st)
			mu.Unlock()
			continue
		}
		port := 22
		if res.Settings.Port != nil {
			port = *res.Settings.Port
		}
		addr := net.JoinHostPort(h.Hostname, strconv.Itoa(port))
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if d, err := netprobe.Probe(ctx, addr, probeTimeout); err == nil {
				st.State, st.LatencyMs = "up", max(1, int(d.Milliseconds()))
			} else {
				st.State = "down"
			}
			mu.Lock()
			out = append(out, st)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return out, nil
}

// Wake sends a Wake-on-LAN packet to the host's MAC address.
func (s *HostService) Wake(ctx context.Context, hostID string) error {
	h, err := s.repo.Host(ctx, hostID)
	if err != nil {
		return err
	}
	res, err := s.repo.Resolve(ctx, h)
	if err != nil {
		return err
	}
	return netprobe.Wake(deref(res.Settings.MACAddress), deref(res.Settings.WakeBroadcast))
}

// prepare wakes and knocks before the first network connection of ht,
// when the host asks for it.
func prepareConnection(ctx context.Context, ht resolvedHost) error {
	if ht.wakeOnConnect && ht.mac != "" {
		if err := netprobe.Wake(ht.mac, ht.wakeBroadcast); err != nil {
			return err
		}
		// Wait for the machine to boot: up to two minutes.
		deadline := time.Now().Add(2 * time.Minute)
		addr := net.JoinHostPort(ht.target.Host, strconv.Itoa(ht.target.Port))
		for time.Now().Before(deadline) {
			if _, err := netprobe.Probe(ctx, addr, 2*time.Second); err == nil {
				break
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
	}
	if ht.knock != "" {
		steps, err := netprobe.ParseKnock(ht.knock)
		if err != nil {
			return err
		}
		if err := netprobe.Knock(ctx, ht.target.Host, steps, 300*time.Millisecond); err != nil {
			return err
		}
		time.Sleep(300 * time.Millisecond) // let the firewall open
	}
	return nil
}
