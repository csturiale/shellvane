package sync

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/net/webdav"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/store"
)

func deviceOn(t *testing.T, st Store, id string) *device {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	key, err := OpenKeyIn(context.Background(), st, "correct horse", testKDF)
	if err != nil {
		t.Fatal(err)
	}
	return &device{db: db, hosts: hosts.New(db), s: &Syncer{DB: db, Store: st, Key: key, Name: id}, state: &State{Device: id}}
}

func roundTrip(t *testing.T, a, b *device) {
	t.Helper()
	ctx := context.Background()
	if _, err := a.hosts.CreateHost(ctx, hosts.HostInput{Name: "web", Hostname: "web.example.com"}); err != nil {
		t.Fatal(err)
	}
	a.sync(t)
	if r := b.sync(t); r.Updated != 1 || r.Devices != 1 {
		t.Fatalf("pull = %+v", r)
	}
	if got := names(t, b); got["web"] != "web.example.com" {
		t.Fatalf("b hosts = %v", got)
	}
	if _, err := b.hosts.CreateHost(ctx, hosts.HostInput{Name: "db", Hostname: "db.example.com"}); err != nil {
		t.Fatal(err)
	}
	b.sync(t)
	a.sync(t)
	if got := names(t, a); got["db"] != "db.example.com" {
		t.Fatalf("a hosts = %v", got)
	}
	devs, err := a.s.Devices(ctx, a.state.Device)
	if err != nil || len(devs) != 1 || devs[0].Name != b.s.Name {
		t.Fatalf("devices = %+v, %v", devs, err)
	}
}

func TestSyncThroughWebDAV(t *testing.T) {
	h := &webdav.Handler{FileSystem: webdav.NewMemFS(), LockSystem: webdav.NewMemLS()}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "me" || p != "pw" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	}))
	defer srv.Close()

	url := srv.URL + "/Shellvane-sync"
	a := deviceOn(t, WebDAVStore{URL: url, User: "me", Password: "pw"}, "laptop")
	b := deviceOn(t, WebDAVStore{URL: url + "/", User: "me", Password: "pw"}, "desktop")
	roundTrip(t, a, b)

	if _, err := OpenKeyIn(context.Background(), WebDAVStore{URL: url, User: "me", Password: "pw"}, "wrong passphrase", testKDF); errs.CodeOf(err) != ErrWrongPassphrase {
		t.Errorf("wrong passphrase: %v", err)
	}
	if _, err := OpenKeyIn(context.Background(), WebDAVStore{URL: url, User: "me", Password: "nope"}, "correct horse", testKDF); errs.CodeOf(err) != ErrWebDAV {
		t.Errorf("wrong password: %v", err)
	}
	if w := (WebDAVStore{URL: "https://me:pw@dav.example.org/x"}).Where(); w != "https://dav.example.org/x" {
		t.Errorf("Where = %q", w)
	}
}

func TestSyncThroughGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	remote := filepath.Join(t.TempDir(), "sync.git")
	if out, err := exec.Command("git", "init", "--quiet", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	a := deviceOn(t, GitStore{Remote: remote, Dir: filepath.Join(t.TempDir(), "clone")}, "laptop")
	b := deviceOn(t, GitStore{Remote: remote, Dir: filepath.Join(t.TempDir(), "clone")}, "desktop")
	roundTrip(t, a, b)

	bad := GitStore{Remote: filepath.Join(t.TempDir(), "missing.git"), Dir: filepath.Join(t.TempDir(), "clone")}
	if _, err := OpenKeyIn(context.Background(), bad, "correct horse", testKDF); errs.CodeOf(err) != ErrGit {
		t.Errorf("missing repository: %v", err)
	}
}

func TestGitExclusiveConflict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	remote := filepath.Join(t.TempDir(), "team.git")
	if out, err := exec.Command("git", "init", "--quiet", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	a := GitStore{Remote: remote, Dir: filepath.Join(t.TempDir(), "a")}
	b := GitStore{Remote: remote, Dir: filepath.Join(t.TempDir(), "b")}
	for _, s := range []GitStore{a, b} {
		if err := s.Begin(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// Both write the same file of an empty repository; a pushes first.
	if err := a.Put(ctx, "team.json", []byte("a1")); err != nil {
		t.Fatal(err)
	}
	if err := a.EndExclusive(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := b.Put(ctx, "team.json", []byte("b1")); err != nil {
		t.Fatal(err)
	}
	if err := b.EndExclusive(ctx, "b"); errs.CodeOf(err) != ErrGitConflict {
		t.Fatalf("second push: %v", err)
	}
	// b's clone now holds a's version; redoing the change works.
	if got, _ := b.Get(ctx, "team.json"); string(got) != "a1" {
		t.Fatalf("after reset b reads %q", got)
	}
	if err := b.Put(ctx, "team.json", []byte("a1+b")); err != nil {
		t.Fatal(err)
	}
	if err := b.EndExclusive(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	// a, behind now, conflicts and catches up the same way.
	if err := a.Put(ctx, "team.json", []byte("a2")); err != nil {
		t.Fatal(err)
	}
	if err := a.EndExclusive(ctx, "a"); errs.CodeOf(err) != ErrGitConflict {
		t.Fatalf("stale push: %v", err)
	}
	if got, _ := a.Get(ctx, "team.json"); string(got) != "a1+b" {
		t.Fatalf("after reset a reads %q", got)
	}
}

func TestGitErrorKinds(t *testing.T) {
	cases := map[string]errs.Code{
		"fatal: Authentication failed for 'https://ghb.example/scm/aic/demo.git/'":                                                                ErrGitAuth,
		"fatal: could not read Username for 'https://github.com': terminal prompts disabled":                                                      ErrGitAuth,
		"fatal: unable to access 'https://github.com/x/y.git/': Failed to connect to github.com port 443 after 29 ms: Couldn't connect to server": ErrGitNetwork,
		"fatal: 'x' does not appear to be a git repository":                                                                                       ErrGit,
	}
	for out, want := range cases {
		if got := gitCode(out); got != want {
			t.Errorf("gitCode(%q) = %s, want %s", out, got, want)
		}
	}
	if r := gitReason("fatal: '/x/a.git' does not appear to be a git repository\nfatal: Could not read from remote repository.\n\nPlease make sure you have the correct access rights\nand the repository exists."); r != "'/x/a.git' does not appear to be a git repository" {
		t.Errorf("gitReason = %q", r)
	}
	if u := (GitStore{Remote: "https://ghb.example/scm/demo.git", User: "c1"}).cloneURL(); u != "https://c1@ghb.example/scm/demo.git" {
		t.Errorf("cloneURL = %q", u)
	}
	if u := (GitStore{Remote: "https://ghb.example/scm/demo.git", User: "c1", Password: "t"}).cloneURL(); u != "https://ghb.example/scm/demo.git" {
		t.Errorf("cloneURL with token = %q", u)
	}
	if u := (GitStore{Remote: "git@github.com:x/y.git", User: "c1"}).cloneURL(); u != "git@github.com:x/y.git" {
		t.Errorf("cloneURL ssh = %q", u)
	}
}

// The server's default branch (main) is not where the first device pushed
// Shellvane's folder: the others find it on its branch anyway.
func TestGitFindsTheDataOnAnotherBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ctx := context.Background()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	remote := filepath.Join(t.TempDir(), "team.git")
	run(".", "init", "--quiet", "--bare", remote)

	a := GitStore{Remote: remote, Dir: filepath.Join(t.TempDir(), "a")}
	if err := a.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	run(a.Dir, "checkout", "--quiet", "-b", "master")
	if err := a.Put(ctx, "team.json", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := a.EndExclusive(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	// The server's default branch is main, with only a README.
	other := filepath.Join(t.TempDir(), "other")
	run(".", "init", "--quiet", other)
	os.WriteFile(filepath.Join(other, "README"), []byte("hi"), 0o644)
	run(other, "add", "README")
	run(other, "commit", "--quiet", "-m", "readme")
	run(other, "push", "--quiet", remote, "HEAD:refs/heads/main")
	run(remote, "symbolic-ref", "HEAD", "refs/heads/main")

	b := GitStore{Remote: remote, Dir: filepath.Join(t.TempDir(), "b")}
	if err := b.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	if got, err := b.Get(ctx, "team.json"); err != nil || string(got) != "v1" {
		t.Fatalf("b reads %q, %v", got, err)
	}
	// b's changes go to that branch too, and a sees them.
	if err := b.Put(ctx, "team.json", []byte("v2")); err != nil {
		t.Fatal(err)
	}
	if err := b.EndExclusive(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	if err := a.ResetToRemote(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.Get(ctx, "team.json"); string(got) != "v2" {
		t.Fatalf("a reads %q", got)
	}
}
