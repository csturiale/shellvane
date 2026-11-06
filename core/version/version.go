// Package version exposes build information, set at link time:
//
//	go build -ldflags "-X github.com/csturiale/shellvane/core/version.Version=1.2.3"
package version

// Version is the application version. "dev" for local builds.
var Version = "dev"

// Commit is the VCS revision the binary was built from.
var Commit = ""

// UserAgent identifies Shellvane in requests to its website (update checks,
// gallery, plugin store): only the version and the platform, never anything
// about the person or the computer, so downloads and active versions can be
// counted without tracking anyone.
func UserAgent() string {
	return "Shellvane/" + Version + " (" + goos + "/" + goarch + ")"
}

// Date is the release day (YYYY-MM-DD) of official builds, set at link
// time; empty for local builds.
var Date = ""

// SiteURL is the project's website: update manifest, gallery and plugin
// store. Change it here (and in frontend/src/app/core/site.ts) to move.
const SiteURL = "https://termoraplus.com"
