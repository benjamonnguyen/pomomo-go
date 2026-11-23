package pomomo

import "time"

// rolling stats windows in UTC days, including today
const (
	WeekDays    = 7
	MonthDays   = 30
	QuarterDays = 90
)

// WindowStart returns the start of the UTC day that begins a rolling window of days ending today.
func WindowStart(now time.Time, days int) time.Time {
	return now.UTC().Truncate(24*time.Hour).AddDate(0, 0, -(days - 1))
}

// FocusIntervalRecord is a pomodoro completed by a participant.
type FocusIntervalRecord struct {
	GuildID, UserID string
	SessionID       SessionID
	Pomodoro        int // session's pomodoro number, makes crediting idempotent
	Duration        time.Duration
	CompletedAt     time.Time
}

// ActiveDays counts UTC days with a completed pomodoro in each rolling window.
type ActiveDays struct {
	Week, Month, Quarter int
}

type UserStats struct {
	FocusTime  time.Duration
	Pomodoros  int
	Sessions   int
	LastActive time.Time // zero if never active
	ActiveDays ActiveDays
}

// LeaderboardEntry holds a user's stats for the week.
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
