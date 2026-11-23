# Milestone 3 — Weekly recap card and Monday weeks

**Goal:** `/stats me` replies with a rendered PNG "Weekly Recap" card showing an initial badge, the date range, focus time per day, streak fires, and totals. Weekly stats use calendar weeks that always start on Monday (UTC). Streaks stay rolling.

**Status:** Final (2026-11-23).

## Background

- **M02 windows:** M02 uses rolling windows everywhere (`pomomo.WindowStart`, `sqlite/stats.go`, `buildLeaderboard`): week = last 7 days, month = last 30, quarter = last 90.
- **Why a PNG:** Discord can't render charts in messages, so the card is a PNG attached to the reply.
- **Prototype:** the session scratchpad `card/` holds the renderer `card.go` and the reviewed test PNGs.
  - It renders an 1800×1080 card (900×540 logical, 2×) using `golang.org/x/image` (opentype, vector, draw).
  - It embeds Noto Sans Regular, SemiBold and Bold, plus a 🔥 bitmap taken from Noto Color Emoji.
- **Base:** builds on the unpushed `ptxrtppm`, `kpnqwpkx` and `vttysqxv` (1-based interval, medals, server total line).

## Decisions

- **Weekly periods are calendar periods, UTC:**
  - A week runs from Monday 00:00 to the next Monday 00:00. A month is the calendar month.
  - Add `pomomo.WeekStart(now)` and `pomomo.MonthStart(now)`.
  - These apply to the leaderboard ranking, the server total line, the card's days and tiles, and "This week" and "This month" totals.
- **Streaks stay rolling:** 🔥 is 4+ active days in the last 7 days, 🔥🔥 is 16+ in the last 30, and 🔥🔥🔥 is 64+ in the last 90. These are M02's `WindowStart` and `*Days` constants, unchanged. This applies to both the leaderboard markers and the card's fires.
- **Leaderboard:**
  - It ranks focus time since Monday 00:00 UTC.
  - The header gains the date range as a second line: `### This week's leaderboard`, then `-# Nov 23 – Nov 29 UTC`.
- **Server total line:**
  - The `Server total:` figure is this week so far.
  - It is compared with last week up to the same point. For example, Mon 00:00 → Wed 15:00 this week against Mon 00:00 → Wed 15:00 last week. Comparing against the whole of last week would read "down" every Monday.
- **`/stats me` card** (ephemeral, PNG attachment `weekly-recap.png`), laid out as in the prototype:
  - **Header:**
    - An initial badge (accent circle with the display name's first letter), then `<display name>'s Weekly Recap`, truncated with … when long.
    - `Mon, Nov 23 – Sun, Nov 29 UTC` below.
  - **Top right**, replacing the example's "POMOBOT / FOCUS. GROW. REPEAT.":
    - Three fire slots: earned tiers in color, unearned in grey.
    - A caption: `NO STREAK YET`, `4+ DAYS THIS WEEK`, `16+ DAYS THIS MONTH` or `64+ DAYS THIS QUARTER`.
  - **No stat tiles** (no pomodoros, sessions or active days); the card is shorter than the prototype (900×440 logical).
  - **Weekly Stats** (replacing "Daily focus"):
    - Mon–Sun bars scaled to the week's best day, with a 1h minimum scale. Each bar is labelled with its time.
    - Today's label is in the accent color; future days are dimmed.
  - **Total Focus** panel (replacing "Week highlights"): This week, This month, and Lifetime (whole hours, with a thousands separator).
  - **Footer:**
    - Left: `Best day: <weekday> · <time>`, or `A fresh week is a fresh start.` when the week is empty.
    - Right: `Pomomo`.
  - **Style:** dark only, with the tomato accent `#ED6A43`.
- **No avatar download:** the card never fetches avatars.
- **Names the font can't draw:**
  - Characters Noto Sans has no glyph for (CJK, emoji in nicknames) are stripped before drawing, so no empty boxes appear.
  - A name that ends up empty falls back to `Your Weekly Recap` with a `?` badge.
  - No fallback fonts for now.
- **Text fallback:** if rendering or sending the card fails, the reply is a short Components V2 text version: header, date range, and totals only. It has no daily focus times, no best day, no counts, and no last-active line:
  ```
  ### ben_jamon's Weekly Recap 🔥
  -# Mon, Nov 23 – Sun, Nov 29 UTC
  **Total focus**
  This week 4h 10m · This month 26h 0m · Lifetime 72h
  ```
  - The name is markdown-escaped.
  - This replaces M02's `UserStatsComponents`, so the "No pomodoros yet" reply is gone. An empty week still renders the card.
- **End message unchanged:** it still ends with the text leaderboard. The card is only for `/stats me`.

## Design

- **New root-level package `image`** (`github.com/benjamonnguyen/pomomo-go/image`, directory `image/`):
  - `Recap` is pure data: name, week start, now, per-day focus `[7]time.Duration`, streak tier (0–3), month focus, and lifetime focus.
  - `RenderRecap(Recap) ([]byte, error)` returns the PNG.
  - Assets are embedded with `go:embed` under `image/assets/`: the fonts, `fire.png`, and their license files.
  - No Discord or DB dependencies.
  - It shares its name with the standard library's `image`, so the bot imports it as `pomoimage`. Inside the package, the standard library is imported normally.
- **Model** (`stats_model.go`):
  - Add `WeekStart(now)` and `MonthStart(now)`.
  - Add `WeeklyStats`: `Days [7]time.Duration` from Monday, month and lifetime focus, and `ActiveDays` (rolling, for streaks).
- **Repo** (`sqlite/stats.go`):
  - `GetWeeklyStats(ctx, gid, uid, now)` replaces `GetUserStats`. It returns per-day sums for the Monday week (`GROUP BY date(completed_at,'unixepoch')`) and the month, lifetime and rolling active-day aggregates.
  - `GetLeaderboard` ranks from `WeekStart(now)`; its active days stay rolling.
- **`buildLeaderboard`:**
  - this week = `[WeekStart(now), now)`;
  - last week = `[WeekStart(now)-7d, now-7d)`.
- **Messenger:** `disgo.Followup` gains file attachments (e.g. `FollowupWithFiles(files []*discord.File, components ...)`, or a variadic option; decided at task time). It's sent with `IsComponentsV2` and a `MediaGallery` that references `attachment://weekly-recap.png`.
- **Handler:**
  - `Stats` for the `me` scope: get the display name, then the weekly stats, then build the `Recap`, then render it.
  - It sends the card as the deferred ephemeral followup. If rendering or sending fails, it sends the text fallback.
- **Dependencies and assets:**
  - New dependency: `golang.org/x/image`.
  - Noto Sans ×3 (OFL, about 1.6 MB) and `fire.png` from Noto Color Emoji (Apache-2.0), with license files.

## Pitfalls

- **Monday reset:** the leaderboard and "this week" numbers reset at Monday 00:00 UTC, which is Sunday afternoon or evening in the Americas.
- **Mixed windows:** the bars cover the Monday week, but 🔥 uses the last 7 days, so someone can have 🔥 on a Monday with only one bar. That's expected given rolling streaks.
- **Render cost:** about 50–100 ms of CPU and about 10 MB of allocations per card at 2×. That's fine at current volume; the defer allows 15 minutes.
- **Ephemeral attachments:** the card must be sent as the deferred followup, not as a separate message, or it won't be ephemeral.
- **Binary size:** grows by about 2 MB (fonts).
- **Package name:** `image` shadows the standard library in importers, so the bot always imports it with the `pomoimage` alias.

## Scope

- In:
  - Monday weeks and calendar months for weekly stats (repo, leaderboard, server total line, `/stats me`).
  - The `image` package and its assets.
  - Followup file attachments.
  - The `/stats me` card with its text fallback.
  - Tests:
    - week and month boundaries, including Sunday 23:59 → Monday 00:00 and year edges;
    - per-day sums;
    - a render smoke test (valid PNG, 1800×880, all streak tiers, long and unsupported names);
    - the handler attaching the file, plus its fallback;
    - leaderboard and week-over-week windows.
  - No schema change, so no migrations.
- Out:
  - avatars, user timezones, a light-theme card;
  - cards for `/stats server` or the end message;
  - history beyond the current week;
  - caching rendered cards;
  - CJK or emoji fallback fonts.
