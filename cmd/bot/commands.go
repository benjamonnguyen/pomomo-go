package main

import (
	"context"
	"fmt"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/dgutils"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

const (
	skipButtonPrefix = "skip_"
)

type CommandHandler interface {
	StartSession(s *discordgo.Session, m *discordgo.InteractionCreate)
	EditSettings(s *discordgo.Session, m *discordgo.InteractionCreate)
	SkipInterval(s *discordgo.Session, m *discordgo.InteractionCreate)
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
		if _, err := r.Followup(discordgo.WebhookParams{Content: "Failed to start session"}); err != nil {
			log.Error(err)
		}
		return
	}
	log.Debug("started session", "id", session.sessionID)

	// Create skip button
	skipButton := discordgo.Button{
		Label:    "Skip",
		Style:    discordgo.PrimaryButton,
		CustomID: skipButtonPrefix + session.sessionID,
	}

	actionRow := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{skipButton},
	}
	msg, err := r.Followup(discordgo.WebhookParams{
		Content:    fmt.Sprintf("%+v", session),
		Components: []discordgo.MessageComponent{actionRow},
	})
	if err != nil {
		log.Error(err)
		return
	}
	if err := s.ChannelMessagePin(m.ChannelID, msg.ID); err != nil {
		log.Error("failed to pin message", "err", err)
	}

	// TODO SessionManager goroutine to update session
	// TODO go next interval with sound (ffmpeg?)

	// TODO impl Resume/Pause
	// TODO impl Stop
}

func (h *commandHandler) SkipInterval(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := m.MessageComponentData()
	customID := data.CustomID

	// Handle skip button
	if len(customID) > len(skipButtonPrefix) && customID[:len(skipButtonPrefix)] == skipButtonPrefix {
		sessionID := customID[len(skipButtonPrefix):]

		// Acknowledge interaction within 3 seconds
		err := s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		if err != nil {
			log.Error("failed to acknowledge button interaction", "err", err)
			return
		}

		// TODO: Actually skip the interval using sessionManager
		log.Debug("skip button pressed", "sessionID", sessionID)

		// Update the original message to show interval was skipped
		content := "Interval skipped! Session ID: " + sessionID
		_, err = s.InteractionResponseEdit(m.Interaction, &discordgo.WebhookEdit{
			Content: &content,
		})
		if err != nil {
			log.Error("failed to update message after skip", "err", err)
		}
	}
}

func (h *commandHandler) EditSettings(s *discordgo.Session, m *discordgo.InteractionCreate) {
	// TODO EditSettings
}
