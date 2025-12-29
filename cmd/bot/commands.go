package main

import (
	"context"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/dgutils"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

const (
	defaultErrorMsg = "Looks like something went wrong. Try again in a bit or reach out to support."
)

type CommandHandler interface {
	StartSession(s *discordgo.Session, m *discordgo.InteractionCreate)
	SkipInterval(s *discordgo.Session, m *discordgo.InteractionCreate)
	EndSession(s *discordgo.Session, m *discordgo.InteractionCreate)
	TogglePause(s *discordgo.Session, m *discordgo.InteractionCreate)
	JoinSession(s *discordgo.Session, m *discordgo.InteractionCreate)
}

type commandHandler struct {
	parentCtx      context.Context
	sessionManager SessionManager
}

func NewCommandHandler(parentCtx context.Context, sm SessionManager) CommandHandler {
	return &commandHandler{
		parentCtx:      parentCtx,
		sessionManager: sm,
	}
}

func (h *commandHandler) timeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(h.parentCtx, 10*time.Second)
}

func (h *commandHandler) StartSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := m.ApplicationCommandData()
	if data.Name != pomomo.StartCommand.Name {
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

	if h.sessionManager.HasSession(m.GuildID, m.ChannelID) {
		if _, err := dgutils.Respond(s, m.Interaction, false, dgutils.TextDisplay("This channel already has an active session.")); err != nil {
			log.Error(err)
		}
		return
	}

	session := Session{
		channelID:         m.ChannelID,
		guildID:           m.GuildID,
		settings:          settings,
		currentInterval:   PomodoroInterval,
		intervalStartedAt: time.Now(),
	}
	msg, err := dgutils.Respond(s, m.Interaction, true, session.MessageComponents()...)
	if err != nil {
		log.Error(err)
		return
	}

	session, err = h.sessionManager.StartSession(timeout, startSessionRequest{
		channelID: m.ChannelID,
		guildID:   m.GuildID,
		messageID: msg.ID,
		settings:  settings,
	})
	if err != nil {
		log.Error("failed to start session", "err", err)
		if _, err := dgutils.EditResponse(s, m.Interaction, dgutils.TextDisplay("Failed to start session.")); err != nil {
			log.Error(err)
		}
		return
	}
	log.Debug("started session", "id", session.sessionID)

	if err := s.ChannelMessagePin(m.ChannelID, msg.ID); err != nil {
		log.Error("failed to pin message", "err", err)
	}

	// TODO SessionManager goroutine to update session msg info and go next
	// TODO go next interval with sound (ffmpeg?)
	// TODO SessionManager goroutine to update stats with lesser frequency

	// TODO impl Resume/Pause
	// TODO impl Stop
}

func (h *commandHandler) SkipInterval(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := m.MessageComponentData()
	id, err := dgutils.FromCustomID(data.CustomID)
	if err != nil {
		return
	}
	if id.Type != "skip" {
		return
	}

	timeout, c := h.timeout()
	defer c()

	followup, err := dgutils.DeferMessageUpdate(s, m.Interaction)
	if err != nil {
		log.Error(err)
		return
	}

	session, err := h.sessionManager.SkipInterval(timeout, cacheKey{
		guildID:   id.GuildID,
		channelID: id.ChannelID,
	})
	if err != nil {
		log.Error("failed to skip interval", "err", err)
		components := append(session.MessageComponents(), dgutils.TextDisplay(defaultErrorMsg))
		if _, err := followup(components...); err != nil {
			log.Error(err)
		}
		return
	}
	log.Debug("skipped interval", "new", session.currentInterval)

	_, err = followup(session.MessageComponents()...)
	if err != nil {
		log.Error(err)
		return
	}
}

func (h *commandHandler) EndSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	panic("not implemented")
}

func (h *commandHandler) TogglePause(s *discordgo.Session, m *discordgo.InteractionCreate) {
	panic("not implemented")
}

func (h *commandHandler) JoinSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	panic("not implemented")
}
