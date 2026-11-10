// Package netproxy decides how the app's own HTTP requests (update
// checks, the gallery, the plugin store, usage reports, WebDAV and S3
// sync, credential plugins) reach the network: through the system's proxy
// (the default), directly, or through a proxy set in Settings. Go reads
// only the proxy environment variables, not the proxy configured in macOS
// or Windows, which corporate networks often use.
package netproxy

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http/httpproxy"

	"github.com/csturiale/shellvane/core/errs"
)

// Modes.
const (
	ModeSystem = "system" // environment variables, else the system's proxy
	ModeNone   = "none"   // direct connections
	ModeManual = "manual" // the proxy in URL
)

// ErrInvalid: the manual proxy is not a proxy URL.
const ErrInvalid errs.Code = "proxy.invalid"

// Config is what Settings › Network saves.
type Config struct {
	Mode string `json:"mode"`
	// URL of the manual proxy: http://host:port, https://… or socks5://…;
	// a user and password may be included.
	URL string `json:"url"`
	// NoProxy lists hosts reached directly (comma separated, as NO_PROXY:
	// example.org, .corp, 10.0.0.0/8).
	NoProxy string `json:"noProxy"`
}

// Normalize fills the default mode and checks the manual proxy.
func (c Config) Normalize() (Config, error) {
	c.URL = strings.TrimSpace(c.URL)
	c.NoProxy = strings.TrimSpace(c.NoProxy)
	switch c.Mode {
	case "", ModeSystem:
		c.Mode = ModeSystem
	case ModeNone:
	case ModeManual:
		if _, err := parseProxy(c.URL); err != nil {
			return c, err
		}
	default:
		return c, errs.New(ErrInvalid, errs.Params{"url": c.Mode})
	}
	return c, nil
}

func parseProxy(raw string) (*url.URL, error) {
	if raw != "" && !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") {
		return nil, errs.New(ErrInvalid, errs.Params{"url": raw})
	}
	return u, nil
}

// File is where the configuration is kept, in the app's data folder.
func File(dataDir string) string { return filepath.Join(dataDir, "proxy.json") }

// Load reads the configuration; a missing or broken file gives the default.
func Load(path string) Config {
	var c Config
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &c)
	}
	c, err := c.Normalize()
	if err != nil {
		return Config{Mode: ModeSystem}
	}
	return c
}

// Save writes the configuration.
func Save(path string, c Config) error {
	c, err := c.Normalize()
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(path, data, 0o600)
}

var (
	mu      sync.Mutex
	current = Config{Mode: ModeSystem}
	// System proxy, read from the OS at most once a minute.
	sysAt  time.Time
	sysCfg *httpproxy.Config
	// readSystem reads the OS's proxy settings (tests replace it).
	readSystem = systemProxy
	// envConfig reads the proxy variables, at each request: those of the
	// login shell are added at startup, in the background.
	envConfig = httpproxy.FromEnvironment
)

// Apply makes c the configuration of every request from now on.
func Apply(c Config) {
	c, err := c.Normalize()
	if err != nil {
		c = Config{Mode: ModeSystem}
	}
	mu.Lock()
	current = c
	sysAt = time.Time{}
	mu.Unlock()
}

// Current returns the configuration in use.
func Current() Config {
	mu.Lock()
	defer mu.Unlock()
	return current
}

// Install makes http.DefaultTransport (and the transports cloned from it)
// use Proxy.
func Install() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.Proxy = Proxy
	}
}

// Proxy is an http.Transport's Proxy function for the configuration.
func Proxy(req *http.Request) (*url.URL, error) {
	return For(req.URL)
}

// For returns the proxy for a URL, nil for a direct connection.
func For(u *url.URL) (*url.URL, error) {
	if loopback(u.Hostname()) {
		return nil, nil
	}
	c := Current()
	switch c.Mode {
	case ModeNone:
		return nil, nil
	case ModeManual:
		p, err := parseProxy(c.URL)
		if err != nil {
			return nil, err
		}
		return (&httpproxy.Config{HTTPProxy: p.String(), HTTPSProxy: p.String(), NoProxy: c.NoProxy}).ProxyFunc()(u)
	}
	if env := envConfig(); hasProxy(env) {
		return env.ProxyFunc()(u)
	}
	if sys := cachedSystem(); sys != nil {
		return sys.ProxyFunc()(u)
	}
	return nil, nil
}

// Describe names the proxy used for u, "" for a direct connection.
func Describe(u *url.URL) string {
	p, err := For(u)
	if err != nil || p == nil {
		return ""
	}
	q := *p
	q.User = nil
	return q.String()
}

func hasProxy(c *httpproxy.Config) bool { return c != nil && (c.HTTPProxy != "" || c.HTTPSProxy != "") }

func cachedSystem() *httpproxy.Config {
	mu.Lock()
	defer mu.Unlock()
	if time.Since(sysAt) > time.Minute {
		sysCfg = readSystem()
		sysAt = time.Now()
	}
	return sysCfg
}

func loopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ChildEnv returns the proxy variables for the app's child processes
// that reach the network (git for team and sync storage, kubectl), which
// read only the environment: the manual proxy, none, or the system's proxy
// when the environment has none. Nil when they can keep the app's own.
func ChildEnv() []string {
	c := Current()
	set := func(httpP, httpsP, no string) []string {
		return []string{"HTTP_PROXY=" + httpP, "http_proxy=" + httpP, "HTTPS_PROXY=" + httpsP, "https_proxy=" + httpsP, "NO_PROXY=" + no, "no_proxy=" + no}
	}
	switch c.Mode {
	case ModeManual:
		p, err := parseProxy(c.URL)
		if err != nil {
			return nil
		}
		return set(p.String(), p.String(), c.NoProxy)
	case ModeNone:
		return set("", "", "")
	}
	if hasProxy(envConfig()) {
		return nil
	}
	if sys := cachedSystem(); sys != nil {
		return set(sys.HTTPProxy, sys.HTTPSProxy, sys.NoProxy)
	}
	return nil
}
