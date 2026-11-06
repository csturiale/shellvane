-- Optional OpenSSH certificate signed for a key (authorized_keys format).
ALTER TABLE keys ADD COLUMN certificate TEXT NOT NULL DEFAULT '';
