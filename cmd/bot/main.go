package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	_ = os.Getenv("POMOMO_SUPER_USER_ID")
	botName := os.Getenv("POMOMO_BOT_NAME")
	if botName == "" {
		botName = "Pomomo"
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

	// handlers
	bot.AddHandler(pomomo.HandleStartCommand)

	// Open a connection
	if err := bot.Open(); err != nil {
		log.Fatalln("Error opening connection:", err)
	}
	defer bot.Close()

	// Wait for a termination signal
	fmt.Println(botName + " bot is now running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	fmt.Println("Terminating " + botName)
}

// TODO replace usage with followUpper
func followUpMessageCreate(content string, s *discordgo.Session, it *discordgo.Interaction) {
	s.FollowupMessageCreate(it, false, &discordgo.WebhookParams{
		Content: content,
	})
}

type followUpper struct {
	s  *discordgo.Session
	it *discordgo.Interaction
}

func (fu followUpper) CreateMsg(format string, a ...any) {
	fu.s.FollowupMessageCreate(fu.it, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf(format, a...),
	})
}

func getUser(m *discordgo.Interaction) *discordgo.User {
	if m.Member != nil {
		return m.Member.User
	}
	return m.User
}
