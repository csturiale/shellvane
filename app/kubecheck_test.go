//go:build !windows

package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/kube"
)

func TestMissingContextSaysWhichOnesThereAre(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kubectl"), []byte("#!/bin/sh\nprintf 'pay-qat\\nshop-prd\\n'\n"), 0o755)
	t.Setenv("PATH", dir)
	env := os.Environ()
	err := missingContext(context.Background(), env, "shop-qat", "/tmp/pane.yaml"+string(os.PathListSeparator)+"/home/alice/.kube/config")
	if errs.CodeOf(err) != kube.ErrContextMissing {
		t.Fatalf("err = %v", err)
	}
	p := errs.ToPayload(err).Params
	if p["context"] != "shop-qat" || p["available"] != "pay-qat, shop-prd" || p["files"] != "/home/alice/.kube/config" {
		t.Fatalf("params = %v", p)
	}
	if missingContext(context.Background(), env, "shop-prd", "/tmp/pane.yaml") != nil {
		t.Fatal("a context that is there")
	}
}
