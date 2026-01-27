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
	guildID, textCID, voiceCID, messageID string
	settings                              models.SessionSettings
}

type SessionManager interface {
	HasSession(textCID string) bool
	StartSession(context.Context, startSessionRequest) (models.Session, error)
	EndSession(ctx context.Context, cid pomomo.TextChannelID) (models.Session, error)
	SkipInterval(ctx context.Context, cid pomomo.TextChannelID) (models.Session, error)
	// TogglePause(context.Context, sessionKey) (Session, error)
	RestoreSessions() error

	//
	HasVoiceSession(voiceCID string) bool
	GuildSessionCnt(gid string) int

	// hooks
	OnSessionUpdate(func(context.Context, models.Session))
	OnSessionNextInterval(func(context.Context, models.Session))
	OnSessionCleanup(func(context.Context, models.Session))

	//
	Shutdown() error
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
		sessions:         make(map[pomomo.TextChannelID]*models.Session),
		locks:            make(map[pomomo.TextChannelID]*sync.Mutex),
		cancelFuncs:      make(map[pomomo.TextChannelID]func()),
		guildSessionCnts: make(map[string]int),
		voiceChannels:    make(map[pomomo.VoiceChannelID]struct{}),
	}

	return &sessionManager{
		cache:     &cache,
		repo:      repo,
		tx:        tx,
		parentCtx: ctx,
	}
}

func (m *sessionManager) HasVoiceSession(voiceCID string) bool {
	m.cache.cacheMu.RLock()
	defer m.cache.cacheMu.RUnlock()
	_, exists := m.cache.voiceChannels[pomomo.VoiceChannelID(voiceCID)]
	return exists
}

func (m *sessionManager) GuildSessionCnt(gid string) int {
	m.cache.cacheMu.RLock()
	defer m.cache.cacheMu.RUnlock()
	return m.cache.guildSessionCnts[gid]
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
				// has been stale for more than an hour
				go func() {
					if _, err := m.endSession(m.parentCtx, session); err != nil {
						log.Error("failed ending stale session", "sessionID", session.ID, "err", err)
					}
					log.Info("ended stale session", "sessionID", session.ID)
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
		m.startUpdateLoop(sessionCtx, session.Record.TextCID)
	}
	log.Info("restored pending sessions", "count", len(toRestore))
	return nil
}

func (m *sessionManager) HasSession(textCID string) bool {
	return m.cache.Has(pomomo.TextChannelID(textCID))
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
		_, err := m.repo.UpdateSession(ctx, s.ID, s.Record)
		return err
	})
}

func (m *sessionManager) startUpdateLoop(ctx context.Context, cid pomomo.TextChannelID) {
	m.wg.Go(func() {
		var updateMu, nextIntervalMu sync.Mutex
		ticker := time.NewTicker(updateTickRate)
		for {
			func() {
				s, unlock := m.cache.Get(cid)
				defer unlock()
				if s == nil {
					log.Info("ending update loop - session not found", "textCID", cid)
					return
				}

				prevStatus := s.Record.CurrentInterval
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

				if prevStatus != s.Record.CurrentInterval && m.onNextInterval != nil {
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
	session := models.NewSession("", req.guildID, req.textCID, req.voiceCID, req.messageID, req.settings)

	if m.cache.Has(session.Record.TextCID) {
		return models.Session{}, fmt.Errorf("session already exists for guild %s channel %s", req.guildID, req.textCID)
	}

	// Execute transaction
	release := m.cache.Hold(session.Record.TextCID)
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		// Insert session record
		inserted, err := m.repo.InsertSession(ctx, session.Record)
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
	m.startUpdateLoop(sessionCtxs[0], session.Record.TextCID)
	return session, nil
}

func (m *sessionManager) SkipInterval(ctx context.Context, cid pomomo.TextChannelID) (models.Session, error) {
	s, unlock := m.cache.Get(cid)
	if s == nil {
		return models.Session{}, fmt.Errorf("session not found for textCID: %v", cid)
	}
	defer unlock()

	s.GoNextInterval(false)

	// Update database with new interval state
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := m.repo.UpdateSession(ctx, s.ID, s.Record)
		return err
	})
	if err != nil {
		return models.Session{}, fmt.Errorf("failed to skip interval: %w", err)
	}

	return *s, nil
}

func (m *sessionManager) endSession(ctx context.Context, s models.Session) (models.Session, error) {
	// Create a new record with ended status for database update
	s.Record.Status = pomomo.SessionEnded
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		_, err := m.repo.UpdateSession(ctx, s.ID, s.Record)
		if err != nil {
			return fmt.Errorf("failed to update session status: %w", err)
		}

		_, err = m.repo.DeleteSettings(ctx, s.ID)
		if err != nil {
			return fmt.Errorf("failed to delete settings: %w", err)
		}
		return nil
	})
	if m.onCleanup != nil {
		go m.onCleanup(ctx, s)
	}
	return s, err
}

func (m *sessionManager) EndSession(ctx context.Context, cid pomomo.TextChannelID) (models.Session, error) {
	s, unlock := m.cache.Get(cid)
	if s == nil {
		return models.Session{}, fmt.Errorf("session not found for textCID: %v", cid)
	}

	session, err := m.endSession(ctx, *s)
	unlock()
	if err != nil {
		return models.Session{}, fmt.Errorf("failed to end session: %w", err)
	}

	m.cache.Remove(cid)
	return session, nil
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
	cacheMu          sync.RWMutex
	sessions         map[pomomo.TextChannelID]*models.Session
	locks            map[pomomo.TextChannelID]*sync.Mutex
	cancelFuncs      map[pomomo.TextChannelID]func()
	voiceChannels    map[pomomo.VoiceChannelID]struct{}
	guildSessionCnts map[string]int
}

// Add returns cancellable session contexts
func (c *sessionCache) Add(ctx context.Context, sessions ...*models.Session) []context.Context {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	sessionCtxs := make([]context.Context, 0, len(sessions))
	for _, s := range sessions {
		key := s.Record.TextCID
		_, exists := c.locks[key] // checks locks instead of sessions in case of Hold()
		if exists {
			panic("session already exists for key: " + key)
		}
		c.locks[key] = &sync.Mutex{}
		c.sessions[key] = s
		sessionCtx, cancel := context.WithCancel(ctx)
		c.cancelFuncs[key] = cancel
		sessionCtxs = append(sessionCtxs, sessionCtx)
		c.voiceChannels[s.Record.VoiceCID] = struct{}{}
		c.guildSessionCnts[s.Record.GuildID] += 1
	}

	return sessionCtxs
}

func (c *sessionCache) Remove(cid pomomo.TextChannelID) {
	s, _ := c.Get(cid)
	if s == nil {
		log.Debug("session not found", "textCID", cid)
		return
	}

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cancelFuncs[cid]()
	delete(c.cancelFuncs, cid)
	delete(c.sessions, cid)
	delete(c.locks, cid)
	delete(c.voiceChannels, s.Record.VoiceCID)
	c.guildSessionCnts[s.Record.GuildID] -= 1
}

func (c *sessionCache) Get(cid pomomo.TextChannelID) (*models.Session, func()) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	_, exists := c.locks[cid] // checks locks instead of sessions in case of Hold()
	if !exists {
		return nil, nil
	}

	l := c.locks[cid]
	l.Lock()
	return c.sessions[cid], l.Unlock
}

func (c *sessionCache) Has(cid pomomo.TextChannelID) bool {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	_, exists := c.locks[cid] // checks locks instead of sessions in case of Hold()
	return exists
}

func (c *sessionCache) Hold(cid pomomo.TextChannelID) func() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if l := c.locks[cid]; l != nil {
		l.Lock()
		return l.Unlock
	}

	mu := &sync.Mutex{}
	mu.Lock()
	c.locks[cid] = mu
	return func() {
		c.cacheMu.Lock()
		defer c.cacheMu.Unlock()
		delete(c.locks, cid)
		mu.Unlock()
	}
}
