package main

import (
	"context"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/bwmarrin/discordgo"
	"github.com/charmbracelet/log"
)

const (
	defaultErrorMsg = "Looks like something went wrong. Try again in a bit or reach out to support."
)

func RemoveParticipantOnVoiceChannelLeave(ctx context.Context, vsUpdater VoiceStateSvc, pp ParticipantsProvider, s *discordgo.Session, u *discordgo.VoiceStateUpdate) {
	if u.BeforeUpdate == nil {
		// don't need to handle joins since participation is removed on leave
		return
	}
	if u.ChannelID == u.BeforeUpdate.ChannelID {
		return
	}
	cid := pomomo.VoiceChannelID(u.ChannelID)
	unlock := pp.AcquireVoiceChannelLock(cid)
	defer unlock()

	if p := pp.Get(u.UserID, cid); p != nil {
		if err := pp.Delete(ctx, p.ID); err != nil {
			log.Error("failed participant delete on voice channel leave", "err", err, "gid", u.GuildID, "uid", u.UserID)
		}
		if err := vsUpdater.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, p.Record.IsMuted, p.Record.IsDeafened); err != nil {
			log.Error("failed voice state restore on voice channel leave", "err", err, "gid", u.GuildID, "uid", u.UserID)
		}
	}
}

func StartSession(ctx context.Context, sessionManager SessionManager, dm DiscordMessenger, pp ParticipantsProvider, s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := m.ApplicationCommandData()
	if data.Name != pomomo.StartCommand.Name {
		return
	}

	// Parse command options with defaults
	settings := models.SessionSettings{
		Pomodoro:   20 * time.Minute,
		ShortBreak: 5 * time.Minute,
		LongBreak:  15 * time.Minute,
		Intervals:  4,
	}
	for _, opt := range data.Options {
		val, ok := opt.Value.(float64)
		if !ok {
			continue
		}
		intVal := int(val)
		switch opt.Name {
		case pomomo.PomodoroOption:
			settings.Pomodoro = time.Duration(intVal) * time.Minute
		case pomomo.ShortBreakOption:
			settings.ShortBreak = time.Duration(intVal) * time.Minute
		case pomomo.LongBreakOption:
			settings.LongBreak = time.Duration(intVal) * time.Minute
		case pomomo.IntervalsOption:
			settings.Intervals = intVal
		}
	}

	// TODO multisession
	if sessionManager.GuildSessionCnt(m.GuildID) > 0 {
		if _, err := dm.Respond(m.Interaction, false, TextDisplay("Pomomo is limited to one session per server for now.")); err != nil {
			log.Error(err)
		}
		return
	}

	if sessionManager.HasSession(m.ChannelID) {
		if _, err := dm.Respond(m.Interaction, false, TextDisplay("This channel already has an active session.")); err != nil {
			log.Error(err)
		}
		return
	}

	// get voice channel
	vs, err := s.State.VoiceState(m.GuildID, m.Member.User.ID)
	if err != nil {
		log.Debug("failed to get voice state", "userID", m.Member.User.ID, "guildID", m.GuildID, "err", err)
		_, err = dm.Respond(m.Interaction, false, TextDisplay("Pomomo couldn't find your voice channel. Please join a voice channel with permissions and try again."))
		if err != nil {
			log.Error(err)
		}
		return
	}
	if sessionManager.HasVoiceSession(vs.ChannelID) {
		_, err = dm.Respond(m.Interaction, false, TextDisplay("Your voice channel already has an active session. Please join another voice channel and try again."))
		if err != nil {
			log.Error(err)
		}
		return
	}

	session := models.NewSession("", m.GuildID, m.ChannelID, vs.ChannelID, "", settings)
	session.GoNextInterval(false) // initialize fields for display - "real" session is created by sessionManager
	msg, err := dm.Respond(m.Interaction, true, SessionMessageComponents(session)...)
	if err != nil {
		log.Error(err)
		return
	}

	session, err = sessionManager.StartSession(ctx, startSessionRequest{
		guildID:   m.GuildID,
		textCID:   m.ChannelID,
		voiceCID:  vs.ChannelID,
		messageID: msg.ID,
		settings:  settings,
		user: struct {
			id   string
			mute bool
			deaf bool
		}{
			id:   m.Member.User.ID,
			mute: m.Member.Mute,
			deaf: m.Member.Deaf,
		},
	})
	if err != nil {
		log.Error("failed to start session", "err", err)
		if _, err := dm.EditResponse(m.Interaction, TextDisplay("Failed to start session.")); err != nil {
			log.Error(err)
		}
		return
	}
	log.Debug("started session", "id", session.ID)

	if err := s.ChannelMessagePin(m.ChannelID, msg.ID); err != nil {
		log.Error("failed to pin message", "err", err)
	}
	// TODO SessionManager goroutine to update stats with lesser frequency
}

func SkipInterval(ctx context.Context, sessionManager SessionManager, dm DiscordMessenger, s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := m.MessageComponentData()
	id, err := FromCustomID(data.CustomID)
	if err != nil {
		return
	}
	if id.Type != "skip" {
		return
	}

	followup, err := dm.DeferMessageUpdate(m.Interaction)
	if err != nil {
		log.Error(err)
		return
	}

	session, err := sessionManager.SkipInterval(ctx, id.TextCID)
	if err != nil {
		log.Error("failed to skip interval", "err", err)
		components := append(SessionMessageComponents(session), TextDisplay(defaultErrorMsg))
		if _, err := followup(components...); err != nil {
			log.Error(err)
		}
		return
	}
	log.Debug("skipped interval", "new", session.Record.CurrentInterval)

	_, err = followup(SessionMessageComponents(session)...)
	if err != nil {
		log.Error(err)
		return
	}
}

func EndSession(ctx context.Context, sessionManager SessionManager, s *discordgo.Session, m *discordgo.InteractionCreate) {
	if m.Type != discordgo.InteractionMessageComponent {
		return
	}

	data := m.MessageComponentData()
	id, err := FromCustomID(data.CustomID)
	if err != nil {
		return
	}
	if id.Type != "end" {
		return
	}

	if err := s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		log.Error("failed EndSession ack", "err", err)
		return
	}

	session, err := sessionManager.EndSession(ctx, id.TextCID)
	if err != nil {
		log.Error("failed EndSession", "sid", session.ID, "gid", session.Record.GuildID, "err", err)
		return
	}
	log.Debug("ended session", "id", session.ID)
}

func JoinSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
	panic("not implemented")
}
