package app

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"math/big"
	"slices"
	"time"
)

// keychainAlgorithm is the index of shellvane_cc_sign's algorithm.
func keychainAlgorithm(pub crypto.PublicKey, opts crypto.SignerOpts) (int, error) {
	hashes := map[crypto.Hash]int{crypto.SHA1: 0, crypto.SHA256: 1, crypto.SHA384: 2, crypto.SHA512: 3}
	h, ok := hashes[opts.HashFunc()]
	if !ok {
		return 0, errors.New("keychain: unsupported hash")
	}
	switch pub.(type) {
	case *rsa.PublicKey:
		if _, pss := opts.(*rsa.PSSOptions); pss {
			if h == 0 {
				return 0, errors.New("keychain: unsupported hash")
			}
			return 3 + h, nil // PSS SHA-256/384/512: 4, 5, 6
		}
		return h, nil
	case *ecdsa.PublicKey:
		return 7 + h, nil
	}
	return 0, errors.New("keychain: unsupported key type")
}

// clientCertMatches tells whether a certificate (with the chain the system
// built for it, leaf first) can answer a server that accepts the
// authorities acceptableCAs (DER names, empty: any): in force at now,
// usable for client authentication, issued under one of them.
func clientCertMatches(chain []*x509.Certificate, acceptableCAs [][]byte, now time.Time) bool {
	if len(chain) == 0 {
		return false
	}
	leaf := chain[0]
	if now.Before(leaf.NotBefore) || now.After(leaf.NotAfter) {
		return false
	}
	if len(leaf.ExtKeyUsage) > 0 && !slices.ContainsFunc(leaf.ExtKeyUsage, func(u x509.ExtKeyUsage) bool {
		return u == x509.ExtKeyUsageClientAuth || u == x509.ExtKeyUsageAny
	}) {
		return false
	}
	if len(acceptableCAs) == 0 {
		return true
	}
	for _, c := range chain {
		for _, ca := range acceptableCAs {
			if bytes.Equal(c.RawIssuer, ca) || bytes.Equal(c.RawSubject, ca) && c != leaf {
				return true
			}
		}
	}
	return false
}

// ecdsaASN1 turns a raw ECDSA signature (r and s of the same size, as
// Windows' CNG gives it) into the DER form TLS sends.
func ecdsaASN1(raw []byte) ([]byte, error) {
	if len(raw) == 0 || len(raw)%2 != 0 {
		return nil, errors.New("bad ECDSA signature")
	}
	half := len(raw) / 2
	return asn1.Marshal(struct{ R, S *big.Int }{new(big.Int).SetBytes(raw[:half]), new(big.Int).SetBytes(raw[half:])})
}
