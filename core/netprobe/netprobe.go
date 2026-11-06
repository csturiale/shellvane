// Package netprobe checks whether hosts answer, wakes them with Wake-on-LAN
// and opens firewalls with port knocking.
package netprobe

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes.
const (
	ErrBadMAC   errs.Code = "netprobe.badMac"
	ErrBadKnock errs.Code = "netprobe.badKnock"
	ErrWake     errs.Code = "netprobe.wakeFailed"
	ErrKnock    errs.Code = "netprobe.knockFailed"
)

// Probe opens a TCP connection to addr and returns how long it took.
func Probe(ctx context.Context, addr string, timeout time.Duration) (time.Duration, error) {
	d := net.Dialer{Timeout: timeout}
	start := time.Now()
	c, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, err
	}
	c.Close()
	return time.Since(start), nil
}

// ParseMAC reads "aa:bb:cc:dd:ee:ff", "aa-bb-...", or "aabb.ccdd.eeff".
func ParseMAC(s string) (net.HardwareAddr, error) {
	mac, err := net.ParseMAC(strings.TrimSpace(s))
	if err != nil || len(mac) != 6 {
		return nil, errs.New(ErrBadMAC, errs.Params{"mac": s})
	}
	return mac, nil
}

// MagicPacket is 6 × 0xFF followed by the MAC 16 times.
func MagicPacket(mac net.HardwareAddr) []byte {
	p := make([]byte, 0, 102)
	for range 6 {
		p = append(p, 0xff)
	}
	for range 16 {
		p = append(p, mac...)
	}
	return p
}

// Wake sends a Wake-on-LAN magic packet to broadcast ("" means
// 255.255.255.255:9; a port may be omitted).
func Wake(mac, broadcast string) error {
	hw, err := ParseMAC(mac)
	if err != nil {
		return err
	}
	if broadcast == "" {
		broadcast = "255.255.255.255"
	}
	if _, _, err := net.SplitHostPort(broadcast); err != nil {
		broadcast = net.JoinHostPort(broadcast, "9")
	}
	c, err := net.Dial("udp", broadcast)
	if err != nil {
		return errs.Wrap(err, ErrWake, errs.Params{"addr": broadcast})
	}
	defer c.Close()
	if _, err := c.Write(MagicPacket(hw)); err != nil {
		return errs.Wrap(err, ErrWake, errs.Params{"addr": broadcast})
	}
	return nil
}

// KnockStep is one port of a knocking sequence.
type KnockStep struct {
	Port int
	UDP  bool
}

// ParseKnock reads "7000,8000/udp,9000/tcp" (spaces allowed).
func ParseKnock(spec string) ([]KnockStep, error) {
	var out []KnockStep
	for _, part := range strings.FieldsFunc(spec, func(r rune) bool { return r == ',' || r == ' ' }) {
		port, proto, _ := strings.Cut(strings.ToLower(part), "/")
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || (proto != "" && proto != "tcp" && proto != "udp") {
			return nil, errs.New(ErrBadKnock, errs.Params{"value": part})
		}
		out = append(out, KnockStep{Port: n, UDP: proto == "udp"})
	}
	if len(out) == 0 {
		return nil, errs.New(ErrBadKnock, errs.Params{"value": spec})
	}
	return out, nil
}

// Knock touches each port of the sequence in order, delay apart. TCP
// knocks are connection attempts whose outcome does not matter (the port
// is normally closed); UDP knocks send an empty datagram.
func Knock(ctx context.Context, host string, steps []KnockStep, delay time.Duration) error {
	for i, s := range steps {
		addr := net.JoinHostPort(host, strconv.Itoa(s.Port))
		if s.UDP {
			c, err := net.Dial("udp", addr)
			if err != nil {
				return errs.Wrap(err, ErrKnock, errs.Params{"addr": addr})
			}
			c.Write(nil)
			c.Close()
		} else {
			d := net.Dialer{Timeout: 300 * time.Millisecond}
			if c, err := d.DialContext(ctx, "tcp", addr); err == nil {
				c.Close()
			}
		}
		if i < len(steps)-1 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
