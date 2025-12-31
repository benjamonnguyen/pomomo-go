package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/dgutils"
	"github.com/bwmarrin/discordgo"
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

func (s Session) key() cacheKey {
	return cacheKey{
		guildID:   s.guildID,
		channelID: s.channelID,
	}
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

func (s Session) TimerBar() string {
	const length = 20
	filledChar := timerBarFilledChar
	emptyChar := timerBarEmptyChar
	remaining := s.RemainingTime().Minutes()
	if remaining <= 0 {
		return strings.Repeat(emptyChar, length)
	}
	percentage := remaining / s.CurrentDuration().Minutes()
	filled := min(int(math.Round(percentage*length*10)/10), length)
	return strings.Repeat(filledChar, filled) + strings.Repeat(emptyChar, length-filled)
}

func (s Session) MessageComponents() []discordgo.MessageComponent {
	if s.status == pomomo.SessionEnded {
		return []discordgo.MessageComponent{
			getEndMessage(),
		}
	}
	// action row
	skipButton := discordgo.Button{
		Label: "Skip",
		Style: discordgo.PrimaryButton,
		CustomID: dgutils.InteractionID{
			Type:      "skip",
			GuildID:   s.guildID,
			ChannelID: s.channelID,
		}.ToCustomID(),
	}
	endButton := discordgo.Button{
		Label: "End",
		Style: discordgo.DangerButton,
		CustomID: dgutils.InteractionID{
			Type:      "end",
			GuildID:   s.guildID,
			ChannelID: s.channelID,
		}.ToCustomID(),
	}

	actionRow := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{skipButton, endButton},
	}

	// settings
	settingsTextParts := []string{
		"### Session Settings",
		fmt.Sprintf("%s: %d min", PomodoroInterval, int(s.settings.pomodoro.Minutes())),
		fmt.Sprintf("%s: %d min", ShortBreakInterval, int(s.settings.shortBreak.Minutes())),
		fmt.Sprintf("%s: %d min", LongBreakInterval, int(s.settings.longBreak.Minutes())),
		fmt.Sprintf("%s: %d | %d", "Interval", s.stats.completedPomodoros%s.settings.intervals, s.settings.intervals),
	}
	switch s.currentInterval {
	case PomodoroInterval:
		settingsTextParts[1] = fmt.Sprintf("**%s**\n%s", settingsTextParts[1], s.TimerBar())
	case ShortBreakInterval:
		settingsTextParts[2] = fmt.Sprintf("**%s**\n%s", settingsTextParts[2], s.TimerBar())
	case LongBreakInterval:
		settingsTextParts[3] = fmt.Sprintf("**%s**\n%s", settingsTextParts[3], s.TimerBar())
	}
	accentColor := dgutils.ColorGreen
	if s.status == pomomo.SessionPaused {
		accentColor = dgutils.ColorLightGrey
	}
	settingsContainer := discordgo.Container{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{
				Content: strings.Join(settingsTextParts, "\n"),
			},
		},
		AccentColor: accentColor.ToInt(),
	}

	//
	return []discordgo.MessageComponent{
		getStartMessage(),
		settingsContainer,
		actionRow,
	}
}

func getStartMessage() discordgo.MessageComponent {
	return dgutils.TextDisplay("It's productivity o'clock!")
}

func getEndMessage() discordgo.MessageComponent {
	// TODO display stats in end message
	return dgutils.TextDisplay("Good stuff!")
}
