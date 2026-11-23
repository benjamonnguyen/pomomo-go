# M02 Follow-up 01 — Verification

Automated coverage: `go vet` and `go test -race ./...` pass at every M02 change. The tests cover:

- migrations (up, plus a one-off down/up check)
- session stats persistence
- stats queries: windows, idempotent inserts, leaderboard ranking and limit, server totals
- crediting: attached vs detached participants, break transitions, skips, restore catch-up
- streak tiers, duration formatting, markdown escaping
- leaderboard assembly: departed users skipped, partial on lookup error, no mentions
- `/stats` scope default and ephemerality
- `Adapter.DisplayName` against a fake REST server (nickname → global name → username)
- the end-message summary

Nothing has run against Discord yet.

## Recommended verification (single dev bot, test guild)

Run: `POMOMO_CONFIG_PATH=$HOME/.pomomo/.env.dev go run ./cmd/bot`. Register first: `POMOMO_CONFIG_PATH=... go run ./cmd/register`.

| #   | Status | Check | How | Pass criteria |
| --- | ------ | ----- | --- | ------------- |
| V01 | | Registration | `go run ./cmd/register` | Prints `start: <id>` and `stats: <id>`; `/stats` shows a `scope` picker with `me` / `server` |
| V02 | | Migrations on an existing DB | Start the bot against a copy of a prod DB | Starts cleanly; existing sessions restore with `completed_pomodoros = 0` |
| V03 | | Crediting | `/start pomodoro:1 short_break:1`, wait for a pomodoro to complete | A `focus_intervals` row for each participant still in the voice channel; the interval counter advances |
| V04 | | Interval count survives restart | Restart the bot mid-session | "Interval x \| n" keeps its value after restore |
| V05 | | `/stats` | Run with no option, then with `scope:me` | Ephemeral "Your stats" with focus time, pomodoros, active days, last active (relative timestamp) |
| V06 | | `/stats scope:server` | Run it | Public "This week's leaderboard" with server display names (nickname if set; markdown characters render literally), nobody pinged, totals line |
| V07 | | Departed member omitted | Credit a pomodoro to an alt account, kick it, run `/stats scope:server` | The alt isn't listed; the next user moves up |
| V08 | | End message | Press **End** after at least one pomodoro | Farewell, `**Session complete** · <duration> · N 🍅`, then the weekly leaderboard |
| V09 | | Skip isn't credited | Press **Skip** during a pomodoro | No new `focus_intervals` row for that pomodoro |
| V10 | | Streak markers | Insert rows across days for a test user (sqlite on a dev DB copy) | 🔥 at 4/7, 🔥🔥 at 16/30, 🔥🔥🔥 at 64/90 |

## Deploy notes

- Back up both DBs with the bots stopped. Migrations 000004 and 000005 run on startup.
- **Rolling back to a pre-M02 image requires restoring that backup.** golang-migrate refuses to start when the DB version (5) isn't in the image's migrations.
- Run `cmd/register` with each bot's token so `/stats` shows up. Global commands can take a while to propagate.

## Known gaps / candidates for follow-up

- **No SQLite `busy_timeout`:** two concurrent write transactions fail immediately with `SQLITE_BUSY` instead of waiting. The M02 tests hit this when a write raced the update loop's tick. In prod, a Join or Skip landing at the same moment as a tick can fail the same way. Fix: add `_pragma=busy_timeout(5000)` to the DB URL (config) or in deadsimple's `Open`.
- **Restored sessions count downtime:** `StartedAt` comes from the session row's `created_at`, so their end-message duration includes the time the bot was down.
- **Lag catch-up is credited:** when the update loop lags (e.g. host suspend), `updateSession` advances one interval per tick and credits each pomodoro it passes. Restore catch-up isn't affected.
- **Zero-minute pomodoros** (`/start pomodoro:0`) are never credited.
- **No retention:** stats are kept after the bot leaves a guild, and there's no way to delete a user's stats.
