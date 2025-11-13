CREATE TABLE IF NOT EXISTS session_settings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id INTEGER,
	pomodoro_duration INTEGER NOT NULL,
	short_break_duration INTEGER NOT NULL,
	long_break_duration INTEGER NOT NULL,
	intervals INTEGER NOT NULL,
	FOREIGN KEY (session_id) REFERENCES sessions(id)
);