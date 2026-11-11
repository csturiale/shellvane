package hosts

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/settings"
)

// ErrTemplateInvalid is returned for text that is not a group export.
const ErrTemplateInvalid errs.Code = "hosts.templateInvalid"

// TemplateKind marks a group export.
const TemplateKind = "shellvane-group-template"

// Template is a tree of groups with their settings and, optionally, their
// hosts, to share a set of connections with colleagues. It never holds
// secrets: no vault keys, passwords, notes or knock sequences.
type Template struct {
	Kind    string          `json:"kind"`
	Version int             `json:"version"`
	Groups  []TemplateGroup `json:"groups"`
}

// TemplateGroup is a group of an export.
type TemplateGroup struct {
	Name     string            `json:"name"`
	Color    string            `json:"color,omitempty"`
	Settings settings.Settings `json:"settings"`
	// Jumps names the jump hosts (settings keep host IDs, which mean
	// nothing on another computer).
	Jumps  []string        `json:"jumps,omitempty"`
	Hosts  []TemplateHost  `json:"hosts,omitempty"`
	Groups []TemplateGroup `json:"groups,omitempty"`
	// PersonalUsers: the user names are left out, each member keeps their
	// own (set on the top group of a shared group).
	PersonalUsers bool `json:"personalUsers,omitempty"`
}

// WithoutUsers returns g with the user names of its groups and hosts
// removed, at every level.
func (g TemplateGroup) WithoutUsers() TemplateGroup {
	g.Settings.User = nil
	hs := make([]TemplateHost, len(g.Hosts))
	for i, h := range g.Hosts {
		h.Settings.User = nil
		hs[i] = h
	}
	g.Hosts = hs
	gs := make([]TemplateGroup, len(g.Groups))
	for i, c := range g.Groups {
		gs[i] = c.WithoutUsers()
	}
	g.Groups = gs
	return g
}

// TemplateHost is a host of an export.
type TemplateHost struct {
	Name     string            `json:"name"`
	Hostname string            `json:"hostname"`
	Tags     []string          `json:"tags,omitempty"`
	Settings settings.Settings `json:"settings"`
	Jumps    []string          `json:"jumps,omitempty"`
}

// TemplateResult tells what an import did.
type TemplateResult struct {
	Created int `json:"created"`
	// Reused counts groups that already existed with the same name in the
	// same place: they are kept as they are.
	Reused int `json:"reused"`
	// HostsCreated and HostsSkipped count hosts; a host is skipped when its
	// group already has one with the same name and address.
	HostsCreated int `json:"hostsCreated"`
	HostsSkipped int `json:"hostsSkipped"`
	// CommandsLeftOut lists the commands of the export not imported.
	CommandsLeftOut []string `json:"commandsLeftOut"`
}

// templateSettings drops what only makes sense on this computer or is a
// secret: vault keys, jump host IDs (exported by name), local themes, the
// Wake-on-LAN address and the knock sequence.
func templateSettings(s settings.Settings) settings.Settings {
	s.IdentityID = nil
	s.CredentialPlugin = nil // a configuration of this computer
	s.JumpChain = nil
	s.TerminalThemeID = nil
	s.MACAddress = nil
	s.WakeBroadcast = nil
	s.KnockSequence = nil
	return s
}

// ExportTemplate returns the group groupID and its subgroups, or every
// group when groupID is empty; withHosts adds their hosts.
func ExportTemplate(inv Inventory, groupID string, withHosts bool) (Template, error) {
	return ExportTemplateKeeping(inv, groupID, withHosts, nil)
}

// ExportTemplateKeeping is ExportTemplate keeping the credential settings
// that keep accepts (team logins, valid on every member's computer).
func ExportTemplateKeeping(inv Inventory, groupID string, withHosts bool, keep func(ref string) bool) (Template, error) {
	clean := func(s settings.Settings) settings.Settings {
		cred := s.CredentialPlugin
		s = templateSettings(s)
		if cred != nil && keep != nil && keep(*cred) {
			s.CredentialPlugin = cred
		}
		return s
	}
	children := map[string][]Group{}
	hostsIn := map[string][]Host{}
	names := map[string]string{}
	found := groupID == ""
	for _, g := range inv.Groups {
		children[g.ParentID] = append(children[g.ParentID], g)
		if g.ID == groupID {
			found = true
		}
	}
	for _, h := range inv.Hosts {
		hostsIn[h.GroupID] = append(hostsIn[h.GroupID], h)
		names[h.ID] = h.Name
	}
	if !found {
		return Template{}, errs.New(ErrGroupNotFound, errs.Params{"id": groupID})
	}
	jumps := func(chain []string) []string {
		var out []string
		for _, id := range chain {
			if n, ok := names[id]; ok {
				out = append(out, n)
			}
		}
		return out
	}
	var build func(g Group) TemplateGroup
	build = func(g Group) TemplateGroup {
		tg := TemplateGroup{Name: g.Name, Color: g.Color, Settings: clean(g.Settings), Jumps: jumps(g.Settings.JumpChain)}
		if withHosts {
			for _, h := range hostsIn[g.ID] {
				tg.Hosts = append(tg.Hosts, TemplateHost{
					Name: h.Name, Hostname: h.Hostname, Tags: h.Tags,
					Settings: clean(h.Settings), Jumps: jumps(h.Settings.JumpChain),
				})
			}
		}
		for _, c := range children[g.ID] {
			tg.Groups = append(tg.Groups, build(c))
		}
		return tg
	}
	t := Template{Kind: TemplateKind, Version: 1, Groups: []TemplateGroup{}}
	for _, g := range inv.Groups {
		if (groupID == "" && g.ParentID == "") || g.ID == groupID {
			t.Groups = append(t.Groups, build(g))
		}
	}
	return t, nil
}

// MarshalTemplate renders an export as indented JSON.
func MarshalTemplate(t Template) string {
	b, _ := json.MarshalIndent(t, "", "  ")
	return string(b) + "\n"
}

// ParseTemplate reads an export made by MarshalTemplate.
func ParseTemplate(text string) (Template, error) {
	var t Template
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &t); err != nil || t.Kind != TemplateKind || t.Version != 1 {
		return Template{}, errs.New(ErrTemplateInvalid, nil)
	}
	var check func(gs []TemplateGroup) bool
	check = func(gs []TemplateGroup) bool {
		for _, g := range gs {
			if strings.TrimSpace(g.Name) == "" || !check(g.Groups) {
				return false
			}
			for _, h := range g.Hosts {
				if strings.TrimSpace(h.Name) == "" || strings.TrimSpace(h.Hostname) == "" {
					return false
				}
			}
		}
		return true
	}
	if len(t.Groups) == 0 || !check(t.Groups) {
		return Template{}, errs.New(ErrTemplateInvalid, nil)
	}
	return t, nil
}

// ImportTemplate creates the export's groups and hosts under parentID
// (empty for the top level). A group whose name is already used in the
// same place is reused, settings untouched, and receives what it lacks; a
// host already in its group with the same name and address is skipped.
// Jump hosts are linked by name once every host exists.
func (r *Repo) ImportTemplate(ctx context.Context, parentID string, t Template) (TemplateResult, error) {
	var res TemplateResult
	if err := r.requireGroup(ctx, parentID); err != nil {
		return res, err
	}
	inv, err := r.List(ctx)
	if err != nil {
		return res, err
	}
	existing := map[string]map[string]string{} // parent → lower name → id
	for _, g := range inv.Groups {
		if existing[g.ParentID] == nil {
			existing[g.ParentID] = map[string]string{}
		}
		existing[g.ParentID][strings.ToLower(g.Name)] = g.ID
	}
	hostKey := func(group, name, hostname string) string {
		return group + "\x00" + strings.ToLower(name) + "\x00" + strings.ToLower(hostname)
	}
	haveHost := map[string]bool{}
	byName := map[string]string{} // lower host name → id, for jump hosts
	for _, h := range inv.Hosts {
		haveHost[hostKey(h.GroupID, h.Name, h.Hostname)] = true
		byName[strings.ToLower(h.Name)] = h.ID
	}

	// Created items that name jump hosts, linked at the end.
	type pendingGroup struct {
		g     Group
		jumps []string
	}
	type pendingHost struct {
		h     Host
		jumps []string
	}
	var groupJumps []pendingGroup
	var hostJumps []pendingHost

	var add func(parent string, gs []TemplateGroup) error
	add = func(parent string, gs []TemplateGroup) error {
		for _, tg := range gs {
			id, ok := existing[parent][strings.ToLower(strings.TrimSpace(tg.Name))]
			if ok {
				res.Reused++
			} else {
				g, err := r.CreateGroup(ctx, GroupInput{ParentID: parent, Name: tg.Name, Color: tg.Color, Settings: templateSettings(tg.Settings)})
				if err != nil {
					return err
				}
				id = g.ID
				res.Created++
				if len(tg.Jumps) > 0 {
					groupJumps = append(groupJumps, pendingGroup{g, tg.Jumps})
				}
			}
			for _, th := range tg.Hosts {
				k := hostKey(id, strings.TrimSpace(th.Name), strings.TrimSpace(th.Hostname))
				if haveHost[k] {
					res.HostsSkipped++
					continue
				}
				h, err := r.CreateHost(ctx, HostInput{GroupID: id, Name: th.Name, Hostname: th.Hostname, Tags: th.Tags, Settings: templateSettings(th.Settings)})
				if err != nil {
					return err
				}
				haveHost[k] = true
				byName[strings.ToLower(h.Name)] = h.ID
				res.HostsCreated++
				if len(th.Jumps) > 0 {
					hostJumps = append(hostJumps, pendingHost{h, th.Jumps})
				}
			}
			if err := add(id, tg.Groups); err != nil {
				return err
			}
		}
		return nil
	}
	if err := add(parentID, t.Groups); err != nil {
		return res, err
	}

	chain := func(names []string, self string) []string {
		var ids []string
		for _, n := range names {
			if id, ok := byName[strings.ToLower(n)]; ok && id != self {
				ids = append(ids, id)
			}
		}
		return ids
	}
	for _, p := range groupJumps {
		if ids := chain(p.jumps, ""); len(ids) > 0 {
			p.g.Settings.JumpChain = ids
			if _, err := r.UpdateGroup(ctx, p.g.ID, GroupInput{ParentID: p.g.ParentID, Name: p.g.Name, Color: p.g.Color, Settings: p.g.Settings}); err != nil {
				return res, err
			}
		}
	}
	for _, p := range hostJumps {
		if ids := chain(p.jumps, p.h.ID); len(ids) > 0 {
			p.h.Settings.JumpChain = ids
			if _, err := r.UpdateHost(ctx, p.h.ID, HostInput{GroupID: p.h.GroupID, Name: p.h.Name, Hostname: p.h.Hostname, Tags: p.h.Tags, Settings: p.h.Settings}); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}
