package main

import (
	"context"
	"embed"
	"errors"
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
	"github.com/benjamonnguyen/pomomo-go/disgo"
	"github.com/benjamonnguyen/pomomo-go/sqlite"
	"github.com/charmbracelet/log"
	disgoorg "github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/cache"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/disgo/voice"
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
	gatewayOpts := []gateway.ConfigOpt{
		gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildVoiceStates),
	}
	if shardCnt != "" {
		n, err := strconv.Atoi(shardCnt)
		panicif(err)
		gatewayOpts = append(gatewayOpts, gateway.WithShardCount(n))
	}
	if shardID != "" {
		n, err := strconv.Atoi(shardID)
		panicif(err)
		gatewayOpts = append(gatewayOpts, gateway.WithShardID(n))
	}
	cl, err := disgoorg.New(botToken,
		bot.WithLogger(disgo.NewLogger(log.Default())), // after log output/level are set
		bot.WithGatewayConfigOpts(gatewayOpts...),
		bot.WithCacheConfigOpts(cache.WithCaches(cache.FlagGuilds, cache.FlagVoiceStates)),
		bot.WithRestClientConfigOpts(
			rest.WithHTTPClient(&http.Client{Timeout: (20 * time.Second)}),
			rest.WithUserAgent(fmt.Sprintf("%s (%s, v%s)", botName, RepoURL, Version)),
		),
		// handlers block on REST/DB calls - don't stall the gateway read loop
		bot.WithEventManagerConfigOpts(bot.WithAsyncEventsEnabled()),
		// DAVE (E2EE) implementation; swap for golibdave.NewSession if dave-go misbehaves
		bot.WithVoiceManagerConfigOpts(voice.WithDaveSessionCreateFunc(disgo.DaveSessionCreateFunc())),
	)
	if err != nil {
		log.Fatal(err)
	}

	dm := disgo.NewMessenger(cl.Rest)
	discordAdapter := disgo.NewAdapter(cl.Rest, cl.Caches, cl.VoiceManager)

	// participant manager
	pm := NewParticipantManager(participantRepo, tx, *log.Default())

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
	// start go routine so that we don't get deadlocked from a recursive trigger
	// use topCtx since ending the session cancels the session ctx, which would fail participant cleanup
	endSessionAsync := func(s models.Session, reason string) {
		go func() {
			_, err := sessionManager.EndSession(topCtx, s.Record.TextCID)
			if err != nil {
				log.Error("failed to end "+reason+" session", "sid", s.ID, "err", err)
				return
			}
			log.Info("ended "+reason+" session", "sid", s.ID)
		}()
	}
	timerMsgFailures := newTimerMsgFailures()
	sessionManager.AfterUpdate(func(ctx context.Context, before, curr models.Session) {
		if curr.Record.Status == pomomo.SessionEnded {
			timerMsgFailures.reset(curr.ID)
			var wg sync.WaitGroup
			wg.Go(func() {
				// handle channel message cleanup
				_, err := dm.EditChannelMessage(curr.Record.TextCID, curr.Record.MessageID, SessionMessageComponents(curr)...)
				if err != nil {
					log.Error("failed to edit discord channel message", "textCID", curr.Record.TextCID, "messageID", curr.Record.MessageID, "sessionID", curr.ID, "err", err)
				}
				if err := unpinMessage(cl.Rest, curr.Record.TextCID, curr.Record.MessageID); err != nil {
					log.Error("failed to unpin discord channel message", "textCID", curr.Record.TextCID, "messageID", curr.Record.MessageID, "sessionID", curr.ID, "err", err)
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
					closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if err := discordAdapter.CloseVoice(closeCtx, curr.Record.GuildID); err != nil {
						log.Error(err)
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
			endSessionAsync(curr, "empty")
			return
		}

		var wg sync.WaitGroup
		wg.Go(func() {
			// update timer bar
			_, err := dm.EditChannelMessage(curr.Record.TextCID, curr.Record.MessageID, SessionMessageComponents(curr)...)
			if err != nil {
				log.Error("failed to edit discord channel message",
					"textCID", curr.Record.TextCID, "messageID", curr.Record.MessageID, "sessionID", curr.ID, "err", err)
				// bot removed from guild or channel - leave events stop arriving, so the session would never empty
				if timerMsgFailures.unreachable(curr.ID, err) {
					endSessionAsync(curr, "unreachable")
				}
				return
			}
			timerMsgFailures.reset(curr.ID)
		})

		wg.Go(func() {
			autoshusher.Autoshush(ctx, participants, before, curr)
			// TODO persist participant stats
		})

		wg.Wait()
	})

	// discord event hooks
	cl.AddEventListeners(
		bot.NewListenerFunc(func(e *events.GuildJoin) {
			log.Info("added to guild", "gid", e.GuildID, "members", e.Guild.MemberCount)
		}),
		bot.NewListenerFunc(func(e *events.GuildLeave) {
			log.Info("removed from guild", "gid", e.GuildID)
			// removed from guild - no more voice state or interaction events for its sessions
			if err := sessionManager.EndGuildSessions(topCtx, e.GuildID.String()); err != nil {
				log.Error("failed to end sessions on guild leave", "gid", e.GuildID, "err", err)
			}
		}),
		bot.NewListenerFunc(func(u *events.GuildVoiceStateUpdate) {
			_ = RestoreParticipantVoiceStateOnChannelJoin(topCtx, discordAdapter, pm, u) ||
				RemoveParticipantOnVoiceChannelLeave(topCtx, discordAdapter, pm, u)
		}),
		bot.NewListenerFunc(func(e *events.ApplicationCommandInteractionCreate) {
			_ = StartSession(topCtx, sessionManager, dm, discordAdapter, cl.Rest, e)
		}),
		bot.NewListenerFunc(func(e *events.ComponentInteractionCreate) {
			_ = SkipInterval(topCtx, sessionManager, dm, e) ||
				EndSession(topCtx, sessionManager, dm, e) ||
				JoinSession(topCtx, sessionManager, autoshusher, pm, dm, discordAdapter, e)
		}),
	)

	// start up
	if err := cl.OpenGateway(initTimeout); err != nil {
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
		// closes voice conns, then gateway and rest
		cl.Close(shutdownTimeout)
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
	MoveMember(gid, uid string, cid pomomo.VoiceChannelID) error
}

func unpinMessage(p MessagePinner, cid pomomo.TextChannelID, messageID string) error {
	ids, err := disgo.ParseIDs(string(cid), messageID)
	if err != nil {
		return err
	}
	return p.UnpinMessage(ids[0], ids[1])
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

// maxTimerMsgMissingAccess is how many consecutive ticks a session's timer message can be
// inaccessible before it's ended, in case access is restored.
const maxTimerMsgMissingAccess = 3

// timerMsgFailures tracks sessions whose timer message can't be edited.
type timerMsgFailures struct {
	mu            sync.Mutex
	missingAccess map[pomomo.SessionID]int
}

func newTimerMsgFailures() *timerMsgFailures {
	return &timerMsgFailures{missingAccess: make(map[pomomo.SessionID]int)}
}

// unreachable records a failed edit and reports whether the session should be ended.
func (f *timerMsgFailures) unreachable(sid pomomo.SessionID, err error) bool {
	switch restErrCode(err) {
	case rest.JSONErrorCodeUnknownGuild, rest.JSONErrorCodeUnknownChannel, rest.JSONErrorCodeUnknownMessage:
		return true
	case rest.JSONErrorCodeMissingAccess:
		f.mu.Lock()
		defer f.mu.Unlock()
		f.missingAccess[sid]++
		return f.missingAccess[sid] >= maxTimerMsgMissingAccess
	}
	return false
}

func (f *timerMsgFailures) reset(sid pomomo.SessionID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.missingAccess, sid)
}

// restErrCode returns the Discord JSON error code of a REST error, or 0.
func restErrCode(err error) rest.JSONErrorCode {
	var rerr *rest.Error
	if errors.As(err, &rerr) {
		return rerr.Code
	}
	return 0
}

// logRestErr logs missing permissions as a warning since they're server config, not bot errors.
func logRestErr(msg string, err error, keyvals ...any) {
	log.Helper()
	lvl := log.ErrorLevel
	if restErrCode(err) == rest.JSONErrorCodeLackPermissionsToPerformAction {
		lvl = log.WarnLevel
	}
	log.Log(lvl, msg, append([]any{"err", err}, keyvals...)...)
}

func panicif(err error) {
	if err != nil {
		panic(err)
	}
}
