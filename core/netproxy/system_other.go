//go:build !darwin && !windows

package netproxy

import "golang.org/x/net/http/httpproxy"

// systemProxy: on Linux the proxy comes from the environment variables
// (also those of the login shell, read at startup).
func systemProxy() *httpproxy.Config { return nil }
