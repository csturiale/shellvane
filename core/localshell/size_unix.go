//go:build !windows

package localshell

import (
	pty "github.com/aymanbagabas/go-pty"
	"golang.org/x/sys/unix"
)

// resize sets the size on the terminal end: macOS ignores it on the
// controlling end.
func resize(p pty.Pty, cols, rows int) error {
	u, ok := p.(pty.UnixPty)
	if !ok {
		return p.Resize(cols, rows)
	}
	return unix.IoctlSetWinsize(int(u.Slave().Fd()), unix.TIOCSWINSZ, &unix.Winsize{Row: uint16(rows), Col: uint16(cols)})
}

func closeTerminalEnd(p pty.Pty) {
	if u, ok := p.(pty.UnixPty); ok {
		u.Slave().Close()
	}
}
