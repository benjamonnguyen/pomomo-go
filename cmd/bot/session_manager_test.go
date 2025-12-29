package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/Thiht/transactor"
)

// mockSessionRepo is a mock implementation of pomomo.SessionRepo
type mockSessionRepo struct {
	insertSessionFunc func(context.Context, pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error)
	insertSettingsFunc func(context.Context, pomomo.SessionSettingsRecord) (pomomo.ExistingSessionSettingsRecord, error)
	// other methods not needed for these tests
}

func (m *mockSessionRepo) InsertSession(ctx context.Context, sr pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
	if m.insertSessionFunc != nil {
		return m.insertSessionFunc(ctx, sr)
	}
	return pomomo.ExistingSessionRecord{}, nil
}

func (m *mockSessionRepo) UpdateSession(ctx context.Context, id string, s pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
	return pomomo.ExistingSessionRecord{}, nil
}

func (m *mockSessionRepo) DeleteSession(ctx context.Context, id string) (pomomo.ExistingSessionRecord, error) {
	return pomomo.ExistingSessionRecord{}, nil
}

func (m *mockSessionRepo) GetSession(ctx context.Context, id string) (pomomo.ExistingSessionRecord, error) {
	return pomomo.ExistingSessionRecord{}, nil
}

func (m *mockSessionRepo) InsertSettings(ctx context.Context, ssr pomomo.SessionSettingsRecord) (pomomo.ExistingSessionSettingsRecord, error) {
	if m.insertSettingsFunc != nil {
		return m.insertSettingsFunc(ctx, ssr)
	}
	return pomomo.ExistingSessionSettingsRecord{}, nil
}

func (m *mockSessionRepo) DeleteSettings(ctx context.Context, id string) (pomomo.ExistingSessionSettingsRecord, error) {
	return pomomo.ExistingSessionSettingsRecord{}, nil
}

func (m *mockSessionRepo) GetSettings(ctx context.Context, id string) (pomomo.ExistingSessionSettingsRecord, error) {
	return pomomo.ExistingSessionSettingsRecord{}, nil
}

// mockTransactor is a mock implementation of transactor.Transactor
type mockTransactor struct {
	withinTransactionFunc func(context.Context, func(context.Context) error) error
}

func (m *mockTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if m.withinTransactionFunc != nil {
		return m.withinTransactionFunc(ctx, fn)
	}
	return fn(ctx)
}

var _ transactor.Transactor = (*mockTransactor)(nil)

func TestSessionManager_StartSession(t *testing.T) {
	t.Parallel()

	validRequest := startSessionRequest{
		guildID:   "guild123",
		channelID: "channel456",
		settings: SessionSettings{
			pomodoro:   25 * time.Minute,
			shortBreak: 5 * time.Minute,
			longBreak:  15 * time.Minute,
			intervals:  4,
		},
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		expectedSessionID := "session-123"
		repo := &mockSessionRepo{
			insertSessionFunc: func(ctx context.Context, sr pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
				// Verify the session record matches request
				if sr.GuildID != validRequest.guildID || sr.ChannelID != validRequest.channelID {
					t.Errorf("SessionRecord mismatch: got guild %q channel %q", sr.GuildID, sr.ChannelID)
				}
				if sr.Status != pomomo.SessionRunning {
					t.Errorf("expected status SessionRunning, got %v", sr.Status)
				}
				// Return session with ID
				return pomomo.ExistingSessionRecord{
					DBRow: pomomo.DBRow{
						ID: expectedSessionID,
					},
					SessionRecord: sr,
				}, nil
			},
			insertSettingsFunc: func(ctx context.Context, ssr pomomo.SessionSettingsRecord) (pomomo.ExistingSessionSettingsRecord, error) {
				// Verify settings match request
				if ssr.SessionID != expectedSessionID {
					t.Errorf("Settings SessionID mismatch: got %q", ssr.SessionID)
				}
				if ssr.Pomodoro != validRequest.settings.pomodoro {
					t.Errorf("Pomodoro duration mismatch")
				}
				if ssr.ShortBreak != validRequest.settings.shortBreak {
					t.Errorf("ShortBreak duration mismatch")
				}
				if ssr.LongBreak != validRequest.settings.longBreak {
					t.Errorf("LongBreak duration mismatch")
				}
				if ssr.Intervals != validRequest.settings.intervals {
					t.Errorf("Intervals mismatch")
				}
				return pomomo.ExistingSessionSettingsRecord{
					DBRow: pomomo.DBRow{
						ID: "settings-123",
					},
					SessionSettingsRecord: ssr,
				}, nil
			},
		}
		tx := &mockTransactor{}
		manager := NewSessionManager(repo, tx)

		// Call StartSession
		session, err := manager.StartSession(context.Background(), validRequest)
		if err != nil {
			t.Fatalf("StartSession failed: %v", err)
		}

		// Verify returned session
		if session == nil {
			t.Fatal("expected session to be non-nil")
		}
		if session.sessionID != expectedSessionID {
			t.Errorf("expected sessionID %q, got %q", expectedSessionID, session.sessionID)
		}
		if session.guildID != validRequest.guildID {
			t.Errorf("expected guildID %q, got %q", validRequest.guildID, session.guildID)
		}
		if session.channelID != validRequest.channelID {
			t.Errorf("expected channelID %q, got %q", validRequest.channelID, session.channelID)
		}
		if session.currentInterval != PomodoroInterval {
			t.Errorf("expected currentInterval Pomodoro, got %q", session.currentInterval)
		}
		if session.stats.completedPomodoros != 0 {
			t.Errorf("expected completedPomodoros 0, got %d", session.stats.completedPomodoros)
		}
		// Verify cache contains session
		key := session.key()
		cached := manager.(*sessionManager).cache[key]
		if cached == nil {
			t.Fatal("expected session to be cached")
		}
		if cached != session {
			t.Error("cached session is not the same instance")
		}
	})

	t.Run("duplicate session", func(t *testing.T) {
		t.Parallel()

		repo := &mockSessionRepo{}
		tx := &mockTransactor{}
		manager := NewSessionManager(repo, tx)

		// Start first session
		_, err := manager.StartSession(context.Background(), validRequest)
		if err != nil {
			t.Fatalf("first StartSession failed: %v", err)
		}

		// Try to start duplicate session
		_, err = manager.StartSession(context.Background(), validRequest)
		if err == nil {
			t.Fatal("expected error for duplicate session")
		}
		expectedErr := "session already exists for guild guild123 channel channel456"
		if err.Error() != expectedErr {
			t.Errorf("expected error %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("validation error - missing guildID", func(t *testing.T) {
		t.Parallel()

		repo := &mockSessionRepo{}
		tx := &mockTransactor{}
		manager := NewSessionManager(repo, tx)

		invalidRequest := validRequest
		invalidRequest.guildID = ""
		_, err := manager.StartSession(context.Background(), invalidRequest)
		if err == nil {
			t.Fatal("expected error for missing guildID")
		}
		// error from cacheKey.validate
	})

	t.Run("validation error - missing channelID", func(t *testing.T) {
		t.Parallel()

		repo := &mockSessionRepo{}
		tx := &mockTransactor{}
		manager := NewSessionManager(repo, tx)

		invalidRequest := validRequest
		invalidRequest.channelID = ""
		_, err := manager.StartSession(context.Background(), invalidRequest)
		if err == nil {
			t.Fatal("expected error for missing channelID")
		}
	})

	t.Run("transaction rollback on settings insert failure", func(t *testing.T) {
		t.Parallel()

		repo := &mockSessionRepo{
			insertSessionFunc: func(ctx context.Context, sr pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
				// Return a session with ID
				return pomomo.ExistingSessionRecord{
					DBRow: pomomo.DBRow{
						ID: "session123",
					},
					SessionRecord: sr,
				}, nil
			},
			insertSettingsFunc: func(ctx context.Context, ssr pomomo.SessionSettingsRecord) (pomomo.ExistingSessionSettingsRecord, error) {
				// Simulate failure
				return pomomo.ExistingSessionSettingsRecord{}, errors.New("settings insert failed")
			},
		}
		tx := &mockTransactor{}
		manager := NewSessionManager(repo, tx)

		_, err := manager.StartSession(context.Background(), validRequest)
		if err == nil {
			t.Fatal("expected error when settings insert fails")
		}
		// Verify session not cached (transaction rolled back)
		key := cacheKey{guildID: validRequest.guildID, channelID: validRequest.channelID}
		if _, exists := manager.(*sessionManager).cache[key]; exists {
			t.Error("session should not be cached after transaction rollback")
		}
	})

	t.Run("session insert error", func(t *testing.T) {
		t.Parallel()

		repo := &mockSessionRepo{
			insertSessionFunc: func(ctx context.Context, sr pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
				return pomomo.ExistingSessionRecord{}, errors.New("session insert failed")
			},
		}
		tx := &mockTransactor{}
		manager := NewSessionManager(repo, tx)

		_, err := manager.StartSession(context.Background(), validRequest)
		if err == nil {
			t.Fatal("expected error when session insert fails")
		}
	})

	t.Run("transactor error", func(t *testing.T) {
		t.Parallel()

		repo := &mockSessionRepo{}
		tx := &mockTransactor{
			withinTransactionFunc: func(ctx context.Context, fn func(context.Context) error) error {
				return errors.New("transaction begin failed")
			},
		}
		manager := NewSessionManager(repo, tx)

		_, err := manager.StartSession(context.Background(), validRequest)
		if err == nil {
			t.Fatal("expected error when transactor fails")
		}
		// Verify no session cached
		key := cacheKey{guildID: validRequest.guildID, channelID: validRequest.channelID}
		if _, exists := manager.(*sessionManager).cache[key]; exists {
			t.Error("session should not be cached after transactor error")
		}
	})
}

func TestSessionManager_SkipInterval(t *testing.T) {
	t.Parallel()

	validKey := cacheKey{
		guildID:   "guild123",
		channelID: "channel456",
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		repo := &mockSessionRepo{}
		tx := &mockTransactor{}
		manager := NewSessionManager(repo, tx)

		// Manually create and cache a session
		session := &Session{
			guildID:         validKey.guildID,
			channelID:       validKey.channelID,
			currentInterval: PomodoroInterval,
			settings: SessionSettings{
				pomodoro:   25 * time.Minute,
				shortBreak: 5 * time.Minute,
				longBreak:  15 * time.Minute,
				intervals:  4,
			},
		}
		manager.(*sessionManager).cache[validKey] = session

		// Call SkipInterval
		updatedSession, err := manager.SkipInterval(context.Background(), validKey)
		if err != nil {
			t.Fatalf("SkipInterval failed: %v", err)
		}

		if updatedSession == nil {
			t.Fatal("expected session to be non-nil")
		}
		if updatedSession.currentInterval != ShortBreakInterval {
			t.Errorf("expected interval to transition to Short Break, got %q", updatedSession.currentInterval)
		}
		// Verify stats not updated (shouldUpdateStats = false)
		if updatedSession.stats.completedPomodoros != 0 {
			t.Errorf("expected completedPomodoros to remain 0, got %d", updatedSession.stats.completedPomodoros)
		}
	})

	t.Run("session not found", func(t *testing.T) {
		t.Parallel()

		repo := &mockSessionRepo{}
		tx := &mockTransactor{}
		manager := NewSessionManager(repo, tx)

		nonExistentKey := cacheKey{
			guildID:   "nonexistent",
			channelID: "nonexistent",
		}
		_, err := manager.SkipInterval(context.Background(), nonExistentKey)
		if err == nil {
			t.Fatal("expected error when session not found")
		}
		expectedErr := "session not found for key"
		if err.Error()[:len(expectedErr)] != expectedErr {
			t.Errorf("expected error to contain %q, got %q", expectedErr, err.Error())
		}
	})

	t.Run("interval transition logic", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name           string
			initialInterval SessionInterval
			expectedInterval SessionInterval
		}{
			{"pomodoro to short break", PomodoroInterval, ShortBreakInterval},
			{"short break to pomodoro", ShortBreakInterval, PomodoroInterval},
			{"long break to pomodoro", LongBreakInterval, PomodoroInterval},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				repo := &mockSessionRepo{}
				tx := &mockTransactor{}
				manager := NewSessionManager(repo, tx)

				session := &Session{
					guildID:         validKey.guildID,
					channelID:       validKey.channelID,
					currentInterval: tc.initialInterval,
					settings: SessionSettings{
						pomodoro:   25 * time.Minute,
						shortBreak: 5 * time.Minute,
						longBreak:  15 * time.Minute,
						intervals:  4,
					},
				}
				manager.(*sessionManager).cache[validKey] = session

				updatedSession, err := manager.SkipInterval(context.Background(), validKey)
				if err != nil {
					t.Fatalf("SkipInterval failed: %v", err)
				}
				if updatedSession.currentInterval != tc.expectedInterval {
					t.Errorf("expected interval %q, got %q", tc.expectedInterval, updatedSession.currentInterval)
				}
			})
		}
	})
}

func TestCacheKey_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     cacheKey
		wantErr bool
	}{
		{"valid", cacheKey{guildID: "g", channelID: "c"}, false},
		{"missing guildID", cacheKey{guildID: "", channelID: "c"}, true},
		{"missing channelID", cacheKey{guildID: "g", channelID: ""}, true},
		{"both missing", cacheKey{guildID: "", channelID: ""}, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.key.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("cacheKey.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}