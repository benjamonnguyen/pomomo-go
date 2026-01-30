package main

import (
	"fmt"
	"strconv"

	"github.com/benjamonnguyen/deadsimple/config"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

func main() {
	// Set global logger to debug level
	log.SetLevel(log.DebugLevel)

	cfg, err := pomomo.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	var botToken, shardCnt, shardID string
	panicif(cfg.GetMany([]config.Key{
		pomomo.BotTokenKey,
		pomomo.ShardCountKey,
		pomomo.ShardIDKey,
	}, &botToken, &shardCnt, &shardID))

	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}
	if shardCnt != "" {
		n, err := strconv.Atoi(shardCnt)
		panicif(err)
		bot.ShardCount = n
	}
	if shardID != "" {
		n, err := strconv.Atoi(shardID)
		panicif(err)
		bot.ShardID = n
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
