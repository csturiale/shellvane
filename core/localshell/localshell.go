// Package localshell runs a shell of this computer in a pseudo-terminal
// (PTY on Unix, ConPTY on Windows).
package localshell

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"

	pty "github.com/aymanbagabas/go-pty"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrStart reports a shell that could not be started.
const ErrStart errs.Code = "local.startFailed"

// Program is a shell the user can open.
type Program struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	Path string   `json:"path"`
	Args []string `json:"args"`
}

// Options describe the shell to start.
type Options struct {
	Program    Program
	Dir        string   // "" is the home directory
	Env        []string // added to the app's environment
	Cols, Rows int
}

// Shell is a running local shell.
type Shell struct {
	pty  pty.Pty
	cmd  *pty.Cmd
	once sync.Once
	done chan struct{}
	code int
}

// Start runs the program in a new pseudo-terminal.
func Start(o Options) (*Shell, error) {
	p, err := pty.New()
	if err != nil {
		return nil, errs.Wrap(err, ErrStart, errs.Params{"shell": o.Program.Name})
	}
	if o.Cols > 0 && o.Rows > 0 {
		resize(p, o.Cols, o.Rows)
	}
	cmd := p.Command(o.Program.Path, o.Program.Args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=Shellvane")
	cmd.Env = append(cmd.Env, o.Env...)
	cmd.Dir = o.Dir
	if cmd.Dir == "" {
		cmd.Dir, _ = os.UserHomeDir()
	}
	if err := cmd.Start(); err != nil {
		p.Close()
		return nil, errs.Wrap(err, ErrStart, errs.Params{"shell": o.Program.Name})
	}
	s := &Shell{pty: p, cmd: cmd, done: make(chan struct{})}
	go func() {
		err := cmd.Wait()
		s.code = exitCode(err)
		// Our copy of the terminal end kept it open for resizing; closing it
		// lets Read see the end of output.
		closeTerminalEnd(p)
		close(s.done)
	}()
	return s, nil
}

func exitCode(err error) int {
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	default:
		return -1
	}
}

// Read reads terminal output; it returns io.EOF once the shell has exited.
func (s *Shell) Read(p []byte) (int, error) {
	n, err := s.pty.Read(p)
	if err != nil && n == 0 {
		// Linux reports EIO when the last process closes the terminal.
		return 0, io.EOF
	}
	return n, nil
}

// Write sends keystrokes.
func (s *Shell) Write(p []byte) (int, error) { return s.pty.Write(p) }

// Resize changes the terminal size.
func (s *Shell) Resize(cols, rows int) error { return resize(s.pty, cols, rows) }

// Wait blocks until the shell exits and returns its exit code.
func (s *Shell) Wait() int {
	<-s.done
	return s.code
}

// Close ends the shell.
func (s *Shell) Close() error {
	s.once.Do(func() {
		select {
		case <-s.done:
		default:
			if s.cmd.Process != nil {
				s.cmd.Process.Kill()
			}
		}
		s.pty.Close()
	})
	return nil
}
