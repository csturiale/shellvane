package app

import (
	"context"
	"encoding/json"

	"github.com/csturiale/shellvane/core/themes"
)

// ThemeService stores custom terminal color schemes and imports them from
// other terminals' files.
type ThemeService struct {
	repo *themes.Repo
}

// NewThemeService returns a service over the theme repository.
func NewThemeService(repo *themes.Repo) *ThemeService { return &ThemeService{repo: repo} }

// List returns the custom schemes.
func (s *ThemeService) List(ctx context.Context) ([]themes.Theme, error) {
	return s.repo.List(ctx)
}

// Save creates (empty ID) or replaces a scheme.
func (s *ThemeService) Save(ctx context.Context, t themes.Theme) (themes.Theme, error) {
	return s.repo.Save(ctx, t)
}

// Delete removes a scheme.
func (s *ThemeService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// Import reads schemes from a file's name and content (iTerm2, Windows
// Terminal, Base16/24, Alacritty, Kitty, Ghostty) and saves them, renaming
// those whose name is taken.
func (s *ThemeService) Import(ctx context.Context, fileName, content string) ([]themes.Theme, error) {
	parsed, err := themes.Parse(fileName, []byte(content))
	if err != nil {
		return nil, err
	}
	var out []themes.Theme
	for _, t := range parsed {
		if err := t.Normalize(); err != nil {
			return out, err
		}
		if t.Name, err = s.repo.FreeName(ctx, t.Name); err != nil {
			return out, err
		}
		saved, err := s.repo.Save(ctx, t)
		if err != nil {
			return out, err
		}
		out = append(out, saved)
	}
	return out, nil
}

// Export returns a scheme as a Windows Terminal scheme (JSON), a format
// most terminals can import.
func (s *ThemeService) Export(t themes.Theme) (string, error) {
	if err := t.Normalize(); err != nil {
		return "", err
	}
	names := map[string]string{"cursor": "cursorColor", "magenta": "purple", "brightMagenta": "brightPurple"}
	out := map[string]string{"name": t.Name}
	for k, v := range t.Colors {
		if k == "cursorAccent" {
			continue
		}
		if n, ok := names[k]; ok {
			k = n
		}
		out[k] = v
	}
	data, err := json.MarshalIndent(out, "", "  ")
	return string(data), err
}
