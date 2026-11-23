# Phase 1

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T01 | Done | Migrations: `completed_pomodoros` and `skips` columns on `sessions`; `focus_intervals` table and indexes | | Use `migrate create -ext sql -dir cmd/bot/migrations -seq <name>` |

# Phase 2

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T02 | Done | Persist session stats: move `CompletedPomodoros`/`Skips` into `pomomo.SessionRecord`, read and write them in the session repo, add `models.Session.StartedAt` | T01 | Drop `models.SessionStats` |
| T03 | Done | Stats repo in `sqlite/`: `InsertFocusIntervals` (insert or ignore), `GetUserStats`, `GetLeaderboard` (with active days per user), `GetServerTotals` | T01 | Rolling UTC windows of 7/30/90 days, including today |

# Phase 3

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T04 | Done | Credit attached participants on pomodoro → break in `sessionManager.updateSession`, in the same transaction | T02, T03 | Restore catch-up, skips, and detached participants aren't credited |
| T05 | Done | Stats rendering: streak tiers, duration format, `me` view, leaderboard view; display name lookup (`GetMember`, skip 10007) | T03 | Escaped server display names, no mentions |

# Phase 4

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T06 | Done | `/stats [scope]` command: definition, registration in `cmd/register`, handler (`me` ephemeral, `server` public) | T05 | Done |
| T07 | Done | End message: duration and pomodoros plus the weekly leaderboard; remove stats TODOs | T02, T04, T05 | The message is sent without the leaderboard if building it fails |
