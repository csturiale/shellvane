// Package app is the service layer exposed to the frontend through Wails
// bindings. It adapts core packages to the UI and holds no business logic.
package app

import (
	"runtime"

	"github.com/csturiale/shellvane/core/i18n"
	"github.com/csturiale/shellvane/core/version"
)

// Info describes the running application.
type Info struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// AppService exposes application-level information and preferences.
type AppService struct {
	// OnLanguageChange is called after the UI language changes, so that
	// Go-rendered strings (menus, tray) can be rebuilt.
	OnLanguageChange func(*i18n.Translator)

	tr *i18n.Translator
}

// NewAppService returns a service using the given initial language.
func NewAppService(lang string) *AppService {
	return &AppService{tr: i18n.New(lang)}
}

// Info returns build and platform information.
func (s *AppService) Info() Info {
	return Info{
		Name:    s.tr.T("app.name", nil),
		Version: version.Version,
		Commit:  version.Commit,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}

// SupportedLanguages returns the languages available for Go-rendered strings.
func (s *AppService) SupportedLanguages() []string {
	tags := i18n.Supported()
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = t.String()
	}
	return out
}

// TranslatorOf returns the translator of Go-rendered strings (a function,
// not a method, so that it is not exposed to the frontend).
func TranslatorOf(s *AppService) *i18n.Translator { return s.tr }

// SetLanguage switches the language of Go-rendered strings and returns the
// language actually applied (the best supported match).
func (s *AppService) SetLanguage(lang string) string {
	s.tr = i18n.New(lang)
	if s.OnLanguageChange != nil {
		s.OnLanguageChange(s.tr)
	}
	return s.tr.Language().String()
}
