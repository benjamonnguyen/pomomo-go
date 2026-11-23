package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Thiht/transactor"
	txStdLib "github.com/Thiht/transactor/stdlib"
	dsdb "github.com/benjamonnguyen/deadsimple/db/sqlite"
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/sqlite"
	"github.com/charmbracelet/log"
)

// newTestDB returns a migrated sqlite db in a temp dir.
func newTestDB(t *testing.T) (transactor.Transactor, txStdLib.DBGetter) {
	t.Helper()
	db, err := dsdb.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.RunMigrations(migrations); err != nil {
		t.Fatal(err)
	}
	return txStdLib.NewTransactor(db.DB(), txStdLib.NestedTransactionsSavepoints)
}

func TestMigrations(t *testing.T) {
	_, dbGetter := newTestDB(t)
	db := dbGetter(t.Context())
	if _, err := db.ExecContext(t.Context(), "SELECT completed_pomodoros, skips FROM sessions"); err != nil {
		t.Error(err)
	}
	if _, err := db.ExecContext(t.Context(), "SELECT id, guild_id, user_id, session_id, pomodoro, duration, completed_at FROM focus_intervals"); err != nil {
		t.Error(err)
	}
}

func TestSessionRepoPersistsStats(t *testing.T) {
	_, dbGetter := newTestDB(t)
	repo := sqlite.NewSessionRepo(dbGetter, *log.Default())
	ctx := t.Context()

	inserted, err := repo.InsertSession(ctx, pomomo.SessionRecord{GuildID: "1", TextCID: "2", VoiceCID: "3", MessageID: "4", CompletedPomodoros: 2})
	if err != nil {
		t.Fatal(err)
	}
	r := inserted.SessionRecord
	r.CompletedPomodoros, r.Skips = 3, 1
	if _, err := repo.UpdateSession(ctx, inserted.ID, r); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetSession(ctx, inserted.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CompletedPomodoros != 3 || got.Skips != 1 {
		t.Errorf("stats not persisted: %+v", got.SessionRecord)
	}
}

func TestStatsRepo(t *testing.T) {
	_, dbGetter := newTestDB(t)
	repo := sqlite.NewStatsRepo(dbGetter, *log.Default())
	ctx := t.Context()
	now := time.Date(2026, 11, 23, 15, 0, 0, 0, time.UTC)
	daysAgo := func(d int) time.Time { return now.AddDate(0, 0, -d) }
	pomodoro := func(uid, sid string, n int, at time.Time) pomomo.FocusIntervalRecord {
		return pomomo.FocusIntervalRecord{GuildID: "g", UserID: uid, SessionID: pomomo.SessionID(sid), Pomodoro: n, Duration: 20 * time.Minute, CompletedAt: at}
	}

	records := []pomomo.FocusIntervalRecord{
		// alice: 2 today (same session), 1 six days ago, 1 eight days ago (outside week)
		pomodoro("alice", "s1", 1, now),
		pomodoro("alice", "s1", 2, now.Add(-time.Hour)),
		pomodoro("alice", "s2", 1, daysAgo(6)),
		pomodoro("alice", "s3", 1, daysAgo(8)),
		// bob: 1 yesterday
		pomodoro("bob", "s4", 1, daysAgo(1)),
		// carol: only last month
		pomodoro("carol", "s5", 1, daysAgo(20)),
		// other guild
		{GuildID: "other", UserID: "alice", SessionID: "s6", Pomodoro: 1, Duration: time.Hour, CompletedAt: now},
	}
	if err := repo.InsertFocusIntervals(ctx, records...); err != nil {
		t.Fatal(err)
	}
	// crediting the same pomodoro again is a no-op
	if err := repo.InsertFocusIntervals(ctx, pomodoro("alice", "s1", 1, now)); err != nil {
		t.Fatal(err)
	}

	alice, err := repo.GetUserStats(ctx, "g", "alice", now)
	if err != nil {
		t.Fatal(err)
	}
	if !alice.LastActive.Equal(now) {
		t.Errorf("alice last active: %v", alice.LastActive)
	}
	alice.LastActive = time.Time{}
	want := pomomo.UserStats{FocusTime: 80 * time.Minute, Pomodoros: 4, Sessions: 3, ActiveDays: pomomo.ActiveDays{Week: 2, Month: 3, Quarter: 3}}
	if alice != want {
		t.Errorf("alice stats:\n got %+v\nwant %+v", alice, want)
	}
	if nobody, err := repo.GetUserStats(ctx, "g", "nobody", now); err != nil || nobody != (pomomo.UserStats{}) {
		t.Errorf("expected empty stats, got %+v, %v", nobody, err)
	}

	board, err := repo.GetLeaderboard(ctx, "g", now, 5)
	if err != nil {
		t.Fatal(err)
	}
	// carol has no pomodoros this week, alice's week excludes 8 days ago
	if len(board) != 2 || board[0].UserID != "alice" || board[0].FocusTime != 60*time.Minute || board[0].Pomodoros != 3 ||
		board[0].ActiveDays != (pomomo.ActiveDays{Week: 2, Month: 3, Quarter: 3}) || board[1].UserID != "bob" {
		t.Errorf("unexpected leaderboard: %+v", board)
	}
	if board, _ := repo.GetLeaderboard(ctx, "g", now, 1); len(board) != 1 {
		t.Errorf("limit not applied: %+v", board)
	}

	totals, err := repo.GetServerTotals(ctx, "g", pomomo.WindowStart(now, pomomo.WeekDays))
	if err != nil {
		t.Fatal(err)
	}
	if totals != (pomomo.ServerTotals{FocusTime: 80 * time.Minute, Pomodoros: 4, Sessions: 3, Users: 2}) {
		t.Errorf("unexpected totals: %+v", totals)
	}
}

func TestWindowStart(t *testing.T) {
	now := time.Date(2026, 11, 23, 23, 59, 0, 0, time.UTC)
	if got := pomomo.WindowStart(now, pomomo.WeekDays); !got.Equal(time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("week start: %v", got)
	}
	if got := pomomo.WindowStart(now, 1); !got.Equal(time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("today start: %v", got)
	}
}
