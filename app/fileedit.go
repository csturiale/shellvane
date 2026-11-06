package app

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/files"
)

// EventFileEdit reports what happened to a file edited on this computer.
const EventFileEdit = "files:edit"

// editPoll is how often edited files are checked for saves.
const editPoll = time.Second

// FileEdit is a remote file opened in a local editor.
type FileEdit struct {
	ID        string `json:"id"`
	FSID      string `json:"fsId"`
	Path      string `json:"path"` // on the server
	Name      string `json:"name"`
	LocalPath string `json:"localPath"`
	// State is "editing", "uploaded", "failed" or "stopped".
	State    string        `json:"state"`
	Uploaded string        `json:"uploaded"` // RFC 3339, last upload
	Error    *errs.Payload `json:"error,omitempty"`
}

type editJob struct {
	FileEdit
	cancel context.CancelFunc
}

type edits struct {
	mu   sync.Mutex
	jobs map[string]*editJob
}

// ErrEditRunnable: the file would be run, not opened in an editor.
const ErrEditRunnable errs.Code = "files.editRunnable"

// runnable are the extensions the system runs instead of opening in an
// editor: a remote file with one of them is never opened by Edit.
var runnable = map[string]bool{
	".app": true, ".command": true, ".tool": true, ".terminal": true, ".workflow": true, ".scpt": true, ".applescript": true, ".pkg": true, ".dmg": true, ".jar": true,
	".exe": true, ".bat": true, ".cmd": true, ".com": true, ".scr": true, ".msi": true, ".msc": true, ".ps1": true, ".vbs": true, ".vbe": true, ".js": true, ".jse": true,
	".wsf": true, ".wsh": true, ".hta": true, ".lnk": true, ".pif": true, ".cpl": true, ".reg": true, ".url": true, ".desktop": true, ".appimage": true, ".run": true,
	".webloc": true, ".inetloc": true, ".fileloc": true, ".library-ms": true, ".settingcontent-ms": true,
}

// Edit downloads a file, opens it with the system's default application
// and uploads it again every time it is saved, until StopEdit.
func (s *FilesService) Edit(fsID, path string) (FileEdit, error) {
	fsys, err := s.fs(fsID)
	if err != nil {
		return FileEdit{}, err
	}
	name := filepath.Base(filepath.FromSlash(path))
	if runnable[strings.ToLower(filepath.Ext(name))] || strings.HasSuffix(strings.ToLower(name), ".app/") {
		return FileEdit{}, errs.New(ErrEditRunnable, errs.Params{"name": name})
	}
	dir, err := os.MkdirTemp("", "shellvane-edit-")
	if err != nil {
		return FileEdit{}, errs.Wrap(err, files.ErrFailed, errs.Params{"path": path})
	}
	local := filepath.Join(dir, filepath.Base(filepath.FromSlash(path)))
	if err := download(fsys, path, local); err != nil {
		os.RemoveAll(dir)
		return FileEdit{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &editJob{FileEdit: FileEdit{ID: uuid.NewString(), FSID: fsID, Path: path, Name: filepath.Base(local), LocalPath: local, State: "editing"}, cancel: cancel}
	s.edits.mu.Lock()
	if s.edits.jobs == nil {
		s.edits.jobs = map[string]*editJob{}
	}
	s.edits.jobs[job.ID] = job
	s.edits.mu.Unlock()

	if app := application.Get(); app != nil {
		if err := app.Browser.OpenFile(local); err != nil {
			log.Printf("edit %s: open: %v", local, err)
		}
	}
	go s.watchEdit(ctx, job, fsys)
	s.emit(EventFileEdit, job.FileEdit)
	return job.FileEdit, nil
}

// StopEdit stops uploading saves and deletes the local copy.
func (s *FilesService) StopEdit(id string) {
	s.edits.mu.Lock()
	job, ok := s.edits.jobs[id]
	delete(s.edits.jobs, id)
	s.edits.mu.Unlock()
	if ok {
		job.cancel()
	}
}

// Edits lists the files being edited.
func (s *FilesService) Edits() []FileEdit {
	s.edits.mu.Lock()
	defer s.edits.mu.Unlock()
	out := []FileEdit{}
	for _, j := range s.edits.jobs {
		out = append(out, j.FileEdit)
	}
	return out
}

func (s *FilesService) watchEdit(ctx context.Context, job *editJob, fsys files.FS) {
	defer os.RemoveAll(filepath.Dir(job.LocalPath))
	last := modTime(job.LocalPath)
	tick := time.NewTicker(editPoll)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			job.State = "stopped"
			s.emit(EventFileEdit, job.FileEdit)
			return
		case <-tick.C:
		}
		mt := modTime(job.LocalPath)
		if mt.IsZero() || !mt.After(last) {
			continue
		}
		last = mt
		// Editors often write in several steps: wait for the file to settle.
		time.Sleep(200 * time.Millisecond)
		if err := upload(job.LocalPath, fsys, job.Path); err != nil {
			p := errs.ToPayload(err)
			job.State, job.Error = "failed", &p
		} else {
			job.State, job.Error, job.Uploaded = "uploaded", nil, time.Now().UTC().Format(time.RFC3339)
		}
		s.emit(EventFileEdit, job.FileEdit)
	}
}

func modTime(p string) time.Time {
	fi, err := os.Stat(p)
	if err != nil {
		return time.Time{}
	}
	return fi.ModTime()
}

func download(fsys files.FS, remote, local string) error {
	r, err := fsys.Open(remote)
	if err != nil {
		return err
	}
	defer r.Close()
	f, err := os.OpenFile(local, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return errs.Wrap(err, files.ErrFailed, errs.Params{"path": local})
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return errs.Wrap(err, files.ErrFailed, errs.Params{"path": remote})
	}
	return f.Close()
}

func upload(local string, fsys files.FS, remote string) error {
	f, err := os.Open(local)
	if err != nil {
		return errs.Wrap(err, files.ErrFailed, errs.Params{"path": local})
	}
	defer f.Close()
	w, err := fsys.Create(remote)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, f); err != nil {
		w.Close()
		return errs.Wrap(err, files.ErrFailed, errs.Params{"path": remote})
	}
	return w.Close()
}

// RevealLocal shows a local file or folder in the system's file manager.
func (s *FilesService) RevealLocal(path string) error {
	app := application.Get()
	if app == nil {
		return errs.New(files.ErrFailed, errs.Params{"path": path})
	}
	fi, err := os.Stat(path)
	if err != nil {
		return errs.New(files.ErrNotFound, errs.Params{"path": path})
	}
	return app.Env.OpenFileManager(path, !fi.IsDir())
}
