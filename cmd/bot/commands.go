package main

import (
	"context"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
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
	parentCtx        context.Context
	sessionManager   SessionManager
	discordMessenger DiscordMessenger
}

func NewCommandHandler(parentCtx context.Context, sm SessionManager, dm DiscordMessenger) CommandHandler {
	return &commandHandler{
		parentCtx:        parentCtx,
		sessionManager:   sm,
		discordMessenger: dm,
	}
}

func (h *commandHandler) StartSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := m.ApplicationCommandData()
	if data.Name != pomomo.StartCommand.Name {
		return
	}

	// Parse command options with defaults
	settings := models.SessionSettings{
		Pomodoro:   20 * time.Minute,
		ShortBreak: 5 * time.Minute,
		LongBreak:  15 * time.Minute,
		Intervals:  4,
	}
	for _, opt := range data.Options {
		val, ok := opt.Value.(float64)
		if !ok {
			continue
		}
		intVal := int(val)
		switch opt.Name {
		case pomomo.PomodoroOption:
			settings.Pomodoro = time.Duration(intVal) * time.Minute
		case pomomo.ShortBreakOption:
			settings.ShortBreak = time.Duration(intVal) * time.Minute
		case pomomo.LongBreakOption:
			settings.LongBreak = time.Duration(intVal) * time.Minute
		case pomomo.IntervalsOption:
			settings.Intervals = intVal
		}
	}

	if h.sessionManager.HasSession(m.GuildID, m.ChannelID) {
		if _, err := h.discordMessenger.Respond(m.Interaction, false, TextDisplay("This channel already has an active session.")); err != nil {
			log.Error(err)
		}
		return
	}

	// get voice channel
	vs, err := s.State.VoiceState(m.GuildID, m.Member.User.ID)
	if err != nil {
		log.Error("failed to get voice state", "userID", m.Member.User.ID, "guildID", m.GuildID, "err", err)
		_, err = h.discordMessenger.Respond(m.Interaction, false, TextDisplay("Pomomo couldn't find your voice channel. Please join a voice channel with permissions and try again."))
		if err != nil {
			log.Error(err)
		}
		return
	}
	if h.sessionManager.HasVoiceConnection(vs.ChannelID) {
		_, err = h.discordMessenger.Respond(m.Interaction, false, TextDisplay("Your voice channel already has an active session. Please join another voice channel and try again."))
		if err != nil {
			log.Error(err)
		}
		return
	}

	session := models.NewSession("", m.GuildID, m.ChannelID, vs.ChannelID, "", settings)
	session.GoNextInterval(false) // initialize fields for display - "real" session is created by sessionManager
	msg, err := h.discordMessenger.Respond(m.Interaction, true, SessionMessageComponents(session)...)
	if err != nil {
		log.Error(err)
		return
	}

	session, err = h.sessionManager.StartSession(h.parentCtx, startSessionRequest{
		guildID:   m.GuildID,
		textCID:   m.ChannelID,
		voiceCID:  vs.ChannelID,
		messageID: msg.ID,
		settings:  settings,
	})
	if err != nil {
		log.Error("failed to start session", "err", err)
		if _, err := h.discordMessenger.EditResponse(m.Interaction, TextDisplay("Failed to start session.")); err != nil {
			log.Error(err)
		}
		return
	}
	log.Debug("started session", "id", session.ID)

	if err := s.ChannelMessagePin(m.ChannelID, msg.ID); err != nil {
		log.Error("failed to pin message", "err", err)
	}

	// TODO SessionManager goroutine to update stats with lesser frequency

	// TODO impl Resume/Pause
	// TODO impl Stop
}

func (h *commandHandler) SkipInterval(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := m.MessageComponentData()
	id, err := FromCustomID(data.CustomID)
	if err != nil {
		return
	}
	if id.Type != "skip" {
		return
	}

	followup, err := h.discordMessenger.DeferMessageUpdate(m.Interaction)
	if err != nil {
		log.Error(err)
		return
	}

	session, err := h.sessionManager.SkipInterval(h.parentCtx, sessionKey{
		guildID:   id.GuildID,
		channelID: id.ChannelID,
	})
	if err != nil {
		log.Error("failed to skip interval", "err", err)
		components := append(SessionMessageComponents(session), TextDisplay(defaultErrorMsg))
		if _, err := followup(components...); err != nil {
			log.Error(err)
		}
		return
	}
	log.Debug("skipped interval", "new", session.Record.CurrentInterval)

	_, err = followup(SessionMessageComponents(session)...)
	if err != nil {
		log.Error(err)
		return
	}
}

func (h *commandHandler) EndSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := m.MessageComponentData()
	id, err := FromCustomID(data.CustomID)
	if err != nil {
		return
	}
	if id.Type != "end" {
		return
	}

	followup, err := h.discordMessenger.DeferMessageUpdate(m.Interaction)
	if err != nil {
		log.Error(err)
		return
	}

	session, err := h.sessionManager.EndSession(h.parentCtx, sessionKey{
		guildID:   id.GuildID,
		channelID: id.ChannelID,
	})
	if err != nil {
		log.Error("failed to end session", "err", err)
		components := append(SessionMessageComponents(session), TextDisplay(defaultErrorMsg))
		if _, err := followup(components...); err != nil {
			log.Error(err)
		}
		return
	}
	log.Debug("ended session", "id", session.ID)

	_, err = followup(SessionMessageComponents(session)...)
	if err != nil {
		log.Error(err)
		return
	}

	// Unpin the session message
	if err := s.ChannelMessageUnpin(id.ChannelID, m.Message.ID); err != nil {
		log.Error("failed to unpin message", "customID", id.ToCustomID(), "message", m.Message.ID, "err", err)
	}
}

func (h *commandHandler) TogglePause(s *discordgo.Session, m *discordgo.InteractionCreate) {
	panic("not implemented")
}

func (h *commandHandler) JoinSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	panic("not implemented")
}
