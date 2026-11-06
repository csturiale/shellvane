package serialport

import (
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in    string
		want  Settings
		label string
	}{
		{"serial:///dev/ttyUSB0", Settings{Port: "/dev/ttyUSB0", Baud: 9600, DataBits: 8, Parity: "none", StopBits: "1"}, "/dev/ttyUSB0 9600 8N1"},
		{"serial://COM3?baud=115200&parity=even&data=7&stop=2", Settings{Port: "COM3", Baud: 115200, DataBits: 7, Parity: "even", StopBits: "2"}, "COM3 115200 7E2"},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil || got != c.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
			continue
		}
		if got.Label() != c.label {
			t.Errorf("Label = %q, want %q", got.Label(), c.label)
		}
		if back, err := Parse(got.Spec()); err != nil || back != got {
			t.Errorf("Spec %q does not round-trip: %+v, %v", got.Spec(), back, err)
		}
	}
	for _, bad := range []string{"", "telnet://x", "serial://", "serial://COM1?baud=fast", "serial://COM1?parity=maybe", "serial://COM1?data=9", "serial://COM1?stop=3"} {
		if _, err := Parse(bad); errs.CodeOf(err) != ErrInvalid {
			t.Errorf("Parse(%q) = %v", bad, err)
		}
	}
}

func TestOpenMissingPort(t *testing.T) {
	if _, err := Open(Settings{Port: "/nonexistent/tty-shellvane"}); errs.CodeOf(err) != ErrOpen {
		t.Fatalf("Open = %v", err)
	}
	if Ports() == nil {
		t.Fatal("Ports must not be nil")
	}
}
