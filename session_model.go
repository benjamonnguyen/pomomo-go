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

type (
	SessionID            string
	SessionParticipantID string
	VoiceChannelID       string
	TextChannelID        string
)

type SessionRecord struct {
	GuildID, MessageID string
	VoiceCID           VoiceChannelID
	TextCID            TextChannelID

	//
	IntervalStartedAt    time.Time
	TimeRemainingAtStart time.Duration
	CurrentInterval      SessionInterval
	Status               SessionStatus
}

type ExistingSessionRecord struct {
	ExistingRecord[SessionID]
	SessionRecord
}

type SessionParticipantRecord struct {
	SessionID           SessionID
	UserID              string
	VoiceCID            VoiceChannelID
	IsMuted, IsDeafened string
}

type ExistingSessionParticipantRecord struct {
	ExistingRecord[SessionParticipantID]
	SessionParticipantRecord
}

type SessionSettingsRecord struct {
	SessionID SessionID

	//
	Pomodoro   time.Duration
	ShortBreak time.Duration
	LongBreak  time.Duration
	Intervals  int
}

type ExistingSessionSettingsRecord struct {
	ExistingRecord[SessionID]
	SessionSettingsRecord
}

type SessionRepo interface {
	InsertSession(context.Context, SessionRecord) (ExistingSessionRecord, error)
	UpdateSession(context.Context, SessionID, SessionRecord) (ExistingSessionRecord, error)
	DeleteSession(context.Context, SessionID) (ExistingSessionRecord, error)
	GetSession(context.Context, SessionID) (ExistingSessionRecord, error)
	GetSessionsByStatus(context.Context, ...SessionStatus) ([]ExistingSessionRecord, error)

	// settings
	InsertSettings(context.Context, SessionSettingsRecord) (ExistingSessionSettingsRecord, error)
	GetSettings(context.Context, SessionID) (ExistingSessionSettingsRecord, error)
	// settings are only cascade deleted

	// participants
	InsertParticipant(context.Context, SessionParticipantRecord) (ExistingSessionParticipantRecord, error)
	DeleteParticipant(context.Context, SessionParticipantID) (ExistingSessionParticipantRecord, error)
	GetAllParticipants(context.Context) ([]ExistingSessionParticipantRecord, error)
}
