package app

import (
	"net/url"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrDetachExpired: the window that should show the moved terminals came
// too late, or the token was used already.
const ErrDetachExpired errs.Code = "session.detachExpired"

// EventSessionReturn carries the token of a tab a detached window gives
// back to the main window.
const EventSessionReturn = "session:return"

// detachTimeout is how long moved sessions wait for their new window.
const detachTimeout = time.Minute

type detachedTab struct {
	payload  string
	sessions []string
}

// EnableDetach sets how a new window is opened (the Wails application in
// the app; a fake in tests). It is not a method so that Wails does not
// expose it as a binding.
func EnableDetach(s *SessionService, open func(url string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openWindow = open
}

// Detach moves sessions to a new window. payload is the frontend's
// description of the tab (layout and panes), handed back as is by
// TakeDetached; the sessions stay open while the windows swap.
func (s *SessionService) Detach(payload string, sessionIDs []string) error {
	token, err := s.stage(payload, sessionIDs)
	if err != nil {
		return err
	}
	s.mu.Lock()
	open := s.openWindow
	s.mu.Unlock()
	if open != nil {
		open("/?detached=" + url.QueryEscape(token))
	}
	return nil
}

// Reattach gives a detached window's tab back to the main window, which
// takes it with TakeDetached when the session:return event arrives.
func (s *SessionService) Reattach(payload string, sessionIDs []string) error {
	token, err := s.stage(payload, sessionIDs)
	if err != nil {
		return err
	}
	s.emit(EventSessionReturn, token)
	return nil
}

// stage keeps the sessions open for the window that takes them with the
// returned token.
func (s *SessionService) stage(payload string, sessionIDs []string) (string, error) {
	var moved []*session
	for _, id := range sessionIDs {
		sess, err := s.get(id)
		if err != nil {
			return "", err
		}
		moved = append(moved, sess)
	}
	for _, sess := range moved {
		sess.handover.Store(true)
	}
	token := uuid.NewString()
	s.mu.Lock()
	if s.detached == nil {
		s.detached = map[string]detachedTab{}
	}
	s.detached[token] = detachedTab{payload: payload, sessions: sessionIDs}
	s.mu.Unlock()

	// A window that never asks must not leave sessions running unseen.
	time.AfterFunc(detachTimeout, func() {
		s.mu.Lock()
		_, waiting := s.detached[token]
		delete(s.detached, token)
		s.mu.Unlock()
		if waiting {
			for _, id := range sessionIDs {
				s.Close(id)
			}
		}
	})
	return token, nil
}

// TakeDetached returns, once, the tab moved with token.
func (s *SessionService) TakeDetached(token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.detached[token]
	if !ok {
		return "", errs.New(ErrDetachExpired, nil)
	}
	delete(s.detached, token)
	return t.payload, nil
}
