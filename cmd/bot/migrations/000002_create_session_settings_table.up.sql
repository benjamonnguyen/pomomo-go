CREATE TABLE session_settings (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    pomodoro_duration INTEGER NOT NULL,
    short_break_duration INTEGER NOT NULL,
    long_break_duration INTEGER NOT NULL,
    intervals INTEGER NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX session_id_idx ON session_settings (session_id);