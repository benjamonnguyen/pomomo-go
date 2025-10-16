package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var isProd bool

func main() {
	flag.BoolVar(&isProd, "prod", false, "")
	flag.Parse()
	if isProd {
		godotenv.Load(".env")
	} else {
		godotenv.Load(".env.dev")
	}

	//
	token := os.Getenv("POMOMO_BOT_TOKEN")
	if token == "" {
		log.Fatalln("provide POMOMO_BOT_TOKEN")
	}
	bot, err := discordgo.New("Bot " + token)
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
