//go:build windows

package app

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/csturiale/shellvane/core/plugins"
)

// EnableClientCertificates lets the plugins' own HTTPS calls (the
// CyberArk API after a sign-in) answer a server's request for a client
// certificate as the browser does: with the certificate of the user's
// store (Personal) that the server accepts, or the one the user picks in
// the system's chooser when several match; signing with the key where
// Windows keeps it (TPM, smart card, software), which never leaves it.
// The sign-in windows need nothing: WebView2 shows the chooser by itself.
func EnableClientCertificates() {
	plugins.SystemClientCertificate = storeCertificate
}

var (
	crypt32                               = windows.NewLazySystemDLL("crypt32.dll")
	procCertGetCertificateContextProperty = crypt32.NewProc("CertGetCertificateContextProperty")
	cryptui                               = windows.NewLazySystemDLL("cryptui.dll")
	procSelectCertificateFromStore        = cryptui.NewProc("CryptUIDlgSelectCertificateFromStore")
	ncrypt                                = windows.NewLazySystemDLL("ncrypt.dll")
	procNCryptSignHash                    = ncrypt.NewProc("NCryptSignHash")
)

const (
	certKeyProvInfoPropID    = 2
	cryptAcquireCacheFlag    = 0x1
	cryptAcquirePreferNCrypt = 0x00020000
	bcryptPadPKCS1           = 0x2
	bcryptPadPSS             = 0x8
)

// The certificates chosen for each host while the app runs.
var storeCerts = struct {
	sync.Mutex
	byHost map[string]*tls.Certificate
}{byHost: map[string]*tls.Certificate{}}

// storeCertificate is plugins.SystemClientCertificate on Windows.
func storeCertificate(host string, acceptableCAs [][]byte) (*tls.Certificate, error) {
	storeCerts.Lock()
	defer storeCerts.Unlock()
	if c := storeCerts.byHost[host]; c != nil {
		return c, nil
	}
	store, err := windows.CertOpenSystemStore(0, windows.StringToUTF16Ptr("MY"))
	if err != nil {
		return nil, err
	}
	defer windows.CertCloseStore(store, 0)
	var candidates []*windows.CertContext
	defer func() {
		for _, c := range candidates {
			windows.CertFreeCertificateContext(c)
		}
	}()
	now := time.Now()
	var prev *windows.CertContext
	for {
		cur, err := windows.CertEnumCertificatesInStore(store, prev)
		if err != nil || cur == nil {
			break
		}
		prev = cur
		if hasPrivateKey(cur) && clientCertMatches(chainOf(cur), acceptableCAs, now) {
			candidates = append(candidates, windows.CertDuplicateCertificateContext(cur))
		}
	}
	var chosen *windows.CertContext
	switch len(candidates) {
	case 0:
		return nil, nil
	case 1:
		chosen = candidates[0]
	default:
		if chosen = chooseCertificate(candidates, host); chosen == nil {
			return nil, nil
		}
	}
	c, err := certificateWithKey(chosen)
	if err != nil {
		return nil, err
	}
	storeCerts.byHost[host] = c
	return c, nil
}

// der is the certificate's encoding (a copy).
func der(c *windows.CertContext) []byte {
	return append([]byte(nil), unsafe.Slice(c.EncodedCert, c.Length)...)
}

func hasPrivateKey(c *windows.CertContext) bool {
	var size uint32
	r, _, _ := procCertGetCertificateContextProperty.Call(uintptr(unsafe.Pointer(c)), certKeyProvInfoPropID, 0, uintptr(unsafe.Pointer(&size)))
	return r != 0 && size > 0
}

// chainOf is the certificate and the authorities Windows finds above it
// (just the certificate when it cannot build the chain).
func chainOf(c *windows.CertContext) []*x509.Certificate {
	leaf, err := x509.ParseCertificate(der(c))
	if err != nil {
		return nil
	}
	out := []*x509.Certificate{leaf}
	para := windows.CertChainPara{Size: uint32(unsafe.Sizeof(windows.CertChainPara{}))}
	var chain *windows.CertChainContext
	if windows.CertGetCertificateChain(0, c, nil, c.Store, &para, 0, 0, &chain) != nil || chain == nil {
		return out
	}
	defer windows.CertFreeCertificateChain(chain)
	if chain.ChainCount == 0 {
		return out
	}
	simple := unsafe.Slice(chain.Chains, chain.ChainCount)[0]
	for i, el := range unsafe.Slice(simple.Elements, simple.NumElements) {
		if i == 0 || el.CertContext == nil {
			continue
		}
		if ca, err := x509.ParseCertificate(der(el.CertContext)); err == nil {
			out = append(out, ca)
		}
	}
	return out
}

// chooseCertificate shows the system's chooser with the candidates; nil
// when the user cancels or it cannot show.
func chooseCertificate(candidates []*windows.CertContext, host string) *windows.CertContext {
	if procSelectCertificateFromStore.Find() != nil {
		return candidates[0]
	}
	mem, err := windows.CertOpenStore(windows.CERT_STORE_PROV_MEMORY, 0, 0, 0, 0)
	if err != nil {
		return candidates[0]
	}
	defer windows.CertCloseStore(mem, 0)
	for _, c := range candidates {
		windows.CertAddCertificateContextToStore(mem, c, windows.CERT_STORE_ADD_ALWAYS, nil)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	r, _, _ := procSelectCertificateFromStore.Call(uintptr(mem), 0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr("Shellvane"))),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(host))), 0, 0, 0)
	if r == 0 {
		return nil
	}
	picked := *(**windows.CertContext)(unsafe.Pointer(&r)) // a C pointer
	defer windows.CertFreeCertificateContext(picked)
	want := string(der(picked))
	for _, c := range candidates {
		if string(der(c)) == want {
			return c
		}
	}
	return nil
}

// certificateWithKey opens the certificate's key (Windows may ask for a
// smart card's PIN) for TLS signatures.
func certificateWithKey(c *windows.CertContext) (*tls.Certificate, error) {
	raw := der(c)
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return nil, err
	}
	var key windows.Handle
	var spec uint32
	var mustFree bool
	if err := windows.CryptAcquireCertificatePrivateKey(c, cryptAcquireCacheFlag|cryptAcquirePreferNCrypt, nil, &key, &spec, &mustFree); err != nil {
		return nil, fmt.Errorf("certificate key: %w", err)
	}
	if spec != windows.CERT_NCRYPT_KEY_SPEC {
		return nil, errors.New("certificate key: only CNG keys can sign (legacy CryptoAPI provider)")
	}
	// The key stays open while the app runs: the certificate is reused
	// (cached by Windows too, CRYPT_ACQUIRE_CACHE_FLAG).
	return &tls.Certificate{Certificate: [][]byte{raw}, Leaf: leaf, PrivateKey: &cngSigner{key: key, pub: leaf.PublicKey}}, nil
}

// cngSigner signs TLS handshakes with a CNG key.
type cngSigner struct {
	key windows.Handle
	pub crypto.PublicKey
}

func (s *cngSigner) Public() crypto.PublicKey { return s.pub }

func (s *cngSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	names := map[crypto.Hash]string{crypto.SHA1: "SHA1", crypto.SHA256: "SHA256", crypto.SHA384: "SHA384", crypto.SHA512: "SHA512"}
	name, ok := names[opts.HashFunc()]
	if !ok {
		return nil, errors.New("certificate key: unsupported hash")
	}
	alg := windows.StringToUTF16Ptr(name)
	var padding unsafe.Pointer
	var flags uint32
	switch s.pub.(type) {
	case *rsa.PublicKey:
		if _, pss := opts.(*rsa.PSSOptions); pss {
			padding = unsafe.Pointer(&struct {
				alg  *uint16
				salt uint32
			}{alg, uint32(opts.HashFunc().Size())})
			flags = bcryptPadPSS
		} else {
			padding = unsafe.Pointer(&struct{ alg *uint16 }{alg})
			flags = bcryptPadPKCS1
		}
	case *ecdsa.PublicKey:
	default:
		return nil, errors.New("certificate key: unsupported key type")
	}
	var size uint32
	if r, _, _ := procNCryptSignHash.Call(uintptr(s.key), uintptr(padding), uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)), 0, 0, uintptr(unsafe.Pointer(&size)), uintptr(flags)); r != 0 {
		return nil, fmt.Errorf("certificate key: NCryptSignHash 0x%x", uint32(r))
	}
	sig := make([]byte, size)
	if r, _, _ := procNCryptSignHash.Call(uintptr(s.key), uintptr(padding), uintptr(unsafe.Pointer(&digest[0])), uintptr(len(digest)), uintptr(unsafe.Pointer(&sig[0])), uintptr(size), uintptr(unsafe.Pointer(&size)), uintptr(flags)); r != 0 {
		return nil, fmt.Errorf("certificate key: NCryptSignHash 0x%x", uint32(r))
	}
	runtime.KeepAlive(alg)
	sig = sig[:size]
	if _, ec := s.pub.(*ecdsa.PublicKey); ec {
		return ecdsaASN1(sig)
	}
	return sig, nil
}
