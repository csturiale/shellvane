-- Plugins the user installed (built-in ones included): nothing runs, and
-- no panel appears, until the user installs it.
CREATE TABLE plugin_installs (
    plugin_id    TEXT PRIMARY KEY,
    installed_at TEXT NOT NULL
);

-- Plugins already set up stay installed.
INSERT INTO plugin_installs (plugin_id, installed_at)
SELECT DISTINCT plugin_id, strftime('%Y-%m-%dT%H:%M:%SZ', 'now') FROM plugin_configs;
