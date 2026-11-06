package anonymize

import "testing"

func TestConsistentPseudonyms(t *testing.T) {
	a := New()
	if got := a.Name(KindHost, "db.example.com"); got != "host-01" {
		t.Fatalf("host = %q", got)
	}
	if got := a.Name(KindHost, "web.example.com"); got != "host-02" {
		t.Fatalf("host = %q", got)
	}
	if got := a.Name(KindHost, "db.example.com"); got != "host-01" {
		t.Fatalf("same host = %q", got)
	}
	if got := a.Name(KindUser, "alice"); got != "user-01" {
		t.Fatalf("user = %q", got)
	}
	if got := a.Name(KindHost, "192.168.1.10"); got != "10.255.0.1" {
		t.Fatalf("ip = %q", got)
	}
	if got := a.Host("[2001:db8::1]:2222"); got != "10.255.0.2:2222" {
		t.Fatalf("v6 = %q", got)
	}
}

func TestText(t *testing.T) {
	a := New()
	a.Name(KindHost, "db.example.com")
	a.Name(KindUser, "alice")
	got := a.Text("alice@db.example.com from 192.168.1.10 and 172.16.0.1, again 192.168.1.10; not 999.1.1.1")
	want := "user-01@host-01 from 10.255.0.1 and 10.255.0.2, again 10.255.0.1; not 999.1.1.1"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}
