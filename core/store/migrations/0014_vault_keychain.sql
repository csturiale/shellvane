-- Keychain unlock: the data key also wrapped by a random secret kept in the
-- system keychain (macOS Keychain, Windows Credential Manager, Secret
-- Service), so the vault opens without typing the master password.
ALTER TABLE vault_meta ADD COLUMN device_wrapped BLOB;
ALTER TABLE vault_meta ADD COLUMN device_account TEXT;
