package vault

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes of the recovery key.
const (
	ErrNoRecoveryKey    errs.Code = "vault.noRecoveryKey"
	ErrWrongRecoveryKey errs.Code = "vault.wrongRecoveryKey"
)

// Format identifier from before the rename to Shellvane: never change it,
// or data written by earlier versions could no longer be read or verified.
const recoveryLabel = "termora-recovery-v1"

var recoveryEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// CreateRecoveryKey makes a new recovery key (replacing any previous one)
// and returns it, formatted in groups: XXXX-XXXX-... It is shown once and
// never stored in clear. The vault must be unlocked.
func (v *Vault) CreateRecoveryKey(ctx context.Context) (string, error) {
	v.mu.RLock()
	dek := append([]byte(nil), v.dek...)
	v.mu.RUnlock()
	if len(dek) == 0 {
		return "", errs.New(ErrLocked, nil)
	}
	defer zero(dek)
	raw := make([]byte, 20) // 160 bits
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := recoveryEncoding.EncodeToString(raw)
	salt, wrapped, err := sealDEK([]byte(code), dek, v.kdf, recoveryLabel)
	if err != nil {
		return "", err
	}
	_, err = v.db.ExecContext(ctx, `UPDATE vault_meta SET recovery_salt = ?, recovery_time = ?, recovery_memory = ?, recovery_threads = ?, recovery_wrapped = ? WHERE id = 1`,
		salt, v.kdf.Time, v.kdf.Memory, v.kdf.Threads, wrapped)
	if err != nil {
		return "", err
	}
	var groups []string
	for i := 0; i < len(code); i += 4 {
		groups = append(groups, code[i:min(i+4, len(code))])
	}
	return strings.Join(groups, "-"), nil
}

// HasRecoveryKey reports whether a recovery key exists.
func (v *Vault) HasRecoveryKey(ctx context.Context) (bool, error) {
	var wrapped []byte
	err := v.db.QueryRowContext(ctx, `SELECT recovery_wrapped FROM vault_meta WHERE id = 1`).Scan(&wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return len(wrapped) > 0, err
}

// RemoveRecoveryKey forgets the recovery key.
func (v *Vault) RemoveRecoveryKey(ctx context.Context) error {
	_, err := v.db.ExecContext(ctx, `UPDATE vault_meta SET recovery_salt = NULL, recovery_wrapped = NULL WHERE id = 1`)
	return err
}

// Recover sets a new master password with the recovery key and unlocks
// the vault. The recovery key stays valid.
func (v *Vault) Recover(ctx context.Context, recoveryKey string, newPassword []byte) error {
	if len(newPassword) < MinPasswordLength {
		return errs.New(ErrWeakPassword, errs.Params{"min": MinPasswordLength})
	}
	var salt, wrapped []byte
	var p KDFParams
	var t, m, th sql.NullInt64
	err := v.db.QueryRowContext(ctx, `SELECT recovery_salt, recovery_time, recovery_memory, recovery_threads, recovery_wrapped FROM vault_meta WHERE id = 1`).
		Scan(&salt, &t, &m, &th, &wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return errs.New(ErrNotSetUp, nil)
	}
	if err != nil {
		return err
	}
	if len(wrapped) == 0 {
		return errs.New(ErrNoRecoveryKey, nil)
	}
	p = KDFParams{Time: uint32(t.Int64), Memory: uint32(m.Int64), Threads: uint8(th.Int64)}
	code := strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(recoveryKey)))
	dek, err := openDEK([]byte(code), salt, wrapped, p, recoveryLabel)
	if err != nil {
		return errs.New(ErrWrongRecoveryKey, nil)
	}
	defer zero(dek)
	salt2, wrapped2, err := v.wrap(newPassword, dek, v.kdf)
	if err != nil {
		return err
	}
	if _, err := v.db.ExecContext(ctx, `UPDATE vault_meta SET kdf_salt = ?, kdf_time = ?, kdf_memory = ?, kdf_threads = ?, wrapped_key = ? WHERE id = 1`,
		salt2, v.kdf.Time, v.kdf.Memory, v.kdf.Threads, wrapped2); err != nil {
		return err
	}
	v.mu.Lock()
	zero(v.dek)
	v.dek = append([]byte(nil), dek...)
	v.mu.Unlock()
	return nil
}

func sealDEK(secret, dek []byte, p KDFParams, label string) (salt, wrapped []byte, err error) {
	salt = make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	kek := argon2.IDKey(secret, salt, p.Time, p.Memory, p.Threads, chacha20poly1305.KeySize)
	defer zero(kek)
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return salt, aead.Seal(nonce, nonce, dek, []byte(label)), nil
}

func openDEK(secret, salt, wrapped []byte, p KDFParams, label string) ([]byte, error) {
	kek := argon2.IDKey(secret, salt, p.Time, p.Memory, p.Threads, chacha20poly1305.KeySize)
	defer zero(kek)
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, err
	}
	if len(wrapped) < aead.NonceSize() {
		return nil, errs.New(ErrCorrupt, nil)
	}
	return aead.Open(nil, wrapped[:aead.NonceSize()], wrapped[aead.NonceSize():], []byte(label))
}
