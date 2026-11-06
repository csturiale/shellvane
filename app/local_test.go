package app

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/kube"
)

func TestLocalPaneKubeFile(t *testing.T) {
	ctx := context.Background()
	svc := NewSessionService(nil, nil, nil, nil, func(string, any) {})
	dir := t.TempDir()
	EnableLocal(svc, dir)

	info, err := svc.OpenLocal(ctx, 80, 24, LocalOptions{Persist: "shellvane-t1", Kube: &kube.Selection{Context: "dev"}})
	if err != nil {
		t.Fatal(err)
	}
	if !info.Local || info.Kube == nil || !info.Kube.Live || info.Kube.Context != "dev" {
		t.Fatalf("info = %+v", info)
	}
	file := svc.kubeDir() + string(os.PathSeparator) + "shellvane-t1.yaml"
	if data, err := os.ReadFile(file); err != nil || !strings.Contains(string(data), `current-context: "dev"`) {
		t.Fatalf("file %q: %v", data, err)
	}
	if err := svc.SetKube(ctx, info.ID, kube.Selection{Context: "prod"}); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(file); !strings.Contains(string(data), `current-context: "prod"`) {
		t.Fatalf("not switched: %s", data)
	}
	svc.Close(info.ID)
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("the pane's file must be removed with the pane")
	}
}
