package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	txStdLib "github.com/Thiht/transactor/stdlib"
	"github.com/benjamonnguyen/deadsimple/config"
	dsdb "github.com/benjamonnguyen/deadsimple/database/sqlite"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/benjamonnguyen/pomomo-go/sqlite"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

const (
	RepoURL = "https://github.com/benjamonnguyen/pomomo-go"
	Version = "0.0.0"
)

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
	var pomodoroSoundPath, longBreakSoundPath, shortBreakSoundPath, idleSoundPath string
	panicif(cfg.GetMany([]config.Key{
		pomomo.DatabaseURLKey,
		pomomo.BotTokenKey,
		pomomo.BotNameKey,
		pomomo.PomodoroSoundPathKey,
		pomomo.LongBreakSoundPathKey,
		pomomo.ShortBreakSoundPathKey,
		pomomo.IdleSoundPathKey,
	}, &dbURL, &botToken, &botName, &pomodoroSoundPath, &longBreakSoundPath, &shortBreakSoundPath, &idleSoundPath))

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

	// set up discord cl
	cl, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}
	cl.ShouldRetryOnRateLimit = false
	// ShardID:                            0,
	// ShardCount:                         1,
	cl.Client = &http.Client{Timeout: (20 * time.Second)}
	cl.UserAgent = fmt.Sprintf("%s (%s, v%s)", botName, RepoURL, Version)
	cl.ShouldReconnectVoiceOnSessionError = false
	dm := NewDiscordMessenger(cl)

	audioPlayer := NewAudioPlayer(map[Audio]string{
		PomodoroAudio:   pomodoroSoundPath,
		LongBreakAudio:  longBreakSoundPath,
		ShortBreakAudio: shortBreakSoundPath,
		IdleAudio:       idleSoundPath,
	}, cl)

	// session objects
	sessionRepo := sqlite.NewSessionRepo(dbGetter, *log.Default())
	sessionManager := NewSessionManager(topCtx, sessionRepo, tx)
	sessionManager.OnSessionUpdate(func(ctx context.Context, s models.Session) {
		_, err := dm.EditChannelMessage(s.ChannelID(), s.MessageID(), SessionMessageComponents(s)...)
		if err != nil {
			log.Error("failed to edit discord channel message", "channelID", s.ChannelID(), "messageID", s.MessageID(), "sessionID", s.ID, "err", err)
		}
	})
	sessionManager.OnSessionNextInterval(func(ctx context.Context, s models.Session) {
		switch s.Status() {
		case pomomo.SessionRunning:
			var audio Audio
			switch s.CurrentInterval() {
			case pomomo.PomodoroInterval:
				audio = PomodoroAudio
			case pomomo.LongBreakInterval:
				audio = LongBreakAudio
			case pomomo.ShortBreakInterval:
				audio = ShortBreakAudio
			}
			if err := audioPlayer.Play(audio, s.GuildID(), s.ChannelID()); err != nil {
				log.Error("failed to play interval alert", "audio", audio, "guildID", s.GuildID(), "channelID", s.ChannelID(), "err", err)
			}
			log.Debug("played alert", "interval", s.Status())
			// case pomomo.SessionIdle:
			// 	audioPlayer.Play(IdleAudio, s.GuildID(), s.ChannelID())
		}
	})
	sessionManager.OnSessionCleanup(func(ctx context.Context, s models.Session) {
		_, err := dm.EditChannelMessage(s.ChannelID(), s.MessageID(), SessionMessageComponents(s)...)
		if err != nil {
			log.Error("failed to edit discord channel message", "channelID", s.ChannelID(), "messageID", s.MessageID(), "sessionID", s.ID, "err", err)
		}
		if err := cl.ChannelMessageUnpin(s.ChannelID(), s.MessageID()); err != nil {
			log.Error("failed to unpin discord channel message", "channelID", s.ChannelID(), "messageID", s.MessageID(), "sessionID", s.ID, "err", err)
		}
	})
	err = sessionManager.RestoreSessions()
	panicif(err)

	// command handler
	cm := NewCommandHandler(topCtx, sessionManager, dm)
	cl.AddHandler(cm.StartSession)
	cl.AddHandler(cm.SkipInterval)
	cl.AddHandler(cm.EndSession)

	// open connection
	if err := cl.Open(); err != nil {
		log.Fatal("Error opening connection", "err", err)
	}
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
		audioPlayer.Close()
		if err := cl.Close(); err != nil {
			log.Error(err)
		}
		c()
	}()
	<-shutdownTimeout.Done()
	if shutdownTimeout.Err() != context.Canceled {
		log.Error("failed to shut down gracefully", "err", shutdownTimeout.Err())
	}
}

func panicif(err error) {
	if err != nil {
		panic(err)
	}
}
