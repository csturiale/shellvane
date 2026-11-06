-- Teams this computer belongs to: where the team file is, and what was
-- verified last (the admins' signing keys, the epoch), against rollbacks.
CREATE TABLE teams (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    location   TEXT NOT NULL,
    trusted    TEXT NOT NULL,
    min_epoch  INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT NOT NULL
);
