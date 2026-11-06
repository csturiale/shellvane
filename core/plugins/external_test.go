package plugins

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

// TestHelperPlugin is the fake external plugin, run as a child process.
func TestHelperPlugin(t *testing.T) {
	if os.Getenv("SHELLVANE_FAKE_PLUGIN") != "1" {
		t.Skip("helper process")
	}
	in := bufio.NewScanner(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	in.Scan()
	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params struct {
			Settings map[string]string `json:"settings"`
			Item     string            `json:"item"`
		} `json:"params"`
	}
	json.Unmarshal(in.Bytes(), &req)
	reply := func(result any) {
		out.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		os.Exit(0)
	}
	switch req.Method {
	case "inventory.list":
		reply(map[string]any{"hosts": []Host{{Key: "k1", Name: "from-" + req.Params.Settings["zone"], Hostname: "10.1.1.1", Group: []string{"z"}}}})
	case "panel.render":
		// Ask Shellvane to run a command on the host, then show its output.
		out.Encode(map[string]any{"jsonrpc": "2.0", "id": "x1", "method": "host.exec", "params": map[string]string{"command": "uptime"}})
		in.Scan()
		var res struct {
			Result struct {
				Output string `json:"output"`
				Code   int    `json:"code"`
			} `json:"result"`
		}
		json.Unmarshal(in.Bytes(), &res)
		reply(Panel{Title: "Fake", Text: fmt.Sprintf("%s (%d)", res.Result.Output, res.Result.Code)})
	case "actions.run":
		out.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": 1, "message": "no such item " + req.Params.Item}})
		os.Exit(0)
	}
	os.Exit(3) // crash
}

func writePlugin(t *testing.T, dir string, caps ...string) {
	t.Helper()
	m := Manifest{ID: "fake-cloud", Name: "Fake cloud", Version: "1.0.0", Protocol: 1, Capabilities: caps,
		Command: []string{os.Args[0], "-test.run=^TestHelperPlugin$"}}
	data, _ := json.Marshal(m)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, ManifestFile), data, 0o644)
}

func TestExternalPlugin(t *testing.T) {
	t.Setenv("SHELLVANE_FAKE_PLUGIN", "1")
	dir := filepath.Join(t.TempDir(), "fake")
	writePlugin(t, dir, CapInventory, CapPanel, CapActions)
	p, err := LoadExternal(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	hosts, err := p.Inventory(ctx, map[string]string{"zone": "eu"})
	if err != nil || len(hosts) != 1 || hosts[0].Name != "from-eu" {
		t.Fatalf("inventory = %+v, %v", hosts, err)
	}
	var ran string
	panel, err := p.Panel(ctx, nil, Target{}, func(_ context.Context, command string) (string, int, error) {
		ran = command
		return "up 3 days", 0, nil
	})
	if err != nil || panel.Text != "up 3 days (0)" || ran != "uptime" {
		t.Fatalf("panel = %+v, %v (ran %q)", panel, err, ran)
	}
	if _, err := p.Run(ctx, nil, Target{}, "x", "db", nil); errs.CodeOf(err) != ErrRemote {
		t.Fatalf("remote error: %v", err)
	}
	if _, err := p.Credentials(ctx, nil, Target{}); errs.CodeOf(err) != ErrUnsupported {
		t.Fatalf("undeclared capability: %v", err)
	}
	if _, err := p.Actions(ctx, nil, Target{}); errs.CodeOf(err) != ErrCrashed {
		t.Fatalf("crash: %v", err)
	}
}

func TestManifestChecks(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ManifestFile), []byte(`{"id":"Bad ID","name":"x","protocol":1,"command":["x"],"capabilities":["inventory"]}`), 0o644)
	if _, err := ReadManifest(dir); errs.CodeOf(err) != ErrManifest {
		t.Fatalf("bad id: %v", err)
	}
	os.WriteFile(filepath.Join(dir, ManifestFile), []byte(`{"id":"ok","name":"x","protocol":2,"command":["x"],"capabilities":["inventory"]}`), 0o644)
	if _, err := ReadManifest(dir); errs.CodeOf(err) != ErrManifest {
		t.Fatalf("protocol: %v", err)
	}
}

func zipPlugin(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	p := filepath.Join(t.TempDir(), "p.zip")
	f, _ := os.Create(p)
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
	data, _ := os.ReadFile(p)
	return data
}

func TestInstallFromStore(t *testing.T) {
	manifest := `{"id":"fake-cloud","name":"Fake","version":"2","protocol":1,"command":["./run.sh"],"capabilities":["inventory"]}`
	good := zipPlugin(t, map[string]string{"fake-cloud/plugin.json": manifest, "fake-cloud/run.sh": "#!/bin/sh\n"})
	evil := zipPlugin(t, map[string]string{"../escape.txt": "x", "plugin.json": manifest})
	sum := sha256.Sum256(good)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.json":
			fmt.Fprintf(w, `{"plugins":[{"id":"fake-cloud","name":"Fake","version":"2","url":"%s/good.zip","sha256":"%s"}]}`, "http://"+r.Host, hex.EncodeToString(sum[:]))
		case "/good.zip":
			w.Write(good)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	idx, err := FetchIndex(ctx, srv.URL+"/index.json")
	if err != nil || len(idx.Plugins) != 1 {
		t.Fatalf("index = %+v, %v", idx, err)
	}
	file, err := Download(ctx, idx.Plugins[0])
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file)
	root := t.TempDir()
	m, err := Install(file, root)
	if err != nil || m.ID != "fake-cloud" || !fileExists(filepath.Join(root, "fake-cloud", "run.sh")) {
		t.Fatalf("install = %+v, %v", m, err)
	}
	if list, failed := LoadAll(root); len(list) != 1 || len(failed) != 0 {
		t.Fatalf("load = %v, %v", list, failed)
	}

	tampered := idx.Plugins[0]
	tampered.SHA256 = hex.EncodeToString(make([]byte, 32))
	if _, err := Download(ctx, tampered); errs.CodeOf(err) != ErrChecksum {
		t.Fatalf("tampered package: %v", err)
	}
	if _, err := FetchIndex(ctx, "http://example.org/index.json"); errs.CodeOf(err) != ErrStore {
		t.Fatalf("plain http: %v", err)
	}
	evilFile := filepath.Join(t.TempDir(), "evil.zip")
	os.WriteFile(evilFile, evil, 0o644)
	if _, err := Install(evilFile, root); errs.CodeOf(err) != ErrPackage {
		t.Fatalf("zip slip: %v", err)
	}
	if err := Uninstall(root, "fake-cloud"); err != nil || fileExists(filepath.Join(root, "fake-cloud", ManifestFile)) {
		t.Fatalf("uninstall: %v", err)
	}
}
