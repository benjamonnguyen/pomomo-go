package main

import (
	"context"
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
	// now is a Monday: the week only has alice's 2 from today; bob's Sunday is last week.
	// active days stay rolling, so alice's week still counts 6 days ago.
	if len(board) != 1 || board[0].UserID != "alice" || board[0].FocusTime != 40*time.Minute || board[0].Pomodoros != 2 ||
		board[0].ActiveDays != (pomomo.ActiveDays{Week: 2, Month: 3, Quarter: 3}) {
		t.Errorf("unexpected leaderboard: %+v", board)
	}
	if err := repo.InsertFocusIntervals(ctx, pomodoro("bob", "s7", 1, now.Add(-2*time.Hour))); err != nil {
		t.Fatal(err)
	}
	if board, _ := repo.GetLeaderboard(ctx, "g", now, 1); len(board) != 1 || board[0].UserID != "alice" {
		t.Errorf("limit not applied: %+v", board)
	}

	weekStart := pomomo.WeekStart(now)
	totals, err := repo.GetServerTotals(ctx, "g", weekStart, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if totals != (pomomo.ServerTotals{FocusTime: 60 * time.Minute, Pomodoros: 3, Sessions: 2, Users: 2}) {
		t.Errorf("unexpected totals: %+v", totals)
	}
	// previous Monday week: alice 6 days ago and bob's Sunday; until is exclusive
	prev, err := repo.GetServerTotals(ctx, "g", weekStart.AddDate(0, 0, -7), weekStart)
	if err != nil {
		t.Fatal(err)
	}
	if prev.Pomodoros != 2 || prev.FocusTime != 40*time.Minute {
		t.Errorf("unexpected previous week totals: %+v", prev)
	}
}

func TestWeekStart(t *testing.T) {
	day := func(y int, m time.Month, d, h, min int) time.Time { return time.Date(y, m, d, h, min, 0, 0, time.UTC) }
	for now, want := range map[time.Time]time.Time{
		day(2026, 11, 23, 0, 0):   day(2026, 11, 23, 0, 0), // Monday 00:00
		day(2026, 11, 29, 23, 59): day(2026, 11, 23, 0, 0), // Sunday 23:59
		day(2026, 11, 30, 0, 0):   day(2026, 11, 30, 0, 0), // next Monday
		day(2027, 1, 1, 12, 0):    day(2026, 12, 28, 0, 0), // year edge (Friday)
	} {
		if got := pomomo.WeekStart(now); !got.Equal(want) {
			t.Errorf("WeekStart(%v) = %v, want %v", now, got, want)
		}
	}
	// non-UTC input is converted first: Mon 01:00 in UTC+2 is Sun 23:00 UTC
	if got := pomomo.WeekStart(time.Date(2026, 11, 30, 1, 0, 0, 0, time.FixedZone("", 2*3600))); !got.Equal(day(2026, 11, 23, 0, 0)) {
		t.Errorf("WeekStart in UTC+2 = %v", got)
	}
	for now, want := range map[time.Time]time.Time{
		day(2026, 12, 31, 23, 59): day(2026, 12, 1, 0, 0),
		day(2027, 1, 1, 0, 0):     day(2027, 1, 1, 0, 0),
	} {
		if got := pomomo.MonthStart(now); !got.Equal(want) {
			t.Errorf("MonthStart(%v) = %v, want %v", now, got, want)
		}
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

type testManagers struct {
	sm    *sessionManager
	pm    ParticipantsManager
	repo  SessionRepo
	stats interface {
		GetUserStats(ctx context.Context, guildID, userID string, now time.Time) (pomomo.UserStats, error)
	}
}

func newTestManagers(t *testing.T) testManagers {
	tx, dbGetter := newTestDB(t)
	sessionRepo := sqlite.NewSessionRepo(dbGetter, *log.Default())
	statsRepo := sqlite.NewStatsRepo(dbGetter, *log.Default())
	pm := NewParticipantManager(sqlite.NewParticipantRepo(dbGetter, *log.Default()), tx, *log.Default())
	sm := NewSessionManager(t.Context(), sessionRepo, statsRepo, pm, tx).(*sessionManager)
	t.Cleanup(func() { _ = sm.Shutdown() })
	return testManagers{sm: sm, pm: pm, repo: sessionRepo, stats: statsRepo}
}

var testSettings = pomomo.SessionSettingsRecord{Pomodoro: 20 * time.Minute, ShortBreak: 5 * time.Minute, LongBreak: 15 * time.Minute, Intervals: 4}

func TestUpdateSessionCreditsAttachedParticipants(t *testing.T) {
	m := newTestManagers(t)
	ctx := t.Context()
	req := startSessionRequest{guildID: "g", textCID: "t", voiceCID: "v", messageID: "m", settings: testSettings}
	req.user.id = "starter"
	session, err := m.sm.StartSession(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	// the update loop's first tick starts the first pomodoro - wait so test writes don't race it (no busy_timeout)
	for deadline := time.Now().Add(time.Second); ; time.Sleep(10 * time.Millisecond) {
		if s, _ := m.sm.GetSession("t"); s.Record.CurrentInterval == pomomo.PomodoroInterval {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("session never started its first pomodoro")
		}
	}

	// a participant who left voice and was detached
	if _, err := m.pm.Insert(ctx, pomomo.ParticipantRecord{SessionID: session.ID, GuildID: "g", VoiceCID: "v", UserID: "detached"}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.pm.DetachFromChannel(ctx, "detached", "v"); err != nil {
		t.Fatal(err)
	}

	completePomodoro := func() {
		t.Helper()
		s, unlock := m.sm.cache.Get("t")
		defer unlock()
		s.Record.IntervalStartedAt = s.Record.IntervalStartedAt.Add(-s.Record.TimeRemainingAtStart - time.Second)
		if err := m.sm.updateSession(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	completePomodoro()
	// break ends - nothing to credit
	completePomodoro()

	starter, err := m.stats.GetUserStats(ctx, "g", "starter", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if starter.Pomodoros != 1 || starter.FocusTime != 20*time.Minute {
		t.Errorf("starter should be credited once, got %+v", starter)
	}
	if detached, _ := m.stats.GetUserStats(ctx, "g", "detached", time.Now()); detached.Pomodoros != 0 {
		t.Errorf("detached participant shouldn't be credited, got %+v", detached)
	}
	if stored, _ := m.repo.GetSession(ctx, session.ID); stored.CompletedPomodoros != 1 {
		t.Errorf("completed pomodoros not persisted, got %d", stored.CompletedPomodoros)
	}

	// skips aren't credited
	if _, err := m.sm.SkipInterval(ctx, "t"); err != nil {
		t.Fatal(err)
	}
	if starter, _ := m.stats.GetUserStats(ctx, "g", "starter", time.Now()); starter.Pomodoros != 1 {
		t.Errorf("skipped pomodoro shouldn't be credited, got %+v", starter)
	}
}

func TestRestoreSessionsDoesNotCreditCatchUp(t *testing.T) {
	m := newTestManagers(t)
	ctx := t.Context()
	// session that was mid-pomodoro 50 minutes ago, while the bot was down
	inserted, err := m.repo.InsertSession(ctx, pomomo.SessionRecord{
		GuildID: "g", TextCID: "t", VoiceCID: "v", MessageID: "m",
		IntervalStartedAt: time.Now().Add(-50 * time.Minute), TimeRemainingAtStart: 20 * time.Minute,
		CurrentInterval: pomomo.PomodoroInterval, Status: pomomo.SessionRunning,
	})
	if err != nil {
		t.Fatal(err)
	}
	settings := testSettings
	settings.SessionID = inserted.ID
	if _, err := m.repo.InsertSettings(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if _, err := m.pm.Insert(ctx, pomomo.ParticipantRecord{SessionID: inserted.ID, GuildID: "g", VoiceCID: "v", UserID: "u"}); err != nil {
		t.Fatal(err)
	}

	if err := m.sm.RestoreSessions(ctx); err != nil {
		t.Fatal(err)
	}
	s, err := m.sm.GetSession("t")
	if err != nil {
		t.Fatal(err)
	}
	if s.Record.CompletedPomodoros == 0 {
		t.Error("catch-up should still advance the pomodoro count")
	}
	if u, _ := m.stats.GetUserStats(ctx, "g", "u", time.Now()); u.Pomodoros != 0 {
		t.Errorf("catch-up pomodoros shouldn't be credited, got %+v", u)
	}
}
