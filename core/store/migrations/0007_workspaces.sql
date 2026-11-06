-- Saved arrangements of tabs, splits and sessions. The layout is opaque
-- JSON owned by the UI. The row with id 'last' is the session that was open
-- when the app quit, restored at startup.
CREATE TABLE workspaces (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    layout     TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX workspaces_name ON workspaces (name COLLATE NOCASE);
