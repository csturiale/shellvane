package vault

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"golang.org/x/crypto/chacha20poly1305"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes of the keychain unlock.
const (
	ErrNoDeviceUnlock errs.Code = "vault.noKeychainUnlock"
	// ErrDeviceSecretGone: the keychain no longer has the secret (another
	// computer, a reset keychain); the master password still works.
	ErrDeviceSecretGone errs.Code = "vault.keychainSecretGone"
)

// Format identifier from before the rename to Shellvane: never change it,
// or data written by earlier versions could no longer be read or verified.
const deviceLabel = "termora-device-v1"

// SecretStore keeps one secret per account outside the database: the
// system keychain in the app, a map in tests.
type SecretStore interface {
	Get(account string) (string, error)
	Set(account, secret string) error
	Delete(account string) error
}

// deviceAccount names this vault's secret in the keychain; it is chosen at
// random when the unlock is enabled, so it survives password changes.
func (v *Vault) deviceAccount(ctx context.Context) (string, error) {
	var account sql.NullString
	err := v.db.QueryRowContext(ctx, `SELECT device_account FROM vault_meta WHERE id = 1`).Scan(&account)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errs.New(ErrNotSetUp, nil)
	}
	if err != nil {
		return "", err
	}
	return account.String, nil
}

// EnableDeviceUnlock wraps the data key with a new random secret saved in
// store, replacing any previous one. The vault must be unlocked. The
// secret is 256 random bits, so no key derivation is needed.
func (v *Vault) EnableDeviceUnlock(ctx context.Context, store SecretStore) error {
	v.mu.RLock()
	dek := append([]byte(nil), v.dek...)
	v.mu.RUnlock()
	if len(dek) == 0 {
		return errs.New(ErrLocked, nil)
	}
	defer zero(dek)
	old, err := v.deviceAccount(ctx)
	if err != nil {
		return err
	}
	if old != "" {
		_ = store.Delete(old)
	}
	id := make([]byte, 8)
	if _, err := rand.Read(id); err != nil {
		return err
	}
	account := "vault-" + hex.EncodeToString(id)
	secret := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(secret); err != nil {
		return err
	}
	defer zero(secret)
	aead, err := chacha20poly1305.NewX(secret)
	if err != nil {
		return err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	wrapped := aead.Seal(nonce, nonce, dek, []byte(deviceLabel))
	if err := store.Set(account, base64.StdEncoding.EncodeToString(secret)); err != nil {
		return err
	}
	_, err = v.db.ExecContext(ctx, `UPDATE vault_meta SET device_wrapped = ?, device_account = ? WHERE id = 1`, wrapped, account)
	return err
}

// HasDeviceUnlock reports whether the keychain unlock is set up.
func (v *Vault) HasDeviceUnlock(ctx context.Context) (bool, error) {
	var wrapped []byte
	err := v.db.QueryRowContext(ctx, `SELECT device_wrapped FROM vault_meta WHERE id = 1`).Scan(&wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return len(wrapped) > 0, err
}

// DisableDeviceUnlock forgets the wrapped key and the keychain secret.
func (v *Vault) DisableDeviceUnlock(ctx context.Context, store SecretStore) error {
	if account, err := v.deviceAccount(ctx); err == nil && account != "" {
		_ = store.Delete(account) // already gone is fine
	}
	_, err := v.db.ExecContext(ctx, `UPDATE vault_meta SET device_wrapped = NULL, device_account = NULL WHERE id = 1`)
	return err
}

// DeviceSecretPresent tells, without unlocking, whether store still has
// the secret of the device unlock: nil when it has, ErrDeviceSecretGone
// when the keychain answers that it is missing, another error when the
// keychain cannot be read now.
func (v *Vault) DeviceSecretPresent(ctx context.Context, store SecretStore) error {
	account, err := v.deviceAccount(ctx)
	if err != nil {
		return err
	}
	if _, err := store.Get(account); err != nil {
		if errs.CodeOf(err) == "keychain.notFound" {
			return errs.New(ErrDeviceSecretGone, nil)
		}
		return err
	}
	return nil
}

// UnlockWithDevice opens the vault with the secret in store.
func (v *Vault) UnlockWithDevice(ctx context.Context, store SecretStore) error {
	var wrapped []byte
	err := v.db.QueryRowContext(ctx, `SELECT device_wrapped FROM vault_meta WHERE id = 1`).Scan(&wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.New(ErrNotSetUp, nil)
	}
	if err != nil {
		return err
	}
	if len(wrapped) == 0 {
		return errs.New(ErrNoDeviceUnlock, nil)
	}
	account, err := v.deviceAccount(ctx)
	if err != nil {
		return err
	}
	encoded, err := store.Get(account)
	if err != nil {
		return errs.Wrap(err, ErrDeviceSecretGone, nil)
	}
	secret, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(secret) != chacha20poly1305.KeySize {
		return errs.New(ErrDeviceSecretGone, nil)
	}
	defer zero(secret)
	aead, err := chacha20poly1305.NewX(secret)
	if err != nil {
		return err
	}
	if len(wrapped) < aead.NonceSize() {
		return errs.New(ErrCorrupt, nil)
	}
	dek, err := aead.Open(nil, wrapped[:aead.NonceSize()], wrapped[aead.NonceSize():], []byte(deviceLabel))
	if err != nil {
		return errs.New(ErrDeviceSecretGone, nil)
	}
	v.mu.Lock()
	zero(v.dek)
	v.dek = dek
	v.mu.Unlock()
	return nil
}

// SetDeviceBiometric makes the keychain unlock ask the system to confirm
// the user (Touch ID, Windows Hello) first. The caller verifies.
func (v *Vault) SetDeviceBiometric(ctx context.Context, on bool) error {
	val := 0
	if on {
		val = 1
	}
	_, err := v.db.ExecContext(ctx, `UPDATE vault_meta SET device_biometric = ? WHERE id = 1`, val)
	return err
}

// DeviceBiometric reports whether the keychain unlock asks for Touch ID
// or Windows Hello.
func (v *Vault) DeviceBiometric(ctx context.Context) (bool, error) {
	var on int
	err := v.db.QueryRowContext(ctx, `SELECT device_biometric FROM vault_meta WHERE id = 1`).Scan(&on)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return on == 1, err
}
