package importers

import (
	"archive/zip"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/csturiale/shellvane/core/tunnel"
)

// Registry locations of saved sessions (Windows).
const (
	PuTTYRegistryPath = `Software\SimonTatham\PuTTY\Sessions`
	KiTTYRegistryPath = `Software\9bis.com\KiTTY\Sessions`
)

// ErrRegistryUnsupported is returned by registry readers outside Windows.
var ErrRegistryUnsupported = errors.New("registry not available on this platform")

// PuTTYDir reads PuTTY sessions saved as files (~/.putty/sessions on Unix):
// one file per session, named after the URL-encoded session name, with
// "Key=Value" lines.
func PuTTYDir(dir string) (Result, error) {
	sessions, err := readSessionFiles(dir, "=")
	if err != nil {
		return Result{}, err
	}
	return fromSessions(sessions, false), nil
}

// KiTTYDir reads the Sessions folder of a portable KiTTY, whose files hold
// "Key\Value\" lines.
func KiTTYDir(dir string) (Result, error) {
	sessions, err := readSessionFiles(dir, `\`)
	if err != nil {
		return Result{}, err
	}
	return fromSessions(sessions, true), nil
}

// PuTTYRegistry reads PuTTY sessions from the Windows registry.
func PuTTYRegistry() (Result, error) {
	sessions, err := readRegistrySessions(PuTTYRegistryPath)
	if err != nil {
		return Result{}, err
	}
	return fromSessions(sessions, false), nil
}

// KiTTYRegistry reads KiTTY sessions from the Windows registry.
func KiTTYRegistry() (Result, error) {
	sessions, err := readRegistrySessions(KiTTYRegistryPath)
	if err != nil {
		return Result{}, err
	}
	return fromSessions(sessions, true), nil
}

// maxSessionFile bounds one session file (they are a few kilobytes).
const maxSessionFile = 1 << 20

// readSessionFiles reads the session files of dir, a folder or a .zip of
// it. Subfolders are groups (portable KiTTY keeps them so): sessions are
// keyed by their path, "QA/NEX/web01".
func readSessionFiles(dir, sep string) (map[string]map[string]string, error) {
	root, closeFS, err := openSessionsFS(dir)
	if err != nil {
		return nil, err
	}
	defer closeFS()
	out := map[string]map[string]string{}
	err = fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entry: skip it
		}
		name := d.Name()
		if d.IsDir() {
			if p != "." && (strings.HasPrefix(name, ".") || name == "__MACOSX") {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || !d.Type().IsRegular() {
			return nil // ._ files of the macOS Finder, .DS_Store
		}
		if info, err := d.Info(); err != nil || info.Size() > maxSessionFile {
			return nil
		}
		data, err := fs.ReadFile(root, p)
		if err != nil {
			return nil
		}
		kv := map[string]string{}
		for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			key, value, ok := strings.Cut(line, sep)
			if !ok {
				continue
			}
			if sep == `\` {
				value = strings.TrimSuffix(value, `\`)
			}
			kv[key] = value
		}
		out[p] = kv
		return nil
	})
	return out, err
}

// openSessionsFS opens a folder or a .zip. A zip (or folder) holding only
// the Sessions folder itself is entered.
func openSessionsFS(dir string) (fs.FS, func(), error) {
	var root fs.FS
	closeFS := func() {}
	if strings.EqualFold(filepath.Ext(dir), ".zip") {
		z, err := zip.OpenReader(dir)
		if err != nil {
			return nil, nil, err
		}
		root, closeFS = z, func() { z.Close() }
	} else {
		info, err := os.Stat(dir)
		if err != nil {
			return nil, nil, err
		}
		if !info.IsDir() {
			return nil, nil, &fs.PathError{Op: "open", Path: dir, Err: fs.ErrInvalid}
		}
		root = os.DirFS(dir)
	}
	for range 3 {
		entries, err := fs.ReadDir(root, ".")
		if err != nil {
			break
		}
		var dirs []string
		files := 0
		for _, e := range entries {
			switch {
			case strings.HasPrefix(e.Name(), ".") || e.Name() == "__MACOSX":
			case e.IsDir():
				dirs = append(dirs, e.Name())
			default:
				files++
			}
		}
		if files > 0 || len(dirs) != 1 || !strings.EqualFold(dirs[0], "Sessions") {
			break
		}
		sub, err := fs.Sub(root, dirs[0])
		if err != nil {
			break
		}
		root = sub
	}
	return root, closeFS, nil
}

// fromSessions converts sessions (name → values) in a stable order.
func fromSessions(sessions map[string]map[string]string, kitty bool) Result {
	var res Result
	names := make([]string, 0, len(sessions))
	for n := range sessions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, key := range names {
		// The key is the file's path: its folders are the fallback group.
		folders := strings.Split(key, "/")
		raw := folders[len(folders)-1]
		folders = folders[:len(folders)-1]
		for i, f := range folders {
			if u, err := url.PathUnescape(f); err == nil {
				folders[i] = u
			}
		}
		name, err := url.PathUnescape(raw)
		if err != nil {
			name = raw
		}
		if name == "Default Settings" {
			continue
		}
		kv := sessions[key]
		if proto := kv["Protocol"]; proto != "" && proto != "ssh" {
			res.warn("protocolSkipped", map[string]any{"session": name, "protocol": proto})
			continue
		}
		hostname := kv["HostName"]
		if kitty {
			// Portable KiTTY writes values URL-encoded ("host%09" for a
			// stray tab typed after the name).
			if u, err := url.PathUnescape(hostname); err == nil {
				hostname = u
			}
		}
		h := Host{Name: name, Hostname: strings.TrimSpace(hostname), User: strings.TrimSpace(kv["UserName"])}
		if user, host, ok := strings.Cut(h.Hostname, "@"); ok {
			h.User, h.Hostname = user, host
		}
		if h.Hostname == "" {
			res.warn("noHostname", map[string]any{"session": name})
			continue
		}
		if p, err := strconv.Atoi(kv["PortNumber"]); err == nil && p != 22 {
			h.Port = p
		}
		if len(folders) > 0 {
			h.Group = folders
		}
		if kitty {
			folder := kv["Folder"]
			if u, err := url.PathUnescape(folder); err == nil {
				folder = u // portable KiTTY writes it URL-encoded: PROD%5CNEX
			}
			if folder = strings.Trim(strings.ReplaceAll(folder, `\`, "/"), "/"); folder != "" && folder != "Default" {
				h.Group = strings.Split(folder, "/")
			}
			if kv["Password"] != "" {
				res.warn("passwordSkipped", map[string]any{"session": name})
			}
		}
		if kv["PublicKeyFile"] != "" {
			res.warn("keyFileSkipped", map[string]any{"session": name, "path": kv["PublicKeyFile"]})
		}
		h.Tunnels = parsePuTTYForwardings(kv["PortForwardings"], name, &res)
		res.Hosts = append(res.Hosts, h)
	}
	return res
}

// parsePuTTYForwardings reads "L8080=localhost:80,R9000=host:3000,D1080";
// entries may start with 4 or 6 (address family) and the listening side may
// include an address ("L127.0.0.1:8080=...").
func parsePuTTYForwardings(value, session string, res *Result) []tunnel.Spec {
	var out []tunnel.Spec
	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		e := strings.TrimLeft(entry, "46")
		if e == "" {
			continue
		}
		bad := func() { res.warn("forwardInvalid", map[string]any{"session": session, "value": entry}) }
		kind := map[byte]tunnel.Kind{'L': tunnel.Local, 'R': tunnel.Remote, 'D': tunnel.Dynamic}[e[0]]
		if kind == "" {
			bad()
			continue
		}
		src, dest, _ := strings.Cut(e[1:], "=")
		bindAddr, bindPort, ok := splitBind(src)
		if !ok {
			bad()
			continue
		}
		spec := tunnel.Spec{Kind: kind, BindAddr: bindAddr, BindPort: bindPort}
		if kind != tunnel.Dynamic {
			host, port, ok := splitBind(dest)
			if !ok || dest == "" {
				bad()
				continue
			}
			spec.DestHost, spec.DestPort = host, port
		}
		out = append(out, spec)
	}
	return out
}
