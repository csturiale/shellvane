package sessionlog

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes of the file listing.
const (
	ErrNotFound errs.Code = "sessionlog.notFound"
	ErrTooLarge errs.Code = "sessionlog.tooLarge"
)

// MaxRead bounds a file read for the viewer or the player.
const MaxRead = 64 << 20

// File is a saved log or recording.
type File struct {
	Path     string  `json:"path"`
	Name     string  `json:"name"`
	Kind     string  `json:"kind"` // "log" or "recording"
	Size     int64   `json:"size"`
	Modified string  `json:"modified"`
	Title    string  `json:"title"`
	Duration float64 `json:"duration"` // recordings, seconds
}

// List returns the logs (*.log) and recordings (*.cast) under dir, newest
// first.
func List(dir string) ([]File, error) {
	out := []File{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		kind := ""
		switch filepath.Ext(path) {
		case ".log":
			kind = "log"
		case ".cast":
			kind = "recording"
		default:
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		f := File{Path: path, Name: d.Name(), Kind: kind, Size: fi.Size(), Modified: fi.ModTime().UTC().Format(time.RFC3339)}
		if kind == "recording" {
			f.Title, f.Duration = castInfo(path)
		}
		out = append(out, f)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified > out[j].Modified })
	return out, nil
}

// castInfo reads the title and the time of the last event.
func castInfo(path string) (string, float64) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 16<<20)
	var h Header
	if sc.Scan() {
		json.Unmarshal(sc.Bytes(), &h)
	}
	var last float64
	for sc.Scan() {
		var ev []any
		if json.Unmarshal(sc.Bytes(), &ev) == nil && len(ev) > 0 {
			if t, ok := ev[0].(float64); ok {
				last = t
			}
		}
	}
	return h.Title, last
}

// Read returns a file under dir (never outside it).
func Read(dir, path string) (string, error) {
	if err := inside(dir, path); err != nil {
		return "", err
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", errs.New(ErrNotFound, errs.Params{"path": path})
	}
	if fi.Size() > MaxRead {
		return "", errs.New(ErrTooLarge, errs.Params{"path": path})
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

// Delete removes a file under dir.
func Delete(dir, path string) error {
	if err := inside(dir, path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		return errs.New(ErrNotFound, errs.Params{"path": path})
	}
	return nil
}

func inside(dir, path string) error {
	rel, err := filepath.Rel(dir, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return errs.New(ErrNotFound, errs.Params{"path": path})
	}
	return nil
}
