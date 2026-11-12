package sync

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/netproxy"
	"github.com/csturiale/shellvane/core/shellenv"
)

// Error codes of the storage backends.
const (
	ErrWebDAV errs.Code = "sync.webdav"
	ErrGit    errs.Code = "sync.git"
	// ErrGitAuth: the repository refused the credentials, or asked for some.
	ErrGitAuth errs.Code = "sync.gitAuth"
	// ErrGitNetwork: the repository's server cannot be reached.
	ErrGitNetwork errs.Code = "sync.gitNetwork"
)

// Store is where the sync files live: a shared folder, a WebDAV server, a
// Git repository. It holds only encrypted files and sync.json.
type Store interface {
	// Get returns a file; a missing one is os.ErrNotExist.
	Get(ctx context.Context, name string) ([]byte, error)
	Put(ctx context.Context, name string, data []byte) error
	// List names the files.
	List(ctx context.Context) ([]string, error)
	// Where is shown to the user (a path or a URL, no credentials).
	Where() string
}

// Session is implemented by stores that must fetch before a sync and
// publish after it (Git).
type Session interface {
	Begin(ctx context.Context) error
	End(ctx context.Context, device string) error
}

// FolderStore keeps the files in <folder>/Shellvane-sync.
type FolderStore struct{ Folder string }

// Where is the shared folder.
func (f FolderStore) Where() string { return f.Folder }

func (f FolderStore) dir() string { return Dir(f.Folder) }

// Get reads a file.
func (f FolderStore) Get(_ context.Context, name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.Join(f.dir(), name))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, errs.Wrap(err, ErrFolder, errs.Params{"path": f.Folder})
	}
	return data, err
}

// Put writes a file atomically.
func (f FolderStore) Put(_ context.Context, name string, data []byte) error {
	if err := os.MkdirAll(f.dir(), 0o700); err != nil {
		return errs.Wrap(err, ErrFolder, errs.Params{"path": f.Folder})
	}
	p := filepath.Join(f.dir(), name)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return errs.Wrap(err, ErrFolder, errs.Params{"path": f.Folder})
	}
	if err := os.Rename(tmp, p); err != nil {
		return errs.Wrap(err, ErrFolder, errs.Params{"path": f.Folder})
	}
	return nil
}

// List names the files.
func (f FolderStore) List(context.Context) ([]string, error) {
	entries, err := os.ReadDir(f.dir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errs.Wrap(err, ErrFolder, errs.Params{"path": f.Folder})
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// WebDAVStore keeps the files in a collection of a WebDAV server
// (Nextcloud, ownCloud, a NAS, Apache mod_dav...).
type WebDAVStore struct {
	// URL is the collection, e.g. https://cloud.example.org/remote.php/dav/files/me/Shellvane-sync/
	URL      string
	User     string
	Password string
	Client   *http.Client
}

// Where is the URL without credentials.
func (w WebDAVStore) Where() string {
	if u, err := url.Parse(w.URL); err == nil {
		u.User = nil
		return u.String()
	}
	return w.URL
}

func (w WebDAVStore) client() *http.Client {
	if w.Client != nil {
		return w.Client
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (w WebDAVStore) base() string {
	if strings.HasSuffix(w.URL, "/") {
		return w.URL
	}
	return w.URL + "/"
}

func (w WebDAVStore) do(ctx context.Context, method, target string, body []byte, header map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, errs.Wrap(err, ErrWebDAV, errs.Params{"url": w.Where(), "status": ""})
	}
	if w.User != "" || w.Password != "" {
		req.SetBasicAuth(w.User, w.Password)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := w.client().Do(req)
	if err != nil {
		return nil, errs.Wrap(err, ErrWebDAV, errs.Params{"url": w.Where(), "status": ""})
	}
	return resp, nil
}

func (w WebDAVStore) fail(resp *http.Response) error {
	return errs.New(ErrWebDAV, errs.Params{"url": w.Where(), "status": resp.Status})
}

// Get downloads a file.
func (w WebDAVStore) Get(ctx context.Context, name string) ([]byte, error) {
	resp, err := w.do(ctx, http.MethodGet, w.base()+url.PathEscape(name), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}
	if resp.StatusCode/100 != 2 {
		return nil, w.fail(resp)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 256<<20))
}

// Put uploads a file, creating the collection the first time.
func (w WebDAVStore) Put(ctx context.Context, name string, data []byte) error {
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := w.do(ctx, http.MethodPut, w.base()+url.PathEscape(name), data, map[string]string{"Content-Type": "application/octet-stream"})
		if err != nil {
			return err
		}
		resp.Body.Close()
		switch {
		case resp.StatusCode/100 == 2:
			return nil
		case attempt == 0 && (resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusNotFound):
			mk, err := w.do(ctx, "MKCOL", w.base(), nil, nil)
			if err != nil {
				return err
			}
			mk.Body.Close()
		default:
			return w.fail(resp)
		}
	}
	return errs.New(ErrWebDAV, errs.Params{"url": w.Where(), "status": "MKCOL"})
}

// List names the files of the collection (PROPFIND, depth 1).
func (w WebDAVStore) List(ctx context.Context) ([]string, error) {
	body := []byte(`<?xml version="1.0" encoding="utf-8"?><d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`)
	resp, err := w.do(ctx, "PROPFIND", w.base(), body, map[string]string{"Depth": "1", "Content-Type": "application/xml"})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode/100 != 2 {
		return nil, w.fail(resp)
	}
	var ms struct {
		Responses []struct {
			Href       string `xml:"href"`
			Collection *struct {
			} `xml:"propstat>prop>resourcetype>collection"`
		} `xml:"response"`
	}
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&ms); err != nil {
		return nil, errs.Wrap(err, ErrWebDAV, errs.Params{"url": w.Where(), "status": resp.Status})
	}
	var out []string
	for _, r := range ms.Responses {
		if r.Collection != nil {
			continue
		}
		p := r.Href
		if u, err := url.Parse(p); err == nil {
			p = u.Path
		}
		if name, err := url.PathUnescape(path.Base(p)); err == nil && name != "" && name != "/" {
			out = append(out, name)
		}
	}
	return out, nil
}

// GitStore keeps the files in a Git repository, cloned under Dir; it uses
// the git command and the user's own credentials (SSH keys, credential
// helper). Each device commits only its own file, so pushes do not
// conflict.
type GitStore struct {
	Remote string // the repository URL
	Dir    string // the local clone
	// Git is the git command; "git" when empty.
	Git string
	// User and Password (a token) log in to an HTTPS remote; empty uses
	// the computer's own git credentials (SSH key, credential helper).
	User     string
	Password string
}

// ErrGitConflict: someone pushed meanwhile; the local commit was dropped
// and the clone matches the remote again, so the change can be redone.
const ErrGitConflict errs.Code = "sync.gitConflict"

// Where is the repository URL without credentials.
func (g GitStore) Where() string {
	if u, err := url.Parse(g.Remote); err == nil && u.User != nil {
		u.User = nil
		return u.String()
	}
	return g.Remote
}

func (g GitStore) folder() FolderStore { return FolderStore{Folder: g.Dir} }

// Run runs git in dir the way the store does: the user's credentials, the
// app's proxy, never a prompt; errors carry an auth or network code.
func (g GitStore) Run(ctx context.Context, dir string, args ...string) (string, error) {
	return g.git(ctx, dir, args...)
}

func (g GitStore) git(ctx context.Context, dir string, args ...string) (string, error) {
	bin := g.Git
	if bin == "" {
		bin = "git"
	}
	// Proxies and certificates may come from the login shell, read at start.
	shellenv.Wait(ctx)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	// Never wait for a password on a terminal nobody sees.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=", "SSH_ASKPASS=")
	// The proxy of Settings › Network (git reads only the environment).
	cmd.Env = append(cmd.Env, netproxy.ChildEnv()...)
	if g.Password != "" {
		// HTTPS credentials through the environment (git 2.31+), not the
		// command line, where other processes could read them.
		auth := base64.StdEncoding.EncodeToString([]byte(g.User + ":" + g.Password))
		// Scoped to the repository's URL: never sent to another server.
		cmd.Env = append(cmd.Env, "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=http."+g.Where()+".extraHeader", "GIT_CONFIG_VALUE_0=Authorization: Basic "+auth)
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	text := strings.TrimSpace(out.String())
	if err != nil {
		line := gitReason(text)
		return text, errs.Wrap(fmt.Errorf("git %s: %w: %s", args[0], err, text), gitCode(text), errs.Params{"detail": line, "url": g.Where()})
	}
	return text, nil
}

// cloneURL is the remote with the user name when one is given without a
// token, for the credential helper to find that user's password.
func (g GitStore) cloneURL() string {
	if g.User == "" || g.Password != "" {
		return g.Remote
	}
	u, err := url.Parse(g.Remote)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return g.Remote
	}
	u.User = url.User(g.User)
	return u.String()
}

// gitReason is the line of git's output that says what went wrong: the
// first "fatal:" or "error:" line, else the last one.
func gitReason(out string) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "fatal:") || strings.HasPrefix(l, "error:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(l, "fatal:"), "error:"))
		}
	}
	return strings.TrimSpace(lines[len(lines)-1])
}

// gitCode tells an authentication or network failure from other ones.
func gitCode(out string) errs.Code {
	o := strings.ToLower(out)
	for _, s := range []string{"authentication failed", "could not read username", "could not read password", "terminal prompts disabled", "permission denied (publickey", "invalid username or password", "returned error: 401", "returned error: 403"} {
		if strings.Contains(o, s) {
			return ErrGitAuth
		}
	}
	for _, s := range []string{"could not resolve host", "failed to connect", "couldn't connect to server", "connection timed out", "connection refused", "network is unreachable", "could not resolve proxy", "ssl certificate problem", "unable to get local issuer certificate"} {
		if strings.Contains(o, s) {
			return ErrGitNetwork
		}
	}
	return ErrGit
}

// Begin clones the repository the first time, then pulls.
func (g GitStore) Begin(ctx context.Context) error {
	if _, err := os.Stat(filepath.Join(g.Dir, ".git")); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(g.Dir), 0o700); err != nil {
			return errs.Wrap(err, ErrFolder, errs.Params{"path": g.Dir})
		}
		if _, err := g.git(ctx, filepath.Dir(g.Dir), "clone", "--quiet", "--", g.cloneURL(), g.Dir); err != nil {
			return err
		}
		return g.followData(ctx)
	}
	// An empty repository has nothing to pull yet.
	if _, err := g.git(ctx, g.Dir, "rev-parse", "--verify", "HEAD"); err != nil {
		return g.followData(ctx)
	}
	if _, err := g.git(ctx, g.Dir, "ls-remote", "--exit-code", "--heads", "origin"); err != nil {
		return nil // nothing pushed yet
	}
	if _, err := g.git(ctx, g.Dir, "pull", "--quiet", "--rebase"); err != nil {
		return err
	}
	return g.followData(ctx)
}

// followData moves the clone to the branch that holds Shellvane's folder
// when the one checked out does not. A clone takes the server's default
// branch (main, on many servers) while the first device may have pushed
// Shellvane's data to another one (git's own default, often master): the
// others would find nothing.
func (g GitStore) followData(ctx context.Context) error {
	if fi, err := os.Stat(filepath.Join(g.Dir, DirName)); err == nil && fi.IsDir() {
		return nil
	}
	refs, err := g.git(ctx, g.Dir, "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	if err != nil {
		return nil // nothing fetched: an empty repository
	}
	for _, ref := range strings.Fields(refs) {
		if ref == "origin/HEAD" || ref == "origin" {
			continue
		}
		if out, err := g.git(ctx, g.Dir, "ls-tree", "-d", "--name-only", ref, DirName); err != nil || strings.TrimSpace(out) == "" {
			continue
		}
		branch := strings.TrimPrefix(ref, "origin/")
		if _, err := g.git(ctx, g.Dir, "checkout", "--quiet", "-B", branch, ref); err != nil {
			return err
		}
		_, err := g.git(ctx, g.Dir, "branch", "--quiet", "--set-upstream-to", ref)
		return err
	}
	return nil
}

// End commits and pushes the device's changes.
func (g GitStore) End(ctx context.Context, device string) error {
	if _, err := g.git(ctx, g.Dir, "add", "-A", DirName); err != nil {
		return err
	}
	if out, err := g.git(ctx, g.Dir, "status", "--porcelain", DirName); err != nil || out == "" {
		return err
	}
	if _, err := g.git(ctx, g.Dir, "-c", "user.name=Shellvane", "-c", "user.email=sync@shellvane.invalid",
		"commit", "--quiet", "-m", "Shellvane sync: "+device); err != nil {
		return err
	}
	if _, err := g.git(ctx, g.Dir, "push", "--quiet", "-u", "origin", "HEAD"); err == nil {
		return nil
	}
	// Another device pushed meanwhile: its commit touches only its file.
	if _, err := g.git(ctx, g.Dir, "pull", "--quiet", "--rebase"); err != nil {
		return err
	}
	_, err := g.git(ctx, g.Dir, "push", "--quiet", "-u", "origin", "HEAD")
	return err
}

// EndExclusive commits and pushes a file several people write (a team
// file). When someone pushed meanwhile it does not merge: it drops the
// local commit, resets the clone to the remote and returns ErrGitConflict,
// for the caller to redo its change on the new version.
func (g GitStore) EndExclusive(ctx context.Context, device string) error {
	if _, err := g.git(ctx, g.Dir, "add", "-A", DirName); err != nil {
		return err
	}
	if out, err := g.git(ctx, g.Dir, "status", "--porcelain", DirName); err != nil || out == "" {
		return err
	}
	if _, err := g.git(ctx, g.Dir, "-c", "user.name=Shellvane", "-c", "user.email=sync@shellvane.invalid",
		"commit", "--quiet", "-m", "Shellvane team: "+device); err != nil {
		return err
	}
	if _, err := g.git(ctx, g.Dir, "push", "--quiet", "-u", "origin", "HEAD"); err == nil {
		return nil
	}
	if err := g.ResetToRemote(ctx); err != nil {
		return err
	}
	return errs.New(ErrGitConflict, nil)
}

// ResetToRemote makes the clone match the remote branch, dropping local
// commits and any interrupted rebase.
func (g GitStore) ResetToRemote(ctx context.Context) error {
	g.git(ctx, g.Dir, "rebase", "--abort") // left by an older version, if any
	if _, err := g.git(ctx, g.Dir, "fetch", "--quiet", "origin"); err != nil {
		return err
	}
	target := "@{u}"
	if _, err := g.git(ctx, g.Dir, "rev-parse", "--verify", "--quiet", target); err != nil {
		// No upstream yet (a first push that lost the race): use the
		// remote branch of the same name, if someone pushed it.
		branch, err := g.git(ctx, g.Dir, "symbolic-ref", "--short", "HEAD")
		if err != nil {
			return err
		}
		target = "origin/" + branch
		if _, err := g.git(ctx, g.Dir, "rev-parse", "--verify", "--quiet", target); err != nil {
			return g.followData(ctx) // nothing pushed on this branch: maybe on another
		}
		if _, err := g.git(ctx, g.Dir, "branch", "--quiet", "--set-upstream-to", target); err != nil {
			return err
		}
	}
	if _, err := g.git(ctx, g.Dir, "reset", "--quiet", "--hard", target); err != nil {
		return err
	}
	// A clone made by an older version may sit on a branch without the data.
	return g.followData(ctx)
}

// Get reads a file of the clone.
func (g GitStore) Get(ctx context.Context, name string) ([]byte, error) {
	return g.folder().Get(ctx, name)
}

// Put writes a file in the clone; End publishes it.
func (g GitStore) Put(ctx context.Context, name string, data []byte) error {
	return g.folder().Put(ctx, name, data)
}

// List names the files of the clone.
func (g GitStore) List(ctx context.Context) ([]string, error) { return g.folder().List(ctx) }
