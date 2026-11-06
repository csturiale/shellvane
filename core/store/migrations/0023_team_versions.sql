-- The highest roster and items versions seen (with min_epoch): an older
-- team file put back on the storage is refused.
ALTER TABLE teams ADD COLUMN roster_seq INTEGER NOT NULL DEFAULT 0;
ALTER TABLE teams ADD COLUMN items_seq INTEGER NOT NULL DEFAULT 0;
