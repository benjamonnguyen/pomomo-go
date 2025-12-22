package pomomo

import (
	"time"
)

type DBRow struct {
	ID        string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Session struct {
	DBRow
	GuildID   string
	ChannelID string

	//
	StartedAt      time.Time
	SecondsElapsed int
	Status         uint8
}

type SessionStatus uint8

const (
	_ SessionStatus = iota
	SessionRunning
	SessionPaused
	SessionEnded
)

type SessionSettings struct {
	DBRow
	SessionID          int
	PomodoroDuration   time.Duration
	ShortBreakDuration time.Duration

	//
	Pomodoro   time.Duration
	ShortBreak time.Duration
	LongBreak  time.Duration
	Intervals  int
}
