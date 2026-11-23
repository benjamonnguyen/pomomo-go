package pomomo

import "time"

// rolling streak windows in UTC days, including today
const (
	WeekDays    = 7
	MonthDays   = 30
	QuarterDays = 90
)

// WindowStart returns the start of the UTC day that begins a rolling window of days ending today.
func WindowStart(now time.Time, days int) time.Time {
	return now.UTC().Truncate(24*time.Hour).AddDate(0, 0, -(days - 1))
}

// WeekStart returns 00:00 UTC of the Monday on or before now.
func WeekStart(now time.Time) time.Time {
	day := now.UTC().Truncate(24 * time.Hour)
	return day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
}

// MonthStart returns 00:00 UTC on the first of now's month.
func MonthStart(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// FocusIntervalRecord is a pomodoro completed by a participant.
type FocusIntervalRecord struct {
	GuildID, UserID string
	SessionID       SessionID
	Pomodoro        int // session's pomodoro number, makes crediting idempotent
	Duration        time.Duration
	CompletedAt     time.Time
}

// ActiveDays counts UTC days with a completed pomodoro in each rolling streak window.
type ActiveDays struct {
	Week, Month, Quarter int
}

// WeeklyStats is a user's Monday week in a server, plus month and lifetime totals.
type WeeklyStats struct {
	Days       [7]time.Duration // focus time per UTC day, Monday first
	Month      time.Duration    // since the 1st of the month
	Lifetime   time.Duration
	ActiveDays ActiveDays // rolling, for streaks
}

// LeaderboardEntry holds a user's stats for the week, starting Monday.
type LeaderboardEntry struct {
	UserID     string
	FocusTime  time.Duration
	Pomodoros  int
	ActiveDays ActiveDays
}

type ServerTotals struct {
	FocusTime time.Duration
	Pomodoros int
	Sessions  int
	Users     int
}
