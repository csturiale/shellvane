-- The keychain unlock can require Touch ID or Windows Hello first.
ALTER TABLE vault_meta ADD COLUMN device_biometric INTEGER NOT NULL DEFAULT 0;
