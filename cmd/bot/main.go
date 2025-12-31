package main

import (
	"context"
	"embed"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	// logger
	log.SetLevel(log.DebugLevel)
	log.SetReportCaller(true)
	topCtx, c := context.WithCancel(context.Background())

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

	// set up bot
	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}

	// service objects
	sessionRepo := sqlite.NewSessionRepo(dbGetter, *log.Default())
	sessionManager := NewSessionManager(sessionRepo, tx, bot)

	// command handler
	cm := NewCommandHandler(topCtx, sessionManager)
	bot.AddHandler(cm.StartSession)
	bot.AddHandler(cm.SkipInterval)

	// open connection
	if err := bot.Open(); err != nil {
		log.Fatal("Error opening connection", "err", err)
	}
	defer bot.Close() //nolint
	log.Info(botName + " running. Press CTRL-C to exit.")

	// graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Info("terminating " + botName)
	c()
	shutdownTimeout, c := context.WithTimeout(context.Background(), time.Minute)
	go func() {
		if err := sessionManager.Shutdown(); err != nil {
			log.Error(err)
		}
		c()
	}()
	<-shutdownTimeout.Done()
}

func panicif(err error) {
	if err != nil {
		panic(err)
	}
}
