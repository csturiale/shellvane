// Package biometric asks the operating system to confirm that the user is
// present: Touch ID (or the Mac's password) on macOS, Windows Hello (face,
// fingerprint or PIN) on Windows. Other systems report it as unavailable.
package biometric

import (
	"context"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes.
const (
	ErrUnavailable errs.Code = "biometric.unavailable"
	ErrCancelled   errs.Code = "biometric.cancelled"
	ErrFailed      errs.Code = "biometric.failed"
)

// Kinds of verification, for the UI.
const (
	KindNone    = ""
	KindTouchID = "touchid"
	KindHello   = "hello"
)

// Verifier confirms the user's presence. reason is shown by the system's
// dialog (already translated).
type Verifier interface {
	Kind() string
	Available(ctx context.Context) bool
	Verify(ctx context.Context, reason string) error
}

// System is this computer's verifier.
func System() Verifier { return system{} }
