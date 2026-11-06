-- The last verified team file, to open the team read-only when its storage
-- cannot be reached and to publish it again somewhere else.
ALTER TABLE teams ADD COLUMN cache TEXT NOT NULL DEFAULT '';
ALTER TABLE teams ADD COLUMN cached_at TEXT NOT NULL DEFAULT '';

-- Environments, snippets, workspaces and rules shared with (or mirrored
-- from) a team: the local object of each team item. approved is the
-- digest of the commands the person approved (environments, rules).
CREATE TABLE team_links (
    team_id    TEXT NOT NULL,
    item_id    TEXT NOT NULL,
    kind       TEXT NOT NULL,
    local_id   TEXT NOT NULL,
    approved   TEXT NOT NULL DEFAULT '',
    changed_by TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (team_id, item_id)
);
