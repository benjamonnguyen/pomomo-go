# M03 Follow-up 01 — Verification

Automated coverage: `go vet` and `go test -race ./...` pass at the end of M03. The tests cover:

- `WeekStart` and `MonthStart`: Monday 00:00, Sunday 23:59, next Monday, year edge, non-UTC input
- `GetWeeklyStats`: per-day buckets across UTC midnight and the week boundaries, month and lifetime totals, rolling active days, empty user
- leaderboard ranking from Monday (last Sunday excluded) with rolling active days, and server totals windows (this week so far vs last week to the same point)
- the leaderboard's date range line
- `RenderRecap`: valid 1800×880 PNG for tiers 0–3, an empty week, and long, unsupported (CJK/emoji) and empty names; `drawable` glyph stripping
- the text fallback: title, streak, date range, totals; no counts or daily times
- `/stats me`: ephemeral, `weekly-recap.png` attached and referenced by a media gallery; text fallback when the upload fails
- `Messenger.FollowupWithFiles` against a fake REST server (multipart with `payload_json` and `files[0]`)

Render benchmark: about 65 ms and 35 MB allocated per card (`go test ./image -bench .`). Review PNGs can be written with `POMOMO_RECAP_OUT=<dir> go test ./image`.

Nothing has run against Discord yet.

## Recommended verification (single dev bot, test guild)

Run: `POMOMO_CONFIG_PATH=$HOME/.pomomo/.env.dev go run ./cmd/bot`. No re-registration needed; the commands are unchanged.

| #   | Status | Check | How | Pass criteria |
| --- | ------ | ----- | --- | ------------- |
| V01 | | Card renders in Discord | `/stats` (no option) | Only you see the reply; it's a single image showing your server nickname, this Monday–Sunday range, today's label highlighted |
| V02 | | Card on mobile and light theme | Open the same reply on a phone, and with Discord's light theme | Text is legible when scaled down; the dark card looks intentional on light theme |
| V03 | | Fresh user | `/stats` from an account with no pomodoros | Card with empty bars, `0m` totals, "A fresh week is a fresh start.", `NO STREAK YET` |
| V04 | | Crediting shows up | Complete a pomodoro (`/start pomodoro:1`), then `/stats` | Today's bar and "This week", "This month" and "Lifetime" grow by 1m |
| V05 | | Odd names | Set a nickname with CJK/emoji (e.g. `トマト 🍅 Fan`) and one with markdown (`*a_b*`) | No empty boxes on the card; markdown characters drawn literally |
| V06 | | Leaderboard week | `/stats scope:server` | Header has `-# <Mon> – <Sun> UTC`; only time since Monday 00:00 UTC is ranked |
| V07 | | Server total comparison | On a dev DB copy with last week's data, run `/stats scope:server` | "up/down X% from last week" compares with last week up to the same weekday and time |
| V08 | | Streaks stay rolling | User with 4+ active days in the last 7 days, some before Monday | 🔥 on both the card and the leaderboard on Monday |
| V09 | | Text fallback | Temporarily break the upload (e.g. point the image URL at a missing attachment name in a local build) | The text recap arrives instead: title with fires, date range, totals |

## Deploy notes

- No migrations and no command changes. A deploy is a plain image swap; rolling back to the M02 image needs no DB restore.
- The image grows by about 2 MB (fonts).

## Known gaps / candidates for follow-up

- Names in scripts Noto Sans lacks (CJK, emoji) are stripped. Add Noto Sans CJK (+16 MB) if users hit it.
- Weekly numbers reset at Monday 00:00 UTC, which is Sunday afternoon or evening in the Americas. User timezones are out of scope.
- The card's bars use the Monday week while 🔥 uses rolling windows, so a Monday can show 🔥 with a single bar.
- Still open from M02: no SQLite `busy_timeout`.
