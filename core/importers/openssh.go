package importers

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/csturiale/shellvane/core/tunnel"
)

// maxIncludeDepth stops include loops.
const maxIncludeDepth = 16

// OpenSSH reads an ssh_config file and the files it includes. Every Host line
// without wildcards becomes a host per alias; "Host *" and other patterns
// are reported, since their settings apply to many hosts.
func OpenSSH(path string) (Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	p := &sshParser{base: filepath.Dir(path)}
	p.parse(f, path, 0)
	p.flush()
	return p.res, nil
}

type sshParser struct {
	base    string // relative Include paths resolve from ~/.ssh, i.e. here
	res     Result
	aliases []string
	cur     Host
	inMatch bool
}

func (p *sshParser) parse(r io.Reader, file string, depth int) {
	sc := bufio.NewScanner(r)
	line := 0
	for sc.Scan() {
		line++
		key, args := splitDirective(sc.Text())
		if key == "" {
			continue
		}
		at := map[string]any{"file": filepath.Base(file), "line": line}
		switch key {
		case "include":
			if depth >= maxIncludeDepth {
				p.res.warn("includeLoop", at)
				continue
			}
			for _, pattern := range args {
				pattern = expandHome(pattern)
				if !filepath.IsAbs(pattern) {
					pattern = filepath.Join(p.base, pattern)
				}
				matches, _ := filepath.Glob(pattern)
				for _, m := range matches {
					if f, err := os.Open(m); err == nil {
						p.parse(f, m, depth+1)
						f.Close()
					}
				}
			}
		case "host":
			p.flush()
			p.inMatch = false
			p.aliases = nil
			for _, a := range args {
				if strings.ContainsAny(a, "*?!") {
					p.res.warn("patternSkipped", map[string]any{"pattern": a, "file": at["file"], "line": line})
					continue
				}
				p.aliases = append(p.aliases, a)
			}
		case "match":
			p.flush()
			p.aliases = nil
			p.inMatch = true
			p.res.warn("matchSkipped", at)
		default:
			if p.inMatch || len(p.aliases) == 0 || len(args) == 0 {
				continue
			}
			p.apply(key, args, at)
		}
	}
}

func (p *sshParser) apply(key string, args []string, at map[string]any) {
	switch key {
	case "hostname":
		p.cur.Hostname = args[0]
	case "user":
		p.cur.User = args[0]
	case "port":
		if n, err := strconv.Atoi(args[0]); err == nil {
			p.cur.Port = n
		}
	case "localforward", "remoteforward":
		kind := tunnel.Local
		if key == "remoteforward" {
			kind = tunnel.Remote
		}
		if len(args) < 2 {
			p.res.warn("forwardInvalid", at)
			return
		}
		bindAddr, bindPort, ok1 := splitBind(args[0])
		destHost, destPort, ok2 := splitBind(args[1])
		if !ok1 || !ok2 {
			p.res.warn("forwardInvalid", at)
			return
		}
		p.cur.Tunnels = append(p.cur.Tunnels, tunnel.Spec{Kind: kind, BindAddr: bindAddr, BindPort: bindPort, DestHost: destHost, DestPort: destPort})
	case "dynamicforward":
		addr, port, ok := splitBind(args[0])
		if !ok {
			p.res.warn("forwardInvalid", at)
			return
		}
		p.cur.Tunnels = append(p.cur.Tunnels, tunnel.Spec{Kind: tunnel.Dynamic, BindAddr: addr, BindPort: port})
	case "proxyjump":
		if !strings.EqualFold(args[0], "none") {
			for _, hop := range strings.Split(strings.Join(args, ""), ",") {
				if hop = strings.TrimSpace(hop); hop != "" {
					p.cur.ProxyJump = append(p.cur.ProxyJump, hop)
				}
			}
		}
	case "proxycommand":
		if !strings.EqualFold(args[0], "none") {
			p.cur.ProxyCommand = strings.Join(args, " ")
		}
	case "setenv":
		for _, kv := range args {
			if strings.Contains(kv, "=") {
				p.cur.Environment = append(p.cur.Environment, kv)
			}
		}
	case "remotecommand":
		if !strings.EqualFold(args[0], "none") {
			p.cur.RemoteCommand = strings.Join(args, " ")
		}
	case "preferredauthentications":
		known := map[string]bool{"publickey": true, "keyboard-interactive": true, "password": true}
		p.cur.AuthMethods = nil
		for _, m := range strings.Split(strings.Join(args, ""), ",") {
			if m = strings.ToLower(strings.TrimSpace(m)); known[m] {
				p.cur.AuthMethods = append(p.cur.AuthMethods, m)
			}
		}
	case "identitiesonly":
		p.cur.IdentitiesOnly = strings.EqualFold(args[0], "yes")
	case "identityfile", "certificatefile", "forwardagent":
		at["directive"] = key
		at["host"] = strings.Join(p.aliases, " ")
		p.res.warn("directiveSkipped", at)
	}
}

// flush turns the current Host block into one host per alias.
func (p *sshParser) flush() {
	for _, a := range p.aliases {
		h := p.cur
		h.Name = a
		if h.Hostname == "" {
			h.Hostname = a
		}
		h.Tunnels = append([]tunnel.Spec(nil), p.cur.Tunnels...)
		h.ProxyJump = append([]string(nil), p.cur.ProxyJump...)
		h.Environment = append([]string(nil), p.cur.Environment...)
		p.res.Hosts = append(p.res.Hosts, h)
	}
	p.cur = Host{}
}

// splitDirective returns the lower-case keyword and its arguments, handling
// "Key value", "Key=value" and quoted arguments.
func splitDirective(line string) (string, []string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", nil
	}
	i := strings.IndexAny(line, " \t=")
	if i < 0 {
		return strings.ToLower(line), nil
	}
	key := strings.ToLower(line[:i])
	rest := strings.TrimLeft(line[i:], " \t")
	rest = strings.TrimPrefix(rest, "=")
	return key, fields(strings.TrimSpace(rest))
}

func fields(s string) []string {
	var out []string
	var cur strings.Builder
	quoted := false
	for _, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
		case (r == ' ' || r == '\t') && !quoted:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// splitBind parses "port", "addr:port" or "[v6]:port".
func splitBind(s string) (string, int, bool) {
	host, port := "", s
	if i := strings.LastIndex(s, ":"); i >= 0 {
		host, port = strings.Trim(s[:i], "[]"), s[i+1:]
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return "", 0, false
	}
	switch host {
	case "":
		host = "127.0.0.1"
	case "*":
		host = "0.0.0.0"
	}
	return host, n, true
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}
