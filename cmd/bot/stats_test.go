package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/disgoorg/disgo/discord"
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
}

func (f *fakeStatsRepo) GetLeaderboard(ctx context.Context, guildID string, now time.Time, limit int) ([]pomomo.LeaderboardEntry, error) {
	f.limit = limit
	return f.entries, nil
}

func (f *fakeStatsRepo) GetServerTotals(ctx context.Context, guildID string, since time.Time) (pomomo.ServerTotals, error) {
	return pomomo.ServerTotals{FocusTime: time.Hour, Pomodoros: 3, Sessions: 1, Users: 2}, nil
}

// fakeMembers: uids starting with "gone" have left, "err" fail lookup
type fakeMembers struct{}

func (fakeMembers) Username(gid, uid string) (string, bool, error) {
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
	for _, uid := range []string{"a", "gone1", "b", "err1", "c", "d", "e", "f"} {
		entries = append(entries, pomomo.LeaderboardEntry{UserID: uid, FocusTime: 20 * time.Minute, Pomodoros: 1})
	}
	entries[0].ActiveDays.Week = 4
	repo := &fakeStatsRepo{entries: entries}

	components, err := buildLeaderboard(t.Context(), repo, fakeMembers{}, "g", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	text := componentsText(t, components)
	if repo.limit != leaderboardCandidates {
		t.Errorf("expected %d candidates, got %d", leaderboardCandidates, repo.limit)
	}
	for _, want := range []string{`1. **name\\_a** — 20m · 1 🍅 🔥`, `5. **name\\_e**`, "This week: 1h 0m focused"} {
		if !strings.Contains(text, want) {
			t.Errorf("leaderboard missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{"gone1", "err1", "name\\\\_f", `\u003c@`} {
		if strings.Contains(text, unwanted) {
			t.Errorf("leaderboard shouldn't contain %q:\n%s", unwanted, text)
		}
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
