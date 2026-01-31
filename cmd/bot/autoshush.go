package main

import (
	"context"
	"sync"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/charmbracelet/log"
)

type Autoshusher interface {
	Autoshush(ctx context.Context, participants []models.Participant, before, curr models.Session)
}

var _ Autoshusher = (*autoshusher)(nil)

type autoshusher struct {
	loadFn loadOpusAudio
	sendFn sendOpusAudio
	pm     ParticipantsManager
	vs     VoiceStateAdapter
}

func (a *autoshusher) Autoshush(ctx context.Context, participants []models.Participant, before, curr models.Session) {
	if before.Record.CurrentInterval == curr.Record.CurrentInterval {
		return
	}
	var wg sync.WaitGroup
	skipped := curr.Stats.Skips > before.Stats.Skips // don't play interval alert if interval was skipped
	if curr.Record.CurrentInterval != pomomo.PomodoroInterval {
		// unshush before playing
		for _, p := range participants {
			wg.Go(func() {
				if err := restoreVoiceState(ctx, a.vs, p); err != nil {
					log.Error(err)
				}
			})
		}
		wg.Wait()
		if !skipped {
			if err := playIntervalAlert(ctx, curr, a.loadFn, a.sendFn); err != nil {
				log.Error("failed to play interval alert", "guildID", curr.Record.GuildID, "channelID", curr.Record.VoiceCID, "err", err)
			}
		}
	} else {
		// shush after playing
		if !skipped {
			if err := playIntervalAlert(ctx, curr, a.loadFn, a.sendFn); err != nil {
				log.Error("failed to play interval alert", "guildID", curr.Record.GuildID, "channelID", curr.Record.VoiceCID, "err", err)
			}
		}
		// update voice state in case it's been changed during a break
		var toUpdate []models.Participant
		var mu sync.Mutex
		for _, p := range participants {
			wg.Go(func() {
				currVs, err := getVoiceState(ctx, a.vs, p)
				if err != nil {
					log.Error("failed GetVoiceState", "err", err, "sid", p.Record.SessionID, "uid", p.Record.UserID)
					return
				}
				if currVs.Mute != p.Record.IsMuted || currVs.Deaf != p.Record.IsDeafened {
					updated, err := a.pm.UpdateVoiceState(ctx, p.Record.UserID, p.Record.VoiceCID, currVs)
					if err != nil {
						log.Error("failed UpdateVoiceState in sync", "err", err, "sid", p.Record.SessionID, "uid", p.Record.UserID)
						return
					}
					mu.Lock()
					defer mu.Unlock()
					toUpdate = append(toUpdate, updated)
				} else {
					mu.Lock()
					defer mu.Unlock()
					toUpdate = append(toUpdate, p)
				}
			})
		}
		wg.Wait()
		go func() {
			for _, p := range toUpdate {
				if err := updateVoiceState(ctx, a.vs, !curr.Settings.NoMute, !curr.Settings.NoDeafen, p); err != nil {
					log.Error(err)
				}
			}
		}()
	}
}
