-- Keys on hardware tokens (PKCS#11) have no private part in the vault:
-- secret_id becomes optional and "token" describes where the key lives
-- (JSON: module, serial, label, key id).
CREATE TABLE keys_new (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL,
    bits        INTEGER NOT NULL,
    public_key  TEXT NOT NULL,
    fingerprint TEXT NOT NULL UNIQUE,
    comment     TEXT NOT NULL DEFAULT '',
    secret_id   TEXT REFERENCES secrets (id),
    created_at  TEXT NOT NULL,
    certificate TEXT NOT NULL DEFAULT '',
    token       TEXT NOT NULL DEFAULT ''
);
INSERT INTO keys_new (id, name, type, bits, public_key, fingerprint, comment, secret_id, created_at, certificate)
    SELECT id, name, type, bits, public_key, fingerprint, comment, secret_id, created_at, certificate FROM keys;
DROP TABLE keys;
ALTER TABLE keys_new RENAME TO keys;
