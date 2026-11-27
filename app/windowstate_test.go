package app

import (
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestWindowStateIsRestoredOnlyWhereVisible(t *testing.T) {
	path := filepath.Join(t.TempDir(), "window.json")
	if _, ok := LoadWindowState(path); ok {
		t.Fatal("no file, no state")
	}
	s := WindowState{X: 2000, Y: 100, Width: 1200, Height: 800, Maximised: true}
	saveWindowState(path, s)
	if got, ok := LoadWindowState(path); !ok || got != s {
		t.Fatalf("loaded %+v, %v", got, ok)
	}
	laptop := application.Rect{X: 0, Y: 0, Width: 1512, Height: 945}
	external := application.Rect{X: 1512, Y: 0, Width: 2560, Height: 1415}
	if !visibleOn(s, []application.Rect{laptop, external}) {
		t.Fatal("on the external screen")
	}
	if visibleOn(s, []application.Rect{laptop}) {
		t.Fatal("the external screen is unplugged: only 0 px would show")
	}
	// Partly off the edge: enough of it to grab is visible, or not.
	if !visibleOn(WindowState{X: 1300, Y: 100, Width: 800, Height: 600}, []application.Rect{laptop}) {
		t.Fatal("212 px on the laptop is enough")
	}
	if visibleOn(WindowState{X: 1450, Y: 100, Width: 800, Height: 600}, []application.Rect{laptop}) {
		t.Fatal("62 px on the laptop is not")
	}
}
