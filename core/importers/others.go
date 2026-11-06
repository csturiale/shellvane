package importers

import (
	"bufio"
	"encoding/xml"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Saved passwords are never imported: each importer reports them so the
// user can store them in the vault at the first login.

// readINI reads "[section]" and "key=value" lines; keys keep their case.
func readINI(path string) (sections []string, values map[string]map[string]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	values = map[string]map[string]string{}
	cur := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(sc.Text(), "\ufeff"))
		switch {
		case line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]"):
			cur = line[1 : len(line)-1]
			if _, ok := values[cur]; !ok {
				sections = append(sections, cur)
				values[cur] = map[string]string{}
			}
		default:
			if k, v, ok := strings.Cut(line, "="); ok && cur != "" {
				values[cur][strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	return sections, values, sc.Err()
}

func splitPath(p string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' }) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func port(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 && n <= 65535 && n != def {
		return n
	}
	return 0
}

// --- MobaXterm --------------------------------------------------------

// mobaTypes names MobaXterm's session types other than SSH (109).
var mobaTypes = map[string]string{"91": "RDP", "128": "VNC", "130": "FTP", "140": "SFTP", "98": "Telnet", "131": "serial", "100": "Rlogin"}

// MobaXterm reads MobaXterm.ini or a .mxtsessions export: [Bookmarks*]
// sections (SubRep is the folder) with "name=#109#0%host%port%user%..."
// lines for SSH sessions.
func MobaXterm(path string) (Result, error) {
	sections, values, err := readINI(path)
	if err != nil {
		return Result{}, err
	}
	var res Result
	for _, sec := range sections {
		if !strings.HasPrefix(sec, "Bookmarks") {
			continue
		}
		kv := values[sec]
		group := splitPath(kv["SubRep"])
		names := make([]string, 0, len(kv))
		for k := range kv {
			if k != "SubRep" && k != "ImgNum" {
				names = append(names, k)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			parts := strings.SplitN(kv[name], "#", 4)
			if len(parts) < 3 || parts[0] != "" {
				continue
			}
			if parts[1] != "109" {
				proto := mobaTypes[parts[1]]
				if proto == "" {
					proto = "#" + parts[1]
				}
				res.warn("protocolSkipped", map[string]any{"session": name, "protocol": proto})
				continue
			}
			f := strings.Split(parts[2], "%")
			h := Host{Name: name, Group: group}
			if len(f) > 1 {
				h.Hostname = f[1]
			}
			if len(f) > 2 {
				h.Port = port(f[2], 22)
			}
			if len(f) > 3 {
				h.User = f[3]
			}
			if h.Hostname == "" {
				res.warn("noHostname", map[string]any{"session": name})
				continue
			}
			res.Hosts = append(res.Hosts, h)
		}
	}
	return res, nil
}

// --- SecureCRT --------------------------------------------------------

// SecureCRT reads the Sessions folder: one .ini per session, subfolders
// are groups, lines like S:"Hostname"=web and D:"[SSH2] Port"=00000016.
func SecureCRT(dir string) (Result, error) {
	var res Result
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() || !strings.EqualFold(filepath.Ext(name), ".ini") || strings.HasPrefix(name, "__") || strings.EqualFold(name, "Default.ini") {
			return nil
		}
		kv, err := secureCRTValues(path)
		if err != nil {
			return err
		}
		session := strings.TrimSuffix(name, filepath.Ext(name))
		rel, _ := filepath.Rel(dir, filepath.Dir(path))
		if proto := kv["Protocol Name"]; proto != "" && !strings.HasPrefix(strings.ToUpper(proto), "SSH") {
			res.warn("protocolSkipped", map[string]any{"session": session, "protocol": proto})
			return nil
		}
		h := Host{Name: session, Hostname: kv["Hostname"], User: kv["Username"]}
		if rel != "." {
			h.Group = splitPath(rel)
		}
		if p, err := strconv.ParseInt(kv["[SSH2] Port"], 16, 32); err == nil {
			h.Port = port(strconv.Itoa(int(p)), 22)
		}
		if h.Hostname == "" {
			res.warn("noHostname", map[string]any{"session": session})
			return nil
		}
		if kv["Password V2"] != "" || kv["Password"] != "" {
			res.warn("passwordSkipped", map[string]any{"session": session})
		}
		res.Hosts = append(res.Hosts, h)
		return nil
	})
	return res, err
}

func secureCRTValues(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	kv := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		// S:"Key"=value, D:"Key"=hex, B:... (binary, skipped)
		if len(line) < 4 || line[1] != ':' || line[2] != '"' {
			continue
		}
		end := strings.Index(line[3:], `"=`)
		if end < 0 {
			continue
		}
		kv[line[3:3+end]] = line[3+end+2:]
	}
	return kv, nil
}

// --- WinSCP -----------------------------------------------------------

// WinSCP reads WinSCP.ini: [Sessions\folder/name] sections, URL-encoded,
// with HostName, PortNumber, UserName and FSProtocol (FTP and WebDAV are
// skipped).
func WinSCP(path string) (Result, error) {
	sections, values, err := readINI(path)
	if err != nil {
		return Result{}, err
	}
	var res Result
	for _, sec := range sections {
		raw, ok := strings.CutPrefix(sec, `Sessions\`)
		if !ok {
			continue
		}
		full, err := url.PathUnescape(raw)
		if err != nil {
			full = raw
		}
		if full == "Default Settings" {
			continue
		}
		kv := values[sec]
		parts := splitPath(full)
		name := parts[len(parts)-1]
		switch kv["FSProtocol"] {
		case "", "0", "1", "2": // SCP, SFTP (SCP fallback), SFTP
		default:
			res.warn("protocolSkipped", map[string]any{"session": name, "protocol": map[string]string{"5": "FTP", "6": "WebDAV", "7": "S3"}[kv["FSProtocol"]]})
			continue
		}
		h := Host{Name: name, Group: parts[:len(parts)-1], Hostname: kv["HostName"], User: kv["UserName"], Port: port(kv["PortNumber"], 22)}
		if h.Hostname == "" {
			res.warn("noHostname", map[string]any{"session": name})
			continue
		}
		if kv["Password"] != "" {
			res.warn("passwordSkipped", map[string]any{"session": name})
		}
		res.Hosts = append(res.Hosts, h)
	}
	return res, nil
}

// --- FileZilla --------------------------------------------------------

// FileZilla reads sitemanager.xml; only SFTP sites (Protocol 1) are SSH.
func FileZilla(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	type server struct {
		Host     string `xml:"Host"`
		Port     string `xml:"Port"`
		Protocol string `xml:"Protocol"`
		User     string `xml:"User"`
		Pass     string `xml:"Pass"`
		Name     string `xml:"Name"`
	}
	type folder struct {
		Name    string   `xml:",chardata"`
		Servers []server `xml:"Server"`
		Folders []folder `xml:"Folder"`
	}
	var doc struct {
		Servers struct {
			Servers []server `xml:"Server"`
			Folders []folder `xml:"Folder"`
		} `xml:"Servers"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return Result{}, err
	}
	var res Result
	var walk func(group []string, servers []server, folders []folder)
	walk = func(group []string, servers []server, folders []folder) {
		for _, s := range servers {
			name := strings.TrimSpace(s.Name)
			if name == "" {
				name = s.Host
			}
			if s.Protocol != "1" {
				res.warn("protocolSkipped", map[string]any{"session": name, "protocol": map[string]string{"0": "FTP", "3": "FTPS", "4": "FTPES"}[s.Protocol]})
				continue
			}
			if s.Host == "" {
				res.warn("noHostname", map[string]any{"session": name})
				continue
			}
			if s.Pass != "" {
				res.warn("passwordSkipped", map[string]any{"session": name})
			}
			res.Hosts = append(res.Hosts, Host{Name: name, Hostname: s.Host, User: s.User, Port: port(s.Port, 22), Group: append([]string(nil), group...)})
		}
		for _, f := range folders {
			walk(append(append([]string(nil), group...), strings.TrimSpace(f.Name)), f.Servers, f.Folders)
		}
	}
	walk(nil, doc.Servers.Servers, doc.Servers.Folders)
	return res, nil
}

// --- Tabby ------------------------------------------------------------

// Tabby reads config.yaml: SSH profiles with their group and jump host.
func Tabby(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	var doc struct {
		Profiles []struct {
			ID      string `yaml:"id"`
			Type    string `yaml:"type"`
			Name    string `yaml:"name"`
			Group   string `yaml:"group"`
			Options struct {
				Host     string `yaml:"host"`
				Port     int    `yaml:"port"`
				User     string `yaml:"user"`
				JumpHost string `yaml:"jumpHost"`
				Password string `yaml:"password"`
			} `yaml:"options"`
		} `yaml:"profiles"`
		Groups []struct {
			ID   string `yaml:"id"`
			Name string `yaml:"name"`
		} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Result{}, err
	}
	groups := map[string]string{}
	for _, g := range doc.Groups {
		groups[g.ID] = g.Name
	}
	names := map[string]string{}
	for _, p := range doc.Profiles {
		names[p.ID] = p.Name
	}
	var res Result
	for _, p := range doc.Profiles {
		if p.Type != "ssh" {
			if p.Type != "" && p.Type != "local" {
				res.warn("protocolSkipped", map[string]any{"session": p.Name, "protocol": p.Type})
			}
			continue
		}
		h := Host{Name: p.Name, Hostname: p.Options.Host, User: p.Options.User, Port: port(strconv.Itoa(p.Options.Port), 22)}
		if g := groups[p.Group]; g != "" {
			h.Group = splitPath(g)
		} else if p.Group != "" {
			h.Group = splitPath(p.Group)
		}
		if j := p.Options.JumpHost; j != "" {
			if n, ok := names[j]; ok {
				j = n
			}
			h.ProxyJump = []string{j}
		}
		if h.Hostname == "" {
			res.warn("noHostname", map[string]any{"session": p.Name})
			continue
		}
		if p.Options.Password != "" {
			res.warn("passwordSkipped", map[string]any{"session": p.Name})
		}
		res.Hosts = append(res.Hosts, h)
	}
	return res, nil
}

// --- mRemoteNG --------------------------------------------------------

// MRemoteNG reads confCons.xml: nested Node elements, Type="Container" for
// folders, Protocol="SSH2"/"SSH1" for SSH connections.
func MRemoteNG(path string) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	type node struct {
		Name     string `xml:"Name,attr"`
		Type     string `xml:"Type,attr"`
		Hostname string `xml:"Hostname,attr"`
		Port     string `xml:"Port,attr"`
		Username string `xml:"Username,attr"`
		Password string `xml:"Password,attr"`
		Protocol string `xml:"Protocol,attr"`
		Nodes    []node `xml:"Node"`
	}
	var doc struct {
		Nodes []node `xml:"Node"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return Result{}, err
	}
	var res Result
	var walk func(group []string, nodes []node)
	walk = func(group []string, nodes []node) {
		for _, n := range nodes {
			if n.Type == "Container" {
				walk(append(append([]string(nil), group...), n.Name), n.Nodes)
				continue
			}
			if !strings.HasPrefix(strings.ToUpper(n.Protocol), "SSH") {
				res.warn("protocolSkipped", map[string]any{"session": n.Name, "protocol": n.Protocol})
				continue
			}
			if n.Hostname == "" {
				res.warn("noHostname", map[string]any{"session": n.Name})
				continue
			}
			if n.Password != "" {
				res.warn("passwordSkipped", map[string]any{"session": n.Name})
			}
			res.Hosts = append(res.Hosts, Host{Name: n.Name, Hostname: n.Hostname, User: n.Username, Port: port(n.Port, 22), Group: append([]string(nil), group...)})
		}
	}
	walk(nil, doc.Nodes)
	return res, nil
}
