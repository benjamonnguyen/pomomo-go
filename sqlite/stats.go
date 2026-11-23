package sqlite

import (
	"context"
	"time"

	txStdLib "github.com/Thiht/transactor/stdlib"
	"github.com/charmbracelet/log"
	"github.com/google/uuid"

	"github.com/benjamonnguyen/pomomo-go"
)

// activeDaysCols counts distinct UTC days per window; args are the week, month and quarter starts
const activeDaysCols = `COUNT(DISTINCT CASE WHEN completed_at >= ? THEN date(completed_at, 'unixepoch') END),
	COUNT(DISTINCT CASE WHEN completed_at >= ? THEN date(completed_at, 'unixepoch') END),
	COUNT(DISTINCT CASE WHEN completed_at >= ? THEN date(completed_at, 'unixepoch') END)`

type statsRepo struct {
	dbGetter txStdLib.DBGetter
	l        log.Logger
}

func NewStatsRepo(dbGetter txStdLib.DBGetter, logger log.Logger) *statsRepo {
	return &statsRepo{
		dbGetter: dbGetter,
		l:        logger,
	}
}

// InsertFocusIntervals ignores records that were already credited.
func (r *statsRepo) InsertFocusIntervals(ctx context.Context, records ...pomomo.FocusIntervalRecord) error {
	db := r.dbGetter(ctx)
	query := "INSERT OR IGNORE INTO focus_intervals (id, guild_id, user_id, session_id, pomodoro, duration, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?)"
	for _, rec := range records {
		args := []any{uuid.NewString(), rec.GuildID, rec.UserID, string(rec.SessionID), rec.Pomodoro, int64(rec.Duration.Seconds()), rec.CompletedAt.Unix()}
		r.l.Debug("inserting focus interval", "query", query, "args", args)
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}

func windowArgs(now time.Time) []any {
	return []any{
		pomomo.WindowStart(now, pomomo.WeekDays).Unix(),
		pomomo.WindowStart(now, pomomo.MonthDays).Unix(),
		pomomo.WindowStart(now, pomomo.QuarterDays).Unix(),
	}
}

func (r *statsRepo) GetUserStats(ctx context.Context, guildID, userID string, now time.Time) (pomomo.UserStats, error) {
	query := `SELECT COALESCE(SUM(duration), 0), COUNT(*), COUNT(DISTINCT session_id), COALESCE(MAX(completed_at), 0), ` + activeDaysCols + `
	FROM focus_intervals WHERE guild_id = ? AND user_id = ?`
	args := append(windowArgs(now), guildID, userID)

	var focusSecs, lastActive int64
	var s pomomo.UserStats
	row := r.dbGetter(ctx).QueryRowContext(ctx, query, args...)
	if err := row.Scan(&focusSecs, &s.Pomodoros, &s.Sessions, &lastActive, &s.ActiveDays.Week, &s.ActiveDays.Month, &s.ActiveDays.Quarter); err != nil {
		return pomomo.UserStats{}, err
	}
	s.FocusTime = time.Duration(focusSecs) * time.Second
	if lastActive > 0 {
		s.LastActive = time.Unix(lastActive, 0)
	}
	return s, nil
}

// GetLeaderboard returns users ranked by focus time this week.
func (r *statsRepo) GetLeaderboard(ctx context.Context, guildID string, now time.Time, limit int) ([]pomomo.LeaderboardEntry, error) {
	week := pomomo.WindowStart(now, pomomo.WeekDays).Unix()
	query := `SELECT user_id,
	SUM(CASE WHEN completed_at >= ? THEN duration ELSE 0 END) AS week_focus,
	SUM(CASE WHEN completed_at >= ? THEN 1 ELSE 0 END),
	` + activeDaysCols + `
	FROM focus_intervals WHERE guild_id = ? AND completed_at >= ?
	GROUP BY user_id HAVING week_focus > 0
	ORDER BY week_focus DESC, user_id LIMIT ?`
	args := append([]any{week, week}, windowArgs(now)...)
	args = append(args, guildID, pomomo.WindowStart(now, pomomo.QuarterDays).Unix(), limit)

	rows, err := r.dbGetter(ctx).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint

	var entries []pomomo.LeaderboardEntry
	for rows.Next() {
		var e pomomo.LeaderboardEntry
		var focusSecs int64
		if err := rows.Scan(&e.UserID, &focusSecs, &e.Pomodoros, &e.ActiveDays.Week, &e.ActiveDays.Month, &e.ActiveDays.Quarter); err != nil {
			return nil, err
		}
		e.FocusTime = time.Duration(focusSecs) * time.Second
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *statsRepo) GetServerTotals(ctx context.Context, guildID string, since time.Time) (pomomo.ServerTotals, error) {
	query := `SELECT COALESCE(SUM(duration), 0), COUNT(*), COUNT(DISTINCT session_id), COUNT(DISTINCT user_id)
	FROM focus_intervals WHERE guild_id = ? AND completed_at >= ?`

	var focusSecs int64
	var t pomomo.ServerTotals
	row := r.dbGetter(ctx).QueryRowContext(ctx, query, guildID, since.Unix())
	if err := row.Scan(&focusSecs, &t.Pomodoros, &t.Sessions, &t.Users); err != nil {
		return pomomo.ServerTotals{}, err
	}
	t.FocusTime = time.Duration(focusSecs) * time.Second
	return t, nil
}
