package kube

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOverlay(t *testing.T) {
	got := string(Overlay(Selection{Context: "prod-eu"}, ContextDef{}))
	if !strings.Contains(got, `current-context: "prod-eu"`) || strings.Contains(got, "contexts:") {
		t.Fatalf("overlay = %s", got)
	}
	got = string(Overlay(Selection{Context: `we"ird`, Namespace: "web"}, ContextDef{Cluster: "c1", User: "u1"}))
	for _, want := range []string{`current-context: "we\"ird"`, `- name: "we\"ird"`, `cluster: "c1"`, `user: "u1"`, `namespace: "web"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(string(Overlay(Selection{}, ContextDef{})), "current-context") {
		t.Fatal("an empty selection must not set a context")
	}
}

func TestParseContextsAndNames(t *testing.T) {
	if got := ParseContexts([]byte("prod\n\ndev\n")); strings.Join(got, ",") != "dev,prod" {
		t.Fatalf("contexts = %v", got)
	}
	if SafeName("shellvane-ab/../x") != "shellvane-abx" || SafeName("..") != "pane" {
		t.Fatal("SafeName")
	}
}

func TestLookup(t *testing.T) {
	run := func(_ context.Context, args ...string) ([]byte, error) {
		q := args[len(args)-1]
		switch {
		case strings.Contains(q, `"prod"`) && strings.HasSuffix(q, "cluster}"):
			return []byte("c-prod\n"), nil
		case strings.Contains(q, `"prod"`):
			return []byte("admin"), nil
		}
		return nil, nil
	}
	def, err := Lookup(context.Background(), run, "prod")
	if err != nil || def.Cluster != "c-prod" || def.User != "admin" {
		t.Fatalf("lookup = %+v, %v", def, err)
	}
	if _, err := Lookup(context.Background(), run, "nope"); err == nil {
		t.Fatal("unknown context accepted")
	}
}

// The remote script is run by a real shell against a fake kubectl, then
// the file it writes is merged by the same rules kubectl uses.
func TestRemoteSetup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell")
	}
	home := t.TempDir()
	bin := filepath.Join(home, "bin")
	os.MkdirAll(bin, 0o755)
	fake := "#!/bin/sh\ncase \"$*\" in *cluster*) echo c-prod;; *user*) echo admin;; esac\n"
	os.WriteFile(filepath.Join(bin, "kubectl"), []byte(fake), 0o755)

	script := RemoteSetup("shellvane-1", Selection{Context: "prod", Namespace: "web"}) + `printf '%s' "$KUBECONFIG"`
	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.Env = []string{"HOME=" + home, "PATH=" + bin + ":/usr/bin:/bin"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	file := filepath.Join(home, ".kube", "shellvane", "shellvane-1.yaml")
	if string(out) != file+":"+filepath.Join(home, ".kube", "config") {
		t.Fatalf("KUBECONFIG = %q", out)
	}
	data, _ := os.ReadFile(file)
	want := string(Overlay(Selection{Context: "prod", Namespace: "web"}, ContextDef{Cluster: "c-prod", User: "admin"}))
	if strings.TrimPrefix(string(data), "") != want {
		t.Fatalf("file:\n%s\nwant:\n%s", data, want)
	}
	if fi, _ := os.Stat(file); fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", fi.Mode())
	}

	// Switching without a namespace rewrites the same file.
	cmd = exec.Command("/bin/sh", "-c", RemoteWrite("shellvane-1", Selection{Context: "dev"}))
	cmd.Env = []string{"HOME=" + home, "PATH=" + bin + ":/usr/bin:/bin"}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	data, _ = os.ReadFile(file)
	if string(data) != string(Overlay(Selection{Context: "dev"}, ContextDef{})) {
		t.Fatalf("rewritten file:\n%s", data)
	}
}

func TestOverlayRoute(t *testing.T) {
	def := ContextDef{Cluster: "gke_p", User: "u", Server: "https://10.0.0.2", CAData: "Q0E="}
	got := string(Overlay(Selection{Context: "gke", Route: &Route{ProxyURL: "socks5://127.0.0.1:1080"}}, def))
	for _, want := range []string{`cluster: "shellvane-gke_p"`, `- name: "shellvane-gke_p"`, `server: "https://10.0.0.2"`, `certificate-authority-data: "Q0E="`, `proxy-url: "socks5://127.0.0.1:1080"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in\n%s", want, got)
		}
	}
	got = string(Overlay(Selection{Context: "gke", Namespace: "web", Route: &Route{Server: "https://127.0.0.1:6443"}}, def))
	for _, want := range []string{`server: "https://127.0.0.1:6443"`, `tls-server-name: "10.0.0.2"`, `namespace: "web"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in\n%s", want, got)
		}
	}
	if strings.Contains(got, "proxy-url") {
		t.Fatal("a forward needs no proxy")
	}
}

func TestLookupCluster(t *testing.T) {
	run := func(_ context.Context, args ...string) ([]byte, error) {
		q := args[len(args)-1]
		switch {
		case strings.HasSuffix(q, ".server}"):
			return []byte("https://10.0.0.2\n"), nil
		case strings.HasSuffix(q, "certificate-authority-data}"):
			return []byte("Q0E="), nil
		}
		return nil, nil
	}
	def, err := LookupCluster(context.Background(), run, ContextDef{Cluster: "gke_p", User: "u"})
	if err != nil || def.Server != "https://10.0.0.2" || def.CAData != "Q0E=" || def.User != "u" {
		t.Fatalf("def = %+v, %v", def, err)
	}
}
