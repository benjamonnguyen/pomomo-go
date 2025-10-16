package pomomo

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

var StartCommand = discordgo.ApplicationCommand{
	Name:        "start",
	Description: "start pomodoro session",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "pomodoro",
			Description: "pomodoro duration in minutes (Default: 20)",
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "short_break",
			Description: "short break duration in minutes (Default: 5)",
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "long_break",
			Description: "long break duration in minutes (Default: 15)",
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "intervals",
			Description: "number of intervals between long breaks (Default: 4)",
		},
	},
}

func HandleStartCommand(s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := m.ApplicationCommandData()
	if data.Name != StartCommand.Name {
		return
	}

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
