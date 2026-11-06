package app

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/i18n"
	"github.com/csturiale/shellvane/core/vault"
)

// trayRecent is how many recent hosts the tray lists.
const trayRecent = 8

// TrayModel is what the tray menu shows.
type TrayModel struct {
	Recent  []TrayItem
	Tunnels []TrayItem // Active: running (or starting, reconnecting)
	Vault   vault.State
}

// TrayItem is a host or a tunnel in the tray.
type TrayItem struct {
	ID, Name string
	Active   bool
}

// Tray is the system tray icon: recent hosts, tunnels to start or stop
// without opening a window, and the vault lock.
type Tray struct {
	hosts   *hosts.Repo
	tunnels *TunnelService
	vault   *VaultService
	control *Control

	mu    sync.Mutex
	tr    *i18n.Translator
	tray  *application.SystemTray
	shown TrayModel
	built bool
}

// NewTray returns a tray; Start shows it.
func NewTray(repo *hosts.Repo, tunnels *TunnelService, vaultSvc *VaultService, control *Control, tr *i18n.Translator) *Tray {
	return &Tray{hosts: repo, tunnels: tunnels, vault: vaultSvc, control: control, tr: tr}
}

// Start creates the tray icon and keeps its menu current.
func (t *Tray) Start(a *application.App, icon []byte) {
	t.mu.Lock()
	t.tray = a.SystemTray.New()
	t.tray.SetIcon(icon)
	t.tray.SetTooltip(t.tr.T("app.name", nil))
	t.mu.Unlock()
	t.Refresh()
	go func() {
		for range time.Tick(5 * time.Second) {
			t.Refresh()
		}
	}()
}

// SetTranslator rebuilds the menu in another language.
func (t *Tray) SetTranslator(tr *i18n.Translator) {
	t.mu.Lock()
	t.tr = tr
	t.built = false
	t.mu.Unlock()
	t.Refresh()
}

// Refresh rebuilds the menu when what it shows changed.
func (t *Tray) Refresh() {
	m, err := t.model(context.Background())
	if err != nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tray == nil || (t.built && reflect.DeepEqual(m, t.shown)) {
		return
	}
	t.shown, t.built = m, true
	t.tray.SetMenu(t.menu(m))
}

func (t *Tray) model(ctx context.Context) (TrayModel, error) {
	var m TrayModel
	inv, err := t.hosts.List(ctx)
	if err != nil {
		return m, err
	}
	m.Recent = recentHosts(inv.Hosts, trayRecent)
	list, err := t.tunnels.List(ctx)
	if err != nil {
		return m, err
	}
	for _, v := range list {
		active := v.Status.State == TunnelRunning || v.Status.State == TunnelStarting || v.Status.State == TunnelReconnecting
		m.Tunnels = append(m.Tunnels, TrayItem{ID: v.Profile.ID, Name: v.Profile.Name, Active: active})
	}
	st, _ := t.vault.Status(ctx)
	m.Vault = st.State
	return m, nil
}

// recentHosts are the hosts used most recently, newest first.
func recentHosts(list []hosts.Host, n int) []TrayItem {
	used := make([]hosts.Host, 0, len(list))
	for _, h := range list {
		if h.LastUsed != "" {
			used = append(used, h)
		}
	}
	sort.SliceStable(used, func(i, j int) bool { return used[i].LastUsed > used[j].LastUsed })
	var out []TrayItem
	for _, h := range used[:min(n, len(used))] {
		out = append(out, TrayItem{ID: h.ID, Name: h.Name})
	}
	return out
}

func (t *Tray) menu(m TrayModel) *application.Menu {
	menu := application.NewMenu()
	menu.Add(t.tr.T("tray.openWindow", nil)).OnClick(func(*application.Context) { t.control.Show() })
	menu.AddSeparator()
	menu.AddSectionHeader(t.tr.T("tray.recent", nil))
	if len(m.Recent) == 0 {
		menu.Add(t.tr.T("tray.noRecent", nil)).SetEnabled(false)
	}
	for _, h := range m.Recent {
		id := h.ID
		menu.Add(h.Name).OnClick(func(*application.Context) { _ = t.control.Open(context.Background(), id) })
	}
	menu.AddSeparator()
	active := 0
	for _, tn := range m.Tunnels {
		if tn.Active {
			active++
		}
	}
	menu.AddSectionHeader(t.tr.N("tray.activeTunnels", active, nil))
	if len(m.Tunnels) == 0 {
		menu.Add(t.tr.T("tray.noTunnels", nil)).SetEnabled(false)
	}
	for _, tn := range m.Tunnels {
		id, on := tn.ID, tn.Active
		menu.AddCheckbox(tn.Name, on).OnClick(func(*application.Context) {
			if on {
				t.tunnels.Stop(id)
			} else {
				_ = t.tunnels.Start(context.Background(), id)
			}
			go func() { time.Sleep(300 * time.Millisecond); t.Refresh() }()
		})
	}
	menu.AddSeparator()
	if m.Vault == vault.StateUnlocked {
		menu.Add(t.tr.T("tray.lockVault", nil)).OnClick(func(*application.Context) {
			t.vault.Lock(context.Background())
			t.Refresh()
		})
	}
	menu.Add(t.tr.T("menu.quit", nil)).OnClick(func(*application.Context) { application.Get().Quit() })
	return menu
}
