package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

// DialogService opens the system's file and folder choosers for the
// fields that take a path on this computer.
type DialogService struct{}

// NewDialogService returns the service.
func NewDialogService() *DialogService { return &DialogService{} }

// ChooseFile asks for a file. current is the field's value: the chooser
// opens in its folder. Hidden files are shown (keys and certificates
// often live in ~/.ssh, ~/.kube…). It returns "" when cancelled.
func (s *DialogService) ChooseFile(title, current string) (string, error) {
	a := application.Get()
	if a == nil {
		return "", errs.New(plugins.ErrUnsupported, nil)
	}
	d := a.Dialog.OpenFile().SetTitle(title).CanChooseFiles(true).CanChooseDirectories(false).ShowHiddenFiles(true).ResolvesAliases(true)
	if dir := startDir(current); dir != "" {
		d = d.SetDirectory(dir)
	}
	path, err := d.PromptForSingleSelection()
	if err != nil {
		return "", nil // cancelled
	}
	return path, nil
}

// ChooseFolder asks for a folder, opening at current when it exists. It
// returns "" when cancelled.
func (s *DialogService) ChooseFolder(title, current string) (string, error) {
	a := application.Get()
	if a == nil {
		return "", errs.New(plugins.ErrUnsupported, nil)
	}
	d := a.Dialog.OpenFile().SetTitle(title).CanChooseFiles(false).CanChooseDirectories(true).CanCreateDirectories(true).ShowHiddenFiles(true)
	if dir := startDir(current); dir != "" {
		d = d.SetDirectory(dir)
	}
	path, err := d.PromptForSingleSelection()
	if err != nil {
		return "", nil // cancelled
	}
	return path, nil
}

// startDir is where a chooser opens for a field holding current: the
// folder itself, or a file's folder; ~ is the home folder. "" when it
// does not exist (the system's default).
func startDir(current string) string {
	p := strings.TrimSpace(current)
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		p = filepath.Join(home, p[1:])
	}
	for _, d := range []string{p, filepath.Dir(p)} {
		if st, err := os.Stat(d); err == nil && st.IsDir() {
			return d
		}
	}
	return ""
}
