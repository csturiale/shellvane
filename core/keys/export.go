package keys

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/vault"
)

// Export formats.
const (
	FormatOpenSSH = "openssh"
	FormatPPK     = "ppk"
)

// Private returns a key's private part with its comment. The vault must
// be unlocked.
func (r *Repo) Private(ctx context.Context, id string) (crypto.Signer, string, error) {
	var secretID sql.NullString
	var comment string
	err := r.db.QueryRowContext(ctx, `SELECT secret_id, comment FROM keys WHERE id = ?`, id).Scan(&secretID, &comment)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", errs.New(ErrNotFound, errs.Params{"id": id})
	}
	if err != nil {
		return nil, "", err
	}
	if !secretID.Valid {
		return nil, "", errs.New(ErrOnToken, errs.Params{"id": id})
	}
	pemBytes, err := r.vault.Get(ctx, secretID.String)
	if err != nil {
		return nil, "", err
	}
	defer vault.Zero(pemBytes)
	raw, err := ssh.ParseRawPrivateKey(pemBytes)
	if err != nil {
		return nil, "", errs.Wrap(err, ErrUnsupported, nil)
	}
	s, err := asSigner(raw)
	return s, comment, err
}

// Export writes a private key as OpenSSH PEM or PuTTY PPK (version 3),
// encrypted with the passphrase when it is not empty.
func Export(priv crypto.Signer, comment, format, passphrase string) (string, error) {
	switch format {
	case FormatOpenSSH:
		var block *pem.Block
		var err error
		if passphrase == "" {
			block, err = ssh.MarshalPrivateKey(priv, comment)
		} else {
			block, err = ssh.MarshalPrivateKeyWithPassphrase(priv, comment, []byte(passphrase))
		}
		if err != nil {
			return "", errs.Wrap(err, ErrUnsupported, nil)
		}
		return string(pem.EncodeToMemory(block)), nil
	case FormatPPK:
		return marshalPPK(priv, comment, passphrase)
	}
	return "", errs.New(ErrUnsupported, errs.Params{"format": format})
}

// sshString appends an SSH string (uint32 length, bytes).
func sshString(b *bytes.Buffer, data []byte) {
	binary.Write(b, binary.BigEndian, uint32(len(data)))
	b.Write(data)
}

// mpint appends an SSH mpint.
func mpint(b *bytes.Buffer, n *big.Int) {
	data := n.Bytes()
	if len(data) > 0 && data[0]&0x80 != 0 {
		data = append([]byte{0}, data...)
	}
	sshString(b, data)
}

// PPK v3 Argon2id parameters (PuTTY's defaults are similar).
const (
	ppkArgonMemory = 8192
	ppkArgonPasses = 13
	ppkArgonLanes  = 1
)

func marshalPPK(priv crypto.Signer, comment, passphrase string) (string, error) {
	signer, err := ssh.NewSignerFromSigner(priv)
	if err != nil {
		return "", errs.Wrap(err, ErrUnsupported, nil)
	}
	pub := signer.PublicKey()
	var private bytes.Buffer
	switch k := priv.(type) {
	case ed25519.PrivateKey:
		sshString(&private, k.Seed())
	case *ed25519.PrivateKey:
		sshString(&private, k.Seed())
	case *ecdsa.PrivateKey:
		mpint(&private, k.D)
	case *rsa.PrivateKey:
		if len(k.Primes) != 2 {
			return "", errs.New(ErrUnsupported, nil)
		}
		k.Precompute()
		mpint(&private, k.D)
		mpint(&private, k.Primes[0])
		mpint(&private, k.Primes[1])
		mpint(&private, k.Precomputed.Qinv)
	default:
		return "", errs.New(ErrUnsupported, nil)
	}

	encryption := "none"
	var header strings.Builder
	var macKey []byte
	plain := private.Bytes()
	blob := plain
	if passphrase != "" {
		encryption = "aes256-cbc"
		if pad := len(plain) % aes.BlockSize; pad != 0 {
			padding := make([]byte, aes.BlockSize-pad)
			rand.Read(padding)
			plain = append(plain, padding...)
		}
		salt := make([]byte, 16)
		rand.Read(salt)
		h := argon2.IDKey([]byte(passphrase), salt, ppkArgonPasses, ppkArgonMemory, ppkArgonLanes, 80)
		key, iv := h[:32], h[32:48]
		macKey = h[48:]
		block, _ := aes.NewCipher(key)
		blob = make([]byte, len(plain))
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(blob, plain)
		fmt.Fprintf(&header, "Key-Derivation: Argon2id\nArgon2-Memory: %d\nArgon2-Passes: %d\nArgon2-Parallelism: %d\nArgon2-Salt: %s\n",
			ppkArgonMemory, ppkArgonPasses, ppkArgonLanes, hex.EncodeToString(salt))
	}

	var macData bytes.Buffer
	sshString(&macData, []byte(pub.Type()))
	sshString(&macData, []byte(encryption))
	sshString(&macData, []byte(comment))
	sshString(&macData, pub.Marshal())
	sshString(&macData, plain)
	m := hmac.New(sha256.New, macKey)
	m.Write(macData.Bytes())

	var out strings.Builder
	fmt.Fprintf(&out, "PuTTY-User-Key-File-3: %s\nEncryption: %s\nComment: %s\n", pub.Type(), encryption, comment)
	writeLines := func(label string, data []byte) {
		enc := base64.StdEncoding.EncodeToString(data)
		var lines []string
		for len(enc) > 64 {
			lines = append(lines, enc[:64])
			enc = enc[64:]
		}
		lines = append(lines, enc)
		fmt.Fprintf(&out, "%s-Lines: %d\n%s\n", label, len(lines), strings.Join(lines, "\n"))
	}
	writeLines("Public", pub.Marshal())
	out.WriteString(header.String())
	writeLines("Private", blob)
	fmt.Fprintf(&out, "Private-MAC: %s\n", hex.EncodeToString(m.Sum(nil)))
	return out.String(), nil
}
