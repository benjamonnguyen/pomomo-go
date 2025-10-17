package commands

import (
	"log"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/mydiscordgo"
	"github.com/bwmarrin/discordgo"
)

func HandleStartCommand(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := m.ApplicationCommandData()
	if data.Name != pomomo.StartCommand.Name {
		return
	}

	r := mydiscordgo.NewInteractionResponder(s, m.Interaction)
	if err := r.DeferResponse(); err != nil {
		log.Println(err)
		return
	}

	// create session TODO

	// TODO HandleStartCommand
	err := s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "test",
		},
	})
	if err != nil {
		log.Fatalln(err)
	}

	// TODO handle err
	// _ = s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
	// 	Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	// })
}
