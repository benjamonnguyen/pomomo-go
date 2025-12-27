package main

import (
	"embed"
	"os"
	"os/signal"
	"syscall"

	txStdLib "github.com/Thiht/transactor/stdlib"
	"github.com/benjamonnguyen/deadsimple/config"
	dsdb "github.com/benjamonnguyen/deadsimple/database/sqlite"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/sqlite"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func main() {
	// config
	cfg, err := pomomo.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	var dbURL, botToken, botName string
	panicif(cfg.GetMany([]config.Key{
		pomomo.DatabaseURLKey,
		pomomo.BotTokenKey,
		pomomo.BotNameKey,
	}, &dbURL, &botToken, &botName))

	// db
	log.Info("opening db", "url", dbURL)
	db, err := dsdb.Open(dbURL)
	if err != nil {
		log.Fatal("failed database open", "err", err)
	}
	if err := db.RunMigrations(migrations); err != nil {
		log.Fatal("failed migration", "err", err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx, dbGetter := txStdLib.NewTransactor(
		db.DB(),
		txStdLib.NestedTransactionsSavepoints,
	)

	// service objects
	sessionRepo := sqlite.NewSessionRepo(dbGetter, *log.Default())
	sessionManager := NewSessionManager(sessionRepo, tx)

	// set up bot
	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}

	// command handler
	cm := NewCommandHandler(sessionManager)
	bot.AddHandler(cm.StartSession)

	// open connection
	if err := bot.Open(); err != nil {
		log.Fatal("Error opening connection", "err", err)
	}
	defer bot.Close() //nolint

	//
	log.Info(botName + " running. Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Info("terminating " + botName)
}

func panicif(err error) {
	if err != nil {
		panic(err)
	}
}
