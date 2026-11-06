-- Tags (JSON array), favorites and the time of the last connection.
ALTER TABLE hosts ADD COLUMN tags TEXT NOT NULL DEFAULT '[]';
ALTER TABLE hosts ADD COLUMN favorite INTEGER NOT NULL DEFAULT 0;
ALTER TABLE hosts ADD COLUMN last_used TEXT NOT NULL DEFAULT '';
