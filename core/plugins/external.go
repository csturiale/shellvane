package plugins

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Protocol is the version of the JSON-RPC protocol external plugins speak.
const Protocol = 1

// ManifestFile is the manifest of an external plugin, in its folder.
const ManifestFile = "plugin.json"

// CallTimeout bounds one call to an external plugin.
const CallTimeout = 2 * time.Minute

// Error codes of external plugins.
const (
	ErrManifest errs.Code = "plugins.manifest"
	ErrCrashed  errs.Code = "plugins.crashed"
	ErrRemote   errs.Code = "plugins.remote" // the plugin answered an error
)

var validID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)

// External is a plugin run as a program speaking JSON-RPC 2.0 on stdin and
// stdout, one message per line (docs/adr/0003-plugins.md). Shellvane starts
// it for each call: it answers, may ask Shellvane to run commands on the
// host (host.exec) meanwhile, and exits.
type External struct {
	manifest Manifest
	dir      string
}

// LoadExternal reads the plugin in dir.
func LoadExternal(dir string) (*External, error) {
	m, err := ReadManifest(dir)
	if err != nil {
		return nil, err
	}
	return &External{manifest: m, dir: dir}, nil
}

// ReadManifest reads and checks dir/plugin.json.
func ReadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return Manifest{}, errs.Wrap(err, ErrManifest, errs.Params{"detail": "plugin.json"})
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, errs.Wrap(err, ErrManifest, errs.Params{"detail": err.Error()})
	}
	bad := func(detail string) (Manifest, error) {
		return Manifest{}, errs.New(ErrManifest, errs.Params{"detail": detail})
	}
	switch {
	case !validID.MatchString(m.ID):
		return bad("id")
	case strings.TrimSpace(m.Name) == "":
		return bad("name")
	case m.Protocol != Protocol:
		return bad("protocol")
	case len(m.Command) == 0 || strings.TrimSpace(m.Command[0]) == "":
		return bad("command")
	case len(m.Capabilities) == 0:
		return bad("capabilities")
	}
	for _, c := range m.Capabilities {
		if c != CapInventory && c != CapCredentials && c != CapPanel && c != CapActions {
			return bad("capabilities: " + c)
		}
	}
	m.BuiltIn = false
	return m, nil
}

// LoadAll loads every plugin folder in root; broken ones are returned as
// errors by folder name, the others are loaded.
func LoadAll(root string) ([]*External, map[string]error) {
	entries, _ := os.ReadDir(root)
	var out []*External
	failed := map[string]error{}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		p, err := LoadExternal(filepath.Join(root, e.Name()))
		if err != nil {
			failed[e.Name()] = err
			continue
		}
		out = append(out, p)
	}
	return out, failed
}

// Manifest describes the plugin.
func (p *External) Manifest() Manifest { return p.manifest }

// Dir is the plugin's folder.
func (p *External) Dir() string { return p.dir }

// Inventory calls inventory.list.
func (p *External) Inventory(ctx context.Context, settings map[string]string) ([]Host, error) {
	var res struct {
		Hosts []Host `json:"hosts"`
	}
	err := p.call(ctx, CapInventory, "inventory.list", map[string]any{"settings": settings}, nil, &res)
	return res.Hosts, err
}

// Credentials calls credentials.get.
func (p *External) Credentials(ctx context.Context, settings map[string]string, t Target) (Credential, error) {
	var res Credential
	err := p.call(ctx, CapCredentials, "credentials.get", map[string]any{"settings": settings, "target": t}, nil, &res)
	return res, err
}

// Panel calls panel.render.
func (p *External) Panel(ctx context.Context, settings map[string]string, t Target, exec Exec) (Panel, error) {
	var res Panel
	err := p.call(ctx, CapPanel, "panel.render", map[string]any{"settings": settings, "target": t}, exec, &res)
	return res, err
}

// Actions calls actions.list.
func (p *External) Actions(ctx context.Context, settings map[string]string, t Target) ([]Action, error) {
	var res struct {
		Actions []Action `json:"actions"`
	}
	err := p.call(ctx, CapActions, "actions.list", map[string]any{"settings": settings, "target": t}, nil, &res)
	return res.Actions, err
}

// Run calls actions.run.
func (p *External) Run(ctx context.Context, settings map[string]string, t Target, action, item string, exec Exec) (Result, error) {
	var res Result
	err := p.call(ctx, CapActions, "actions.run", map[string]any{"settings": settings, "target": t, "action": action, "item": item}, exec, &res)
	return res, err
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// call starts the plugin, sends one request and serves its host.exec
// requests until it answers.
func (p *External) call(ctx context.Context, capability, method string, params any, hostExec Exec, into any) error {
	if !p.manifest.Has(capability) {
		return errs.New(ErrUnsupported, nil)
	}
	ctx, cancel := context.WithTimeout(ctx, CallTimeout)
	defer cancel()
	argv := append([]string(nil), p.manifest.Command...)
	if exe := filepath.Join(p.dir, filepath.FromSlash(argv[0])); fileExists(exe) {
		argv[0] = exe // a program shipped in the plugin's folder
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = p.dir
	cmd.Env = append(os.Environ(), "SHELLVANE_PLUGIN_PROTOCOL=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr tailBuffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return errs.Wrap(err, ErrCrashed, errs.Params{"id": p.manifest.ID, "detail": err.Error()})
	}
	defer func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()
	enc := json.NewEncoder(stdin)
	raw, _ := json.Marshal(params)
	if err := enc.Encode(rpcMessage{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: method, Params: raw}); err != nil {
		return errs.Wrap(err, ErrCrashed, errs.Params{"id": p.manifest.ID, "detail": err.Error()})
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 32<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return errs.Wrap(err, ErrResponse, errs.Params{"command": p.manifest.ID})
		}
		if msg.Method != "" {
			// A request from the plugin.
			enc.Encode(p.serve(ctx, msg, hostExec))
			continue
		}
		if string(msg.ID) != "1" {
			continue
		}
		if msg.Error != nil {
			return errs.New(ErrRemote, errs.Params{"id": p.manifest.ID, "detail": msg.Error.Message})
		}
		if into != nil && len(msg.Result) > 0 {
			if err := json.Unmarshal(msg.Result, into); err != nil {
				return errs.Wrap(err, ErrResponse, errs.Params{"command": p.manifest.ID})
			}
		}
		return nil
	}
	if ctx.Err() != nil {
		return errs.Wrap(ctx.Err(), ErrCrashed, errs.Params{"id": p.manifest.ID, "detail": "timeout"})
	}
	return errs.New(ErrCrashed, errs.Params{"id": p.manifest.ID, "detail": stderr.last()})
}

// serve answers a request of the plugin: host.exec runs a command on the
// host of the panel or action, over Shellvane's connection.
func (p *External) serve(ctx context.Context, msg rpcMessage, hostExec Exec) rpcMessage {
	reply := rpcMessage{JSONRPC: "2.0", ID: msg.ID}
	if msg.Method != "host.exec" || hostExec == nil {
		reply.Error = &rpcError{Code: -32601, Message: "method not available: " + msg.Method}
		return reply
	}
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil || strings.TrimSpace(params.Command) == "" {
		reply.Error = &rpcError{Code: -32602, Message: "command required"}
		return reply
	}
	out, code, err := hostExec(ctx, params.Command)
	if err != nil {
		reply.Error = &rpcError{Code: -32000, Message: err.Error()}
		return reply
	}
	reply.Result, _ = json.Marshal(map[string]any{"output": out, "code": code})
	return reply
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// tailBuffer keeps the last 4 KB of a stream.
type tailBuffer struct{ b []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if len(t.b) > 4096 {
		t.b = t.b[len(t.b)-4096:]
	}
	return len(p), nil
}

func (t *tailBuffer) last() string {
	lines := strings.Split(strings.TrimSpace(string(t.b)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

var _ io.Writer = (*tailBuffer)(nil)
