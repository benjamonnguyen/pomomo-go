# M03 Follow-up 01 — Verification

Automated coverage: `go vet` and `go test -race ./...` pass at the end of M03. The tests cover:

- `WeekStart` and `MonthStart`: Monday 00:00, Sunday 23:59, next Monday, year edge, non-UTC input
- `GetWeeklyStats`: per-day buckets across UTC midnight and the week boundaries, month and lifetime totals, rolling active days, empty user
- leaderboard ranking from Monday (last Sunday excluded) with rolling active days, and server totals windows (this week so far vs last week to the same point)
- the leaderboard's date range line (no "UTC")
- `RenderRecap`: valid 1800×764 PNG for tiers 0–3, an empty week, and an out-of-range tier
- the text fallback: `Your Pomomo Stats`, streak, date range, totals; no counts, daily times or "UTC"
- `/stats me`: ephemeral, `pomomo-stats.png` attached and referenced by a media gallery; text fallback when the upload fails
- `Messenger.FollowupWithFiles` against a fake REST server (multipart with `payload_json` and `files[0]`)

Render benchmark: about 65 ms and 35 MB allocated per card (`go test ./image -bench .`). Review PNGs can be written with `POMOMO_RECAP_OUT=<dir> go test ./image`.

Nothing has run against Discord yet.

## Recommended verification (single dev bot, test guild)

Run: `POMOMO_CONFIG_PATH=$HOME/.pomomo/.env.dev go run ./cmd/bot`. No re-registration needed; the commands are unchanged.

| #   | Status | Check | How | Pass criteria |
| --- | ------ | ----- | --- | ------------- |
| V01 | | Card renders in Discord | `/stats` (no option) | Only you see the reply; it's a single image titled "Your Pomomo Stats" with this Monday–Sunday range and today's label highlighted |
| V02 | | Card on mobile and light theme | Open the same reply on a phone, and with Discord's light theme | Text is legible when scaled down; the dark card looks intentional on light theme |
| V03 | | Fresh user | `/stats` from an account with no pomodoros | Card with empty bars, `0m` totals, three grey fires |
| V04 | | Crediting shows up | Complete a pomodoro (`/start pomodoro:1`), then `/stats` | Today's bar and "This week", "This month" and "Lifetime" grow by 1m |
| V06 | | Leaderboard week | `/stats scope:server` | Header has `-# <Mon> – <Sun>`; only time since Monday 00:00 UTC is ranked |
| V07 | | Server total comparison | On a dev DB copy with last week's data, run `/stats scope:server` | "up/down X% from last week" compares with last week up to the same weekday and time |
| V08 | | Streaks stay rolling | User with 4+ active days in the last 7 days, some before Monday | 🔥 on both the card and the leaderboard on Monday |
| V09 | | Text fallback | Temporarily break the upload (e.g. point the image URL at a missing attachment name in a local build) | The text version arrives instead: `Your Pomomo Stats` with fires, date range, totals |

## Deploy notes

- No migrations and no command changes. A deploy is a plain image swap; rolling back to the M02 image needs no DB restore.
- The image grows by about 2 MB (fonts).

## Known gaps / candidates for follow-up

- Weekly numbers reset at Monday 00:00 UTC (the card and leaderboard don't say "UTC"), which is Sunday afternoon or evening in the Americas. User timezones are out of scope.
- The card's bars use the Monday week while 🔥 uses rolling windows, so a Monday can show 🔥 with a single bar.
- Still open from M02: no SQLite `busy_timeout`.
