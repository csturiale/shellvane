package diagnostics

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/anonymize"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/settings"
)

// File is one file of the support bundle.
type File struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

// BundleInput is what goes into a support bundle. It must never hold
// secrets: the vault, keys and passwords are not even passed in.
type BundleInput struct {
	Version, Commit, OS, Arch string
	Inventory                 hosts.Inventory
	// Preferences are the UI settings; personal lists are left out.
	Preferences map[string]string
	Handshakes  []Handshake
	// Counts of other items (tunnels, snippets...), never their content.
	Counts map[string]int
}

// Preferences whose values are personal data rather than settings.
var privatePreferences = []string{"history", "bookmark", "recent", "workspace", "draft", "notes", "personal"}

// Settings that may hold names, paths, commands or credentials: only
// whether they are set is kept.
var opaqueSettings = map[string]bool{
	"identityId": true, "proxy": true, "proxyCommand": true, "passwordCommand": true,
	"certificateCommand": true, "preConnectCommand": true, "startupCommands": true, "kubeContext": true, "kubeNamespace": true,
}

// Bundle builds the files of an anonymized support bundle.
func Bundle(in BundleInput, a *anonymize.Anonymizer) []File {
	hostNames := map[string]string{}
	for _, h := range in.Inventory.Hosts {
		hostNames[h.ID] = a.Name(anonymize.KindHost, h.Name)
		a.Host(h.Hostname)
	}
	groupNames := map[string]string{}
	for _, g := range in.Inventory.Groups {
		groupNames[g.ID] = a.Name(anonymize.KindGroup, g.Name)
	}

	type group struct {
		Name     string         `json:"name"`
		Parent   string         `json:"parent,omitempty"`
		Color    string         `json:"color,omitempty"`
		Settings map[string]any `json:"settings,omitempty"`
	}
	type host struct {
		Name     string         `json:"name"`
		Address  string         `json:"address"`
		Group    string         `json:"group,omitempty"`
		Tags     []string       `json:"tags,omitempty"`
		HasNotes bool           `json:"hasNotes,omitempty"`
		Settings map[string]any `json:"settings,omitempty"`
	}
	var inv struct {
		Groups []group `json:"groups"`
		Hosts  []host  `json:"hosts"`
	}
	inv.Groups, inv.Hosts = []group{}, []host{}
	for _, g := range in.Inventory.Groups {
		inv.Groups = append(inv.Groups, group{Name: groupNames[g.ID], Parent: groupNames[g.ParentID], Color: g.Color, Settings: cleanSettings(g.Settings, a, hostNames)})
	}
	for _, h := range in.Inventory.Hosts {
		var tags []string
		for _, t := range h.Tags {
			tags = append(tags, a.Name(anonymize.KindTag, t))
		}
		inv.Hosts = append(inv.Hosts, host{
			Name: hostNames[h.ID], Address: a.Host(h.Hostname), Group: groupNames[h.GroupID],
			Tags: tags, HasNotes: h.Notes != "", Settings: cleanSettings(h.Settings, a, hostNames),
		})
	}

	prefs := map[string]string{}
	for k, v := range in.Preferences {
		if !isPrivate(k) {
			prefs[k] = a.Text(v)
		}
	}

	var hs strings.Builder
	for _, h := range in.Handshakes {
		hs.WriteString(Format(h, a))
		hs.WriteString("\n")
	}
	if hs.Len() == 0 {
		hs.WriteString("No connection attempts since the application started.\n")
	}

	system := map[string]any{
		"version": in.Version, "commit": in.Commit, "os": in.OS, "arch": in.Arch,
		"created": time.Now().UTC().Format(time.RFC3339), "counts": in.Counts,
	}
	return []File{
		{Name: "README.txt", Content: readme},
		{Name: "system.json", Content: toJSON(system)},
		{Name: "preferences.json", Content: toJSON(prefs)},
		{Name: "inventory.json", Content: toJSON(inv)},
		{Name: "handshakes.txt", Content: hs.String()},
	}
}

const readme = `Shellvane support bundle

Names of hosts, groups, tags and users and every address were replaced by
pseudonyms (host-01, group-01, tag-01, user-01, 10.255.0.x), the same
everywhere in this bundle. The table of correspondences was not saved.
No vault, private key, password, TOTP seed or command was included;
settings that may hold them only say "(set)".
`

func isPrivate(key string) bool {
	k := strings.ToLower(key)
	for _, p := range privatePreferences {
		if strings.Contains(k, p) {
			return true
		}
	}
	return false
}

func cleanSettings(s settings.Settings, a *anonymize.Anonymizer, hostNames map[string]string) map[string]any {
	raw, _ := json.Marshal(s)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	for k, v := range m {
		switch {
		case opaqueSettings[k]:
			m[k] = "(set)"
		case k == "user":
			m[k] = a.Name(anonymize.KindUser, v.(string))
		case k == "jumpChain":
			var chain []string
			for _, id := range v.([]any) {
				name, ok := hostNames[id.(string)]
				if !ok {
					name = "(unknown)"
				}
				chain = append(chain, name)
			}
			m[k] = chain
		}
	}
	if len(m) == 0 {
		return nil
	}
	return m
}

func toJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b) + "\n"
}

// Zip packs the files in a ZIP archive.
func Zip(files []File) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, f := range files {
		fw, err := w.Create("shellvane-support/" + f.Name)
		if err != nil {
			return nil, err
		}
		if _, err := fw.Write([]byte(f.Content)); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
