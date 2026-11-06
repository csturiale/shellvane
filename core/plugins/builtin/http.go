package builtin

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

// client returns an HTTP client; with a pinned SHA-256 fingerprint it
// accepts exactly that certificate (self-signed ones, common on Proxmox)
// and nothing else.
func client(fingerprint string) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	pin := strings.ToLower(strings.NewReplacer(":", "", " ", "").Replace(fingerprint))
	if pin != "" {
		tr.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, // replaced by the pin check below
			VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
				if len(raw) == 0 {
					return errs.New(plugins.ErrResponse, errs.Params{"command": "tls"})
				}
				sum := sha256.Sum256(raw[0])
				if hex.EncodeToString(sum[:]) != pin {
					return errs.New(plugins.ErrResponse, errs.Params{"command": "tls fingerprint"})
				}
				return nil
			},
		}
	}
	return &http.Client{Transport: tr, Timeout: 60 * time.Second}
}

// getJSON fetches url with the authorization header and decodes it.
func getJSON(ctx context.Context, c *http.Client, rawURL, auth string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return errs.Wrap(err, plugins.ErrSetting, errs.Params{"key": "url"})
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		return errs.Wrap(err, plugins.ErrCommand, errs.Params{"command": hostOf(rawURL), "detail": err.Error()})
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return errs.New(plugins.ErrCommand, errs.Params{"command": hostOf(rawURL), "detail": resp.Status})
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(into); err != nil {
		return badJSON(err, hostOf(rawURL))
	}
	return nil
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil {
		return u.Host
	}
	return raw
}

func trimURL(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }

// Proxmox lists the virtual machines and containers of a Proxmox VE
// cluster through its API, with an API token from the vault.
type Proxmox struct {
	plugins.Base
	Client func(fingerprint string) *http.Client // tests replace it
}

// Manifest describes the plugin.
func (Proxmox) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "proxmox", Name: "Proxmox VE", Version: "1", Description: "plugins.proxmox.description", BuiltIn: true,
		Capabilities: []string{plugins.CapInventory},
		Settings: []plugins.Setting{
			{Key: "url", Label: "plugins.settings.url", Placeholder: "https://pve.example.org:8006", Required: true},
			{Key: "tokenId", Label: "plugins.settings.proxmoxTokenId", Placeholder: "shellvane@pve!inventory", Required: true},
			{Key: "token", Label: "plugins.settings.token", Secret: true, Required: true},
			{Key: "fingerprint", Label: "plugins.settings.fingerprint", Placeholder: "AB:CD:…"},
			{Key: "user", Label: "plugins.settings.user", Placeholder: "root"},
		},
	}
}

// Inventory lists the running guests, with their address from the guest
// agent (VMs) or the container's interfaces when available.
func (p Proxmox) Inventory(ctx context.Context, s map[string]string) ([]plugins.Host, error) {
	if err := plugins.Require(s, "url", "tokenId", "token"); err != nil {
		return nil, err
	}
	mk := p.Client
	if mk == nil {
		mk = client
	}
	c := mk(s["fingerprint"])
	base := trimURL(s["url"]) + "/api2/json"
	auth := "PVEAPIToken=" + strings.TrimSpace(s["tokenId"]) + "=" + strings.TrimSpace(s["token"])
	var res struct {
		Data []struct {
			VMID   int    `json:"vmid"`
			Name   string `json:"name"`
			Node   string `json:"node"`
			Type   string `json:"type"`
			Status string `json:"status"`
			Tags   string `json:"tags"`
		} `json:"data"`
	}
	if err := getJSON(ctx, c, base+"/cluster/resources?type=vm", auth, &res); err != nil {
		return nil, err
	}
	var hosts []plugins.Host
	for _, g := range res.Data {
		if g.Status != "running" {
			continue
		}
		addr := p.guestAddress(ctx, c, base, auth, g.Node, g.Type, g.VMID)
		var tags []string
		for _, t := range strings.FieldsFunc(g.Tags, func(r rune) bool { return r == ';' || r == ',' }) {
			tags = append(tags, strings.TrimSpace(t))
		}
		hosts = append(hosts, plugins.Host{
			Key: fmt.Sprintf("%s/%d", g.Type, g.VMID), Name: or(g.Name, fmt.Sprint(g.VMID)), Hostname: or(addr, g.Name),
			User: or(s["user"], "root"), Group: []string{g.Node}, Tags: tags,
			Notes: fmt.Sprintf("Proxmox %s %d on %s", g.Type, g.VMID, g.Node),
		})
	}
	return hosts, nil
}

// guestAddress returns the first non-loopback IPv4 of a guest, or "".
func (p Proxmox) guestAddress(ctx context.Context, c *http.Client, base, auth, node, kind string, vmid int) string {
	switch kind {
	case "qemu":
		var res struct {
			Data struct {
				Result []struct {
					IPAddresses []struct {
						Type    string `json:"ip-address-type"`
						Address string `json:"ip-address"`
					} `json:"ip-addresses"`
				} `json:"result"`
			} `json:"data"`
		}
		if getJSON(ctx, c, fmt.Sprintf("%s/nodes/%s/qemu/%d/agent/network-get-interfaces", base, node, vmid), auth, &res) != nil {
			return ""
		}
		for _, nic := range res.Data.Result {
			for _, a := range nic.IPAddresses {
				if a.Type == "ipv4" && !net.ParseIP(a.Address).IsLoopback() {
					return a.Address
				}
			}
		}
	case "lxc":
		var res struct {
			Data []struct {
				Inet string `json:"inet"`
			} `json:"data"`
		}
		if getJSON(ctx, c, fmt.Sprintf("%s/nodes/%s/lxc/%d/interfaces", base, node, vmid), auth, &res) != nil {
			return ""
		}
		for _, nic := range res.Data {
			ip, _, err := net.ParseCIDR(nic.Inet)
			if err == nil && ip.To4() != nil && !ip.IsLoopback() {
				return ip.String()
			}
		}
	}
	return ""
}

// NetBox lists the devices and virtual machines of a NetBox instance that
// have a primary IP, through its API.
type NetBox struct {
	plugins.Base
	Client func(fingerprint string) *http.Client
}

// Manifest describes the plugin.
func (NetBox) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "netbox", Name: "NetBox", Version: "1", Description: "plugins.netbox.description", BuiltIn: true,
		Capabilities: []string{plugins.CapInventory},
		Settings: []plugins.Setting{
			{Key: "url", Label: "plugins.settings.url", Placeholder: "https://netbox.example.org", Required: true},
			{Key: "token", Label: "plugins.settings.token", Secret: true, Required: true},
			{Key: "filter", Label: "plugins.settings.netboxFilter", Placeholder: "status=active&tag=ssh"},
			{Key: "user", Label: "plugins.settings.user"},
		},
	}
}

type netboxPage struct {
	Next    string `json:"next"`
	Results []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		PrimaryIP *struct {
			Address string `json:"address"`
		} `json:"primary_ip"`
		Site    *struct{ Name string }  `json:"site"`
		Cluster *struct{ Name string }  `json:"cluster"`
		Tags    []struct{ Name string } `json:"tags"`
		Status  *struct{ Value string } `json:"status"`
	} `json:"results"`
}

// Inventory lists devices and virtual machines, grouped by site or cluster.
func (p NetBox) Inventory(ctx context.Context, s map[string]string) ([]plugins.Host, error) {
	if err := plugins.Require(s, "url", "token"); err != nil {
		return nil, err
	}
	mk := p.Client
	if mk == nil {
		mk = client
	}
	c := mk("")
	auth := "Token " + strings.TrimSpace(s["token"])
	var hosts []plugins.Host
	for _, kind := range []string{"dcim/devices", "virtualization/virtual-machines"} {
		next := trimURL(s["url"]) + "/api/" + kind + "/?limit=500"
		if f := strings.TrimSpace(s["filter"]); f != "" {
			next += "&" + strings.TrimPrefix(f, "&")
		}
		for pages := 0; next != "" && pages < 50; pages++ {
			var page netboxPage
			if err := getJSON(ctx, c, next, auth, &page); err != nil {
				return nil, err
			}
			for _, r := range page.Results {
				if r.PrimaryIP == nil || r.PrimaryIP.Address == "" {
					continue
				}
				ip := strings.SplitN(r.PrimaryIP.Address, "/", 2)[0]
				group := "NetBox"
				if r.Site != nil && r.Site.Name != "" {
					group = r.Site.Name
				} else if r.Cluster != nil && r.Cluster.Name != "" {
					group = r.Cluster.Name
				}
				var tags []string
				for _, t := range r.Tags {
					tags = append(tags, t.Name)
				}
				hosts = append(hosts, plugins.Host{
					Key: fmt.Sprintf("%s/%d", kind, r.ID), Name: or(r.Name, ip), Hostname: ip, User: s["user"],
					Group: []string{group}, Tags: tags, Notes: "NetBox " + kind + " " + r.Name,
				})
			}
			next = page.Next
		}
	}
	return hosts, nil
}
