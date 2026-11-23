# State

**Current milestone:** M03 — weekly recap card for `/stats me`; Monday weeks for weekly stats
**Position:** M03 T01–T07 are implemented, and `go test -race ./...` passes. Nothing is pushed and nothing has run against Discord. Next:
- user review of the card PNGs (`POMOMO_RECAP_OUT=<dir> go test ./image`);
- dev-bot verification (M03 FOLLOWUP-01 V01–V09);
- push and deploy with the pending changes below. There are no migrations or command changes.

Still open from M02: watch the prod logs and run M02 FOLLOWUP-01 V03–V10 in prod, plus M01 FOLLOWUP-01 V20–V25. M02 has been in prod since 2026-11-23 19:03 UTC (from `main` @ `d8559c0e`); `/stats` is registered globally (`stats: 1574500834404868138`).
**Pending deploy:** `ptxrtppm`, `kpnqwpkx` (leaderboard top 3 shown as 🥇🥈🥉) and `vttysqxv` (leaderboard rows drop pomodoro counts, the 🔥 marker moves right after the name, and the totals line becomes `Server total: Nh · up/down X% from last week`). In `ptxrtppm`, the session message's "Interval x | n" was 0-based (it showed `0 | 4` during the first pomodoro). It's now 1-based, and during a break it shows the pomodoro that was just completed (the long break shows `4 | 4`). None of these are pushed yet; the M03 stack sits on top of them.

## Deploy / rollback

- Running: `ghcr.io/benjamonnguyen/pomomo-bot:latest` = `:m02-2026-11-23` (digest sha256:6c391cd590d8…), built from `main` @ `d8559c0e`. ghcr's `:latest` is the same image.
- Rollback options on starling: `:m01-fixes-2026-11-20` (45ec3168ffdb, the previous prod), `:m01-disgo` (b304a023a2fa) or `:pre-m01` (02e698df90b5). To roll back: `docker tag …:<tag> …:latest && docker compose up -d` in `~/stacks/pomomo-bot`. Don't `pull`.
- DB backups: `backups/bot{0,1}-2026-11-23-1903-pre-m02.db` (taken with the bots stopped; needed for any rollback past M02), plus the earlier `*-2026-11-20-1931-pre-m01-fixes.db` and `*-2026-11-19-1938-pre-m01.db`.
- **M02 adds migrations 000004 and 000005.** An older image refuses to start on a migrated DB, because golang-migrate can't find the DB's version in its source. So rolling back past M02 also means restoring the pre-deploy DB backup. Back up with the bots stopped before deploying.

## Decisions

- M01: disgo v0.20.0, dave-go v0.5.1 (golibdave fallback in FOLLOWUP-01), `go` 1.26.0, `golang:1.26-alpine`, `CGO_ENABLED=0`; adapter package `disgo/`; async event dispatch; one voice conn per guild per session.
- M02 (details in `M02/PLAN.md`):
  - Stats are per server. A user gets credit only for full pomodoros completed while attached to the session's voice channel.
  - Active days are UTC days; windows are rolling 7/30/90 days.
  - Streak tiers: 🔥 at 4+ days in the week, 🔥🔥 at 16+ in the month, 🔥🔥🔥 at 64+ in the quarter.
  - `/stats me` is ephemeral; `/stats server` is a public weekly top 5 with server display names (no mentions), and departed members are omitted.
  - The end message shows duration, pomodoros, and the leaderboard. There's no per-participant message (Discord can't send unsolicited ephemerals).

- M03 (details in `M03/PLAN.md`):
  - Weekly stats use Monday–Sunday UTC weeks and calendar months: the leaderboard ranking, the server total (compared with last week up to the same point), and the card. 🔥 streaks stay rolling.
  - `/stats me` is an ephemeral PNG card, `Your Pomomo Stats`, drawn by the root-level `image` package (imported as `pomoimage`). It has no name, avatar, tiles, streak caption or footer; fires fill from the left; there's no "UTC" in date ranges; and the panel reads `TOTAL FOCUS TIME`.
  - Text fallback: title with fires, date range, and This week · This month · Lifetime.

## Blockers

- None.

## Prod triage 2026-11-22 (deployed with M02)

- `tpltzpox` disconnect detach / no 40032 · `onqmmktq` clearer user-facing errors · `znukpxmt` detach participants no longer in voice (guild-loaded guard) · `womypomz` end unreachable or left-guild sessions · `uzqqzxrr` docs · `tzmprzyv` log guild add/remove (home-server report prompt updated in jj change `qktuxkso`, not committed).
- To verify: V20–V25 in FOLLOWUP-01. Still open from M01: V03, V06, V15–V18.

## M02 changes (above the triage stack)

- `kknmsput` plan/tasks · `lmwqtqvp` T01 migrations · `vlvrmxnu` T02 persist session stats · `kkmumwsv` T03 stats repo · `kktlpulp` T04 crediting (also fixes a `StartSession` data race on the returned session) · `lplntsll` T05 rendering and username lookup · `tsknxxvq` T06 `/stats` · `ovkuznnt` T07 end message · `mlsttwsy` follow-up and state · `rpqutnky` server display names on leaderboard.
- Deploy notes: `bot0` and `bot1` are two shards of one Discord app (both env files register the same command IDs), so one `cmd/register` run covers both. To register without the tokens leaving starling, build `cmd/register` for linux, scp it to starling, and run it with `POMOMO_CONFIG_PATH=~/stacks/pomomo-bot/.env.prod0`.
- Post-deploy log noise to clean up: `autoshush.go:71` logs `guild … not loaded` at ERRO for each restored participant on every restart (expected, since that's the startup guard); it should be Debug or Warn. Interactions that arrive while sessions are still restoring get "session not found".
- Open follow-up: no SQLite `busy_timeout` (concurrent writes fail with `SQLITE_BUSY`); see M02 FOLLOWUP-01.

## M03 changes (above the pending deploy changes)

- `xrwltxmu` plan · `zuqmulwo` T02 `image` package · `sztxxkrs` T01 Monday weeks · `syxkqwqk` T03 followup files · `rzxumlks` T04 `GetWeeklyStats` · `zyvylluk` T05 text fallback · `prnqvnzu` T06 `/stats me` card · `wpwsuwnt` T07 follow-up and state · `rqyntsos` best day removed from the card · `uwqzyxln` card simplified (no name, badge, caption or footer; `Your Pomomo Stats`; no "UTC").
- New dependency `golang.org/x/image` (plus `x/text`, indirect). `go mod tidy` needs the sandbox disabled, because the module cache is read-only.

## Context

- Unexplained 2026-11-19 ops events (double start at 20:27, ungraceful recreate at 20:37) were never root-caused. Check `docker logs` and `journalctl -u docker` on starling if they recur.
- Local test image `pomomo-bot:m01-test` exists (not pushed); safe to `docker rmi`.
- The sandbox's Go build cache is read-only: use `GOCACHE=$TMPDIR/gocache`.
