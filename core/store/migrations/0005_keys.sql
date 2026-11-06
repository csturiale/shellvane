-- SSH keys. The private key is a vault secret; the public part is readable
-- while the vault is locked.
CREATE TABLE keys (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    bits        INTEGER NOT NULL,
    public_key  TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    comment     TEXT NOT NULL DEFAULT '',
    secret_id   TEXT NOT NULL REFERENCES secrets (id),
    created_at  TEXT NOT NULL
);
