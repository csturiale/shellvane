-- Configured plugins: one row per configuration (a plugin may be set up
-- several times, e.g. two AWS profiles). Secret settings live in the vault.
CREATE TABLE plugin_configs (
    id         TEXT PRIMARY KEY,
    plugin_id  TEXT NOT NULL,
    name       TEXT NOT NULL,
    settings   TEXT NOT NULL,
    group_id   TEXT,
    last_sync  TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX plugin_configs_name ON plugin_configs (name COLLATE NOCASE);

-- Hosts an inventory plugin created, by their key in the source, so a
-- refresh updates them instead of adding them again.
CREATE TABLE plugin_hosts (
    config_id TEXT NOT NULL,
    key       TEXT NOT NULL,
    host_id   TEXT NOT NULL,
    PRIMARY KEY (config_id, key)
);
