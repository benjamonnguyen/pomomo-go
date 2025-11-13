package main

import (
	"fmt"
	"log"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/bwmarrin/discordgo"
)

func main() {
	config, err := pomomo.LoadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	//
	bot, err := discordgo.New("Bot " + config.BotToken)
	if err != nil {
		log.Fatalln(err)
	}

	// Open a connection
	if err := bot.Open(); err != nil {
		log.Fatalln("Error opening connection:", err)
	}
	defer bot.Close()

	app, _ := bot.Application("@me")

	cmds := []*discordgo.ApplicationCommand{
		&pomomo.StartCommand,
	}

	created, err := bot.ApplicationCommandBulkOverwrite(app.ID, "", cmds)
	if err != nil {
		log.Fatalln(err)
	}

	for _, cmd := range created {
		fmt.Printf("%s: %s\n", cmd.Name, cmd.Description)
	}
}
