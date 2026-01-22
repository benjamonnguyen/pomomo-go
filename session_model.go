package pomomo

import (
	"context"
	"time"
)

type SessionStatus uint8

const (
	_ SessionStatus = iota
	SessionRunning
	SessionPaused
	SessionEnded
)

type SessionInterval uint8

const (
	_ SessionInterval = iota
	PomodoroInterval
	ShortBreakInterval
	LongBreakInterval
)

func (i SessionInterval) String() string {
	switch i {
	case PomodoroInterval:
		return "Pomodoro"
	case ShortBreakInterval:
		return "Short Break"
	case LongBreakInterval:
		return "Long Break"
	default:
		panic("no matching enum for SessionInterval: " + string(i))
	}
}

type SessionRecord struct {
	GuildID   string
	ChannelID string
	MessageID string

	//
	IntervalStartedAt time.Time
	CurrentInterval   SessionInterval
	Status            SessionStatus
}

type ExistingSessionRecord struct {
	DBRow
	SessionRecord
}

type SessionSettingsRecord struct {
	SessionID string

	//
	Pomodoro   time.Duration
	ShortBreak time.Duration
	LongBreak  time.Duration
	Intervals  int
}

type ExistingSessionSettingsRecord struct {
	DBRow
	SessionSettingsRecord
}

type SessionRepo interface {
	InsertSession(context.Context, SessionRecord) (ExistingSessionRecord, error)
	UpdateSession(ctx context.Context, id string, s SessionRecord) (ExistingSessionRecord, error)
	DeleteSession(ctx context.Context, id string) (ExistingSessionRecord, error)
	GetSession(ctx context.Context, id string) (ExistingSessionRecord, error)
	GetByStatus(ctx context.Context, status ...SessionStatus) ([]ExistingSessionRecord, error)
	InsertSettings(context.Context, SessionSettingsRecord) (ExistingSessionSettingsRecord, error)
	DeleteSettings(ctx context.Context, id string) (ExistingSessionSettingsRecord, error)
	GetSettings(ctx context.Context, id string) (ExistingSessionSettingsRecord, error)
}
