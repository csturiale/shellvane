package files

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write := func(dir, rel, body string, mod time.Time) {
		p := filepath.Join(dir, rel)
		os.MkdirAll(filepath.Dir(p), 0o700)
		os.WriteFile(p, []byte(body), 0o600)
		os.Chtimes(p, mod, mod)
	}
	old, now := time.Now().Add(-time.Hour), time.Now()
	write(a, "same.txt", "x", old)
	write(b, "same.txt", "x", old.Add(300*time.Millisecond)) // within the slack
	write(a, "changed.txt", "new", now)
	write(b, "changed.txt", "old", old)
	write(a, "stale.txt", "a", old)
	write(b, "stale.txt", "b", now)
	write(a, "only-a.txt", "1", now)
	write(a, "newdir/deep/f.txt", "1", now)
	write(a, "sub/x.txt", "1", now)
	write(b, "sub/x.txt", "1", now)
	write(b, "sub/extra.txt", "1", now)
	got, err := Compare(context.Background(), Local{}, a, Local{}, b)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"changed.txt": DiffNewer, "newdir": DiffNew, "only-a.txt": DiffNew, "stale.txt": DiffOlder, "sub/extra.txt": DiffExtra}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for _, d := range got {
		if want[d.Rel] != d.Kind {
			t.Errorf("%s = %s, want %s", d.Rel, d.Kind, want[d.Rel])
		}
		if d.Rel == "newdir" && !d.IsDir {
			t.Error("newdir is a folder")
		}
	}
}
