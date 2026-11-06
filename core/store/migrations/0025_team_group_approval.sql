-- The digest of a shared group's commands (local and remote commands,
-- X11) this person approved: until then they are left out of the copy.
ALTER TABLE team_groups ADD COLUMN approved TEXT NOT NULL DEFAULT '';
-- The digest of the commands in the team's version, to show a review.
ALTER TABLE team_groups ADD COLUMN commands TEXT NOT NULL DEFAULT '';
