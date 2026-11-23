# State

**Current milestone:** M03 — weekly recap card for `/stats me`; Monday weeks for weekly stats
**Position:** M03 was deployed to prod on 2026-11-23 at 22:31 UTC (starling.lan, both shards) from `main` @ `950d0758`, together with the pending `ptxrtppm`, `kpnqwpkx` and `vttysqxv`. Both containers came up with no restarts, and bot1 restored its 2 running sessions. It has not been verified on the dev bot or in Discord. Next:
- run `/stats` (card) and `/stats scope:server` in prod, using M03 FOLLOWUP-01 V01–V09 as the checklist;
- watch the logs for `failed to send recap card`.

Still open from M02: M02 FOLLOWUP-01 V03–V10 in prod, plus M01 FOLLOWUP-01 V20–V25. `/stats` is registered globally (`stats: 1574500834404868138`); M03 changed no commands.

## Deploy / rollback

- Running: `ghcr.io/benjamonnguyen/pomomo-bot:latest` = `:m03-2026-11-23` (digest sha256:3f5c15bc60c0…), built from `main` @ `950d0758`. ghcr's `:latest` is the same image.
- Rollback options on starling:
  - `:m02-2026-11-23` (6c391cd590d8, the previous prod; no DB restore needed, because M03 has no migrations);
  - `:m01-fixes-2026-11-20` (45ec3168ffdb), `:m01-disgo` (b304a023a2fa) or `:pre-m01` (02e698df90b5).
  - To roll back: `docker tag …:<tag> …:latest && docker compose up -d` in `~/stacks/pomomo-bot`. Don't `pull`.
- DB backups: `backups/bot{0,1}-2026-11-23-2231-pre-m03.db` and `*-2026-11-23-1903-pre-m02.db` (both taken with the bots stopped; the pre-M02 one is needed for any rollback past M02), plus the earlier `*-2026-11-20-1931-pre-m01-fixes.db` and `*-2026-11-19-1938-pre-m01.db`.
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

## M03 changes (deployed 2026-11-23 22:31 UTC)

- `xrwltxmu` plan · `zuqmulwo` T02 `image` package · `sztxxkrs` T01 Monday weeks · `syxkqwqk` T03 followup files · `rzxumlks` T04 `GetWeeklyStats` · `zyvylluk` T05 text fallback · `prnqvnzu` T06 `/stats me` card · `wpwsuwnt` T07 follow-up and state · `rqyntsos` best day removed from the card · `uwqzyxln` card simplified (no name, badge, caption or footer; `Your Pomomo Stats`; no "UTC").
- New dependency `golang.org/x/image` (plus `x/text`, indirect). `go mod tidy` needs the sandbox disabled, because the module cache is read-only.

## Context

- Unexplained 2026-11-19 ops events (double start at 20:27, ungraceful recreate at 20:37) were never root-caused. Check `docker logs` and `journalctl -u docker` on starling if they recur.
- Local test image `pomomo-bot:m01-test` exists (not pushed); safe to `docker rmi`.
- The sandbox's Go build cache is read-only: use `GOCACHE=$TMPDIR/gocache`.
