package builtin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/plugins"
)

// fakeRun answers a command with canned output and records the call.
func fakeRun(out string, calls *[]string) Runner {
	return func(_ context.Context, name string, args ...string) ([]byte, error) {
		*calls = append(*calls, name+" "+strings.Join(args, " "))
		return []byte(out), nil
	}
}

func TestAWS(t *testing.T) {
	var calls []string
	p := AWS{Cmd: fakeRun(`{"Reservations":[{"Instances":[
		{"InstanceId":"i-1","PublicIpAddress":"3.3.3.3","PrivateIpAddress":"10.0.0.1","Placement":{"AvailabilityZone":"eu-west-1a"},"Tags":[{"Key":"Name","Value":"web"},{"Key":"env","Value":"prod"}]},
		{"InstanceId":"i-2","PrivateIpAddress":"10.0.0.2","Placement":{"AvailabilityZone":"eu-west-1b"}}]}]}`, &calls)}
	hosts, err := p.Inventory(context.Background(), map[string]string{"profile": "work", "region": "eu-west-1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []plugins.Host{
		{Key: "i-1", Name: "web", Hostname: "3.3.3.3", User: "ec2-user", Group: []string{"eu-west-1"}, Tags: []string{"env=prod"}, Notes: "EC2 i-1 · eu-west-1a"},
		{Key: "i-2", Name: "i-2", Hostname: "10.0.0.2", User: "ec2-user", Group: []string{"eu-west-1"}, Notes: "EC2 i-2 · eu-west-1b"},
	}
	if !reflect.DeepEqual(hosts, want) {
		t.Fatalf("hosts =\n%+v\nwant\n%+v", hosts, want)
	}
	if !strings.Contains(calls[0], "--profile work") || !strings.Contains(calls[0], "--region eu-west-1") {
		t.Errorf("call = %s", calls[0])
	}
	private, _ := p.Inventory(context.Background(), map[string]string{"address": "private"})
	if private[0].Hostname != "10.0.0.1" {
		t.Errorf("private address = %s", private[0].Hostname)
	}
}

func TestGCPAndAzure(t *testing.T) {
	var calls []string
	g := GCP{Cmd: fakeRun(`[{"name":"db","id":"42","zone":"https://x/zones/europe-west1-b","labels":{"tier":"data"},
		"networkInterfaces":[{"networkIP":"10.1.0.5","accessConfigs":[{"natIP":"34.1.1.1"}]}]}]`, &calls)}
	hosts, err := g.Inventory(context.Background(), map[string]string{"project": "acme"})
	if err != nil || len(hosts) != 1 || hosts[0].Hostname != "34.1.1.1" || hosts[0].Group[0] != "europe-west1-b" || hosts[0].Tags[0] != "tier=data" {
		t.Fatalf("gcp = %+v, %v", hosts, err)
	}
	a := Azure{Cmd: fakeRun(`[{"id":"/sub/x/vm1","name":"vm1","resourceGroup":"rg-web","location":"westeurope","powerState":"VM running","publicIps":"20.1.1.1","privateIps":"10.2.0.4,10.2.0.5"},
		{"id":"/sub/x/vm2","name":"vm2","resourceGroup":"rg-web","powerState":"VM deallocated","privateIps":"10.2.0.9"}]`, &calls)}
	hosts, err = a.Inventory(context.Background(), map[string]string{"address": "private"})
	if err != nil || len(hosts) != 1 || hosts[0].Hostname != "10.2.0.4" || hosts[0].User != "azureuser" || hosts[0].Group[0] != "rg-web" {
		t.Fatalf("azure = %+v, %v", hosts, err)
	}
}

func TestKubernetesAnsibleTerraform(t *testing.T) {
	var calls []string
	k := Kubernetes{Cmd: fakeRun(`{"items":[{"metadata":{"name":"node-1","uid":"u1","labels":{"node-role.kubernetes.io/control-plane":""}},
		"status":{"addresses":[{"type":"InternalIP","address":"10.3.0.1"},{"type":"Hostname","address":"node-1"}]}}]}`, &calls)}
	hosts, err := k.Inventory(context.Background(), map[string]string{"context": "prod"})
	if err != nil || hosts[0].Hostname != "10.3.0.1" || hosts[0].Group[0] != "prod" || hosts[0].Tags[0] != "role=control-plane" {
		t.Fatalf("kubernetes = %+v, %v", hosts, err)
	}

	an := Ansible{Cmd: fakeRun(`{"_meta":{"hostvars":{"web1":{"ansible_host":"192.168.1.10","ansible_user":"deploy","ansible_port":2222},"db1":{}}},
		"all":{"children":["ungrouped","prod"]},"prod":{"children":["web"],"hosts":["db1"]},"web":{"hosts":["web1"]},"ungrouped":{}}`, &calls)}
	if _, err := an.Inventory(context.Background(), map[string]string{}); errs.CodeOf(err) != plugins.ErrSetting {
		t.Fatalf("missing inventory: %v", err)
	}
	hosts, err = an.Inventory(context.Background(), map[string]string{"inventory": "inv.yml"})
	if err != nil {
		t.Fatal(err)
	}
	want := []plugins.Host{
		{Key: "db1", Name: "db1", Hostname: "db1", Group: []string{"prod"}},
		{Key: "web1", Name: "web1", Hostname: "192.168.1.10", Port: 2222, User: "deploy", Group: []string{"prod", "web"}},
	}
	if !reflect.DeepEqual(hosts, want) {
		t.Fatalf("ansible =\n%+v\nwant\n%+v", hosts, want)
	}

	tf := Terraform{Cmd: fakeRun(`{"values":{"root_module":{"resources":[
		{"address":"aws_instance.web","type":"aws_instance","name":"web","values":{"public_ip":"3.4.5.6","private_ip":"10.0.1.1","tags":{"Name":"web-prod"}}},
		{"address":"aws_s3_bucket.logs","type":"aws_s3_bucket","name":"logs","values":{}}],
		"child_modules":[{"resources":[{"address":"module.db.hcloud_server.db","type":"hcloud_server","name":"db","values":{"name":"db-1","ipv4_address":"5.6.7.8"}}]}]}}}`, &calls)}
	hosts, err = tf.Inventory(context.Background(), map[string]string{"dir": "infra"})
	if err != nil || len(hosts) != 2 || hosts[0].Name != "web-prod" || hosts[0].Hostname != "3.4.5.6" || hosts[1].Name != "db-1" || hosts[1].Group[0] != "hcloud" {
		t.Fatalf("terraform = %+v, %v", hosts, err)
	}
	if !strings.Contains(calls[len(calls)-1], "-chdir=infra show -json") {
		t.Errorf("call = %s", calls[len(calls)-1])
	}
}

func TestProxmoxWithPinnedCertificate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "PVEAPIToken=shellvane@pve!inv=s3cret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api2/json/cluster/resources":
			w.Write([]byte(`{"data":[{"vmid":100,"name":"web","node":"pve1","type":"qemu","status":"running","tags":"prod;web"},
				{"vmid":101,"name":"ct","node":"pve2","type":"lxc","status":"running"},
				{"vmid":102,"name":"off","node":"pve1","type":"qemu","status":"stopped"}]}`))
		case "/api2/json/nodes/pve1/qemu/100/agent/network-get-interfaces":
			w.Write([]byte(`{"data":{"result":[{"ip-addresses":[{"ip-address-type":"ipv4","ip-address":"127.0.0.1"}]},{"ip-addresses":[{"ip-address-type":"ipv4","ip-address":"192.168.10.5"}]}]}}`))
		case "/api2/json/nodes/pve2/lxc/101/interfaces":
			w.Write([]byte(`{"data":[{"inet":"127.0.0.1/8"},{"inet":"192.168.10.6/24"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	sum := sha256.Sum256(srv.Certificate().Raw)
	pin := hex.EncodeToString(sum[:])
	settings := map[string]string{"url": srv.URL, "tokenId": "shellvane@pve!inv", "token": "s3cret", "fingerprint": strings.ToUpper(pin)}

	hosts, err := Proxmox{}.Inventory(context.Background(), settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[0].Hostname != "192.168.10.5" || hosts[0].Group[0] != "pve1" || !reflect.DeepEqual(hosts[0].Tags, []string{"prod", "web"}) || hosts[1].Hostname != "192.168.10.6" {
		t.Fatalf("hosts = %+v", hosts)
	}
	settings["fingerprint"] = strings.Repeat("ab", 32)
	if _, err := (Proxmox{}).Inventory(context.Background(), settings); err == nil {
		t.Fatal("a certificate with another fingerprint was accepted")
	}
	settings["fingerprint"] = ""
	if _, err := (Proxmox{}).Inventory(context.Background(), settings); err == nil {
		t.Fatal("a self-signed certificate was accepted without a pinned fingerprint")
	}
}

func TestNetBoxPages(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Token tok" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		switch {
		case r.URL.Path == "/api/dcim/devices/" && r.URL.Query().Get("page") == "":
			if r.URL.Query().Get("status") != "active" {
				t.Errorf("filter not passed: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"next":"` + srv.URL + `/api/dcim/devices/?page=2","results":[{"id":1,"name":"sw1","primary_ip":{"address":"10.9.0.1/24"},"site":{"name":"MIL"},"tags":[{"name":"core"}]}]}`))
		case r.URL.Path == "/api/dcim/devices/":
			w.Write([]byte(`{"next":null,"results":[{"id":2,"name":"noip","primary_ip":null}]}`))
		case r.URL.Path == "/api/virtualization/virtual-machines/":
			w.Write([]byte(`{"next":null,"results":[{"id":7,"name":"vm7","primary_ip":{"address":"10.9.1.7/32"},"cluster":{"name":"k-prod"}}]}`))
		}
	}))
	defer srv.Close()
	hosts, err := NetBox{}.Inventory(context.Background(), map[string]string{"url": srv.URL + "/", "token": "tok", "filter": "status=active"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[0].Hostname != "10.9.0.1" || hosts[0].Group[0] != "MIL" || hosts[1].Group[0] != "k-prod" || hosts[1].Hostname != "10.9.1.7" {
		t.Fatalf("hosts = %+v", hosts)
	}
}
