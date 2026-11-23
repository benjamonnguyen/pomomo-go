package main

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/disgo"
	pomoimage "github.com/benjamonnguyen/pomomo-go/image"
	"github.com/charmbracelet/log"
	"github.com/disgoorg/disgo/discord"
)

const (
	leaderboardSize = 5
	// extra candidates so users who left the server can be skipped
	leaderboardCandidates = 25
)

type StatsRepo interface {
	GetWeeklyStats(ctx context.Context, guildID, userID string, now time.Time) (pomomo.WeeklyStats, error)
	GetLeaderboard(ctx context.Context, guildID string, now time.Time, limit int) ([]pomomo.LeaderboardEntry, error)
	GetServerTotals(ctx context.Context, guildID string, since, until time.Time) (pomomo.ServerTotals, error)
}

// MemberLookup resolves server display names; inGuild is false when the user has left the server.
var _ MemberLookup = (*disgo.Adapter)(nil)

type MemberLookup interface {
	DisplayName(gid, uid string) (name string, inGuild bool, err error)
}

// streakTier returns the highest hot streak tier reached, 0–3.
func streakTier(d pomomo.ActiveDays) int {
	switch {
	case d.Quarter >= 64:
		return 3
	case d.Month >= 16:
		return 2
	case d.Week >= 4:
		return 1
	default:
		return 0
	}
}

func streakMarker(d pomomo.ActiveDays) string {
	return strings.Repeat("🔥", streakTier(d))
}

func formatDuration(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

// formatTotal rounds down to whole hours once past an hour
func formatTotal(d time.Duration) string {
	if d < time.Hour {
		return formatDuration(d)
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// weekOverWeek describes the change from last week; empty without a baseline.
func weekOverWeek(curr, prev time.Duration) string {
	if prev <= 0 {
		return ""
	}
	pct := int(math.Round(float64(curr-prev) / float64(prev) * 100))
	switch {
	case pct > 0:
		return fmt.Sprintf("up %d%% from last week", pct)
	case pct < 0:
		return fmt.Sprintf("down %d%% from last week", -pct)
	default:
		return "same as last week"
	}
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

// newRecap builds the recap card's data; name is the user's server display name.
func newRecap(name string, s pomomo.WeeklyStats, now time.Time) pomoimage.Recap {
	return pomoimage.Recap{
		Name:      name,
		WeekStart: pomomo.WeekStart(now),
		Now:       now,
		Days:      s.Days,
		Streak:    streakTier(s.ActiveDays),
		Month:     s.Month,
		Lifetime:  s.Lifetime,
	}
}

// RecapComponents is the text version of the recap card, sent when the card can't be.
func RecapComponents(r pomoimage.Recap) []discord.LayoutComponent {
	title := "Your Weekly Recap"
	if name := strings.TrimSpace(r.Name); name != "" {
		title = escapeMarkdown(name) + "'s Weekly Recap"
	}
	if marker := strings.Repeat("🔥", r.Streak); marker != "" {
		title += " " + marker
	}
	weekEnd := r.WeekStart.AddDate(0, 0, 6)
	lines := []string{
		"### " + title,
		"-# " + r.WeekStart.Format("Mon, Jan 2") + " – " + weekEnd.Format("Mon, Jan 2") + " UTC",
		"**Total focus**",
		fmt.Sprintf("This week %s · This month %s · Lifetime %s",
			pomoimage.FormatDuration(r.Week()), pomoimage.FormatDuration(r.Month), pomoimage.FormatHours(r.Lifetime)),
	}
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

// weekRange formats the Monday week as "Nov 23 – Nov 29 UTC".
func weekRange(weekStart time.Time) string {
	return weekStart.Format("Jan 2") + " – " + weekStart.AddDate(0, 0, 6).Format("Jan 2") + " UTC"
}

func LeaderboardComponents(rows []leaderboardRow, weekStart time.Time, thisWeek, lastWeek pomomo.ServerTotals) []discord.LayoutComponent {
	if len(rows) == 0 {
		return []discord.LayoutComponent{TextDisplay("No pomodoros this week.")}
	}
	lines := []string{"### This week's leaderboard", "-# " + weekRange(weekStart)}
	for i, r := range rows {
		name := "**" + escapeMarkdown(r.Name) + "**"
		if marker := streakMarker(r.ActiveDays); marker != "" {
			name += " " + marker
		}
		lines = append(lines, fmt.Sprintf("%s %s — %s", rankLabel(i), name, formatDuration(r.FocusTime)))
	}
	total := "Server total: " + formatTotal(thisWeek.FocusTime)
	if change := weekOverWeek(thisWeek.FocusTime, lastWeek.FocusTime); change != "" {
		total += " · " + change
	}
	lines = append(lines, "", total)
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
	// compare with last week up to the same point so Mondays don't always read "down"
	weekStart := pomomo.WeekStart(now)
	thisWeek, err := repo.GetServerTotals(ctx, gid, weekStart, now)
	if err != nil {
		return nil, fmt.Errorf("failed to get server totals: %w", err)
	}
	lastWeek, err := repo.GetServerTotals(ctx, gid, weekStart.AddDate(0, 0, -7), now.AddDate(0, 0, -7))
	if err != nil {
		return nil, fmt.Errorf("failed to get last week's server totals: %w", err)
	}
	return LeaderboardComponents(rows, weekStart, thisWeek, lastWeek), nil
}
