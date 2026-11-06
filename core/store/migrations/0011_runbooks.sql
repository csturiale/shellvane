-- Runbooks: steps run on several hosts, as JSON.
CREATE TABLE runbooks (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    data       TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX runbooks_name ON runbooks (name COLLATE NOCASE);
