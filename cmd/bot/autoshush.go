package main

import (
	"context"
	"sync"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/charmbracelet/log"
)

type Autoshusher interface {
	// Shush sets voice state for session participants in target voice channel.
	// Shush preserves existing mute or deafen voice state.
	Shush(context.Context, models.Session)

	// Close restores voice state of participants
	Close()
}

type VoiceStateSvc interface {
	UpdateVoiceState(gid, uid string, mute, deaf bool) error
	GetVoiceState(gid, uid string) (pomomo.VoiceState, error)
}

type autoshusher struct {
	participantsProvider ParticipantsProvider
	vs                   VoiceStateSvc
	l                    log.Logger
}

func NewAutoshusher(
	p ParticipantsProvider,
	vs VoiceStateSvc,
	l log.Logger,
) Autoshusher {
	return &autoshusher{
		participantsProvider: p,
		vs:                   vs,
		l:                    l,
	}
}

func (o *autoshusher) Shush(ctx context.Context, s models.Session) {
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
				if err := o.vs.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, true, p.Record.IsDeafened || !s.Record.NoDeafen); err != nil {
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

func (o *autoshusher) Close() {
	var wg sync.WaitGroup
	cids := o.participantsProvider.GetVoiceChannelIDs()
	for _, cid := range cids {
		wg.Go(func() {
			// permanently acquire locks
			_ = o.participantsProvider.AcquireVoiceChannelLock(cid)
			participants := o.participantsProvider.GetAll(cid)
			for _, p := range participants {
				if err := o.vs.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, p.Record.IsMuted, p.Record.IsDeafened); err != nil {
					o.l.Error("failed to restore original voice state", "err", err, "gid", p.Record.GuildID, "sid", p.Record.SessionID, "uid", p.Record.UserID)
				}
			}
		})
	}
	wg.Wait()
}
