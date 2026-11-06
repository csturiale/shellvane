package hosts

import (
	"context"
	"strings"

	"github.com/csturiale/shellvane/core/settings"
)

// MirrorResult tells what a mirror changed.
type MirrorResult struct {
	Added   int `json:"added"`
	Updated int `json:"updated"`
	Removed int `json:"removed"`
}

// MirrorTemplate makes the group rootID and everything below it match tg
// (a shared group): subgroups and hosts are matched by name and updated in
// place, so their IDs — and the tunnels, environments and favorites that
// refer to them — stay; what tg no longer has is removed. The root keeps
// its local name. Settings that only make sense on this computer (the
// host's key, terminal theme, Wake-on-LAN, knock sequence) are kept from
// the local copy when tg does not set them. Jump hosts are linked by name,
// inside the mirrored group first.
func (r *Repo) MirrorTemplate(ctx context.Context, rootID string, tg TemplateGroup) (MirrorResult, error) {
	var res MirrorResult
	inv, err := r.List(ctx)
	if err != nil {
		return res, err
	}
	groups := map[string]Group{}
	children := map[string][]Group{}
	for _, g := range inv.Groups {
		groups[g.ID] = g
		children[g.ParentID] = append(children[g.ParentID], g)
	}
	hostsIn := map[string][]Host{}
	for _, h := range inv.Hosts {
		hostsIn[h.GroupID] = append(hostsIn[h.GroupID], h)
	}
	root, ok := groups[rootID]
	if !ok {
		return res, r.requireGroup(ctx, rootID)
	}

	type pending struct {
		isGroup bool
		id      string
		jumps   []string
	}
	var jumps []pending
	inTree := map[string]string{} // lower host name → id, inside the mirror

	var mirror func(g Group, tg TemplateGroup, isRoot bool) error
	mirror = func(g Group, tg TemplateGroup, isRoot bool) error {
		name := g.Name
		if !isRoot {
			name = tg.Name
		}
		in := GroupInput{ParentID: g.ParentID, Name: name, Color: tg.Color, Settings: keepLocal(tg.Settings, g.Settings)}
		if _, err := r.UpdateGroup(ctx, g.ID, in); err != nil {
			return err
		}
		jumps = append(jumps, pending{isGroup: true, id: g.ID, jumps: tg.Jumps})

		// Hosts, by name.
		local := map[string]Host{}
		for _, h := range hostsIn[g.ID] {
			local[strings.ToLower(h.Name)] = h
		}
		seen := map[string]bool{}
		for _, th := range tg.Hosts {
			key := strings.ToLower(strings.TrimSpace(th.Name))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			if h, ok := local[key]; ok {
				_, err := r.UpdateHost(ctx, h.ID, HostInput{GroupID: g.ID, Name: th.Name, Hostname: th.Hostname, Notes: h.Notes, Tags: th.Tags,
					Settings: keepLocal(th.Settings, h.Settings), Favorite: h.Favorite})
				if err != nil {
					return err
				}
				inTree[key] = h.ID
				res.Updated++
				jumps = append(jumps, pending{id: h.ID, jumps: th.Jumps})
				continue
			}
			h, err := r.CreateHost(ctx, HostInput{GroupID: g.ID, Name: th.Name, Hostname: th.Hostname, Tags: th.Tags, Settings: keepLocal(th.Settings, settings.Settings{})})
			if err != nil {
				return err
			}
			inTree[key] = h.ID
			res.Added++
			jumps = append(jumps, pending{id: h.ID, jumps: th.Jumps})
		}
		for key, h := range local {
			if !seen[key] {
				if err := r.DeleteHost(ctx, h.ID); err != nil {
					return err
				}
				res.Removed++
			}
		}

		// Subgroups, by name.
		sub := map[string]Group{}
		for _, c := range children[g.ID] {
			sub[strings.ToLower(c.Name)] = c
		}
		kept := map[string]bool{}
		for _, child := range tg.Groups {
			key := strings.ToLower(strings.TrimSpace(child.Name))
			if key == "" || kept[key] {
				continue
			}
			kept[key] = true
			c, ok := sub[key]
			if !ok {
				created, err := r.CreateGroup(ctx, GroupInput{ParentID: g.ID, Name: child.Name, Color: child.Color})
				if err != nil {
					return err
				}
				c = created
			}
			if err := mirror(c, child, false); err != nil {
				return err
			}
		}
		for key, c := range sub {
			if !kept[key] {
				if err := r.DeleteGroup(ctx, c.ID); err != nil {
					return err
				}
				res.Removed++
			}
		}
		return nil
	}
	if err := mirror(root, tg, true); err != nil {
		return res, err
	}

	// Jump hosts: inside the mirror first, then anywhere.
	inv, err = r.List(ctx)
	if err != nil {
		return res, err
	}
	anywhere := map[string]string{}
	for _, h := range inv.Hosts {
		anywhere[strings.ToLower(h.Name)] = h.ID
	}
	resolve := func(names []string, self string) []string {
		var ids []string
		for _, n := range names {
			k := strings.ToLower(n)
			id, ok := inTree[k]
			if !ok {
				id, ok = anywhere[k]
			}
			if ok && id != self {
				ids = append(ids, id)
			}
		}
		return ids
	}
	for _, p := range jumps {
		ids := resolve(p.jumps, p.id)
		if p.isGroup {
			g, err := r.groupByID(ctx, p.id)
			if err != nil {
				return res, err
			}
			if !sameChain(g.Settings.JumpChain, ids) {
				g.Settings.JumpChain = ids
				if _, err := r.UpdateGroup(ctx, g.ID, GroupInput{ParentID: g.ParentID, Name: g.Name, Color: g.Color, Settings: g.Settings}); err != nil {
					return res, err
				}
			}
			continue
		}
		h, err := r.Host(ctx, p.id)
		if err != nil {
			return res, err
		}
		if !sameChain(h.Settings.JumpChain, ids) {
			h.Settings.JumpChain = ids
			if _, err := r.UpdateHost(ctx, h.ID, HostInput{GroupID: h.GroupID, Name: h.Name, Hostname: h.Hostname, Notes: h.Notes, Tags: h.Tags, Settings: h.Settings, Favorite: h.Favorite}); err != nil {
				return res, err
			}
		}
	}
	return res, nil
}

// keepLocal takes shared settings and keeps the local-only ones of the
// local copy where the shared ones leave them unset.
func keepLocal(shared, local settings.Settings) settings.Settings {
	shared.JumpChain = nil // linked by name afterwards
	if shared.IdentityID == nil {
		shared.IdentityID = local.IdentityID
	}
	if shared.TerminalThemeID == nil {
		shared.TerminalThemeID = local.TerminalThemeID
	}
	if shared.MACAddress == nil {
		shared.MACAddress = local.MACAddress
	}
	if shared.WakeBroadcast == nil {
		shared.WakeBroadcast = local.WakeBroadcast
	}
	if shared.KnockSequence == nil {
		shared.KnockSequence = local.KnockSequence
	}
	if shared.CredentialPlugin == nil {
		shared.CredentialPlugin = local.CredentialPlugin
	}
	return shared
}

func sameChain(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *Repo) groupByID(ctx context.Context, id string) (Group, error) {
	inv, err := r.List(ctx)
	if err != nil {
		return Group{}, err
	}
	for _, g := range inv.Groups {
		if g.ID == id {
			return g, nil
		}
	}
	return Group{}, r.requireGroup(ctx, id)
}
