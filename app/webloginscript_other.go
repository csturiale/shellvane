//go:build (!darwin && !linux) || !cgo || (linux && (gtk3 || android || server))

package app

import "github.com/wailsapp/wails/v3/pkg/application"

// addDocumentStartScript is not available here: on Windows the script is
// given with the window's options instead (see WebLoginWindow); elsewhere
// (Linux with GTK 3) the sign-in relies on the probe run while the page is
// open.
func addDocumentStartScript(*application.WebviewWindow, string) bool { return false }
