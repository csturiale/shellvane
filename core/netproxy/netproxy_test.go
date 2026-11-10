package netproxy

import (
	"net/url"
	"path/filepath"
	"testing"

	"golang.org/x/net/http/httpproxy"

	"github.com/csturiale/shellvane/core/errs"
)

func must(t *testing.T, raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestParseScutil(t *testing.T) {
	out := `<dictionary> {
  ExceptionsList : <array> {
    0 : *.local
    1 : 169.254/16
    2 : *.corp.example
  }
  ExcludeSimpleHostnames : 1
  HTTPEnable : 1
  HTTPPort : 8080
  HTTPProxy : proxy.corp.example
  HTTPSEnable : 1
  HTTPSPort : 8443
  HTTPSProxy : proxy.corp.example
  ProxyAutoConfigEnable : 0
}`
	c := parseScutil(out)
	if c == nil || c.HTTPProxy != "http://proxy.corp.example:8080" || c.HTTPSProxy != "http://proxy.corp.example:8443" {
		t.Fatalf("config = %+v", c)
	}
	if c.NoProxy != ".local,169.254/16,.corp.example" {
		t.Fatalf("no proxy = %q", c.NoProxy)
	}
	if parseScutil("<dictionary> {\n  HTTPEnable : 0\n  ProxyAutoConfigEnable : 1\n}") != nil {
		t.Fatal("no static proxy must give nil")
	}
}

func TestParseWindows(t *testing.T) {
	c := parseWindows("proxy.corp:3128", "*.corp;10.*;<local>")
	if c.HTTPProxy != "http://proxy.corp:3128" || c.HTTPSProxy != "http://proxy.corp:3128" || c.NoProxy != ".corp,10.0.0.0/8" {
		t.Fatalf("simple = %+v", c)
	}
	c = parseWindows("http=web:80;https=secure:443;socks=s:1080", "")
	if c.HTTPProxy != "http://web:80" || c.HTTPSProxy != "http://secure:443" {
		t.Fatalf("per scheme = %+v", c)
	}
	c = parseWindows("socks=s:1080", "")
	if c.HTTPSProxy != "socks5://s:1080" {
		t.Fatalf("socks = %+v", c)
	}
	if parseWindows("", "") != nil {
		t.Fatal("empty")
	}
}

func TestModes(t *testing.T) {
	defer func() { envConfig = httpproxy.FromEnvironment; readSystem = systemProxy; Apply(Config{}) }()
	envConfig = func() *httpproxy.Config { return &httpproxy.Config{} }
	readSystem = func() *httpproxy.Config {
		return &httpproxy.Config{HTTPProxy: "http://sys:8080", HTTPSProxy: "http://sys:8080", NoProxy: ".corp"}
	}
	gh := must(t, "https://github.com/x")

	Apply(Config{Mode: ModeSystem})
	if p, _ := For(gh); p == nil || p.Host != "sys:8080" {
		t.Fatalf("system: %v", p)
	}
	if p, _ := For(must(t, "https://git.corp/x")); p != nil {
		t.Fatalf("exception: %v", p)
	}
	if p, _ := For(must(t, "http://127.0.0.1:9000/x")); p != nil {
		t.Fatal("loopback is direct")
	}
	if !contains(ChildEnv(), "HTTPS_PROXY=http://sys:8080") {
		t.Fatalf("git gets the system proxy: %v", ChildEnv())
	}

	Apply(Config{Mode: ModeManual, URL: "proxy.example:3128", NoProxy: "internal.example"})
	if p, _ := For(gh); p == nil || p.String() != "http://proxy.example:3128" {
		t.Fatalf("manual: %v", p)
	}
	if p, _ := For(must(t, "https://internal.example/x")); p != nil {
		t.Fatal("manual exception")
	}
	if env := ChildEnv(); !contains(env, "HTTPS_PROXY=http://proxy.example:3128") || !contains(env, "NO_PROXY=internal.example") {
		t.Fatalf("child processes get the manual proxy: %v", env)
	}
	if Describe(must(t, "https://github.com")) != "http://proxy.example:3128" {
		t.Fatal("describe")
	}

	Apply(Config{Mode: ModeNone})
	if p, _ := For(gh); p != nil {
		t.Fatal("none is direct")
	}

	// The environment wins over the system's proxy.
	envConfig = func() *httpproxy.Config { return &httpproxy.Config{HTTPSProxy: "http://env:1"} }
	Apply(Config{Mode: ModeSystem})
	if p, _ := For(gh); p == nil || p.Host != "env:1" {
		t.Fatalf("env: %v", p)
	}
	if ChildEnv() != nil {
		t.Fatal("children keep the environment's proxy")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestLoadSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proxy.json")
	if c := Load(path); c.Mode != ModeSystem {
		t.Fatalf("default = %+v", c)
	}
	if err := Save(path, Config{Mode: ModeManual, URL: "ftp://x"}); errs.CodeOf(err) != ErrInvalid {
		t.Fatalf("bad url: %v", err)
	}
	if err := Save(path, Config{Mode: ModeManual, URL: "http://user:pw@p:8080"}); err != nil {
		t.Fatal(err)
	}
	if c := Load(path); c.Mode != ModeManual || c.URL != "http://user:pw@p:8080" {
		t.Fatalf("loaded = %+v", c)
	}
}
