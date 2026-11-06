package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	gosync "sync"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes of files received in a terminal (ZMODEM sz).
const (
	ErrDownload        errs.Code = "downloads.failed"
	ErrDownloadUnknown errs.Code = "downloads.unknown"
)

// DownloadsService saves files a terminal receives (ZMODEM) in the
// Downloads folder. The frontend sends them in chunks: a binding call
// carries base64, so one huge call would hold the whole file several times.
type DownloadsService struct {
	dir string // empty: ~/Downloads

	mu    gosync.Mutex
	files map[string]*receivedFile
}

type receivedFile struct {
	f    *os.File
	path string
}

// NewDownloadsService saves into dir, or ~/Downloads when dir is empty.
func NewDownloadsService(dir string) *DownloadsService {
	return &DownloadsService{dir: dir, files: map[string]*receivedFile{}}
}

func (s *DownloadsService) folder() (string, error) {
	if s.dir != "" {
		return s.dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Downloads"), nil
}

// safeName keeps only the file name a server proposed: no folders, no
// path tricks, no control characters.
func safeName(name string) string {
	name = strings.ReplaceAll(name, `\`, "/")
	name = filepath.Base(strings.TrimRight(name, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || strings.ContainsRune(`<>:"|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "download"
	}
	return name
}

// Begin creates the file for name, never replacing an existing one
// ("name (1).ext" then), and returns its id.
func (s *DownloadsService) Begin(name string) (string, error) {
	dir, err := s.folder()
	if err != nil {
		return "", errs.Wrap(err, ErrDownload, errs.Params{"name": name})
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", errs.Wrap(err, ErrDownload, errs.Params{"name": name})
	}
	base := safeName(name)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 0; i < 1000; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		path := filepath.Join(dir, candidate)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", errs.Wrap(err, ErrDownload, errs.Params{"name": name})
		}
		id := uuid.NewString()
		s.mu.Lock()
		s.files[id] = &receivedFile{f: f, path: path}
		s.mu.Unlock()
		return id, nil
	}
	return "", errs.New(ErrDownload, errs.Params{"name": name})
}

func (s *DownloadsService) get(id string) (*receivedFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.files[id]
	if !ok {
		return nil, errs.New(ErrDownloadUnknown, nil)
	}
	return d, nil
}

// Append adds a chunk to the file.
func (s *DownloadsService) Append(id string, data []byte) error {
	d, err := s.get(id)
	if err != nil {
		return err
	}
	if _, err := d.f.Write(data); err != nil {
		return errs.Wrap(err, ErrDownload, errs.Params{"name": filepath.Base(d.path)})
	}
	return nil
}

// Finish closes the file and returns where it is.
func (s *DownloadsService) Finish(id string) (string, error) {
	d, err := s.get(id)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	delete(s.files, id)
	s.mu.Unlock()
	if err := d.f.Close(); err != nil {
		return "", errs.Wrap(err, ErrDownload, errs.Params{"name": filepath.Base(d.path)})
	}
	return d.path, nil
}

// Cancel deletes a file that was not received completely.
func (s *DownloadsService) Cancel(id string) {
	d, err := s.get(id)
	if err != nil {
		return
	}
	s.mu.Lock()
	delete(s.files, id)
	s.mu.Unlock()
	d.f.Close()
	os.Remove(d.path)
}
