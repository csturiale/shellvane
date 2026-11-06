package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/files"
)

// Events of the files service.
const (
	EventTransfer     = "transfer:progress"
	EventFilesDropped = "files:dropped"
)

// LocalFS is the ID of the local file system.
const LocalFS = "local"

// Error codes of the files service.
const (
	// ErrFSNotFound reports an unknown or closed file system ID.
	ErrFSNotFound errs.Code = "files.fsNotFound"
	// ErrSFTPUnavailable means the server refused the SFTP subsystem.
	ErrSFTPUnavailable errs.Code = "files.sftpUnavailable"
	// ErrInvalidMode rejects permission values above 0o7777.
	ErrInvalidMode errs.Code = "files.invalidMode"
)

// maxParallelTransfers bounds concurrent transfers; the rest wait queued.
const maxParallelTransfers = 3

// RemoteFSInfo describes an open remote file system.
type RemoteFSInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Home  string `json:"home"`
	// Shell means the server has no SFTP: files go through shell commands.
	Shell bool `json:"shell"`
}

// Transfer states.
const (
	TransferQueued    = "queued"
	TransferRunning   = "running"
	TransferDone      = "done"
	TransferFailed    = "failed"
	TransferCancelled = "cancelled"
)

// TransferStatus is emitted while a transfer progresses.
type TransferStatus struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	From    string        `json:"from"` // file system IDs
	To      string        `json:"to"`
	Dest    string        `json:"dest"`
	Done    int64         `json:"done"`
	Total   int64         `json:"total"`
	Current string        `json:"current"`
	State   string        `json:"state"`
	Error   *errs.Payload `json:"error,omitempty"`
}

// DroppedFiles is emitted when files are dropped on an element marked with
// data-file-drop-target.
type DroppedFiles struct {
	Files  []string `json:"files"`
	Target string   `json:"target"`
}

// FilesService browses local and SFTP file systems and moves files between them.
type FilesService struct {
	sessions *SessionService
	emit     Emitter
	slots    chan struct{}

	mu        sync.Mutex
	remotes   map[string]*remoteFS
	transfers map[string]*transferJob
	edits     edits
}

type remoteFS struct {
	fs      files.FS
	close   func() error // the SFTP session; nothing for the shell fallback
	release func()
	title   string
}

type transferJob struct {
	cancel context.CancelFunc
	status TransferStatus
}

// NewFilesService returns a service using sessions' connection pool.
func NewFilesService(sessions *SessionService, emit Emitter) *FilesService {
	return &FilesService{
		sessions:  sessions,
		emit:      emit,
		slots:     make(chan struct{}, maxParallelTransfers),
		remotes:   map[string]*remoteFS{},
		transfers: map[string]*transferJob{},
	}
}

// Connect opens an SFTP session on the host's (shared) SSH connection.
func (s *FilesService) Connect(ctx context.Context, hostID string) (RemoteFSInfo, error) {
	ht, err := s.sessions.hostTarget(ctx, hostID)
	if err != nil {
		return RemoteFSInfo{}, err
	}
	id := uuid.NewString()
	client, release, err := s.sessions.connect(ctx, ht, "sftp:"+id)
	if err != nil {
		return RemoteFSInfo{}, err
	}
	var fsys files.FS
	var closeFS func() error
	shell := false
	sc, err := sftp.NewClient(client, sftp.UseConcurrentWrites(true), sftp.UseConcurrentReads(true))
	if err == nil {
		r := files.NewRemote(sc)
		fsys, closeFS = r, r.Close
	} else {
		// No SFTP subsystem: shell commands, like scp.
		sh := files.NewShellFS(sshRunner{client})
		if perr := sh.Probe(); perr != nil {
			release()
			return RemoteFSInfo{}, errs.Wrap(err, ErrSFTPUnavailable, errs.Params{"host": ht.name})
		}
		fsys, closeFS, shell = sh, func() error { return nil }, true
	}
	home, err := fsys.Home()
	if err != nil {
		closeFS()
		release()
		return RemoteFSInfo{}, err
	}
	s.mu.Lock()
	s.remotes[id] = &remoteFS{fs: fsys, close: closeFS, release: release, title: ht.name}
	s.mu.Unlock()
	return RemoteFSInfo{ID: id, Title: ht.name, Home: home, Shell: shell}, nil
}

// Disconnect closes an SFTP session.
func (s *FilesService) Disconnect(id string) {
	s.mu.Lock()
	r, ok := s.remotes[id]
	delete(s.remotes, id)
	s.mu.Unlock()
	if ok {
		r.close()
		r.release()
	}
}

// Home returns the starting directory.
func (s *FilesService) Home(fsID string) (string, error) {
	fsys, err := s.fs(fsID)
	if err != nil {
		return "", err
	}
	return fsys.Home()
}

// List returns a directory's entries, directories first.
func (s *FilesService) List(fsID, dir string) ([]files.Entry, error) {
	fsys, err := s.fs(fsID)
	if err != nil {
		return nil, err
	}
	return fsys.List(dir)
}

// Parent returns the directory containing path.
func (s *FilesService) Parent(fsID, path string) (string, error) {
	fsys, err := s.fs(fsID)
	if err != nil {
		return "", err
	}
	return fsys.Dir(path), nil
}

// Join builds a path inside dir with the file system's separator.
func (s *FilesService) Join(fsID, dir, name string) (string, error) {
	fsys, err := s.fs(fsID)
	if err != nil {
		return "", err
	}
	return fsys.Join(dir, name), nil
}

// Mkdir creates a directory.
func (s *FilesService) Mkdir(fsID, path string) error {
	fsys, err := s.fs(fsID)
	if err != nil {
		return err
	}
	return fsys.Mkdir(path)
}

// Rename moves a file or directory; it never overwrites.
func (s *FilesService) Rename(fsID, oldPath, newPath string) error {
	fsys, err := s.fs(fsID)
	if err != nil {
		return err
	}
	return fsys.Rename(oldPath, newPath)
}

// Remove deletes a file or a directory with its contents.
func (s *FilesService) Remove(fsID, path string) error {
	fsys, err := s.fs(fsID)
	if err != nil {
		return err
	}
	return fsys.Remove(path)
}

// Chmod sets permission bits (e.g. 0o755); recursive applies them to a
// directory's whole content.
func (s *FilesService) Chmod(fsID, path string, mode uint32, recursive bool) error {
	if mode > 0o7777 {
		return errs.New(ErrInvalidMode, errs.Params{"mode": mode})
	}
	fsys, err := s.fs(fsID)
	if err != nil {
		return err
	}
	return files.ChmodAll(fsys, path, mode, recursive)
}

// Chown sets the numeric owner and group.
func (s *FilesService) Chown(fsID, path string, uid, gid int) error {
	fsys, err := s.fs(fsID)
	if err != nil {
		return err
	}
	return fsys.Chown(path, uid, gid)
}

// Symlink creates link pointing to target.
func (s *FilesService) Symlink(fsID, target, link string) error {
	fsys, err := s.fs(fsID)
	if err != nil {
		return err
	}
	return fsys.Symlink(target, link)
}

// Transfer copies src from one file system into dstDir of another, which
// may both be remote: data then flows through this computer, in memory. It
// returns at once; progress arrives as events.
func (s *FilesService) Transfer(fromID, src, toID, dstDir string, opts files.Options) (string, error) {
	from, err := s.fs(fromID)
	if err != nil {
		return "", err
	}
	to, err := s.fs(toID)
	if err != nil {
		return "", err
	}
	entry, err := from.Stat(src)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &transferJob{cancel: cancel, status: TransferStatus{
		ID: uuid.NewString(), Name: entry.Name, From: fromID, To: toID, Dest: dstDir, State: TransferQueued,
	}}
	s.mu.Lock()
	s.transfers[job.status.ID] = job
	s.mu.Unlock()
	s.publish(job, nil)

	go func() {
		select {
		case s.slots <- struct{}{}:
		case <-ctx.Done():
			s.finish(job, ctx.Err())
			return
		}
		defer func() { <-s.slots }()
		s.publish(job, func(st *TransferStatus) { st.State = TransferRunning })
		var last time.Time
		err := files.CopyWith(ctx, from, src, to, dstDir, opts, func(done, total int64, current string) {
			if time.Since(last) < 100*time.Millisecond && done < total {
				return
			}
			last = time.Now()
			s.publish(job, func(st *TransferStatus) { st.Done, st.Total, st.Current = done, total, current })
		})
		s.finish(job, err)
	}()
	return job.status.ID, nil
}

// CancelTransfer stops a queued or running transfer.
func (s *FilesService) CancelTransfer(id string) {
	s.mu.Lock()
	job, ok := s.transfers[id]
	s.mu.Unlock()
	if ok {
		job.cancel()
	}
}

// Transfers lists the transfers of this session, oldest first.
func (s *FilesService) Transfers() []TransferStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TransferStatus, 0, len(s.transfers))
	for _, j := range s.transfers {
		out = append(out, j.status)
	}
	return out
}

// ClearFinished forgets transfers that are no longer running.
func (s *FilesService) ClearFinished() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, j := range s.transfers {
		if st := j.status.State; st == TransferDone || st == TransferFailed || st == TransferCancelled {
			delete(s.transfers, id)
		}
	}
}

func (s *FilesService) finish(job *transferJob, err error) {
	s.publish(job, func(st *TransferStatus) {
		switch {
		case err == nil:
			st.State, st.Done = TransferDone, st.Total
		case errs.CodeOf(err) == files.ErrCancelled || err == context.Canceled:
			st.State = TransferCancelled
		default:
			p := errs.ToPayload(err)
			st.State, st.Error = TransferFailed, &p
		}
	})
}

func (s *FilesService) publish(job *transferJob, change func(*TransferStatus)) {
	s.mu.Lock()
	if change != nil {
		change(&job.status)
	}
	st := job.status
	s.mu.Unlock()
	s.emit(EventTransfer, st)
}

func (s *FilesService) fs(id string) (files.FS, error) {
	if id == LocalFS {
		return files.Local{}, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.remotes[id]; ok {
		return r.fs, nil
	}
	return nil, errs.New(ErrFSNotFound, errs.Params{"id": id})
}

// Preview returns the beginning of a file: text, an image, or "binary".
func (s *FilesService) Preview(fsID, path string) (files.Preview, error) {
	fsys, err := s.fs(fsID)
	if err != nil {
		return files.Preview{}, err
	}
	return files.MakePreview(fsys, path)
}

// Compare lists how the folder dstDir differs from srcDir, recursively.
func (s *FilesService) Compare(ctx context.Context, fromID, srcDir, toID, dstDir string) ([]files.Diff, error) {
	from, err := s.fs(fromID)
	if err != nil {
		return nil, err
	}
	to, err := s.fs(toID)
	if err != nil {
		return nil, err
	}
	out, err := files.Compare(ctx, from, srcDir, to, dstDir)
	if out == nil {
		out = []files.Diff{}
	}
	return out, err
}

// SyncFolders applies differences found by Compare: new and newer files
// and folders are copied from srcDir to dstDir (as transfers), and, when
// deleteExtra, what exists only in dstDir is removed. It returns how
// many transfers started.
func (s *FilesService) SyncFolders(fromID, srcDir, toID, dstDir string, diffs []files.Diff, deleteExtra bool) (int, error) {
	from, err := s.fs(fromID)
	if err != nil {
		return 0, err
	}
	to, err := s.fs(toID)
	if err != nil {
		return 0, err
	}
	path := func(fsys files.FS, base, rel string) string {
		return fsys.Join(append([]string{base}, strings.Split(rel, "/")...)...)
	}
	started := 0
	for _, d := range diffs {
		switch d.Kind {
		case files.DiffNew, files.DiffNewer:
			dst := path(to, dstDir, d.Rel)
			if _, err := s.Transfer(fromID, path(from, srcDir, d.Rel), toID, to.Dir(dst), files.Options{Conflict: files.ConflictOverwrite}); err != nil {
				return started, err
			}
			started++
		case files.DiffExtra:
			if deleteExtra {
				if err := to.Remove(path(to, dstDir, d.Rel)); err != nil {
					return started, err
				}
			}
		}
	}
	return started, nil
}
