ALTER TABLE sessions RENAME COLUMN channel_id TO text_channel_id;
ALTER TABLE sessions ADD COLUMN voice_channel_id TEXT NOT NULL DEFAULT '';
UPDATE sessions SET voice_channel_id = '' WHERE voice_channel_id IS NULL;