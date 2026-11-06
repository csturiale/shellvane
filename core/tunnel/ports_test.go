package tunnel

import (
	"reflect"
	"testing"
)

func TestParseListening(t *testing.T) {
	cases := map[string][]RemotePort{
		"#ss\nLISTEN 0 4096 127.0.0.53%lo:53 0.0.0.0:* users:((\"systemd-resolve\",pid=1,fd=14))\nLISTEN 0 511 *:3000 *:*\nLISTEN 0 128 [::]:22 [::]:*\n": {
			{Port: 22, Addr: "::"}, {Port: 53, Addr: "127.0.0.53", Process: "systemd-resolve"}, {Port: 3000, Addr: "0.0.0.0"},
		},
		"#netstat\nActive Internet connections\ntcp 0 0 0.0.0.0:22 0.0.0.0:* LISTEN 812/sshd\ntcp6 0 0 :::8080 :::* LISTEN -\ntcp 0 0 10.0.0.1:22 10.0.0.2:5000 ESTABLISHED 9/sshd\n": {
			{Port: 22, Addr: "0.0.0.0", Process: "sshd"}, {Port: 8080, Addr: "::"},
		},
		"#netstat\ntcp4 0 0 *.5432 *.* LISTEN\ntcp4 0 0 127.0.0.1.6379 *.* LISTEN\n": {
			{Port: 5432, Addr: "0.0.0.0"}, {Port: 6379, Addr: "127.0.0.1"},
		},
		"#proc\n  sl  local_address rem_address   st\n   0: 0100007F:1F90 00000000:0000 0A 0\n   1: 00000000:0016 00000000:0000 0A 0\n   2: 0100007F:0016 0100007F:A000 01 0\n" +
			"   0: 00000000000000000000000001000000:0BB8 00000000000000000000000000000000:0000 0A 0\n": {
			{Port: 22, Addr: "0.0.0.0"}, {Port: 3000, Addr: "::1"}, {Port: 8080, Addr: "127.0.0.1"},
		},
		"#none\n": nil,
	}
	for in, want := range cases {
		if got := ParseListening(in); !reflect.DeepEqual(got, want) {
			t.Errorf("ParseListening(%q)\n got %+v\nwant %+v", in, got, want)
		}
	}
	if DestHost("0.0.0.0") != "127.0.0.1" || DestHost("::") != "::1" || DestHost("10.1.1.1") != "10.1.1.1" {
		t.Error("DestHost")
	}
}
