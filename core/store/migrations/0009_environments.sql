-- Environments: a jump host, cloud logins, commands, tunnels, kubectl
-- contexts and a layout started together. The definition is JSON.
CREATE TABLE environments (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    data       TEXT NOT NULL,
    updated_at TEXT NOT NULL
);
CREATE UNIQUE INDEX environments_name ON environments (name COLLATE NOCASE);
