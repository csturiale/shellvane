package app

import (
	"database/sql"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/automation"
	"github.com/csturiale/shellvane/core/environments"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/snippets"
	"github.com/csturiale/shellvane/core/tunnel"
	"github.com/csturiale/shellvane/core/vault"
	"github.com/csturiale/shellvane/core/workspaces"
)

// ExtensionDeps is what an extension may build its services on.
type ExtensionDeps struct {
	DB       *sql.DB
	Vault    *vault.Vault
	Sessions *SessionService
	DataDir  string
	Emit     Emitter
	// Stores an extension may read or write.
	Hosts        *hosts.Repo
	Environments *environments.Repo
	Tunnels      *tunnel.Repo
	Snippets     *snippets.Repo
	Workspaces   *workspaces.Repo
	Rules        *automation.Repo
	// AddBroker routes host credential references starting with prefix
	// (for example "team:") to b.
	AddBroker func(prefix string, b Broker)
	// Options are the service options every service must use (coded errors).
	Options application.ServiceOptions
}

// Extension adds services to the application. Features distributed apart
// from the community edition register one from an init function; the
// community edition simply has none.
type Extension struct {
	Name     string
	Services func(ExtensionDeps) []application.Service
}

var (
	extensionsMu sync.Mutex
	extensions   []Extension
)

// RegisterExtension adds an extension; call it from an init function.
func RegisterExtension(e Extension) {
	extensionsMu.Lock()
	defer extensionsMu.Unlock()
	extensions = append(extensions, e)
}

// ExtensionServices returns the services of every registered extension.
func ExtensionServices(d ExtensionDeps) []application.Service {
	extensionsMu.Lock()
	list := append([]Extension(nil), extensions...)
	extensionsMu.Unlock()
	var out []application.Service
	for _, e := range list {
		out = append(out, e.Services(d)...)
	}
	return out
}
