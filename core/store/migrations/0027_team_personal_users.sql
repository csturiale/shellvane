-- A shared host group whose user names are personal: published without
-- them, each member keeps their own.
ALTER TABLE team_groups ADD COLUMN personal_users INTEGER NOT NULL DEFAULT 0;
