-- Single-row vault header: KDF parameters and the data key wrapped by the
-- key derived from the master password.
CREATE TABLE vault_meta (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    version     INTEGER NOT NULL,
    kdf_salt    BLOB NOT NULL,
    kdf_time    INTEGER NOT NULL,
    kdf_memory  INTEGER NOT NULL,
    kdf_threads INTEGER NOT NULL,
    wrapped_key BLOB NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE secrets (
    id         TEXT PRIMARY KEY,
    kind       TEXT NOT NULL,
    ciphertext BLOB NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Saved credentials: which secret answers which prompt. scope is
-- "password:user@host:port" or "passphrase:<key fingerprint>".
CREATE TABLE credentials (
    scope      TEXT PRIMARY KEY,
    secret_id  TEXT NOT NULL REFERENCES secrets (id) ON DELETE CASCADE,
    label      TEXT NOT NULL,
    created_at TEXT NOT NULL
);
