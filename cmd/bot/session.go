package main

import (
	"time"

	"github.com/benjamonnguyen/pomomo-go"
)

const (
	timerBarFilledChar = "⣶"
	timerBarEmptyChar  = "⡀"
)

type Session struct {
	sessionID, guildID, channelID string
	messageID                     string // Discord message ID
	settings                      SessionSettings
	stats                         SessionStats
	currentInterval               pomomo.SessionInterval
	intervalStartedAt             time.Time // When current interval started
	status                        pomomo.SessionStatus
	// TODO voice connection instance
}

// TODO stats

func NewSession(session pomomo.ExistingSessionRecord, settings pomomo.ExistingSessionSettingsRecord) Session {
	return Session{
		sessionID:         session.ID,
		guildID:           session.GuildID,
		channelID:         session.ChannelID,
		messageID:         session.MessageID,
		currentInterval:   session.CurrentInterval,
		intervalStartedAt: session.IntervalStartedAt,
		status:            session.Status,

		settings: SessionSettings{
			pomodoro:   settings.Pomodoro,
			shortBreak: settings.ShortBreak,
			longBreak:  settings.LongBreak,
			intervals:  settings.Intervals,
		},
	}
}

func (s Session) toRecord() pomomo.SessionRecord {
	return pomomo.SessionRecord{
		GuildID:           s.guildID,
		ChannelID:         s.channelID,
		MessageID:         s.messageID,
		CurrentInterval:   s.currentInterval,
		IntervalStartedAt: s.intervalStartedAt,
		Status:            s.status,
	}
}

type SessionSettings struct {
	pomodoro, shortBreak, longBreak time.Duration
	intervals                       int
}

type SessionStats struct {
	completedPomodoros int
}

func (s Session) key() sessionKey {
	key := sessionKey{
		guildID:   s.guildID,
		channelID: s.channelID,
	}
	if err := key.validate(); err != nil {
		panic(err)
	}
	return key
}

func (s *Session) goNextInterval(shouldUpdateStats bool) {
	// update stats
	if shouldUpdateStats {
		if s.currentInterval == pomomo.PomodoroInterval {
			s.stats.completedPomodoros++
		}
	}

	// update interval
	var next pomomo.SessionInterval
	switch s.currentInterval {
	case pomomo.PomodoroInterval:
		// After pomodoro, decide break type based on completed pomodoros
		if s.stats.completedPomodoros > 0 && s.stats.completedPomodoros%s.settings.intervals == 0 {
			next = pomomo.LongBreakInterval
		} else {
			next = pomomo.ShortBreakInterval
		}
	case pomomo.ShortBreakInterval, pomomo.LongBreakInterval:
		// After any break, next is pomodoro
		next = pomomo.PomodoroInterval
	}
	s.currentInterval = next
	s.intervalStartedAt = time.Now()
}

func (s Session) RemainingTime() time.Duration {
	return s.CurrentDuration() - time.Since(s.intervalStartedAt)
}

func (s Session) CurrentDuration() time.Duration {
	switch s.currentInterval {
	case pomomo.PomodoroInterval:
		return s.settings.pomodoro
	case pomomo.ShortBreakInterval:
		return s.settings.shortBreak
	case pomomo.LongBreakInterval:
		return s.settings.longBreak
	default:
		panic("unexpected interval state")
	}
}
