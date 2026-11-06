package app

import (
	"context"

	"github.com/csturiale/shellvane/core/snippets"
)

// SnippetService exposes the snippet library.
type SnippetService struct {
	repo *snippets.Repo
}

// NewSnippetService returns a service over repo.
func NewSnippetService(repo *snippets.Repo) *SnippetService { return &SnippetService{repo: repo} }

// List returns all snippets.
func (s *SnippetService) List(ctx context.Context) ([]snippets.Snippet, error) {
	return s.repo.List(ctx)
}

// Create adds a snippet.
func (s *SnippetService) Create(ctx context.Context, in snippets.Input) (snippets.Snippet, error) {
	return s.repo.Create(ctx, in)
}

// Update edits a snippet.
func (s *SnippetService) Update(ctx context.Context, id string, in snippets.Input) (snippets.Snippet, error) {
	return s.repo.Update(ctx, id, in)
}

// Delete removes a snippet.
func (s *SnippetService) Delete(ctx context.Context, id string) error { return s.repo.Delete(ctx, id) }

// Export renders the snippets as a Shellvane snippets file; ids picks some,
// empty exports them all.
func (s *SnippetService) Export(ctx context.Context, ids []string) (string, error) {
	list, err := s.repo.List(ctx)
	if err != nil {
		return "", err
	}
	if len(ids) > 0 {
		want := map[string]bool{}
		for _, id := range ids {
			want[id] = true
		}
		var picked []snippets.Snippet
		for _, sn := range list {
			if want[sn.ID] {
				picked = append(picked, sn)
			}
		}
		list = picked
	}
	return snippets.Export(list), nil
}

// Import reads a Shellvane snippets file or a shell script / list of
// commands and saves the snippets not already present.
func (s *SnippetService) Import(ctx context.Context, fileName, content string) (snippets.ImportResult, error) {
	in, err := snippets.Parse(fileName, content)
	if err != nil {
		return snippets.ImportResult{}, err
	}
	return s.repo.Import(ctx, in)
}
