package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Thiht/transactor"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/charmbracelet/log"
)

var updateTickRate = 20 * time.Second

type startSessionRequest struct {
	guildID, channelID, messageID string
	settings                      models.SessionSettings
}

type SessionManager interface {
	HasSession(guildID, channelID string) bool
	StartSession(context.Context, startSessionRequest) (models.Session, error)
	EndSession(context.Context, sessionKey) (models.Session, error)
	SkipInterval(context.Context, sessionKey) (models.Session, error)
	// TogglePause(context.Context, sessionKey) (Session, error)

	RestoreSessions() error

	OnSessionUpdate(func(context.Context, models.Session))
	OnSessionNextInterval(func(context.Context, models.Session))
	OnSessionCleanup(func(context.Context, models.Session))
	Shutdown() error
}

type sessionKey struct {
	guildID, channelID string
}

func (k sessionKey) String() string {
	return fmt.Sprintf("%s:%s", k.guildID, k.channelID)
}

func key(s models.Session) sessionKey {
	return sessionKey{
		guildID:   s.GuildID(),
		channelID: s.ChannelID(),
	}
}

type sessionManager struct {
	repo      pomomo.SessionRepo
	tx        transactor.Transactor
	cache     *sessionCache
	wg        sync.WaitGroup
	parentCtx context.Context

	onUpdate       func(context.Context, models.Session)
	onNextInterval func(context.Context, models.Session)
	onCleanup      func(context.Context, models.Session)
}

func NewSessionManager(ctx context.Context, repo pomomo.SessionRepo, tx transactor.Transactor) SessionManager {
	cache := sessionCache{
		sessions:    make(map[sessionKey]*models.Session),
		locks:       make(map[sessionKey]*sync.Mutex),
		cancelFuncs: make(map[sessionKey]func()),
	}

	return &sessionManager{
		cache:     &cache,
		repo:      repo,
		tx:        tx,
		parentCtx: ctx,
	}
}

func (m *sessionManager) OnSessionCleanup(f func(context.Context, models.Session)) {
	m.onCleanup = f
}

func (m *sessionManager) OnSessionNextInterval(f func(context.Context, models.Session)) {
	m.onNextInterval = f
}

func (m *sessionManager) RestoreSessions() error {
	var toRestore []*models.Session
	err := m.tx.WithinTransaction(m.parentCtx, func(ctx context.Context) error {
		pendingSessionRecords, err := m.repo.GetSessionsByStatus(ctx, pomomo.SessionRunning, pomomo.SessionPaused)
		if err != nil {
			return err
		}

		for _, r := range pendingSessionRecords {
			existingSettings, err := m.repo.GetSettings(ctx, r.ID)
			if err != nil {
				return err
			}
			session := models.SessionFromExistingRecords(r, existingSettings)
			if session.TimeRemaining() < (-1 * time.Hour) {
				// clean up stale session
				go func() {
					if m.onCleanup != nil {
						m.onCleanup(m.parentCtx, session)
					}
				}()
				continue
			}
			for session.TimeRemaining() <= 0 {
				session.GoNextInterval(true)
			}
			toRestore = append(toRestore, &session)
		}

		return nil
	})
	if err != nil {
		return err
	}

	sessionCtxs := m.cache.Add(m.parentCtx, toRestore...)
	for i, sessionCtx := range sessionCtxs {
		session := *toRestore[i]
		m.startUpdateLoop(sessionCtx, key(session))
	}
	log.Info("restored pending sessions", "count", len(toRestore))
	return nil
}

func (m *sessionManager) HasSession(guildID, channelID string) bool {
	return m.cache.Has(sessionKey{
		guildID:   guildID,
		channelID: channelID,
	})
}

func (m *sessionManager) OnSessionUpdate(handler func(context.Context, models.Session)) {
	m.onUpdate = handler
}

func (m *sessionManager) updateSession(ctx context.Context, s *models.Session) error {
	if s.TimeRemaining() > 0 {
		return nil
	}
	s.GoNextInterval(true)
	return m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := m.repo.UpdateSession(ctx, s.ID, s.Record())
		return err
	})
}

func (m *sessionManager) startUpdateLoop(ctx context.Context, key sessionKey) {
	m.wg.Go(func() {
		var updateMu, nextIntervalMu sync.Mutex
		ticker := time.NewTicker(updateTickRate)
		for {
			func() {
				s, unlock := m.cache.Get(key)
				defer unlock()
				if s == nil {
					log.Info("ending update loop - session not found", "key", key.String())
					return
				}

				prevStatus := s.CurrentInterval()
				if err := m.updateSession(ctx, s); err != nil {
					log.Error("failed to update session interval in timer", "sessionID", s.ID, "err", err)
					return
				}

				if m.onUpdate != nil {
					// can't rely on onSessionUpdate to handle ctx timeout - if still locked, skip call
					go func() {
						if updateMu.TryLock() {
							defer updateMu.Unlock()
							m.onUpdate(ctx, *s)
						}
					}()
				}

				if prevStatus != s.CurrentInterval() && m.onNextInterval != nil {
					go func() {
						if nextIntervalMu.TryLock() {
							defer nextIntervalMu.Unlock()
							m.onNextInterval(ctx, *s)
						}
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

func (m *sessionManager) StartSession(ctx context.Context, req startSessionRequest) (models.Session, error) {
	session := models.NewSession("", req.guildID, req.channelID, req.messageID, req.settings)
	key := key(session)

	if m.cache.Has(key) {
		return models.Session{}, fmt.Errorf("session already exists for guild %s channel %s", req.guildID, req.channelID)
	}

	// Execute transaction
	release := m.cache.Hold(key)
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		// Insert session record
		inserted, err := m.repo.InsertSession(ctx, session.Record())
		if err != nil {
			return fmt.Errorf("failed to insert session: %w", err)
		}

		// Insert session settings record
		settingsRecord := pomomo.SessionSettingsRecord{
			SessionID:  inserted.ID,
			Pomodoro:   req.settings.Pomodoro,
			ShortBreak: req.settings.ShortBreak,
			LongBreak:  req.settings.LongBreak,
			Intervals:  req.settings.Intervals,
		}
		_, err = m.repo.InsertSettings(ctx, settingsRecord)
		if err != nil {
			return fmt.Errorf("failed to insert session settings: %w", err)
		}

		// Update session with ID
		session.ID = inserted.ID
		return nil
	})
	release()
	if err != nil {
		return models.Session{}, fmt.Errorf("failed to start session: %w", err)
	}

	sessionCtxs := m.cache.Add(m.parentCtx, &session)
	m.startUpdateLoop(sessionCtxs[0], key)
	return session, nil
}

func (m *sessionManager) SkipInterval(ctx context.Context, key sessionKey) (models.Session, error) {
	s, unlock := m.cache.Get(key)
	if s == nil {
		return models.Session{}, fmt.Errorf("session not found for key: %v", key)
	}
	defer unlock()

	s.GoNextInterval(false)

	// Update database with new interval state
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := m.repo.UpdateSession(ctx, s.ID, s.Record())
		return err
	})
	if err != nil {
		return models.Session{}, fmt.Errorf("failed to skip interval: %w", err)
	}

	return *s, nil
}

func (m *sessionManager) EndSession(ctx context.Context, key sessionKey) (models.Session, error) {
	s, unlock := m.cache.Get(key)
	if s == nil {
		return models.Session{}, fmt.Errorf("session not found for key: %v", key)
	}

	sessionCopy := *s
	// Create a new record with ended status for database update
	endedRecord := sessionCopy.Record()
	endedRecord.Status = pomomo.SessionEnded
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := m.repo.UpdateSession(ctx, sessionCopy.ID, endedRecord)
		if err != nil {
			return fmt.Errorf("failed to update session status: %w", err)
		}

		_, err = m.repo.DeleteSettings(ctx, sessionCopy.ID)
		if err != nil {
			return fmt.Errorf("failed to delete settings: %w", err)
		}
		return nil
	})
	unlock()
	if err != nil {
		return models.Session{}, fmt.Errorf("failed to end session: %w", err)
	}

	m.cache.Remove(key)
	return sessionCopy, nil
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
	sessions    map[sessionKey]*models.Session
	locks       map[sessionKey]*sync.Mutex
	cancelFuncs map[sessionKey]func()
}

// Add returns cancellable session contexts
func (c *sessionCache) Add(ctx context.Context, sessions ...*models.Session) []context.Context {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	sessionCtxs := make([]context.Context, 0, len(sessions))
	for _, s := range sessions {
		key := key(*s)
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

func (c *sessionCache) Get(key sessionKey) (*models.Session, func()) {
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
