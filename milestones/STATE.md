# State

**Current milestone:** M02 — `/stats` command (plan final 2026-11-23, executing)
**Position:** M02 tasks are being executed in jj changes above the M01 triage stack; see `milestones/M02/TASKS.md`. Neither M02 nor the 2026-11-22 triage stack is pushed or deployed.

## Deploy / rollback

- Running: `ghcr.io/benjamonnguyen/pomomo-bot:latest` = `:m01-fixes-2026-11-20` (digest sha256:45ec3168ffdb…), built from `main` @ `1999914e`.
- Rollback options on starling: `:m01-disgo` (b304a023a2fa, the M01 build without the fixes) or `:pre-m01` (02e698df90b5, discordgo). To roll back: `docker tag …:<tag> …:latest && docker compose up -d` in `~/stacks/pomomo-bot`. Don't `pull`, or `:latest` comes back from ghcr.
- DB backups: `backups/*-2026-11-20-1931-pre-m01-fixes.db` (taken with the bots stopped) and `backups/bot{0,1}-2026-11-19-1938-pre-m01.db`.
- **M02 adds migrations 000004 and 000005.** An older image refuses to start on a migrated DB, because golang-migrate can't find the DB's version in its source. So rolling back past M02 also means restoring the pre-deploy DB backup. Back up with the bots stopped before deploying.

## Decisions

- M01: disgo v0.20.0, dave-go v0.5.1 (golibdave fallback in FOLLOWUP-01), `go` 1.26.0, `golang:1.26-alpine`, `CGO_ENABLED=0`; adapter package `disgo/`; async event dispatch; one voice conn per guild per session.
- M02 (details in `M02/PLAN.md`):
  - Stats are per server. A user gets credit only for full pomodoros completed while attached to the session's voice channel.
  - Active days are UTC days; windows are rolling 7/30/90 days.
  - Streak tiers: 🔥 at 4+ days in the week, 🔥🔥 at 16+ in the month, 🔥🔥🔥 at 64+ in the quarter.
  - `/stats me` is ephemeral; `/stats server` is a public weekly top 5 with plain usernames, and departed members are omitted.
  - The end message shows duration, pomodoros, and the leaderboard. There's no per-participant message (Discord can't send unsolicited ephemerals).

## Blockers

- None.

## Pending deploy: prod triage 2026-11-22 (on top of `main`, below M02)

- `tpltzpox` disconnect detach / no 40032 · `onqmmktq` clearer user-facing errors · `znukpxmt` detach participants no longer in voice (guild-loaded guard) · `womypomz` end unreachable or left-guild sessions · `uzqqzxrr` docs · `tzmprzyv` log guild add/remove (home-server report prompt updated in jj change `qktuxkso`, not committed).
- Orphaned prod session `8513f4a5` ends about 1 minute after this deploys (`ended unreachable session`).
- To verify: V20–V25 in FOLLOWUP-01. Still open from M01: V03, V06, V15–V18.

## Context

- Unexplained 2026-11-19 ops events (double start at 20:27, ungraceful recreate at 20:37) were never root-caused. Check `docker logs` and `journalctl -u docker` on starling if they recur.
- Local test image `pomomo-bot:m01-test` exists (not pushed); safe to `docker rmi`.
- The sandbox's Go build cache is read-only: use `GOCACHE=$TMPDIR/gocache`.
