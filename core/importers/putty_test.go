package importers

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/tunnel"
)

func codes(res Result) map[string]int {
	m := map[string]int{}
	for _, w := range res.Warnings {
		m[w.Code]++
	}
	return m
}

func TestPuTTYDir(t *testing.T) {
	res, err := PuTTYDir(filepath.Join("..", "..", "testdata", "importers", "putty", "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hosts) != 1 {
		t.Fatalf("hosts = %+v", res.Hosts)
	}
	h := res.Hosts[0]
	if h.Name != "web server" || h.Hostname != "web.example.org" || h.User != "root" || h.Port != 2222 {
		t.Errorf("host = %+v", h)
	}
	want := []tunnel.Spec{
		{Kind: tunnel.Local, BindAddr: "127.0.0.1", BindPort: 8080, DestHost: "localhost", DestPort: 80},
		{Kind: tunnel.Remote, BindAddr: "127.0.0.1", BindPort: 9000, DestHost: "127.0.0.1", DestPort: 3000},
		{Kind: tunnel.Dynamic, BindAddr: "127.0.0.1", BindPort: 1080},
	}
	if !reflect.DeepEqual(h.Tunnels, want) {
		t.Errorf("tunnels = %+v", h.Tunnels)
	}
	c := codes(res)
	if c["protocolSkipped"] != 1 || c["forwardInvalid"] != 1 || c["keyFileSkipped"] != 1 {
		t.Errorf("warnings = %+v", res.Warnings)
	}
}

func TestKiTTYDir(t *testing.T) {
	res, err := KiTTYDir(filepath.Join("..", "..", "testdata", "importers", "kitty", "Sessions"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hosts) != 1 {
		t.Fatalf("hosts = %+v", res.Hosts)
	}
	h := res.Hosts[0]
	if h.Hostname != "db.internal" || h.User != "postgres" || h.Port != 0 || !reflect.DeepEqual(h.Group, []string{"Production", "Databases"}) {
		t.Errorf("host = %+v", h)
	}
	if codes(res)["passwordSkipped"] != 1 {
		t.Errorf("warnings = %+v", res.Warnings)
	}
}

func TestRegistryOutsideWindows(t *testing.T) {
	if _, err := PuTTYRegistry(); err == nil && filepath.Separator == '/' {
		t.Fatal("registry must be unsupported outside Windows")
	}
}

// Portable KiTTY keeps groups as subfolders, with an URL-encoded Folder;
// a zip made on macOS adds __MACOSX and ._ files.
func TestKiTTYFoldersAndZip(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o644)
	}
	session := func(host, folder string) string {
		return "HostName\\" + host + "\\\r\nProtocol\\ssh\\\r\nPortNumber\\22\\\r\nFolder\\" + folder + "\\\r\n"
	}
	write("Sessions/top", session("top.example%09", "Default"))
	write("Sessions/Default%20Settings", "Protocol\\ssh\\\r\n")
	write("Sessions/QA/web%2001", session("web01.qa", "QA"))
	write("Sessions/QA/NEX/nex1", session("nex1.qa", "QA%5CNEX"))
	write("Sessions/QA/NEX/nofolder", session("nex2.qa", "Default"))
	write("Sessions/QA/._web%2001", "\x00\x05\x16\x07binary")
	write("__MACOSX/Sessions/._QA", "binary")

	check := func(res Result) {
		t.Helper()
		got := map[string]string{}
		for _, h := range res.Hosts {
			got[h.Name] = h.Hostname + " " + strings.Join(h.Group, "/")
		}
		want := map[string]string{
			"top":      "top.example ",
			"web 01":   "web01.qa QA",
			"nex1":     "nex1.qa QA/NEX",
			"nofolder": "nex2.qa QA/NEX",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("hosts = %v", got)
		}
	}
	res, err := KiTTYDir(filepath.Join(dir, "Sessions"))
	if err != nil {
		t.Fatal(err)
	}
	check(res)

	// The same as a zip, with the Sessions folder and the Finder's files.
	zipPath := filepath.Join(t.TempDir(), "Sessions.zip")
	f, _ := os.Create(zipPath)
	zw := zip.NewWriter(f)
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		w, _ := zw.Create(filepath.ToSlash(rel))
		data, _ := os.ReadFile(p)
		w.Write(data)
		return nil
	})
	zw.Close()
	f.Close()
	res, err = KiTTYDir(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	check(res)
}
