CREATE TABLE groups (
    id         TEXT PRIMARY KEY,
    parent_id  TEXT REFERENCES groups (id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    color      TEXT NOT NULL DEFAULT '',
    settings   TEXT NOT NULL DEFAULT '{}',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX groups_parent ON groups (parent_id);

CREATE TABLE hosts (
    id         TEXT PRIMARY KEY,
    group_id   TEXT REFERENCES groups (id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    hostname   TEXT NOT NULL,
    notes      TEXT NOT NULL DEFAULT '',
    settings   TEXT NOT NULL DEFAULT '{}',
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX hosts_group ON hosts (group_id);
