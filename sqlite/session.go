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

	"github.com/benjamonnguyen/deadsimple/db/sqliteutil"
	"github.com/benjamonnguyen/pomomo-go"
)

const (
	SelectAllSessions = "SELECT id, guild_id, text_channel_id, voice_channel_id, message_id, interval_started_at, time_remaining_at_start, current_interval, status, created_at, updated_at FROM sessions"
	SelectAllSettings = "SELECT session_id, pomodoro_duration, short_break_duration, long_break_duration, intervals, no_mute, no_deafen, created_at, updated_at FROM session_settings"
)

type sessionEntity struct {
	ID                     string
	GuildID                string
	TextChannelID          string
	VoiceChannelID         string
	MessageID              string
	IntervalStartedAt      int64
	TimeRemainingAtStartMS int64
	CurrentInterval        uint8
	Status                 uint8
	CreatedAt              int64
	UpdatedAt              int64
}

type sessionSettingsEntity struct {
	SessionID          string
	PomodoroDuration   int
	ShortBreakDuration int
	LongBreakDuration  int
	Intervals          int
	NoMute             bool
	NoDeafen           bool
	CreatedAt          int64
	UpdatedAt          int64
}

// sessionRepo
type sessionRepo struct {
	dbGetter txStdLib.DBGetter
	l        log.Logger
	*participantRepo
}

func NewSessionRepo(dbGetter txStdLib.DBGetter, logger log.Logger) *sessionRepo {
	return &sessionRepo{
		l:               logger,
		dbGetter:        dbGetter,
		participantRepo: &participantRepo{dbGetter: dbGetter, l: logger},
	}
}

func (r *sessionRepo) InsertSession(ctx context.Context, session pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
	db := r.dbGetter(ctx)
	existingRecord := pomomo.ExistingSessionRecord{
		SessionRecord:  session,
		ExistingRecord: pomomo.NewExistingRecord[pomomo.SessionID](uuid.NewString()),
	}
	e := mapToSessionEntity(existingRecord)

	args := []any{
		e.ID,
		e.GuildID,
		e.TextChannelID,
		e.VoiceChannelID,
		e.MessageID,
		e.IntervalStartedAt,
		e.TimeRemainingAtStartMS,
		e.CurrentInterval,
		e.Status,
		e.CreatedAt,
		e.UpdatedAt,
	}
	query := "INSERT INTO sessions (id, guild_id, text_channel_id, voice_channel_id, message_id, interval_started_at, time_remaining_at_start, current_interval, status, created_at, updated_at) VALUES " + sqliteutil.GenerateParameters(len(args))
	r.l.Debug("creating session", "query", query, "args", args)
	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingSessionRecord{}, err
	}

	return existingRecord, nil
}

func (r *sessionRepo) UpdateSession(ctx context.Context, id pomomo.SessionID, s pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
	existing, err := r.GetSession(ctx, id)
	if err != nil {
		return existing, err
	}

	existing.SessionRecord = s
	existing.UpdatedAt = time.Now()
	e := mapToSessionEntity(existing)

	query := "UPDATE sessions SET guild_id = ?, text_channel_id = ?, voice_channel_id = ?, message_id = ?, interval_started_at = ?, time_remaining_at_start = ?, current_interval = ?, status = ?, updated_at = ? WHERE id = ?"
	args := []any{
		e.GuildID,
		e.TextChannelID,
		e.VoiceChannelID,
		e.MessageID,
		e.IntervalStartedAt,
		e.TimeRemainingAtStartMS,
		e.CurrentInterval,
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

func (r *sessionRepo) DeleteSession(ctx context.Context, id pomomo.SessionID) (pomomo.ExistingSessionRecord, error) {
	existing, err := r.GetSession(ctx, id)
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

func (r *sessionRepo) GetSession(ctx context.Context, id pomomo.SessionID) (pomomo.ExistingSessionRecord, error) {
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

func (r *sessionRepo) GetSessionsByStatus(ctx context.Context, statuses ...pomomo.SessionStatus) ([]pomomo.ExistingSessionRecord, error) {
	if len(statuses) == 0 {
		return nil, nil
	}

	db := r.dbGetter(ctx)
	query := fmt.Sprintf("%s WHERE status IN %s", SelectAllSessions, sqliteutil.GenerateParameters(len(statuses)))
	log.Debug("getting sessions by status", "query", query, "statuses", statuses)
	var statusInts []any
	for _, s := range statuses {
		statusInts = append(statusInts, uint8(s))
	}
	rows, err := db.QueryContext(ctx, query, statusInts...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint

	var sessions []pomomo.ExistingSessionRecord
	for rows.Next() {
		session, err := extractSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *sessionRepo) InsertSettings(ctx context.Context, settings pomomo.SessionSettingsRecord) (pomomo.ExistingSessionSettingsRecord, error) {
	if settings.SessionID == "" {
		return pomomo.ExistingSessionSettingsRecord{}, fmt.Errorf("provide required field 'SessionID'")
	}

	db := r.dbGetter(ctx)

	existingRecord := pomomo.ExistingSessionSettingsRecord{
		SessionSettingsRecord: settings,
		ExistingRecord:        pomomo.NewExistingRecord[pomomo.SessionID](string(settings.SessionID)),
	}
	e := mapToSessionSettingsEntity(existingRecord)

	args := []any{
		e.SessionID,
		e.PomodoroDuration,
		e.ShortBreakDuration,
		e.LongBreakDuration,
		e.Intervals,
		e.NoMute,
		e.NoDeafen,
		e.CreatedAt,
		e.UpdatedAt,
	}
	query := "INSERT INTO session_settings (session_id, pomodoro_duration, short_break_duration, long_break_duration, intervals, no_mute, no_deafen, created_at, updated_at) VALUES " + sqliteutil.GenerateParameters(len(args))
	r.l.Debug("creating session settings", "query", query, "args", args)
	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return existingRecord, nil
}

func (r *sessionRepo) DeleteSettings(ctx context.Context, id pomomo.SessionID) (pomomo.ExistingSessionSettingsRecord, error) {
	existing, err := r.GetSettings(ctx, id)
	if err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	db := r.dbGetter(ctx)
	query := "DELETE FROM session_settings WHERE session_id = ?"
	r.l.Debug("deleting session settings", "query", query, "session_id", id)
	if _, err := db.ExecContext(ctx, query, id); err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return existing, nil
}

func (r *sessionRepo) GetSettings(ctx context.Context, id pomomo.SessionID) (pomomo.ExistingSessionSettingsRecord, error) {
	if id == "" {
		return pomomo.ExistingSessionSettingsRecord{}, fmt.Errorf("provide id")
	}

	db := r.dbGetter(ctx)
	row := db.QueryRowContext(
		ctx,
		fmt.Sprintf("%s WHERE session_id=?", SelectAllSettings), id,
	)

	return extractSessionSettings(row)
}

func extractSession(s sqliteutil.Scannable) (pomomo.ExistingSessionRecord, error) {
	var e sessionEntity
	if err := s.Scan(&e.ID, &e.GuildID, &e.TextChannelID, &e.VoiceChannelID, &e.MessageID, &e.IntervalStartedAt, &e.TimeRemainingAtStartMS, &e.CurrentInterval, &e.Status, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pomomo.ExistingSessionRecord{}, ErrNotFound
		}
		return pomomo.ExistingSessionRecord{}, err
	}

	return mapToExistingSessionRecord(e), nil
}

func extractSessionSettings(s sqliteutil.Scannable) (pomomo.ExistingSessionSettingsRecord, error) {
	var e sessionSettingsEntity
	if err := s.Scan(&e.SessionID, &e.PomodoroDuration, &e.ShortBreakDuration, &e.LongBreakDuration, &e.Intervals, &e.NoMute, &e.NoDeafen, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pomomo.ExistingSessionSettingsRecord{}, ErrNotFound
		}
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return mapToExistingSessionSettingsRecord(e), nil
}

func mapToSessionEntity(session pomomo.ExistingSessionRecord) sessionEntity {
	return sessionEntity{
		ID:                     string(session.ID),
		GuildID:                session.GuildID,
		TextChannelID:          string(session.TextCID),
		VoiceChannelID:         string(session.VoiceCID),
		MessageID:              session.MessageID,
		IntervalStartedAt:      session.IntervalStartedAt.Unix(),
		TimeRemainingAtStartMS: session.TimeRemainingAtStart.Milliseconds(),
		CurrentInterval:        uint8(session.CurrentInterval),
		Status:                 uint8(session.Status),
		CreatedAt:              session.CreatedAt.Unix(),
		UpdatedAt:              session.UpdatedAt.Unix(),
	}
}

func mapToSessionSettingsEntity(settings pomomo.ExistingSessionSettingsRecord) sessionSettingsEntity {
	return sessionSettingsEntity{
		SessionID:          string(settings.SessionID),
		PomodoroDuration:   int(settings.Pomodoro.Seconds()),
		ShortBreakDuration: int(settings.ShortBreak.Seconds()),
		LongBreakDuration:  int(settings.LongBreak.Seconds()),
		Intervals:          settings.Intervals,
		NoMute:             settings.NoMute,
		NoDeafen:           settings.NoDeafen,
		CreatedAt:          settings.CreatedAt.Unix(),
		UpdatedAt:          settings.UpdatedAt.Unix(),
	}
}

func mapToExistingSessionRecord(e sessionEntity) pomomo.ExistingSessionRecord {
	return pomomo.ExistingSessionRecord{
		ExistingRecord: pomomo.ExistingRecord[pomomo.SessionID]{
			ID:        pomomo.SessionID(e.ID),
			CreatedAt: time.Unix(int64(e.CreatedAt), 0),
			UpdatedAt: time.Unix(int64(e.UpdatedAt), 0),
		},
		SessionRecord: pomomo.SessionRecord{
			GuildID:              e.GuildID,
			TextCID:              pomomo.TextChannelID(e.TextChannelID),
			VoiceCID:             pomomo.VoiceChannelID(e.VoiceChannelID),
			MessageID:            e.MessageID,
			IntervalStartedAt:    time.Unix(int64(e.IntervalStartedAt), 0),
			TimeRemainingAtStart: time.Duration(e.TimeRemainingAtStartMS) * time.Millisecond,
			CurrentInterval:      pomomo.SessionInterval(e.CurrentInterval),
			Status:               pomomo.SessionStatus(e.Status),
			// NoDeafen moved to settings
		},
	}
}

func mapToExistingSessionSettingsRecord(e sessionSettingsEntity) pomomo.ExistingSessionSettingsRecord {
	return pomomo.ExistingSessionSettingsRecord{
		ExistingRecord: pomomo.ExistingRecord[pomomo.SessionID]{
			ID:        pomomo.SessionID(e.SessionID),
			CreatedAt: time.Unix(int64(e.CreatedAt), 0),
			UpdatedAt: time.Unix(int64(e.UpdatedAt), 0),
		},
		SessionSettingsRecord: pomomo.SessionSettingsRecord{
			SessionID:  pomomo.SessionID(e.SessionID),
			Pomodoro:   time.Duration(e.PomodoroDuration) * time.Second,
			ShortBreak: time.Duration(e.ShortBreakDuration) * time.Second,
			LongBreak:  time.Duration(e.LongBreakDuration) * time.Second,
			Intervals:  e.Intervals,
			NoMute:     e.NoMute,
			NoDeafen:   e.NoDeafen,
		},
	}
}
