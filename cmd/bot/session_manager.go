package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Thiht/transactor"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/charmbracelet/log"
)

var updateTickRate = 20 * time.Second

type startSessionRequest struct {
	guildID, channelID, messageID string
	settings                      SessionSettings
}

type SessionManager interface {
	HasSession(guildID, channelID string) bool
	StartSession(context.Context, startSessionRequest) (Session, error)
	EndSession(context.Context, sessionKey) (Session, error)
	SkipInterval(context.Context, sessionKey) (Session, error)
	// TogglePause(context.Context, sessionKey) (Session, error)

	OnSessionUpdate(func(context.Context, Session))
	Shutdown() error
}

type sessionKey struct {
	guildID, channelID string
}

func (k sessionKey) String() string {
	return fmt.Sprintf("%s:%s", k.guildID, k.channelID)
}

func (k sessionKey) validate() error {
	if k.guildID == "" || k.channelID == "" {
		return fmt.Errorf("cacheKey requires guild and channel IDs")
	}
	return nil
}

type sessionManager struct {
	repo      pomomo.SessionRepo
	tx        transactor.Transactor
	cache     *sessionCache
	wg        sync.WaitGroup
	parentCtx context.Context

	onSessionUpdate func(context.Context, Session)
}

func NewSessionManager(ctx context.Context, repo pomomo.SessionRepo, tx transactor.Transactor) SessionManager {
	cache := sessionCache{
		sessions:    make(map[sessionKey]*Session),
		locks:       make(map[sessionKey]*sync.Mutex),
		cancelFuncs: make(map[sessionKey]func()),
	}

	mgr := &sessionManager{
		cache:     &cache,
		repo:      repo,
		tx:        tx,
		parentCtx: ctx,
	}

	mgr.restorePendingSessions()
	return mgr
}

// TODO end stale sessions (last update > 5 minutes)

func (m *sessionManager) restorePendingSessions() {
	var toRestore []*Session
	err := m.tx.WithinTransaction(m.parentCtx, func(ctx context.Context) error {
		pendingSessions, err := m.repo.GetByStatus(ctx, pomomo.SessionRunning, pomomo.SessionPaused)
		if err != nil {
			return err
		}

		for _, pendingSession := range pendingSessions {
			existingSettings, err := m.repo.GetSettings(ctx, pendingSession.ID)
			if err != nil {
				return err
			}
			session := NewSession(pendingSession, existingSettings)
			toRestore = append(toRestore, &session)
		}
		return nil
	})
	panicif(err)

	sessionCtxs := m.cache.Add(m.parentCtx, toRestore...)
	for i, sessionCtx := range sessionCtxs {
		m.startUpdateLoop(sessionCtx, toRestore[i].key())
	}
	log.Info("restored pending sessions", "count", len(toRestore))
}

func (m *sessionManager) HasSession(guildID, channelID string) bool {
	return m.cache.Has(sessionKey{
		guildID:   guildID,
		channelID: channelID,
	})
}

func (m *sessionManager) OnSessionUpdate(handler func(context.Context, Session)) {
	m.onSessionUpdate = handler
}

func (m *sessionManager) updateSession(ctx context.Context, s *Session) error {
	if s.RemainingTime() <= 0 {
		s.goNextInterval(true)
	}
	return m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := m.repo.UpdateSession(ctx, s.sessionID, s.toRecord())
		return err
	})
}

func (m *sessionManager) startUpdateLoop(ctx context.Context, key sessionKey) {
	m.wg.Go(func() {
		var updateMu sync.Mutex
		ticker := time.NewTicker(updateTickRate)
		for {
			func() {
				s, unlock := m.cache.Get(key)
				defer unlock()
				if s == nil {
					log.Info("ending update loop - session not found", "key", key.String())
					return
				}
				if err := m.updateSession(ctx, s); err != nil {
					log.Error("failed to update session interval in timer", "sessionID", s.sessionID, "err", err)
				} else if m.onSessionUpdate != nil {
					// can't rely on onSessionUpdate to handle ctx timeout - if still locked, skip call
					go func() {
						if updateMu.TryLock() {
							m.onSessionUpdate(ctx, *s)
						}
						defer updateMu.Unlock()
					}()
				}
			}()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				continue
			}
		}
	})
}

func (m *sessionManager) StartSession(ctx context.Context, req startSessionRequest) (Session, error) {
	session := &Session{
		channelID:         req.channelID,
		guildID:           req.guildID,
		messageID:         req.messageID,
		settings:          req.settings,
		currentInterval:   pomomo.PomodoroInterval,
		intervalStartedAt: time.Now(),
		status:            pomomo.SessionRunning,
	}
	key := session.key()

	if m.cache.Has(key) {
		return Session{}, fmt.Errorf("session already exists for guild %s channel %s", req.guildID, req.channelID)
	}

	// Execute transaction
	release := m.cache.Hold(key)
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		// Insert session record
		inserted, err := m.repo.InsertSession(ctx, session.toRecord())
		if err != nil {
			return fmt.Errorf("failed to insert session: %w", err)
		}

		// Insert session settings record
		settingsRecord := pomomo.SessionSettingsRecord{
			SessionID:  inserted.ID,
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
		session.sessionID = inserted.ID
		return nil
	})
	release()
	if err != nil {
		return Session{}, fmt.Errorf("failed to start session: %w", err)
	}

	sessionCtxs := m.cache.Add(m.parentCtx, session)
	m.startUpdateLoop(sessionCtxs[0], key)
	return *session, nil
}

func (m *sessionManager) SkipInterval(ctx context.Context, key sessionKey) (Session, error) {
	s, unlock := m.cache.Get(key)
	if s == nil {
		return Session{}, fmt.Errorf("session not found for key: %v", key)
	}
	defer unlock()

	s.goNextInterval(false)

	// Update database with new interval state
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := m.repo.UpdateSession(ctx, s.sessionID, s.toRecord())
		return err
	})
	if err != nil {
		return Session{}, fmt.Errorf("failed to skip interval: %w", err)
	}

	return *s, nil
}

func (m *sessionManager) EndSession(ctx context.Context, key sessionKey) (Session, error) {
	s, unlock := m.cache.Get(key)
	if s == nil {
		return Session{}, fmt.Errorf("session not found for key: %v", key)
	}

	copy := *s
	copy.status = pomomo.SessionEnded
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		record := copy.toRecord()
		_, err := m.repo.UpdateSession(ctx, copy.sessionID, record)
		if err != nil {
			return fmt.Errorf("failed to update session status: %w", err)
		}

		_, err = m.repo.DeleteSettings(ctx, copy.sessionID)
		if err != nil {
			return fmt.Errorf("failed to delete settings: %w", err)
		}
		return nil
	})
	unlock()
	if err != nil {
		return Session{}, fmt.Errorf("failed to end session: %w", err)
	}

	m.cache.Remove(key)
	return copy, nil
}

func (m *sessionManager) Shutdown() error {
	m.cache.cacheMu.Lock()
	for _, c := range m.cache.cancelFuncs {
		c()
	}

	// Wait for all timer goroutines to exit
	m.wg.Wait()
	return nil
}

// Cache

type sessionCache struct {
	cacheMu     sync.RWMutex
	sessions    map[sessionKey]*Session
	locks       map[sessionKey]*sync.Mutex
	cancelFuncs map[sessionKey]func()
}

// Add returns cancellable session contexts
func (c *sessionCache) Add(ctx context.Context, sessions ...*Session) []context.Context {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	sessionCtxs := make([]context.Context, 0, len(sessions))
	for _, s := range sessions {
		key := s.key()
		_, exists := c.locks[key] // checks locks instead of sessions in case of Hold()
		if exists {
			panic("session already exists for key: " + key.String())
		}
		c.locks[key] = &sync.Mutex{}
		c.sessions[key] = s
		sessionCtx, cancel := context.WithCancel(ctx)
		c.cancelFuncs[key] = cancel
		sessionCtxs = append(sessionCtxs, sessionCtx)
	}

	return sessionCtxs
}

func (c *sessionCache) Remove(key sessionKey) {
	if !c.Has(key) {
		log.Debug("session not found", "key", key.String())
		return
	}

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.locks[key].Lock()
	c.cancelFuncs[key]()
	delete(c.cancelFuncs, key)
	delete(c.sessions, key)
	delete(c.locks, key)
}

func (c *sessionCache) Get(key sessionKey) (*Session, func()) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	_, exists := c.locks[key] // checks locks instead of sessions in case of Hold()
	if !exists {
		return nil, nil
	}

	l := c.locks[key]
	l.Lock()
	return c.sessions[key], l.Unlock
}

func (c *sessionCache) Has(key sessionKey) bool {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	_, exists := c.locks[key] // checks locks instead of sessions in case of Hold()
	return exists
}

func (c *sessionCache) Hold(key sessionKey) func() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if l := c.locks[key]; l != nil {
		l.Lock()
		return l.Unlock
	}

	mu := &sync.Mutex{}
	mu.Lock()
	c.locks[key] = mu
	return func() {
		c.cacheMu.Lock()
		defer c.cacheMu.Unlock()
		delete(c.locks, key)
		mu.Unlock()
	}
}
