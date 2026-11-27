package app

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// WindowState is where the main window was: its bounds when not
// maximised (in the screens' coordinates, so the screen it was on), and
// whether it was maximised.
type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximised bool `json:"maximised"`
}

// LoadWindowState reads the saved state; false when there is none.
func LoadWindowState(path string) (WindowState, bool) {
	var s WindowState
	b, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(b, &s) != nil || s.Width <= 0 || s.Height <= 0 {
		return WindowState{}, false
	}
	return s, true
}

func saveWindowState(path string, s WindowState) {
	b, _ := json.Marshal(s)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		tmp := path + ".tmp"
		if os.WriteFile(tmp, b, 0o600) == nil {
			os.Rename(tmp, path)
		}
	}
}

// visibleOn tells whether enough of the saved bounds (a corner of at least
// 120×80, enough to drag the window) lies on one of the screens' work
// areas: a screen unplugged since, or a smaller one, leaves the window
// where the system puts it.
func visibleOn(s WindowState, areas []application.Rect) bool {
	for _, a := range areas {
		w := min(s.X+s.Width, a.X+a.Width) - max(s.X, a.X)
		h := min(s.Y+s.Height, a.Y+a.Height) - max(s.Y, a.Y)
		if w >= 120 && h >= 80 {
			return true
		}
	}
	return false
}

// RememberWindow puts win back where it was at the last run (size, screen,
// maximised) once the app has started, and saves where it is whenever it
// is moved, resized, maximised or closed.
func RememberWindow(a *application.App, win *application.WebviewWindow, path string) {
	var mu sync.Mutex
	saved, _ := LoadWindowState(path)
	var timer *time.Timer
	save := func(now bool) {
		mu.Lock()
		defer mu.Unlock()
		if win.IsMinimised() || win.IsFullscreen() {
			return
		}
		if win.IsMaximised() {
			saved.Maximised = true // the bounds stay the ones before
		} else {
			b := win.Bounds()
			if b.Width <= 0 || b.Height <= 0 {
				return
			}
			saved = WindowState{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
		}
		s := saved
		if timer != nil {
			timer.Stop()
		}
		if now {
			saveWindowState(path, s)
			return
		}
		timer = time.AfterFunc(500*time.Millisecond, func() { saveWindowState(path, s) })
	}
	restore := func() {
		s, ok := LoadWindowState(path)
		if !ok {
			return
		}
		var areas []application.Rect
		for _, sc := range a.Screen.GetAll() {
			areas = append(areas, sc.WorkArea)
		}
		if visibleOn(s, areas) {
			win.SetBounds(application.Rect{X: s.X, Y: s.Y, Width: s.Width, Height: s.Height})
		} else {
			log.Printf("window: the screen it was on is not there, left where the system puts it")
		}
		if s.Maximised {
			win.Maximise()
		}
	}
	a.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) { restore() })
	for _, ev := range []events.WindowEventType{events.Common.WindowDidMove, events.Common.WindowDidResize, events.Common.WindowMaximise, events.Common.WindowUnMaximise} {
		win.OnWindowEvent(ev, func(*application.WindowEvent) { save(false) })
	}
	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) { save(true) })
}
