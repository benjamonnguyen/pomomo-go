package main

import (
	"context"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/dgutils"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

type CommandHandler interface {
	StartSession(s *discordgo.Session, m *discordgo.InteractionCreate)
	EditSession(s *discordgo.Session, m *discordgo.InteractionCreate)
}

type commandHandler struct {
	sessionManager SessionManager
}

func NewCommandHandler(sm SessionManager) CommandHandler {
	return &commandHandler{
		sessionManager: sm,
	}
}

func (h *commandHandler) timeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}

func (h *commandHandler) StartSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := m.ApplicationCommandData()
	if data.Name != pomomo.StartCommand.Name {
		return
	}

	r := dgutils.NewInteractionResponder(s, m.Interaction)
	if err := r.DeferResponse(); err != nil {
		log.Error(err)
		return
	}

	timeout, c := h.timeout()
	defer c()

	// Parse command options with defaults
	settings := SessionSettings{
		pomodoro:   20 * time.Minute,
		shortBreak: 5 * time.Minute,
		longBreak:  15 * time.Minute,
		intervals:  4,
	}
	for _, opt := range data.Options {
		val, ok := opt.Value.(float64)
		if !ok {
			continue
		}
		intVal := int(val)
		switch opt.Name {
		case pomomo.PomodoroOption:
			settings.pomodoro = time.Duration(intVal) * time.Minute
		case pomomo.ShortBreakOption:
			settings.shortBreak = time.Duration(intVal) * time.Minute
		case pomomo.LongBreakOption:
			settings.longBreak = time.Duration(intVal) * time.Minute
		case pomomo.IntervalsOption:
			settings.intervals = intVal
		}
	}

	req := startSessionRequest{
		guildID:   m.GuildID,
		channelID: m.ChannelID,
		settings:  settings,
	}

	session, err := h.sessionManager.StartSession(timeout, req)
	if err != nil {
		log.Error("failed to start session", "err", err)
		if _, err := r.FollowupWithMessage("Failed to start session"); err != nil {
			log.Error(err)
		}
		return
	}

	if _, err := r.FollowupWithMessage("Session started! Session ID: " + session.sessionID); err != nil {
		log.Error(err)
		return
	}

	// TODO skip button (to help with quickly testing)
	// TODO SessionManager goroutine to update session
	// TODO go next interval with sound (ffmpeg?)

	// TODO impl Resume/Pause
	// TODO impl Stop
}

func (h *commandHandler) EditSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	// TODO EditSession
}
