package files

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/csturiale/shellvane/core/errs"
)

// Progress reports bytes copied so far out of total.
type Progress func(done, total int64, current string)

// What to do when a file already exists at the destination.
const (
	ConflictOverwrite = "overwrite"
	ConflictSkip      = "skip"
	ConflictRename    = "rename" // "name (1).ext"
	ConflictNewer     = "newer"  // copy only if the source is newer
	ConflictResume    = "resume" // continue a smaller file, skip an equal one
)

// Options tune a copy.
type Options struct {
	Conflict string `json:"conflict"`
}

// Copy copies src (a file or a directory, recursively) from one file system
// into the directory dstDir of another. It stops when ctx ends. Existing
// files with the same name are overwritten; directories are merged.
func Copy(ctx context.Context, from FS, src string, to FS, dstDir string, progress Progress) error {
	return CopyWith(ctx, from, src, to, dstDir, Options{}, progress)
}

// CopyWith is Copy with a conflict policy.
func CopyWith(ctx context.Context, from FS, src string, to FS, dstDir string, opts Options, progress Progress) error {
	root, err := from.Stat(src)
	if err != nil {
		return err
	}
	total, err := size(ctx, from, root)
	if err != nil {
		return err
	}
	var done int64
	return copyEntry(ctx, from, root, to, dstDir, opts, &done, total, progress)
}

func size(ctx context.Context, fs FS, e Entry) (int64, error) {
	if !e.IsDir {
		return e.Size, nil
	}
	if ctx.Err() != nil {
		return 0, errs.Wrap(ctx.Err(), ErrCancelled, nil)
	}
	list, err := fs.List(e.Path)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, c := range list {
		if c.IsLink || !safeEntryName(c.Name) {
			continue // links are not followed: no loops, no surprises
		}
		s, err := size(ctx, fs, c)
		if err != nil {
			return 0, err
		}
		n += s
	}
	return n, nil
}

// safeEntryName: names come from the other side (a server could send
// "..", or a name with a separator) and must stay inside the destination.
func safeEntryName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
}

func copyEntry(ctx context.Context, from FS, e Entry, to FS, dstDir string, opts Options, done *int64, total int64, progress Progress) error {
	if !safeEntryName(e.Name) {
		return nil // skipped: it would land outside the destination
	}
	dst := to.Join(dstDir, e.Name)
	if e.IsDir {
		if err := to.Mkdir(dst); err != nil && errs.CodeOf(err) != ErrExists {
			return err
		}
		list, err := from.List(e.Path)
		if err != nil {
			return err
		}
		for _, c := range list {
			if c.IsLink {
				continue
			}
			if err := copyEntry(ctx, from, c, to, dst, opts, done, total, progress); err != nil {
				return err
			}
		}
		return nil
	}

	// An existing file: apply the conflict policy.
	var offset int64
	if existing, err := to.Stat(dst); err == nil && !existing.IsDir {
		switch opts.Conflict {
		case ConflictSkip:
			return skip(e, done, total, progress)
		case ConflictNewer:
			if existing.ModTime >= e.ModTime {
				return skip(e, done, total, progress)
			}
		case ConflictRename:
			dst = freeName(to, dstDir, e.Name)
		case ConflictResume:
			if existing.Size == e.Size {
				return skip(e, done, total, progress)
			}
			if existing.Size < e.Size {
				offset = existing.Size
			}
		}
	}

	var r io.ReadCloser
	var w io.WriteCloser
	var err error
	if offset > 0 {
		r, err = from.OpenAt(e.Path, offset)
	} else {
		r, err = from.Open(e.Path)
	}
	if err != nil {
		return err
	}
	defer r.Close()
	if offset > 0 {
		w, err = to.Append(dst)
		*done += offset
	} else {
		w, err = to.Create(dst)
	}
	if err != nil {
		return err
	}
	buf := make([]byte, 256*1024)
	for {
		if ctx.Err() != nil {
			w.Close()
			if opts.Conflict != ConflictResume {
				to.Remove(dst) // never leave a truncated file behind
			}
			return errs.Wrap(ctx.Err(), ErrCancelled, nil)
		}
		n, rerr := r.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				w.Close()
				return errs.Wrap(werr, ErrFailed, errs.Params{"path": dst})
			}
			*done += int64(n)
			if progress != nil {
				progress(*done, total, e.Name)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			w.Close()
			return errs.Wrap(rerr, ErrFailed, errs.Params{"path": e.Path})
		}
	}
	if err := w.Close(); err != nil {
		return errs.Wrap(err, ErrFailed, errs.Params{"path": dst})
	}
	return nil
}

func skip(e Entry, done *int64, total int64, progress Progress) error {
	*done += e.Size
	if progress != nil {
		progress(*done, total, e.Name)
	}
	return nil
}

// freeName returns "name (n).ext" in dir that does not exist yet.
func freeName(fsys FS, dir, name string) string {
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		p := fsys.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := fsys.Stat(p); err != nil {
			return p
		}
	}
}
