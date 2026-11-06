// Package launch starts external clients: remote desktop (RDP), VNC and
// Mosh, with the first program found on this system.
package launch

import (
	"net"
	"os/exec"
	"runtime"
	"strconv"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrNoClient reports that no client for the protocol is installed.
const ErrNoClient errs.Code = "launch.noClient"

// Kinds of clients.
const (
	RDP = "rdp"
	VNC = "vnc"
)

// Candidate is a program and its arguments.
type Candidate struct {
	Path string
	Args []string
}

// Candidates lists the clients to try, in order, for kind on goos.
func Candidates(goos, kind, host string, port int) []Candidate {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	switch kind {
	case RDP:
		switch goos {
		case "windows":
			return []Candidate{{"mstsc.exe", []string{"/v:" + addr}}}
		case "darwin":
			return []Candidate{{"open", []string{"rdp://full%20address=s:" + addr}}}
		default:
			return []Candidate{
				{"xfreerdp3", []string{"/v:" + addr, "/cert:tofu"}},
				{"xfreerdp", []string{"/v:" + addr, "/cert:tofu"}},
				{"remmina", []string{"-c", "rdp://" + addr}},
			}
		}
	case VNC:
		// vncviewer takes "host::port" for a full port number.
		full := host + "::" + strconv.Itoa(port)
		switch goos {
		case "windows":
			return []Candidate{{"vncviewer.exe", []string{full}}, {"tvnviewer.exe", []string{addr}}}
		case "darwin":
			return []Candidate{{"open", []string{"vnc://" + addr}}}
		default:
			return []Candidate{
				{"vncviewer", []string{full}},
				{"remmina", []string{"-c", "vnc://" + addr}},
				{"xdg-open", []string{"vnc://" + addr}},
			}
		}
	}
	return nil
}

// Find returns the first installed candidate.
func Find(kind, host string, port int) (Candidate, error) {
	for _, c := range Candidates(runtime.GOOS, kind, host, port) {
		if p, err := exec.LookPath(c.Path); err == nil {
			c.Path = p
			return c, nil
		}
	}
	return Candidate{}, errs.New(ErrNoClient, errs.Params{"kind": kind})
}

// Available tells whether a client for kind is installed.
func Available(kind string) bool {
	_, err := Find(kind, "localhost", 1)
	return err == nil
}

// Start runs a candidate without waiting for it.
func Start(c Candidate) error {
	cmd := exec.Command(c.Path, c.Args...)
	if err := cmd.Start(); err != nil {
		return errs.Wrap(err, ErrNoClient, nil)
	}
	go cmd.Wait()
	return nil
}

// DefaultPort of a protocol.
func DefaultPort(kind string) int {
	if kind == VNC {
		return 5900
	}
	return 3389
}
