CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    guild_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    seconds_elapsed INTEGER NOT NULL,
    status INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX guild_channel_idx ON sessions (guild_id, channel_id);
CREATE INDEX status_idx ON sessions (status);