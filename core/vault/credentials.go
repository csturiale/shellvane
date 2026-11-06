package vault

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Credential is a saved answer to a login prompt. Its metadata is readable
// while the vault is locked; the secret is not.
type Credential struct {
	Scope     string `json:"scope"`
	Label     string `json:"label"`
	CreatedAt string `json:"createdAt"`
}

// PasswordScope identifies the password of user@host:port.
func PasswordScope(user, host string, port int) string {
	return fmt.Sprintf("password:%s@%s:%d", user, host, port)
}

// PassphraseScope identifies the passphrase of a private key.
func PassphraseScope(fingerprint string) string {
	return "passphrase:" + fingerprint
}

// HasCredential reports whether a credential is saved for scope.
func (v *Vault) HasCredential(ctx context.Context, scope string) (bool, error) {
	var n int
	err := v.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM credentials WHERE scope = ?`, scope).Scan(&n)
	return n > 0, err
}

// Credential returns the saved secret for scope; found is false when none is
// saved. The vault must be unlocked.
func (v *Vault) Credential(ctx context.Context, scope string) (secret []byte, found bool, err error) {
	var id string
	err = v.db.QueryRowContext(ctx, `SELECT secret_id FROM credentials WHERE scope = ?`, scope).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	secret, err = v.Get(ctx, id)
	return secret, err == nil, err
}

// SaveCredential stores or replaces the secret for scope.
func (v *Vault) SaveCredential(ctx context.Context, scope, label string, secret []byte) error {
	var id string
	err := v.db.QueryRowContext(ctx, `SELECT secret_id FROM credentials WHERE scope = ?`, scope).Scan(&id)
	switch {
	case err == nil:
		if err := v.Update(ctx, id, secret); err != nil {
			return err
		}
		_, err = v.db.ExecContext(ctx, `UPDATE credentials SET label = ? WHERE scope = ?`, label, scope)
		return err
	case errors.Is(err, sql.ErrNoRows):
		id, err := v.Put(ctx, "credential", secret)
		if err != nil {
			return err
		}
		_, err = v.db.ExecContext(ctx, `INSERT INTO credentials (scope, secret_id, label, created_at) VALUES (?, ?, ?, ?)`,
			scope, id, label, v.timestamp())
		return err
	default:
		return err
	}
}

// Credentials lists saved credentials, newest first.
func (v *Vault) Credentials(ctx context.Context) ([]Credential, error) {
	rows, err := v.db.QueryContext(ctx, `SELECT scope, label, created_at FROM credentials ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Credential{}
	for rows.Next() {
		var c Credential
		if err := rows.Scan(&c.Scope, &c.Label, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// DeleteCredential forgets a saved credential and its secret.
func (v *Vault) DeleteCredential(ctx context.Context, scope string) error {
	var id string
	err := v.db.QueryRowContext(ctx, `SELECT secret_id FROM credentials WHERE scope = ?`, scope).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return v.Delete(ctx, id) // the credential row cascades
}
