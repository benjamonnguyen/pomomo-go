package main

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	txStdLib "github.com/Thiht/transactor/stdlib"
	"github.com/benjamonnguyen/deadsimple/cfg"
	dsdb "github.com/benjamonnguyen/deadsimple/db/sqlite"
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

//go:embed sounds/*.dca
var sounds embed.FS

const (
	RepoURL = "https://github.com/benjamonnguyen/pomomo-go"
	Version = "0.0.0"
)

func main() {
	// config
	conf, err := pomomo.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	var dbURL, botToken, botName string
	var shardID, shardCnt string
	var logLvl, logFile string
	panicif(conf.GetMany([]cfg.Key{
		pomomo.DatabaseURLKey,
		pomomo.BotTokenKey,
		pomomo.BotNameKey,
		pomomo.ShardIDKey,
		pomomo.ShardCountKey,
		pomomo.LogLevelKey,
		pomomo.LogFileKey,
	}, &dbURL, &botToken, &botName,
		&shardID, &shardCnt, &logLvl, &logFile))

	// logger
	log.SetReportCaller(true)
	lvl, err := log.ParseLevel(logLvl)
	if err != nil {
		log.Info("failed to parse log level - falling back to INFO", "err", err, "logLvl", logLvl)
	}
	log.SetLevel(lvl)
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		panicif(err)
		log.SetOutput(f)
	}

	//
	topCtx, topCtxC := context.WithCancel(context.Background())
	initTimeout, initTimeoutC := context.WithTimeout(topCtx, 10*time.Second)

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
	// repos
	sessionRepo := sqlite.NewSessionRepo(dbGetter, *log.Default())
	participantRepo := sqlite.NewParticipantRepo(dbGetter, *log.Default())

	// set up discord cl
	cl, err := dg.New("Bot " + botToken)
	if err != nil {
		log.Fatal(err)
	}
	cl.ShouldRetryOnRateLimit = false
	if shardCnt != "" {
		n, err := strconv.Atoi(shardCnt)
		panicif(err)
		cl.ShardCount = n
	}
	if shardID != "" {
		n, err := strconv.Atoi(shardID)
		panicif(err)
		cl.ShardID = n
	}
	cl.Client = &http.Client{Timeout: (20 * time.Second)}
	cl.UserAgent = fmt.Sprintf("%s (%s, v%s)", botName, RepoURL, Version)
	cl.ShouldReconnectVoiceOnSessionError = true

	dm := NewDiscordMessenger(cl)
	discordAdapter := discordgo.NewDiscordAdapter(cl)

	// participant manager
	pm := NewParticipantManager(participantRepo, *log.Default())

	// audio
	opusAudioLoader := newOpusAudioLoader(sounds)
	autoshusher := &autoshusher{
		loadFn: opusAudioLoader.Load,
		sendFn: discordAdapter.SendOpusAudio,
		pm:     pm,
		vs:     discordAdapter,
	}

	// session manager
	sessionManager := NewSessionManager(topCtx, sessionRepo, pm, tx)
	sessionManager.AfterUpdate(func(ctx context.Context, before, curr models.Session) {
		if curr.Record.Status == pomomo.SessionEnded {
			var wg sync.WaitGroup
			wg.Go(func() {
				// handle channel message cleanup
				_, err := dm.EditChannelMessage(curr.Record.TextCID, curr.Record.MessageID, SessionMessageComponents(curr)...)
				if err != nil {
					log.Error("failed to edit discord channel message", "channelID", curr.Record.VoiceCID, "messageID", curr.Record.MessageID, "sessionID", curr.ID, "err", err)
				}
				if err := cl.ChannelMessageUnpin(string(curr.Record.TextCID), curr.Record.MessageID); err != nil {
					log.Error("failed to unpin discord channel message", "channelID", curr.Record.VoiceCID, "messageID", curr.Record.MessageID, "sessionID", curr.ID, "err", err)
				}
			})
			wg.Go(func() {
				// handle participant cleanup
				unlock := pm.AcquireVoiceChannelLock(curr.Record.VoiceCID)
				defer unlock()
				participants := pm.GetAll(curr.Record.VoiceCID)

				var wgg sync.WaitGroup
				for _, p := range participants {
					if err := restoreVoiceState(ctx, discordAdapter, p); err != nil {
						log.Error(err)
					}
					if err := pm.Delete(ctx, p.ID); err != nil {
						log.Error(err)
					}
				}
				wgg.Wait()

				cnt := sessionManager.GuildSessionCnt(curr.Record.GuildID)
				if cnt == 0 {
					if conn := cl.VoiceConnections[string(curr.Record.GuildID)]; conn != nil {
						if err := conn.Disconnect(); err != nil {
							log.Error(err)
						}
					}
				}
			})
			wg.Wait()
			return
		}

		unlock := pm.AcquireVoiceChannelLock(curr.Record.VoiceCID)
		defer unlock()
		participants := pm.GetAll(curr.Record.VoiceCID)

		// end empty session
		if len(participants) == 0 {
			// start go routine so that we don't get deadlocked from a recursive trigger
			go func() {
				_, err := sessionManager.EndSession(ctx, curr.Record.TextCID)
				if err != nil {
					log.Error("failed to end empty session", "sid", curr.ID, "err", err)
					return
				}
				log.Debug("ended empty session", "sid", curr.ID)
			}()
			return
		}

		var wg sync.WaitGroup
		wg.Go(func() {
			// update timer bar
			_, err := dm.EditChannelMessage(curr.Record.TextCID, curr.Record.MessageID, SessionMessageComponents(curr)...)
			if err != nil {
				log.Error("failed to edit discord channel message",
					"channelID", curr.Record.VoiceCID, "messageID", curr.Record.MessageID, "sessionID", curr.ID, "err", err)
			}
		})

		wg.Go(func() {
			autoshusher.Autoshush(ctx, participants, before, curr)
			// TODO persist participant stats
		})

		wg.Wait()
	})

	// discord event hooks
	cl.AddHandler(func(s *dg.Session, u *dg.VoiceStateUpdate) {
		_ = RestoreParticipantVoiceStateOnChannelJoin(topCtx, discordAdapter, pm, s, u) ||
			RemoveParticipantOnVoiceChannelLeave(topCtx, discordAdapter, pm, s, u)
	})
	cl.AddHandler(func(s *dg.Session, m *dg.InteractionCreate) {
		_ = StartSession(topCtx, sessionManager, dm, pm, s, m) ||
			SkipInterval(topCtx, sessionManager, dm, s, m) ||
			EndSession(topCtx, sessionManager, s, m) ||
			JoinSession(topCtx, sessionManager, autoshusher, pm, dm, s, m)
	})

	// start up
	if err := cl.Open(); err != nil {
		log.Fatal("Error opening connection", "err", err)
	}
	panicif(pm.RestoreCache(initTimeout))
	panicif(sessionManager.RestoreSessions(initTimeout))
	initTimeoutC()
	log.Info(botName + " running. Press CTRL-C to exit.")

	// graceful shutdown
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Info("terminating " + botName)
	topCtxC()
	shutdownTimeout, shutdownTimeoutC := context.WithTimeout(context.Background(), 10*time.Second)
	go func() {
		// to ensure proper shutdown ordering...
		if err := sessionManager.Shutdown(); err != nil {
			log.Error(err)
		}
		var wg sync.WaitGroup
		for _, cid := range pm.GetVoiceChannelIDs() {
			participants := pm.GetAll(cid)
			for _, p := range participants {
				wg.Go(func() {
					if err := restoreVoiceState(shutdownTimeout, discordAdapter, p); err != nil {
						log.Error(err)
					}
				})
			}
		}
		wg.Wait()
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

type (
	loadOpusAudio func(audio) [][]byte
	sendOpusAudio func(context.Context, [][]byte, string, pomomo.VoiceChannelID) error
)

func playIntervalAlert(
	ctx context.Context, s models.Session,
	loadFn loadOpusAudio, sendFn sendOpusAudio,
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
		data := loadFn(a)
		if data == nil {
			return fmt.Errorf("no data for audio %s", a)
		}
		return sendFn(ctx, data, s.Record.GuildID, s.Record.VoiceCID)

		// TODO case pomomo.SessionIdle:
		// 	audioPlayer.Play(IdleAudio, s.Record.GuildID, s.Record.VoiceCID)
	}
	return nil
}

type VoiceStateAdapter interface {
	UpdateVoiceState(gid, uid string, mute, deaf bool) error
	GetVoiceState(gid, uid string) (pomomo.VoiceState, error)
}

func updateVoiceState(ctx context.Context, vs VoiceStateAdapter, mute, deafen bool, p models.Participant) error {
	return vs.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, mute || p.Record.IsMuted, deafen || p.Record.IsDeafened)
}

func restoreVoiceState(ctx context.Context, vs VoiceStateAdapter, p models.Participant) error {
	return vs.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, p.Record.IsMuted, p.Record.IsDeafened)
}

func getVoiceState(ctx context.Context, vs VoiceStateAdapter, p models.Participant) (pomomo.VoiceState, error) {
	return vs.GetVoiceState(p.Record.GuildID, p.Record.UserID)
}

func panicif(err error) {
	if err != nil {
		panic(err)
	}
}
