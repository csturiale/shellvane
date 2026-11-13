//go:build darwin && cgo

package app

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework WebKit -framework Security -framework SecurityInterface -framework AppKit
#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>
#import <Security/Security.h>
#import <SecurityInterface/SFChooseIdentityPanel.h>
#import <objc/runtime.h>
#include <stdlib.h>
#include <string.h>

// The identity chosen for each host while the app runs, so a site that asks
// again (every sign-in does) is not asked about twice.
static NSMutableDictionary *shellvane_cc_chosen;

// The identities of the keychains whose certificate was issued by one of
// the authorities the server accepts (all of them when it names none).
static NSArray *shellvane_cc_candidates(NSArray<NSData *> *issuers) {
	NSMutableDictionary *query = [@{
		(__bridge id)kSecClass: (__bridge id)kSecClassIdentity,
		(__bridge id)kSecReturnRef: @YES,
		(__bridge id)kSecMatchLimit: (__bridge id)kSecMatchLimitAll,
	} mutableCopy];
	if (issuers.count > 0) {
		query[(__bridge id)kSecMatchIssuers] = issuers;
	}
	CFTypeRef result = NULL;
	if (SecItemCopyMatching((__bridge CFDictionaryRef)query, &result) != errSecSuccess || result == NULL) {
		return @[];
	}
	return CFBridgingRelease(result);
}

static void shellvane_cc_use(NSString *host, id identity,
		void (^completionHandler)(NSURLSessionAuthChallengeDisposition, NSURLCredential *)) {
	shellvane_cc_chosen[host] = identity;
	NSURLCredential *credential = [NSURLCredential credentialWithIdentity:(__bridge SecIdentityRef)identity
		certificates:nil persistence:NSURLCredentialPersistenceForSession];
	completionHandler(NSURLSessionAuthChallengeUseCredential, credential);
}

// WKNavigationDelegate's client certificate challenge, answered as a
// browser does: with the one identity the server accepts, or the one the
// person picks in the system's chooser when several match. It is what
// sign-ins that check the device need (Microsoft Entra on a Mac registered
// with Intune asks for the device certificate). Every other challenge,
// server trust included, gets WebKit's default handling.
static void shellvane_cc_challenge(id self, SEL _cmd, WKWebView *webView, NSURLAuthenticationChallenge *challenge,
		void (^completionHandler)(NSURLSessionAuthChallengeDisposition, NSURLCredential *)) {
	NSURLProtectionSpace *space = challenge.protectionSpace;
	if (![space.authenticationMethod isEqualToString:NSURLAuthenticationMethodClientCertificate]) {
		completionHandler(NSURLSessionAuthChallengePerformDefaultHandling, nil);
		return;
	}
	NSString *host = space.host.lowercaseString ?: @"";
	if (shellvane_cc_chosen == nil) {
		shellvane_cc_chosen = [NSMutableDictionary dictionary];
	}
	NSArray *candidates = shellvane_cc_candidates(space.distinguishedNames);
	id remembered = shellvane_cc_chosen[host];
	if (remembered != nil && [candidates containsObject:remembered]) {
		shellvane_cc_use(host, remembered, completionHandler);
		return;
	}
	if (candidates.count == 0) {
		completionHandler(NSURLSessionAuthChallengePerformDefaultHandling, nil);
		return;
	}
	if (candidates.count == 1) {
		shellvane_cc_use(host, candidates[0], completionHandler);
		return;
	}
	// Several: the system's chooser, as Safari shows it. After this callback
	// returns, so WebKit is not held meanwhile.
	dispatch_async(dispatch_get_main_queue(), ^{
		SFChooseIdentityPanel *panel = [SFChooseIdentityPanel sharedChooseIdentityPanel];
		// The panel's own text is macOS's, in the system's language; the
		// message only names the site that asks.
		if ([panel runModalForIdentities:candidates message:host] == NSModalResponseOK && panel.identity != NULL) {
			shellvane_cc_use(host, (__bridge id)panel.identity, completionHandler);
		} else {
			completionHandler(NSURLSessionAuthChallengePerformDefaultHandling, nil);
		}
	});
}

// For the app's own HTTPS calls (plugins): the identity to answer host
// with, retained, NULL for none. Same choice as in the web windows (a
// sign-in there already picked one for the host), on the main thread
// where the chooser can show.
static void *shellvane_cc_pick(const char *chost, const void **issuers, const int *lens, int n) {
	NSString *host = [[NSString stringWithUTF8String:chost] lowercaseString] ?: @"";
	NSMutableArray *list = [NSMutableArray array];
	for (int i = 0; i < n; i++) {
		[list addObject:[NSData dataWithBytes:issuers[i] length:lens[i]]];
	}
	__block id chosen = nil;
	void (^pick)(void) = ^{
		if (shellvane_cc_chosen == nil) {
			shellvane_cc_chosen = [NSMutableDictionary dictionary];
		}
		NSArray *candidates = shellvane_cc_candidates(list);
		id remembered = shellvane_cc_chosen[host];
		if (remembered != nil && [candidates containsObject:remembered]) {
			chosen = remembered;
			return;
		}
		if (candidates.count == 1) {
			chosen = candidates[0];
		} else if (candidates.count > 1) {
			SFChooseIdentityPanel *panel = [SFChooseIdentityPanel sharedChooseIdentityPanel];
			if ([panel runModalForIdentities:candidates message:host] == NSModalResponseOK && panel.identity != NULL) {
				chosen = (__bridge id)panel.identity;
			}
		}
		if (chosen != nil) {
			shellvane_cc_chosen[host] = chosen;
		}
	};
	if ([NSThread isMainThread]) {
		pick();
	} else {
		dispatch_sync(dispatch_get_main_queue(), pick);
	}
	return chosen != nil ? (__bridge_retained void *)chosen : NULL;
}

static void shellvane_cc_release(void *ref) {
	if (ref != NULL) {
		CFRelease((CFTypeRef)ref);
	}
}

// The identity's certificate (DER, malloc'd) and private key (retained).
static int shellvane_cc_parts(void *identity, void **der, int *len, void **key) {
	SecCertificateRef cert = NULL;
	if (SecIdentityCopyCertificate((SecIdentityRef)identity, &cert) != errSecSuccess || cert == NULL) {
		return 0;
	}
	NSData *data = CFBridgingRelease(SecCertificateCopyData(cert));
	CFRelease(cert);
	SecKeyRef k = NULL;
	if (data == nil || SecIdentityCopyPrivateKey((SecIdentityRef)identity, &k) != errSecSuccess || k == NULL) {
		return 0;
	}
	*der = malloc(data.length);
	memcpy(*der, data.bytes, data.length);
	*len = (int)data.length;
	*key = k;
	return 1;
}

// Signs a digest with the key: algorithm by the number Go passes (see
// keychainAlgorithm). The signature is malloc'd; on failure *message is.
static void *shellvane_cc_sign(void *key, int alg, const void *digest, int dlen, int *outLen, char **message) {
	SecKeyAlgorithm algorithms[] = {
		kSecKeyAlgorithmRSASignatureDigestPKCS1v15SHA1, kSecKeyAlgorithmRSASignatureDigestPKCS1v15SHA256,
		kSecKeyAlgorithmRSASignatureDigestPKCS1v15SHA384, kSecKeyAlgorithmRSASignatureDigestPKCS1v15SHA512,
		kSecKeyAlgorithmRSASignatureDigestPSSSHA256, kSecKeyAlgorithmRSASignatureDigestPSSSHA384,
		kSecKeyAlgorithmRSASignatureDigestPSSSHA512, kSecKeyAlgorithmECDSASignatureDigestX962SHA1,
		kSecKeyAlgorithmECDSASignatureDigestX962SHA256, kSecKeyAlgorithmECDSASignatureDigestX962SHA384,
		kSecKeyAlgorithmECDSASignatureDigestX962SHA512,
	};
	if (alg < 0 || alg >= (int)(sizeof(algorithms) / sizeof(algorithms[0]))) {
		*message = strdup("unsupported signature algorithm");
		return NULL;
	}
	CFErrorRef err = NULL;
	NSData *input = [NSData dataWithBytes:digest length:dlen];
	NSData *sig = CFBridgingRelease(SecKeyCreateSignature((SecKeyRef)key, algorithms[alg], (__bridge CFDataRef)input, &err));
	if (sig == nil) {
		NSError *e = CFBridgingRelease(err);
		*message = strdup((e.localizedDescription ?: @"the key did not sign").UTF8String);
		return NULL;
	}
	void *out = malloc(sig.length);
	memcpy(out, sig.bytes, sig.length);
	*outLen = (int)sig.length;
	return out;
}

// Adds the challenge handler to Wails' WKNavigationDelegate class, which has
// none. 1: added, 2: the class has one already (left alone), 0: no class.
static int shellvane_cc_install(void) {
	Class cls = objc_getClass("WebviewWindowDelegate");
	if (cls == Nil) {
		return 0;
	}
	SEL sel = @selector(webView:didReceiveAuthenticationChallenge:completionHandler:);
	if (class_getInstanceMethod(cls, sel) != NULL) {
		return 2;
	}
	return class_addMethod(cls, sel, (IMP)shellvane_cc_challenge, "v@:@@@?") ? 1 : 0;
}
*/
import "C"

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"sync"
	"unsafe"

	"github.com/csturiale/shellvane/core/plugins"
)

// EnableClientCertificates lets the app's web windows answer a site's
// request for a client certificate as a browser does: the certificate the
// site accepts, or the person's choice when several match. On a Mac
// registered with Intune this is the device certificate Microsoft Entra
// asks for. macOS asks once whether Shellvane may use the key.
//
// The plugins' own HTTPS calls (the CyberArk API after a sign-in in such a
// window) answer with the same certificate, signing with the keychain's
// key, which never leaves it.
func EnableClientCertificates() {
	plugins.SystemClientCertificate = keychainCertificate
	switch C.shellvane_cc_install() {
	case 1:
	case 2:
		log.Printf("client certificates: the web view already handles certificate requests")
	default:
		log.Printf("client certificates: web view delegate not found")
	}
}

// The certificates found for each host while the app runs.
var keychainCerts = struct {
	sync.Mutex
	byHost map[string]*tls.Certificate
}{byHost: map[string]*tls.Certificate{}}

// keychainCertificate is plugins.SystemClientCertificate on macOS.
func keychainCertificate(host string, acceptableCAs [][]byte) (*tls.Certificate, error) {
	keychainCerts.Lock()
	defer keychainCerts.Unlock()
	if c := keychainCerts.byHost[host]; c != nil {
		return c, nil
	}
	n := len(acceptableCAs)
	issuers := C.malloc(C.size_t(max(n, 1)) * C.size_t(unsafe.Sizeof(uintptr(0))))
	lens := C.malloc(C.size_t(max(n, 1)) * C.size_t(unsafe.Sizeof(C.int(0))))
	defer C.free(issuers)
	defer C.free(lens)
	ptrs := unsafe.Slice((*unsafe.Pointer)(issuers), max(n, 1))
	sizes := unsafe.Slice((*C.int)(lens), max(n, 1))
	for i, ca := range acceptableCAs {
		ptrs[i] = C.CBytes(ca)
		sizes[i] = C.int(len(ca))
		defer C.free(ptrs[i])
	}
	chost := C.CString(host)
	defer C.free(unsafe.Pointer(chost))
	identity := C.shellvane_cc_pick(chost, (*unsafe.Pointer)(issuers), (*C.int)(lens), C.int(n))
	if identity == nil {
		return nil, nil
	}
	defer C.shellvane_cc_release(identity)
	var der, key unsafe.Pointer
	var size C.int
	if C.shellvane_cc_parts(identity, &der, &size, &key) == 0 {
		return nil, errors.New("keychain identity without certificate or key")
	}
	raw := C.GoBytes(der, size)
	C.free(der)
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		C.shellvane_cc_release(key)
		return nil, err
	}
	// The key stays retained while the app runs: the certificate is reused.
	c := &tls.Certificate{Certificate: [][]byte{raw}, Leaf: leaf, PrivateKey: &keychainSigner{key: key, pub: leaf.PublicKey}}
	keychainCerts.byHost[host] = c
	return c, nil
}

// keychainSigner signs TLS handshakes with a keychain key.
type keychainSigner struct {
	key unsafe.Pointer // SecKeyRef
	pub crypto.PublicKey
}

func (s *keychainSigner) Public() crypto.PublicKey { return s.pub }

func (s *keychainSigner) Sign(_ io.Reader, digest []byte, opts crypto.SignerOpts) ([]byte, error) {
	alg, err := keychainAlgorithm(s.pub, opts)
	if err != nil {
		return nil, err
	}
	var size C.int
	var message *C.char
	d := C.CBytes(digest)
	defer C.free(d)
	sig := C.shellvane_cc_sign(s.key, C.int(alg), d, C.int(len(digest)), &size, &message)
	if sig == nil {
		defer C.free(unsafe.Pointer(message))
		return nil, errors.New("keychain: " + C.GoString(message))
	}
	defer C.free(sig)
	return C.GoBytes(sig, size), nil
}
