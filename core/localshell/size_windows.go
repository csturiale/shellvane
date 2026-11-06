//go:build windows

package localshell

import pty "github.com/aymanbagabas/go-pty"

func resize(p pty.Pty, cols, rows int) error { return p.Resize(cols, rows) }

// closeTerminalEnd has nothing to do: ConPTY ends the output itself.
func closeTerminalEnd(pty.Pty) {}
