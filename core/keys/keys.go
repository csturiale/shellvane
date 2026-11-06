// Package keys generates, imports and stores SSH private keys. Private keys
// live in the vault; the public part and fingerprint are stored in clear so
// they can be listed and copied while the vault is locked.
package keys

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kayrus/putty"
	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/vault"
)

// Error codes returned by this package.
const (
	ErrUnsupported        errs.Code = "keys.unsupported"
	ErrPassphraseRequired errs.Code = "keys.passphraseRequired"
	ErrWrongPassphrase    errs.Code = "keys.wrongPassphrase"
	ErrNotFound           errs.Code = "keys.notFound"
	ErrDuplicate          errs.Code = "keys.duplicate"
	ErrNameRequired       errs.Code = "keys.nameRequired"
	ErrCertificate        errs.Code = "keys.certificateInvalid"
	ErrCertificateKey     errs.Code = "keys.certificateMismatch"
	// ErrOnToken: the key is on a hardware token, so it cannot be read,
	// exported or rotated, only used to sign.
	ErrOnToken errs.Code = "keys.onToken"
)

// Key is a stored key, without its private part.
type Key struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // ssh-ed25519, ssh-rsa, ecdsa-sha2-nistp256...
	Bits        int    `json:"bits"`
	PublicKey   string `json:"publicKey"` // authorized_keys line
	Fingerprint string `json:"fingerprint"`
	Comment     string `json:"comment"`
	CreatedAt   string `json:"createdAt"`
	// Certificate fields are empty when the key has no certificate.
	Certificate     string   `json:"certificate"`
	CertPrincipals  []string `json:"certPrincipals"`
	CertValidBefore string   `json:"certValidBefore"` // RFC 3339; "" = forever
	// Token is set for a key on a hardware token: its private part never
	// leaves the token and is not in the vault.
	Token *TokenRef `json:"token,omitempty"`
}

// TokenRef locates a key on a PKCS#11 token.
type TokenRef struct {
	Module string `json:"module"` // the PKCS#11 library
	Serial string `json:"serial"` // the token's serial number
	Label  string `json:"label"`  // the token's label, shown to the user
	KeyID  string `json:"keyId"`  // CKA_ID, hex
}

// Generate creates a private key: kind is "ed25519", "rsa" (bits 3072 or
// 4096) or "ecdsa" (bits 256, 384 or 521).
func Generate(kind string, bits int) (crypto.Signer, error) {
	switch kind {
	case "ed25519":
		_, k, err := ed25519.GenerateKey(rand.Reader)
		return k, err
	case "rsa":
		if bits != 3072 && bits != 4096 {
			bits = 4096
		}
		return rsa.GenerateKey(rand.Reader, bits)
	case "ecdsa":
		curve := map[int]elliptic.Curve{256: elliptic.P256(), 384: elliptic.P384(), 521: elliptic.P521()}[bits]
		if curve == nil {
			curve = elliptic.P256()
		}
		return ecdsa.GenerateKey(curve, rand.Reader)
	default:
		return nil, errs.New(ErrUnsupported, errs.Params{"kind": kind})
	}
}

// Parse reads a private key in OpenSSH, PEM (PKCS#1/PKCS#8/SEC1) or PuTTY
// (.ppk v2 and v3) format. passphrase may be empty for unencrypted keys.
func Parse(data, passphrase []byte) (crypto.Signer, string, error) {
	data = bytes.TrimSpace(data)
	if bytes.HasPrefix(data, []byte("PuTTY-User-Key-File-")) {
		return parsePPK(data, passphrase)
	}
	var raw any
	var err error
	if len(passphrase) > 0 {
		raw, err = ssh.ParseRawPrivateKeyWithPassphrase(data, passphrase)
	} else {
		raw, err = ssh.ParseRawPrivateKey(data)
	}
	var missing *ssh.PassphraseMissingError
	switch {
	case errors.As(err, &missing):
		return nil, "", errs.New(ErrPassphraseRequired, nil)
	case errors.Is(err, x509.IncorrectPasswordError):
		return nil, "", errs.New(ErrWrongPassphrase, nil)
	case err != nil:
		return nil, "", errs.Wrap(err, ErrUnsupported, nil)
	}
	// x/crypto does not expose the comment of OpenSSH keys.
	signer, err := asSigner(raw)
	return signer, "", err
}

func parsePPK(data, passphrase []byte) (crypto.Signer, string, error) {
	k, err := putty.New(data)
	if err != nil {
		return nil, "", errs.Wrap(err, ErrUnsupported, nil)
	}
	if k.Encryption != "none" && len(passphrase) == 0 {
		return nil, "", errs.New(ErrPassphraseRequired, nil)
	}
	raw, err := k.ParseRawPrivateKey(passphrase)
	if err != nil {
		if k.Encryption != "none" {
			return nil, "", errs.Wrap(err, ErrWrongPassphrase, nil)
		}
		return nil, "", errs.Wrap(err, ErrUnsupported, nil)
	}
	signer, err := asSigner(raw)
	return signer, k.Comment, err
}

func asSigner(raw any) (crypto.Signer, error) {
	switch k := raw.(type) {
	case *ed25519.PrivateKey:
		return *k, nil
	case crypto.Signer:
		return k, nil
	default:
		return nil, errs.New(ErrUnsupported, nil)
	}
}

// Repo stores keys; private parts go to the vault.
type Repo struct {
	db    *sql.DB
	vault *vault.Vault
	now   func() time.Time
}

// NewRepo returns a key store over a migrated database and its vault.
func NewRepo(db *sql.DB, v *vault.Vault) *Repo { return &Repo{db: db, vault: v, now: time.Now} }

// Add stores a private key under name. The vault must be unlocked.
func (r *Repo) Add(ctx context.Context, name string, priv crypto.Signer, comment string) (Key, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Key{}, errs.New(ErrNameRequired, nil)
	}
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		return Key{}, errs.Wrap(err, ErrUnsupported, nil)
	}
	pub := signer.PublicKey()
	k := Key{
		ID:          uuid.Must(uuid.NewV7()).String(),
		Name:        name,
		Type:        pub.Type(),
		Bits:        bitsOf(priv),
		PublicKey:   authorizedLine(pub, comment),
		Fingerprint: ssh.FingerprintSHA256(pub),
		Comment:     comment,
		CreatedAt:   r.now().UTC().Format(time.RFC3339),
	}
	var dup int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM keys WHERE fingerprint = ?`, k.Fingerprint).Scan(&dup)
	if dup > 0 {
		return Key{}, errs.New(ErrDuplicate, errs.Params{"fingerprint": k.Fingerprint})
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return Key{}, errs.Wrap(err, ErrUnsupported, nil)
	}
	pemBytes := pem.EncodeToMemory(block)
	defer vault.Zero(pemBytes)
	secretID, err := r.vault.Put(ctx, "privateKey", pemBytes)
	if err != nil {
		return Key{}, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO keys (id, name, type, bits, public_key, fingerprint, comment, secret_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, k.ID, k.Name, k.Type, k.Bits, k.PublicKey, k.Fingerprint, k.Comment, secretID, k.CreatedAt)
	if err != nil {
		r.vault.Delete(ctx, secretID)
		return Key{}, err
	}
	return k, nil
}

// List returns all keys, newest first.
func (r *Repo) List(ctx context.Context) ([]Key, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, type, bits, public_key, fingerprint, comment, created_at, certificate, token FROM keys ORDER BY created_at DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Key{}
	for rows.Next() {
		var k Key
		var token string
		if err := rows.Scan(&k.ID, &k.Name, &k.Type, &k.Bits, &k.PublicKey, &k.Fingerprint, &k.Comment, &k.CreatedAt, &k.Certificate, &token); err != nil {
			return nil, err
		}
		k.Token = parseToken(token)
		describeCert(&k)
		out = append(out, k)
	}
	return out, rows.Err()
}

// Rename changes a key's display name.
func (r *Repo) Rename(ctx context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errs.New(ErrNameRequired, nil)
	}
	res, err := r.db.ExecContext(ctx, `UPDATE keys SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return nil
}

// Delete removes a key and its private part.
func (r *Repo) Delete(ctx context.Context, id string) error {
	var secretID sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT secret_id FROM keys WHERE id = ?`, id).Scan(&secretID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM keys WHERE id = ?`, id); err != nil {
		return err
	}
	if !secretID.Valid {
		return nil // a hardware key: nothing in the vault
	}
	return r.vault.Delete(ctx, secretID.String)
}

// SetCertificate attaches an OpenSSH certificate to a key ("" removes it).
// The certificate must certify this key.
func (r *Repo) SetCertificate(ctx context.Context, id, certLine string) error {
	certLine = strings.TrimSpace(certLine)
	if certLine != "" {
		cert, err := ParseCertificate(certLine)
		if err != nil {
			return err
		}
		var fp string
		if err := r.db.QueryRowContext(ctx, `SELECT fingerprint FROM keys WHERE id = ?`, id).Scan(&fp); err != nil {
			return errs.New(ErrNotFound, errs.Params{"id": id})
		}
		if ssh.FingerprintSHA256(cert.Key) != fp {
			return errs.New(ErrCertificateKey, nil)
		}
	}
	res, err := r.db.ExecContext(ctx, `UPDATE keys SET certificate = ? WHERE id = ?`, certLine, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return nil
}

// ParseCertificate reads a certificate in authorized_keys format.
func ParseCertificate(line string) (*ssh.Certificate, error) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(line))
	if err != nil {
		return nil, errs.Wrap(err, ErrCertificate, nil)
	}
	cert, ok := pub.(*ssh.Certificate)
	if !ok || cert.CertType != ssh.UserCert {
		return nil, errs.New(ErrCertificate, nil)
	}
	return cert, nil
}

func describeCert(k *Key) {
	if k.Certificate == "" {
		return
	}
	cert, err := ParseCertificate(k.Certificate)
	if err != nil {
		return
	}
	k.CertPrincipals = cert.ValidPrincipals
	if cert.ValidBefore != ssh.CertTimeInfinity {
		k.CertValidBefore = time.Unix(int64(cert.ValidBefore), 0).UTC().Format(time.RFC3339)
	}
}

// CertSigner presents cert together with the private key.
func CertSigner(cert *ssh.Certificate, s ssh.Signer) (ssh.Signer, error) {
	cs, err := ssh.NewCertSigner(cert, s)
	if err != nil {
		return nil, errs.Wrap(err, ErrCertificateKey, nil)
	}
	return cs, nil
}

// Signer loads a private key for authentication, with its certificate when
// one is attached and still valid. The vault must be unlocked.
func (r *Repo) Signer(ctx context.Context, id string) (ssh.Signer, error) {
	var secretID sql.NullString
	var certLine string
	err := r.db.QueryRowContext(ctx, `SELECT secret_id, certificate FROM keys WHERE id = ?`, id).Scan(&secretID, &certLine)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.New(ErrNotFound, errs.Params{"id": id})
	}
	if err != nil {
		return nil, err
	}
	if !secretID.Valid {
		return nil, errs.New(ErrOnToken, errs.Params{"id": id})
	}
	pemBytes, err := r.vault.Get(ctx, secretID.String)
	if err != nil {
		return nil, err
	}
	defer vault.Zero(pemBytes)
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}
	return WithCertificate(signer, certLine), nil
}

// WithCertificate adds a key's certificate to its signer while it is
// valid; an expired one is left out (the plain key may still be accepted).
func WithCertificate(signer ssh.Signer, certLine string) ssh.Signer {
	if certLine == "" {
		return signer
	}
	cert, err := ParseCertificate(certLine)
	if err != nil {
		return signer
	}
	if cert.ValidBefore != ssh.CertTimeInfinity && time.Now().Unix() >= int64(cert.ValidBefore) {
		return signer
	}
	cs, err := CertSigner(cert, signer)
	if err != nil {
		return signer
	}
	return cs
}

// AddToken records a key that lives on a hardware token. Nothing goes in
// the vault, so the vault may be locked.
func (r *Repo) AddToken(ctx context.Context, name string, ref TokenRef, publicKey string) (Key, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Key{}, errs.New(ErrNameRequired, nil)
	}
	pub, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return Key{}, errs.Wrap(err, ErrUnsupported, nil)
	}
	bits := 0
	if cp, ok := pub.(ssh.CryptoPublicKey); ok {
		bits = publicBits(cp.CryptoPublicKey())
	}
	k := Key{
		ID: uuid.Must(uuid.NewV7()).String(), Name: name, Type: pub.Type(), Bits: bits,
		PublicKey: authorizedLine(pub, comment), Fingerprint: ssh.FingerprintSHA256(pub), Comment: comment,
		CreatedAt: r.now().UTC().Format(time.RFC3339), Token: &ref,
	}
	var dup int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM keys WHERE fingerprint = ?`, k.Fingerprint).Scan(&dup)
	if dup > 0 {
		return Key{}, errs.New(ErrDuplicate, errs.Params{"fingerprint": k.Fingerprint})
	}
	token, _ := json.Marshal(ref)
	_, err = r.db.ExecContext(ctx, `INSERT INTO keys (id, name, type, bits, public_key, fingerprint, comment, secret_id, created_at, token)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)`, k.ID, k.Name, k.Type, k.Bits, k.PublicKey, k.Fingerprint, k.Comment, k.CreatedAt, string(token))
	return k, err
}

// TokenOf returns where a hardware key lives, its public key and its
// certificate; ok is false for keys kept in the vault.
func (r *Repo) TokenOf(ctx context.Context, id string) (ref TokenRef, publicKey, certLine string, ok bool, err error) {
	var token string
	err = r.db.QueryRowContext(ctx, `SELECT token, public_key, certificate FROM keys WHERE id = ?`, id).Scan(&token, &publicKey, &certLine)
	if errors.Is(err, sql.ErrNoRows) {
		return ref, "", "", false, errs.New(ErrNotFound, errs.Params{"id": id})
	}
	if err != nil {
		return ref, "", "", false, err
	}
	t := parseToken(token)
	if t == nil {
		return ref, publicKey, certLine, false, nil
	}
	return *t, publicKey, certLine, true, nil
}

func parseToken(s string) *TokenRef {
	if s == "" {
		return nil
	}
	var t TokenRef
	if json.Unmarshal([]byte(s), &t) != nil || t.Module == "" {
		return nil
	}
	return &t
}

// PublicKey returns a key's public key, for certificate commands.
func (r *Repo) PublicKey(ctx context.Context, id string) (string, error) {
	var line string
	if err := r.db.QueryRowContext(ctx, `SELECT public_key FROM keys WHERE id = ?`, id).Scan(&line); err != nil {
		return "", errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return line, nil
}

func authorizedLine(pub ssh.PublicKey, comment string) string {
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	if comment != "" {
		line += " " + comment
	}
	return line
}

// publicBits is the size of a public key.
func publicBits(k crypto.PublicKey) int {
	switch p := k.(type) {
	case *rsa.PublicKey:
		return p.N.BitLen()
	case *ecdsa.PublicKey:
		return p.Curve.Params().BitSize
	case ed25519.PublicKey:
		return 256
	}
	return 0
}

func bitsOf(k crypto.Signer) int {
	switch p := k.(type) {
	case *rsa.PrivateKey:
		return p.N.BitLen()
	case *ecdsa.PrivateKey:
		return p.Curve.Params().BitSize
	case ed25519.PrivateKey:
		return 256
	}
	return 0
}
