package main

import (
	"path/filepath"
	"testing"

	"github.com/Thiht/transactor"
	txStdLib "github.com/Thiht/transactor/stdlib"
	dsdb "github.com/benjamonnguyen/deadsimple/db/sqlite"
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
