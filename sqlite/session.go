// Package sqlite implements repo interfaces
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	txStdLib "github.com/Thiht/transactor/stdlib"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"

	"github.com/benjamonnguyen/deadsimple/database/sqliteutil"
	"github.com/benjamonnguyen/pomomo-go"
)

const (
	SelectAllSessions = "SELECT id, guild_id, channel_id, started_at, seconds_elapsed, status, created_at, updated_at FROM sessions"
	SelectAllSettings = "SELECT id, session_id, pomodoro_duration, short_break_duration, long_break_duration, intervals, created_at, updated_at FROM session_settings"
)

type sessionEntity struct {
	ID             string
	GuildID        string
	ChannelID      string
	StartedAt      int64
	SecondsElapsed int
	Status         uint8
	CreatedAt      int64
	UpdatedAt      int64
}

type sessionSettingsEntity struct {
	ID                 string
	SessionID          string
	PomodoroDuration   int
	ShortBreakDuration int
	LongBreakDuration  int
	Intervals          int
	CreatedAt          int64
	UpdatedAt          int64
}

// sessionRepo
type sessionRepo struct {
	dbGetter txStdLib.DBGetter
	l        log.Logger
}

var _ pomomo.SessionRepo = (*sessionRepo)(nil)

func NewSessionRepo(dbGetter txStdLib.DBGetter, logger log.Logger) pomomo.SessionRepo {
	return &sessionRepo{
		l:        logger,
		dbGetter: dbGetter,
	}
}

func (r *sessionRepo) InsertSession(ctx context.Context, session pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
	if session.GuildID == "" || session.ChannelID == "" {
		return pomomo.ExistingSessionRecord{}, fmt.Errorf("provide required fields 'GuildID' and 'ChannelID'")
	}

	db := r.dbGetter(ctx)
	now := time.Now()

	existingRecord := pomomo.ExistingSessionRecord{
		SessionRecord: session,
		DBRow: pomomo.DBRow{
			ID:        fmt.Sprintf("%d", now.UnixNano()), // Simple ID generation
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	e := mapToSessionEntity(existingRecord)

	args := []any{
		e.ID,
		e.GuildID,
		e.ChannelID,
		e.StartedAt,
		e.SecondsElapsed,
		e.Status,
		e.CreatedAt,
		e.UpdatedAt,
	}
	query := "INSERT INTO sessions (id, guild_id, channel_id, started_at, seconds_elapsed, status, created_at, updated_at) VALUES " + sqliteutil.GenerateParameters(len(args))
	r.l.Debug("creating session", "query", query, "args", args)
	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingSessionRecord{}, err
	}

	return existingRecord, nil
}

func (r *sessionRepo) UpdateSession(ctx context.Context, id int, s pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
	existing, err := r.GetSession(ctx, fmt.Sprintf("%d", id))
	if err != nil {
		return existing, err
	}

	existing.SessionRecord = s
	existing.UpdatedAt = time.Now()
	e := mapToSessionEntity(existing)

	query := "UPDATE sessions SET guild_id = ?, channel_id = ?, started_at = ?, seconds_elapsed = ?, status = ?, updated_at = ? WHERE id = ?"
	args := []any{
		e.GuildID,
		e.ChannelID,
		e.StartedAt,
		e.SecondsElapsed,
		e.Status,
		e.UpdatedAt,
		e.ID,
	}
	r.l.Debug("updating session", "query", query, "args", args)
	_, err = r.dbGetter(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingSessionRecord{}, err
	}

	return existing, nil
}

func (r *sessionRepo) DeleteSession(ctx context.Context, id int) (pomomo.ExistingSessionRecord, error) {
	existing, err := r.GetSession(ctx, fmt.Sprintf("%d", id))
	if err != nil {
		return pomomo.ExistingSessionRecord{}, err
	}

	db := r.dbGetter(ctx)
	query := "DELETE FROM sessions WHERE id = ?"
	r.l.Debug("deleting session", "query", query, "id", id)
	if _, err := db.ExecContext(ctx, query, id); err != nil {
		return pomomo.ExistingSessionRecord{}, err
	}

	return existing, nil
}

func (r *sessionRepo) GetSession(ctx context.Context, id string) (pomomo.ExistingSessionRecord, error) {
	if id == "" {
		return pomomo.ExistingSessionRecord{}, fmt.Errorf("provide id")
	}

	db := r.dbGetter(ctx)
	row := db.QueryRowContext(
		ctx,
		fmt.Sprintf("%s WHERE id=?", SelectAllSessions), id,
	)

	return extractSession(row)
}

func (r *sessionRepo) InsertSettings(ctx context.Context, settings pomomo.SessionSettingsRecord) (pomomo.ExistingSessionSettingsRecord, error) {
	if settings.SessionID == "" {
		return pomomo.ExistingSessionSettingsRecord{}, fmt.Errorf("provide required field 'SessionID'")
	}

	db := r.dbGetter(ctx)
	now := time.Now()

	existingRecord := pomomo.ExistingSessionSettingsRecord{
		SessionSettingsRecord: settings,
		DBRow: pomomo.DBRow{
			ID:        uuid.NewString(),
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	e := mapToSessionSettingsEntity(existingRecord)

	args := []any{
		e.ID,
		e.SessionID,
		e.PomodoroDuration,
		e.ShortBreakDuration,
		e.LongBreakDuration,
		e.Intervals,
		e.CreatedAt,
		e.UpdatedAt,
	}
	query := "INSERT INTO session_settings (id, session_id, pomodoro_duration, short_break_duration, long_break_duration, intervals, created_at, updated_at) VALUES " + sqliteutil.GenerateParameters(len(args))
	r.l.Debug("creating session settings", "query", query, "args", args)
	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return existingRecord, nil
}

func (r *sessionRepo) UpdateSetting(ctx context.Context, id int, s pomomo.SessionSettingsRecord) (pomomo.ExistingSessionSettingsRecord, error) {
	existing, err := r.GetSetting(ctx, fmt.Sprintf("%d", id))
	if err != nil {
		return existing, err
	}

	existing.SessionSettingsRecord = s
	existing.UpdatedAt = time.Now()
	e := mapToSessionSettingsEntity(existing)

	query := "UPDATE session_settings SET session_id = ?, pomodoro_duration = ?, short_break_duration = ?, long_break_duration = ?, intervals = ?, updated_at = ? WHERE id = ?"
	args := []any{
		e.SessionID,
		e.PomodoroDuration,
		e.ShortBreakDuration,
		e.LongBreakDuration,
		e.Intervals,
		e.UpdatedAt,
		e.ID,
	}
	r.l.Debug("updating session setting", "query", query, "args", args)
	_, err = r.dbGetter(ctx).ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return existing, nil
}

func (r *sessionRepo) DeleteSetting(ctx context.Context, id int) (pomomo.ExistingSessionSettingsRecord, error) {
	existing, err := r.GetSetting(ctx, fmt.Sprintf("%d", id))
	if err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	db := r.dbGetter(ctx)
	query := "DELETE FROM session_settings WHERE id = ?"
	r.l.Debug("deleting session setting", "query", query, "id", id)
	if _, err := db.ExecContext(ctx, query, id); err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return existing, nil
}

func (r *sessionRepo) GetSetting(ctx context.Context, id string) (pomomo.ExistingSessionSettingsRecord, error) {
	if id == "" {
		return pomomo.ExistingSessionSettingsRecord{}, fmt.Errorf("provide id")
	}

	db := r.dbGetter(ctx)
	row := db.QueryRowContext(
		ctx,
		fmt.Sprintf("%s WHERE id=?", SelectAllSettings), id,
	)

	return extractSessionSettings(row)
}

func extractSession(s sqliteutil.Scannable) (pomomo.ExistingSessionRecord, error) {
	var e sessionEntity
	if err := s.Scan(&e.ID, &e.GuildID, &e.ChannelID, &e.StartedAt, &e.SecondsElapsed, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pomomo.ExistingSessionRecord{}, ErrNotFound
		}
		return pomomo.ExistingSessionRecord{}, err
	}

	return mapToExistingSessionRecord(e), nil
}

func extractSessionSettings(s sqliteutil.Scannable) (pomomo.ExistingSessionSettingsRecord, error) {
	var e sessionSettingsEntity
	if err := s.Scan(&e.ID, &e.SessionID, &e.PomodoroDuration, &e.ShortBreakDuration, &e.LongBreakDuration, &e.Intervals, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pomomo.ExistingSessionSettingsRecord{}, ErrNotFound
		}
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return mapToExistingSessionSettingsRecord(e), nil
}

func mapToSessionEntity(session pomomo.ExistingSessionRecord) sessionEntity {
	return sessionEntity{
		ID:             session.ID,
		GuildID:        session.GuildID,
		ChannelID:      session.ChannelID,
		StartedAt:      session.StartedAt.Unix(),
		SecondsElapsed: session.SecondsElapsed,
		Status:         uint8(session.Status),
		CreatedAt:      session.CreatedAt.Unix(),
		UpdatedAt:      session.UpdatedAt.Unix(),
	}
}

func mapToSessionSettingsEntity(settings pomomo.ExistingSessionSettingsRecord) sessionSettingsEntity {
	return sessionSettingsEntity{
		ID:                 settings.ID,
		SessionID:          settings.SessionID,
		PomodoroDuration:   int(settings.Pomodoro.Seconds()),
		ShortBreakDuration: int(settings.ShortBreak.Seconds()),
		LongBreakDuration:  int(settings.LongBreak.Seconds()),
		Intervals:          settings.Intervals,
		CreatedAt:          settings.CreatedAt.Unix(),
		UpdatedAt:          settings.UpdatedAt.Unix(),
	}
}

func mapToExistingSessionRecord(e sessionEntity) pomomo.ExistingSessionRecord {
	return pomomo.ExistingSessionRecord{
		DBRow: pomomo.DBRow{
			ID:        e.ID,
			CreatedAt: time.Unix(int64(e.CreatedAt), 0),
			UpdatedAt: time.Unix(int64(e.UpdatedAt), 0),
		},
		SessionRecord: pomomo.SessionRecord{
			GuildID:        e.GuildID,
			ChannelID:      e.ChannelID,
			StartedAt:      time.Unix(int64(e.StartedAt), 0),
			SecondsElapsed: e.SecondsElapsed,
			Status:         pomomo.SessionStatus(e.Status),
		},
	}
}

func mapToExistingSessionSettingsRecord(e sessionSettingsEntity) pomomo.ExistingSessionSettingsRecord {
	return pomomo.ExistingSessionSettingsRecord{
		DBRow: pomomo.DBRow{
			ID:        e.ID,
			CreatedAt: time.Unix(int64(e.CreatedAt), 0),
			UpdatedAt: time.Unix(int64(e.UpdatedAt), 0),
		},
		SessionSettingsRecord: pomomo.SessionSettingsRecord{
			SessionID:  e.SessionID,
			Pomodoro:   time.Duration(e.PomodoroDuration) * time.Second,
			ShortBreak: time.Duration(e.ShortBreakDuration) * time.Second,
			LongBreak:  time.Duration(e.LongBreakDuration) * time.Second,
			Intervals:  e.Intervals,
		},
	}
}
