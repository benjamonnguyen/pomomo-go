// Package discordgo provides Discord API adapters using package github.com/bwmarrin/discordgo
package discordgo

import (
	"context"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

type discordgoAdapter struct {
	cl *discordgo.Session
	l  log.Logger
}

func NewDiscordAdapter(cl *discordgo.Session) *discordgoAdapter {
	return &discordgoAdapter{
		cl: cl,
	}
}

func (w *discordgoAdapter) UpdateVoiceState(gid, uid string, mute, deaf bool) error {
	_, err := w.cl.GuildMemberEdit(gid, uid, &discordgo.GuildMemberParams{
		Mute: &mute,
		Deaf: &deaf,
	})
	return err
}

func (w *discordgoAdapter) SendOpusAudio(ctx context.Context, packets [][]byte, gID string, cID pomomo.VoiceChannelID) error {
	if packets == nil {
		return nil
	}
	conn, err := w.cl.ChannelVoiceJoin(gID, string(cID), false, true)
	if err != nil {
		return err
	}
	if err := conn.Speaking(true); err != nil {
		return err
	}
	for _, p := range packets {
		if err := ctx.Err(); err != nil {
			_ = conn.Speaking(false)
			return err
		}
		conn.OpusSend <- p
	}
	return conn.Speaking(false)
}

func (w *discordgoAdapter) GetVoiceState(gid, uid string) (pomomo.VoiceState, error) {
	vs, err := w.cl.State.VoiceState(gid, uid)
	if err != nil {
		return pomomo.VoiceState{}, err
	}
	return pomomo.VoiceState{
		Mute: vs.Mute,
		Deaf: vs.Deaf,
	}, nil
}
