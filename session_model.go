package pomomo

import (
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
	SessionID      string
	ParticipantID  string
	VoiceChannelID string
	TextChannelID  string
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

type VoiceState struct {
	Mute, Deaf bool
}

type ParticipantRecord struct {
	SessionID           SessionID
	GuildID             string
	UserID              string
	VoiceCID            VoiceChannelID
	IsMuted, IsDeafened bool
}

type ExistingParticipantRecord struct {
	ExistingRecord[ParticipantID]
	ParticipantRecord
}

type SessionSettingsRecord struct {
	SessionID SessionID

	//
	Pomodoro   time.Duration
	ShortBreak time.Duration
	LongBreak  time.Duration
	Intervals  int
	NoMute     bool
	NoDeafen   bool
}

type ExistingSessionSettingsRecord struct {
	ExistingRecord[SessionID]
	SessionSettingsRecord
}
