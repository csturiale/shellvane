package netproxy

import (
	"golang.org/x/net/http/httpproxy"
	"golang.org/x/sys/windows/registry"
)

// systemProxy reads the proxy of Windows' Internet settings (the user's
// ProxyEnable, ProxyServer and ProxyOverride). A proxy auto-configuration
// (PAC) script is not evaluated: set the proxy by hand in Settings ›
// Network then.
func systemProxy() *httpproxy.Config {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	enabled, _, err := k.GetIntegerValue("ProxyEnable")
	if err != nil || enabled == 0 {
		return nil
	}
	server, _, _ := k.GetStringValue("ProxyServer")
	override, _, _ := k.GetStringValue("ProxyOverride")
	return parseWindows(server, override)
}
