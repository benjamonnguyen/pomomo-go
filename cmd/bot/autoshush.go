package main

import (
	"sync"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/bwmarrin/discordgo"
)

type Autoshusher interface {
	// Shush sets voice state for session participants in target text channel and returns participants' user IDs.
	// Shush preserves existing mute or deafen voice state.
	Shush(bool, pomomo.TextChannelID) ([]string, error)

	// AddParticipant stores user's current voice state before adding to session
	AddParticipant(uid string, cid pomomo.TextChannelID) error

	// RemoveParticipant restores user's original voice state before removing from session
	RemoveParticipant(uid string, cid pomomo.TextChannelID) error

	// Restore adds active participants from DB
	Restore() error

	// Close restores voice state of active participants
	Close() error
}

type autoshusher struct {
	cl    *discordgo.Session
	cache *participantsCache
	repo  pomomo.SessionRepo
}

// @implement add sessionRepo
func NewAutoshusher(cl *discordgo.Session) Autoshusher {
	if !cl.State.TrackVoice {
		panic("expected cl.State.TrackVoice == true")
	}
	cl.AddHandler(handleVoiceJoinsAndLeaves) // TODO maybe extract out to main; maybe don't need client in here at all
	return autoshusher{
		cl: cl,
	}
}

func (o *autoshusher) handleVoiceJoinsAndLeaves(s *discordgo.Session, u *discordgo.VoiceStateUpdate) {
	if u.BeforeUpdate == nil {
		// @implement handle add participant case
	}
	p := o.cache.Get(u.ChannelID, u.UserID)
	if p == (pomomo.SessionParticipantRecord) {
		return
	}
		if u.BeforeUpdate == nil {
			// @implement AddParticipant
		} else if u. {
			// @implement RemoveParticipant
		}

	})
// @implement autoshusher
// add cache similar to sessionManager

type participantsCache struct {
	store sync.Map // map[pomomo.VoiceChannelID][]pomomo.SessionParticipantRecord
}

// @implement func (c *participantsCache) Add(pomomo.SessionParticipantRecord) error
// @implement func (c *participantsCache) Remove(userID string) (pomomo.SessionParticipantRecord, error)
// @implement func (c *participantsCache) Get(cid, userID string) pomomo.SessionParticipantRecord
// @implement func (c *participantsCache) GetAll(cid string) []pomomo.SessionParticipantRecord
