package tunnel

import (
	"encoding/hex"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// RemotePort is a TCP port listening on a server.
type RemotePort struct {
	Port int `json:"port"`
	// Addr is the address it listens on ("0.0.0.0", "127.0.0.1", "::"...).
	Addr string `json:"addr"`
	// Process is the program's name when the server tells it ("" otherwise).
	Process string `json:"process"`
}

// ListeningScript prints the server's listening TCP sockets with the first
// tool available: ss, netstat, or /proc on Linux without either. The
// first line names the format.
const ListeningScript = `if command -v ss >/dev/null 2>&1; then echo '#ss'; ss -Hltnp 2>/dev/null || ss -Hltn;
elif command -v netstat >/dev/null 2>&1; then echo '#netstat'; netstat -ltnp 2>/dev/null || netstat -an -p tcp 2>/dev/null || netstat -an;
elif [ -r /proc/net/tcp ]; then echo '#proc'; cat /proc/net/tcp /proc/net/tcp6 2>/dev/null;
else echo '#none'; fi; true`

var processName = regexp.MustCompile(`users:\(\("([^"]+)"`)

// ParseListening reads the output of ListeningScript. Ports are sorted and
// listed once (the first address seen wins, IPv4 before IPv6).
func ParseListening(out string) []RemotePort {
	lines := strings.Split(strings.ReplaceAll(out, "\r", ""), "\n")
	if len(lines) == 0 {
		return nil
	}
	format := strings.TrimSpace(lines[0])
	var found []RemotePort
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		var p RemotePort
		ok := false
		switch format {
		case "#ss":
			// LISTEN 0 4096 127.0.0.1:631 0.0.0.0:* users:(("cupsd",pid=1,fd=7))
			if len(f) >= 4 && f[0] == "LISTEN" {
				p, ok = splitListenAddr(f[3])
				if m := processName.FindStringSubmatch(line); m != nil {
					p.Process = m[1]
				}
			}
		case "#netstat":
			// Linux: tcp 0 0 0.0.0.0:22 0.0.0.0:* LISTEN 812/sshd
			// BSD/macOS: tcp4 0 0 *.22 *.* LISTEN
			if len(f) >= 6 && strings.HasPrefix(f[0], "tcp") && f[5] == "LISTEN" {
				p, ok = splitListenAddr(f[3])
				if len(f) >= 7 {
					if _, name, found := strings.Cut(f[6], "/"); found {
						p.Process = name
					}
				}
			}
		case "#proc":
			// sl local_address rem_address st ...; st 0A is LISTEN.
			if len(f) >= 4 && f[3] == "0A" {
				p, ok = procAddr(f[1])
			}
		}
		if ok && p.Port > 0 {
			found = append(found, p)
		}
	}
	seen := map[int]bool{}
	var out2 []RemotePort
	for _, p := range found {
		if !seen[p.Port] {
			seen[p.Port] = true
			out2 = append(out2, p)
		}
	}
	sort.Slice(out2, func(i, j int) bool { return out2[i].Port < out2[j].Port })
	return out2
}

// splitListenAddr reads "addr:port", "[v6]:port", "*:port", "addr%iface:port"
// and the BSD form "addr.port".
func splitListenAddr(s string) (RemotePort, bool) {
	i := strings.LastIndexAny(s, ":.")
	if i < 0 {
		return RemotePort{}, false
	}
	port, err := strconv.Atoi(s[i+1:])
	if err != nil {
		return RemotePort{}, false
	}
	addr := strings.Trim(s[:i], "[]")
	if j := strings.Index(addr, "%"); j >= 0 {
		addr = addr[:j]
	}
	if addr == "*" || addr == "" {
		addr = "0.0.0.0"
	}
	return RemotePort{Port: port, Addr: addr}, true
}

// procAddr reads "0100007F:1F90" (IPv4) or 32 hex digits (IPv6), stored
// as little-endian 32-bit words.
func procAddr(s string) (RemotePort, bool) {
	hexAddr, hexPort, found := strings.Cut(s, ":")
	if !found {
		return RemotePort{}, false
	}
	port, err := strconv.ParseUint(hexPort, 16, 16)
	if err != nil {
		return RemotePort{}, false
	}
	raw, err := hex.DecodeString(hexAddr)
	if err != nil || (len(raw) != 4 && len(raw) != 16) {
		return RemotePort{}, false
	}
	for w := 0; w < len(raw); w += 4 {
		raw[w], raw[w+1], raw[w+2], raw[w+3] = raw[w+3], raw[w+2], raw[w+1], raw[w]
	}
	return RemotePort{Port: int(port), Addr: net.IP(raw).String()}, true
}

// DestHost is the address to forward to for a port listening on addr:
// wildcard listeners are reached on the loopback.
func DestHost(addr string) string {
	switch addr {
	case "", "0.0.0.0", "*":
		return "127.0.0.1"
	case "::":
		return "::1"
	}
	return addr
}
