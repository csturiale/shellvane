package netproxy

import (
	"context"
	"os/exec"
	"time"

	"golang.org/x/net/http/httpproxy"
)

// systemProxy reads the proxies of System Settings › Network (scutil
// --proxy). A proxy auto-configuration (PAC) file is not evaluated: set the
// proxy by hand in Settings › Network then.
func systemProxy() *httpproxy.Config {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/sbin/scutil", "--proxy").Output()
	if err != nil {
		return nil
	}
	return parseScutil(string(out))
}
