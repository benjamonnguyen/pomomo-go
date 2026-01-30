package main

import (
	"context"
	"sync"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/charmbracelet/log"
)

type VoiceStateManager interface {
	// AutoShush sets voice state for session participants in target voice channel.
	// AutoShush preserves existing mute or deafen voice state.
	AutoShush(context.Context, models.Session)

	// UnshushParticipants restores voice state of session participants
	UnshushParticipants(pomomo.VoiceChannelID)

	// Close restores voice state of participants across all sessions
	Close()
}

type vsSvc interface {
	UpdateVoiceState(gid, uid string, mute, deaf bool) error
	GetVoiceState(gid, uid string) (pomomo.VoiceState, error)
}

type voiceStateMgr struct {
	participantsProvider ParticipantsProvider
	vs                   vsSvc
	l                    log.Logger
}

func NewVoiceStateManager(
	p ParticipantsProvider,
	vs vsSvc,
	l log.Logger,
) VoiceStateManager {
	return &voiceStateMgr{
		participantsProvider: p,
		vs:                   vs,
		l:                    l,
	}
}

func (o *voiceStateMgr) AutoShush(ctx context.Context, s models.Session) {
	unlock := o.participantsProvider.AcquireVoiceChannelLock(s.Record.VoiceCID)
	defer unlock()

	participants := o.participantsProvider.GetAll(s.Record.VoiceCID)
	if len(participants) == 0 {
		return
	}

	var wg sync.WaitGroup
	if s.Record.CurrentInterval == pomomo.PomodoroInterval {
		for _, p := range participants {
			wg.Go(func() {
				// update voice state in case it's been changed during a break
				currVs, err := o.vs.GetVoiceState(p.Record.GuildID, p.Record.UserID)
				if err != nil {
					o.l.Error("failed GetVoiceState", "err", err, "sid", p.Record.SessionID, "uid", p.Record.UserID)
				} else {
					if currVs.Mute != p.Record.IsMuted || currVs.Deaf != p.Record.IsDeafened {
						updated, err := o.participantsProvider.UpdateVoiceState(ctx, p.Record.UserID, p.Record.VoiceCID, currVs)
						if err != nil {
							o.l.Error("failed UpdateVoiceState in sync", "err", err, "sid", p.Record.SessionID, "uid", p.Record.UserID)
						}
						p = updated
					}
				}

				// shush
				mute := p.Record.IsMuted || !s.Settings.NoMute
				deafen := p.Record.IsDeafened || !s.Settings.NoDeafen
				if err := o.vs.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, mute, deafen); err != nil {
					o.l.Error("failed UpdateVoiceState", "gid", p.Record.GuildID, "uid", p.Record.UserID, "sid", p.Record.SessionID, "err", err)
				}
			})
		}
	} else {
		// restore voice state
		for _, p := range participants {
			wg.Go(func() {
				if err := o.vs.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, p.Record.IsMuted, p.Record.IsDeafened); err != nil {
					o.l.Error("failed UpdateVoiceState", p.Record.GuildID, "uid", p.Record.UserID, "sid", p.Record.SessionID, "err", err)
				}
			})
		}
	}
	wg.Wait()
}

func (o *voiceStateMgr) UnshushParticipants(cid pomomo.VoiceChannelID) {
	unlock := o.participantsProvider.AcquireVoiceChannelLock(cid)
	defer unlock()
	var wg sync.WaitGroup
	for _, p := range o.participantsProvider.GetAll(cid) {
		wg.Go(func() {
			if err := o.vs.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, p.Record.IsMuted, p.Record.IsDeafened); err != nil {
				o.l.Error("failed to restore original voice state", "err", err, "sid", p.Record.SessionID, "uid", p.Record.UserID)
			}
		})
	}
	wg.Wait()
}

func (o *voiceStateMgr) Close() {
	var wg sync.WaitGroup
	cids := o.participantsProvider.GetVoiceChannelIDs()
	for _, cid := range cids {
		wg.Go(func() {
			o.UnshushParticipants(cid)
		})
	}
	wg.Wait()
}
