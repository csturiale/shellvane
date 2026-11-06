package plugins

import (
	"context"
	"time"

	"github.com/csturiale/shellvane/core/errs"
)

// ErrNotInstalled: the plugin must be installed before it is used.
const ErrNotInstalled errs.Code = "plugins.notInstalled"

// Installed returns the IDs of the installed plugins.
func (c *Configs) Installed(ctx context.Context) (map[string]bool, error) {
	rows, err := c.db.QueryContext(ctx, `SELECT plugin_id FROM plugin_installs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// SetInstalled marks a plugin installed or not.
func (c *Configs) SetInstalled(ctx context.Context, id string, on bool) error {
	if !on {
		_, err := c.db.ExecContext(ctx, `DELETE FROM plugin_installs WHERE plugin_id = ?`, id)
		return err
	}
	_, err := c.db.ExecContext(ctx, `INSERT OR IGNORE INTO plugin_installs (plugin_id, installed_at) VALUES (?, ?)`,
		id, c.now().UTC().Format(time.RFC3339))
	return err
}
