package app

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/csturiale/shellvane/core/netproxy"
	"github.com/csturiale/shellvane/core/version"
)

// NetworkService is Settings › Network: how the app's own requests reach
// the internet (the system's proxy, none, or one set by hand).
type NetworkService struct {
	path string
	mu   sync.Mutex
}

// NewNetworkService loads the saved configuration from dataDir and applies
// it to every request of the app.
func NewNetworkService(dataDir string) *NetworkService {
	s := &NetworkService{path: netproxy.File(dataDir)}
	netproxy.Install()
	netproxy.Apply(netproxy.Load(s.path))
	return s
}

// Proxy returns the saved configuration.
func (s *NetworkService) Proxy() netproxy.Config { return netproxy.Current() }

// SetProxy saves and applies a configuration.
func (s *NetworkService) SetProxy(c netproxy.Config) (netproxy.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := c.Normalize()
	if err != nil {
		return netproxy.Current(), err
	}
	if err := netproxy.Save(s.path, c); err != nil {
		return netproxy.Current(), err
	}
	netproxy.Apply(c)
	return c, nil
}

// ReachResult is the outcome of reaching one address.
type ReachResult struct {
	URL    string `json:"url"`
	Via    string `json:"via"`    // the proxy used, "" for a direct connection
	OK     bool   `json:"ok"`     // an HTTP answer came back
	Status int    `json:"status"` // its status code
	Detail string `json:"detail"` // why it failed
	// Blocked: the answer is a page of a filtering proxy (e.g. Zscaler)
	// refusing the site, not the site itself.
	Blocked bool `json:"blocked"`
}

// testTargets are what the app reaches: updates (GitHub releases and
// their downloads), the website (gallery, plugin store, usage reports).
var testTargets = []string{
	"https://github.com/" + version.PublicRepo + "/releases/latest/download/manifest.json",
	GalleryURL,
}

// Test reaches the addresses the app uses, with the current configuration.
func (s *NetworkService) Test(ctx context.Context) []ReachResult {
	client := &http.Client{Timeout: 20 * time.Second}
	out := make([]ReachResult, len(testTargets))
	var wg sync.WaitGroup
	for i, target := range testTargets {
		wg.Add(1)
		go func(i int, target string) {
			defer wg.Done()
			r := ReachResult{URL: target}
			if u, err := url.Parse(target); err == nil {
				r.Via = netproxy.Describe(u)
			}
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
			req.Header.Set("User-Agent", "Shellvane/"+version.Version)
			resp, err := client.Do(req)
			if err != nil {
				r.Detail = err.Error()
				out[i] = r
				return
			}
			resp.Body.Close()
			r.Status = resp.StatusCode
			r.OK = resp.StatusCode < 400
			r.Blocked = blockedBy(resp)
			if r.Blocked {
				r.OK = false
			}
			out[i] = r
		}(i, target)
	}
	wg.Wait()
	return out
}

// blockedBy recognizes a filtering proxy's refusal page.
func blockedBy(resp *http.Response) bool {
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusOK && resp.StatusCode != 307 && resp.StatusCode != 302 {
		return false
	}
	for _, h := range []string{"X-Zscaler-Error", "Zscaler-Error"} {
		if resp.Header.Get(h) != "" {
			return true
		}
	}
	loc := resp.Header.Get("Location")
	srv := resp.Header.Get("Server")
	for _, s := range []string{"zscaler", "gateway.zscloud", "bluecoat", "forcepoint", "websense", "netskope"} {
		if containsFold(loc, s) || containsFold(srv, s) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), sub)
}
