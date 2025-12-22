package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Thiht/transactor"
	"github.com/benjamonnguyen/pomomo-go"
)

type Session struct {
	sessionID, guildID, channelID string
	settings                      SessionSettings
	// TODO connection instance
}

type SessionSettings struct {
	pomodoro, shortBreak, longBreak time.Duration
	intervals                       int
}

func (s Session) key() cacheKey {
	return cacheKey{
		guildID:   s.guildID,
		channelID: s.channelID,
	}
}

type startSessionRequest struct {
	guildID, channelID string
	settings           SessionSettings
}

type SessionManager interface {
	StartSession(context.Context, startSessionRequest) (Session, error)
	// EndSession(context.Context, *Session)
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
	cache map[cacheKey]Session
	repo  pomomo.SessionRepo
	tx    transactor.Transactor
}

func NewSessionManager(repo pomomo.SessionRepo, tx transactor.Transactor) SessionManager {
	// TODO repo.GetByStatus(...status) to populate cache
	return &sessionManager{
		cache: make(map[cacheKey]Session),
		repo:  repo,
		tx:    tx,
	}
}

func (m *sessionManager) StartSession(ctx context.Context, req startSessionRequest) (Session, error) {
	s := Session{
		channelID: req.channelID,
		guildID:   req.guildID,
		settings:  req.settings,
	}
	key := s.key()
	if err := key.validate(); err != nil {
		return s, err
	}
	if _, exists := m.cache[key]; exists {
		return s, fmt.Errorf("session already exists for guild %s channel %s", req.guildID, req.channelID)
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
		return s, fmt.Errorf("failed to start session: %w", err)
	}

	return s, nil
}
