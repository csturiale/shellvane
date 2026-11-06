-- Custom terminal color schemes (imported or edited), as JSON.
CREATE TABLE themes (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    data       TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX themes_name ON themes (name COLLATE NOCASE);
