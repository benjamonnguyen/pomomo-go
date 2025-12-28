package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/benjamonnguyen/pomomo-go/cmd/bot/dgutils"
	"github.com/bwmarrin/discordgo"
)

type Session struct {
	sessionID, guildID, channelID string
	settings                      SessionSettings
	stats                         SessionStats
	currentInterval               SessionInterval
	// TODO connection instance
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
}

func (s Session) MessageComponents() []discordgo.MessageComponent {
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

	actionRow := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{skipButton},
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
		settingsTextParts[1] = bold(settingsTextParts[1])
	case ShortBreakInterval:
		settingsTextParts[2] = bold(settingsTextParts[2])
	case LongBreakInterval:
		settingsTextParts[3] = bold(settingsTextParts[3])
	}
	settingsContainer := discordgo.Container{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{
				Content: strings.Join(settingsTextParts, "\n"),
			},
		},
		AccentColor: dgutils.ColorGreen.ToInt(),
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

func bold(s string) string {
	return fmt.Sprintf("**%s**", s)
}
