# State

**Current milestone:** M02 — `/stats` command
**Position:** M02 and the 2026-11-22 triage stack were deployed to prod on 2026-11-23 at 19:03 UTC (starling.lan, both shards), from `main` @ `d8559c0e`. They have not been verified on the dev bot. `/stats` is registered globally (`stats: 1574500834404868138`). The orphaned session `8513f4a5` ended at 19:03:51 (`ended unreachable session`). Next: watch the prod logs and run the M02 FOLLOWUP-01 V03–V10 checks in prod, plus V20–V25 from M01 FOLLOWUP-01.
**Next milestone:** M03 — weekly recap card for `/stats me` and calendar weeks (Monday start, UTC) everywhere. `M03/PLAN.md` is final and `M03/TASKS.md` (T01–T07) is generated, in jj change `xrwltxmu`; execution has not started. Decisions: streaks stay rolling, weekly stats use Monday weeks and calendar months, there's a root-level `image` package (imported as `pomoimage`), no avatar download, and the text fallback mirrors the card without pomodoro, session, or active-day counts. The prototype renderer and the reviewed test PNGs are in the session scratchpad (`card/`, `pngs/`) and will be copied into `cmd/bot/recap` during execution.
**Pending deploy:** `ptxrtppm`, `kpnqwpkx` (leaderboard top 3 shown as 🥇🥈🥉) and `vttysqxv` (leaderboard rows drop pomodoro counts, the 🔥 marker moves right after the name, and the totals line becomes `Server total: Nh · up/down X% from last week`). In `ptxrtppm`, the session message's "Interval x | n" was 0-based (it showed `0 | 4` during the first pomodoro). It's now 1-based, and during a break it shows the pomodoro that was just completed (the long break shows `4 | 4`). Neither change is pushed yet.

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

## Context

- Unexplained 2026-11-19 ops events (double start at 20:27, ungraceful recreate at 20:37) were never root-caused. Check `docker logs` and `journalctl -u docker` on starling if they recur.
- Local test image `pomomo-bot:m01-test` exists (not pushed); safe to `docker rmi`.
- The sandbox's Go build cache is read-only: use `GOCACHE=$TMPDIR/gocache`.
