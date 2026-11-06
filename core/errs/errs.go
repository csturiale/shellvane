// Package errs defines coded errors.
//
// The core never returns user-facing text. Every error that may reach the user
// carries a stable Code and optional Params; the UI translates the code into
// the active language. Error() returns an English message meant for logs only.
package errs

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Code is a stable, dot-separated identifier such as "ssh.hostKeyMismatch".
// Codes double as translation keys (prefixed with "errors.") in the UI.
type Code string

// Generic codes shared across packages.
const (
	Internal     Code = "internal"
	InvalidInput Code = "invalidInput"
	NotFound     Code = "notFound"
)

// Params holds the values interpolated into the translated message.
type Params map[string]any

// Error is an error with a stable code.
type Error struct {
	Code   Code
	Params Params
	Err    error // underlying cause, never shown to the user as-is
}

// New returns an error with the given code and params.
func New(code Code, params Params) *Error {
	return &Error{Code: code, Params: params}
}

// Wrap attaches a code to an underlying error.
func Wrap(err error, code Code, params Params) *Error {
	return &Error{Code: code, Params: params, Err: err}
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(string(e.Code))
	if len(e.Params) > 0 {
		keys := make([]string, 0, len(e.Params))
		for k := range e.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString(" (")
		for i, k := range keys {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s=%v", k, e.Params[k])
		}
		b.WriteString(")")
	}
	if e.Err != nil {
		b.WriteString(": ")
		b.WriteString(e.Err.Error())
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.Err }

// Is reports whether target is an *Error with the same code.
func (e *Error) Is(target error) bool {
	var t *Error
	if errors.As(target, &t) {
		return t.Code == e.Code
	}
	return false
}

// Payload is the JSON shape sent to the frontend.
type Payload struct {
	Code   Code   `json:"code"`
	Params Params `json:"params,omitempty"`
	Detail string `json:"detail,omitempty"` // English, for diagnostics panels
}

// CodeOf returns the code of the first *Error in err's chain, or Internal.
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return Internal
}

// ToPayload converts any error into a Payload. Errors without a code become
// Internal, keeping their message only as a diagnostic detail.
func ToPayload(err error) Payload {
	var e *Error
	if errors.As(err, &e) {
		p := Payload{Code: e.Code, Params: e.Params}
		if e.Err != nil {
			p.Detail = e.Err.Error()
		}
		return p
	}
	return Payload{Code: Internal, Detail: err.Error()}
}

// MarshalJSON encodes the error as a Payload, whoever marshals it.
func (e *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(ToPayload(e))
}

// Marshal lets errors cross the Wails bridge in a structured form.
func Marshal(err error) []byte {
	b, jsonErr := json.Marshal(ToPayload(err))
	if jsonErr != nil {
		return nil
	}
	return b
}
