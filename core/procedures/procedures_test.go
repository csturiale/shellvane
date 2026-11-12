package procedures

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/csturiale/shellvane/core/errs"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFolderSource(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "db", "restore.md"), "intro\n# Restore the database\n\n```sh\npg_restore\n```\n")
	write(t, filepath.Join(dir, "README.markdown"), "no heading here")
	write(t, filepath.Join(dir, "notes.txt"), "not markdown")
	write(t, filepath.Join(dir, ".github", "x.md"), "# hidden")
	outside := filepath.Join(t.TempDir(), "secret.md")
	write(t, outside, "# secret")
	os.Symlink(outside, filepath.Join(dir, "link.md"))

	l := Library{Dir: t.TempDir()}
	src := Source{Kind: "folder", Folder: dir}
	docs, err := l.List(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []Doc{{Path: "README.markdown", Title: "README"}, {Path: "db/restore.md", Title: "Restore the database"}}
	if len(docs) != len(want) || docs[0] != want[0] || docs[1] != want[1] {
		t.Fatalf("docs = %+v", docs)
	}
	text, err := l.Read(src, "db/restore.md")
	if err != nil || !strings.Contains(text, "pg_restore") {
		t.Fatalf("read = %q, %v", text, err)
	}
	for _, bad := range []string{"../" + filepath.Base(dir) + "/notes.txt", "link.md", "notes.txt", "../../etc/passwd"} {
		if _, err := l.Read(src, bad); errs.CodeOf(err) != ErrNotFound {
			t.Errorf("read %q: %v", bad, err)
		}
	}
	if _, err := l.List(Source{Kind: "folder", Folder: "relative/path"}); errs.CodeOf(err) != ErrSource {
		t.Errorf("relative folder: %v", err)
	}
}

func TestGitSource(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "--quiet", "--initial-branch=main")
	write(t, filepath.Join(repo, "ops", "deploy.md"), "# Deploy\n")
	run("add", ".")
	run("commit", "--quiet", "-m", "first")

	l := Library{Dir: t.TempDir()}
	src := Source{Kind: "git", URL: repo, Branch: "main", Folder: "ops"}
	if !l.Synced(src).IsZero() {
		t.Fatal("synced before the clone")
	}
	if err := l.Sync(ctx, src); err != nil {
		t.Fatal(err)
	}
	if docs, err := l.List(src); err != nil || len(docs) != 1 || docs[0].Title != "Deploy" {
		t.Fatalf("docs = %+v, %v", docs, err)
	}

	write(t, filepath.Join(repo, "ops", "rollback.md"), "# Rollback\n")
	run("add", ".")
	run("commit", "--quiet", "-m", "second")
	if err := l.Sync(ctx, src); err != nil {
		t.Fatal(err)
	}
	if docs, _ := l.List(src); len(docs) != 2 {
		t.Fatalf("after the update: %+v", docs)
	}
	if l.Synced(src).IsZero() {
		t.Fatal("no sync time")
	}
	// The folder inside the repository cannot lead out of the clone.
	if root, _ := l.Root(Source{Kind: "git", URL: repo, Folder: "../../.."}); !strings.HasPrefix(root, l.Dir) {
		t.Fatalf("root = %s", root)
	}
}
