// Package models helps control struct access and mutation
package models

import (
	"time"

	"github.com/benjamonnguyen/pomomo-go"
)

type Participant struct {
	ID                pomomo.ParticipantID
	Record            pomomo.ParticipantRecord
	StartedIntervalAt time.Time
}

type Session struct {
	ID       pomomo.SessionID
	Settings pomomo.SessionSettingsRecord
	// StartedAt is when the session record was created
	StartedAt time.Time
	Record    pomomo.SessionRecord
	Greeting  string
}

func SessionFromExistingRecords(record pomomo.ExistingSessionRecord, settings pomomo.ExistingSessionSettingsRecord) Session {
	if record.ID == "" || record.TextCID == "" || record.VoiceCID == "" || record.GuildID == "" || record.MessageID == "" {
		panic("missing required IDs")
	}
	return Session{
		ID:        record.ID,
		Record:    record.SessionRecord,
		Settings:  settings.SessionSettingsRecord,
		StartedAt: record.CreatedAt,
	}
}

func NewSession(sessionID, guildID, textCID, voiceCID, messageID string, settings pomomo.SessionSettingsRecord) Session {
	if guildID == "" || textCID == "" || voiceCID == "" {
		panic("missing required IDs")
	}
	s := Session{
		ID: pomomo.SessionID(sessionID),
		Record: pomomo.SessionRecord{
			GuildID:   guildID,
			VoiceCID:  pomomo.VoiceChannelID(voiceCID),
			TextCID:   pomomo.TextChannelID(textCID),
			MessageID: messageID,
			Status:    pomomo.SessionRunning,
		},
		Settings:  settings,
		StartedAt: time.Now(),
	}
	s.Record.TimeRemainingAtStart = s.CurrentDuration()
	return s
}

func (s Session) TimeRemaining() time.Duration {
	return s.Record.TimeRemainingAtStart - time.Since(s.Record.IntervalStartedAt)
}

func (s Session) CurrentDuration() time.Duration {
	switch s.Record.CurrentInterval {
	case pomomo.PomodoroInterval:
		return s.Settings.Pomodoro
	case pomomo.ShortBreakInterval:
		return s.Settings.ShortBreak
	case pomomo.LongBreakInterval:
		return s.Settings.LongBreak
	default:
		return 0
	}
}

func (s *Session) GoNextInterval(shouldUpdateStats bool) {
	// TODO could be external
	if shouldUpdateStats {
		if s.Record.CurrentInterval == pomomo.PomodoroInterval {
			s.Record.CompletedPomodoros++
		}
	}

	// update interval
	var next pomomo.SessionInterval
	if s.Record.CurrentInterval == pomomo.PomodoroInterval {
		// After pomodoro, decide break type based on completed pomodoros
		if s.Record.CompletedPomodoros > 0 && s.Record.CompletedPomodoros%s.Settings.Intervals == 0 {
			next = pomomo.LongBreakInterval
		} else {
			next = pomomo.ShortBreakInterval
		}
	} else {
		// After any break, next is pomodoro
		next = pomomo.PomodoroInterval
	}
	s.Record.CurrentInterval = next

	// update time/duration
	if s.Record.Status == pomomo.SessionRunning && !s.Record.IntervalStartedAt.IsZero() {
		// may need multiple calls to catch up
		s.Record.IntervalStartedAt = s.Record.IntervalStartedAt.Add(s.Record.TimeRemainingAtStart)
	} else {
		// Session is paused or hasn't started; start now
		s.Record.IntervalStartedAt = time.Now()
	}
	s.Record.TimeRemainingAtStart = s.CurrentDuration()
}
