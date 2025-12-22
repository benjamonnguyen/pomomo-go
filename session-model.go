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

type SessionRecord struct {
	GuildID   string
	ChannelID string

	//
	StartedAt      time.Time
	SecondsElapsed int
	Status         SessionStatus
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
	UpdateSession(ctx context.Context, id int, s SessionRecord) (ExistingSessionRecord, error)
	DeleteSession(ctx context.Context, id int) (ExistingSessionRecord, error)
	GetSession(ctx context.Context, id string) (ExistingSessionRecord, error)
	InsertSettings(context.Context, SessionSettingsRecord) (ExistingSessionSettingsRecord, error)
	UpdateSetting(ctx context.Context, id int, s SessionSettingsRecord) (ExistingSessionSettingsRecord, error)
	DeleteSetting(ctx context.Context, id int) (ExistingSessionSettingsRecord, error)
	GetSetting(ctx context.Context, id string) (ExistingSessionSettingsRecord, error)
}
