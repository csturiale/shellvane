//go:build !darwin || !cgo

package app

// EnableClientCertificates is needed on macOS only: WebView2 (Windows)
// shows the system's certificate chooser by itself when a site asks for a
// client certificate; on Linux WebKitGTK has no system certificate store
// to choose from.
func EnableClientCertificates() {}
