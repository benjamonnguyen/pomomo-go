package main

import (
	"sync"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

type Autoshusher interface {
	// Shush sets voice state for session participants in target text channel and returns participants' user IDs.
	// Shush preserves existing mute or deafen voice state.
	Shush(bool, pomomo.TextChannelID) ([]string, error)

	// AddParticipant stores user's current voice state before adding to session
	// @implement set StartedIntervalAt
	AddParticipant(uid string, cid pomomo.TextChannelID) error

	// RemoveParticipant restores user's original voice state before removing from session
	RemoveParticipant(uid string, cid pomomo.TextChannelID) error

	// Restore adds active participants from DB
	// @implement GetAllParticipants and call AddParticipant() to repopulate cache
	Restore() error

	// Close restores voice state of active participants
	Close() error
}

type autoshusher struct {
	cl    *discordgo.Session
	cache *participantsCache
	repo  pomomo.SessionRepo
	parentCtx context.Context
}

// @implement add sessionRepo
func NewAutoshusher(cl *discordgo.Session) Autoshusher {
	if !cl.State.TrackVoice {
		panic("expected cl.State.TrackVoice == true")
	}
	cl.AddHandler(handleVoiceChannelLeave) // TODO maybe extract out to main; maybe don't need client in here at all
	return autoshusher{
		cl: cl,
	}
}

func (o *autoshusher) handleVoiceChannelLeave(s *discordgo.Session, u *discordgo.VoiceStateUpdate) {
	if u.BeforeUpdate == nil {
	// don't need to handle the case on join since participation is removed on leave
		return
	}
	if u.ChannelID != "" {
		// current channelID empty should mean 
		return
	}
	p := o.cache.Get(u.ChannelID, u.UserID)
	if p == (pomomo.SessionParticipantRecord) {
		return
	}
	if 
	if err := o.repo.DeleteParticipant(o.parentCtx, p.UserID); err != nil {
		log.Error("failed deleting participant on leave", "err", err, "voiceCID", u.ChannelID, "uid", u.UserID)
	}
	})
// @implement autoshusher
// add cache similar to sessionManager

type participantsCache struct {
	store sync.Map // map[pomomo.VoiceChannelID][]pomomo.SessionParticipantRecord
}

// @implement func (c *participantsCache) Add(SessionParticipant) error
// @implement func (c *participantsCache) Remove(userID string) (SessionParticipant, error)
// @implement func (c *participantsCache) Get(cid, userID string) SessionParticipant
// @implement func (c *participantsCache) GetAll(cid string) []SessionParticipant
