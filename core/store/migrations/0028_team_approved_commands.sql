-- The commands the person approved, as a JSON list: a review after a
-- change in the team shows what changed since.
ALTER TABLE team_links ADD COLUMN approved_commands TEXT NOT NULL DEFAULT '';
ALTER TABLE team_groups ADD COLUMN approved_commands TEXT NOT NULL DEFAULT '';
