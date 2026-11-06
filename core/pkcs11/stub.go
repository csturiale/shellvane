//go:build !cgo

package pkcs11

import (
	"crypto"
	"io"

	"github.com/csturiale/shellvane/core/errs"
)

// Supported is false: this build cannot load PKCS#11 modules (no cgo).
const Supported = false

// Tokens is not available in this build.
func Tokens(string) ([]Token, error) { return nil, errs.New(ErrUnsupported, nil) }

// Keys is not available in this build.
func Keys(string, string) ([]Key, error) { return nil, errs.New(ErrUnsupported, nil) }

// Signer is not available in this build.
type Signer struct{}

// NewSigner is not available in this build.
func NewSigner(string, string, string, string, func(int) (string, error)) (*Signer, error) {
	return nil, errs.New(ErrUnsupported, nil)
}

// Public is never called.
func (*Signer) Public() crypto.PublicKey { return nil }

// Sign is never called.
func (*Signer) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return nil, errs.New(ErrUnsupported, nil)
}

// Close does nothing.
func (*Signer) Close() {}
