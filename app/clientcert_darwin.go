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

import "log"

// EnableClientCertificates lets the app's web windows answer a site's
// request for a client certificate as a browser does: the certificate the
// site accepts, or the person's choice when several match. On a Mac
// registered with Intune this is the device certificate Microsoft Entra
// asks for. macOS asks once whether Shellvane may use the key.
func EnableClientCertificates() {
	switch C.shellvane_cc_install() {
	case 1:
	case 2:
		log.Printf("client certificates: the web view already handles certificate requests")
	default:
		log.Printf("client certificates: web view delegate not found")
	}
}
