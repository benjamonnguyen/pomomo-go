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

func RemoveParticipantOnVoiceChannelLeave(ctx context.Context, vsSvc vsSvc, pp ParticipantsManager, s *discordgo.Session, u *discordgo.VoiceStateUpdate) {
	if u.BeforeUpdate == nil {
		// don't need to handle joins since participation is removed on leave
		return
	}
	if u.ChannelID == u.BeforeUpdate.ChannelID {
		return
	}
	cid := pomomo.VoiceChannelID(u.BeforeUpdate.ChannelID)
	unlock := pp.AcquireVoiceChannelLock(cid)
	defer unlock()

	if p := pp.Get(u.UserID, cid); p != nil {
		if err := pp.Delete(ctx, p.ID); err != nil {
			log.Error("failed participant delete on voice channel leave", "err", err, "gid", u.GuildID, "uid", u.UserID)
		}
		if err := vsSvc.UpdateVoiceState(p.Record.GuildID, p.Record.UserID, p.Record.IsMuted, p.Record.IsDeafened); err != nil {
			log.Error("failed voice state restore on voice channel leave", "err", err, "gid", u.GuildID, "uid", u.UserID)
		}
		log.Debug("removed participant on voice channel leave", "uid", u.UserID, "sid", p.Record.SessionID)
	}
}

func StartSession(ctx context.Context, sessionManager SessionManager, dm DiscordMessenger, pp ParticipantsManager, s *discordgo.Session, m *discordgo.InteractionCreate) bool {
	if m.Type != discordgo.InteractionApplicationCommand {
		return false
	}

	data := m.ApplicationCommandData()
	if data.Name != pomomo.StartCommand.Name {
		return false
	}

	// Parse command options with defaults
	settings := pomomo.SessionSettingsRecord{
		Pomodoro:   20 * time.Minute,
		ShortBreak: 5 * time.Minute,
		LongBreak:  15 * time.Minute,
		Intervals:  4,
		NoMute:     false,
		NoDeafen:   false,
	}
	for _, opt := range data.Options {
		switch opt.Name {
		case pomomo.PomodoroOption, pomomo.ShortBreakOption, pomomo.LongBreakOption, pomomo.IntervalsOption:
			if val, ok := opt.Value.(float64); ok {
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
		case pomomo.NoMuteOption:
			if val, ok := opt.Value.(bool); ok {
				settings.NoMute = val
			}
		case pomomo.NoDeafenOption:
			if val, ok := opt.Value.(bool); ok {
				settings.NoDeafen = val
			}
		}
	}

	// TODO multisession
	if sessionManager.GuildSessionCnt(m.GuildID) > 0 {
		if _, err := dm.Respond(m.Interaction, false, TextDisplay("Pomomo is limited to one session per server for now.")); err != nil {
			log.Error(err)
		}
		return true
	}

	if sessionManager.HasSession(m.ChannelID) {
		if _, err := dm.Respond(m.Interaction, false, TextDisplay("This channel already has an active session.")); err != nil {
			log.Error(err)
		}
		return true
	}

	// get voice channel
	vs, err := s.State.VoiceState(m.GuildID, m.Member.User.ID)
	if err != nil {
		log.Debug("failed to get voice state", "userID", m.Member.User.ID, "guildID", m.GuildID, "err", err)
		_, err = dm.Respond(m.Interaction, false, TextDisplay("Pomomo couldn't find your voice channel. Please join a voice channel with permissions and try again."))
		if err != nil {
			log.Error(err)
		}
		return true
	}
	if sessionManager.HasVoiceSession(vs.ChannelID) {
		_, err = dm.Respond(m.Interaction, false, TextDisplay("Your voice channel already has an active session. Please join another voice channel and try again."))
		if err != nil {
			log.Error(err)
		}
		return true
	}

	session := models.NewSession("", m.GuildID, m.ChannelID, vs.ChannelID, "", settings)
	session.GoNextInterval(false) // initialize fields for display - "real" session is created by sessionManager
	msg, err := dm.Respond(m.Interaction, true, SessionMessageComponents(session)...)
	if err != nil {
		log.Error(err)
		return true
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
		return true
	}
	log.Info("started session", "id", session.ID)

	if err := s.ChannelMessagePin(m.ChannelID, msg.ID); err != nil {
		log.Error("failed to pin message", "err", err)
	}
	// TODO SessionManager goroutine to update stats with lesser frequency

	return true
}

func SkipInterval(ctx context.Context, sessionManager SessionManager, dm DiscordMessenger, s *discordgo.Session, m *discordgo.InteractionCreate) bool {
	if m.Type != discordgo.InteractionMessageComponent {
		return false
	}

	data := m.MessageComponentData()
	id, err := FromCustomID(data.CustomID)
	if err != nil {
		return false
	}
	if id.Type != "skip" {
		return false
	}

	followup, err := dm.DeferMessageUpdate(m.Interaction)
	if err != nil {
		log.Error(err)
		return true
	}

	session, err := sessionManager.SkipInterval(ctx, id.TextCID)
	if err != nil {
		log.Error("failed to skip interval", "err", err)
		components := append(SessionMessageComponents(session), TextDisplay(defaultErrorMsg))
		if _, err := followup(components...); err != nil {
			log.Error(err)
		}
		return true
	}
	log.Info("skipped interval", "new", session.Record.CurrentInterval)

	_, err = followup(SessionMessageComponents(session)...)
	if err != nil {
		log.Error(err)
		return true
	}

	return true
}

func EndSession(ctx context.Context, sessionManager SessionManager, s *discordgo.Session, m *discordgo.InteractionCreate) bool {
	if m.Type != discordgo.InteractionMessageComponent {
		return false
	}

	data := m.MessageComponentData()
	id, err := FromCustomID(data.CustomID)
	if err != nil {
		return false
	}
	if id.Type != "end" {
		return false
	}

	if err := s.InteractionRespond(m.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		log.Error("failed EndSession ack", "err", err)
		return true
	}

	session, err := sessionManager.EndSession(ctx, id.TextCID)
	if err != nil {
		log.Error("failed EndSession", "sid", session.ID, "gid", session.Record.GuildID, "err", err)
		return true
	}
	log.Info("ended session", "id", session.ID)
	return true
}

func JoinSession(ctx context.Context, sessionManager SessionManager, vsMgr VoiceStateUpdater, pp ParticipantsManager, dm DiscordMessenger, s *discordgo.Session, m *discordgo.InteractionCreate) bool {
	if m.Type != discordgo.InteractionMessageComponent {
		return false
	}

	data := m.MessageComponentData()
	id, err := FromCustomID(data.CustomID)
	if err != nil {
		return false
	}
	if id.Type != "join" {
		return false
	}

	followup, err := dm.DeferMessageCreate(m.Interaction, true)
	if err != nil {
		log.Error(err)
		return true
	}

	// Get the session
	session, err := sessionManager.GetSession(id.TextCID)
	if err != nil {
		log.Error("failed to get session", "err", err)
		if _, err := followup(TextDisplay(defaultErrorMsg)); err != nil {
			log.Error(err)
		}
		return true
	}

	// Check if user is already a participant in another session
	pID, err := pp.GetParticipantID(ctx, m.Member.User.ID)
	if err != nil {
		log.Error("failed to check existing participant", "err", err)
		if _, err := followup(TextDisplay(defaultErrorMsg)); err != nil {
			log.Error(err)
		}
		return true
	}
	if pID != "" {
		// for simplicity will just disallow joining in this case
		if _, err := followup(TextDisplay("You are already in a session.")); err != nil {
			log.Error(err)
		}
		return true

		// 	log.Error("failed to delete existing participant", "err", err)
		// 	if _, err := followup(TextDisplay(defaultErrorMsg)); err != nil {
		// 		log.Error(err)
		// 	}
		// 	return true
		// }
	}

	// Get user's current voice state
	vs, err := s.State.VoiceState(m.GuildID, m.Member.User.ID)
	if err != nil {
		log.Error("failed to get voice state", "userID", m.Member.User.ID, "sid", session.ID, "err", err)
		if _, err := followup(TextDisplay(defaultErrorMsg)); err != nil {
			log.Error(err)
		}
		return true
	}

	// Move user to session's voice channel
	if vs.ChannelID != string(session.Record.VoiceCID) {
		voiceCIDStr := string(session.Record.VoiceCID)
		err = s.GuildMemberMove(m.GuildID, m.Member.User.ID, &voiceCIDStr)
		if err != nil {
			log.Error("failed to move user to voice channel", "err", err, "voiceCID", session.Record.VoiceCID)
			if _, err := followup(TextDisplay("Failed to move you to the session voice channel.")); err != nil {
				log.Error(err)
			}
			return true
		}
	}

	// Insert participant
	_, err = pp.Insert(ctx, pomomo.ParticipantRecord{
		SessionID:  session.ID,
		GuildID:    session.Record.GuildID,
		VoiceCID:   session.Record.VoiceCID,
		UserID:     m.Member.User.ID,
		IsMuted:    vs.Mute,
		IsDeafened: vs.Deaf,
	})
	if err != nil {
		log.Error("failed to insert participant", "err", err)
		if _, err := followup(TextDisplay(defaultErrorMsg)); err != nil {
			log.Error(err)
		}
		return true
	}

	go vsMgr.AutoShush(ctx, session)

	_, err = followup(TextDisplay("Joined session!"))
	if err != nil {
		log.Error(err)
		return true
	}
	log.Info("user joined session", "userID", m.Member.User.ID, "cid", session.Record.VoiceCID, "sessionID", session.ID)
	return true
}
