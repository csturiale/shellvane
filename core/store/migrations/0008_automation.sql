-- Output triggers: highlight, notify or respond when a regex matches a
-- terminal line. Scope: global, a group (inherited) or a host.
CREATE TABLE rules (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    pattern      TEXT NOT NULL,
    ignore_case  INTEGER NOT NULL DEFAULT 0,
    action       TEXT NOT NULL,                 -- highlight | notify | respond
    color        TEXT NOT NULL DEFAULT '',
    response     TEXT NOT NULL DEFAULT '',
    secret_scope TEXT NOT NULL DEFAULT '',      -- respond with a vault credential
    enter        INTEGER NOT NULL DEFAULT 1,
    sound        INTEGER NOT NULL DEFAULT 0,
    scope_kind   TEXT NOT NULL DEFAULT 'global', -- global | group | host
    scope_id     TEXT NOT NULL DEFAULT '',
    enabled      INTEGER NOT NULL DEFAULT 1,
    created_at   TEXT NOT NULL,
    updated_at   TEXT NOT NULL
);

-- What automation did on the user's behalf (automatic responses, runbooks).
CREATE TABLE audit_log (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    at      TEXT NOT NULL,
    kind    TEXT NOT NULL,
    host    TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    detail  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX audit_log_at ON audit_log (at);
