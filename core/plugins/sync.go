package plugins

import (
	"context"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
)

// SyncResult tells what a refresh changed.
type SyncResult struct {
	GroupID string `json:"groupId"`
	Added   int    `json:"added"`
	Updated int    `json:"updated"`
	Removed int    `json:"removed"`
	Skipped int    `json:"skipped"` // hosts the inventory refused (bad address)
}

// Sync puts the hosts an inventory found in the configuration's group,
// subgroups following each host's Group path. Hosts seen before (by key)
// are updated, keeping what the user set on them (favorite, keys, colors,
// other settings); hosts that disappeared from the source are removed.
func Sync(ctx context.Context, repo *hosts.Repo, configs *Configs, cfg Config, found []Host) (SyncResult, error) {
	inv, err := repo.List(ctx)
	if err != nil {
		return SyncResult{}, err
	}
	groups := map[string]hosts.Group{}
	for _, g := range inv.Groups {
		groups[g.ID] = g
	}
	existing := map[string]hosts.Host{}
	for _, h := range inv.Hosts {
		existing[h.ID] = h
	}
	root := cfg.GroupID
	if _, ok := groups[root]; !ok || root == "" {
		g, err := repo.CreateGroup(ctx, hosts.GroupInput{Name: cfg.Name})
		if err != nil {
			return SyncResult{}, err
		}
		root = g.ID
		groups[g.ID] = g
	}
	tree := newGroupTree(repo, groups)
	known, err := configs.Hosts(ctx, cfg.ID)
	if err != nil {
		return SyncResult{}, err
	}
	res := SyncResult{GroupID: root}
	next := map[string]string{}
	for _, h := range found {
		if h.Key == "" || next[h.Key] != "" {
			continue
		}
		groupID, err := tree.ensure(ctx, root, h.Group)
		if err != nil {
			return res, err
		}
		in := hosts.HostInput{GroupID: groupID, Name: h.Name, Hostname: h.Hostname, Notes: h.Notes, Tags: h.Tags}
		if old, ok := existing[known[h.Key]]; ok {
			in.Settings, in.Favorite = old.Settings, old.Favorite
			apply(&in, h)
			if _, err := repo.UpdateHost(ctx, old.ID, in); err != nil {
				if isInvalid(err) {
					res.Skipped++
					next[h.Key] = old.ID
					continue
				}
				return res, err
			}
			next[h.Key] = old.ID
			res.Updated++
			continue
		}
		apply(&in, h)
		created, err := repo.CreateHost(ctx, in)
		if err != nil {
			if isInvalid(err) {
				res.Skipped++
				continue
			}
			return res, err
		}
		next[h.Key] = created.ID
		res.Added++
	}
	for key, id := range known {
		if _, still := next[key]; still {
			continue
		}
		if _, ok := existing[id]; ok {
			if err := repo.DeleteHost(ctx, id); err != nil {
				return res, err
			}
			res.Removed++
		}
	}
	if err := configs.SetHosts(ctx, cfg.ID, next); err != nil {
		return res, err
	}
	if err := pruneEmpty(ctx, repo, root); err != nil {
		return res, err
	}
	cfg.GroupID = root
	cfg.LastSync = configs.now().UTC().Format(time.RFC3339)
	if _, err := configs.Save(ctx, cfg); err != nil {
		return res, err
	}
	return res, nil
}

// apply copies the source's port and user over the host's settings.
func apply(in *hosts.HostInput, h Host) {
	if h.Port > 0 {
		port := h.Port
		in.Settings.Port = &port
	}
	if h.User != "" {
		user := h.User
		in.Settings.User = &user
	}
}

func isInvalid(err error) bool {
	switch errs.CodeOf(err) {
	case hosts.ErrHostnameRequired, hosts.ErrHostnameInvalid, hosts.ErrPortInvalid:
		return true
	}
	return false
}

// groupTree finds or creates subgroups by name.
type groupTree struct {
	repo     *hosts.Repo
	children map[string]map[string]string // parent → lower-case name → id
}

func newGroupTree(repo *hosts.Repo, groups map[string]hosts.Group) *groupTree {
	t := &groupTree{repo: repo, children: map[string]map[string]string{}}
	for _, g := range groups {
		t.add(g.ParentID, g.Name, g.ID)
	}
	return t
}

func (t *groupTree) add(parent, name, id string) {
	if t.children[parent] == nil {
		t.children[parent] = map[string]string{}
	}
	t.children[parent][strings.ToLower(name)] = id
}

func (t *groupTree) ensure(ctx context.Context, root string, path []string) (string, error) {
	cur := root
	for _, name := range path {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if id, ok := t.children[cur][strings.ToLower(name)]; ok {
			cur = id
			continue
		}
		g, err := t.repo.CreateGroup(ctx, hosts.GroupInput{ParentID: cur, Name: name})
		if err != nil {
			return "", err
		}
		t.add(cur, name, g.ID)
		cur = g.ID
	}
	return cur, nil
}

// pruneEmpty deletes the empty subgroups below root (never root itself).
func pruneEmpty(ctx context.Context, repo *hosts.Repo, root string) error {
	for {
		inv, err := repo.List(ctx)
		if err != nil {
			return err
		}
		used := map[string]bool{}
		parent := map[string]string{}
		for _, g := range inv.Groups {
			used[g.ParentID] = true
			parent[g.ID] = g.ParentID
		}
		for _, h := range inv.Hosts {
			used[h.GroupID] = true
		}
		below := func(id string) bool {
			for p := parent[id]; p != ""; p = parent[p] {
				if p == root {
					return true
				}
			}
			return false
		}
		removed := false
		for _, g := range inv.Groups {
			if !used[g.ID] && below(g.ID) {
				if err := repo.DeleteGroup(ctx, g.ID); err != nil {
					return err
				}
				removed = true
			}
		}
		if !removed {
			return nil
		}
	}
}
