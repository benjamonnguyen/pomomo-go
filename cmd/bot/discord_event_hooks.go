package main

import (
	"context"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

const (
	defaultErrorMsg = "Looks like something went wrong. Try again in a bit or reach out to support."
)

// voiceChannels returns the voice channel before and after the update; "" means not in voice (or not cached before).
func voiceChannels(u *events.GuildVoiceStateUpdate) (before, after pomomo.VoiceChannelID) {
	if u.OldVoiceState.ChannelID != nil {
		before = pomomo.VoiceChannelID(u.OldVoiceState.ChannelID.String())
	}
	if u.VoiceState.ChannelID != nil {
		after = pomomo.VoiceChannelID(u.VoiceState.ChannelID.String())
	}
	return before, after
}

func RemoveParticipantOnVoiceChannelLeave(ctx context.Context, vs VoiceStateAdapter, pm ParticipantsManager, u *events.GuildVoiceStateUpdate) bool {
	before, after := voiceChannels(u)
	if before == "" {
		// don't need to handle joins since participation is removed on leave
		return false
	}
	if before == after {
		return false
	}
	gid, uid := u.VoiceState.GuildID.String(), u.VoiceState.UserID.String()
	cid := before
	unlock := pm.AcquireVoiceChannelLock(cid)
	defer unlock()

	if p := pm.Get(uid, cid); p != (models.Participant{}) {
		// voice state can't be modified once disconnected (40032) - detach so it's restored on next join
		restored := false
		if after != "" {
			if err := restoreVoiceState(ctx, vs, p); err != nil {
				logRestErr("failed voice state restore on voice channel leave", err, "gid", gid, "uid", uid)
			} else {
				restored = true
			}
		}
		if !restored {
			if _, err := pm.DetachFromChannel(ctx, uid, cid); err != nil {
				log.Error("failed to detach participant from channel", "err", err, "gid", gid, "uid", uid)
			}
			log.Debug("detached participant from voice channel", "uid", uid, "sid", p.Record.SessionID)
			return true
		}
		if err := pm.Delete(ctx, p.ID); err != nil {
			log.Error("failed participant delete on voice channel leave", "err", err, "gid", gid, "uid", uid)
		} else {
			log.Debug("removed participant on voice channel leave", "uid", uid, "sid", p.Record.SessionID)
		}
		return true
	}
	return false
}

func RestoreParticipantVoiceStateOnChannelJoin(ctx context.Context, vs VoiceStateAdapter, pm ParticipantsManager, u *events.GuildVoiceStateUpdate) bool {
	before, after := voiceChannels(u)
	if before == after || after == "" {
		// voice state can only be restored while connected
		return false
	}
	gid, uid := u.VoiceState.GuildID.String(), u.VoiceState.UserID.String()
	unlock := pm.AcquireVoiceChannelLock("")
	defer unlock()

	// get detached participants that we failed to restore
	if p := pm.Get(uid, ""); p != (models.Participant{}) && p.Record.GuildID == gid {
		if err := restoreVoiceState(ctx, vs, p); err != nil {
			logRestErr("failed voice state restore on voice channel join", err, "gid", gid, "uid", uid)
			return true
		}
		if err := pm.Delete(ctx, p.ID); err != nil {
			log.Error("failed participant delete after voice state restore", "err", err, "gid", gid, "uid", uid)
		} else {
			log.Debug("removed participant after voice state restore", "uid", uid, "sid", p.Record.SessionID)
		}
		return true
	}
	return false
}

// guildMember returns the interaction's guild and member; ok is false outside of guilds.
func guildMember(it discord.Interaction) (gid string, member *discord.ResolvedMember, ok bool) {
	if it.GuildID() == nil || it.Member() == nil {
		return "", nil, false
	}
	return it.GuildID().String(), it.Member(), true
}

func StartSession(ctx context.Context, sessionManager SessionManager, dm DiscordMessenger, vsa VoiceStateAdapter, pinner MessagePinner, e *events.ApplicationCommandInteractionCreate) bool {
	if e.Data.Type() != discord.ApplicationCommandTypeSlash {
		return false
	}

	data := e.SlashCommandInteractionData()
	if data.CommandName() != pomomo.StartCommand.Name {
		return false
	}
	it := e.ApplicationCommandInteraction

	guildID, member, ok := guildMember(it)
	if !ok {
		if _, err := dm.Respond(it, false, TextDisplay("Pomomo sessions can only be started in a server.")); err != nil {
			log.Error(err)
		}
		return true
	}
	channelID := it.Channel().ID().String()
	userID := member.User.ID.String()

	// Parse command options with defaults
	settings := pomomo.SessionSettingsRecord{
		Pomodoro:   20 * time.Minute,
		ShortBreak: 5 * time.Minute,
		LongBreak:  15 * time.Minute,
		Intervals:  4,
		NoMute:     false,
		NoDeafen:   false,
	}
	if v, ok := data.OptInt(pomomo.PomodoroOption); ok {
		settings.Pomodoro = time.Duration(v) * time.Minute
	}
	if v, ok := data.OptInt(pomomo.ShortBreakOption); ok {
		settings.ShortBreak = time.Duration(v) * time.Minute
	}
	if v, ok := data.OptInt(pomomo.LongBreakOption); ok {
		settings.LongBreak = time.Duration(v) * time.Minute
	}
	if v, ok := data.OptInt(pomomo.IntervalsOption); ok {
		settings.Intervals = v
	}
	if v, ok := data.OptBool(pomomo.NoMuteOption); ok {
		settings.NoMute = v
	}
	if v, ok := data.OptBool(pomomo.NoDeafenOption); ok {
		settings.NoDeafen = v
	}

	// TODO multisession
	if sessionManager.GuildSessionCnt(guildID) > 0 {
		if _, err := dm.Respond(it, false, TextDisplay("Pomomo is limited to one session per server for now.")); err != nil {
			log.Error(err)
		}
		return true
	}

	if sessionManager.HasSession(channelID) {
		if _, err := dm.Respond(it, false, TextDisplay("This channel already has an active session.")); err != nil {
			log.Error(err)
		}
		return true
	}

	// get voice channel
	vs, err := vsa.GetVoiceState(guildID, userID)
	if err != nil {
		log.Debug("failed to get voice state", "userID", userID, "guildID", guildID, "err", err)
		_, err = dm.Respond(it, false, TextDisplay("Pomomo couldn't find your voice channel. Please join a voice channel with permissions and try again."))
		if err != nil {
			log.Error(err)
		}
		return true
	}
	if sessionManager.HasVoiceSession(string(vs.ChannelID)) {
		_, err = dm.Respond(it, false, TextDisplay("Your voice channel already has an active session. Please join another voice channel and try again."))
		if err != nil {
			log.Error(err)
		}
		return true
	}

	session := models.NewSession("", guildID, channelID, string(vs.ChannelID), "", settings)
	session.GoNextInterval(false) // initialize fields for display - "real" session is created by sessionManager
	msg, err := dm.Respond(it, true, SessionMessageComponents(session)...)
	if err != nil {
		log.Error(err)
		return true
	}

	session, err = sessionManager.StartSession(ctx, startSessionRequest{
		guildID:   guildID,
		textCID:   channelID,
		voiceCID:  string(vs.ChannelID),
		messageID: msg.ID.String(),
		settings:  settings,
		user: struct {
			id   string
			mute bool
			deaf bool
		}{
			id:   userID,
			mute: member.Mute,
			deaf: member.Deaf,
		},
	})
	if err != nil {
		log.Error("failed to start session", "err", err)
		if _, err := dm.EditResponse(it, TextDisplay("Failed to start session.")); err != nil {
			log.Error(err)
		}
		return true
	}
	log.Info("started session", "id", session.ID)

	if err := pinner.PinMessage(msg.ChannelID, msg.ID); err != nil {
		log.Error("failed to pin message", "err", err)
	}
	// TODO SessionManager goroutine to update stats with lesser frequency

	return true
}

// MessagePinner is satisfied by rest.Rest
type MessagePinner interface {
	PinMessage(channelID snowflake.ID, messageID snowflake.ID, opts ...rest.RequestOpt) error
	UnpinMessage(channelID snowflake.ID, messageID snowflake.ID, opts ...rest.RequestOpt) error
}

func SkipInterval(ctx context.Context, sessionManager SessionManager, dm DiscordMessenger, e *events.ComponentInteractionCreate) bool {
	id, err := FromCustomID(e.Data.CustomID())
	if err != nil {
		return false
	}
	if id.Type != "skip" {
		return false
	}

	followup, err := dm.DeferMessageUpdate(e.ComponentInteraction)
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

func EndSession(ctx context.Context, sessionManager SessionManager, dm DiscordMessenger, e *events.ComponentInteractionCreate) bool {
	id, err := FromCustomID(e.Data.CustomID())
	if err != nil {
		return false
	}
	if id.Type != "end" {
		return false
	}

	if _, err := dm.DeferMessageUpdate(e.ComponentInteraction); err != nil {
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

func JoinSession(ctx context.Context, sessionManager SessionManager, a Autoshusher, pp ParticipantsManager, dm DiscordMessenger, vsa VoiceStateAdapter, e *events.ComponentInteractionCreate) bool {
	id, err := FromCustomID(e.Data.CustomID())
	if err != nil {
		return false
	}
	if id.Type != "join" {
		return false
	}
	it := e.ComponentInteraction

	followup, err := dm.DeferMessageCreate(it, true)
	if err != nil {
		log.Error(err)
		return true
	}

	guildID, member, ok := guildMember(it)
	if !ok {
		if _, err := followup(TextDisplay(defaultErrorMsg)); err != nil {
			log.Error(err)
		}
		return true
	}
	userID := member.User.ID.String()

	// Get the session
	session, err := sessionManager.GetSession(id.TextCID)
	if err != nil {
		log.Error("failed to get session", "err", err)
		if _, err := followup(TextDisplay(defaultErrorMsg)); err != nil {
			log.Error(err)
		}
		return true
	}

	// Check if user is already a participant; leftover records from previous sessions are replaced on insert
	if p := pp.Get(userID, session.Record.VoiceCID); p != (models.Participant{}) {
		if _, err := followup(TextDisplay("You are already in this session.")); err != nil {
			log.Error(err)
		}
		return true
	}

	// Get user's current voice state
	vs, err := vsa.GetVoiceState(guildID, userID)
	if err != nil {
		log.Error("failed to get voice state", "userID", userID, "sid", session.ID, "err", err)
		if _, err := followup(TextDisplay(defaultErrorMsg)); err != nil {
			log.Error(err)
		}
		return true
	}

	// Move user to session's voice channel
	if vs.ChannelID != session.Record.VoiceCID {
		err = vsa.MoveMember(guildID, userID, session.Record.VoiceCID)
		if err != nil {
			log.Error("failed to move user to voice channel", "err", err, "voiceCID", session.Record.VoiceCID)
			if _, err := followup(TextDisplay("Failed to move you to the session voice channel.")); err != nil {
				log.Error(err)
			}
			return true
		}
	}

	// Insert participant
	participant, err := pp.Insert(ctx, pomomo.ParticipantRecord{
		SessionID:  session.ID,
		GuildID:    session.Record.GuildID,
		VoiceCID:   session.Record.VoiceCID,
		UserID:     userID,
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

	go func() {
		a.Autoshush(ctx, []models.Participant{participant}, models.Session{}, session)
	}()

	_, err = followup(TextDisplay("Joined session!"))
	if err != nil {
		log.Error(err)
		return true
	}
	log.Info("user joined session", "userID", userID, "cid", session.Record.VoiceCID, "sessionID", session.ID)
	return true
}
