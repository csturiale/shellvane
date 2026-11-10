package netproxy

import (
	"strings"

	"golang.org/x/net/http/httpproxy"
)

// parseScutil reads the output of scutil --proxy: HTTPEnable/HTTPProxy/
// HTTPPort, HTTPSEnable/HTTPSProxy/HTTPSPort, SOCKSEnable/SOCKSProxy/
// SOCKSPort and ExceptionsList.
func parseScutil(out string) *httpproxy.Config {
	v := map[string]string{}
	var exceptions []string
	inList := false
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ExceptionsList") {
			inList = true
			continue
		}
		if inList {
			if line == "}" {
				inList = false
				continue
			}
			if _, val, ok := strings.Cut(line, ":"); ok {
				exceptions = append(exceptions, strings.TrimSpace(val))
			}
			continue
		}
		if k, val, ok := strings.Cut(line, ":"); ok {
			v[strings.TrimSpace(k)] = strings.TrimSpace(val)
		}
	}
	proxy := func(kind, scheme string) string {
		if v[kind+"Enable"] != "1" || v[kind+"Proxy"] == "" {
			return ""
		}
		hp := v[kind+"Proxy"]
		if p := v[kind+"Port"]; p != "" {
			hp += ":" + p
		}
		return scheme + "://" + hp
	}
	c := &httpproxy.Config{HTTPProxy: proxy("HTTP", "http"), HTTPSProxy: proxy("HTTPS", "http")}
	if socks := proxy("SOCKS", "socks5"); socks != "" {
		if c.HTTPProxy == "" {
			c.HTTPProxy = socks
		}
		if c.HTTPSProxy == "" {
			c.HTTPSProxy = socks
		}
	}
	if c.HTTPProxy == "" && c.HTTPSProxy == "" {
		return nil
	}
	if v["ExcludeSimpleHostnames"] == "1" {
		exceptions = append(exceptions, "<local>")
	}
	c.NoProxy = noProxyList(exceptions)
	return c
}

// parseWindows reads ProxyServer ("host:port" or
// "http=host:port;https=host:port;socks=host:port") and ProxyOverride
// ("*.corp;<local>").
func parseWindows(server, override string) *httpproxy.Config {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil
	}
	c := &httpproxy.Config{}
	if !strings.Contains(server, "=") {
		p := withScheme(server, "http")
		c.HTTPProxy, c.HTTPSProxy = p, p
	} else {
		var socks string
		for _, part := range strings.Split(server, ";") {
			k, val, ok := strings.Cut(strings.TrimSpace(part), "=")
			if !ok || val == "" {
				continue
			}
			switch strings.ToLower(k) {
			case "http":
				c.HTTPProxy = withScheme(val, "http")
			case "https":
				c.HTTPSProxy = withScheme(val, "http")
			case "socks":
				socks = withScheme(val, "socks5")
			}
		}
		if c.HTTPSProxy == "" {
			c.HTTPSProxy = firstOf(c.HTTPProxy, socks)
		}
		if c.HTTPProxy == "" {
			c.HTTPProxy = firstOf(c.HTTPSProxy, socks)
		}
	}
	if c.HTTPProxy == "" && c.HTTPSProxy == "" {
		return nil
	}
	c.NoProxy = noProxyList(strings.Split(override, ";"))
	return c
}

func withScheme(hp, scheme string) string {
	hp = strings.TrimSpace(hp)
	if strings.Contains(hp, "://") {
		return hp
	}
	return scheme + "://" + hp
}

func firstOf(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// noProxyList turns exception patterns (*.corp, 10.*, <local>) into a
// NO_PROXY list. <local> (plain host names) has no NO_PROXY equivalent and
// is dropped: loopback addresses are always direct anyway.
func noProxyList(patterns []string) string {
	var out []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		switch {
		case p == "" || p == "<local>":
			continue
		case strings.HasPrefix(p, "*."):
			out = append(out, p[1:]) // .corp
		case strings.HasSuffix(p, ".*"):
			// 10.* : as a CIDR when it is an address prefix.
			parts := strings.Split(strings.TrimSuffix(p, ".*"), ".")
			if cidr := prefixCIDR(parts); cidr != "" {
				out = append(out, cidr)
			}
		default:
			out = append(out, strings.TrimPrefix(p, "*"))
		}
	}
	return strings.Join(out, ",")
}

func prefixCIDR(parts []string) string {
	if len(parts) == 0 || len(parts) > 3 {
		return ""
	}
	octets := []string{"0", "0", "0", "0"}
	for i, p := range parts {
		for _, r := range p {
			if r < '0' || r > '9' {
				return ""
			}
		}
		octets[i] = p
	}
	return strings.Join(octets, ".") + "/" + []string{"", "8", "16", "24"}[len(parts)]
}
