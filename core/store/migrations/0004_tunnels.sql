CREATE TABLE tunnels (
    id         TEXT PRIMARY KEY,
    host_id    TEXT NOT NULL REFERENCES hosts (id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL CHECK (kind IN ('local', 'remote', 'dynamic')),
    bind_addr  TEXT NOT NULL,
    bind_port  INTEGER NOT NULL,
    dest_host  TEXT NOT NULL DEFAULT '',
    dest_port  INTEGER NOT NULL DEFAULT 0,
    autostart  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
