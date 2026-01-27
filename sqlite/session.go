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

	"github.com/benjamonnguyen/deadsimple/database/sqliteutil"
	"github.com/benjamonnguyen/pomomo-go"
)

const (
	SelectAllSessions     = "SELECT id, guild_id, text_channel_id, voice_channel_id, message_id, interval_started_at, time_remaining_at_start, current_interval, status, created_at, updated_at FROM sessions"
	SelectAllSettings     = "SELECT session_id, pomodoro_duration, short_break_duration, long_break_duration, intervals, created_at, updated_at FROM session_settings"
	SelectAllParticipants = "SELECT id, user_id, session_id, voice_channel_id, is_muted, is_deafened, created_at, updated_at FROM session_participants"
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
	CreatedAt          int64
	UpdatedAt          int64
}

type sessionParticipantEntity struct {
	ID             string
	UserID         string
	SessionID      string
	VoiceChannelID string
	IsMuted        string
	IsDeafened     string
	CreatedAt      int64
	UpdatedAt      int64
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

func (r *sessionRepo) UpdateSession(ctx context.Context, id string, s pomomo.SessionRecord) (pomomo.ExistingSessionRecord, error) {
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

func (r *sessionRepo) DeleteSession(ctx context.Context, id string) (pomomo.ExistingSessionRecord, error) {
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
	now := time.Now()

	existingRecord := pomomo.ExistingSessionSettingsRecord{
		SessionSettingsRecord: settings,
		DBRow: pomomo.DBRow{
			ID:        settings.SessionID,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	e := mapToSessionSettingsEntity(existingRecord)

	args := []any{
		e.SessionID,
		e.PomodoroDuration,
		e.ShortBreakDuration,
		e.LongBreakDuration,
		e.Intervals,
		e.CreatedAt,
		e.UpdatedAt,
	}
	query := "INSERT INTO session_settings (session_id, pomodoro_duration, short_break_duration, long_break_duration, intervals, created_at, updated_at) VALUES " + sqliteutil.GenerateParameters(len(args))
	r.l.Debug("creating session settings", "query", query, "args", args)
	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return existingRecord, nil
}

func (r *sessionRepo) DeleteSettings(ctx context.Context, id string) (pomomo.ExistingSessionSettingsRecord, error) {
	existing, err := r.GetSettings(ctx, id)
	if err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	db := r.dbGetter(ctx)
	query := "DELETE FROM session_settings WHERE session_id = ?"
	r.l.Debug("deleting session setting", "query", query, "id", id)
	if _, err := db.ExecContext(ctx, query, id); err != nil {
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return existing, nil
}

func (r *sessionRepo) GetSettings(ctx context.Context, id string) (pomomo.ExistingSessionSettingsRecord, error) {
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

func (r *sessionRepo) InsertParticipant(ctx context.Context, participant pomomo.SessionParticipantRecord) (pomomo.ExistingSessionParticipantRecord, error) {
	if participant.UserID == "" || participant.SessionID == "" {
		return pomomo.ExistingSessionParticipantRecord{}, fmt.Errorf("provide required fields 'UserID' and 'SessionID'")
	}

	db := r.dbGetter(ctx)
	now := time.Now()

	existingRecord := pomomo.ExistingSessionParticipantRecord{
		SessionParticipantRecord: participant,
		DBRow: pomomo.DBRow{
			ID:        fmt.Sprintf("%d", now.UnixNano()),
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
	e := mapToParticipantEntity(existingRecord)

	args := []any{
		e.ID,
		e.UserID,
		e.SessionID,
		e.VoiceChannelID,
		e.IsMuted,
		e.IsDeafened,
		e.CreatedAt,
		e.UpdatedAt,
	}
	query := "INSERT INTO session_participants (id, user_id, session_id, voice_channel_id, is_muted, is_deafened, created_at, updated_at) VALUES " + sqliteutil.GenerateParameters(len(args))
	r.l.Debug("creating session participant", "query", query, "args", args)
	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingSessionParticipantRecord{}, err
	}

	return existingRecord, nil
}

func (r *sessionRepo) DeleteParticipant(ctx context.Context, id string) (pomomo.ExistingSessionParticipantRecord, error) {
	existing, err := r.getParticipantByID(ctx, id)
	if err != nil {
		return pomomo.ExistingSessionParticipantRecord{}, err
	}

	db := r.dbGetter(ctx)
	query := "DELETE FROM session_participants WHERE id = ?"
	r.l.Debug("deleting session participant", "query", query, "id", id)
	if _, err := db.ExecContext(ctx, query, id); err != nil {
		return pomomo.ExistingSessionParticipantRecord{}, err
	}

	return existing, nil
}

func (r *sessionRepo) getParticipantByID(ctx context.Context, id string) (pomomo.ExistingSessionParticipantRecord, error) {
	if id == "" {
		return pomomo.ExistingSessionParticipantRecord{}, fmt.Errorf("provide id")
	}

	db := r.dbGetter(ctx)
	row := db.QueryRowContext(
		ctx,
		fmt.Sprintf("%s WHERE id=?", SelectAllParticipants), id,
	)

	return extractParticipant(row)
}

func (r *sessionRepo) GetAllParticipants(ctx context.Context) ([]pomomo.ExistingSessionParticipantRecord, error) {
	db := r.dbGetter(ctx)
	query := SelectAllParticipants
	r.l.Debug("getting all participants", "query", query)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint

	var participants []pomomo.ExistingSessionParticipantRecord
	for rows.Next() {
		participant, err := extractParticipant(rows)
		if err != nil {
			return nil, err
		}
		participants = append(participants, participant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return participants, nil
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
	if err := s.Scan(&e.SessionID, &e.PomodoroDuration, &e.ShortBreakDuration, &e.LongBreakDuration, &e.Intervals, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pomomo.ExistingSessionSettingsRecord{}, ErrNotFound
		}
		return pomomo.ExistingSessionSettingsRecord{}, err
	}

	return mapToExistingSessionSettingsRecord(e), nil
}

func extractParticipant(s sqliteutil.Scannable) (pomomo.ExistingSessionParticipantRecord, error) {
	var e sessionParticipantEntity
	if err := s.Scan(&e.ID, &e.UserID, &e.SessionID, &e.VoiceChannelID, &e.IsMuted, &e.IsDeafened, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pomomo.ExistingSessionParticipantRecord{}, ErrNotFound
		}
		return pomomo.ExistingSessionParticipantRecord{}, err
	}

	return mapToExistingParticipantRecord(e), nil
}

func mapToSessionEntity(session pomomo.ExistingSessionRecord) sessionEntity {
	return sessionEntity{
		ID:                     session.ID,
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
			GuildID:              e.GuildID,
			TextCID:              pomomo.TextChannelID(e.TextChannelID),
			VoiceCID:             pomomo.VoiceChannelID(e.VoiceChannelID),
			MessageID:            e.MessageID,
			IntervalStartedAt:    time.Unix(int64(e.IntervalStartedAt), 0),
			TimeRemainingAtStart: time.Duration(e.TimeRemainingAtStartMS) * time.Millisecond,
			CurrentInterval:      pomomo.SessionInterval(e.CurrentInterval),
			Status:               pomomo.SessionStatus(e.Status),
		},
	}
}

func mapToExistingSessionSettingsRecord(e sessionSettingsEntity) pomomo.ExistingSessionSettingsRecord {
	return pomomo.ExistingSessionSettingsRecord{
		DBRow: pomomo.DBRow{
			ID:        e.SessionID,
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

func mapToParticipantEntity(participant pomomo.ExistingSessionParticipantRecord) sessionParticipantEntity {
	return sessionParticipantEntity{
		ID:             participant.ID,
		UserID:         participant.UserID,
		SessionID:      participant.SessionID,
		VoiceChannelID: string(participant.VoiceCID),
		IsMuted:        participant.IsMuted,
		IsDeafened:     participant.IsDeafened,
		CreatedAt:      participant.CreatedAt.Unix(),
		UpdatedAt:      participant.UpdatedAt.Unix(),
	}
}

func mapToExistingParticipantRecord(e sessionParticipantEntity) pomomo.ExistingSessionParticipantRecord {
	return pomomo.ExistingSessionParticipantRecord{
		DBRow: pomomo.DBRow{
			ID:        e.ID,
			CreatedAt: time.Unix(int64(e.CreatedAt), 0),
			UpdatedAt: time.Unix(int64(e.UpdatedAt), 0),
		},
		SessionParticipantRecord: pomomo.SessionParticipantRecord{
			UserID:     e.UserID,
			SessionID:  e.SessionID,
			VoiceCID:   pomomo.VoiceChannelID(e.VoiceChannelID),
			IsMuted:    e.IsMuted,
			IsDeafened: e.IsDeafened,
		},
	}
}
