package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/kube"
)

// A pane restored before kubectl can be asked (not in PATH yet) still gets
// its own file, with the context: the environment's SetKube fills it later.
func TestLocalKubeFileEvenWhenTheOverlayFails(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no kubectl
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "none"))
	s := &SessionService{dataDir: t.TempDir()}
	file := filepath.Join(s.kubeDir(), "pane.yaml")
	sel := kube.Selection{Context: "shop-prd", Namespace: "argocd", Route: &kube.Route{ProxyURL: "socks5://127.0.0.1:10821"}}
	if got := s.ensureLocalKube(context.Background(), file, sel); got != file {
		t.Fatalf("file = %q", got)
	}
	b, _ := os.ReadFile(file)
	if !strings.Contains(string(b), "current-context: \"shop-prd\"") {
		t.Fatalf("overlay = %s", b)
	}
	// Nothing to look up: written in full at once.
	if got := s.ensureLocalKube(context.Background(), file, kube.Selection{Context: "shop-qat"}); got != file {
		t.Fatalf("file = %q", got)
	}
}
