package app

import (
	"context"

	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/settings"
)

// HostService exposes the host inventory to the frontend.
type HostService struct {
	repo *hosts.Repo
}

// NewHostService returns a service backed by repo.
func NewHostService(repo *hosts.Repo) *HostService {
	return &HostService{repo: repo}
}

// Inventory returns all groups and hosts.
func (s *HostService) Inventory(ctx context.Context) (hosts.Inventory, error) {
	return s.repo.List(ctx)
}

// CreateGroup adds a group.
func (s *HostService) CreateGroup(ctx context.Context, in hosts.GroupInput) (hosts.Group, error) {
	return s.repo.CreateGroup(ctx, in)
}

// UpdateGroup edits a group.
func (s *HostService) UpdateGroup(ctx context.Context, id string, in hosts.GroupInput) (hosts.Group, error) {
	return s.repo.UpdateGroup(ctx, id, in)
}

// MoveGroup changes a group's parent; "" is the root.
func (s *HostService) MoveGroup(ctx context.Context, id, parentID string) error {
	return s.repo.MoveGroup(ctx, id, parentID)
}

// GroupContents counts what deleting a group would remove.
func (s *HostService) GroupContents(ctx context.Context, id string) (hosts.GroupContents, error) {
	return s.repo.Contents(ctx, id)
}

// DeleteGroup deletes a group and everything in it.
func (s *HostService) DeleteGroup(ctx context.Context, id string) error {
	return s.repo.DeleteGroup(ctx, id)
}

// CreateHost adds a host.
func (s *HostService) CreateHost(ctx context.Context, in hosts.HostInput) (hosts.Host, error) {
	return s.repo.CreateHost(ctx, in)
}

// UpdateHost edits a host.
func (s *HostService) UpdateHost(ctx context.Context, id string, in hosts.HostInput) (hosts.Host, error) {
	return s.repo.UpdateHost(ctx, id, in)
}

// MoveHost puts a host in another group; "" is the root.
func (s *HostService) MoveHost(ctx context.Context, id, groupID string) error {
	return s.repo.MoveHost(ctx, id, groupID)
}

// DeleteHost deletes a host.
func (s *HostService) DeleteHost(ctx context.Context, id string) error {
	return s.repo.DeleteHost(ctx, id)
}

// Inherited returns the settings a host in groupID inherits, with the ID of
// the group that set each one (empty for built-in defaults). Forms use it to
// show inherited values as placeholders.
func (s *HostService) Inherited(ctx context.Context, groupID string) (settings.Resolved, error) {
	chain, err := s.repo.Chain(ctx, groupID)
	if err != nil {
		return settings.Resolved{}, err
	}
	return settings.Resolve(chain...), nil
}

// SetFavorite marks or unmarks a host as a favorite.
func (s *HostService) SetFavorite(ctx context.Context, id string, on bool) error {
	return s.repo.SetFavorite(ctx, id, on)
}

// DuplicateHost copies a host; suffix is appended to its name (the UI
// passes the translated " (copy)").
func (s *HostService) DuplicateHost(ctx context.Context, id, suffix string) (hosts.Host, error) {
	return s.repo.DuplicateHost(ctx, id, suffix)
}

// ExportTemplate renders group groupID and its subgroups (every group when
// empty) with their settings and, when withHosts is set, their hosts; vault
// keys, notes and secrets are left out.
func (s *HostService) ExportTemplate(ctx context.Context, groupID string, withHosts bool) (string, error) {
	inv, err := s.repo.List(ctx)
	if err != nil {
		return "", err
	}
	t, err := hosts.ExportTemplate(inv, groupID, withHosts)
	if err != nil {
		return "", err
	}
	return hosts.MarshalTemplate(t), nil
}

// ImportTemplate creates an export's groups and hosts under parentID
// (empty for the top level).
func (s *HostService) ImportTemplate(ctx context.Context, parentID, text string) (hosts.TemplateResult, error) {
	t, err := hosts.ParseTemplate(text)
	if err != nil {
		return hosts.TemplateResult{}, err
	}
	// A pasted export may come from anyone: what would run on this computer
	// or on the servers is left out, and listed, for the person to add by
	// hand if they trust it.
	var left []string
	for i, g := range t.Groups {
		left = append(left, GroupCommands(g)...)
		t.Groups[i] = StripCommands(g)
	}
	res, err := s.repo.ImportTemplate(ctx, parentID, t)
	res.CommandsLeftOut = left
	return res, err
}
