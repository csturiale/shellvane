package importers

import (
	"path/filepath"
	"reflect"
	"testing"
)

func td(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "testdata", "importers"}, parts...)...)
}

func TestOtherImporters(t *testing.T) {
	cases := []struct {
		name     string
		read     func() (Result, error)
		hosts    []Host
		warnings map[string]int
	}{
		{"mobaxterm", func() (Result, error) { return MobaXterm(td("mobaxterm", "MobaXterm.ini")) },
			[]Host{
				{Name: "web-01", Hostname: "web01.example.com", User: "deploy"},
				{Name: "db-01", Hostname: "10.0.2.15", Port: 2222, User: "postgres", Group: []string{"Prod", "Databases"}},
			}, map[string]int{"protocolSkipped": 1}},
		{"securecrt", func() (Result, error) { return SecureCRT(td("securecrt", "Sessions")) },
			[]Host{
				{Name: "api", Hostname: "api.eu.example.org", Port: 2222, User: "deploy", Group: []string{"Prod", "EU"}},
				{Name: "bastion", Hostname: "bastion.example.org", User: "ops"},
			}, map[string]int{"protocolSkipped": 1, "passwordSkipped": 1}},
		{"winscp", func() (Result, error) { return WinSCP(td("winscp", "WinSCP.ini")) },
			[]Host{
				{Name: "files server", Hostname: "files.example.com", Port: 2022, User: "sftpuser", Group: []string{"Prod"}},
				{Name: "scp-box", Hostname: "scp.example.com", User: "root", Group: []string{}},
			}, map[string]int{"protocolSkipped": 1, "passwordSkipped": 1}},
		{"filezilla", func() (Result, error) { return FileZilla(td("filezilla", "sitemanager.xml")) },
			[]Host{
				{Name: "top sftp", Hostname: "top.example.com", User: "alice"},
				{Name: "acme sftp", Hostname: "sftp.acme.example", Port: 2200, User: "acme", Group: []string{"Clients", "ACME"}},
			}, map[string]int{"protocolSkipped": 1, "passwordSkipped": 1}},
		{"tabby", func() (Result, error) { return Tabby(td("tabby", "config.yaml")) },
			[]Host{
				{Name: "jump", Hostname: "jump.example.net", User: "ops"},
				{Name: "app server", Hostname: "10.1.0.20", Port: 2201, User: "app", Group: []string{"Production"}, ProxyJump: []string{"jump"}},
			}, map[string]int{"protocolSkipped": 1, "passwordSkipped": 1}},
		{"mremoteng", func() (Result, error) { return MRemoteNG(td("mremoteng", "confCons.xml")) },
			[]Host{
				{Name: "web", Hostname: "web.example.org", User: "www", Group: []string{"Linux"}},
				{Name: "db", Hostname: "db.example.org", Port: 2222, User: "dba", Group: []string{"Linux", "Inner"}},
			}, map[string]int{"protocolSkipped": 1, "passwordSkipped": 1}},
	}
	for _, c := range cases {
		res, err := c.read()
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !reflect.DeepEqual(res.Hosts, c.hosts) {
			t.Errorf("%s hosts:\n got %+v\nwant %+v", c.name, res.Hosts, c.hosts)
		}
		if got := codes(res); !reflect.DeepEqual(got, c.warnings) {
			t.Errorf("%s warnings = %v (%+v)", c.name, got, res.Warnings)
		}
	}
}
