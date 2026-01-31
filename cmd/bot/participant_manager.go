package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/benjamonnguyen/pomomo-go/sqlite"
	"github.com/charmbracelet/log"
)

type ParticipantsRepo interface {
	InsertParticipant(context.Context, pomomo.ParticipantRecord) (pomomo.ExistingParticipantRecord, error)
	UpdateParticipant(context.Context, pomomo.ParticipantID, pomomo.ParticipantRecord) (pomomo.ExistingParticipantRecord, error)
	DeleteParticipant(context.Context, pomomo.ParticipantID) (pomomo.ExistingParticipantRecord, error)
	GetAllParticipants(context.Context) ([]pomomo.ExistingParticipantRecord, error)
	GetParticipantByUserID(context.Context, string) (pomomo.ExistingParticipantRecord, error)
}

// ParticipantsManager adds caching and event hooks on top PartcipantsRepo
type ParticipantsManager interface {
	// AcquireVoiceChannelLock returns unlockFn that caller is responsible for calling
	AcquireVoiceChannelLock(pomomo.VoiceChannelID) func()

	// CRUD operations

	Insert(context.Context, pomomo.ParticipantRecord) (models.Participant, error)
	Delete(context.Context, pomomo.ParticipantID) error
	UpdateVoiceState(context.Context, string, pomomo.VoiceChannelID, pomomo.VoiceState) (*models.Participant, error)
	Get(string, pomomo.VoiceChannelID) *models.Participant
	GetAll(pomomo.VoiceChannelID) []*models.Participant
	GetVoiceChannelIDs() []pomomo.VoiceChannelID
	GetParticipantID(context.Context, string) (pomomo.ParticipantID, error)

	// lifecycle hooks

	// AfterUpdate allows a function to be set that is called after any update.
	// After insert, before arg is empty.
	// After delete, curr arg is empty.
	AfterUpdate(func(ctx context.Context, before, curr optional[models.Participant]))

	// RestoreCache fetches active participants from repo; should be called after init
	RestoreCache(context.Context) error
}

type participantsMgr struct {
	cache       *participantsCache
	repo        ParticipantsRepo
	l           log.Logger
	afterUpdate func(ctx context.Context, before, curr optional[models.Participant])
}

func NewParticipantManager(repo ParticipantsRepo, l log.Logger) ParticipantsManager {
	return &participantsMgr{
		cache: &participantsCache{
			store: make(map[pomomo.VoiceChannelID][]*models.Participant),
			locks: make(map[pomomo.VoiceChannelID]*sync.Mutex),
		},
		repo: repo,
		l:    l,
	}
}

type participantsCache struct {
	store map[pomomo.VoiceChannelID][]*models.Participant
	locks map[pomomo.VoiceChannelID]*sync.Mutex
	mu    sync.RWMutex
}

func (c *participantsCache) add(p *models.Participant) error {
	log.Debug("participantsCache add", "pid", p.ID)
	if existing := c.get(p.Record.VoiceCID, p.Record.UserID); existing != nil {
		return fmt.Errorf("cache already contains participant with userID %s", p.Record.UserID)
	}
	c.store[p.Record.VoiceCID] = append(c.store[p.Record.VoiceCID], p)
	return nil
}

func (c *participantsCache) remove(cid pomomo.VoiceChannelID, userID string) (models.Participant, error) {
	participants := c.store[cid]
	i := slices.IndexFunc(participants, func(p *models.Participant) bool {
		return p.Record.UserID == userID
	})

	if i == -1 {
		return models.Participant{}, fmt.Errorf("participant not found for cid %s, uid %s", cid, userID)
	}

	removed := participants[i]
	c.store[cid] = slices.Delete(participants, i, i+1)

	if len(c.store[cid]) == 0 {
		delete(c.store, cid)
	}
	return *removed, nil
}

func (c *participantsCache) get(cid pomomo.VoiceChannelID, uid string) *models.Participant {
	participants := c.store[cid]
	i := slices.IndexFunc(participants, func(p *models.Participant) bool {
		return p.Record.UserID == uid
	})

	if i == -1 {
		return nil
	}
	return participants[i]
}

func (pm *participantsMgr) AfterUpdate(fn func(ctx context.Context, before, curr optional[models.Participant])) {
	pm.afterUpdate = fn
}

func (pm *participantsMgr) AcquireVoiceChannelLock(cid pomomo.VoiceChannelID) func() {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	// create if not exists
	if _, exists := pm.cache.locks[cid]; !exists {
		var lock sync.Mutex
		pm.cache.locks[cid] = &lock
	}

	l := pm.cache.locks[cid]
	l.Lock()
	return l.Unlock
}

func (pm *participantsMgr) Insert(ctx context.Context, r pomomo.ParticipantRecord) (models.Participant, error) {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	existingRecord, err := pm.repo.InsertParticipant(ctx, r)
	if err != nil {
		return models.Participant{}, err
	}

	participant := models.Participant{
		ID:                existingRecord.ID,
		Record:            existingRecord.ParticipantRecord,
		StartedIntervalAt: existingRecord.CreatedAt,
	}

	if err := pm.cache.add(&participant); err != nil {
		return models.Participant{}, err
	}
	return participant, nil
}

func (pm *participantsMgr) RestoreCache(ctx context.Context) error {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	records, err := pm.repo.GetAllParticipants(ctx)
	if err != nil {
		return err
	}

	for _, record := range records {
		// cache participant
		participant := &models.Participant{
			ID:                record.ID,
			Record:            record.ParticipantRecord,
			StartedIntervalAt: time.Now(),
		}
		pm.cache.store[record.VoiceCID] = append(pm.cache.store[record.VoiceCID], participant)
	}
	log.Info("restored participantMgr cache", "cnt", len(records))

	return nil
}

func (pm *participantsMgr) Delete(ctx context.Context, id pomomo.ParticipantID) error {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	existing, err := pm.repo.DeleteParticipant(ctx, id)
	if err != nil {
		return err
	}
	_, err = pm.cache.remove(existing.VoiceCID, existing.UserID)
	return err
}

func (pm *participantsMgr) UpdateVoiceState(ctx context.Context, uid string, cid pomomo.VoiceChannelID, vs pomomo.VoiceState) (*models.Participant, error) {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	participant := pm.cache.get(cid, uid)
	if participant == nil {
		return nil, fmt.Errorf("participant not found for voice channel %s, user %s", cid, uid)
	}

	participant.Record.IsMuted = vs.Mute
	participant.Record.IsDeafened = vs.Deaf

	_, err := pm.repo.UpdateParticipant(ctx, participant.ID, participant.Record)
	if err != nil {
		return nil, err
	}

	return participant, nil
}

func (pm *participantsMgr) Get(userID string, cid pomomo.VoiceChannelID) *models.Participant {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	for _, p := range pm.cache.store[cid] {
		if p.Record.UserID == userID {
			return p
		}
	}
	return nil
}

func (pm *participantsMgr) GetAll(cid pomomo.VoiceChannelID) []*models.Participant {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	return pm.cache.store[cid]
}

func (pm *participantsMgr) GetVoiceChannelIDs() []pomomo.VoiceChannelID {
	pm.cache.mu.Lock()
	defer pm.cache.mu.Unlock()

	var res []pomomo.VoiceChannelID
	for cid := range pm.cache.store {
		res = append(res, cid)
	}
	return res
}

func (pm *participantsMgr) GetParticipantID(ctx context.Context, userID string) (pomomo.ParticipantID, error) {
	p, err := pm.repo.GetParticipantByUserID(ctx, userID)
	if err != nil {
		if err == sqlite.ErrNotFound {
			return "", nil
		}
		return "", err
	}
	return p.ID, nil
}
