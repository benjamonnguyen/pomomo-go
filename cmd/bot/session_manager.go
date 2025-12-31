package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Thiht/transactor"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/dgutils"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

var timerTickRate = 20 * time.Second

type startSessionRequest struct {
	guildID, channelID, messageID string
	settings                      SessionSettings
}

type SessionManager interface {
	HasSession(guildID, channelID string) bool
	StartSession(context.Context, startSessionRequest) (Session, error)
	// EndSession(context.Context, *Session) should delete settings
	SkipInterval(context.Context, cacheKey) (Session, error)
	Shutdown() error
}

type cacheKey struct {
	guildID, channelID string
}

func (k cacheKey) String() string {
	return fmt.Sprintf("%s:%s", k.guildID, k.channelID)
}

func (k cacheKey) validate() error {
	if k.guildID == "" || k.channelID == "" {
		return fmt.Errorf("cacheKey requires guild and channel IDs")
	}
	return nil
}

type cacheObject struct {
	session     *Session
	cancelTimer func()
}

type sessionManager struct {
	cacheMu  sync.RWMutex
	cache    map[cacheKey]*cacheObject
	keyCache map[cacheKey]*sync.Mutex
	wg       sync.WaitGroup

	repo           pomomo.SessionRepo
	tx             transactor.Transactor
	discordSession *discordgo.Session
}

func NewSessionManager(repo pomomo.SessionRepo, tx transactor.Transactor, discordSession *discordgo.Session) SessionManager {
	// TODO repo.GetByStatus(...status) to populate cache
	return &sessionManager{
		cache:          make(map[cacheKey]*cacheObject),
		keyCache:       make(map[cacheKey]*sync.Mutex),
		repo:           repo,
		tx:             tx,
		discordSession: discordSession,
	}
}

func (m *sessionManager) HasSession(guildID, channelID string) bool {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	_, exists := m.cache[cacheKey{
		guildID:   guildID,
		channelID: channelID,
	}]
	return exists
}

func (m *sessionManager) deleteSession(key cacheKey) {
	o, _ := m.getCacheObject(key)
	if o != nil {
		delete(m.cache, key)
		delete(m.keyCache, key)
		if o.cancelTimer != nil {
			o.cancelTimer()
		}
	}
}

// returns unlock func
func (m *sessionManager) getCacheObject(key cacheKey) (*cacheObject, func()) {
	m.cacheMu.RLock()
	defer m.cacheMu.RUnlock()
	mu := m.keyCache[key]
	if mu == nil {
		return nil, nil
	}
	mu.Lock()
	return m.cache[key], mu.Unlock
}

func (m *sessionManager) startIntervalTimer(key cacheKey) error {
	o, unlock := m.getCacheObject(key)
	if o == nil {
		panic("no session for key " + key.String())
	}
	ctx, cancel := context.WithCancel(context.Background())
	o.cancelTimer = cancel
	unlock()

	// Start the timer loop goroutine
	m.wg.Go(func() {
		next := time.Now()
		for {
			// Sleep until next minute, but allow cancellation
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next.Add(timerTickRate))):
				// continue loop
			}
			next = time.Now()
			o, unlock := m.getCacheObject(key)
			if o == nil {
				m.deleteSession(key)
				return
			}
			if o.session.RemainingTime() <= 0 {
				o.session.goNextInterval(true)
			}
			_, err := dgutils.EditChannelMessage(m.discordSession, o.session.channelID, o.session.messageID, o.session.MessageComponents()...)
			if err != nil {
				log.Error("failed to edit channel message", "channelID", o.session.channelID, "messageID", o.session.messageID, "err", err)
			}
			// TODO update db
			unlock()
		}
	})
	return nil
}

func (m *sessionManager) stopIntervalTimer(key cacheKey) {
	o, unlock := m.getCacheObject(key)
	if o == nil {
		return
	}
	defer unlock()
	if o.cancelTimer != nil {
		o.cancelTimer()
		o.cancelTimer = nil
	}
}

func (m *sessionManager) StartSession(parentCtx context.Context, req startSessionRequest) (Session, error) {
	session := Session{
		channelID:         req.channelID,
		guildID:           req.guildID,
		messageID:         req.messageID,
		settings:          req.settings,
		currentInterval:   PomodoroInterval,
		intervalStartedAt: time.Now(),
	}
	key := session.key()
	if err := key.validate(); err != nil {
		return Session{}, err
	}

	if m.HasSession(req.guildID, req.channelID) {
		return Session{}, fmt.Errorf("session already exists for guild %s channel %s", req.guildID, req.channelID)
	}

	m.cacheMu.Lock()
	mu := &sync.Mutex{}
	mu.Lock()
	m.keyCache[key] = mu
	m.cache[key] = &cacheObject{
		session: &session,
	}
	m.cacheMu.Unlock()

	// Execute transaction
	err := m.tx.WithinTransaction(parentCtx, func(ctx context.Context) error {
		// Insert session record
		existingSession, err := m.repo.InsertSession(ctx, session.toRecord())
		if err != nil {
			return fmt.Errorf("failed to insert session: %w", err)
		}

		// Insert session settings record
		settingsRecord := pomomo.SessionSettingsRecord{
			SessionID:  existingSession.ID,
			Pomodoro:   req.settings.pomodoro,
			ShortBreak: req.settings.shortBreak,
			LongBreak:  req.settings.longBreak,
			Intervals:  req.settings.intervals,
		}
		_, err = m.repo.InsertSettings(ctx, settingsRecord)
		if err != nil {
			return fmt.Errorf("failed to insert session settings: %w", err)
		}

		// Update session with ID
		session.sessionID = existingSession.ID
		return nil
	})
	if err != nil {
		mu.Unlock()
		m.deleteSession(key)
		return Session{}, fmt.Errorf("failed to start session: %w", err)
	}
	mu.Unlock()

	// Start per-session goroutine
	if err := m.startIntervalTimer(key); err != nil {
		m.deleteSession(key)
		return Session{}, fmt.Errorf("failed to start session timer: %w", err)
	}

	return session, nil
}

func (m *sessionManager) SkipInterval(ctx context.Context, key cacheKey) (Session, error) {
	o, unlock := m.getCacheObject(key)
	if o == nil {
		return Session{}, fmt.Errorf("session not found for key: %v", key)
	}
	defer unlock()

	o.session.goNextInterval(false)

	// TODO: Update database with new interval state
	// m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
	//     return m.repo.UpdateSession(ctx, sessionPtr.sessionID, ...)
	// })

	return *o.session, nil
}

func (m *sessionManager) Shutdown() error {
	// Collect all cache keys
	m.cacheMu.RLock()
	keys := make([]cacheKey, 0, len(m.cache))
	for key := range m.cache {
		keys = append(keys, key)
	}
	m.cacheMu.RUnlock()

	// Cancel timers for all sessions
	for _, key := range keys {
		m.stopIntervalTimer(key)
	}

	// Wait for all timer goroutines to exit
	m.wg.Wait()
	return nil
}
