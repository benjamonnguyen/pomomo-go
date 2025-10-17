package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/commands"
	"github.com/bwmarrin/discordgo"
	_ "modernc.org/sqlite"
)

func main() {
	// config
	pomomo.LoadEnv()

	// db
	initDB(os.Getenv("POMOMO_DB_PATH"))

	_ = os.Getenv("POMOMO_SUPER_USER_ID")
	botName := os.Getenv("POMOMO_BOT_NAME")
	if botName == "" {
		botName = "Pomomo"
	}

	// set up bot
	token := os.Getenv("POMOMO_BOT_TOKEN")
	if token == "" {
		log.Fatalln("provide POMOMO_BOT_TOKEN")
	}
	bot, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalln(err)
	}

	// COMMAND HANDLERS
	bot.AddHandler(commands.HandleStartCommand)

	// open connection
	if err := bot.Open(); err != nil {
		log.Fatalln("Error opening connection:", err)
	}
	defer bot.Close()

	//
	fmt.Println(botName + " bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	fmt.Println("Terminating " + botName)
}
