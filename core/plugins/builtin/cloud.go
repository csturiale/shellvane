package builtin

import (
	"context"
	"encoding/json"
	"path"
	"sort"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

func badJSON(err error, command string) error {
	return errs.Wrap(err, plugins.ErrResponse, errs.Params{"command": command})
}

// tagsOf turns key/value labels into "key=value" tags, sorted.
func tagsOf(m map[string]string, skip ...string) []string {
	var out []string
	for k, v := range m {
		if contains(skip, k) {
			continue
		}
		if v == "" {
			out = append(out, k)
		} else {
			out = append(out, k+"="+v)
		}
	}
	sort.Strings(out)
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// addressOf picks the public or the private address, as the user chose
// ("private" or anything else for public-first).
func addressOf(choice, public, private string) string {
	if choice == "private" {
		return or(private, public)
	}
	return or(public, private)
}

var addressSetting = plugins.Setting{Key: "address", Label: "plugins.settings.address", Placeholder: "public | private"}

// AWS lists running EC2 instances with the aws CLI.
type AWS struct {
	plugins.Base
	Cmd Runner
}

// Manifest describes the plugin.
func (AWS) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "aws-ec2", Name: "AWS EC2", Version: "1", Description: "plugins.aws.description", BuiltIn: true,
		Capabilities: []string{plugins.CapInventory}, Requires: []string{"aws"},
		Settings: []plugins.Setting{
			{Key: "profile", Label: "plugins.settings.awsProfile", Placeholder: "default"},
			{Key: "region", Label: "plugins.settings.region", Placeholder: "eu-west-1"},
			addressSetting,
			{Key: "user", Label: "plugins.settings.user", Placeholder: "ec2-user"},
		},
	}
}

// Inventory lists the instances.
func (p AWS) Inventory(ctx context.Context, s map[string]string) ([]plugins.Host, error) {
	args := []string{"ec2", "describe-instances", "--output", "json", "--filters", "Name=instance-state-name,Values=running"}
	args = appendIf(args, "--profile", s["profile"])
	args = appendIf(args, "--region", s["region"])
	out, err := p.Cmd(ctx, "aws", args...)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Reservations []struct {
			Instances []struct {
				InstanceID       string `json:"InstanceId"`
				PublicIPAddress  string `json:"PublicIpAddress"`
				PrivateIPAddress string `json:"PrivateIpAddress"`
				PublicDNSName    string `json:"PublicDnsName"`
				Placement        struct {
					AvailabilityZone string `json:"AvailabilityZone"`
				} `json:"Placement"`
				Tags []struct{ Key, Value string } `json:"Tags"`
			} `json:"Instances"`
		} `json:"Reservations"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, badJSON(err, "aws")
	}
	var hosts []plugins.Host
	for _, r := range resp.Reservations {
		for _, i := range r.Instances {
			tags := map[string]string{}
			for _, t := range i.Tags {
				tags[t.Key] = t.Value
			}
			zone := i.Placement.AvailabilityZone
			region := strings.TrimRight(zone, "abcdefghijklmnopqrstuvwxyz")
			hosts = append(hosts, plugins.Host{
				Key: i.InstanceID, Name: or(tags["Name"], i.InstanceID),
				Hostname: addressOf(s["address"], or(i.PublicIPAddress, i.PublicDNSName), i.PrivateIPAddress),
				User:     or(s["user"], "ec2-user"), Group: []string{or(region, zone)},
				Tags: tagsOf(tags, "Name"), Notes: "EC2 " + i.InstanceID + " · " + zone,
			})
		}
	}
	return hosts, nil
}

// GCP lists Compute Engine instances with the gcloud CLI.
type GCP struct {
	plugins.Base
	Cmd Runner
}

// Manifest describes the plugin.
func (GCP) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "gcp-compute", Name: "Google Cloud Compute", Version: "1", Description: "plugins.gcp.description", BuiltIn: true,
		Capabilities: []string{plugins.CapInventory}, Requires: []string{"gcloud"},
		Settings: []plugins.Setting{
			{Key: "project", Label: "plugins.settings.gcpProject", Placeholder: "my-project"},
			addressSetting,
			{Key: "user", Label: "plugins.settings.user"},
		},
	}
}

// Inventory lists the running instances.
func (p GCP) Inventory(ctx context.Context, s map[string]string) ([]plugins.Host, error) {
	args := appendIf([]string{"compute", "instances", "list", "--format=json", "--filter=status=RUNNING"}, "--project", s["project"])
	out, err := p.Cmd(ctx, "gcloud", args...)
	if err != nil {
		return nil, err
	}
	var list []struct {
		Name              string            `json:"name"`
		ID                string            `json:"id"`
		Zone              string            `json:"zone"`
		Labels            map[string]string `json:"labels"`
		NetworkInterfaces []struct {
			NetworkIP     string `json:"networkIP"`
			AccessConfigs []struct {
				NatIP string `json:"natIP"`
			} `json:"accessConfigs"`
		} `json:"networkInterfaces"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, badJSON(err, "gcloud")
	}
	var hosts []plugins.Host
	for _, i := range list {
		var public, private string
		if len(i.NetworkInterfaces) > 0 {
			private = i.NetworkInterfaces[0].NetworkIP
			if len(i.NetworkInterfaces[0].AccessConfigs) > 0 {
				public = i.NetworkInterfaces[0].AccessConfigs[0].NatIP
			}
		}
		zone := path.Base(i.Zone)
		hosts = append(hosts, plugins.Host{
			Key: or(i.ID, i.Name), Name: i.Name, Hostname: addressOf(s["address"], public, private),
			User: s["user"], Group: []string{zone}, Tags: tagsOf(i.Labels), Notes: "GCE " + i.Name + " · " + zone,
		})
	}
	return hosts, nil
}

// Azure lists virtual machines with the az CLI.
type Azure struct {
	plugins.Base
	Cmd Runner
}

// Manifest describes the plugin.
func (Azure) Manifest() plugins.Manifest {
	return plugins.Manifest{
		ID: "azure-vm", Name: "Azure VMs", Version: "1", Description: "plugins.azure.description", BuiltIn: true,
		Capabilities: []string{plugins.CapInventory}, Requires: []string{"az"},
		Settings: []plugins.Setting{
			{Key: "subscription", Label: "plugins.settings.azureSubscription"},
			{Key: "resourceGroup", Label: "plugins.settings.azureResourceGroup"},
			addressSetting,
			{Key: "user", Label: "plugins.settings.user", Placeholder: "azureuser"},
		},
	}
}

// Inventory lists the running machines.
func (p Azure) Inventory(ctx context.Context, s map[string]string) ([]plugins.Host, error) {
	args := []string{"vm", "list", "--show-details", "--output", "json"}
	args = appendIf(args, "--subscription", s["subscription"])
	args = appendIf(args, "--resource-group", s["resourceGroup"])
	out, err := p.Cmd(ctx, "az", args...)
	if err != nil {
		return nil, err
	}
	var list []struct {
		ID            string            `json:"id"`
		Name          string            `json:"name"`
		ResourceGroup string            `json:"resourceGroup"`
		Location      string            `json:"location"`
		PowerState    string            `json:"powerState"`
		PublicIPs     string            `json:"publicIps"`
		PrivateIPs    string            `json:"privateIps"`
		Tags          map[string]string `json:"tags"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, badJSON(err, "az")
	}
	var hosts []plugins.Host
	for _, v := range list {
		if v.PowerState != "" && v.PowerState != "VM running" {
			continue
		}
		first := func(csv string) string { return strings.TrimSpace(strings.Split(csv, ",")[0]) }
		hosts = append(hosts, plugins.Host{
			Key: or(v.ID, v.Name), Name: v.Name, Hostname: addressOf(s["address"], first(v.PublicIPs), first(v.PrivateIPs)),
			User: or(s["user"], "azureuser"), Group: []string{or(v.ResourceGroup, v.Location)}, Tags: tagsOf(v.Tags),
			Notes: "Azure " + v.Name + " · " + v.Location,
		})
	}
	return hosts, nil
}
