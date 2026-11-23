# Milestone 2 — `/stats` command

**Goal:** Persist per-user focus stats per server, and expose them through `/stats [scope]` with the choices `me` (default) and `server`. `me` shows your own stats; `server` shows a weekly leaderboard with streak 🔥 markers.

**Status:** Final (2026-11-23).

## Background

- Nothing is tracked today:
  - `models.SessionStats{CompletedPomodoros, Skips}` is in memory only. It is never written to the DB and resets to 0 on restart. That also resets the "Interval x | n" display and the long-break cadence of restored sessions.
  - Rows in `session_participants` are deleted when a user leaves or the session ends, so there is no per-user history.
  - Rows in `session_settings` are deleted when a session ends, so durations are lost.
  - Rows in `sessions` survive as `status=ended`.
  - Placeholders exist: `// TODO persist participant stats` (`cmd/bot/main.go`, afterUpdate), `// TODO display stats in end message` (`cmd/bot/disgo.go`), and `Participant.StartedIntervalAt` (unused).
- Each bot (shard) has its own DB, and a guild only ever lives on one shard, so per-server stats are complete.

## Decisions

- **Option shape:** `/stats` takes an optional string option `scope` with the choices `me` (default) and `server`. Discord shows the choices as a picker, so no `srv` alias is needed.
- **Scope:** stats are per server. `me` means "me in this server".
- **Crediting:** a participant gets credit only for full pomodoros. Partial, skipped, and restore catch-up pomodoros aren't credited.
- **No backfill:** everyone starts at 0.
- **Days and windows:** an *active day* is a UTC day with at least one credited pomodoro. Windows are rolling and include today: week = last 7 days, month = last 30, quarter = last 90. Calendar windows would wipe everyone's streaks on the 1st.
- **Hot streaks:** one marker per user, showing the highest tier reached:
  - 🔥 for 4+ active days in the week
  - 🔥🔥 for 16+ active days in the month
  - 🔥🔥🔥 for 64+ active days in the quarter
- **`/stats me`** (ephemeral): all-time focus time, pomodoros, and sessions joined; active days this week, month, and quarter; the streak marker; and last active. With no data, the reply says "No pomodoros yet — join a session to start tracking."
- **`/stats server`** (public): leaderboard of the top 5 for the week, ranked by focus time. Each row shows rank, server display name, focus time, pomodoros, and the streak marker. Discord markdown has no tables, so rows are list lines. Below the table: server totals for the week (focus time, pomodoros, sessions, active users). With no data, the reply says "No pomodoros this week."
- **Names, not mentions:** the leaderboard shows server display names (nickname, then global display name, then username; markdown-escaped). It never uses `<@id>` mentions, so nobody gets pinged.
- **Departed users are omitted** from the leaderboard. The top 5 is filled from the next-ranked users who are still members.
- **End message:** the farewell message shows the session's duration and completed pomodoros, followed by the server's weekly leaderboard (the same component as `/stats server`). No per-participant message.

## Design

- **New table `focus_intervals`**, one row per participant per completed pomodoro: `id, guild_id, user_id, session_id, duration (seconds), completed_at`.
  - Indexes on `(guild_id, user_id, completed_at)` and `(guild_id, completed_at)`.
  - Each row also stores `pomodoro` (the session's pomodoro number), with a unique key on `(session_id, user_id, pomodoro)`. Writes use `INSERT OR IGNORE`, so crediting is idempotent.
  - No foreign key to `sessions`, so stats survive if a session row is ever deleted.
- **`sessions` gets `completed_pomodoros` and `skips` columns**, written by the existing `UpdateSession`. They move from `models.SessionStats` to `pomomo.SessionRecord`, so restored sessions keep their count and long-break cadence.
- **`models.Session.StartedAt`** comes from the session row's `created_at` and is used for the end-message duration.
- **Crediting** happens in `sessionManager.updateSession` on the pomodoro → break transition, inside the same transaction as the session update. Every participant attached to the session's voice channel at that moment gets a row. It is not done in afterUpdate, because afterUpdate is skipped when the previous call is still running (`updateMu.TryLock`).
- **Restore catch-up:** `RestoreSessions` advances intervals without crediting anyone. It still increments `completed_pomodoros`, so the long-break cadence stays correct.
- **Queries:** a `StatsRepo` in `sqlite/` runs aggregate SQL. Active days use `COUNT(DISTINCT date(completed_at, 'unixepoch'))`.
- **Command:** `pomomo.StatsCommand` is defined in `commands.go` and registered via `cmd/register`. The handler is `Stats` in `discord_event_hooks.go`. Replies use Components v2, matching the session message.
- Migrations are created with `migrate create -ext sql -dir cmd/bot/migrations -seq <name>`.
- **Display names:** the bot has no member cache (no privileged `GuildMembers` intent). The leaderboard query returns the top 25 users by focus time. Members are then fetched over REST with `GetMember` until 5 current members are found. 10007 Unknown Member means the user left, so they're skipped; other errors also skip the row, with a log line.
- **Shared components:** the leaderboard and the `me` view are each built by one function, used by both `/stats` and the end message.

## End message

> Session complete · 1h 40m · 3 🍅
> *(weekly leaderboard)*

- Duration is the time from `StartedAt` to the end. Pomodoros come from `completed_pomodoros`.
- If building the leaderboard fails, the message is sent without it.
- The farewell message is rendered after the session has been credited, so the leaderboard includes this session.

## Pitfalls

- **Name lookups:** at least 5 REST calls per leaderboard render, plus one for each departed user skipped. That's fine at current volume; cache names in memory if it becomes a problem.
- **Detached participants** (`VoiceCID == ""`) must not be credited.
- **Restore catch-up:** a session restored after downtime must not credit the missed intervals. Add a test for this.
- **Bot removed from a server:** its stats stay in the DB. No retention or deletion handling in this milestone.

## Scope

- In: migrations; crediting and session stats persistence; `StatsRepo`; the `/stats` command, its registration, and both views with streaks; the end message (duration, pomodoros, leaderboard); tests for crediting edge cases (restore catch-up, skip, detached, idempotency) and for streak tiers.
- Out: per-participant stats messages at session end (Discord can't send unsolicited ephemerals), cross-bot aggregation, time-range options, user timezones, deleting stats.
