package plugins

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/csturiale/shellvane/core/errs"
)

// Error codes of configurations.
const (
	ErrNameRequired errs.Code = "plugins.nameRequired"
	ErrDuplicate    errs.Code = "plugins.duplicate"
	ErrNotFound     errs.Code = "plugins.notFound"
)

// Config is one configuration of a plugin. Secret settings are not here:
// they are kept in the vault under SecretScope.
type Config struct {
	ID       string            `json:"id"`
	PluginID string            `json:"pluginId"`
	Name     string            `json:"name"`
	Settings map[string]string `json:"settings"`
	// GroupID is the group an inventory plugin fills ("" until the first sync).
	GroupID  string `json:"groupId"`
	LastSync string `json:"lastSync"` // RFC 3339, "" for never
}

// SecretScope is the vault scope of a secret setting.
func SecretScope(configID, key string) string { return "plugin:" + configID + ":" + key }

// Configs stores plugin configurations and the hosts they created.
type Configs struct {
	db  *sql.DB
	now func() time.Time
}

// NewConfigs returns the store over a migrated database.
func NewConfigs(db *sql.DB) *Configs { return &Configs{db: db, now: time.Now} }

// List returns the configurations sorted by name.
func (c *Configs) List(ctx context.Context) ([]Config, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT id, plugin_id, name, settings, COALESCE(group_id, ''), last_sync FROM plugin_configs ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Config{}
	for rows.Next() {
		cfg, err := scanConfig(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cfg)
	}
	return out, rows.Err()
}

// Get returns one configuration.
func (c *Configs) Get(ctx context.Context, id string) (Config, error) {
	row := c.db.QueryRowContext(ctx, `SELECT id, plugin_id, name, settings, COALESCE(group_id, ''), last_sync FROM plugin_configs WHERE id = ?`, id)
	cfg, err := scanConfig(row)
	if err != nil {
		return Config{}, errs.New(ErrNotFound, errs.Params{"id": id})
	}
	return cfg, nil
}

type scanner interface{ Scan(...any) error }

func scanConfig(s scanner) (Config, error) {
	var cfg Config
	var raw string
	if err := s.Scan(&cfg.ID, &cfg.PluginID, &cfg.Name, &raw, &cfg.GroupID, &cfg.LastSync); err != nil {
		return Config{}, err
	}
	cfg.Settings = map[string]string{}
	json.Unmarshal([]byte(raw), &cfg.Settings)
	return cfg, nil
}

// Save creates (empty ID) or replaces a configuration.
func (c *Configs) Save(ctx context.Context, cfg Config) (Config, error) {
	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		return Config{}, errs.New(ErrNameRequired, nil)
	}
	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}
	if cfg.Settings == nil {
		cfg.Settings = map[string]string{}
	}
	raw, _ := json.Marshal(cfg.Settings)
	_, err := c.db.ExecContext(ctx, `
		INSERT INTO plugin_configs (id, plugin_id, name, settings, group_id, last_sync, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET name = excluded.name, settings = excluded.settings, group_id = excluded.group_id,
			last_sync = excluded.last_sync, updated_at = excluded.updated_at`,
		cfg.ID, cfg.PluginID, cfg.Name, string(raw), nullable(cfg.GroupID), cfg.LastSync, c.now().UTC().Format(time.RFC3339))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return Config{}, errs.New(ErrDuplicate, errs.Params{"name": cfg.Name})
	}
	return cfg, err
}

// Delete removes a configuration and forgets its hosts (they stay in the
// inventory as ordinary hosts).
func (c *Configs) Delete(ctx context.Context, id string) error {
	if _, err := c.db.ExecContext(ctx, `DELETE FROM plugin_hosts WHERE config_id = ?`, id); err != nil {
		return err
	}
	_, err := c.db.ExecContext(ctx, `DELETE FROM plugin_configs WHERE id = ?`, id)
	return err
}

// Hosts returns the hosts a configuration created, by key.
func (c *Configs) Hosts(ctx context.Context, configID string) (map[string]string, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT key, host_id FROM plugin_hosts WHERE config_id = ?`, configID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var key, id string
		if err := rows.Scan(&key, &id); err != nil {
			return nil, err
		}
		out[key] = id
	}
	return out, rows.Err()
}

// SetHosts replaces the key → host mapping of a configuration.
func (c *Configs) SetHosts(ctx context.Context, configID string, hosts map[string]string) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM plugin_hosts WHERE config_id = ?`, configID); err != nil {
		return err
	}
	for key, id := range hosts {
		if _, err := tx.ExecContext(ctx, `INSERT INTO plugin_hosts (config_id, key, host_id) VALUES (?, ?, ?)`, configID, key, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
