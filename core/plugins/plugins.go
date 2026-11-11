// Package plugins connects Shellvane to the systems around servers: cloud
// inventories, credential brokers, views and actions on hosts. Built-in
// plugins and external ones (a program speaking JSON-RPC, see
// docs/adr/0003-plugins.md) share the Plugin interface.
package plugins

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes.
const (
	ErrUnknown     errs.Code = "plugins.unknown"
	ErrUnsupported errs.Code = "plugins.unsupported"
	ErrSetting     errs.Code = "plugins.setting"
	ErrCommand     errs.Code = "plugins.command"
	ErrResponse    errs.Code = "plugins.response"
)

// Capabilities.
const (
	CapInventory   = "inventory"
	CapCredentials = "credentials"
	CapPanel       = "panel"
	CapActions     = "actions"
)

// Setting is a field the user fills in for a plugin.
type Setting struct {
	Key         string `json:"key"`
	Label       string `json:"label"`       // translation key for built-ins, text for external plugins
	Placeholder string `json:"placeholder"` // an example value
	Secret      bool   `json:"secret"`      // kept in the vault
	Required    bool   `json:"required"`
}

// Manifest describes a plugin.
type Manifest struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Description  string    `json:"description"` // translation key for built-ins
	Capabilities []string  `json:"capabilities"`
	Settings     []Setting `json:"settings"`
	// Requires names the command-line tools the plugin runs (aws, kubectl…).
	Requires []string `json:"requires"`
	BuiltIn  bool     `json:"builtIn"`
	Protocol int      `json:"protocol,omitempty"` // external plugins: 1
	Command  []string `json:"command,omitempty"`  // external plugins: argv
}

// Has tells whether the plugin offers a capability.
func (m Manifest) Has(capability string) bool {
	for _, c := range m.Capabilities {
		if c == capability {
			return true
		}
	}
	return false
}

// Host is a server found by an inventory plugin.
type Host struct {
	// Key identifies the host in its source (instance ID, node name...), so
	// a refresh updates it instead of adding it again.
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Hostname string   `json:"hostname"`
	Port     int      `json:"port,omitempty"`
	User     string   `json:"user,omitempty"`
	Group    []string `json:"group,omitempty"` // path under the plugin's group
	Tags     []string `json:"tags,omitempty"`
	Notes    string   `json:"notes,omitempty"`
}

// Target is the host a credential, panel or action is for.
type Target struct {
	HostID   string `json:"hostId"`
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	// Safe, when set, replaces the configuration's safe or folder (an
	// environment's login secret may live elsewhere than the hosts').
	Safe string `json:"safe,omitempty"`
	// Purpose says what the secret is for when it is not an SSH login,
	// e.g. "login of environment prod"; brokers that log a reason use it.
	Purpose string `json:"purpose,omitempty"`
}

// Credential is what a broker gives for a login. Empty fields keep the
// host's own settings.
type Credential struct {
	Password    string `json:"password,omitempty"`
	PrivateKey  string `json:"privateKey,omitempty"`  // PEM
	Certificate string `json:"certificate,omitempty"` // OpenSSH certificate of PrivateKey
	// User and Port replace the host's (the broker's login, its node port).
	User string `json:"user,omitempty"`
	Port int    `json:"port,omitempty"`
	// ProxyCommand (OpenSSH syntax, %h %p %r) opens the connection through
	// the broker, e.g. `tsh proxy ssh` or `boundary connect`.
	ProxyCommand string `json:"proxyCommand,omitempty"`
}

// Table is a block of a panel.
type Table struct {
	Title   string     `json:"title"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	// Actions apply to one row: the row's first cell is passed as "item".
	Actions []Action `json:"actions,omitempty"`
}

// Panel is what a plugin shows for a connected host.
type Panel struct {
	Title  string  `json:"title"`
	Tables []Table `json:"tables"`
	Text   string  `json:"text,omitempty"` // free text (e.g. log lines)
}

// Action is a button a plugin adds.
type Action struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Confirm bool   `json:"confirm,omitempty"` // ask before running
}

// Result is the outcome of an action.
type Result struct {
	Output string `json:"output,omitempty"`
	// Command, when set, is typed in a new terminal on the host instead.
	Command string `json:"command,omitempty"`
}

// Exec runs a command on the target host over its connection and returns
// its output (stdout and stderr) and exit code.
type Exec func(ctx context.Context, command string) (output string, code int, err error)

// Plugin is implemented by built-in and external plugins. Methods of
// capabilities a plugin does not have return ErrUnsupported.
type Plugin interface {
	Manifest() Manifest
	Inventory(ctx context.Context, settings map[string]string) ([]Host, error)
	Credentials(ctx context.Context, settings map[string]string, t Target) (Credential, error)
	Panel(ctx context.Context, settings map[string]string, t Target, exec Exec) (Panel, error)
	Actions(ctx context.Context, settings map[string]string, t Target) ([]Action, error)
	Run(ctx context.Context, settings map[string]string, t Target, action, item string, exec Exec) (Result, error)
}

// Base answers ErrUnsupported for every capability; plugins embed it and
// implement theirs.
type Base struct{}

func unsupported() error { return errs.New(ErrUnsupported, nil) }

// Inventory is not offered.
func (Base) Inventory(context.Context, map[string]string) ([]Host, error) {
	return nil, unsupported()
}

// Credentials is not offered.
func (Base) Credentials(context.Context, map[string]string, Target) (Credential, error) {
	return Credential{}, unsupported()
}

// Panel is not offered.
func (Base) Panel(context.Context, map[string]string, Target, Exec) (Panel, error) {
	return Panel{}, unsupported()
}

// Actions is not offered.
func (Base) Actions(context.Context, map[string]string, Target) ([]Action, error) {
	return nil, unsupported()
}

// Run is not offered.
func (Base) Run(context.Context, map[string]string, Target, string, string, Exec) (Result, error) {
	return Result{}, unsupported()
}

// Registry holds the plugins Shellvane knows, by ID.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewRegistry returns a registry with the given plugins.
func NewRegistry(list ...Plugin) *Registry {
	r := &Registry{plugins: map[string]Plugin{}}
	for _, p := range list {
		r.Add(p)
	}
	return r
}

// Add registers a plugin; an external plugin with a built-in's ID replaces it.
func (r *Registry) Add(p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins[p.Manifest().ID] = p
}

// Remove forgets a plugin.
func (r *Registry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, id)
}

// Get returns a plugin.
func (r *Registry) Get(id string) (Plugin, error) {
	r.mu.RLock()
	p, ok := r.plugins[id]
	r.mu.RUnlock()
	if !ok {
		return nil, errs.New(ErrUnknown, errs.Params{"id": id})
	}
	return p, nil
}

// List returns the manifests, built-ins first, then by name.
func (r *Registry) List() []Manifest {
	r.mu.RLock()
	out := make([]Manifest, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p.Manifest())
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].BuiltIn != out[j].BuiltIn {
			return out[i].BuiltIn
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// Require checks the required settings.
func Require(settings map[string]string, keys ...string) error {
	for _, k := range keys {
		if strings.TrimSpace(settings[k]) == "" {
			return errs.New(ErrSetting, errs.Params{"key": k})
		}
	}
	return nil
}
