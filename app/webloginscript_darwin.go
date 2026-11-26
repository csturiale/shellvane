//go:build darwin && cgo

package app

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Foundation -framework AppKit -framework WebKit
#import <AppKit/AppKit.h>
#import <WebKit/WebKit.h>
#include <stdlib.h>

static WKWebView *shellvane_wl_find(NSView *view) {
	if ([view isKindOfClass:[WKWebView class]]) {
		return (WKWebView *)view;
	}
	for (NSView *sub in view.subviews) {
		WKWebView *found = shellvane_wl_find(sub);
		if (found != nil) {
			return found;
		}
	}
	return nil;
}

// Adds a script run at the start of every page (and frame) of the window's
// web view, before the page's own scripts. 1: added, 0: no web view.
static int shellvane_wl_add_script(void *window, const char *source) {
	NSWindow *w = (__bridge NSWindow *)window;
	WKWebView *webView = shellvane_wl_find(w.contentView);
	if (webView == nil) {
		return 0;
	}
	WKUserScript *script = [[WKUserScript alloc] initWithSource:[NSString stringWithUTF8String:source]
		injectionTime:WKUserScriptInjectionTimeAtDocumentStart forMainFrameOnly:NO];
	[webView.configuration.userContentController addUserScript:script];
	return 1;
}
// The page's address and text (JSON: {"u":…,"t":…}, malloc'd), read by
// WebKit itself: not through Wails' ExecJS, which waits for a runtime that
// another site's pages never load. Called off the main thread; NULL when
// there is no web view or no answer within two seconds.
static char *shellvane_wl_read(void *window) {
	__block char *out = NULL;
	dispatch_semaphore_t done = dispatch_semaphore_create(0);
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *w = (__bridge NSWindow *)window;
		WKWebView *webView = shellvane_wl_find(w.contentView);
		if (webView == nil) {
			dispatch_semaphore_signal(done);
			return;
		}
		[webView evaluateJavaScript:@"JSON.stringify({u: String(location.href), t: document.body ? String(document.body.innerText || '').slice(0, 8192) : ''})"
			completionHandler:^(id result, NSError *error) {
				if ([result isKindOfClass:[NSString class]]) {
					out = strdup([(NSString *)result UTF8String]);
				}
				dispatch_semaphore_signal(done);
			}];
	});
	if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 2 * NSEC_PER_SEC)) != 0) {
		return NULL;
	}
	return out;
}
*/
import "C"

import (
	"encoding/json"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// addDocumentStartScript runs source at the start of every page the window
// loads from now on, before the page's own scripts (Wails' JS option runs
// after the page has loaded). False when it cannot be added yet.
func addDocumentStartScript(win *application.WebviewWindow, source string) bool {
	ok := false
	application.InvokeSync(func() {
		native := win.NativeWindow()
		if native == nil {
			return
		}
		cs := C.CString(source)
		defer C.free(unsafe.Pointer(cs))
		ok = C.shellvane_wl_add_script(native, cs) == 1
	})
	return ok
}

// readPage returns the address and the text of the page the window shows,
// read by WebKit (false when it cannot be read now).
func readPage(win *application.WebviewWindow) (string, string, bool) {
	var native unsafe.Pointer
	application.InvokeSync(func() { native = win.NativeWindow() })
	if native == nil {
		return "", "", false
	}
	out := C.shellvane_wl_read(native)
	if out == nil {
		return "", "", false
	}
	defer C.free(unsafe.Pointer(out))
	var page struct{ U, T string }
	if json.Unmarshal([]byte(C.GoString(out)), &page) != nil {
		return "", "", false
	}
	return page.U, page.T, true
}
