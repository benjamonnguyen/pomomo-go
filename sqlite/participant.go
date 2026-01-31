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
	SelectAllParticipants = "SELECT id, user_id, session_id, guild_id, voice_channel_id, is_muted, is_deafened, created_at, updated_at FROM session_participants"
	UpdateParticipant     = "UPDATE session_participants SET user_id = ?, session_id = ?, guild_id = ?, voice_channel_id = ?, is_muted = ?, is_deafened = ?, updated_at = ? WHERE id = ?"
)

type participantEntity struct {
	ID             string
	UserID         string
	SessionID      string
	GuildID        string
	VoiceChannelID string
	IsMuted        bool
	IsDeafened     bool
	CreatedAt      int64
	UpdatedAt      int64
}

type participantRepo struct {
	dbGetter txStdLib.DBGetter
	l        log.Logger
}

func NewParticipantRepo(dbGetter txStdLib.DBGetter, logger log.Logger) *participantRepo {
	return &participantRepo{
		dbGetter: dbGetter,
		l:        logger,
	}
}

func (r *participantRepo) InsertParticipant(ctx context.Context, participant pomomo.ParticipantRecord) (pomomo.ExistingParticipantRecord, error) {
	if participant.UserID == "" || participant.SessionID == "" || participant.GuildID == "" {
		return pomomo.ExistingParticipantRecord{}, fmt.Errorf("provide required fields 'UserID', 'SessionID', and 'GuildID'")
	}

	db := r.dbGetter(ctx)

	existingRecord := pomomo.ExistingParticipantRecord{
		ParticipantRecord: participant,
		ExistingRecord:    pomomo.NewExistingRecord[pomomo.ParticipantID](uuid.NewString()),
	}
	e := mapToParticipantEntity(existingRecord)

	args := []any{
		e.ID,
		e.UserID,
		e.SessionID,
		e.GuildID,
		e.VoiceChannelID,
		e.IsMuted,
		e.IsDeafened,
		e.CreatedAt,
		e.UpdatedAt,
	}
	query := "INSERT INTO session_participants (id, user_id, session_id, guild_id, voice_channel_id, is_muted, is_deafened, created_at, updated_at) VALUES " + sqliteutil.GenerateParameters(len(args))
	r.l.Debug("creating session participant", "query", query, "args", args)
	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return pomomo.ExistingParticipantRecord{}, err
	}

	return existingRecord, nil
}

func (r *participantRepo) DeleteParticipant(ctx context.Context, id pomomo.ParticipantID) (pomomo.ExistingParticipantRecord, error) {
	existing, err := r.getParticipantByID(ctx, id)
	if err != nil {
		return pomomo.ExistingParticipantRecord{}, err
	}

	db := r.dbGetter(ctx)
	query := "DELETE FROM session_participants WHERE id = ?"
	r.l.Debug("deleting session participant", "query", query, "id", id)
	if _, err := db.ExecContext(ctx, query, id); err != nil {
		return pomomo.ExistingParticipantRecord{}, err
	}

	return existing, nil
}

func (r *participantRepo) UpdateParticipant(ctx context.Context, id pomomo.ParticipantID, participant pomomo.ParticipantRecord) (pomomo.ExistingParticipantRecord, error) {
	existing, err := r.getParticipantByID(ctx, id)
	if err != nil {
		return existing, err
	}

	existing.ParticipantRecord = participant
	existing.UpdatedAt = time.Now()
	e := mapToParticipantEntity(existing)

	args := []any{
		e.UserID,
		e.SessionID,
		e.GuildID,
		e.VoiceChannelID,
		e.IsMuted,
		e.IsDeafened,
		e.UpdatedAt,
		e.ID,
	}
	r.l.Debug("updating participant", "query", UpdateParticipant, "args", args)
	_, err = r.dbGetter(ctx).ExecContext(ctx, UpdateParticipant, args...)
	if err != nil {
		return pomomo.ExistingParticipantRecord{}, err
	}

	return existing, nil
}

func (r *participantRepo) getParticipantByID(ctx context.Context, id pomomo.ParticipantID) (pomomo.ExistingParticipantRecord, error) {
	if id == "" {
		return pomomo.ExistingParticipantRecord{}, fmt.Errorf("provide id")
	}

	db := r.dbGetter(ctx)
	row := db.QueryRowContext(
		ctx,
		fmt.Sprintf("%s WHERE id=?", SelectAllParticipants), id,
	)

	return extractParticipant(row)
}

func (r *participantRepo) GetAllParticipants(ctx context.Context) ([]pomomo.ExistingParticipantRecord, error) {
	db := r.dbGetter(ctx)
	query := SelectAllParticipants
	r.l.Debug("getting all participants", "query", query)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint

	var participants []pomomo.ExistingParticipantRecord
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

func (r *participantRepo) GetParticipantByUserID(ctx context.Context, userID string) (pomomo.ExistingParticipantRecord, error) {
	if userID == "" {
		return pomomo.ExistingParticipantRecord{}, fmt.Errorf("provide userID")
	}

	db := r.dbGetter(ctx)
	row := db.QueryRowContext(
		ctx,
		fmt.Sprintf("%s WHERE user_id=?", SelectAllParticipants), userID,
	)

	return extractParticipant(row)
}

func extractParticipant(s sqliteutil.Scannable) (pomomo.ExistingParticipantRecord, error) {
	var e participantEntity
	if err := s.Scan(&e.ID, &e.UserID, &e.SessionID, &e.GuildID, &e.VoiceChannelID, &e.IsMuted, &e.IsDeafened, &e.CreatedAt, &e.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return pomomo.ExistingParticipantRecord{}, ErrNotFound
		}
		return pomomo.ExistingParticipantRecord{}, err
	}

	return mapToExistingParticipantRecord(e), nil
}

func mapToParticipantEntity(participant pomomo.ExistingParticipantRecord) participantEntity {
	return participantEntity{
		ID:             string(participant.ID),
		UserID:         participant.UserID,
		SessionID:      string(participant.SessionID),
		GuildID:        participant.GuildID,
		VoiceChannelID: string(participant.VoiceCID),
		IsMuted:        participant.IsMuted,
		IsDeafened:     participant.IsDeafened,
		CreatedAt:      participant.CreatedAt.Unix(),
		UpdatedAt:      participant.UpdatedAt.Unix(),
	}
}

func mapToExistingParticipantRecord(e participantEntity) pomomo.ExistingParticipantRecord {
	return pomomo.ExistingParticipantRecord{
		ExistingRecord: pomomo.ExistingRecord[pomomo.ParticipantID]{
			ID:        pomomo.ParticipantID(e.ID),
			CreatedAt: time.Unix(int64(e.CreatedAt), 0),
			UpdatedAt: time.Unix(int64(e.UpdatedAt), 0),
		},
		ParticipantRecord: pomomo.ParticipantRecord{
			UserID:     e.UserID,
			SessionID:  pomomo.SessionID(e.SessionID),
			GuildID:    e.GuildID,
			VoiceCID:   pomomo.VoiceChannelID(e.VoiceChannelID),
			IsMuted:    e.IsMuted,
			IsDeafened: e.IsDeafened,
		},
	}
}
