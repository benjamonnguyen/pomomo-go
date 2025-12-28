package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Thiht/transactor"
	"github.com/benjamonnguyen/pomomo-go"
)

type startSessionRequest struct {
	guildID, channelID string
	settings           SessionSettings
}

type SessionManager interface {
	StartSession(context.Context, startSessionRequest) (*Session, error)
	// EndSession(context.Context, *Session) should delete settings
	SkipInterval(context.Context, cacheKey) (*Session, error)
}

type cacheKey struct {
	guildID, channelID string
}

func (k cacheKey) validate() error {
	if k.guildID == "" || k.channelID == "" {
		return fmt.Errorf("cacheKey requires guild and channel IDs")
	}
	return nil
}

type sessionManager struct {
	cache map[cacheKey]*Session
	repo  pomomo.SessionRepo
	tx    transactor.Transactor
}

func NewSessionManager(repo pomomo.SessionRepo, tx transactor.Transactor) SessionManager {
	// TODO repo.GetByStatus(...status) to populate cache
	return &sessionManager{
		cache: make(map[cacheKey]*Session),
		repo:  repo,
		tx:    tx,
	}
}

func (m *sessionManager) StartSession(ctx context.Context, req startSessionRequest) (*Session, error) {
	s := &Session{
		channelID:       req.channelID,
		guildID:         req.guildID,
		settings:        req.settings,
		currentInterval: PomodoroInterval,
	}
	key := s.key()
	if err := key.validate(); err != nil {
		return nil, err
	}
	if _, exists := m.cache[key]; exists {
		return nil, fmt.Errorf("session already exists for guild %s channel %s", req.guildID, req.channelID)
	}

	// Execute transaction
	err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		// Insert session record
		sessionRecord := pomomo.SessionRecord{
			ChannelID: req.channelID,
			GuildID:   req.guildID,
			Status:    pomomo.SessionRunning,
			StartedAt: time.Now(),
		}
		existingSession, err := m.repo.InsertSession(ctx, sessionRecord)
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

		// Store in cache
		s.sessionID = existingSession.ID
		m.cache[key] = s

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start session: %w", err)
	}

	return s, nil
}

func (m *sessionManager) SkipInterval(ctx context.Context, key cacheKey) (*Session, error) {
	s := m.cache[key]
	if s == nil {
		return nil, fmt.Errorf("session not found for key: %v", key)
	}

	s.goNextInterval(false)
	return s, nil
}
