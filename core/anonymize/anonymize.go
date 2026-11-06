// Package anonymize replaces names, addresses and users with consistent
// pseudonyms (host-01, user-01, group-01, 10.255.0.x), so that a support
// bundle or a diagnostic report stays correlated without revealing them.
// The mapping lives only in memory and is never written out.
package anonymize

import (
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Kinds of values.
const (
	KindHost  = "host"
	KindUser  = "user"
	KindGroup = "group"
	KindTag   = "tag"
	KindIP    = "ip"
)

// Anonymizer hands out pseudonyms; the same value always gets the same one.
type Anonymizer struct {
	mu     sync.Mutex
	names  map[string]map[string]string // kind → value → pseudonym
	counts map[string]int
}

// New returns an empty Anonymizer.
func New() *Anonymizer {
	return &Anonymizer{names: map[string]map[string]string{}, counts: map[string]int{}}
}

// Name returns the pseudonym of a value of a kind; "" stays "".
// IP addresses always get an IP pseudonym, whatever the kind.
func (a *Anonymizer) Name(kind, value string) string {
	if value == "" {
		return ""
	}
	if ip := net.ParseIP(strings.Trim(value, "[]")); ip != nil {
		kind = KindIP
		value = ip.String()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	m := a.names[kind]
	if m == nil {
		m = map[string]string{}
		a.names[kind] = m
	}
	if p, ok := m[value]; ok {
		return p
	}
	a.counts[kind]++
	n := a.counts[kind]
	var p string
	if kind == KindIP {
		p = fmt.Sprintf("10.255.%d.%d", (n-1)/250, (n-1)%250+1)
	} else {
		p = fmt.Sprintf("%s-%02d", kind, n)
	}
	m[value] = p
	return p
}

// Host anonymizes "host", "host:port" or "[v6]:port", keeping the port.
func (a *Anonymizer) Host(addr string) string {
	if h, port, err := net.SplitHostPort(addr); err == nil {
		return net.JoinHostPort(a.Name(KindHost, h), port)
	}
	return a.Name(KindHost, addr)
}

var (
	ipv4 = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	ipv6 = regexp.MustCompile(`\b[0-9a-fA-F]{0,4}(?::[0-9a-fA-F]{0,4}){2,7}\b`)
)

// Text replaces, in free text, every value already given a pseudonym and
// any IP address. Longer values are replaced first.
func (a *Anonymizer) Text(s string) string {
	a.mu.Lock()
	type pair struct{ from, to string }
	var pairs []pair
	for kind, m := range a.names {
		if kind == KindIP {
			continue
		}
		for v, p := range m {
			if len(v) >= 3 { // too short values would hit unrelated text
				pairs = append(pairs, pair{v, p})
			}
		}
	}
	a.mu.Unlock()
	sort.Slice(pairs, func(i, j int) bool { return len(pairs[i].from) > len(pairs[j].from) })
	for _, p := range pairs {
		s = strings.ReplaceAll(s, p.from, p.to)
	}
	s = ipv4.ReplaceAllStringFunc(s, func(m string) string {
		if net.ParseIP(m) == nil {
			return m
		}
		return a.Name(KindIP, m)
	})
	return ipv6.ReplaceAllStringFunc(s, func(m string) string {
		if ip := net.ParseIP(m); ip == nil || ip.To4() != nil {
			return m
		}
		return a.Name(KindIP, m)
	})
}
