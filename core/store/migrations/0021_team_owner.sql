-- The team's owner as verified last ({"id", "key"}): a new owner must
-- carry this one's signature.
ALTER TABLE teams ADD COLUMN owner TEXT NOT NULL DEFAULT '';
