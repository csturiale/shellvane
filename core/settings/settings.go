// Package settings implements inheritable connection settings.
//
// Settings flow from groups down to hosts, like ssh_config: the most specific
// level that sets a field wins. A nil field means "not set here, inherit";
// a non-nil field is an explicit value, even if it is the zero value. Slices
// follow the same rule: nil inherits, an empty non-nil slice overrides with
// "nothing".
package settings

import (
	"reflect"
	"strings"
)

// Settings is the set of inheritable options. Every field must be a pointer
// or a slice so that "unset" can be told apart from "set to zero".
type Settings struct {
	Port       *int    `json:"port,omitempty"`
	User       *string `json:"user,omitempty"`
	IdentityID *string `json:"identityId,omitempty"`
	// IdentitiesOnly offers only IdentityID among the keys: not the agent's
	// nor the default key files (OpenSSH's IdentitiesOnly).
	IdentitiesOnly *bool `json:"identitiesOnly,omitempty"`
	// AuthMethods lists the authentication methods in order of preference
	// ("publickey", "keyboard-interactive", "password"); methods left out
	// are not used (OpenSSH's PreferredAuthentications).
	AuthMethods  []string `json:"authMethods,omitempty"`
	JumpChain    []string `json:"jumpChain,omitempty"`    // host IDs, first hop first
	Proxy        *string  `json:"proxy,omitempty"`        // socks5:// or http:// URL
	ProxyCommand *string  `json:"proxyCommand,omitempty"` // OpenSSH syntax, %h %p %r
	// Transport reaches the host through a cloud access service instead of
	// a direct connection: "awsSsm" (Session Manager), "azureBastion" or
	// "gcpIap" (Identity-Aware Proxy), with the vendor's CLI. CloudTarget is
	// the instance (ID, VM resource ID or name; the hostname when empty),
	// CloudRegion the AWS region or GCP zone, CloudAccount the AWS profile,
	// Azure subscription or GCP project, CloudBastion and CloudGroup the
	// Azure Bastion and its resource group.
	// Kerberos tries Active Directory / Kerberos single sign-on first
	// (gssapi-with-mic), with KerberosRealm and KerberosPrincipal when the
	// user name and the host's domain do not tell them.
	Kerberos          *bool   `json:"kerberos,omitempty"`
	KerberosRealm     *string `json:"kerberosRealm,omitempty"`
	KerberosPrincipal *string `json:"kerberosPrincipal,omitempty"`
	Transport         *string `json:"transport,omitempty"`
	CloudTarget       *string `json:"cloudTarget,omitempty"`
	CloudRegion       *string `json:"cloudRegion,omitempty"`
	CloudAccount      *string `json:"cloudAccount,omitempty"`
	CloudBastion      *string `json:"cloudBastion,omitempty"`
	CloudGroup        *string `json:"cloudGroup,omitempty"`
	// PasswordCommand prints the password (password managers: op, bw,
	// keepassxc-cli, pass...). CertificateCommand prints a short-lived
	// certificate for the host's key (Vault SSH, step-ca, Teleport); the
	// public key is in $SHELLVANE_PUBLIC_KEY and $SHELLVANE_PUBLIC_KEY_FILE.
	PasswordCommand    *string `json:"passwordCommand,omitempty"`
	CertificateCommand *string `json:"certificateCommand,omitempty"`
	// PreConnectCommand runs on this computer before every connection, with
	// the same variables (e.g. EC2 Instance Connect pushing the host's key);
	// connecting stops if it fails.
	PreConnectCommand *string `json:"preConnectCommand,omitempty"`
	// CredentialPlugin is the plugin configuration (ID) that brokers the
	// login: HashiCorp Vault, Teleport, Boundary or an external plugin.
	CredentialPlugin  *string `json:"credentialPlugin,omitempty"`
	KeepAliveSeconds  *int    `json:"keepAliveSeconds,omitempty"`
	ConnectTimeoutSec *int    `json:"connectTimeoutSec,omitempty"`
	ForwardAgent      *bool   `json:"forwardAgent,omitempty"`
	// ForwardX11 shows X11 programs started on the server on this computer.
	ForwardX11       *bool    `json:"forwardX11,omitempty"`
	LegacyAlgorithms *bool    `json:"legacyAlgorithms,omitempty"`
	TermType         *string  `json:"termType,omitempty"`
	Encoding         *string  `json:"encoding,omitempty"`
	StartupCommands  []string `json:"startupCommands,omitempty"`
	TerminalThemeID  *string  `json:"terminalThemeId,omitempty"`
	FontSize         *int     `json:"fontSize,omitempty"`
	LogSession       *bool    `json:"logSession,omitempty"`
	// Multiplexer ("tmux" or "screen") wraps the shell in a named session
	// that survives disconnections; reconnecting attaches to it again.
	Multiplexer *string `json:"multiplexer,omitempty"`
	// AutoReconnect reopens a session whose connection dropped.
	AutoReconnect *bool `json:"autoReconnect,omitempty"`
	// KubeContext and KubeNamespace give the host's panes their own kubectl
	// context (and namespace) on the server.
	KubeContext   *string `json:"kubeContext,omitempty"`
	KubeNamespace *string `json:"kubeNamespace,omitempty"`
	// LogFormat is "raw", "text" or "timestamped" for LogSession;
	// RecordSession records sessions in asciinema format.
	LogFormat     *string `json:"logFormat,omitempty"`
	RecordSession *bool   `json:"recordSession,omitempty"`
	// InitialDirectory is where the shell starts ("~/x" is under home).
	InitialDirectory *string `json:"initialDirectory,omitempty"`
	// Environment holds NAME=value lines exported before the shell starts
	// (set by Shellvane's launcher, so the server's AcceptEnv does not apply).
	Environment []string `json:"environment,omitempty"`
	// RemoteCommand runs instead of the interactive shell (OpenSSH's
	// RemoteCommand); the pane ends when it exits.
	RemoteCommand *string `json:"remoteCommand,omitempty"`
	// MACAddress wakes the host with Wake-on-LAN; WakeOnConnect sends the
	// packet before connecting and waits for the host to answer.
	MACAddress    *string `json:"macAddress,omitempty"`
	WakeBroadcast *string `json:"wakeBroadcast,omitempty"`
	WakeOnConnect *bool   `json:"wakeOnConnect,omitempty"`
	// KnockSequence ("7000,8000/udp,9000") is knocked before connecting.
	KnockSequence *string `json:"knockSequence,omitempty"`
	// RemoteDesktop ("rdp" or "vnc") opens the host's desktop with the
	// system's client, on RemoteDesktopPort, through an SSH forward when
	// RemoteDesktopViaSSH is set.
	RemoteDesktop       *string `json:"remoteDesktop,omitempty"`
	RemoteDesktopPort   *int    `json:"remoteDesktopPort,omitempty"`
	RemoteDesktopViaSSH *bool   `json:"remoteDesktopViaSsh,omitempty"`
}

// Level is one step of the inheritance chain.
type Level struct {
	ID       string // host or group ID; empty for built-in defaults
	Settings Settings
}

// Resolved is the effective result plus where each field came from.
type Resolved struct {
	Settings Settings `json:"settings"`
	// Source maps a field's JSON name to the ID of the level that set it.
	// Fields set by nobody are absent; built-in defaults have an empty ID.
	Source map[string]string `json:"source"`
}

// Resolve merges the chain, ordered from most specific (the host) to least
// specific (the built-in defaults).
func Resolve(chain ...Level) Resolved {
	out := Resolved{Source: map[string]string{}}
	dst := reflect.ValueOf(&out.Settings).Elem()
	t := dst.Type()

	for i := 0; i < t.NumField(); i++ {
		name := jsonName(t.Field(i))
		for _, lvl := range chain {
			v := reflect.ValueOf(lvl.Settings).Field(i)
			if v.IsNil() {
				continue
			}
			dst.Field(i).Set(v)
			out.Source[name] = lvl.ID
			break
		}
	}
	return out
}

func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	if name == "" {
		return f.Name
	}
	return name
}

// Defaults are the built-in values at the bottom of every chain.
func Defaults() Level {
	return Level{Settings: Settings{
		Port:              ptr(22),
		KeepAliveSeconds:  ptr(30),
		ConnectTimeoutSec: ptr(15),
		ForwardAgent:      ptr(false),
		LegacyAlgorithms:  ptr(false),
		TermType:          ptr("xterm-256color"),
		Encoding:          ptr("UTF-8"),
		LogSession:        ptr(false),
		LogFormat:         ptr("text"),
		RecordSession:     ptr(false),
		// FontSize and TerminalThemeID stay unset: the app preference applies
		// unless a group or host overrides it.
	}}
}

func ptr[T any](v T) *T { return &v }
