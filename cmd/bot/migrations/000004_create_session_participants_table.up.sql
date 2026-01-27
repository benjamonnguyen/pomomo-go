CREATE TABLE session_participants (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    voice_channel_id TEXT NOT NULL,
    is_muted TEXT NOT NULL,
    is_deafened TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE,
    UNIQUE(user_id, session_id)
);

CREATE INDEX session_id_idx ON session_participants (session_id);
