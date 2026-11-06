package themes

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/csturiale/shellvane/core/errs"
)

// ansiNames are ColorKeys' 16 ANSI colors in order (color0..color15).
var ansiNames = ColorKeys[5:]

// Parse reads color schemes in the format of iTerm2 (.itermcolors),
// Windows Terminal (JSON, a scheme or a settings file), Base16/Base24
// (YAML), Alacritty (TOML or YAML), Kitty and Ghostty. The format comes
// from the file name, else from the content. Themes are not normalized.
func Parse(fileName string, data []byte) ([]Theme, error) {
	base := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	text := strings.TrimPrefix(string(data), "\ufeff")
	trimmed := strings.TrimSpace(text)
	var (
		out []Theme
		err error
	)
	switch ext := strings.ToLower(filepath.Ext(fileName)); {
	case ext == ".itermcolors" || strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<plist"):
		out, err = parseITerm(data, base)
	case ext == ".json" || strings.HasPrefix(trimmed, "{"):
		out, err = parseWindowsTerminal(data, base)
	case ext == ".toml" || strings.Contains(text, "[colors."):
		out, err = parseAlacrittyTOML(text, base)
	case ext == ".yaml" || ext == ".yml":
		out, err = parseYAML(data, base)
	case strings.Contains(text, "palette") && strings.Contains(text, "="):
		out, err = parseGhostty(text, base)
	default:
		out, err = parseKitty(text, base)
		if err != nil {
			out, err = parseYAML(data, base)
		}
	}
	if err != nil {
		return nil, errs.Wrap(err, ErrFormat, nil)
	}
	if len(out) == 0 {
		return nil, errs.New(ErrFormat, nil)
	}
	return out, nil
}

func newTheme(name string) Theme { return Theme{Name: name, Colors: map[string]string{}} }

// set stores a color when it parses; bad values are left out.
func (t *Theme) set(key, value string) {
	if c, err := NormalizeColor(value); err == nil {
		t.Colors[key] = c
	}
}

// --- iTerm2 -----------------------------------------------------------

// parseITerm reads a plist: a dict of "Ansi 0 Color", "Background Color"...
// each a dict of "Red/Green/Blue Component" reals between 0 and 1.
func parseITerm(data []byte, name string) ([]Theme, error) {
	type node struct {
		XMLName xml.Name
		Content string `xml:",chardata"`
		Nodes   []node `xml:",any"`
	}
	var doc node
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	root := doc
	if root.XMLName.Local == "plist" && len(root.Nodes) > 0 {
		root = root.Nodes[0]
	}
	if root.XMLName.Local != "dict" {
		return nil, fmt.Errorf("no dict")
	}
	keys := map[string]string{
		"Background Color": "background", "Foreground Color": "foreground", "Cursor Color": "cursor",
		"Cursor Text Color": "cursorAccent", "Selection Color": "selectionBackground",
	}
	for i, n := range ansiNames {
		keys[fmt.Sprintf("Ansi %d Color", i)] = n
	}
	t := newTheme(name)
	for i := 0; i+1 < len(root.Nodes); i += 2 {
		k, v := root.Nodes[i], root.Nodes[i+1]
		target, ok := keys[strings.TrimSpace(k.Content)]
		if k.XMLName.Local != "key" || !ok || v.XMLName.Local != "dict" {
			continue
		}
		comp := map[string]float64{}
		for j := 0; j+1 < len(v.Nodes); j += 2 {
			f, err := strconv.ParseFloat(strings.TrimSpace(v.Nodes[j+1].Content), 64)
			if err == nil {
				comp[strings.TrimSpace(v.Nodes[j].Content)] = f
			}
		}
		to8 := func(f float64) int { return min(255, max(0, int(f*255+0.5))) }
		t.Colors[target] = fmt.Sprintf("#%02x%02x%02x", to8(comp["Red Component"]), to8(comp["Green Component"]), to8(comp["Blue Component"]))
	}
	return []Theme{t}, nil
}

// --- Windows Terminal -------------------------------------------------

func parseWindowsTerminal(data []byte, name string) ([]Theme, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	var schemes []map[string]any
	if list, ok := raw["schemes"].([]any); ok {
		for _, s := range list {
			if m, ok := s.(map[string]any); ok {
				schemes = append(schemes, m)
			}
		}
	} else {
		schemes = []map[string]any{raw}
	}
	keys := map[string]string{
		"background": "background", "foreground": "foreground", "cursorColor": "cursor", "selectionBackground": "selectionBackground",
		"black": "black", "red": "red", "green": "green", "yellow": "yellow", "blue": "blue", "purple": "magenta", "cyan": "cyan", "white": "white",
		"brightBlack": "brightBlack", "brightRed": "brightRed", "brightGreen": "brightGreen", "brightYellow": "brightYellow",
		"brightBlue": "brightBlue", "brightPurple": "brightMagenta", "brightCyan": "brightCyan", "brightWhite": "brightWhite",
	}
	var out []Theme
	for _, s := range schemes {
		t := newTheme(name)
		if n, ok := s["name"].(string); ok && n != "" {
			t.Name = n
		}
		for from, to := range keys {
			if v, ok := s[from].(string); ok {
				t.set(to, v)
			}
		}
		if t.Colors["background"] != "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// --- Base16 / Base24 and Alacritty YAML --------------------------------

func parseYAML(data []byte, name string) ([]Theme, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if colors, ok := raw["colors"].(map[string]any); ok {
		return []Theme{alacritty(colors, name)}, nil
	}
	base := raw
	if p, ok := raw["palette"].(map[string]any); ok {
		base = p // the 2023 Base16/Base24 format nests the colors
	}
	get := func(k string) string {
		for _, key := range []string{k, strings.ToUpper(k[:4]) + k[4:], "base" + strings.ToUpper(k[4:])} {
			if v, ok := base[key]; ok {
				return fmt.Sprint(v)
			}
		}
		return ""
	}
	if get("base00") == "" {
		return nil, fmt.Errorf("no base16 colors")
	}
	t := newTheme(name)
	for _, k := range []string{"scheme", "name"} {
		if v, ok := raw[k].(string); ok && v != "" {
			t.Name = v
		}
	}
	// The usual base16-shell mapping; Base24 adds bright colors in base12..17.
	m := map[string]string{
		"background": "base00", "foreground": "base05", "cursor": "base05", "selectionBackground": "base02",
		"black": "base00", "red": "base08", "green": "base0B", "yellow": "base0A", "blue": "base0D", "magenta": "base0E", "cyan": "base0C", "white": "base05",
		"brightBlack": "base03", "brightRed": "base08", "brightGreen": "base0B", "brightYellow": "base0A", "brightBlue": "base0D",
		"brightMagenta": "base0E", "brightCyan": "base0C", "brightWhite": "base07",
	}
	if get("base12") != "" {
		for k, v := range map[string]string{"brightRed": "base12", "brightYellow": "base13", "brightGreen": "base14", "brightCyan": "base15", "brightBlue": "base16", "brightMagenta": "base17"} {
			m[k] = v
		}
	}
	for to, from := range m {
		t.set(to, get(strings.ToLower(from)))
	}
	return []Theme{t}, nil
}

// alacritty reads the colors table (YAML or TOML, same shape).
func alacritty(colors map[string]any, name string) Theme {
	t := newTheme(name)
	sub := func(k string) map[string]any { m, _ := colors[k].(map[string]any); return m }
	str := func(m map[string]any, k string) string { v, _ := m[k].(string); return v }
	t.set("background", str(sub("primary"), "background"))
	t.set("foreground", str(sub("primary"), "foreground"))
	t.set("cursor", str(sub("cursor"), "cursor"))
	t.set("cursorAccent", str(sub("cursor"), "text"))
	t.set("selectionBackground", str(sub("selection"), "background"))
	for i, n := range []string{"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white"} {
		t.set(ansiNames[i], str(sub("normal"), n))
		t.set(ansiNames[8+i], str(sub("bright"), n))
	}
	return t
}

// parseAlacrittyTOML reads the [colors.*] tables of alacritty.toml (only
// "key = value" lines are needed).
func parseAlacrittyTOML(text, name string) ([]Theme, error) {
	colors := map[string]any{}
	var table map[string]any
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case line == "":
		case strings.HasPrefix(line, "["):
			section := strings.Trim(line, "[] ")
			table = nil
			if sub, ok := strings.CutPrefix(section, "colors."); ok {
				table = map[string]any{}
				colors[sub] = table
			}
		case table != nil:
			k, v, ok := strings.Cut(line, "=")
			if ok {
				table[strings.TrimSpace(k)] = tomlString(strings.TrimSpace(v))
			}
		}
	}
	if len(colors) == 0 {
		return nil, fmt.Errorf("no colors")
	}
	return []Theme{alacritty(colors, name)}, nil
}

// tomlString reads a quoted value, dropping a trailing comment.
func tomlString(v string) string {
	if v != "" && (v[0] == '"' || v[0] == '\'') {
		if end := strings.IndexByte(v[1:], v[0]); end >= 0 {
			return v[1 : end+1]
		}
	}
	if i := strings.Index(v, "#"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// --- Kitty and Ghostty -------------------------------------------------

func parseKitty(text, name string) ([]Theme, error) {
	t := newTheme(name)
	keys := map[string]string{"background": "background", "foreground": "foreground", "cursor": "cursor", "cursor_text_color": "cursorAccent", "selection_background": "selectionBackground"}
	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 2 || strings.HasPrefix(f[0], "#") {
			continue
		}
		if to, ok := keys[f[0]]; ok {
			t.set(to, f[1])
		} else if n, ok := strings.CutPrefix(f[0], "color"); ok {
			if i, err := strconv.Atoi(n); err == nil && i >= 0 && i < 16 {
				t.set(ansiNames[i], f[1])
			}
		}
	}
	if t.Colors["background"] == "" && t.Colors["black"] == "" {
		return nil, fmt.Errorf("no kitty colors")
	}
	return []Theme{t}, nil
}

func parseGhostty(text, name string) ([]Theme, error) {
	t := newTheme(name)
	keys := map[string]string{"background": "background", "foreground": "foreground", "cursor-color": "cursor", "cursor-text": "cursorAccent", "selection-background": "selectionBackground"}
	sc := bufio.NewScanner(bytes.NewReader([]byte(text)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		k, v, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(line, "#") {
			continue
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		if k == "palette" {
			idx, color, ok := strings.Cut(v, "=")
			if i, err := strconv.Atoi(strings.TrimSpace(idx)); ok && err == nil && i >= 0 && i < 16 {
				t.set(ansiNames[i], color)
			}
		} else if to, ok := keys[k]; ok {
			t.set(to, v)
		}
	}
	return []Theme{t}, nil
}
