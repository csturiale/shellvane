package app

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/csturiale/shellvane/core/errs"
	"github.com/csturiale/shellvane/core/hosts"
	"github.com/csturiale/shellvane/core/plugins"
	"github.com/csturiale/shellvane/core/vault"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// PluginInfo is a plugin as the UI lists it.
type PluginInfo struct {
	plugins.Manifest
	// Missing names the command-line tools the plugin needs and this
	// computer lacks (looked up in PATH).
	Missing   []string `json:"missing"`
	Installed bool     `json:"installed"`
}

// StoreItem is a plugin of the store, with what is installed here.
type StoreItem struct {
	plugins.StoreEntry
	InstalledVersion string `json:"installedVersion"` // "" when not installed
}

// ErrPluginInUse: uninstalling needs the plugin's configurations deleted.
const ErrPluginInUse errs.Code = "plugins.inUse"

// PluginConfig is a configuration with which secret settings are stored.
type PluginConfig struct {
	plugins.Config
	SecretsSet []string `json:"secretsSet"`
}

// PluginService lists plugins, keeps their configurations (secrets in the
// vault) and refreshes inventories into host groups.
type PluginService struct {
	registry *plugins.Registry
	configs  *plugins.Configs
	hosts    *hosts.Repo
	vault    *vault.Vault
	lookPath func(string) (string, error)
	sessions *SessionService
	// root holds the external plugins, one folder each.
	root    string
	builtIn map[string]bool
}

// NewPluginService returns the service; external plugins are loaded from
// root (<data dir>/plugins).
func NewPluginService(registry *plugins.Registry, configs *plugins.Configs, repo *hosts.Repo, v *vault.Vault, root string) *PluginService {
	s := &PluginService{registry: registry, configs: configs, hosts: repo, vault: v, lookPath: exec.LookPath, root: root, builtIn: map[string]bool{}}
	for _, m := range registry.List() {
		s.builtIn[m.ID] = true
	}
	if root != "" {
		list, failed := plugins.LoadAll(root)
		for _, p := range list {
			if !s.builtIn[p.Manifest().ID] {
				registry.Add(p)
			}
		}
		for name, err := range failed {
			log.Printf("plugin %s: %v", name, err)
		}
	}
	return s
}

// Plugins lists the available plugins: built-in ones and those installed
// from a file or the store.
func (s *PluginService) Plugins(ctx context.Context) []PluginInfo {
	installed, _ := s.configs.Installed(ctx)
	var out []PluginInfo
	for _, m := range s.registry.List() {
		info := PluginInfo{Manifest: m, Missing: []string{}, Installed: installed[m.ID]}
		for _, tool := range m.Requires {
			if _, err := s.lookPath(tool); err != nil {
				info.Missing = append(info.Missing, tool)
			}
		}
		out = append(out, info)
	}
	return out
}

// Configs lists the configurations.
func (s *PluginService) Configs(ctx context.Context) ([]PluginConfig, error) {
	list, err := s.configs.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PluginConfig, 0, len(list))
	for _, c := range list {
		out = append(out, s.view(ctx, c))
	}
	return out, nil
}

func (s *PluginService) view(ctx context.Context, c plugins.Config) PluginConfig {
	v := PluginConfig{Config: c, SecretsSet: []string{}}
	if p, err := s.registry.Get(c.PluginID); err == nil && s.vault != nil {
		for _, st := range p.Manifest().Settings {
			if st.Secret {
				if has, _ := s.vault.HasCredential(ctx, plugins.SecretScope(c.ID, st.Key)); has {
					v.SecretsSet = append(v.SecretsSet, st.Key)
				}
			}
		}
	}
	return v
}

// installedPlugin returns an installed plugin.
func (s *PluginService) installedPlugin(ctx context.Context, id string) (plugins.Plugin, error) {
	p, err := s.registry.Get(id)
	if err != nil {
		return nil, err
	}
	if installed, _ := s.configs.Installed(ctx); !installed[id] {
		return nil, errs.New(plugins.ErrNotInstalled, errs.Params{"name": p.Manifest().Name})
	}
	return p, nil
}

// Install enables a built-in plugin.
func (s *PluginService) Install(ctx context.Context, id string) error {
	if _, err := s.registry.Get(id); err != nil {
		return err
	}
	return s.configs.SetInstalled(ctx, id, true)
}

// Uninstall disables a plugin; an external one is also deleted. Its
// configurations must be deleted first.
func (s *PluginService) Uninstall(ctx context.Context, id string) error {
	list, err := s.configs.List(ctx)
	if err != nil {
		return err
	}
	for _, c := range list {
		if c.PluginID == id {
			return errs.New(ErrPluginInUse, errs.Params{"name": c.Name})
		}
	}
	if err := s.configs.SetInstalled(ctx, id, false); err != nil {
		return err
	}
	if !s.builtIn[id] && s.root != "" {
		s.registry.Remove(id)
		return plugins.Uninstall(s.root, id)
	}
	return nil
}

// InstallFile installs an external plugin from a folder or a .zip.
func (s *PluginService) InstallFile(ctx context.Context, path string) (PluginInfo, error) {
	if s.root == "" {
		return PluginInfo{}, errs.New(plugins.ErrUnsupported, nil)
	}
	m, err := plugins.ReadManifestOf(path)
	if err == nil && s.builtIn[m.ID] {
		return PluginInfo{}, errs.New(plugins.ErrBuiltIn, errs.Params{"id": m.ID})
	}
	m, err = plugins.Install(path, s.root)
	if err != nil {
		return PluginInfo{}, err
	}
	p, err := plugins.LoadExternal(filepath.Join(s.root, m.ID))
	if err != nil {
		return PluginInfo{}, err
	}
	s.registry.Add(p)
	if err := s.configs.SetInstalled(ctx, m.ID, true); err != nil {
		return PluginInfo{}, err
	}
	for _, info := range s.Plugins(ctx) {
		if info.ID == m.ID {
			return info, nil
		}
	}
	return PluginInfo{Manifest: m, Installed: true}, nil
}

// Store lists the plugins of a store index (url is the index).
func (s *PluginService) Store(ctx context.Context, url string) ([]StoreItem, error) {
	idx, err := plugins.FetchIndex(ctx, url)
	if err != nil {
		return nil, err
	}
	installed, _ := s.configs.Installed(ctx)
	out := make([]StoreItem, 0, len(idx.Plugins))
	for _, e := range idx.Plugins {
		item := StoreItem{StoreEntry: e}
		if p, err := s.registry.Get(e.ID); err == nil && installed[e.ID] {
			item.InstalledVersion = p.Manifest().Version
		}
		out = append(out, item)
	}
	return out, nil
}

// InstallFromStore downloads a plugin of the store, checks its SHA-256 and
// installs it (an update replaces the installed version).
func (s *PluginService) InstallFromStore(ctx context.Context, url, id string) (PluginInfo, error) {
	idx, err := plugins.FetchIndex(ctx, url)
	if err != nil {
		return PluginInfo{}, err
	}
	for _, e := range idx.Plugins {
		if e.ID != id {
			continue
		}
		file, err := plugins.Download(ctx, e)
		if err != nil {
			return PluginInfo{}, err
		}
		defer os.Remove(file)
		return s.InstallFile(ctx, file)
	}
	return PluginInfo{}, errs.New(plugins.ErrUnknown, errs.Params{"id": id})
}

// SaveConfig creates or updates a configuration. secrets holds the secret
// settings typed now (empty values keep the saved ones); they go to the
// vault, which must be unlocked.
func (s *PluginService) SaveConfig(ctx context.Context, cfg plugins.Config, secrets map[string]string) (PluginConfig, error) {
	p, err := s.installedPlugin(ctx, cfg.PluginID)
	if err != nil {
		return PluginConfig{}, err
	}
	clean := map[string]string{}
	secret := map[string]bool{}
	for _, st := range p.Manifest().Settings {
		secret[st.Key] = st.Secret
		if !st.Secret {
			if v := strings.TrimSpace(cfg.Settings[st.Key]); v != "" {
				clean[st.Key] = v
			}
		}
	}
	cfg.Settings = clean
	cfg.GroupID, cfg.LastSync = "", ""
	if cfg.ID != "" {
		// Sync state is kept by the store, not taken from the UI.
		if old, err := s.configs.Get(ctx, cfg.ID); err == nil {
			cfg.GroupID, cfg.LastSync = old.GroupID, old.LastSync
		}
	}
	for key, value := range secrets {
		if value != "" && secret[key] && s.vault == nil {
			return PluginConfig{}, errs.New(vault.ErrNotSetUp, nil)
		}
	}
	saved, err := s.configs.Save(ctx, cfg)
	if err != nil {
		return PluginConfig{}, err
	}
	for key, value := range secrets {
		if value == "" || !secret[key] {
			continue
		}
		label := saved.Name + " · " + key
		if err := s.vault.SaveCredential(ctx, plugins.SecretScope(saved.ID, key), label, []byte(value)); err != nil {
			return s.view(ctx, saved), err
		}
	}
	return s.view(ctx, saved), nil
}

// DeleteConfig removes a configuration and its secrets; the hosts it
// created stay as ordinary hosts.
func (s *PluginService) DeleteConfig(ctx context.Context, id string) error {
	cfg, err := s.configs.Get(ctx, id)
	if err != nil {
		return err
	}
	if p, err := s.registry.Get(cfg.PluginID); err == nil && s.vault != nil {
		for _, st := range p.Manifest().Settings {
			if st.Secret {
				s.vault.DeleteCredential(ctx, plugins.SecretScope(id, st.Key))
			}
		}
	}
	return s.configs.Delete(ctx, id)
}

// Sync refreshes an inventory configuration into its host group.
func (s *PluginService) Sync(ctx context.Context, id string) (plugins.SyncResult, error) {
	cfg, err := s.configs.Get(ctx, id)
	if err != nil {
		return plugins.SyncResult{}, err
	}
	p, err := s.installedPlugin(ctx, cfg.PluginID)
	if err != nil {
		return plugins.SyncResult{}, err
	}
	settings, err := s.settings(ctx, p, cfg)
	if err != nil {
		return plugins.SyncResult{}, err
	}
	found, err := p.Inventory(ctx, settings)
	if err != nil {
		return plugins.SyncResult{}, err
	}
	return plugins.Sync(ctx, s.hosts, s.configs, cfg, found)
}

// settings merges the secret settings from the vault.
func (s *PluginService) settings(ctx context.Context, p plugins.Plugin, cfg plugins.Config) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range cfg.Settings {
		out[k] = v
	}
	for _, st := range p.Manifest().Settings {
		if !st.Secret || s.vault == nil {
			continue
		}
		scope := plugins.SecretScope(cfg.ID, st.Key)
		if has, _ := s.vault.HasCredential(ctx, scope); !has {
			continue
		}
		secret, found, err := s.vault.Credential(ctx, scope)
		if err != nil {
			return nil, err // locked: the UI asks to unlock
		}
		if found {
			out[st.Key] = string(secret)
			vault.Zero(secret)
		}
	}
	return out, nil
}

// pluginExecTimeout bounds a command a plugin runs on a host.
const pluginExecTimeout = 45 * time.Second

// EnablePanels lets panels and actions run commands on hosts over their
// pooled connections. It is not a method so that Wails does not expose it.
func EnablePanels(s *PluginService, sessions *SessionService) { s.sessions = sessions }

// PanelPlugins lists the plugins with a panel for connected hosts.
func (s *PluginService) PanelPlugins(ctx context.Context) []PluginInfo {
	out := []PluginInfo{}
	for _, p := range s.Plugins(ctx) {
		if p.Installed && (p.Has(plugins.CapPanel) || p.Has(plugins.CapActions)) {
			out = append(out, p)
		}
	}
	return out
}

// Panel shows a plugin's panel for a host.
func (s *PluginService) Panel(ctx context.Context, pluginID, hostID string) (plugins.Panel, error) {
	p, target, exec, release, err := s.onHost(ctx, pluginID, hostID)
	if err != nil {
		return plugins.Panel{}, err
	}
	defer release()
	return p.Panel(ctx, s.pluginSettings(ctx, p), target, exec)
}

// HostActions lists a plugin's actions for a host (outside its tables).
func (s *PluginService) HostActions(ctx context.Context, pluginID, hostID string) ([]plugins.Action, error) {
	p, err := s.installedPlugin(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	target, err := s.target(ctx, hostID)
	if err != nil {
		return nil, err
	}
	list, err := p.Actions(ctx, s.pluginSettings(ctx, p), target)
	if errs.CodeOf(err) == plugins.ErrUnsupported {
		return []plugins.Action{}, nil
	}
	return list, err
}

// RunAction runs a plugin action on a host; item is the row it applies to.
func (s *PluginService) RunAction(ctx context.Context, pluginID, hostID, action, item string) (plugins.Result, error) {
	p, target, exec, release, err := s.onHost(ctx, pluginID, hostID)
	if err != nil {
		return plugins.Result{}, err
	}
	defer release()
	return p.Run(ctx, s.pluginSettings(ctx, p), target, action, item, exec)
}

// pluginSettings returns the settings of the plugin's first configuration
// (panels mostly have none); secrets are left out when the vault is locked.
func (s *PluginService) pluginSettings(ctx context.Context, p plugins.Plugin) map[string]string {
	list, _ := s.configs.List(ctx)
	for _, c := range list {
		if c.PluginID == p.Manifest().ID {
			if out, err := s.settings(ctx, p, c); err == nil {
				return out
			}
			return c.Settings
		}
	}
	return map[string]string{}
}

func (s *PluginService) target(ctx context.Context, hostID string) (plugins.Target, error) {
	if s.sessions == nil {
		return plugins.Target{}, errs.New(plugins.ErrUnsupported, nil)
	}
	ht, err := s.sessions.hostTarget(ctx, hostID)
	if err != nil {
		return plugins.Target{}, err
	}
	return plugins.Target{HostID: hostID, Name: ht.name, Hostname: ht.target.Host, Port: ht.target.Port, User: ht.target.User}, nil
}

// onHost connects to the host (sharing its pooled connection) and returns
// the plugin, the target and an Exec over that connection.
func (s *PluginService) onHost(ctx context.Context, pluginID, hostID string) (plugins.Plugin, plugins.Target, plugins.Exec, func(), error) {
	p, err := s.installedPlugin(ctx, pluginID)
	if err != nil {
		return nil, plugins.Target{}, nil, nil, err
	}
	target, err := s.target(ctx, hostID)
	if err != nil {
		return nil, plugins.Target{}, nil, nil, err
	}
	ht, err := s.sessions.hostTarget(ctx, hostID)
	if err != nil {
		return nil, plugins.Target{}, nil, nil, err
	}
	client, release, err := s.sessions.connect(ctx, ht, "plugin:"+hostID)
	if err != nil {
		return nil, plugins.Target{}, nil, nil, err
	}
	return p, target, sshExec(client), release, nil
}

// sshExec runs commands on their own channel of a connection; output is
// stdout and stderr together, bounded to 4 MB.
func sshExec(client *ssh.Client) plugins.Exec {
	return func(ctx context.Context, command string) (string, int, error) {
		ch, err := client.NewSession()
		if err != nil {
			return "", 0, err
		}
		defer ch.Close()
		var out syncLimitedBuffer
		out.max = 4 << 20
		ch.Stdout, ch.Stderr = &out, &out
		done := make(chan error, 1)
		go func() { done <- ch.Run(command) }()
		select {
		case err = <-done:
		case <-time.After(pluginExecTimeout):
			return out.String(), 0, context.DeadlineExceeded
		case <-ctx.Done():
			return out.String(), 0, ctx.Err()
		}
		var exit *ssh.ExitError
		if errors.As(err, &exit) {
			return out.String(), exit.ExitStatus(), nil
		}
		return out.String(), 0, err
	}
}

type syncLimitedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
	max int
}

func (b *syncLimitedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if room := b.max - b.buf.Len(); room > 0 {
		if len(p) > room {
			b.buf.Write(p[:room])
		} else {
			b.buf.Write(p)
		}
	}
	return len(p), nil
}

func (b *syncLimitedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// ChooseFile asks for a plugin's .zip with the system's file chooser (a
// plugin folder, for development, is typed as a path: Windows' chooser
// cannot offer files and folders together). It returns "" when cancelled.
func (s *PluginService) ChooseFile(title string) (string, error) {
	a := application.Get()
	if a == nil {
		return "", errs.New(plugins.ErrUnsupported, nil)
	}
	path, err := a.Dialog.OpenFile().
		SetTitle(title).
		CanChooseFiles(true).
		AddFilter("Shellvane plugin (*.zip)", "*.zip").
		PromptForSingleSelection()
	if err != nil {
		return "", nil // cancelled
	}
	return path, nil
}
