//go:build !windows && !(darwin && cgo)

package biometric

import (
	"context"

	"github.com/csturiale/shellvane/core/errs"
)

type system struct{}

func (system) Kind() string { return KindNone }

func (system) Available(context.Context) bool { return false }

func (system) Verify(context.Context, string) error { return errs.New(ErrUnavailable, nil) }
