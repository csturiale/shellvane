//go:build (!darwin || !cgo) && !windows

package app

// EnableClientCertificates does nothing here: Linux has no system store of
// certificates with their keys for WebKitGTK or the app to choose from
// (a certificate file can be set in a plugin's settings).
func EnableClientCertificates() {}
