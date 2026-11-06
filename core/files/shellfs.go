package files

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Runner runs shell commands on a server, each on its own channel.
type Runner interface {
	// Run executes a command and returns its output; the error carries
	// the standard error text.
	Run(cmd string) ([]byte, error)
	// Stream starts a command and returns its output.
	Stream(cmd string) (io.ReadCloser, error)
	// Sink starts a command and returns its input; Close waits for it.
	Sink(cmd string) (io.WriteCloser, error)
}

// ShellFS is a file system over shell commands, for servers without the
// SFTP subsystem (like scp does). It needs a POSIX shell and GNU find.
type ShellFS struct {
	r Runner
}

// NewShellFS returns a ShellFS; Probe tells whether the server can serve it.
func NewShellFS(r Runner) *ShellFS { return &ShellFS{r: r} }

// Probe checks that the commands ShellFS needs are there.
func (s *ShellFS) Probe() error {
	_, err := s.r.Run(`find / -maxdepth 0 -printf '' && command -v cat >/dev/null && command -v tail >/dev/null`)
	return err
}

// q quotes a path for the shell.
func q(p string) string { return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'" }

const findFormat = `-printf '%f\0%s\0%m\0%y\0%Y\0%T@\0%U\0%G\0%l\0'`

func (s *ShellFS) Home() (string, error) {
	out, err := s.r.Run(`printf %s "$HOME"`)
	return string(out), shellErr(err, "~")
}

func (s *ShellFS) List(dir string) ([]Entry, error) {
	out, err := s.r.Run("find " + q(dir) + " -mindepth 1 -maxdepth 1 " + findFormat)
	if err != nil {
		return nil, shellErr(err, dir)
	}
	list := parseFind(dir, out)
	sortEntries(list)
	return list, nil
}

func (s *ShellFS) Stat(p string) (Entry, error) {
	out, err := s.r.Run("find " + q(p) + " -maxdepth 0 " + findFormat)
	if err != nil {
		return Entry{}, shellErr(err, p)
	}
	list := parseFind(path.Dir(p), out)
	if len(list) == 0 {
		return Entry{}, errs.New(ErrNotFound, errs.Params{"path": p})
	}
	e := list[0]
	e.Name, e.Path = path.Base(p), p
	return e, nil
}

// parseFind reads 9 NUL-separated fields per entry.
func parseFind(dir string, out []byte) []Entry {
	fields := bytes.Split(out, []byte{0})
	var list []Entry
	for i := 0; i+8 < len(fields); i += 9 {
		f := func(n int) string { return string(fields[i+n]) }
		size, _ := strconv.ParseInt(f(1), 10, 64)
		perm, _ := strconv.ParseUint(f(2), 8, 32)
		secs, _ := strconv.ParseFloat(f(5), 64)
		uid, _ := strconv.Atoi(f(6))
		gid, _ := strconv.Atoi(f(7))
		kind, target := f(3), f(4)
		e := Entry{
			Name:    f(0),
			Path:    path.Join(dir, f(0)),
			Size:    size,
			Perm:    uint32(perm),
			IsDir:   kind == "d" || (kind == "l" && target == "d"),
			IsLink:  kind == "l",
			ModTime: time.Unix(int64(secs), 0).UTC().Format(time.RFC3339),
			UID:     uid,
			GID:     gid,
			Target:  f(8),
		}
		e.Mode = modeString(kind, uint32(perm))
		list = append(list, e)
	}
	return list
}

// modeString renders "drwxr-xr-x" from find's type and octal mode.
func modeString(kind string, perm uint32) string {
	m := os.FileMode(perm & 0o777)
	switch kind {
	case "d":
		m |= os.ModeDir
	case "l":
		m |= os.ModeSymlink
	}
	return m.String()
}

func (s *ShellFS) Mkdir(p string) error {
	return s.run(p, "[ -e "+q(p)+" ] && exit 17; mkdir -- "+q(p))
}

func (s *ShellFS) Rename(oldPath, newPath string) error {
	if _, err := s.r.Run("[ -e " + q(newPath) + " ] || [ -L " + q(newPath) + " ] && exit 17; mv -- " + q(oldPath) + " " + q(newPath)); err != nil {
		return shellErr(err, newPath)
	}
	return nil
}

func (s *ShellFS) Remove(p string) error { return s.run(p, "rm -rf -- "+q(p)) }

func (s *ShellFS) Chmod(p string, mode uint32) error {
	return s.run(p, fmt.Sprintf("chmod %o -- %s", mode&0o7777, q(p)))
}

func (s *ShellFS) Chown(p string, uid, gid int) error {
	return s.run(p, fmt.Sprintf("chown -h %d:%d -- %s", uid, gid, q(p)))
}

func (s *ShellFS) Symlink(target, link string) error {
	return s.run(link, "ln -s -- "+q(target)+" "+q(link))
}

func (s *ShellFS) ReadLink(p string) (string, error) {
	out, err := s.r.Run("readlink -- " + q(p))
	return strings.TrimRight(string(out), "\n"), shellErr(err, p)
}

func (s *ShellFS) Open(p string) (io.ReadCloser, error) { return s.OpenAt(p, 0) }

func (s *ShellFS) OpenAt(p string, offset int64) (io.ReadCloser, error) {
	// Fail early on a missing file: a stream reports errors only at the end.
	if _, err := s.r.Run("[ -r " + q(p) + " ] || { echo 'No such file' >&2; exit 2; }"); err != nil {
		return nil, shellErr(err, p)
	}
	rc, err := s.r.Stream(fmt.Sprintf("tail -c +%d -- %s", offset+1, q(p)))
	return rc, shellErr(err, p)
}

func (s *ShellFS) Create(p string) (io.WriteCloser, error) {
	w, err := s.r.Sink("cat > " + q(p))
	return w, shellErr(err, p)
}

func (s *ShellFS) Append(p string) (io.WriteCloser, error) {
	w, err := s.r.Sink("cat >> " + q(p))
	return w, shellErr(err, p)
}

func (s *ShellFS) Join(elem ...string) string { return path.Join(elem...) }
func (s *ShellFS) Dir(p string) string        { return path.Dir(p) }

func (s *ShellFS) run(p, cmd string) error {
	_, err := s.r.Run(cmd)
	return shellErr(err, p)
}

// ExitError is a command that failed, with its exit status and stderr.
type ExitError struct {
	Status int
	Stderr string
}

func (e *ExitError) Error() string { return fmt.Sprintf("exit %d: %s", e.Status, e.Stderr) }

// shellErr maps a failed command to the package's error codes.
func shellErr(err error, p string) error {
	if err == nil {
		return nil
	}
	params := errs.Params{"path": p}
	var exit *ExitError
	if errors.As(err, &exit) {
		low := strings.ToLower(exit.Stderr)
		switch {
		case exit.Status == 17 || strings.Contains(low, "file exists"):
			return errs.Wrap(err, ErrExists, params)
		case strings.Contains(low, "no such file"):
			return errs.Wrap(err, ErrNotFound, params)
		case strings.Contains(low, "permission denied"), strings.Contains(low, "operation not permitted"):
			return errs.Wrap(err, ErrPermission, params)
		}
	}
	return errs.Wrap(err, ErrFailed, params)
}
