package main

import (
	"context"
	"sync"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/charmbracelet/log"
)

type VoiceStateUpdater interface {
	// AutoShush sets voice state for session participants in target voice channel.
	// AutoShush preserves existing mute or deafen voice state.
	AutoShush(context.Context, models.Session)

	// RestoreVoiceState restores voice state of session participants
	RestoreVoiceState(pomomo.VoiceChannelID)

	// Close restores voice state of participants across all sessions
	Close()
}

type vsSvc interface {
	UpdateVoiceState(gid, uid string, mute, deaf bool) error
	GetVoiceState(gid, uid string) (pomomo.VoiceState, error)
}

type voiceStateMgr struct {
	pm ParticipantsManager
	vs vsSvc
	l  log.Logger
}

func NewVoiceStateManager(
	pm ParticipantsManager,
	vs vsSvc,
	l log.Logger,
) VoiceStateUpdater {
	return &voiceStateMgr{
		pm: pm,
		vs: vs,
		l:  l,
	}
}

func (o *voiceStateMgr) AutoShush(ctx context.Context, s models.Session) {
	unlock := o.pm.AcquireVoiceChannelLock(s.Record.VoiceCID)
	defer unlock()

	participants := o.pm.GetAll(s.Record.VoiceCID)
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
						updated, err := o.pm.UpdateVoiceState(ctx, p.Record.UserID, p.Record.VoiceCID, currVs)
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

func (o *voiceStateMgr) RestoreVoiceState(cid pomomo.VoiceChannelID) {
	unlock := o.pm.AcquireVoiceChannelLock(cid)
	defer unlock()
	var wg sync.WaitGroup
	for _, p := range o.pm.GetAll(cid) {
		wg.Go(func() {
			if err := o.vs.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, p.Record.IsMuted, p.Record.IsDeafened); err != nil {
				o.l.Error("failed to restore original voice state", "err", err, "sid", p.Record.SessionID, "uid", p.Record.UserID)
			}
		})
	}
	wg.Wait()
}

// func (o *voiceStateMgr)

func (o *voiceStateMgr) Close() {
	var wg sync.WaitGroup
	cids := o.pm.GetVoiceChannelIDs()
	for _, cid := range cids {
		wg.Go(func() {
			o.RestoreVoiceState(cid)
		})
	}
	wg.Wait()
}
