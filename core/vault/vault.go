// Package vault stores secrets encrypted at rest.
//
// The master password is stretched with Argon2id into a key-encryption key
// (KEK) that wraps a random data-encryption key (DEK). Secrets are sealed with
// XChaCha20-Poly1305 under the DEK, with the secret's ID and kind as
// associated data so ciphertexts cannot be swapped between records. Changing
// the master password re-wraps only the DEK.
//
// The DEK lives in memory only while the vault is unlocked and is zeroed on
// Lock. Go cannot guarantee that no copy survives in memory; secrets are kept
// as byte slices and cleared by callers when possible.
package vault

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes returned by this package.
const (
	ErrNotSetUp       errs.Code = "vault.notSetUp"
	ErrAlreadySetUp   errs.Code = "vault.alreadySetUp"
	ErrLocked         errs.Code = "vault.locked"
	ErrWrongPassword  errs.Code = "vault.wrongPassword"
	ErrWeakPassword   errs.Code = "vault.weakPassword"
	ErrSecretNotFound errs.Code = "vault.secretNotFound"
	ErrCorrupt        errs.Code = "vault.corrupt"
)

// Version of the on-disk format.
const formatVersion = 1

// MinPasswordLength is the shortest accepted master password.
const MinPasswordLength = 8

// KDFParams are the Argon2id parameters, stored with the vault.
type KDFParams struct {
	Time    uint32
	Memory  uint32 // KiB
	Threads uint8
}

// DefaultKDF follows the OWASP recommendation for Argon2id (64 MiB, t=3).
var DefaultKDF = KDFParams{Time: 3, Memory: 64 * 1024, Threads: 4}

// State of the vault.
type State string

const (
	StateNotSetUp State = "notSetUp"
	StateLocked   State = "locked"
	StateUnlocked State = "unlocked"
)

// Vault is safe for concurrent use.
type Vault struct {
	db  *sql.DB
	kdf KDFParams
	now func() time.Time

	mu  sync.RWMutex
	dek []byte // nil while locked
}

// New returns a vault over a migrated database. kdf applies to new vaults
// and password changes; existing vaults use their stored parameters.
func New(db *sql.DB, kdf KDFParams) *Vault {
	return &Vault{db: db, kdf: kdf, now: time.Now}
}

// State reports whether the vault exists and is unlocked.
func (v *Vault) State(ctx context.Context) (State, error) {
	v.mu.RLock()
	unlocked := v.dek != nil
	v.mu.RUnlock()
	if unlocked {
		return StateUnlocked, nil
	}
	var n int
	if err := v.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM vault_meta`).Scan(&n); err != nil {
		return "", err
	}
	if n == 0 {
		return StateNotSetUp, nil
	}
	return StateLocked, nil
}

// Setup creates the vault and leaves it unlocked.
func (v *Vault) Setup(ctx context.Context, password []byte) error {
	if len(password) < MinPasswordLength {
		return errs.New(ErrWeakPassword, errs.Params{"min": MinPasswordLength})
	}
	state, err := v.State(ctx)
	if err != nil {
		return err
	}
	if state != StateNotSetUp {
		return errs.New(ErrAlreadySetUp, nil)
	}
	dek := make([]byte, chacha20poly1305.KeySize)
	if _, err := rand.Read(dek); err != nil {
		return err
	}
	salt, wrapped, err := v.wrap(password, dek, v.kdf)
	if err != nil {
		return err
	}
	_, err = v.db.ExecContext(ctx, `INSERT INTO vault_meta (id, version, kdf_salt, kdf_time, kdf_memory, kdf_threads, wrapped_key, created_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?)`, formatVersion, salt, v.kdf.Time, v.kdf.Memory, v.kdf.Threads, wrapped, v.timestamp())
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.dek = dek
	v.mu.Unlock()
	return nil
}

// Unlock derives the KEK from password and unwraps the data key.
func (v *Vault) Unlock(ctx context.Context, password []byte) error {
	dek, err := v.unwrap(ctx, password)
	if err != nil {
		return err
	}
	v.mu.Lock()
	zero(v.dek)
	v.dek = dek
	v.mu.Unlock()
	return nil
}

// Lock forgets the data key.
func (v *Vault) Lock() {
	v.mu.Lock()
	zero(v.dek)
	v.dek = nil
	v.mu.Unlock()
}

// ChangePassword re-wraps the data key under a new master password.
func (v *Vault) ChangePassword(ctx context.Context, oldPassword, newPassword []byte) error {
	if len(newPassword) < MinPasswordLength {
		return errs.New(ErrWeakPassword, errs.Params{"min": MinPasswordLength})
	}
	dek, err := v.unwrap(ctx, oldPassword)
	if err != nil {
		return err
	}
	defer zero(dek)
	salt, wrapped, err := v.wrap(newPassword, dek, v.kdf)
	if err != nil {
		return err
	}
	_, err = v.db.ExecContext(ctx, `UPDATE vault_meta SET kdf_salt = ?, kdf_time = ?, kdf_memory = ?, kdf_threads = ?, wrapped_key = ? WHERE id = 1`,
		salt, v.kdf.Time, v.kdf.Memory, v.kdf.Threads, wrapped)
	return err
}

// Put stores a new secret and returns its ID.
func (v *Vault) Put(ctx context.Context, kind string, plaintext []byte) (string, error) {
	id := uuid.Must(uuid.NewV7()).String()
	ct, err := v.seal(id, kind, plaintext)
	if err != nil {
		return "", err
	}
	now := v.timestamp()
	_, err = v.db.ExecContext(ctx, `INSERT INTO secrets (id, kind, ciphertext, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		id, kind, ct, now, now)
	return id, err
}

// Update replaces a secret's value.
func (v *Vault) Update(ctx context.Context, id string, plaintext []byte) error {
	var kind string
	if err := v.db.QueryRowContext(ctx, `SELECT kind FROM secrets WHERE id = ?`, id).Scan(&kind); err != nil {
		return notFound(err, id)
	}
	ct, err := v.seal(id, kind, plaintext)
	if err != nil {
		return err
	}
	_, err = v.db.ExecContext(ctx, `UPDATE secrets SET ciphertext = ?, updated_at = ? WHERE id = ?`, ct, v.timestamp(), id)
	return err
}

// Get decrypts a secret. The caller should zero the result after use.
func (v *Vault) Get(ctx context.Context, id string) ([]byte, error) {
	var kind string
	var ct []byte
	if err := v.db.QueryRowContext(ctx, `SELECT kind, ciphertext FROM secrets WHERE id = ?`, id).Scan(&kind, &ct); err != nil {
		return nil, notFound(err, id)
	}
	return v.open(id, kind, ct)
}

// Delete removes a secret and any credential pointing to it.
func (v *Vault) Delete(ctx context.Context, id string) error {
	_, err := v.db.ExecContext(ctx, `DELETE FROM secrets WHERE id = ?`, id)
	return err
}

func (v *Vault) seal(id, kind string, plaintext []byte) ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.dek == nil {
		return nil, errs.New(ErrLocked, nil)
	}
	aead, err := chacha20poly1305.NewX(v.dek)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plaintext, aad(id, kind)), nil
}

func (v *Vault) open(id, kind string, ct []byte) ([]byte, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.dek == nil {
		return nil, errs.New(ErrLocked, nil)
	}
	aead, err := chacha20poly1305.NewX(v.dek)
	if err != nil {
		return nil, err
	}
	if len(ct) < aead.NonceSize() {
		return nil, errs.New(ErrCorrupt, errs.Params{"id": id})
	}
	pt, err := aead.Open(nil, ct[:aead.NonceSize()], ct[aead.NonceSize():], aad(id, kind))
	if err != nil {
		return nil, errs.Wrap(err, ErrCorrupt, errs.Params{"id": id})
	}
	return pt, nil
}

func (v *Vault) wrap(password, dek []byte, p KDFParams) (salt, wrapped []byte, err error) {
	salt = make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, err
	}
	kek := argon2.IDKey(password, salt, p.Time, p.Memory, p.Threads, chacha20poly1305.KeySize)
	defer zero(kek)
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return salt, aead.Seal(nonce, nonce, dek, []byte("termora-dek-v1")), nil
}

func (v *Vault) unwrap(ctx context.Context, password []byte) ([]byte, error) {
	var salt, wrapped []byte
	var p KDFParams
	err := v.db.QueryRowContext(ctx, `SELECT kdf_salt, kdf_time, kdf_memory, kdf_threads, wrapped_key FROM vault_meta WHERE id = 1`).
		Scan(&salt, &p.Time, &p.Memory, &p.Threads, &wrapped)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.New(ErrNotSetUp, nil)
	}
	if err != nil {
		return nil, err
	}
	kek := argon2.IDKey(password, salt, p.Time, p.Memory, p.Threads, chacha20poly1305.KeySize)
	defer zero(kek)
	aead, err := chacha20poly1305.NewX(kek)
	if err != nil {
		return nil, err
	}
	if len(wrapped) < aead.NonceSize() {
		return nil, errs.New(ErrCorrupt, nil)
	}
	dek, err := aead.Open(nil, wrapped[:aead.NonceSize()], wrapped[aead.NonceSize():], []byte("termora-dek-v1"))
	if err != nil {
		return nil, errs.New(ErrWrongPassword, nil)
	}
	return dek, nil
}

func (v *Vault) timestamp() string { return v.now().UTC().Format(time.RFC3339Nano) }

// Format identifiers (here and in wrapDEK/unwrapDEK) from before the rename to Shellvane: never change it,
// or data written by earlier versions could no longer be read or verified.
func aad(id, kind string) []byte { return []byte("termora-secret-v1|" + id + "|" + kind) }

func notFound(err error, id string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return errs.New(ErrSecretNotFound, errs.Params{"id": id})
	}
	return err
}

// zero overwrites b; used for keys and plaintexts.
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// Zero lets callers clear plaintexts returned by Get.
func Zero(b []byte) { zero(b) }
