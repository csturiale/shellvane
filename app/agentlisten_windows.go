//go:build windows

package app

import (
	"net"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/Microsoft/go-winio"
)

// Pipe names carry the user: on a computer shared by several people (a
// terminal server) nobody else can take the name first.
func agentPath(string) string { return `\\.\pipe\shellvane-agent-` + pipeUser() }

func isPipe(string) bool { return true }

func controlPath(string) string { return `\\.\pipe\shellvane-control-` + pipeUser() }

func pipeUser() string {
	name := os.Getenv("USERNAME")
	if u, err := user.Current(); err == nil {
		name = u.Uid // the SID
	}
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		return '_'
	}, name)
}

func dialLocal(path string) (net.Conn, error) {
	timeout := 2 * time.Second
	return winio.DialPipe(path, &timeout)
}

// listenLocal listens on a named pipe only the owner can open.
func listenLocal(path string) (net.Listener, error) {
	return winio.ListenPipe(path, &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;OW)"})
}
