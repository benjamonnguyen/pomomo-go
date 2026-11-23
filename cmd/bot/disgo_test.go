package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/benjamonnguyen/pomomo-go/disgo"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

func TestVoiceChannels(t *testing.T) {
	a, b := snowflake.ID(1), snowflake.ID(2)
	tests := []struct {
		name          string
		old, curr     *snowflake.ID
		before, after pomomo.VoiceChannelID
	}{
		{"join (old not cached)", nil, &a, "", "1"},
		{"move", &a, &b, "1", "2"},
		{"leave", &a, nil, "1", ""},
		{"mute in place", &a, &a, "1", "1"},
	}
	for _, tt := range tests {
		u := &events.GuildVoiceStateUpdate{
			GenericGuildVoiceState: &events.GenericGuildVoiceState{VoiceState: discord.VoiceState{ChannelID: tt.curr}},
			OldVoiceState:          discord.VoiceState{ChannelID: tt.old},
		}
		before, after := voiceChannels(u)
		if before != tt.before || after != tt.after {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", tt.name, before, after, tt.before, tt.after)
		}
	}
}

func TestSessionMessageComponents(t *testing.T) {
	s := models.NewSession("", "555", "111", "777", "", pomomo.SessionSettingsRecord{
		Pomodoro: 20 * time.Minute, ShortBreak: 5 * time.Minute, LongBreak: 15 * time.Minute, Intervals: 4,
	})
	s.GoNextInterval(false)
	s.Greeting = "hi"

	b, err := json.Marshal(SessionMessageComponents(s))
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	// greeting text display, settings container, action row
	if len(got) != 3 || got[0]["type"] != float64(discord.ComponentTypeTextDisplay) ||
		got[1]["type"] != float64(discord.ComponentTypeContainer) || got[2]["type"] != float64(discord.ComponentTypeActionRow) {
		t.Fatalf("unexpected layout: %s", b)
	}
	if got[1]["accent_color"] != float64(ColorGreen) {
		t.Errorf("unexpected accent color: %v", got[1]["accent_color"])
	}
	buttons, _ := got[2]["components"].([]any)
	var ids []string
	for _, btn := range buttons {
		ids = append(ids, btn.(map[string]any)["custom_id"].(string))
	}
	if strings.Join(ids, ",") != "join:111,end:111,skip:111" {
		t.Errorf("unexpected button custom ids: %v", ids)
	}

	s.Record.Status = pomomo.SessionEnded
	s.Record.CompletedPomodoros = 3
	s.StartedAt = time.Now().Add(-100 * time.Minute)
	ended := SessionMessageComponents(s)
	if len(ended) != 2 {
		t.Fatalf("ended session should show farewell and summary, got %d components", len(ended))
	}
	if summary := ended[1].(discord.TextDisplayComponent).Content; summary != "**Session complete** · 1h 40m · 3 🍅" {
		t.Errorf("unexpected summary: %q", summary)
	}
}

// fakes for voice state hooks

type fakeVoiceStates struct {
	VoiceStateAdapter
	updates int
	err     error
	getErr  error
}

func (f *fakeVoiceStates) GetVoiceState(gid, uid string) (pomomo.VoiceState, error) {
	return pomomo.VoiceState{ChannelID: "1"}, f.getErr
}

func (f *fakeVoiceStates) UpdateVoiceState(gid, uid string, mute, deaf bool) error {
	f.updates++
	return f.err
}

type fakeParticipants struct {
	ParticipantsManager
	p                 models.Participant
	detached, deleted bool
}

func (f *fakeParticipants) AcquireVoiceChannelLock(pomomo.VoiceChannelID) func() { return func() {} }

func (f *fakeParticipants) Get(uid string, cid pomomo.VoiceChannelID) models.Participant {
	if f.p.Record.UserID == uid && f.p.Record.VoiceCID == cid {
		return f.p
	}
	return models.Participant{}
}

func (f *fakeParticipants) DetachFromChannel(ctx context.Context, uid string, cid pomomo.VoiceChannelID) (models.Participant, error) {
	f.detached = true
	return f.p, nil
}

func (f *fakeParticipants) Delete(ctx context.Context, id pomomo.ParticipantID) error {
	f.deleted = true
	return nil
}

func voiceStateUpdate(old, curr *snowflake.ID) *events.GuildVoiceStateUpdate {
	return &events.GuildVoiceStateUpdate{
		GenericGuildVoiceState: &events.GenericGuildVoiceState{VoiceState: discord.VoiceState{GuildID: 555, UserID: 666, ChannelID: curr}},
		OldVoiceState:          discord.VoiceState{ChannelID: old},
	}
}

func TestRemoveParticipantOnVoiceChannelLeave(t *testing.T) {
	a, b := snowflake.ID(1), snowflake.ID(2)
	participant := models.Participant{Record: pomomo.ParticipantRecord{GuildID: "555", UserID: "666", VoiceCID: "1"}}
	tests := []struct {
		name              string
		curr              *snowflake.ID
		restoreErr        error
		updates           int
		detached, deleted bool
	}{
		{"disconnect detaches without restore", nil, nil, 0, true, false},
		{"move restores and deletes", &b, nil, 1, false, true},
		{"move with failed restore detaches", &b, errors.New("50013"), 1, true, false},
	}
	for _, tt := range tests {
		vs := &fakeVoiceStates{err: tt.restoreErr}
		pm := &fakeParticipants{p: participant}
		if !RemoveParticipantOnVoiceChannelLeave(context.Background(), vs, pm, voiceStateUpdate(&a, tt.curr)) {
			t.Errorf("%s: expected participant to be handled", tt.name)
		}
		if vs.updates != tt.updates || pm.detached != tt.detached || pm.deleted != tt.deleted {
			t.Errorf("%s: got updates=%d detached=%v deleted=%v", tt.name, vs.updates, pm.detached, pm.deleted)
		}
	}
}

func TestRestoreParticipantVoiceStateOnChannelJoin(t *testing.T) {
	a := snowflake.ID(1)
	detached := models.Participant{Record: pomomo.ParticipantRecord{GuildID: "555", UserID: "666"}}

	vs, pm := &fakeVoiceStates{}, &fakeParticipants{p: detached}
	if RestoreParticipantVoiceStateOnChannelJoin(context.Background(), vs, pm, voiceStateUpdate(&a, nil)) || vs.updates != 0 {
		t.Error("disconnect shouldn't attempt restore")
	}
	if !RestoreParticipantVoiceStateOnChannelJoin(context.Background(), vs, pm, voiceStateUpdate(nil, &a)) || vs.updates != 1 || !pm.deleted {
		t.Errorf("join should restore and delete detached participant, got updates=%d deleted=%v", vs.updates, pm.deleted)
	}
}

func TestAutoshushDetachesParticipantNotInVoice(t *testing.T) {
	p := models.Participant{Record: pomomo.ParticipantRecord{GuildID: "555", UserID: "666", VoiceCID: "1"}}
	vs := &fakeVoiceStates{getErr: disgo.ErrVoiceStateNotFound}
	pm := &fakeParticipants{p: p}
	a := &autoshusher{pm: pm, vs: vs}

	before := models.Session{Record: pomomo.SessionRecord{CurrentInterval: pomomo.ShortBreakInterval}}
	curr := models.Session{Record: pomomo.SessionRecord{CurrentInterval: pomomo.PomodoroInterval, Skips: 1}}
	a.Autoshush(context.Background(), []models.Participant{p}, before, curr)

	if !pm.detached {
		t.Error("participant not in voice should be detached")
	}
	time.Sleep(10 * time.Millisecond) // shush runs async
	if vs.updates != 0 {
		t.Errorf("detached participant shouldn't be shushed, got %d updates", vs.updates)
	}
}

func TestTimerMsgFailures(t *testing.T) {
	f := newTimerMsgFailures()
	if !f.unreachable("s1", &rest.Error{Code: rest.JSONErrorCodeUnknownMessage}) {
		t.Error("deleted timer message should end session")
	}
	if f.unreachable("s2", errors.New("timeout")) {
		t.Error("transient error shouldn't end session")
	}

	missingAccess := &rest.Error{Code: rest.JSONErrorCodeMissingAccess}
	for i := 1; i < maxTimerMsgMissingAccess; i++ {
		if f.unreachable("s2", missingAccess) {
			t.Fatalf("ended after %d missing access failures", i)
		}
	}
	f.reset("s2")
	if f.unreachable("s2", missingAccess) {
		t.Error("reset should clear missing access count")
	}
	for i := 1; i < maxTimerMsgMissingAccess-1; i++ {
		f.unreachable("s2", missingAccess)
	}
	if !f.unreachable("s2", missingAccess) {
		t.Errorf("should end after %d consecutive missing access failures", maxTimerMsgMissingAccess)
	}
}
