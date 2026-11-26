-- A shared host group whose team logins travel with it. Groups shared
-- before keep doing so; a new share asks the person sharing.
ALTER TABLE team_groups ADD COLUMN share_logins INTEGER NOT NULL DEFAULT 1;
