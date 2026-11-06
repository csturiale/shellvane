// Package importers reads hosts from other tools: OpenSSH config files,
// PuTTY and KiTTY sessions. Importers never write anything; the result is
// previewed by the user and then saved by the caller.
//
// Anything that cannot be imported is reported as a Warning, never dropped
// silently.
package importers

import (
	"github.com/csturiale/shellvane/core/tunnel"
)

// Host is an imported connection.
type Host struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"` // 0 when not set (inherit)
	User     string `json:"user"`
	// Group is the folder path in the source tool, if any.
	Group   []string      `json:"group"`
	Tunnels []tunnel.Spec `json:"tunnels"`
	// ProxyJump lists jump hosts as written in the source: aliases of other
	// imported hosts or [user@]host[:port].
	ProxyJump    []string `json:"proxyJump"`
	ProxyCommand string   `json:"proxyCommand"`
	// Environment (NAME=value) and RemoteCommand come from SetEnv and
	// RemoteCommand.
	Environment   []string `json:"environment"`
	RemoteCommand string   `json:"remoteCommand"`
	// AuthMethods and IdentitiesOnly come from PreferredAuthentications and
	// IdentitiesOnly.
	AuthMethods    []string `json:"authMethods"`
	IdentitiesOnly bool     `json:"identitiesOnly"`
}

// Warning is something that was not imported. Code is a translation key
// under "import.warnings."; Params fill it in.
type Warning struct {
	Code   string         `json:"code"`
	Params map[string]any `json:"params"`
}

// Result is the outcome of reading a source.
type Result struct {
	Hosts    []Host    `json:"hosts"`
	Warnings []Warning `json:"warnings"`
}

func (r *Result) warn(code string, params map[string]any) {
	r.Warnings = append(r.Warnings, Warning{Code: code, Params: params})
}
