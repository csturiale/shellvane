//go:build !windows

package localshell

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Programs lists the shells installed on this computer, the user's login
// shell first.
func Programs() []Program {
	var out []Program
	seen := map[string]bool{}
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		if fi, err := os.Stat(path); err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
			return
		}
		seen[path] = true
		name := filepath.Base(path)
		// A login shell reads the profile, like a new terminal window.
		out = append(out, Program{ID: path, Name: name, Path: path, Args: []string{"-l"}})
	}
	add(os.Getenv("SHELL"))
	if f, err := os.Open("/etc/shells"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" && !strings.HasPrefix(line, "#") && !strings.HasSuffix(line, "nologin") {
				add(line)
			}
		}
	}
	add("/bin/bash")
	add("/bin/sh")
	return dedupeByName(out)
}

// dedupeByName keeps one entry per shell name (/bin/bash and
// /usr/bin/bash are the same on merged-/usr systems).
func dedupeByName(list []Program) []Program {
	var out []Program
	names := map[string]bool{}
	for _, p := range list {
		real, err := filepath.EvalSymlinks(p.Path)
		if err != nil {
			real = p.Path
		}
		key := filepath.Base(real)
		if names[key] {
			continue
		}
		names[key] = true
		out = append(out, p)
	}
	return out
}
