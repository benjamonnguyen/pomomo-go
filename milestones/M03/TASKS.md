# Phase 1

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T01 | Done | Monday weeks: add `pomomo.WeekStart` and `MonthStart`, plus `WeeklyStats`. `GetLeaderboard` ranks from `WeekStart`. `buildLeaderboard` compares `[WeekStart, now)` with `[WeekStart-7d, now-7d)`. The leaderboard header gains a date range line. | | Streak active days stay rolling (`WindowStart`). Boundary tests: Sun 23:59 → Mon 00:00, year edge. |
| T02 | Done | Root-level `image` package: `Recap`, `RenderRecap`; port the prototype; embed Noto Sans ×3, `fire.png` and license files; strip unsupported glyphs from names; add `golang.org/x/image` | | Smoke tests: valid 1800×880 PNG; tiers 0–3; empty week; long, unsupported and empty names |
| T03 | Done | Followup file attachments in `disgo/messenger.go` (Components V2 + `MediaGallery` referencing `attachment://…`) | | Keeps the existing `Followup` callers working |

# Phase 2

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T04 | | `GetWeeklyStats` in `sqlite/stats.go`: per-day sums for the Monday week; month and lifetime focus; rolling active days. Replaces `GetUserStats`. | T01 | Test per-day bucketing across UTC midnight and the week boundary |

# Phase 3

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T05 | | Text fallback: `RecapComponents(name, stats, now)`: header with fires, date range, and This week · This month · Lifetime. No daily times or best day. Replaces `UserStatsComponents`. | T04 | Markdown-escaped name |
| T06 | | `/stats me` handler: display name → `GetWeeklyStats` → `pomoimage.RenderRecap` → ephemeral followup with `weekly-recap.png`. Falls back to text on render or send failure. | T02, T03, T05 | Handler tests: file attached, fallback path |

# Phase 4

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T07 | | FOLLOWUP-01 (verification steps) and STATE update | T06 | |
