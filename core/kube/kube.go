// Package kube gives each terminal pane its own kubectl context without
// touching the shared kubeconfig: the pane's KUBECONFIG starts with a small
// file of its own (current-context, and a copy of the context with another
// namespace), followed by the user's kubeconfig. kubectl takes
// current-context and each context from the first file that sets them, so
// credentials are never copied. Rewriting the file switches the pane live.
package kube

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrKubectl         errs.Code = "kube.kubectlFailed"
	ErrUnknownContext  errs.Code = "kube.unknownContext"
	ErrOverlayDisabled errs.Code = "kube.notEnabled"
)

// Selection is a pane's context and namespace ("" = the context's own).
type Selection struct {
	Context   string `json:"context"`
	Namespace string `json:"namespace"`
	// Route reaches the cluster through a tunnel (local panes only).
	Route *Route `json:"route,omitempty"`
}

// Route sends kubectl through a tunnel: ProxyURL for a SOCKS tunnel
// (socks5://127.0.0.1:1080), or Server for a local forward to the API
// server (https://127.0.0.1:6443), which keeps the original TLS name.
type Route struct {
	ProxyURL string `json:"proxyUrl"`
	Server   string `json:"server"`
}

// Empty reports whether nothing is selected.
func (s Selection) Empty() bool { return s.Context == "" && s.Namespace == "" }

// ContextDef is what the overlay copies from the user's context: its
// cluster and user, and with a route the cluster's connection settings
// (public CA certificates, never credentials).
type ContextDef struct {
	Cluster string
	User    string

	Server        string
	CAData        string
	CAFile        string
	TLSServerName string
	Insecure      bool
}

// Overlay renders the pane's file. def is needed only with a namespace.
func Overlay(sel Selection, def ContextDef) []byte {
	var b strings.Builder
	b.WriteString("# Written by Shellvane for one terminal pane; safe to delete.\napiVersion: v1\nkind: Config\n")
	if sel.Context == "" {
		return []byte(b.String())
	}
	b.WriteString("current-context: " + quote(sel.Context) + "\n")
	if sel.Namespace == "" && sel.Route == nil {
		return []byte(b.String())
	}
	cluster := def.Cluster
	if sel.Route != nil {
		cluster = "shellvane-" + def.Cluster // a copy: the user's cluster is unchanged
	}
	b.WriteString("contexts:\n- name: " + quote(sel.Context) + "\n  context:\n")
	b.WriteString("    cluster: " + quote(cluster) + "\n")
	b.WriteString("    user: " + quote(def.User) + "\n")
	if sel.Namespace != "" {
		b.WriteString("    namespace: " + quote(sel.Namespace) + "\n")
	}
	if sel.Route != nil {
		writeCluster(&b, cluster, *sel.Route, def)
	}
	return []byte(b.String())
}

func writeCluster(b *strings.Builder, name string, r Route, def ContextDef) {
	b.WriteString("clusters:\n- name: " + quote(name) + "\n  cluster:\n")
	server, tlsName := def.Server, def.TLSServerName
	if r.Server != "" {
		server = r.Server
		if tlsName == "" {
			tlsName = hostOf(def.Server) // the certificate names the real host
		}
	}
	b.WriteString("    server: " + quote(server) + "\n")
	if def.CAData != "" {
		b.WriteString("    certificate-authority-data: " + quote(def.CAData) + "\n")
	}
	if def.CAFile != "" {
		b.WriteString("    certificate-authority: " + quote(def.CAFile) + "\n")
	}
	if tlsName != "" {
		b.WriteString("    tls-server-name: " + quote(tlsName) + "\n")
	}
	if def.Insecure {
		b.WriteString("    insecure-skip-tls-verify: true\n")
	}
	if r.ProxyURL != "" {
		b.WriteString("    proxy-url: " + quote(r.ProxyURL) + "\n")
	}
}

// hostOf returns the host name of an https:// URL.
func hostOf(server string) string {
	u, err := url.Parse(server)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// quote writes a YAML double-quoted scalar (JSON strings are valid ones).
func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ParseContexts reads `kubectl config get-contexts -o name`.
func ParseContexts(out []byte) []string {
	var list []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			list = append(list, line)
		}
	}
	sort.Strings(list)
	return list
}

// Original returns the user's KUBECONFIG: the environment variable, or
// ~/.kube/config.
func Original() string {
	if v := os.Getenv("KUBECONFIG"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
}

// Env is the KUBECONFIG value of a pane whose file is overlay.
func Env(overlay, original string) string {
	return overlay + string(os.PathListSeparator) + original
}

// Runner runs kubectl with arguments and returns its standard output.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// Lookup finds the cluster and user of a context, using the user's own
// kubeconfig (never the pane's file).
func Lookup(ctx context.Context, run Runner, name string) (ContextDef, error) {
	field := func(f string) (string, error) {
		out, err := run(ctx, "config", "view", "-o", `jsonpath={.contexts[?(@.name=="`+name+`")].context.`+f+`}`)
		return strings.TrimSpace(string(out)), err
	}
	cluster, err := field("cluster")
	if err != nil {
		return ContextDef{}, errs.Wrap(err, ErrKubectl, nil)
	}
	user, err := field("user")
	if err != nil {
		return ContextDef{}, errs.Wrap(err, ErrKubectl, nil)
	}
	if cluster == "" {
		return ContextDef{}, errs.New(ErrUnknownContext, errs.Params{"context": name})
	}
	return ContextDef{Cluster: cluster, User: user}, nil
}

// LookupCluster adds the cluster's connection settings to def, for a route.
func LookupCluster(ctx context.Context, run Runner, def ContextDef) (ContextDef, error) {
	field := func(f string) string {
		out, _ := run(ctx, "config", "view", "--raw", "-o", `jsonpath={.clusters[?(@.name=="`+def.Cluster+`")].cluster.`+f+`}`)
		return strings.TrimSpace(string(out))
	}
	def.Server = field("server")
	if def.Server == "" {
		return def, errs.New(ErrUnknownContext, errs.Params{"context": def.Cluster})
	}
	def.CAData = field("certificate-authority-data")
	def.CAFile = field("certificate-authority")
	def.TLSServerName = field("tls-server-name")
	def.Insecure = field("insecure-skip-tls-verify") == "true"
	return def, nil
}

// RemoteSetup returns POSIX shell commands that write the pane's file on a
// server and export KUBECONFIG; name identifies the pane. With a
// namespace, the context's cluster and user are read with kubectl there.
func RemoteSetup(name string, sel Selection) string {
	var b strings.Builder
	b.WriteString(`umask 077; _tk_orig="${KUBECONFIG:-$HOME/.kube/config}"; _tk_f="$HOME/.kube/shellvane/` + name + `.yaml"; mkdir -p "$HOME/.kube/shellvane"; `)
	b.WriteString(RemoteWrite(name, sel))
	b.WriteString(`export KUBECONFIG="$_tk_f:$_tk_orig"; `)
	return b.String()
}

// RemoteWrite returns commands that (re)write the pane's file; they expect
// $_tk_f and $_tk_orig, or set them when run alone.
func RemoteWrite(name string, sel Selection) string {
	var b strings.Builder
	b.WriteString(`: "${_tk_f:=$HOME/.kube/shellvane/` + name + `.yaml}"; : "${_tk_orig:=${KUBECONFIG:-$HOME/.kube/config}}"; `)
	if sel.Namespace == "" {
		b.WriteString("printf '%s' " + ShellQuote(string(Overlay(sel, ContextDef{}))) + ` > "$_tk_f"; `)
		return b.String()
	}
	ctxQ := ShellQuote(sel.Context)
	jp := func(f string) string {
		return `"$(KUBECONFIG="$_tk_orig" kubectl config view -o jsonpath='{.contexts[?(@.name=="'` + ctxQ + `'")].context.` + f + `}')"`
	}
	head := string(Overlay(Selection{Context: sel.Context}, ContextDef{}))
	b.WriteString(`{ printf '%s' ` + ShellQuote(head) + `; `)
	b.WriteString(`printf 'contexts:\n- name: %s\n  context:\n    cluster: "%s"\n    user: "%s"\n    namespace: %s\n' ` +
		ShellQuote(quote(sel.Context)) + ` ` + jp("cluster") + ` ` + jp("user") + ` ` + ShellQuote(quote(sel.Namespace)) + `; } > "$_tk_f"; `)
	return b.String()
}

// ShellQuote quotes s for a POSIX shell.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// SafeName keeps characters usable in a file name on every OS.
func SafeName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "pane"
	}
	return b.String()
}
