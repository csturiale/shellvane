//go:build !windows

package app

import (
	"net"
	"os"
	"path/filepath"
	"time"
)

func agentPath(dataDir string) string { return filepath.Join(dataDir, "agent.sock") }

func isPipe(string) bool { return false }

func controlPath(dataDir string) string { return filepath.Join(dataDir, "control.sock") }

func dialLocal(path string) (net.Conn, error) { return net.DialTimeout("unix", path, 2*time.Second) }

// listenLocal listens on a Unix socket only the user can use.
func listenLocal(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	os.Remove(path) // a socket left by a crash
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}
