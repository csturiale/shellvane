-- Fingerprints of a shared copy as last mirrored or published (mirrored)
-- and of the team's version of it (remote): a copy changed here since is
-- kept, not replaced, until the person publishes or discards the change.
ALTER TABLE team_links ADD COLUMN mirrored TEXT NOT NULL DEFAULT '';
ALTER TABLE team_links ADD COLUMN remote TEXT NOT NULL DEFAULT '';
ALTER TABLE team_groups ADD COLUMN mirrored TEXT NOT NULL DEFAULT '';
ALTER TABLE team_groups ADD COLUMN remote TEXT NOT NULL DEFAULT '';
