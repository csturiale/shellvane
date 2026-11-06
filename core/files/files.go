// Package files gives local and remote (SFTP) file systems one interface,
// so browsing and transfers work the same in both directions.
package files

import (
	"io"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrNotFound   errs.Code = "files.notFound"
	ErrPermission errs.Code = "files.permission"
	ErrExists     errs.Code = "files.exists"
	ErrFailed     errs.Code = "files.failed"
	ErrCancelled  errs.Code = "files.cancelled"
)

// Entry is a file or directory.
type Entry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"` // e.g. "drwxr-xr-x"
	Perm    uint32 `json:"perm"` // permission bits, e.g. 0o755
	IsDir   bool   `json:"isDir"`
	IsLink  bool   `json:"isLink"`
	ModTime string `json:"modTime"` // RFC 3339
	// UID and GID own the file (-1 when unknown, e.g. on Windows).
	UID int `json:"uid"`
	GID int `json:"gid"`
	// Target is where a symbolic link points.
	Target string `json:"target"`
}

// FS is a file system: the local disk or a remote SFTP server.
type FS interface {
	// Home is where browsing starts.
	Home() (string, error)
	List(dir string) ([]Entry, error)
	Stat(path string) (Entry, error)
	Mkdir(path string) error
	Rename(oldPath, newPath string) error
	// Remove deletes a file, or a directory with its contents.
	Remove(path string) error
	// Chmod sets permission bits (0o000–0o7777).
	Chmod(path string, mode uint32) error
	// Chown sets the owner and group (numeric IDs).
	Chown(path string, uid, gid int) error
	// Symlink creates link pointing to target.
	Symlink(target, link string) error
	ReadLink(path string) (string, error)
	Open(path string) (io.ReadCloser, error)
	// OpenAt reads from an offset (to resume a transfer).
	OpenAt(path string, offset int64) (io.ReadCloser, error)
	Create(path string) (io.WriteCloser, error)
	// Append writes at the end of an existing file (to resume a transfer).
	Append(path string) (io.WriteCloser, error)
	Join(elem ...string) string
	Dir(path string) string
}

func entryOf(dir string, fi fs.FileInfo, join func(...string) string) Entry {
	uid, gid := owner(fi)
	return Entry{
		UID:     uid,
		GID:     gid,
		Name:    fi.Name(),
		Path:    join(dir, fi.Name()),
		Size:    fi.Size(),
		Mode:    fi.Mode().String(),
		Perm:    unixPerm(fi.Mode()),
		IsDir:   fi.IsDir(),
		IsLink:  fi.Mode()&fs.ModeSymlink != 0,
		ModTime: fi.ModTime().UTC().Format(time.RFC3339),
	}
}

// sortEntries puts directories first, then names case-insensitively.
func sortEntries(list []Entry) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].IsDir != list[j].IsDir {
			return list[i].IsDir
		}
		return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
	})
}

// classify maps file system errors to coded errors.
func classify(err error, path string) error {
	if err == nil {
		return nil
	}
	p := errs.Params{"path": path}
	switch {
	case errorsIs(err, fs.ErrNotExist):
		return errs.Wrap(err, ErrNotFound, p)
	case errorsIs(err, fs.ErrPermission):
		return errs.Wrap(err, ErrPermission, p)
	case errorsIs(err, fs.ErrExist):
		return errs.Wrap(err, ErrExists, p)
	default:
		return errs.Wrap(err, ErrFailed, p)
	}
}

// unixPerm converts a Go file mode to Unix permission bits, special bits
// included (0o4000 setuid, 0o2000 setgid, 0o1000 sticky).
func unixPerm(m fs.FileMode) uint32 {
	p := uint32(m.Perm())
	if m&fs.ModeSetuid != 0 {
		p |= 0o4000
	}
	if m&fs.ModeSetgid != 0 {
		p |= 0o2000
	}
	if m&fs.ModeSticky != 0 {
		p |= 0o1000
	}
	return p
}

// goMode is the inverse of unixPerm.
func goMode(p uint32) fs.FileMode {
	m := fs.FileMode(p & 0o777)
	if p&0o4000 != 0 {
		m |= fs.ModeSetuid
	}
	if p&0o2000 != 0 {
		m |= fs.ModeSetgid
	}
	if p&0o1000 != 0 {
		m |= fs.ModeSticky
	}
	return m
}

// ChmodAll applies mode to path and, for directories, to everything below
// it. Links are not followed.
func ChmodAll(fsys FS, path string, mode uint32, recursive bool) error {
	if err := fsys.Chmod(path, mode); err != nil {
		return err
	}
	if !recursive {
		return nil
	}
	e, err := fsys.Stat(path)
	if err != nil || !e.IsDir {
		return err
	}
	list, err := fsys.List(path)
	if err != nil {
		return err
	}
	for _, c := range list {
		if c.IsLink {
			continue
		}
		if err := ChmodAll(fsys, c.Path, mode, c.IsDir); err != nil {
			return err
		}
	}
	return nil
}
