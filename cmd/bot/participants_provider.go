package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/charmbracelet/log"
)

type ParticipantsProvider interface {
	Insert(context.Context, pomomo.SessionParticipantRecord) (models.SessionParticipant, error)
	Delete(context.Context, pomomo.SessionParticipantID) error
	UpdateVoiceState(context.Context, string, pomomo.VoiceChannelID, pomomo.VoiceState) (*models.SessionParticipant, error)
	Get(string, pomomo.VoiceChannelID) *models.SessionParticipant
	GetAll(pomomo.VoiceChannelID) []*models.SessionParticipant
	GetVoiceChannelIDs() []pomomo.VoiceChannelID

	// AcquireVoiceChannelLock returns unlockFn that caller is responsible for calling
	AcquireVoiceChannelLock(pomomo.VoiceChannelID) func()

	// RestoreCache fetches active participants from repo
	RestoreCache(context.Context) error
}

type participantsProvider struct {
	cache *participantsCache
	repo  pomomo.SessionRepo
	l     log.Logger
}

func NewParticipantsProvider(repo pomomo.SessionRepo, l log.Logger) ParticipantsProvider {
	return &participantsProvider{
		cache: &participantsCache{
			store: make(map[pomomo.VoiceChannelID][]*models.SessionParticipant),
			locks: make(map[pomomo.VoiceChannelID]*sync.Mutex),
		},
		repo: repo,
		l:    l,
	}
}

type participantsCache struct {
	store map[pomomo.VoiceChannelID][]*models.SessionParticipant
	locks map[pomomo.VoiceChannelID]*sync.Mutex
	mu    sync.RWMutex
}

func (c *participantsCache) add(p *models.SessionParticipant) error {
	if existing := c.get(p.Record.VoiceCID, p.Record.UserID); existing != nil {
		return fmt.Errorf("cache already contains participant with userID %s", p.Record.UserID)
	}
	c.store[p.Record.VoiceCID] = append(c.store[p.Record.VoiceCID], p)
	return nil
}

func (c *participantsCache) remove(cid pomomo.VoiceChannelID, userID string) (models.SessionParticipant, error) {
	participants := c.store[cid]
	i := slices.IndexFunc(participants, func(p *models.SessionParticipant) bool {
		return p.Record.UserID == userID
	})

	if i == -1 {
		return models.SessionParticipant{}, fmt.Errorf("participant not found for cid %s, uid %s", cid, userID)
	}

	removed := participants[i]
	c.store[cid] = slices.Delete(participants, i, i+1)

	if len(c.store[cid]) == 0 {
		delete(c.store, cid)
	}
	return *removed, nil
}

func (c *participantsCache) get(cid pomomo.VoiceChannelID, uid string) *models.SessionParticipant {
	participants := c.store[cid]
	i := slices.IndexFunc(participants, func(p *models.SessionParticipant) bool {
		return p.Record.UserID == uid
	})

	if i == -1 {
		return nil
	}
	return participants[i]
}

func (pp *participantsProvider) AcquireVoiceChannelLock(cid pomomo.VoiceChannelID) func() {
	pp.cache.mu.Lock()
	defer pp.cache.mu.Unlock()

	// create if not exists
	if _, exists := pp.cache.locks[cid]; !exists {
		var lock sync.Mutex
		pp.cache.locks[cid] = &lock
	}

	l := pp.cache.locks[cid]
	l.Lock()
	return l.Unlock
}

func (pp *participantsProvider) Insert(ctx context.Context, r pomomo.SessionParticipantRecord) (models.SessionParticipant, error) {
	pp.cache.mu.Lock()
	defer pp.cache.mu.Unlock()

	existingRecord, err := pp.repo.InsertParticipant(ctx, r)
	if err != nil {
		return models.SessionParticipant{}, err
	}

	participant := models.SessionParticipant{
		ID:                existingRecord.ID,
		Record:            existingRecord.SessionParticipantRecord,
		StartedIntervalAt: existingRecord.CreatedAt,
	}

	if err := pp.cache.add(&participant); err != nil {
		return models.SessionParticipant{}, err
	}
	return participant, nil
}

func (pp *participantsProvider) RestoreCache(ctx context.Context) error {
	pp.cache.mu.Lock()
	defer pp.cache.mu.Unlock()

	records, err := pp.repo.GetAllParticipants(ctx)
	if err != nil {
		return err
	}

	for _, record := range records {
		// cache participant
		participant := &models.SessionParticipant{
			ID:                record.ID,
			Record:            record.SessionParticipantRecord,
			StartedIntervalAt: time.Now(),
		}
		pp.cache.store[record.VoiceCID] = append(pp.cache.store[record.VoiceCID], participant)
	}
	log.Info("restored participantsProvider cache", "cnt", len(records))

	return nil
}

func (pp *participantsProvider) Delete(ctx context.Context, id pomomo.SessionParticipantID) error {
	pp.cache.mu.Lock()
	defer pp.cache.mu.Unlock()

	existing, err := pp.repo.DeleteParticipant(ctx, id)
	if err != nil {
		return err
	}
	_, err = pp.cache.remove(existing.VoiceCID, existing.UserID)
	return err
}

func (pp *participantsProvider) UpdateVoiceState(ctx context.Context, uid string, cid pomomo.VoiceChannelID, vs pomomo.VoiceState) (*models.SessionParticipant, error) {
	pp.cache.mu.Lock()
	defer pp.cache.mu.Unlock()

	participant := pp.cache.get(cid, uid)
	if participant == nil {
		return nil, fmt.Errorf("participant not found for voice channel %s, user %s", cid, uid)
	}

	participant.Record.IsMuted = vs.Mute
	participant.Record.IsDeafened = vs.Deaf

	_, err := pp.repo.UpdateParticipant(ctx, participant.ID, participant.Record)
	if err != nil {
		return nil, err
	}

	return participant, nil
}

func (pp *participantsProvider) Get(userID string, cid pomomo.VoiceChannelID) *models.SessionParticipant {
	pp.cache.mu.Lock()
	defer pp.cache.mu.Unlock()

	for _, p := range pp.cache.store[cid] {
		if p.Record.UserID == userID {
			return p
		}
	}
	return nil
}

func (pp *participantsProvider) GetAll(cid pomomo.VoiceChannelID) []*models.SessionParticipant {
	pp.cache.mu.Lock()
	defer pp.cache.mu.Unlock()

	return pp.cache.store[cid]
}

func (pp *participantsProvider) GetVoiceChannelIDs() []pomomo.VoiceChannelID {
	pp.cache.mu.Lock()
	defer pp.cache.mu.Unlock()

	var res []pomomo.VoiceChannelID
	for cid := range pp.cache.store {
		res = append(res, cid)
	}
	return res
}
