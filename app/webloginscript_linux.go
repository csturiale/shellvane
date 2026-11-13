//go:build linux && cgo && !gtk3 && !android && !server

package app

/*
#cgo linux pkg-config: gtk4 webkitgtk-6.0
#include <gtk/gtk.h>
#include <webkit/webkit.h>
#include <stdlib.h>

static WebKitWebView *shellvane_wl_find(GtkWidget *widget) {
	if (WEBKIT_IS_WEB_VIEW(widget)) {
		return WEBKIT_WEB_VIEW(widget);
	}
	for (GtkWidget *child = gtk_widget_get_first_child(widget); child != NULL; child = gtk_widget_get_next_sibling(child)) {
		WebKitWebView *found = shellvane_wl_find(child);
		if (found != NULL) {
			return found;
		}
	}
	return NULL;
}

// Adds a script run at the start of every page (and frame) of the window's
// web view, before the page's own scripts. 1: added, 0: no web view.
static int shellvane_wl_add_script(void *window, const char *source) {
	WebKitWebView *view = shellvane_wl_find(GTK_WIDGET(window));
	if (view == NULL) {
		return 0;
	}
	WebKitUserScript *script = webkit_user_script_new(source, WEBKIT_USER_CONTENT_INJECT_ALL_FRAMES,
		WEBKIT_USER_SCRIPT_INJECT_AT_DOCUMENT_START, NULL, NULL);
	webkit_user_content_manager_add_script(webkit_web_view_get_user_content_manager(view), script);
	webkit_user_script_unref(script);
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
