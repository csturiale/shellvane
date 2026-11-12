package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/csturiale/shellvane/core/kube"
)

func TestParseKubeTargets(t *testing.T) {
	out := []byte(`{"items":[
	 {"kind":"Pod","metadata":{"name":"web-2"},"spec":{"containers":[{"ports":[{"containerPort":8080}]}]},"status":{"phase":"Running"}},
	 {"kind":"Pod","metadata":{"name":"job-1"},"spec":{"containers":[{}]},"status":{"phase":"Succeeded"}},
	 {"kind":"Service","metadata":{"name":"argocd-server"},"spec":{"ports":[{"port":80},{"port":443}]}}]}`)
	got, err := parseTargets(out)
	if err != nil || len(got) != 2 || got[0].Name != "argocd-server" || got[0].Kind != "service" || len(got[0].Ports) != 2 || got[1].Name != "web-2" || got[1].Ports[0] != 8080 {
		t.Fatalf("targets = %+v, %v", got, err)
	}
	if _, err := parseTargets([]byte("not json")); err == nil {
		t.Fatal("bad output must fail")
	}
}

func TestKubeForwardBackoff(t *testing.T) {
	for n, want := range map[int]time.Duration{0: time.Second, 1: 2 * time.Second, 4: 16 * time.Second, 5: 30 * time.Second, 9: 30 * time.Second} {
		if got := kubeForwardBackoff(n); got != want {
			t.Errorf("backoff(%d) = %v, want %v", n, got, want)
		}
	}
}

// A kubectl that forwards and then ends (the pod went away): the forward
// is running, then restarting, and starts again by itself.
func TestKubeForwardRestarts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script kubectl")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\necho \"$@\" >> " + filepath.Join(bin, "calls") + "\necho 'Forwarding from 127.0.0.1:1 -> 2'\nsleep 0.2\necho 'lost connection to pod' >&2\n"
	os.WriteFile(filepath.Join(bin, "kubectl"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("KUBECONFIG", filepath.Join(bin, "none"))

	var events int
	s := NewKubeForwardService(t.TempDir(), func(string, any) { events++ })
	f, err := s.Start(context.Background(), KubeForwardSpec{Selection: kube.Selection{Context: "shop-prd"}, Kind: "service", Name: "argocd-server", RemotePort: 443})
	if err != nil || f.LocalPort == 0 {
		t.Fatalf("start = %+v, %v", f, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var seen []string
	for time.Now().Before(deadline) {
		v := s.List()[0]
		if len(seen) == 0 || seen[len(seen)-1] != v.State {
			seen = append(seen, v.State)
		}
		if v.Restarts >= 1 && v.State == "running" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	v := s.List()[0]
	if v.Restarts < 1 || v.State != "running" || v.Error != "" {
		t.Fatalf("after a restart: %+v (states %v)", v, seen)
	}
	calls, _ := os.ReadFile(filepath.Join(bin, "calls"))
	if want := "port-forward --address 127.0.0.1 service/argocd-server "; len(calls) == 0 || string(calls[:len(want)]) != want {
		t.Fatalf("kubectl called with %q", calls)
	}
	s.Stop(f.ID)
	if len(s.List()) != 0 || events == 0 {
		t.Fatalf("stop: %v, events %d", s.List(), events)
	}
}
