package sync

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// CloudFolder is a folder kept in step by a cloud drive's desktop client.
type CloudFolder struct {
	// Service is "icloud", "googleDrive", "dropbox", "oneDrive" or
	// "syncthing".
	Service string `json:"service"`
	Path    string `json:"path"`
}

// CloudFolders finds the folders of the cloud drives installed on this
// computer, in their usual places. Each one works as a shared folder: the
// drive's own client does the syncing.
func CloudFolders() []CloudFolder {
	home, err := os.UserHomeDir()
	if err != nil {
		return []CloudFolder{}
	}
	return cloudFolders(runtime.GOOS, home, os.Getenv, driveRoots())
}

func cloudFolders(goos, home string, getenv func(string) string, roots []string) []CloudFolder {
	out := []CloudFolder{}
	seen := map[string]bool{}
	add := func(service, path string) {
		if path == "" || seen[path] || !isDir(path) {
			return
		}
		seen[path] = true
		out = append(out, CloudFolder{Service: service, Path: path})
	}
	cloudStorage := filepath.Join(home, "Library", "CloudStorage")

	// iCloud Drive.
	switch goos {
	case "darwin":
		add("icloud", filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs"))
	case "windows":
		add("icloud", filepath.Join(home, "iCloudDrive"))
	}

	// Google Drive for desktop: a folder under CloudStorage on macOS, a
	// drive letter on Windows; its root holds .shortcut-targets-by-id.
	if goos == "darwin" {
		for _, d := range glob(filepath.Join(cloudStorage, "GoogleDrive-*")) {
			add("googleDrive", myDrive(d))
		}
	}
	for _, r := range roots {
		if isDir(filepath.Join(r, ".shortcut-targets-by-id")) {
			add("googleDrive", myDrive(r))
		}
	}
	add("googleDrive", filepath.Join(home, "Google Drive")) // older clients

	// Dropbox.
	if goos == "darwin" {
		for _, d := range glob(filepath.Join(cloudStorage, "Dropbox*")) {
			add("dropbox", d)
		}
	}
	add("dropbox", filepath.Join(home, "Dropbox"))

	// OneDrive.
	if goos == "windows" {
		add("oneDrive", getenv("OneDrive"))
		add("oneDrive", getenv("OneDriveCommercial"))
	}
	if goos == "darwin" {
		for _, d := range glob(filepath.Join(cloudStorage, "OneDrive*")) {
			add("oneDrive", d)
		}
	}
	add("oneDrive", filepath.Join(home, "OneDrive"))

	// Syncthing's default folder.
	add("syncthing", filepath.Join(home, "Sync"))
	return out
}

// myDrive is the personal folder of a Google Drive root ("My Drive",
// translated in other languages), or the root when it cannot be told.
func myDrive(root string) string {
	if p := filepath.Join(root, "My Drive"); isDir(p) {
		return p
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return root
	}
	var dirs []string
	for _, e := range entries {
		n := e.Name()
		lower := strings.ToLower(n)
		if !e.IsDir() || strings.HasPrefix(n, ".") || strings.Contains(lower, "shared") || strings.Contains(lower, "condivis") || strings.Contains(lower, "partag") || strings.Contains(lower, "freigegeben") || strings.Contains(lower, "compartid") {
			continue
		}
		dirs = append(dirs, filepath.Join(root, n))
	}
	sort.Strings(dirs)
	if len(dirs) == 1 {
		return dirs[0]
	}
	return root
}

func glob(pattern string) []string {
	m, _ := filepath.Glob(pattern)
	sort.Strings(m)
	return m
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// driveRoots are the drive letters on Windows (Google Drive mounts one).
func driveRoots() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	var out []string
	for c := 'D'; c <= 'Z'; c++ {
		r := string(c) + `:\`
		if isDir(r) {
			out = append(out, r)
		}
	}
	return out
}
