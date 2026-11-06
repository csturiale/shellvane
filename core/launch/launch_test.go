package launch

import (
	"reflect"
	"testing"
)

func TestCandidates(t *testing.T) {
	if got := Candidates("windows", RDP, "win.example.com", 3389); !reflect.DeepEqual(got, []Candidate{{"mstsc.exe", []string{"/v:win.example.com:3389"}}}) {
		t.Errorf("windows rdp = %+v", got)
	}
	if got := Candidates("darwin", VNC, "::1", 5901); got[0].Args[0] != "vnc://[::1]:5901" {
		t.Errorf("darwin vnc = %+v", got)
	}
	if got := Candidates("linux", VNC, "h", 5905); got[0].Path != "vncviewer" || got[0].Args[0] != "h::5905" {
		t.Errorf("linux vnc = %+v", got)
	}
	if Candidates("linux", "telnet", "h", 23) != nil {
		t.Error("unknown kind")
	}
	if DefaultPort(VNC) != 5900 || DefaultPort(RDP) != 3389 {
		t.Error("ports")
	}
}
