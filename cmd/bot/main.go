package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/benjamonnguyen/deadsimple/database/sqlite"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/commands"
	"github.com/bwmarrin/discordgo"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	// config
	config, err := pomomo.LoadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	// db
	log.Println("opening db at", config.DatabaseURL)
	db, err := sqlite.Open(config.DatabaseURL)
	if err != nil {
		log.Fatalln("failed database open:", err)
	}
	if err := db.RunMigrations(migrations); err != nil {
		log.Fatalln("failed migration:", err)
	}
	defer func() {
		_ = db.Close()
	}()

	// set up bot
	bot, err := discordgo.New("Bot " + config.BotToken)
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
	fmt.Println(config.BotName + " bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	fmt.Println("Terminating " + config.BotName)
}
