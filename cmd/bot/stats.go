package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/disgo"
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
)

const (
	leaderboardSize = 5
	// extra candidates so users who left the server can be skipped
	leaderboardCandidates = 25
)

type StatsRepo interface {
	GetUserStats(ctx context.Context, guildID, userID string, now time.Time) (pomomo.UserStats, error)
	GetLeaderboard(ctx context.Context, guildID string, now time.Time, limit int) ([]pomomo.LeaderboardEntry, error)
	GetServerTotals(ctx context.Context, guildID string, since time.Time) (pomomo.ServerTotals, error)
}

// MemberLookup resolves server display names; inGuild is false when the user has left the server.
var _ MemberLookup = (*disgo.Adapter)(nil)

type MemberLookup interface {
	DisplayName(gid, uid string) (name string, inGuild bool, err error)
}

// streakMarker returns the highest hot streak tier reached.
func streakMarker(d pomomo.ActiveDays) string {
	switch {
	case d.Quarter >= 64:
		return "🔥🔥🔥"
	case d.Month >= 16:
		return "🔥🔥"
	case d.Week >= 4:
		return "🔥"
	default:
		return ""
	}
}

func formatDuration(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

var markdownEscaper = strings.NewReplacer(`\`, `\\`, `*`, `\*`, `_`, `\_`, `~`, `\~`, "`", "\\`", `|`, `\|`, `>`, `\>`, `#`, `\#`, `-`, `\-`)

func escapeMarkdown(s string) string {
	return markdownEscaper.Replace(s)
}

func statsContainer(content string) discord.ContainerComponent {
	return discord.ContainerComponent{
		Components:  []discord.ContainerSubComponent{TextDisplay(content)},
		AccentColor: int(ColorOrange),
	}
}

func UserStatsComponents(s pomomo.UserStats) []discord.LayoutComponent {
	if s.Pomodoros == 0 {
		return []discord.LayoutComponent{TextDisplay("No pomodoros yet — join a session to start tracking.")}
	}
	lines := []string{
		"### Your stats",
		"**Focus time:** " + formatDuration(s.FocusTime),
		fmt.Sprintf("**Pomodoros:** %d", s.Pomodoros),
		fmt.Sprintf("**Sessions:** %d", s.Sessions),
		fmt.Sprintf("**Active days:** %d/%d this week · %d/%d this month · %d/%d this quarter",
			s.ActiveDays.Week, pomomo.WeekDays, s.ActiveDays.Month, pomomo.MonthDays, s.ActiveDays.Quarter, pomomo.QuarterDays),
	}
	if marker := streakMarker(s.ActiveDays); marker != "" {
		lines = append(lines, "**Streak:** "+marker)
	}
	lines = append(lines, fmt.Sprintf("**Last active:** <t:%d:R>", s.LastActive.Unix()))
	return []discord.LayoutComponent{statsContainer(strings.Join(lines, "\n"))}
}

var medals = []string{"🥇", "🥈", "🥉"}

// rankLabel uses medals for the top 3, then "4." etc.
func rankLabel(i int) string {
	if i < len(medals) {
		return medals[i]
	}
	return fmt.Sprintf("%d.", i+1)
}

type leaderboardRow struct {
	pomomo.LeaderboardEntry
	Name string
}

func LeaderboardComponents(rows []leaderboardRow, totals pomomo.ServerTotals) []discord.LayoutComponent {
	if len(rows) == 0 {
		return []discord.LayoutComponent{TextDisplay("No pomodoros this week.")}
	}
	lines := []string{"### This week's leaderboard"}
	for i, r := range rows {
		line := fmt.Sprintf("%s **%s** — %s · %d 🍅", rankLabel(i), escapeMarkdown(r.Name), formatDuration(r.FocusTime), r.Pomodoros)
		if marker := streakMarker(r.ActiveDays); marker != "" {
			line += " " + marker
		}
		lines = append(lines, line)
	}
	lines = append(lines, fmt.Sprintf("-# This week: %s focused · %d 🍅 · %d sessions · %d members",
		formatDuration(totals.FocusTime), totals.Pomodoros, totals.Sessions, totals.Users))
	return []discord.LayoutComponent{statsContainer(strings.Join(lines, "\n"))}
}

// buildLeaderboard renders the server's weekly top users, omitting users who left the server.
func buildLeaderboard(ctx context.Context, repo StatsRepo, members MemberLookup, gid string, now time.Time) ([]discord.LayoutComponent, error) {
	entries, err := repo.GetLeaderboard(ctx, gid, now, leaderboardCandidates)
	if err != nil {
		return nil, fmt.Errorf("failed to get leaderboard: %w", err)
	}
	var rows []leaderboardRow
	for _, e := range entries {
		if len(rows) == leaderboardSize {
			break
		}
		name, inGuild, err := members.DisplayName(gid, e.UserID)
		if err != nil {
			// likely lost access to the guild - don't burn a REST call per candidate
			log.Warn("failed to look up leaderboard display name - showing partial leaderboard", "gid", gid, "uid", e.UserID, "err", err)
			break
		}
		if !inGuild {
			continue
		}
		rows = append(rows, leaderboardRow{LeaderboardEntry: e, Name: name})
	}
	totals, err := repo.GetServerTotals(ctx, gid, pomomo.WindowStart(now, pomomo.WeekDays))
	if err != nil {
		return nil, fmt.Errorf("failed to get server totals: %w", err)
	}
	return LeaderboardComponents(rows, totals), nil
}
