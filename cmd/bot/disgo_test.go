package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
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
	if ended := SessionMessageComponents(s); len(ended) != 1 {
		t.Errorf("ended session should only show farewell, got %d components", len(ended))
	}
}
