package app

import (
	"context"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/procedures"
)

// EventProcedureReceived is emitted when a page arrives from the browser
// ("Send to Shellvane"): its path in the inbox.
const EventProcedureReceived = "procedures:received"

// ProcedureService reads Markdown procedures from Git repositories and
// folders; the frontend shows them beside the terminals.
type ProcedureService struct {
	lib  procedures.Library
	emit Emitter

	mu       sync.Mutex
	received string // the last page received, until the UI takes it
}

// NewProcedureService keeps repository clones under dataDir/procedures.
func NewProcedureService(dataDir string, emit Emitter) *ProcedureService {
	return &ProcedureService{lib: procedures.Library{Dir: filepath.Join(dataDir, "procedures")}, emit: emit}
}

// ProcedureList is a source's procedures and when it was last updated.
type ProcedureList struct {
	Docs []procedures.Doc `json:"docs"`
	// Synced is RFC 3339, "" when a repository was never cloned.
	Synced string `json:"synced"`
}

// List returns the procedures of a source, cloning a repository the first
// time.
func (s *ProcedureService) List(ctx context.Context, src procedures.Source) (ProcedureList, error) {
	if s.lib.Synced(src).IsZero() {
		if err := s.lib.Sync(ctx, src); err != nil {
			return ProcedureList{}, err
		}
	}
	return s.list(src)
}

// Sync brings a repository up to date and lists it again.
func (s *ProcedureService) Sync(ctx context.Context, src procedures.Source) (ProcedureList, error) {
	if err := s.lib.Sync(ctx, src); err != nil {
		return ProcedureList{}, err
	}
	return s.list(src)
}

func (s *ProcedureService) list(src procedures.Source) (ProcedureList, error) {
	docs, err := s.lib.List(src)
	if err != nil {
		return ProcedureList{}, err
	}
	out := ProcedureList{Docs: docs}
	if t := s.lib.Synced(src); !t.IsZero() {
		out.Synced = t.UTC().Format(time.RFC3339)
	}
	return out, nil
}

// Read returns a procedure's Markdown.
func (s *ProcedureService) Read(src procedures.Source, path string) (string, error) {
	return s.lib.Read(src, path)
}

// Inbox is the source of the pages received from the browser or pasted.
func (s *ProcedureService) Inbox() procedures.Source {
	return s.lib.Inbox()
}

// SaveInbox keeps a page (pasted in the panel) as a procedure.
func (s *ProcedureService) SaveInbox(title, source, markdown string) (procedures.Doc, error) {
	return s.lib.SaveInbox(title, source, markdown)
}

// DeleteInbox removes a received page.
func (s *ProcedureService) DeleteInbox(path string) error {
	return s.lib.DeleteInbox(path)
}

// TakeReceived returns the page received before the UI listened (a link
// that launched the app), once.
func (s *ProcedureService) TakeReceived() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.received
	s.received = ""
	return p
}

// ReceiveProcedureLink keeps the page of a shellvane://procedure link in
// the inbox and tells the UI. It is a function, not a method, so that it
// is not a binding: only links opened through the system reach it.
func ReceiveProcedureLink(s *ProcedureService, raw string) {
	page, err := procedures.ParseLink(raw, func() (string, bool) {
		if app := application.Get(); app != nil && app.Clipboard != nil {
			return app.Clipboard.Text()
		}
		return "", false
	})
	if err != nil {
		log.Printf("procedure link: %v", err)
		return
	}
	doc, err := s.lib.SaveInbox(page.Title, page.Source, page.Markdown)
	if err != nil {
		log.Printf("procedure link: %v", err)
		return
	}
	s.mu.Lock()
	s.received = doc.Path
	s.mu.Unlock()
	if s.emit != nil {
		s.emit(EventProcedureReceived, doc.Path)
	}
}
