CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    guild_id TEXT NOT NULL,
    channel_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    interval_started_at INTEGER NOT NULL,
    current_interval INTEGER NOT NULL,
    status INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX guild_channel_idx ON sessions (guild_id, channel_id);
CREATE INDEX status_idx ON sessions (status);
