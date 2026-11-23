CREATE TABLE focus_intervals (
    id TEXT PRIMARY KEY,
    guild_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    pomodoro INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    completed_at INTEGER NOT NULL,
    UNIQUE(session_id, user_id, pomodoro)
);

CREATE INDEX focus_intervals_guild_user_idx ON focus_intervals (guild_id, user_id, completed_at);
CREATE INDEX focus_intervals_guild_completed_at_idx ON focus_intervals (guild_id, completed_at);
