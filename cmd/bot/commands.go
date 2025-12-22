package main

import (
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/dgutils"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

type CommandHandler interface {
	HandleStartCommand(s *discordgo.Session, m *discordgo.InteractionCreate)
}

type commandHandler struct {
	sessionManager SessionManager
}

func NewCommandHandler(sm SessionManager) CommandHandler {
	return &commandHandler{
		sessionManager: sm,
	}
}

func (h *commandHandler) HandleStartCommand(s *discordgo.Session, m *discordgo.InteractionCreate) {
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

	_, err := r.FollowupWithMessage("test")
	if err != nil {
		log.Error(err)
		return
	}

	// create session TODO

	// TODO HandleStartCommand

	// TODO handle err
	// _ = s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
	// 	Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	// })
}
