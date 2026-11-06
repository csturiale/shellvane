package files

import (
	"io"
	"os"
	"path"

	"github.com/pkg/sftp"
)

// Remote is a directory tree on an SFTP server. Paths always use "/".
type Remote struct {
	c *sftp.Client
}

// NewRemote wraps an SFTP client.
func NewRemote(c *sftp.Client) *Remote { return &Remote{c: c} }

// Close ends the SFTP session (not the SSH connection).
func (r *Remote) Close() error { return r.c.Close() }

func (r *Remote) Home() (string, error) { return r.c.Getwd() }

func (r *Remote) List(dir string) ([]Entry, error) {
	fis, err := r.c.ReadDir(dir)
	if err != nil {
		return nil, classify(err, dir)
	}
	out := make([]Entry, 0, len(fis))
	for _, fi := range fis {
		e := entryOf(dir, fi, r.Join)
		if e.IsLink {
			if target, err := r.c.Stat(e.Path); err == nil {
				e.IsDir = target.IsDir()
			}
			e.Target, _ = r.c.ReadLink(e.Path)
		}
		out = append(out, e)
	}
	sortEntries(out)
	return out, nil
}

func (r *Remote) Stat(p string) (Entry, error) {
	fi, err := r.c.Stat(p)
	if err != nil {
		return Entry{}, classify(err, p)
	}
	return entryOf(path.Dir(p), fi, r.Join), nil
}

func (r *Remote) Mkdir(p string) error { return classify(r.c.Mkdir(p), p) }

func (r *Remote) Rename(oldPath, newPath string) error {
	if _, err := r.c.Lstat(newPath); err == nil {
		return classify(os.ErrExist, newPath)
	}
	return classify(r.c.Rename(oldPath, newPath), oldPath)
}

func (r *Remote) Remove(p string) error {
	fi, err := r.c.Lstat(p)
	if err != nil {
		return classify(err, p)
	}
	if fi.IsDir() {
		return classify(r.c.RemoveAll(p), p)
	}
	return classify(r.c.Remove(p), p)
}

func (r *Remote) Chmod(p string, mode uint32) error {
	return classify(r.c.Chmod(p, goMode(mode)), p)
}

func (r *Remote) Chown(p string, uid, gid int) error {
	return classify(r.c.Chown(p, uid, gid), p)
}

func (r *Remote) Symlink(target, link string) error {
	return classify(r.c.Symlink(target, link), link)
}

func (r *Remote) ReadLink(p string) (string, error) {
	t, err := r.c.ReadLink(p)
	return t, classify(err, p)
}

func (r *Remote) OpenAt(p string, offset int64) (io.ReadCloser, error) {
	f, err := r.c.Open(p)
	if err != nil {
		return nil, classify(err, p)
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		f.Close()
		return nil, classify(err, p)
	}
	return f, nil
}

func (r *Remote) Append(p string) (io.WriteCloser, error) {
	f, err := r.c.OpenFile(p, os.O_WRONLY|os.O_APPEND)
	if err != nil {
		return nil, classify(err, p)
	}
	return f, nil
}

func (r *Remote) Open(p string) (io.ReadCloser, error) {
	f, err := r.c.Open(p)
	if err != nil {
		return nil, classify(err, p)
	}
	return f, nil
}

func (r *Remote) Create(p string) (io.WriteCloser, error) {
	f, err := r.c.Create(p)
	if err != nil {
		return nil, classify(err, p)
	}
	return f, nil
}

func (r *Remote) Join(elem ...string) string { return path.Join(elem...) }
func (r *Remote) Dir(p string) string        { return path.Dir(p) }
