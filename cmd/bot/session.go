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
	currentInterval               SessionInterval
	intervalStartedAt             time.Time // When current interval started
	status                        pomomo.SessionStatus
	// TODO connection instance
}

func (s Session) toRecord() pomomo.SessionRecord {
	return pomomo.SessionRecord{
		GuildID:           s.guildID,
		ChannelID:         s.channelID,
		MessageID:         s.messageID,
		CurrentInterval:   s.currentInterval.enum(),
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

type SessionInterval string

const (
	PomodoroInterval   SessionInterval = "Pomodoro"
	ShortBreakInterval SessionInterval = "Short Break"
	LongBreakInterval  SessionInterval = "Long Break"
)

func (i SessionInterval) enum() pomomo.SessionInterval {
	switch i {
	case PomodoroInterval:
		return pomomo.PomodoroInterval
	case ShortBreakInterval:
		return pomomo.ShortBreakInterval
	case LongBreakInterval:
		return pomomo.LongBreakInterval
	default:
		panic("no matching enum for SessionInterval: " + string(i))
	}
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
		if s.currentInterval == PomodoroInterval {
			s.stats.completedPomodoros++
		}
	}

	// update interval
	var next SessionInterval
	switch s.currentInterval {
	case PomodoroInterval:
		// After pomodoro, decide break type based on completed pomodoros
		if s.stats.completedPomodoros > 0 && s.stats.completedPomodoros%s.settings.intervals == 0 {
			next = LongBreakInterval
		} else {
			next = ShortBreakInterval
		}
	case ShortBreakInterval, LongBreakInterval:
		// After any break, next is pomodoro
		next = PomodoroInterval
	}
	s.currentInterval = next
	s.intervalStartedAt = time.Now()
}

func (s Session) RemainingTime() time.Duration {
	return s.CurrentDuration() - time.Since(s.intervalStartedAt)
}

func (s Session) CurrentDuration() time.Duration {
	switch s.currentInterval {
	case PomodoroInterval:
		return s.settings.pomodoro
	case ShortBreakInterval:
		return s.settings.shortBreak
	case LongBreakInterval:
		return s.settings.longBreak
	default:
		panic("unexpected interval state")
	}
}
