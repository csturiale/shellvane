package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/kube"
	"github.com/csturiale/shellvane/core/netproxy"
	"github.com/csturiale/shellvane/core/shellenv"
)

// EventKubeForwards tells the UI that the Kubernetes port-forwards changed.
const EventKubeForwards = "kube:forwards"

// KubeTarget is a service or a running pod that can be forwarded to.
type KubeTarget struct {
	Kind  string `json:"kind"` // "service" or "pod"
	Name  string `json:"name"`
	Ports []int  `json:"ports"`
}

// KubeForwardSpec is what to forward: a target of a cluster (the
// selection carries the context, namespace and the environment's route).
type KubeForwardSpec struct {
	Selection  kube.Selection `json:"selection"`
	Gcloud     string         `json:"gcloud"` // the environment's gcloud profile, "" for none
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	RemotePort int            `json:"remotePort"`
	LocalPort  int            `json:"localPort"` // 0: a free one
	Label      string         `json:"label"`     // e.g. the environment's name
}

// KubeForward is a running port-forward as the UI shows it.
type KubeForward struct {
	ID         string `json:"id"`
	Context    string `json:"context"`
	Namespace  string `json:"namespace"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	RemotePort int    `json:"remotePort"`
	LocalPort  int    `json:"localPort"`
	Label      string `json:"label"`
	// State is "starting", "running" or "restarting" (kubectl ended, e.g.
	// the pod was replaced: it starts again by itself).
	State    string `json:"state"`
	Error    string `json:"error"`
	Restarts int    `json:"restarts"`
}

// KubeForwardService runs `kubectl port-forward` for clusters reached as
// the environments reach them, starting it again when it ends until it is
// stopped. Forwards last while the app runs.
type KubeForwardService struct {
	dataDir string
	emit    Emitter

	mu   sync.Mutex
	fwds map[string]*kubeForward
}

type kubeForward struct {
	view   KubeForward
	cancel context.CancelFunc
	file   string
}

// NewKubeForwardService returns the service; dataDir holds the kubeconfig
// overlays it writes.
func NewKubeForwardService(dataDir string, emit Emitter) *KubeForwardService {
	if emit == nil {
		emit = func(string, any) {}
	}
	return &KubeForwardService{dataDir: dataDir, emit: emit, fwds: map[string]*kubeForward{}}
}

// Targets lists the services and running pods of the selection's
// namespace with their ports.
func (s *KubeForwardService) Targets(ctx context.Context, sel kube.Selection, gcloud string) ([]KubeTarget, error) {
	file, env, err := s.prepare(ctx, sel, gcloud, "list-"+uuid.NewString())
	if err != nil {
		return nil, err
	}
	defer os.Remove(file)
	shellenv.Wait(ctx)
	ctx, cancel := context.WithTimeout(ctx, kubectlTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "get", "services,pods", "-o", "json")
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, errs.Wrap(err, kube.ErrNotInstalled, nil)
		}
		return nil, errs.Wrap(err, kube.ErrKubectl, errs.Params{"detail": orString(firstLine(stderr.String()), err.Error())})
	}
	return parseTargets(out)
}

// parseTargets reads `kubectl get services,pods -o json`: services with
// their ports, pods that run with the ports their containers declare.
func parseTargets(out []byte) ([]KubeTarget, error) {
	var list struct {
		Items []struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Ports []struct {
					Port int `json:"port"`
				} `json:"ports"`
				Containers []struct {
					Ports []struct {
						ContainerPort int `json:"containerPort"`
					} `json:"ports"`
				} `json:"containers"`
			} `json:"spec"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, errs.Wrap(err, kube.ErrKubectl, errs.Params{"detail": "unexpected kubectl output"})
	}
	targets := []KubeTarget{}
	for _, it := range list.Items {
		t := KubeTarget{Name: it.Metadata.Name, Ports: []int{}}
		switch it.Kind {
		case "Service":
			t.Kind = "service"
			for _, p := range it.Spec.Ports {
				t.Ports = append(t.Ports, p.Port)
			}
		case "Pod":
			if it.Status.Phase != "Running" {
				continue
			}
			t.Kind = "pod"
			for _, c := range it.Spec.Containers {
				for _, p := range c.Ports {
					t.Ports = append(t.Ports, p.ContainerPort)
				}
			}
		default:
			continue
		}
		targets = append(targets, t)
	}
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].Kind != targets[j].Kind {
			return targets[i].Kind == "service" // services first: they outlive pods
		}
		return targets[i].Name < targets[j].Name
	})
	return targets, nil
}

// Start forwards a local port to the target and keeps it up.
func (s *KubeForwardService) Start(ctx context.Context, spec KubeForwardSpec) (KubeForward, error) {
	if spec.Selection.Context == "" || spec.Name == "" || spec.RemotePort <= 0 || (spec.Kind != "service" && spec.Kind != "pod") {
		return KubeForward{}, errs.New(kube.ErrKubectl, errs.Params{"detail": "choose a context, a service or pod and its port"})
	}
	if spec.LocalPort == 0 {
		p, err := freePort()
		if err != nil {
			return KubeForward{}, errs.Wrap(err, kube.ErrKubectl, errs.Params{"detail": err.Error()})
		}
		spec.LocalPort = p
	}
	id := uuid.NewString()
	file, env, err := s.prepare(ctx, spec.Selection, spec.Gcloud, "forward-"+id)
	if err != nil {
		return KubeForward{}, err
	}
	run, cancel := context.WithCancel(context.Background())
	f := &kubeForward{cancel: cancel, file: file, view: KubeForward{
		ID: id, Context: spec.Selection.Context, Namespace: spec.Selection.Namespace, Kind: spec.Kind, Name: spec.Name,
		RemotePort: spec.RemotePort, LocalPort: spec.LocalPort, Label: spec.Label, State: "starting",
	}}
	s.mu.Lock()
	s.fwds[id] = f
	s.mu.Unlock()
	s.changed()
	go s.supervise(run, f, env)
	return f.view, nil
}

// Stop ends a forward.
func (s *KubeForwardService) Stop(id string) {
	s.mu.Lock()
	f := s.fwds[id]
	delete(s.fwds, id)
	s.mu.Unlock()
	if f != nil {
		f.cancel()
		os.Remove(f.file)
		s.changed()
	}
}

// StopAll ends every forward (the app quits).
func (s *KubeForwardService) StopAll() {
	for _, f := range s.List() {
		s.Stop(f.ID)
	}
}

// List returns the forwards, by local port.
func (s *KubeForwardService) List() []KubeForward {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]KubeForward, 0, len(s.fwds))
	for _, f := range s.fwds {
		out = append(out, f.view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LocalPort < out[j].LocalPort })
	return out
}

func (s *KubeForwardService) changed() { s.emit(EventKubeForwards, s.List()) }

func (s *KubeForwardService) update(f *kubeForward, change func(v *KubeForward)) {
	s.mu.Lock()
	_, alive := s.fwds[f.view.ID]
	if alive {
		change(&f.view)
	}
	s.mu.Unlock()
	if alive {
		s.changed()
	}
}

// kubeForwardBackoff is the wait before kubectl starts again: 1, 2, 4…
// seconds, at most 30.
func kubeForwardBackoff(failures int) time.Duration {
	d := time.Second << min(failures, 5)
	return min(d, 30*time.Second)
}

// supervise runs kubectl port-forward until ctx ends, again after each
// exit (the pod behind it was replaced, the connection dropped).
func (s *KubeForwardService) supervise(ctx context.Context, f *kubeForward, env []string) {
	shellenv.Wait(ctx)
	failures := 0
	for ctx.Err() == nil {
		target := f.view.Kind + "/" + f.view.Name
		ports := fmt.Sprintf("%d:%d", f.view.LocalPort, f.view.RemotePort)
		cmd := exec.CommandContext(ctx, "kubectl", "port-forward", "--address", "127.0.0.1", target, ports)
		cmd.Env = env
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		stdout, _ := cmd.StdoutPipe()
		started := time.Now()
		if err := cmd.Start(); err != nil {
			msg := err.Error()
			if errors.Is(err, exec.ErrNotFound) {
				msg = "kubectl is not installed or not in PATH"
			}
			s.update(f, func(v *KubeForward) { v.State, v.Error = "restarting", msg })
		} else {
			sc := bufio.NewScanner(stdout)
			for sc.Scan() {
				if strings.HasPrefix(sc.Text(), "Forwarding from") {
					failures = 0
					s.update(f, func(v *KubeForward) { v.State, v.Error = "running", "" })
				}
			}
			cmd.Wait()
			if ctx.Err() != nil {
				return
			}
			msg := orString(lastLine(stderr.String()), "kubectl port-forward ended")
			s.update(f, func(v *KubeForward) { v.State, v.Error, v.Restarts = "restarting", msg, v.Restarts+1 })
		}
		if time.Since(started) > time.Minute {
			failures = 0 // it ran for a while: start again soon
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(kubeForwardBackoff(failures)):
		}
		failures++
	}
}

// prepare writes the overlay kubeconfig (context, namespace, route) and
// returns kubectl's environment.
func (s *KubeForwardService) prepare(ctx context.Context, sel kube.Selection, gcloud, name string) (string, []string, error) {
	dir := s.dataDir
	if dir == "" {
		dir, _ = os.UserConfigDir()
		dir = filepath.Join(dir, "Shellvane")
	}
	file := filepath.Join(dir, "kube", kube.SafeName(name)+".yaml")
	var def kube.ContextDef
	if sel.Namespace != "" || sel.Route != nil {
		var err error
		if def, err = kube.Lookup(ctx, localKubectl, sel.Context); err == nil && sel.Route != nil {
			def, err = kube.LookupCluster(ctx, localKubectl, def)
		}
		if err != nil {
			return "", nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(file, kube.Overlay(sel, def), 0o600); err != nil {
		return "", nil, err
	}
	env := append(append(os.Environ(), netproxy.ChildEnv()...), "KUBECONFIG="+kube.Env(file, kube.Original()))
	if gdir, err := gcloudDir(s.dataDir, gcloud); err != nil {
		os.Remove(file)
		return "", nil, err
	} else if gdir != "" {
		env = append(env, "CLOUDSDK_CONFIG="+gdir)
	}
	return file, env, nil
}

// freePort asks the system for a free port on 127.0.0.1.
func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

func orString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ServiceShutdown ends the forwards when the app quits, so no kubectl is
// left running.
func (s *KubeForwardService) ServiceShutdown() error {
	s.StopAll()
	return nil
}
