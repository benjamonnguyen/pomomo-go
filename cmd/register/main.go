package main

import (
	"context"
	"fmt"
	"time"

	"github.com/benjamonnguyen/deadsimple/cfg"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
)

func main() {
	// Set global logger to debug level
	log.SetLevel(log.DebugLevel)

	config, err := pomomo.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	var botToken string
	panicif(config.GetMany([]cfg.Key{
		pomomo.BotTokenKey,
	}, &botToken))

	// commands are registered over REST, no gateway connection needed
	client := rest.New(rest.NewClient(botToken))
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Close(ctx)
	}()

	app, err := client.GetCurrentApplication()
	if err != nil {
		log.Fatal("failed getting application", "err", err)
	}

	cmds := []discord.ApplicationCommandCreate{
		pomomo.StartCommand,
		pomomo.StatsCommand,
	}

	created, err := client.SetGlobalCommands(app.ID, cmds)
	if err != nil {
		log.Fatal(err)
	}

	for _, cmd := range created {
		fmt.Printf("%s: %s\n", cmd.Name(), cmd.ID())
	}
}

func panicif(err error) {
	if err != nil {
		panic(err)
	}
}
