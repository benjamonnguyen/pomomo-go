// Package models helps control struct access and mutation
package models

import (
	"time"

	"github.com/benjamonnguyen/pomomo-go"
)

type Session struct {
	ID       string
	Settings SessionSettings
	Stats    SessionStats
	Record   pomomo.SessionRecord
}

func SessionFromExistingRecords(record pomomo.ExistingSessionRecord, settings pomomo.ExistingSessionSettingsRecord) Session {
	if record.ID == "" || record.TextCID == "" || record.VoiceCID == "" || record.GuildID == "" || record.MessageID == "" {
		panic("missing required IDs")
	}
	return Session{
		ID:     record.ID,
		Record: record.SessionRecord,
		Settings: SessionSettings{
			Pomodoro:   settings.Pomodoro,
			ShortBreak: settings.ShortBreak,
			LongBreak:  settings.LongBreak,
			Intervals:  settings.Intervals,
		},
	}
}

func NewSession(sessionID, guildID, textCID, voiceCID, messageID string, settings SessionSettings) Session {
	if guildID == "" || textCID == "" || voiceCID == "" {
		panic("missing required IDs")
	}
	s := Session{
		ID: sessionID,
		Record: pomomo.SessionRecord{
			GuildID:   guildID,
			VoiceCID:  pomomo.VoiceChannelID(voiceCID),
			TextCID:   pomomo.TextChannelID(textCID),
			MessageID: messageID,
			Status:    pomomo.SessionRunning,
		},
		Settings: settings,
	}
	s.Record.TimeRemainingAtStart = s.CurrentDuration()
	return s
}

type SessionSettings struct {
	Pomodoro, ShortBreak, LongBreak time.Duration
	Intervals                       int
}

type SessionStats struct {
	CompletedPomodoros int
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
	if shouldUpdateStats {
		if s.Record.CurrentInterval == pomomo.PomodoroInterval {
			s.Stats.CompletedPomodoros++
		}
	}

	// update interval
	var next pomomo.SessionInterval
	if s.Record.CurrentInterval == pomomo.PomodoroInterval {
		// After pomodoro, decide break type based on completed pomodoros
		if s.Stats.CompletedPomodoros > 0 && s.Stats.CompletedPomodoros%s.Settings.Intervals == 0 {
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
		s.Record.IntervalStartedAt = time.Now()
	}
	s.Record.TimeRemainingAtStart = s.CurrentDuration()
}
