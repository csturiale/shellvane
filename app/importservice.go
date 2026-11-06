package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/importers"
	"github.com/csturiale/shellvane/core/settings"
	"github.com/csturiale/shellvane/core/sshengine"
	"github.com/csturiale/shellvane/core/store"
	"github.com/csturiale/shellvane/core/tunnel"
)

// Error codes of the import service.
const (
	ErrImportRead   errs.Code = "import.readFailed"
	ErrImportSource errs.Code = "import.unknownSource"
)

// ImportSource is a tool the user can import from.
type ImportSource struct {
	ID string `json:"id"` // "openssh", "putty", "kitty", "mobaxterm"...
	// Kind tells the UI what Path means: "file", "dir" or "registry".
	Kind string `json:"kind"`
	// Path is the default location; empty for the registry.
	Path string `json:"path"`
	// Found is true when something exists at the default location.
	Found bool `json:"found"`
}

// ImportRequest lists the previewed hosts the user kept.
type ImportRequest struct {
	Hosts []importers.Host `json:"hosts"`
	// Group is created at the top level to hold everything; empty imports
	// at the top level directly.
	Group string `json:"group"`
}

// ImportSummary reports what was saved.
type ImportSummary struct {
	Hosts   int    `json:"hosts"`
	Skipped int    `json:"skipped"` // already present: same address and user
	Groups  int    `json:"groups"`
	Tunnels int    `json:"tunnels"`
	Backup  string `json:"backup"`
}

// ImportService previews and imports hosts from other tools.
type ImportService struct {
	db        *sql.DB
	hosts     *hosts.Repo
	tunnels   *tunnel.Repo
	backupDir string
}

// NewImportService returns a service; backups go to backupDir.
func NewImportService(db *sql.DB, h *hosts.Repo, t *tunnel.Repo, backupDir string) *ImportService {
	return &ImportService{db: db, hosts: h, tunnels: t, backupDir: backupDir}
}

// Sources lists the supported tools with their default locations.
func (s *ImportService) Sources() []ImportSource {
	home, _ := os.UserHomeDir()
	exists := func(p string) bool { _, err := os.Stat(p); return p != "" && err == nil }
	sshConfig := filepath.Join(home, ".ssh", "config")
	out := []ImportSource{{ID: "openssh", Kind: "file", Path: sshConfig, Found: exists(sshConfig)}}
	if runtime.GOOS == "windows" {
		_, perr := importers.PuTTYRegistry()
		_, kerr := importers.KiTTYRegistry()
		out = append(out,
			ImportSource{ID: "putty", Kind: "registry", Found: perr == nil},
			ImportSource{ID: "kitty", Kind: "registry", Found: kerr == nil},
		)
	} else {
		puttyDir := filepath.Join(home, ".putty", "sessions")
		out = append(out, ImportSource{ID: "putty", Kind: "dir", Path: puttyDir, Found: exists(puttyDir)})
	}
	// Portable KiTTY keeps sessions in a folder next to its executable.
	out = append(out, ImportSource{ID: "kitty", Kind: "dir"})
	for _, o := range otherSources(home) {
		o.Found = exists(o.Path)
		out = append(out, o)
	}
	return out
}

// otherSources are the tools read from a file or folder, with their usual
// location on this system ("" when there is none).
func otherSources(home string) []ImportSource {
	appData := os.Getenv("APPDATA")
	pick := func(windows, mac, linux string) string {
		switch runtime.GOOS {
		case "windows":
			if appData == "" {
				return ""
			}
			return filepath.Join(appData, windows)
		case "darwin":
			if mac == "" {
				return ""
			}
			return filepath.Join(home, mac)
		default:
			if linux == "" {
				return ""
			}
			return filepath.Join(home, linux)
		}
	}
	return []ImportSource{
		{ID: "mobaxterm", Kind: "file", Path: pick(filepath.Join("MobaXterm", "MobaXterm.ini"), "", "")},
		{ID: "securecrt", Kind: "dir", Path: pick(filepath.Join("VanDyke", "Config", "Sessions"), "Library/Application Support/VanDyke/SecureCRT/Config/Sessions", ".vandyke/SecureCRT/Config/Sessions")},
		{ID: "winscp", Kind: "file", Path: pick("WinSCP.ini", "", "")},
		{ID: "filezilla", Kind: "file", Path: pick(filepath.Join("FileZilla", "sitemanager.xml"), ".config/filezilla/sitemanager.xml", ".config/filezilla/sitemanager.xml")},
		{ID: "tabby", Kind: "file", Path: pick(filepath.Join("tabby", "config.yaml"), "Library/Application Support/tabby/config.yaml", ".config/tabby/config.yaml")},
		{ID: "mremoteng", Kind: "file", Path: pick(filepath.Join("mRemoteNG", "confCons.xml"), "", "")},
		// Termius exports hosts as an SSH config file.
		{ID: "termius", Kind: "file"},
	}
}

// Preview reads a source without saving anything.
func (s *ImportService) Preview(source, kind, path string) (importers.Result, error) {
	var res importers.Result
	var err error
	switch {
	case source == "openssh":
		res, err = importers.OpenSSH(path)
	case source == "putty" && kind == "registry":
		res, err = importers.PuTTYRegistry()
	case source == "putty":
		res, err = importers.PuTTYDir(path)
	case source == "kitty" && kind == "registry":
		res, err = importers.KiTTYRegistry()
	case source == "kitty":
		res, err = importers.KiTTYDir(path)
	case source == "mobaxterm":
		res, err = importers.MobaXterm(path)
	case source == "securecrt":
		res, err = importers.SecureCRT(path)
	case source == "winscp":
		res, err = importers.WinSCP(path)
	case source == "filezilla":
		res, err = importers.FileZilla(path)
	case source == "tabby":
		res, err = importers.Tabby(path)
	case source == "mremoteng":
		res, err = importers.MRemoteNG(path)
	case source == "termius":
		res, err = importers.OpenSSH(path)
	default:
		return res, errs.New(ErrImportSource, errs.Params{"source": source})
	}
	if err != nil {
		return res, errs.Wrap(err, ErrImportRead, errs.Params{"path": path})
	}
	if res.Hosts == nil {
		res.Hosts = []importers.Host{}
	}
	if res.Warnings == nil {
		res.Warnings = []importers.Warning{}
	}
	return res, nil
}

// Import saves the chosen hosts, after backing up the database.
func (s *ImportService) Import(ctx context.Context, req ImportRequest) (ImportSummary, error) {
	var sum ImportSummary
	backup, err := store.Backup(ctx, s.db, s.backupDir)
	if err != nil {
		return sum, err
	}
	sum.Backup = backup

	inv, err := s.hosts.List(ctx)
	if err != nil {
		return sum, err
	}
	existing := map[string]bool{}
	for _, h := range inv.Hosts {
		existing[hostKey(h.Hostname, h.Settings.Port, h.Settings.User)] = true
	}
	groups := newGroupIndex(inv.Groups)

	root := ""
	if name := strings.TrimSpace(req.Group); name != "" {
		if root, err = groups.ensure(ctx, s.hosts, "", name, &sum); err != nil {
			return sum, err
		}
	}
	// Imported host IDs by alias, to resolve ProxyJump afterwards.
	byAlias := map[string]string{}
	for _, h := range inv.Hosts {
		byAlias[strings.ToLower(h.Name)] = h.ID
	}
	type pendingJump struct {
		hostID string
		hops   []string
	}
	var jumps []pendingJump

	for _, ih := range req.Hosts {
		var st settings.Settings
		if ih.Port != 0 {
			st.Port = &ih.Port
		}
		if ih.User != "" {
			st.User = &ih.User
		}
		key := hostKey(ih.Hostname, st.Port, st.User)
		if existing[key] {
			sum.Skipped++
			continue
		}
		parent := root
		for _, name := range ih.Group {
			if parent, err = groups.ensure(ctx, s.hosts, parent, name, &sum); err != nil {
				return sum, err
			}
		}
		if ih.ProxyCommand != "" {
			cmd := ih.ProxyCommand
			st.ProxyCommand = &cmd
		}
		if len(ih.Environment) > 0 {
			st.Environment = ih.Environment
		}
		if len(ih.AuthMethods) > 0 {
			st.AuthMethods = ih.AuthMethods
		}
		if ih.IdentitiesOnly {
			st.IdentitiesOnly = &ih.IdentitiesOnly
		}
		if ih.RemoteCommand != "" {
			cmd := ih.RemoteCommand
			st.RemoteCommand = &cmd
		}
		h, err := s.hosts.CreateHost(ctx, hosts.HostInput{GroupID: parent, Name: ih.Name, Hostname: ih.Hostname, Settings: st})
		if err != nil {
			return sum, err
		}
		byAlias[strings.ToLower(ih.Name)] = h.ID
		if len(ih.ProxyJump) > 0 {
			jumps = append(jumps, pendingJump{hostID: h.ID, hops: ih.ProxyJump})
		}
		existing[key] = true
		sum.Hosts++
		for _, spec := range ih.Tunnels {
			if _, err := s.tunnels.Create(ctx, tunnel.Input{HostID: h.ID, Name: importedTunnelName(ih.Name, spec), Spec: spec}); err == nil {
				sum.Tunnels++
			}
		}
	}
	// Second pass: jump hosts may be defined after the hosts that use them.
	for _, j := range jumps {
		chain := []string{}
		for _, hop := range j.hops {
			id, ok := byAlias[strings.ToLower(hop)]
			if !ok {
				// Not an imported alias: [user@]host[:port] becomes a host.
				t, err := sshengine.ParseTarget(hop)
				if err != nil {
					continue
				}
				var st settings.Settings
				if t.User != "" {
					st.User = &t.User
				}
				if t.Port != 22 {
					st.Port = &t.Port
				}
				h, err := s.hosts.CreateHost(ctx, hosts.HostInput{GroupID: root, Name: hop, Hostname: t.Host, Settings: st})
				if err != nil {
					return sum, err
				}
				id = h.ID
				byAlias[strings.ToLower(hop)] = id
				sum.Hosts++
			}
			chain = append(chain, id)
		}
		h, err := s.hosts.Host(ctx, j.hostID)
		if err != nil {
			return sum, err
		}
		h.Settings.JumpChain = chain
		if _, err := s.hosts.UpdateHost(ctx, h.ID, hosts.HostInput{GroupID: h.GroupID, Name: h.Name, Hostname: h.Hostname, Notes: h.Notes, Settings: h.Settings}); err != nil {
			return sum, err
		}
	}
	return sum, nil
}

// hostKey identifies a host for duplicate detection: user, address and
// port as set on the host itself (empty when inherited).
func hostKey(hostname string, port *int, user *string) string {
	p, u := "", ""
	if port != nil {
		p = strconv.Itoa(*port)
	}
	if user != nil {
		u = *user
	}
	return u + "@" + strings.ToLower(hostname) + ":" + p
}

// groupIndex finds or creates groups by parent and name, so re-importing
// reuses the same folders.
type groupIndex map[string]string // parentID + "/" + name → id

func newGroupIndex(groups []hosts.Group) groupIndex {
	idx := groupIndex{}
	for _, g := range groups {
		idx[g.ParentID+"/"+strings.ToLower(g.Name)] = g.ID
	}
	return idx
}

func (idx groupIndex) ensure(ctx context.Context, repo *hosts.Repo, parent, name string, sum *ImportSummary) (string, error) {
	key := parent + "/" + strings.ToLower(name)
	if id, ok := idx[key]; ok {
		return id, nil
	}
	g, err := repo.CreateGroup(ctx, hosts.GroupInput{ParentID: parent, Name: name})
	if err != nil {
		return "", err
	}
	idx[key] = g.ID
	sum.Groups++
	return g.ID, nil
}

// importedTunnelName names an imported tunnel after its host and ports,
// e.g. "bastion SOCKS 1080" or "web L 8080 → db:5432", so that it can be
// recognized and referenced by name (environments do).
func importedTunnelName(host string, spec tunnel.Spec) string {
	port := strconv.Itoa(spec.BindPort)
	switch spec.Kind {
	case tunnel.Dynamic:
		return host + " SOCKS " + port
	case tunnel.Remote:
		return host + " R " + port + " → " + spec.DestHost + ":" + strconv.Itoa(spec.DestPort)
	}
	return host + " L " + port + " → " + spec.DestHost + ":" + strconv.Itoa(spec.DestPort)
}
