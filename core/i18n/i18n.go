// Package i18n translates strings rendered by Go itself: native menus, tray,
// system dialogs and the CLI. Strings rendered by the web UI are translated
// in the frontend (see frontend/public/i18n).
//
// English is the source language. A missing translation falls back to English.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"sync"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.json
var localeFS embed.FS

// Source is the language every message is written in first.
var Source = language.English

var loadBundle = sync.OnceValues(func() (*goi18n.Bundle, error) {
	b := goi18n.NewBundle(Source)
	b.RegisterUnmarshalFunc("json", json.Unmarshal)
	files, err := fs.Glob(localeFS, "locales/*.json")
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if _, err := b.LoadMessageFileFS(localeFS, f); err != nil {
			return nil, fmt.Errorf("load %s: %w", f, err)
		}
	}
	return b, nil
})

// Supported returns the languages that have a locale file, source first.
func Supported() []language.Tag {
	b, err := loadBundle()
	if err != nil {
		return []language.Tag{Source}
	}
	return b.LanguageTags()
}

// Translator renders messages in one language.
type Translator struct {
	lang      language.Tag
	localizer *goi18n.Localizer
}

// New returns a Translator for the best supported match of the preferred
// languages (BCP 47 tags such as "it-IT"), falling back to English.
func New(preferred ...string) *Translator {
	b, err := loadBundle()
	if err != nil {
		// Locale files are embedded and covered by tests; this cannot happen
		// in a released binary.
		panic(err)
	}
	tag, _, _ := language.NewMatcher(b.LanguageTags()).Match(parseTags(preferred)...)
	base, _ := tag.Base()
	lang := language.Make(base.String())
	return &Translator{lang: lang, localizer: goi18n.NewLocalizer(b, lang.String(), Source.String())}
}

// Language returns the language actually used.
func (t *Translator) Language() language.Tag { return t.lang }

// T renders the message id with data. Unknown ids render as the id itself so
// that a missing key is visible rather than silent.
func (t *Translator) T(id string, data map[string]any) string {
	s, err := t.localizer.Localize(&goi18n.LocalizeConfig{MessageID: id, TemplateData: data})
	if err != nil && s == "" {
		return id
	}
	return s
}

// N renders a plural message; count is also available to the template as .Count.
func (t *Translator) N(id string, count int, data map[string]any) string {
	if data == nil {
		data = map[string]any{}
	}
	data["Count"] = count
	s, err := t.localizer.Localize(&goi18n.LocalizeConfig{MessageID: id, TemplateData: data, PluralCount: count})
	if err != nil && s == "" {
		return id
	}
	return s
}

// FromEnv returns the user's language from the POSIX locale variables, e.g.
// "it_IT.UTF-8" becomes "it-IT". It returns "" when nothing is set.
// The GUI gets the OS language from the webview instead; this is for the CLI.
func FromEnv() string {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		val := os.Getenv(v)
		if val == "" || val == "C" || val == "POSIX" {
			continue
		}
		val, _, _ = strings.Cut(val, ".")
		val, _, _ = strings.Cut(val, "@")
		return strings.ReplaceAll(val, "_", "-")
	}
	return ""
}

func parseTags(langs []string) []language.Tag {
	tags := make([]language.Tag, 0, len(langs))
	for _, l := range langs {
		if tag, err := language.Parse(l); err == nil {
			tags = append(tags, tag)
		}
	}
	return tags
}

// localeFiles lists the embedded locale files; used by tests.
func localeFiles() ([]string, error) {
	files, err := fs.Glob(localeFS, "locales/*.json")
	for i, f := range files {
		files[i] = path.Base(f)
	}
	return files, err
}
