// Package keychain keeps secrets in the system keychain: the macOS
// Keychain, the Windows Credential Manager, or the Secret Service (GNOME
// Keyring, KWallet) on Linux.
package keychain

import (
	"github.com/zalando/go-keyring"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrUnavailable is returned when the system keychain cannot be used (on
// Linux: no Secret Service running).
const ErrUnavailable errs.Code = "keychain.unavailable"

// service groups Shellvane's items in the keychain.
const service = "Shellvane"

// System is the system keychain.
type System struct{}

// Get reads a secret.
func (System) Get(account string) (string, error) {
	s, err := keyring.Get(service, account)
	if err != nil {
		return "", errs.Wrap(err, ErrUnavailable, nil)
	}
	return s, nil
}

// Set saves a secret, replacing the previous one.
func (System) Set(account, secret string) error {
	if err := keyring.Set(service, account, secret); err != nil {
		return errs.Wrap(err, ErrUnavailable, nil)
	}
	return nil
}

// Delete removes a secret; a missing one is not an error.
func (System) Delete(account string) error {
	if err := keyring.Delete(service, account); err != nil && err != keyring.ErrNotFound {
		return errs.Wrap(err, ErrUnavailable, nil)
	}
	return nil
}
