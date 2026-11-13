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
*/
import "C"

import (
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
