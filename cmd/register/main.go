package main

import (
	"fmt"

	"github.com/benjamonnguyen/deadsimple/config"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

func main() {
	cfg, err := pomomo.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	var botToken string
	panicif(cfg.GetMany([]config.Key{
		pomomo.BotTokenKey,
	}, &botToken))

	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}

	// Open a connection
	if err := bot.Open(); err != nil {
		log.Fatal("failed opening connection", "err", err)
	}
	defer bot.Close() //nolint

	app, _ := bot.Application("@me")

	cmds := []*discordgo.ApplicationCommand{
		&pomomo.StartCommand,
	}

	created, err := bot.ApplicationCommandBulkOverwrite(app.ID, "", cmds)
	if err != nil {
		log.Fatal(err)
	}

	for _, cmd := range created {
		fmt.Printf("%s: %s\n", cmd.Name, cmd.Description)
	}
}

func panicif(err error) {
	if err != nil {
		panic(err)
	}
}
