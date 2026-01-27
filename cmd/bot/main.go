package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	txStdLib "github.com/Thiht/transactor/stdlib"
	"github.com/benjamonnguyen/deadsimple/config"
	dsdb "github.com/benjamonnguyen/deadsimple/database/sqlite"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/benjamonnguyen/pomomo-go/discordgo"
	"github.com/benjamonnguyen/pomomo-go/sqlite"
	dg "github.com/bwmarrin/discordgo"
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
	topCtx, topCtxC := context.WithCancel(context.Background())
	initTimeout, initTimeoutC := context.WithTimeout(topCtx, 10*time.Second)

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
	defer db.Close() //nolint

	tx, dbGetter := txStdLib.NewTransactor(
		db.DB(),
		txStdLib.NestedTransactionsSavepoints,
	)

	// TODO all service objects should take a logger arg
	sessionRepo := sqlite.NewSessionRepo(dbGetter, *log.Default())
	participantsProvider := NewParticipantsProvider(sessionRepo, *log.Default())
	panicif(participantsProvider.RestoreCache(initTimeout))

	// set up discord cl
	cl, err := dg.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}
	cl.ShouldRetryOnRateLimit = false
	// ShardID:                            0,
	// ShardCount:                         1,
	cl.Client = &http.Client{Timeout: (20 * time.Second)}
	cl.UserAgent = fmt.Sprintf("%s (%s, v%s)", botName, RepoURL, Version)
	cl.ShouldReconnectVoiceOnSessionError = true

	dm := NewDiscordMessenger(cl)
	discordAdapter := discordgo.NewDiscordAdapter(cl)
	opusAudioLoader := newOpusAudioLoader(map[audio]string{
		PomodoroAudio:   pomodoroSoundPath,
		LongBreakAudio:  longBreakSoundPath,
		ShortBreakAudio: shortBreakSoundPath,
		IdleAudio:       idleSoundPath,
	})
	autoshusher := NewAutoshusher(
		participantsProvider,
		discordAdapter,
		*log.Default(),
	)

	// session manager
	sessionManager := NewSessionManager(topCtx, sessionRepo, participantsProvider, tx)
	sessionManager.OnSessionRestore(func(ctx context.Context, s models.Session) {
		autoshusher.Shush(ctx, s)
	})
	sessionManager.OnSessionUpdate(func(ctx context.Context, before, curr models.Session) {
		// update timer bar
		_, err := dm.EditChannelMessage(curr.Record.TextCID, curr.Record.MessageID, SessionMessageComponents(curr)...)
		if err != nil {
			log.Error("failed to edit discord channel message",
				"channelID", curr.Record.VoiceCID, "messageID", curr.Record.MessageID, "sessionID", curr.ID, "err", err)
		}

		if before.Record.CurrentInterval != curr.Record.CurrentInterval {
			// is new interval
			skipped := curr.Stats.Skips > before.Stats.Skips
			if curr.Record.CurrentInterval != pomomo.PomodoroInterval {
				// unshush before playing
				autoshusher.Shush(ctx, curr)
				if !skipped {
					if err := playIntervalAlert(ctx, curr, opusAudioLoader.Load, discordAdapter.SendOpusAudio); err != nil {
						log.Error("failed to play interval alert", "guildID", curr.Record.GuildID, "channelID", curr.Record.VoiceCID, "err", err)
					}
				}
			} else {
				// shush
				if !skipped {
					if err := playIntervalAlert(ctx, curr, opusAudioLoader.Load, discordAdapter.SendOpusAudio); err != nil {
						log.Error("failed to play interval alert", "guildID", curr.Record.GuildID, "channelID", curr.Record.VoiceCID, "err", err)
					}
					autoshusher.Shush(ctx, curr)
				}
			}
			// TODO persist participant stats
		}
	})
	sessionManager.OnSessionCleanup(func(ctx context.Context, s models.Session) {
		go func() {
			_, err := dm.EditChannelMessage(s.Record.TextCID, s.Record.MessageID, SessionMessageComponents(s)...)
			if err != nil {
				log.Error("failed to edit discord channel message", "channelID", s.Record.VoiceCID, "messageID", s.Record.MessageID, "sessionID", s.ID, "err", err)
			}
			if err := cl.ChannelMessageUnpin(string(s.Record.TextCID), s.Record.MessageID); err != nil {
				log.Error("failed to unpin discord channel message", "channelID", s.Record.VoiceCID, "messageID", s.Record.MessageID, "sessionID", s.ID, "err", err)
			}
		}()
		go func() {
			autoshusher.Shush(ctx, s) // unshush
			if sessionManager.GuildSessionCnt(s.Record.GuildID) == 0 {
				if conn := cl.VoiceConnections[string(s.Record.GuildID)]; conn != nil {
					if err := conn.Disconnect(); err != nil {
						log.Error(err)
					}
				}
			}
			// delete participants
			ps := participantsProvider.GetAll(s.Record.VoiceCID)
			for _, p := range ps {
				_ = participantsProvider.Delete(ctx, p.ID)
			}
		}()
	})
	panicif(sessionManager.RestoreSessions(initTimeout))

	// discord event hooks
	cl.AddHandler(func(s *dg.Session, u *dg.VoiceStateUpdate) {
		go RemoveParticipantOnVoiceChannelLeave(topCtx, discordAdapter, participantsProvider, s, u)
	})
	cl.AddHandler(func(s *dg.Session, m *dg.InteractionCreate) {
		var wg sync.WaitGroup
		wg.Go(func() {
			StartSession(topCtx, sessionManager, dm, participantsProvider, s, m)
		})
		wg.Go(func() {
			SkipInterval(topCtx, sessionManager, dm, s, m)
		})
		wg.Go(func() {
			EndSession(topCtx, sessionManager, s, m) // messaging handled in OnSessionCleanup hook
		})
		wg.Wait()
	})

	// open connection
	if err := cl.Open(); err != nil {
		log.Fatal("Error opening connection", "err", err)
	}
	log.Info(botName + " running. Press CTRL-C to exit.")

	// init done
	initTimeoutC()

	// graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Info("terminating " + botName)
	topCtxC()
	shutdownTimeout, shutdownTimeoutC := context.WithTimeout(context.Background(), time.Minute)
	go func() {
		// to ensure proper shutdown ordering...
		if err := sessionManager.Shutdown(); err != nil {
			log.Error(err)
		}
		autoshusher.Close()
		if err := cl.Close(); err != nil {
			log.Error(err)
		}
		shutdownTimeoutC()
	}()
	<-shutdownTimeout.Done()
	if shutdownTimeout.Err() != context.Canceled {
		log.Error("failed to shut down gracefully", "err", shutdownTimeout.Err())
	}
}

func playIntervalAlert(
	ctx context.Context, s models.Session,
	loadOpusAudio func(audio) [][]byte,
	sendOpusAudio func(context.Context, [][]byte, string, pomomo.VoiceChannelID) error,
) error {
	switch s.Record.Status {
	case pomomo.SessionRunning:
		var a audio
		switch s.Record.CurrentInterval {
		case pomomo.PomodoroInterval:
			a = PomodoroAudio
		case pomomo.LongBreakInterval:
			a = LongBreakAudio
		case pomomo.ShortBreakInterval:
			a = ShortBreakAudio
		}
		data := loadOpusAudio(a)
		if data == nil {
			return fmt.Errorf("no data for audio %d", a)
		}
		return sendOpusAudio(ctx, data, s.Record.GuildID, s.Record.VoiceCID)

		// TODO case pomomo.SessionIdle:
		// 	audioPlayer.Play(IdleAudio, s.Record.GuildID, s.Record.VoiceCID)
	}
	return nil
}

func panicif(err error) {
	if err != nil {
		panic(err)
	}
}
