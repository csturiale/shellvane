package app

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"testing"
	"time"
)

func TestKeychainAlgorithm(t *testing.T) {
	rsaKey, ecKey := &rsa.PublicKey{}, &ecdsa.PublicKey{}
	pss := &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA384}
	for _, c := range []struct {
		pub  crypto.PublicKey
		opts crypto.SignerOpts
		want int
	}{
		{rsaKey, crypto.SHA256, 1}, {rsaKey, crypto.SHA1, 0}, {rsaKey, pss, 5},
		{ecKey, crypto.SHA256, 8}, {ecKey, crypto.SHA512, 10},
	} {
		if got, err := keychainAlgorithm(c.pub, c.opts); err != nil || got != c.want {
			t.Errorf("%T %v: %d %v, want %d", c.pub, c.opts.HashFunc(), got, err, c.want)
		}
	}
	if _, err := keychainAlgorithm(rsaKey, &rsa.PSSOptions{Hash: crypto.SHA1}); err == nil {
		t.Error("PSS with SHA-1")
	}
	if _, err := keychainAlgorithm("ed25519?", crypto.SHA256); err == nil {
		t.Error("unknown key")
	}
}

func TestClientCertMatches(t *testing.T) {
	now := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	ca := &x509.Certificate{RawSubject: []byte("ca"), RawIssuer: []byte("root")}
	leaf := &x509.Certificate{RawSubject: []byte("device"), RawIssuer: []byte("ca"), NotBefore: now.AddDate(-1, 0, 0), NotAfter: now.AddDate(1, 0, 0)}
	if !clientCertMatches([]*x509.Certificate{leaf, ca}, [][]byte{[]byte("ca")}, now) ||
		!clientCertMatches([]*x509.Certificate{leaf, ca}, [][]byte{[]byte("root")}, now) ||
		!clientCertMatches([]*x509.Certificate{leaf}, nil, now) {
		t.Fatal("should match")
	}
	if clientCertMatches([]*x509.Certificate{leaf, ca}, [][]byte{[]byte("other"), []byte("device")}, now) {
		t.Fatal("another authority")
	}
	if clientCertMatches([]*x509.Certificate{leaf}, nil, now.AddDate(2, 0, 0)) {
		t.Fatal("expired")
	}
	server := *leaf
	server.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	if clientCertMatches([]*x509.Certificate{&server}, nil, now) {
		t.Fatal("server certificate")
	}
}

func TestECDSAASN1(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	digest := sha256.Sum256([]byte("hello"))
	r, s, _ := ecdsa.Sign(rand.Reader, key, digest[:])
	raw := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	der, err := ecdsaASN1(raw)
	if err != nil || !ecdsa.VerifyASN1(&key.PublicKey, digest[:], der) {
		t.Fatalf("%v", err)
	}
	if _, err := ecdsaASN1([]byte{1, 2, 3}); err == nil {
		t.Fatal("odd length")
	}
}
