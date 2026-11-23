package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/disgo"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func TestStreakMarker(t *testing.T) {
	tests := []struct {
		days pomomo.ActiveDays
		want string
	}{
		{pomomo.ActiveDays{Week: 3, Month: 15, Quarter: 63}, ""},
		{pomomo.ActiveDays{Week: 4, Month: 4, Quarter: 4}, "🔥"},
		{pomomo.ActiveDays{Week: 2, Month: 16, Quarter: 16}, "🔥🔥"},
		{pomomo.ActiveDays{Week: 7, Month: 30, Quarter: 64}, "🔥🔥🔥"},
	}
	for _, tt := range tests {
		if got := streakMarker(tt.days); got != tt.want {
			t.Errorf("streakMarker(%+v) = %q, want %q", tt.days, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "0m", 45 * time.Minute: "45m", 160 * time.Minute: "2h 40m", 25 * time.Hour: "25h 0m"} {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestEscapeMarkdown(t *testing.T) {
	if got := escapeMarkdown("tomato_fan*"); got != `tomato\_fan\*` {
		t.Errorf("got %q", got)
	}
}

type fakeStatsRepo struct {
	StatsRepo
	entries []pomomo.LeaderboardEntry
	limit   int
	windows [][2]time.Time // GetServerTotals calls
}

func (f *fakeStatsRepo) GetLeaderboard(ctx context.Context, guildID string, now time.Time, limit int) ([]pomomo.LeaderboardEntry, error) {
	f.limit = limit
	return f.entries, nil
}

// GetServerTotals returns 214h this week and ~181h last week (up 18%)
func (f *fakeStatsRepo) GetServerTotals(ctx context.Context, guildID string, since, until time.Time) (pomomo.ServerTotals, error) {
	f.windows = append(f.windows, [2]time.Time{since, until})
	if len(f.windows) == 1 {
		return pomomo.ServerTotals{FocusTime: 214*time.Hour + 20*time.Minute}, nil
	}
	return pomomo.ServerTotals{FocusTime: 181*time.Hour + 30*time.Minute}, nil
}

// fakeMembers: uids starting with "gone" have left, "err" fail lookup
type fakeMembers struct{}

func (fakeMembers) DisplayName(gid, uid string) (string, bool, error) {
	switch {
	case strings.HasPrefix(uid, "gone"):
		return "", false, nil
	case strings.HasPrefix(uid, "err"):
		return "", false, errors.New("boom")
	}
	return "name_" + uid, true, nil
}

// componentsText marshals components; note json escapes < and > as \u003c and \u003e
func componentsText(t *testing.T, components []discord.LayoutComponent) string {
	t.Helper()
	b, err := json.Marshal(components)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBuildLeaderboard(t *testing.T) {
	var entries []pomomo.LeaderboardEntry
	for _, uid := range []string{"a", "gone1", "b", "c", "d", "e", "f"} {
		entries = append(entries, pomomo.LeaderboardEntry{UserID: uid, FocusTime: 20 * time.Minute, Pomodoros: 1})
	}
	entries[0].ActiveDays.Week = 4
	repo := &fakeStatsRepo{entries: entries}

	now := time.Date(2026, 11, 25, 15, 0, 0, 0, time.UTC) // Wednesday
	components, err := buildLeaderboard(t.Context(), repo, fakeMembers{}, "g", now)
	if err != nil {
		t.Fatal(err)
	}
	text := componentsText(t, components)
	if repo.limit != leaderboardCandidates {
		t.Errorf("expected %d candidates, got %d", leaderboardCandidates, repo.limit)
	}
	// this week so far vs last week up to the same point
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	wantWindows := [][2]time.Time{{monday, now}, {monday.AddDate(0, 0, -7), now.AddDate(0, 0, -7)}}
	if len(repo.windows) != 2 || !repo.windows[0][0].Equal(wantWindows[0][0]) || !repo.windows[0][1].Equal(wantWindows[0][1]) ||
		!repo.windows[1][0].Equal(wantWindows[1][0]) || !repo.windows[1][1].Equal(wantWindows[1][1]) {
		t.Errorf("server total windows = %v, want %v", repo.windows, wantWindows)
	}
	for _, want := range []string{"-# Nov 23 – Nov 29 UTC", `🥇 **name\\_a** 🔥 — 20m`, `🥈 **name\\_b** — 20m`, `🥉 **name\\_c**`, `4. **name\\_d**`, `5. **name\\_e**`, "Server total: 214h · up 18% from last week"} {
		if !strings.Contains(text, want) {
			t.Errorf("leaderboard missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"gone1", "name\\\\_f", `\u003c@`, "🍅"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("leaderboard shouldn't contain %q:\n%s", unwanted, text)
		}
	}

	// lookup errors stop further lookups and show what was resolved so far
	partial, err := buildLeaderboard(t.Context(), &fakeStatsRepo{entries: []pomomo.LeaderboardEntry{
		{UserID: "a", FocusTime: time.Hour}, {UserID: "err1", FocusTime: time.Hour}, {UserID: "b", FocusTime: time.Hour},
	}}, fakeMembers{}, "g", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if text := componentsText(t, partial); !strings.Contains(text, `name\\_a`) || strings.Contains(text, `name\\_b`) {
		t.Errorf("expected partial leaderboard with only a:\n%s", text)
	}

	empty, _ := buildLeaderboard(t.Context(), &fakeStatsRepo{}, fakeMembers{}, "g", time.Now())
	if !strings.Contains(componentsText(t, empty), "No pomodoros this week.") {
		t.Error("expected empty leaderboard message")
	}
}

func TestUserStatsComponents(t *testing.T) {
	if !strings.Contains(componentsText(t, UserStatsComponents(pomomo.UserStats{})), "No pomodoros yet") {
		t.Error("expected empty stats message")
	}
	text := componentsText(t, UserStatsComponents(pomomo.UserStats{
		FocusTime: 90 * time.Minute, Pomodoros: 4, Sessions: 2, LastActive: time.Unix(1700000000, 0),
		ActiveDays: pomomo.ActiveDays{Week: 1, Month: 16, Quarter: 16},
	}))
	for _, want := range []string{"1h 30m", "**Pomodoros:** 4", "1/7 this week", "16/30 this month", "**Streak:** 🔥🔥", `\u003ct:1700000000:R\u003e`} {
		if !strings.Contains(text, want) {
			t.Errorf("stats missing %q:\n%s", want, text)
		}
	}
}

type fakeMessenger struct {
	DiscordMessenger
	ephemeral bool
	sent      []discord.LayoutComponent
}

func (f *fakeMessenger) DeferMessageCreate(it discord.Interaction, ephemeral bool) (disgo.Followup, error) {
	f.ephemeral = ephemeral
	return func(components ...discord.LayoutComponent) (*discord.Message, error) {
		f.sent = components
		return &discord.Message{}, nil
	}, nil
}

func (f *fakeStatsRepo) GetUserStats(ctx context.Context, guildID, userID string, now time.Time) (pomomo.UserStats, error) {
	return pomomo.UserStats{Pomodoros: 1, FocusTime: 20 * time.Minute, LastActive: now}, nil
}

func statsCommandEvent(t *testing.T, options string) *events.ApplicationCommandInteractionCreate {
	t.Helper()
	it, err := discord.UnmarshalInteraction([]byte(`{
		"id":"333","application_id":"444","type":2,"token":"tok","version":1,
		"guild_id":"555","channel":{"id":"111","type":0},
		"member":{"user":{"id":"666","username":"u"},"roles":[],"joined_at":"2026-01-01T00:00:00Z"},
		"data":{"id":"777","name":"stats","type":1,"options":` + options + `}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	return &events.ApplicationCommandInteractionCreate{ApplicationCommandInteraction: it.(discord.ApplicationCommandInteraction)}
}

func TestStatsCommand(t *testing.T) {
	tests := []struct {
		name, options string
		ephemeral     bool
		want          string
	}{
		{"default is me", `[]`, true, "Your stats"},
		{"me", `[{"name":"scope","type":3,"value":"me"}]`, true, "Your stats"},
		{"server", `[{"name":"scope","type":3,"value":"server"}]`, false, "This week's leaderboard"},
	}
	for _, tt := range tests {
		dm := &fakeMessenger{}
		repo := &fakeStatsRepo{entries: []pomomo.LeaderboardEntry{{UserID: "a", FocusTime: time.Hour, Pomodoros: 3}}}
		if !Stats(t.Context(), repo, fakeMembers{}, dm, statsCommandEvent(t, tt.options)) {
			t.Fatalf("%s: stats command not handled", tt.name)
		}
		if dm.ephemeral != tt.ephemeral {
			t.Errorf("%s: ephemeral = %v, want %v", tt.name, dm.ephemeral, tt.ephemeral)
		}
		if text := componentsText(t, dm.sent); !strings.Contains(text, tt.want) {
			t.Errorf("%s: reply missing %q:\n%s", tt.name, tt.want, text)
		}
	}
}

func TestWeekOverWeek(t *testing.T) {
	tests := []struct {
		curr, prev time.Duration
		want       string
	}{
		{10 * time.Hour, 0, ""},
		{118 * time.Hour, 100 * time.Hour, "up 18% from last week"},
		{95 * time.Hour, 100 * time.Hour, "down 5% from last week"},
		{100 * time.Hour, 100 * time.Hour, "same as last week"},
		{0, 100 * time.Hour, "down 100% from last week"},
	}
	for _, tt := range tests {
		if got := weekOverWeek(tt.curr, tt.prev); got != tt.want {
			t.Errorf("weekOverWeek(%v, %v) = %q, want %q", tt.curr, tt.prev, got, tt.want)
		}
	}
	for d, want := range map[time.Duration]string{45 * time.Minute: "45m", 214*time.Hour + 50*time.Minute: "214h"} {
		if got := formatTotal(d); got != want {
			t.Errorf("formatTotal(%v) = %q, want %q", d, got, want)
		}
	}
}
