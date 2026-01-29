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
	NoDeafen             bool
}

type ExistingSessionRecord struct {
	ExistingRecord[SessionID]
	SessionRecord
}

type VoiceState struct {
	Mute, Deaf bool
}

type SessionParticipantRecord struct {
	SessionID           SessionID
	GuildID             string
	UserID              string
	VoiceCID            VoiceChannelID
	IsMuted, IsDeafened bool
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
	// TODO reconsider SessionRepo being defined here
	InsertSession(context.Context, SessionRecord) (ExistingSessionRecord, error)
	UpdateSession(context.Context, SessionID, SessionRecord) (ExistingSessionRecord, error)
	DeleteSession(context.Context, SessionID) (ExistingSessionRecord, error)
	GetSession(context.Context, SessionID) (ExistingSessionRecord, error)
	GetSessionsByStatus(context.Context, ...SessionStatus) ([]ExistingSessionRecord, error)

	// settings
	InsertSettings(context.Context, SessionSettingsRecord) (ExistingSessionSettingsRecord, error)
	GetSettings(context.Context, SessionID) (ExistingSessionSettingsRecord, error)
	DeleteSettings(context.Context, SessionID) (ExistingSessionSettingsRecord, error)

	// participants
	InsertParticipant(context.Context, SessionParticipantRecord) (ExistingSessionParticipantRecord, error)
	UpdateParticipant(context.Context, SessionParticipantID, SessionParticipantRecord) (ExistingSessionParticipantRecord, error)
	DeleteParticipant(context.Context, SessionParticipantID) (ExistingSessionParticipantRecord, error)
	GetAllParticipants(context.Context) ([]ExistingSessionParticipantRecord, error)
	GetParticipantByUserID(context.Context, string) (ExistingSessionParticipantRecord, error)
}
