package files

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

var errorsIs = errors.Is

// Local is the local disk.
type Local struct{}

func (Local) Home() (string, error) { return os.UserHomeDir() }

func (l Local) List(dir string) ([]Entry, error) {
	des, err := os.ReadDir(dir)
	if err != nil {
		return nil, classify(err, dir)
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			continue // removed while listing
		}
		e := entryOf(dir, fi, l.Join)
		if e.IsLink {
			if target, err := os.Stat(e.Path); err == nil {
				e.IsDir = target.IsDir()
			}
			e.Target, _ = os.Readlink(e.Path)
		}
		out = append(out, e)
	}
	sortEntries(out)
	return out, nil
}

func (l Local) Stat(path string) (Entry, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return Entry{}, classify(err, path)
	}
	return entryOf(filepath.Dir(path), fi, l.Join), nil
}

func (Local) Mkdir(path string) error { return classify(os.Mkdir(path, 0o755), path) }

func (Local) Rename(oldPath, newPath string) error {
	if _, err := os.Lstat(newPath); err == nil {
		return classify(os.ErrExist, newPath)
	}
	return classify(os.Rename(oldPath, newPath), oldPath)
}

func (Local) Remove(path string) error { return classify(os.RemoveAll(path), path) }

func (Local) Chmod(path string, mode uint32) error {
	return classify(os.Chmod(path, goMode(mode)), path)
}

func (Local) Chown(path string, uid, gid int) error {
	return classify(os.Lchown(path, uid, gid), path)
}

func (Local) Symlink(target, link string) error {
	return classify(os.Symlink(target, link), link)
}

func (Local) ReadLink(path string) (string, error) {
	t, err := os.Readlink(path)
	return t, classify(err, path)
}

func (Local) OpenAt(path string, offset int64) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, classify(err, path)
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		f.Close()
		return nil, classify(err, path)
	}
	return f, nil
}

func (Local) Append(path string) (io.WriteCloser, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	return f, classify(err, path)
}

func (Local) Open(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	return f, classify(err, path)
}

func (Local) Create(path string) (io.WriteCloser, error) {
	f, err := os.Create(path)
	return f, classify(err, path)
}

func (Local) Join(elem ...string) string { return filepath.Join(elem...) }
func (Local) Dir(path string) string     { return filepath.Dir(path) }
