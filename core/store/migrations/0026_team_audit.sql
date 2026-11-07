-- The newest audit entry seen on this computer: if it is missing from the
-- team file later, entries were removed (or an older version of Shellvane
-- rewrote the file without its log).
ALTER TABLE teams ADD COLUMN audit_last TEXT NOT NULL DEFAULT '';
