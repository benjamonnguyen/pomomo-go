package main

import (
	"path/filepath"
	"testing"

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
