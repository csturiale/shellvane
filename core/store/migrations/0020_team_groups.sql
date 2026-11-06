-- Host groups mirrored from (or shared to) a team: the local group that
-- holds each shared group item.
CREATE TABLE team_groups (
    team_id  TEXT NOT NULL,
    item_id  TEXT NOT NULL,
    group_id TEXT NOT NULL,
    PRIMARY KEY (team_id, item_id)
);
