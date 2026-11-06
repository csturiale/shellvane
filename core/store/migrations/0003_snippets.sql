CREATE TABLE snippets (
    id         TEXT PRIMARY KEY,
    folder     TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL,
    body       TEXT NOT NULL,
    run        INTEGER NOT NULL DEFAULT 1, -- press Enter after sending
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
