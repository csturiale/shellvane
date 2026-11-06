package files

import (
	"context"
	"sort"
	"time"
)

// Difference kinds.
const (
	DiffNew   = "new"   // only on the source side
	DiffNewer = "newer" // newer on the source side
	DiffOlder = "older" // newer on the destination side (not copied)
	DiffExtra = "extra" // only on the destination side
)

// Diff is a difference between two folders.
type Diff struct {
	Rel     string `json:"rel"` // path relative to the compared folders, "/"-separated
	Kind    string `json:"kind"`
	IsDir   bool   `json:"isDir"`
	SrcSize int64  `json:"srcSize"`
	DstSize int64  `json:"dstSize"`
}

// maxCompare bounds the entries compared, for huge trees.
const maxCompare = 50000

// modSlack absorbs the second precision of SFTP and some file systems.
const modSlack = time.Second

// Compare lists how dstDir differs from srcDir, recursively. Folders
// missing on one side are reported once, not file by file. Files of the
// same size and time (within a second) are the same.
func Compare(ctx context.Context, src FS, srcDir string, dst FS, dstDir string) ([]Diff, error) {
	var out []Diff
	seen := 0
	var walk func(srcPath, dstPath, rel string) error
	walk = func(srcPath, dstPath, rel string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		a, err := src.List(srcPath)
		if err != nil {
			return err
		}
		b, err := dst.List(dstPath)
		if err != nil {
			return err
		}
		seen += len(a) + len(b)
		if seen > maxCompare {
			return nil
		}
		other := map[string]Entry{}
		for _, e := range b {
			other[e.Name] = e
		}
		join := func(name string) string {
			if rel == "" {
				return name
			}
			return rel + "/" + name
		}
		for _, e := range a {
			if e.IsLink {
				continue
			}
			o, ok := other[e.Name]
			delete(other, e.Name)
			switch {
			case !ok:
				out = append(out, Diff{Rel: join(e.Name), Kind: DiffNew, IsDir: e.IsDir, SrcSize: e.Size})
			case e.IsDir && o.IsDir:
				if err := walk(e.Path, o.Path, join(e.Name)); err != nil {
					return err
				}
			case e.IsDir != o.IsDir:
				// A file on one side and a folder on the other: left to the user.
			default:
				if kind := compareFiles(e, o); kind != "" {
					out = append(out, Diff{Rel: join(e.Name), Kind: kind, SrcSize: e.Size, DstSize: o.Size})
				}
			}
		}
		for _, o := range other {
			if !o.IsLink {
				out = append(out, Diff{Rel: join(o.Name), Kind: DiffExtra, IsDir: o.IsDir, DstSize: o.Size})
			}
		}
		return nil
	}
	if err := walk(srcDir, dstDir, ""); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rel < out[j].Rel })
	return out, nil
}

func compareFiles(a, b Entry) string {
	ta, errA := time.Parse(time.RFC3339Nano, a.ModTime)
	tb, errB := time.Parse(time.RFC3339Nano, b.ModTime)
	if errA != nil || errB != nil {
		if a.Size != b.Size {
			return DiffNewer
		}
		return ""
	}
	switch d := ta.Sub(tb); {
	case d > modSlack:
		return DiffNewer
	case d < -modSlack:
		return DiffOlder
	case a.Size != b.Size:
		return DiffNewer // same time, different content: the source wins
	}
	return ""
}
