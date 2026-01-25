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
	record   pomomo.SessionRecord
}

func SessionFromExistingRecords(record pomomo.ExistingSessionRecord, settings pomomo.ExistingSessionSettingsRecord) Session {
	if record.ID == "" || record.ChannelID == "" || record.GuildID == "" || record.MessageID == "" {
		panic("missing required IDs")
	}
	return Session{
		ID:     record.ID,
		record: record.SessionRecord,
		Settings: SessionSettings{
			Pomodoro:   settings.Pomodoro,
			ShortBreak: settings.ShortBreak,
			LongBreak:  settings.LongBreak,
			Intervals:  settings.Intervals,
		},
	}
}

func NewSession(sessionID, guildID, channelID, messageID string, settings SessionSettings) Session {
	if guildID == "" || channelID == "" {
		panic("missing required IDs")
	}
	s := Session{
		ID: sessionID,
		record: pomomo.SessionRecord{
			GuildID:   guildID,
			ChannelID: channelID,
			MessageID: messageID,
			Status:    pomomo.SessionRunning,
		},
		Settings: settings,
	}
	s.record.TimeRemainingAtStart = s.CurrentDuration()
	return s
}

func (s Session) Record() pomomo.SessionRecord {
	return s.record
}

type SessionSettings struct {
	Pomodoro, ShortBreak, LongBreak time.Duration
	Intervals                       int
}

type SessionStats struct {
	CompletedPomodoros int
}

func (s Session) TimeRemaining() time.Duration {
	return s.record.TimeRemainingAtStart - time.Since(s.record.IntervalStartedAt)
}

func (s Session) CurrentDuration() time.Duration {
	switch s.record.CurrentInterval {
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
		if s.record.CurrentInterval == pomomo.PomodoroInterval {
			s.Stats.CompletedPomodoros++
		}
	}

	// update interval
	var next pomomo.SessionInterval
	if s.record.CurrentInterval == pomomo.PomodoroInterval {
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
	s.record.CurrentInterval = next

	// update time/duration
	if s.record.Status == pomomo.SessionRunning && !s.record.IntervalStartedAt.IsZero() {
		// may need multiple calls to catch up
		s.record.IntervalStartedAt = s.record.IntervalStartedAt.Add(s.record.TimeRemainingAtStart)
	} else {
		s.record.IntervalStartedAt = time.Now()
	}
	s.record.TimeRemainingAtStart = s.CurrentDuration()
}

func (s Session) GuildID() string {
	return s.record.GuildID
}

func (s Session) ChannelID() string {
	return s.record.ChannelID
}

func (s Session) MessageID() string {
	return s.record.MessageID
}

func (s Session) Status() pomomo.SessionStatus {
	return s.record.Status
}

func (s Session) CurrentInterval() pomomo.SessionInterval {
	return s.record.CurrentInterval
}
