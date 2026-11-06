package app

import (
	"context"

	"github.com/csturiale/shellvane/core/workspaces"
)

// WorkspaceService saves and restores arrangements of tabs and sessions.
type WorkspaceService struct {
	repo *workspaces.Repo
}

// NewWorkspaceService returns a service over repo.
func NewWorkspaceService(repo *workspaces.Repo) *WorkspaceService {
	return &WorkspaceService{repo: repo}
}

// List returns the saved workspaces.
func (s *WorkspaceService) List(ctx context.Context) ([]workspaces.Workspace, error) {
	return s.repo.List(ctx)
}

// Save stores the layout under name, replacing a workspace with that name.
func (s *WorkspaceService) Save(ctx context.Context, name, layout string) (workspaces.Workspace, error) {
	return s.repo.Save(ctx, name, layout)
}

// Delete removes a workspace.
func (s *WorkspaceService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// SaveLast remembers the open tabs to restore them at the next start; an
// empty layout forgets them.
func (s *WorkspaceService) SaveLast(ctx context.Context, layout string) error {
	return s.repo.SaveLast(ctx, layout)
}

// Last returns the tabs open when the app quit, or "".
func (s *WorkspaceService) Last(ctx context.Context) (string, error) {
	return s.repo.Last(ctx)
}
