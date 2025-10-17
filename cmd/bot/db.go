package main

import (
	"database/sql"
	"fmt"
)

func initDB(dbPath string) (*sql.DB, error) {
	// Open SQLite database (creates if it doesn't exist)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	// Create tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			guild_id TEXT NOT NULL,
			channel_id TEXT NOT NULL,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			seconds_elapsed INTEGER,
			status INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS session_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER,
			pomodoro_duration INTEGER NOT NULL,
			short_break_duration INTEGER NOT NULL,
			long_break_duration INTEGER NOT NULL,
			intervals INTEGER NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return db, nil
}
