package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/csturiale/shellvane/core/plugins"
)

// Kubernetes lists a cluster's nodes with kubectl.
type Kubernetes struct {
	plugins.Base
	Cmd Runner
}

// Manifest describes the plugin.
func (Kubernetes) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "kubernetes-nodes", Name: "Kubernetes nodes", Version: "1", Description: "plugins.kubernetes.description", BuiltIn: true,
		Capabilities: []string{plugins.CapInventory}, Requires: []string{"kubectl"},
		Settings: []plugins.Setting{
			{Key: "context", Label: "plugins.settings.kubeContext"},
			addressSetting,
			{Key: "user", Label: "plugins.settings.user"},
		},
	}
}

// Inventory lists the nodes.
func (p Kubernetes) Inventory(ctx context.Context, s map[string]string) ([]plugins.Host, error) {
	args := appendIf([]string{"get", "nodes", "-o", "json"}, "--context", s["context"])
	out, err := p.Cmd(ctx, "kubectl", args...)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				UID    string            `json:"uid"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Addresses []struct{ Type, Address string } `json:"addresses"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, badJSON(err, "kubectl")
	}
	group := or(s["context"], "cluster")
	var hosts []plugins.Host
	for _, n := range resp.Items {
		addr := map[string]string{}
		for _, a := range n.Status.Addresses {
			addr[a.Type] = a.Address
		}
		var tags []string
		for k := range n.Metadata.Labels {
			if strings.HasPrefix(k, "node-role.kubernetes.io/") {
				tags = append(tags, "role="+strings.TrimPrefix(k, "node-role.kubernetes.io/"))
			}
		}
		sort.Strings(tags)
		hosts = append(hosts, plugins.Host{
			Key: or(n.Metadata.UID, n.Metadata.Name), Name: n.Metadata.Name,
			Hostname: addressOf(s["address"], addr["ExternalIP"], or(addr["InternalIP"], addr["Hostname"])),
			User:     s["user"], Group: []string{group}, Tags: tags, Notes: "Kubernetes node " + n.Metadata.Name,
		})
	}
	return hosts, nil
}

// Ansible reads an Ansible inventory with ansible-inventory.
type Ansible struct {
	plugins.Base
	Cmd Runner
}

// Manifest describes the plugin.
func (Ansible) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "ansible-inventory", Name: "Ansible inventory", Version: "1", Description: "plugins.ansible.description", BuiltIn: true,
		Capabilities: []string{plugins.CapInventory}, Requires: []string{"ansible-inventory"},
		Settings: []plugins.Setting{
			{Key: "inventory", Label: "plugins.settings.ansibleInventory", Placeholder: "~/infra/inventory.yml", Required: true, Kind: "file"},
		},
	}
}

// Inventory lists the hosts, in their first group other than "all".
func (p Ansible) Inventory(ctx context.Context, s map[string]string) ([]plugins.Host, error) {
	if err := plugins.Require(s, "inventory"); err != nil {
		return nil, err
	}
	out, err := p.Cmd(ctx, "ansible-inventory", "-i", expandHome(s["inventory"]), "--list")
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, badJSON(err, "ansible-inventory")
	}
	var meta struct {
		Hostvars map[string]map[string]any `json:"hostvars"`
	}
	if raw, ok := doc["_meta"]; ok {
		json.Unmarshal(raw, &meta)
	}
	type group struct {
		Hosts    []string `json:"hosts"`
		Children []string `json:"children"`
	}
	groups := map[string]group{}
	for name, raw := range doc {
		if name == "_meta" {
			continue
		}
		var g group
		if json.Unmarshal(raw, &g) == nil {
			groups[name] = g
		}
	}
	// Path of each host: the chain of groups from "all" down to its group.
	where := map[string][]string{}
	var walk func(name string, trail []string, seen map[string]bool)
	walk = func(name string, trail []string, seen map[string]bool) {
		if seen[name] {
			return
		}
		seen[name] = true
		g := groups[name]
		if name != "all" && name != "ungrouped" {
			trail = append(append([]string(nil), trail...), name)
		}
		for _, h := range g.Hosts {
			if cur, ok := where[h]; !ok || len(trail) > len(cur) {
				where[h] = trail // the deepest group wins
			}
		}
		children := append([]string(nil), g.Children...)
		sort.Strings(children)
		for _, c := range children {
			walk(c, trail, seen)
		}
	}
	walk("all", nil, map[string]bool{})
	for h := range meta.Hostvars {
		if _, ok := where[h]; !ok {
			where[h] = nil
		}
	}
	names := make([]string, 0, len(where))
	for h := range where {
		names = append(names, h)
	}
	sort.Strings(names)
	var hosts []plugins.Host
	for _, h := range names {
		vars := meta.Hostvars[h]
		str := func(k string) string {
			if v, ok := vars[k]; ok {
				return strings.TrimSpace(fmt.Sprint(v))
			}
			return ""
		}
		port := 0
		fmt.Sscanf(str("ansible_port"), "%d", &port)
		hosts = append(hosts, plugins.Host{
			Key: h, Name: h, Hostname: or(str("ansible_host"), h), Port: port,
			User: or(str("ansible_user"), str("ansible_ssh_user")), Group: where[h],
		})
	}
	return hosts, nil
}

// Terraform reads the machines of a Terraform state (terraform show -json).
type Terraform struct {
	plugins.Base
	Cmd Runner
}

// Manifest describes the plugin.
func (Terraform) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "terraform-state", Name: "Terraform state", Version: "1", Description: "plugins.terraform.description", BuiltIn: true,
		Capabilities: []string{plugins.CapInventory}, Requires: []string{"terraform"},
		Settings: []plugins.Setting{
			{Key: "dir", Label: "plugins.settings.terraformDir", Placeholder: "~/infra/prod", Required: true, Kind: "folder"},
			addressSetting,
			{Key: "user", Label: "plugins.settings.user"},
		},
	}
}

type tfModule struct {
	Resources []struct {
		Address string         `json:"address"`
		Type    string         `json:"type"`
		Name    string         `json:"name"`
		Values  map[string]any `json:"values"`
	} `json:"resources"`
	ChildModules []tfModule `json:"child_modules"`
}

// Inventory lists the machines found in the state.
func (p Terraform) Inventory(ctx context.Context, s map[string]string) ([]plugins.Host, error) {
	if err := plugins.Require(s, "dir"); err != nil {
		return nil, err
	}
	out, err := p.Cmd(ctx, "terraform", "-chdir="+expandHome(s["dir"]), "show", "-json")
	if err != nil {
		return nil, err
	}
	var state struct {
		Values struct {
			RootModule tfModule `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal(out, &state); err != nil {
		return nil, badJSON(err, "terraform")
	}
	var hosts []plugins.Host
	var visit func(m tfModule)
	visit = func(m tfModule) {
		for _, r := range m.Resources {
			public, private, ok := machineAddresses(r.Type, r.Values)
			if !ok {
				continue
			}
			name := r.Name
			if tags, ok := r.Values["tags"].(map[string]any); ok {
				if n, ok := tags["Name"].(string); ok && n != "" {
					name = n
				}
			}
			if n, ok := r.Values["name"].(string); ok && n != "" {
				name = n
			}
			provider := strings.SplitN(r.Type, "_", 2)[0]
			hosts = append(hosts, plugins.Host{
				Key: r.Address, Name: name, Hostname: addressOf(s["address"], public, private),
				User: s["user"], Group: []string{provider}, Notes: "Terraform " + r.Address,
			})
		}
		for _, c := range m.ChildModules {
			visit(c)
		}
	}
	visit(state.Values.RootModule)
	return hosts, nil
}

// machineAddresses reads the addresses of the usual machine resources.
func machineAddresses(kind string, v map[string]any) (public, private string, ok bool) {
	str := func(k string) string { s, _ := v[k].(string); return s }
	switch kind {
	case "aws_instance":
		return str("public_ip"), str("private_ip"), true
	case "google_compute_instance":
		if nics, ok := v["network_interface"].([]any); ok && len(nics) > 0 {
			nic, _ := nics[0].(map[string]any)
			private, _ = nic["network_ip"].(string)
			if acs, ok := nic["access_config"].([]any); ok && len(acs) > 0 {
				ac, _ := acs[0].(map[string]any)
				public, _ = ac["nat_ip"].(string)
			}
		}
		return public, private, true
	case "azurerm_linux_virtual_machine", "azurerm_windows_virtual_machine", "azurerm_virtual_machine":
		return str("public_ip_address"), str("private_ip_address"), true
	case "digitalocean_droplet":
		return str("ipv4_address"), str("ipv4_address_private"), true
	case "hcloud_server":
		return str("ipv4_address"), "", true
	case "linode_instance":
		return str("ip_address"), str("private_ip_address"), true
	case "vultr_instance":
		return str("main_ip"), str("internal_ip"), true
	}
	return "", "", false
}
