//go:build cgo

// Package pkcs11 uses SSH keys kept on hardware tokens (YubiKey, smart
// cards, HSMs) through their PKCS#11 module: the private key never leaves
// the token, which signs on request after the PIN.
package pkcs11

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"strings"
	"sync"

	p11 "github.com/miekg/pkcs11"
	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
)

// Supported is true when this build can load PKCS#11 modules.
const Supported = true

var (
	mu      sync.Mutex
	modules = map[string]*p11.Ctx{}
)

// module loads and initializes a library once; tokens keep working for
// the life of the app.
func module(lib string) (*p11.Ctx, error) {
	mu.Lock()
	defer mu.Unlock()
	if c, ok := modules[lib]; ok {
		return c, nil
	}
	c := p11.New(lib)
	if c == nil {
		return nil, errs.New(ErrModule, errs.Params{"module": lib})
	}
	if err := c.Initialize(); err != nil && !isCode(err, p11.CKR_CRYPTOKI_ALREADY_INITIALIZED) {
		c.Destroy()
		return nil, errs.Wrap(err, ErrModule, errs.Params{"module": lib})
	}
	modules[lib] = c
	return c, nil
}

func isCode(err error, code uint) bool {
	var e p11.Error
	return errors.As(err, &e) && uint(e) == code
}

// Tokens lists the tokens present for a module.
func Tokens(lib string) ([]Token, error) {
	c, err := module(lib)
	if err != nil {
		return nil, err
	}
	slots, err := c.GetSlotList(true)
	if err != nil {
		return nil, errs.Wrap(err, ErrModule, errs.Params{"module": lib})
	}
	out := []Token{}
	for _, s := range slots {
		info, err := c.GetTokenInfo(s)
		if err != nil || info.Flags&p11.CKF_TOKEN_INITIALIZED == 0 {
			continue // an empty slot of a soft token, or a card being set up
		}
		out = append(out, Token{
			Label: strings.TrimSpace(info.Label), Serial: strings.TrimSpace(info.SerialNumber),
			Manufacturer: strings.TrimSpace(info.ManufacturerID), Model: strings.TrimSpace(info.Model),
			NeedsPIN: info.Flags&p11.CKF_LOGIN_REQUIRED != 0,
		})
	}
	return out, nil
}

// slotOf finds the slot of the token with this serial.
func slotOf(c *p11.Ctx, serial string) (uint, error) {
	slots, err := c.GetSlotList(true)
	if err != nil {
		return 0, err
	}
	for _, s := range slots {
		if info, err := c.GetTokenInfo(s); err == nil && strings.TrimSpace(info.SerialNumber) == serial {
			return s, nil
		}
	}
	return 0, errs.New(ErrNoToken, errs.Params{"serial": serial})
}

// Keys lists the keys of a token, read from its public keys and
// certificates (no PIN needed for them on most tokens).
func Keys(lib, serial string) ([]Key, error) {
	c, err := module(lib)
	if err != nil {
		return nil, err
	}
	slot, err := slotOf(c, serial)
	if err != nil {
		return nil, err
	}
	s, err := c.OpenSession(slot, p11.CKF_SERIAL_SESSION)
	if err != nil {
		return nil, errs.Wrap(err, ErrModule, errs.Params{"module": lib})
	}
	defer c.CloseSession(s)

	found := map[string]Key{}
	var order []string
	add := func(id []byte, label string, pub crypto.PublicKey) {
		sshPub, err := ssh.NewPublicKey(pub)
		if err != nil {
			return
		}
		k := hex.EncodeToString(id)
		if _, ok := found[k]; ok {
			return
		}
		found[k] = Key{
			ID: k, Label: label, Type: sshPub.Type(),
			PublicKey:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))),
			Fingerprint: ssh.FingerprintSHA256(sshPub),
		}
		order = append(order, k)
	}
	for _, o := range find(c, s, []*p11.Attribute{p11.NewAttribute(p11.CKA_CLASS, p11.CKO_PUBLIC_KEY)}) {
		attrs, err := c.GetAttributeValue(s, o, []*p11.Attribute{
			p11.NewAttribute(p11.CKA_ID, nil), p11.NewAttribute(p11.CKA_LABEL, nil), p11.NewAttribute(p11.CKA_KEY_TYPE, nil),
		})
		if err != nil || len(attrs) < 3 {
			continue
		}
		if pub := publicKey(c, s, o, attrs[2].Value); pub != nil {
			add(attrs[0].Value, string(attrs[1].Value), pub)
		}
	}
	for _, o := range find(c, s, []*p11.Attribute{p11.NewAttribute(p11.CKA_CLASS, p11.CKO_CERTIFICATE)}) {
		attrs, err := c.GetAttributeValue(s, o, []*p11.Attribute{
			p11.NewAttribute(p11.CKA_ID, nil), p11.NewAttribute(p11.CKA_LABEL, nil), p11.NewAttribute(p11.CKA_VALUE, nil),
		})
		if err != nil || len(attrs) < 3 {
			continue
		}
		if cert, err := x509.ParseCertificate(attrs[2].Value); err == nil {
			add(attrs[0].Value, string(attrs[1].Value), cert.PublicKey)
		}
	}
	out := make([]Key, 0, len(order))
	for _, k := range order {
		out = append(out, found[k])
	}
	return out, nil
}

func find(c *p11.Ctx, s p11.SessionHandle, tmpl []*p11.Attribute) []p11.ObjectHandle {
	if err := c.FindObjectsInit(s, tmpl); err != nil {
		return nil
	}
	defer c.FindObjectsFinal(s)
	var out []p11.ObjectHandle
	for {
		objs, _, err := c.FindObjects(s, 32)
		if err != nil || len(objs) == 0 {
			return out
		}
		out = append(out, objs...)
	}
}

// publicKey reads an RSA or EC public key object.
func publicKey(c *p11.Ctx, s p11.SessionHandle, o p11.ObjectHandle, keyType []byte) crypto.PublicKey {
	switch ulong(keyType) {
	case p11.CKK_RSA:
		a, err := c.GetAttributeValue(s, o, []*p11.Attribute{p11.NewAttribute(p11.CKA_MODULUS, nil), p11.NewAttribute(p11.CKA_PUBLIC_EXPONENT, nil)})
		if err != nil || len(a) < 2 {
			return nil
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(a[0].Value), E: int(new(big.Int).SetBytes(a[1].Value).Int64())}
	case p11.CKK_EC:
		a, err := c.GetAttributeValue(s, o, []*p11.Attribute{p11.NewAttribute(p11.CKA_EC_PARAMS, nil), p11.NewAttribute(p11.CKA_EC_POINT, nil)})
		if err != nil || len(a) < 2 {
			return nil
		}
		curve := curveOf(a[0].Value)
		if curve == nil {
			return nil
		}
		point := a[1].Value
		var raw []byte // CKA_EC_POINT is a DER OCTET STRING around the point
		if _, err := asn1.Unmarshal(point, &raw); err == nil {
			point = raw
		}
		x, y := elliptic.Unmarshal(curve, point) //nolint:staticcheck // a public point, checked by Unmarshal
		if x == nil {
			return nil
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
	}
	return nil
}

var (
	oidP256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7}
	oidP384 = asn1.ObjectIdentifier{1, 3, 132, 0, 34}
	oidP521 = asn1.ObjectIdentifier{1, 3, 132, 0, 35}
)

func curveOf(params []byte) elliptic.Curve {
	var oid asn1.ObjectIdentifier
	if _, err := asn1.Unmarshal(params, &oid); err != nil {
		return nil
	}
	switch {
	case oid.Equal(oidP256):
		return elliptic.P256()
	case oid.Equal(oidP384):
		return elliptic.P384()
	case oid.Equal(oidP521):
		return elliptic.P521()
	}
	return nil
}

// ulong reads a CK_ULONG attribute (native byte order and size).
func ulong(b []byte) uint {
	var v uint
	for i := len(b) - 1; i >= 0; i-- {
		v = v<<8 | uint(b[i])
	}
	return v
}

// Signer signs with a token key, asking for the PIN (pin) the first time
// and when the token forgets the login.
type Signer struct {
	lib, serial string
	id          []byte
	pub         crypto.PublicKey
	pin         func(attempt int) (string, error)

	mu      sync.Mutex
	session p11.SessionHandle
	open    bool
}

// NewSigner returns a crypto.Signer for the token key id (hex) whose
// public key is publicKey (an authorized_keys line).
func NewSigner(lib, serial, id, publicKey string, pin func(attempt int) (string, error)) (*Signer, error) {
	raw, err := hex.DecodeString(id)
	if err != nil {
		return nil, errs.New(ErrNoKey, errs.Params{"id": id})
	}
	sshPub, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKey))
	if err != nil {
		return nil, errs.New(ErrNoKey, errs.Params{"id": id})
	}
	cp, ok := sshPub.(ssh.CryptoPublicKey)
	if !ok {
		return nil, errs.New(ErrNoKey, errs.Params{"id": id})
	}
	return &Signer{lib: lib, serial: serial, id: raw, pub: cp.CryptoPublicKey(), pin: pin}, nil
}

// Public is the key's public half.
func (s *Signer) Public() crypto.PublicKey { return s.pub }

// Sign signs digest on the token: PKCS#1 v1.5 for RSA, raw ECDSA turned
// into ASN.1 for EC keys.
func (s *Signer) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := module(s.lib)
	if err != nil {
		return nil, err
	}
	key, err := s.privateKey(c)
	if err != nil {
		return nil, err
	}
	switch s.pub.(type) {
	case *rsa.PublicKey:
		prefix, ok := digestInfo[opts.HashFunc()]
		if !ok {
			return nil, errs.New(ErrSign, nil)
		}
		return s.sign(c, key, p11.CKM_RSA_PKCS, append(append([]byte(nil), prefix...), digest...))
	case *ecdsa.PublicKey:
		raw, err := s.sign(c, key, p11.CKM_ECDSA, digest)
		if err != nil {
			return nil, err
		}
		half := len(raw) / 2
		return asn1.Marshal(struct{ R, S *big.Int }{new(big.Int).SetBytes(raw[:half]), new(big.Int).SetBytes(raw[half:])})
	}
	return nil, errs.New(ErrSign, nil)
}

func (s *Signer) sign(c *p11.Ctx, key p11.ObjectHandle, mech uint, data []byte) ([]byte, error) {
	if err := c.SignInit(s.session, []*p11.Mechanism{p11.NewMechanism(mech, nil)}, key); err != nil {
		return nil, errs.Wrap(err, ErrSign, nil)
	}
	out, err := c.Sign(s.session, data)
	if err != nil {
		return nil, errs.Wrap(err, ErrSign, nil)
	}
	return out, nil
}

// privateKey opens a logged-in session and finds the key by its CKA_ID.
func (s *Signer) privateKey(c *p11.Ctx) (p11.ObjectHandle, error) {
	if !s.open {
		slot, err := slotOf(c, s.serial)
		if err != nil {
			return 0, err
		}
		session, err := c.OpenSession(slot, p11.CKF_SERIAL_SESSION)
		if err != nil {
			return 0, errs.Wrap(err, ErrModule, errs.Params{"module": s.lib})
		}
		s.session, s.open = session, true
	}
	tmpl := []*p11.Attribute{p11.NewAttribute(p11.CKA_CLASS, p11.CKO_PRIVATE_KEY), p11.NewAttribute(p11.CKA_ID, s.id)}
	if objs := find(c, s.session, tmpl); len(objs) > 0 {
		return objs[0], nil // already logged in, or a key that needs no PIN
	}
	for attempt := 1; attempt <= 3; attempt++ {
		pin, err := s.pin(attempt)
		if err != nil {
			return 0, err
		}
		err = c.Login(s.session, p11.CKU_USER, pin)
		switch {
		case err == nil || isCode(err, p11.CKR_USER_ALREADY_LOGGED_IN):
			if objs := find(c, s.session, tmpl); len(objs) > 0 {
				return objs[0], nil
			}
			return 0, errs.New(ErrNoKey, errs.Params{"id": hex.EncodeToString(s.id)})
		case isCode(err, p11.CKR_PIN_INCORRECT):
			continue
		case isCode(err, p11.CKR_PIN_LOCKED):
			return 0, errs.New(ErrPINLocked, nil)
		default:
			return 0, errs.Wrap(err, ErrModule, errs.Params{"module": s.lib})
		}
	}
	return 0, errs.New(ErrWrongPIN, nil)
}

// Close logs out and closes the session.
func (s *Signer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.open {
		return
	}
	if c, err := module(s.lib); err == nil {
		c.Logout(s.session)
		c.CloseSession(s.session)
	}
	s.open = false
}

// digestInfo are the DER prefixes of PKCS#1 v1.5 signatures.
var digestInfo = map[crypto.Hash][]byte{
	crypto.SHA1:   {0x30, 0x21, 0x30, 0x09, 0x06, 0x05, 0x2b, 0x0e, 0x03, 0x02, 0x1a, 0x05, 0x00, 0x04, 0x14},
	crypto.SHA256: {0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20},
	crypto.SHA512: {0x30, 0x51, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x03, 0x05, 0x00, 0x04, 0x40},
}
