# State

**Current milestone:** M01 — Migrate from discordgo to disgo
**Position:** M01 deployed to prod on 2026-11-19 19:39. The triage fixes were deployed on 2026-11-20 19:31 (starling.lan, both shards), without dev verification. Both shards came up cleanly with 0 sessions to restore. Next: watch prod logs and run the FOLLOWUP-01 checks in prod (V03, V06, V15–V18).

## Deploy / rollback

- Running: `ghcr.io/benjamonnguyen/pomomo-bot:latest` = `:m01-fixes-2026-11-20` (digest sha256:45ec3168ffdb…), built from `main` @ `1999914e`.
- Rollback options on starling: `:m01-disgo` (b304a023a2fa, the M01 build without the fixes) or `:pre-m01` (02e698df90b5, discordgo). To roll back: `docker tag …:<tag> …:latest && docker compose up -d` in `~/stacks/pomomo-bot`. Don't `pull`, or `:latest` comes back from ghcr.
- DB backups: `backups/*-2026-11-20-1931-pre-m01-fixes.db` (taken with the bots stopped) and `backups/bot{0,1}-2026-11-19-1938-pre-m01.db`. No schema changes.

## Decisions

- Target library: `github.com/disgoorg/disgo` v0.20.0 (chosen over cherry-picking discordgo PR #1694 alone, and over `sfossl/gda` which lacks voice + state cache).
- DAVE: pure-Go `thomas-vilte/dave-go` first; fall back to cgo `golibdave` on a Debian-slim image if it fails verification (procedure in FOLLOWUP-01). **Currently shipped: dave-go v0.5.1.**
- No interim discordgo hotfix; stale-participant fix ships with M01.
- Image built locally from the parent dir and pushed manually; `../discordgo` dropped from the build.
- One voice conn per guild for the session's lifetime (not per alert).
- Verify with a single dev bot (`.env.dev`); no multi-shard testing.
- Pinned: disgo v0.20.0, snowflake v2.0.3, dave-go v0.5.1 (godave v0.3.0, mls-go v1.6.0); `go` 1.26.0, builder `golang:1.26-alpine`, `CGO_ENABLED=0`.
- Adapter package `disgo/` (package `disgo`); disgoorg's root module imported as `disgoorg` in `main.go`.
- Voice hooks use one `GuildVoiceStateUpdate` listener (old vs new channel) instead of Join/Move/Leave events.
- Async event dispatch enabled (handlers block on REST/DB).

## Blockers

- None.

## Prod triage (report for 2026-11-19 UTC)

- Errors before the 19:39 deploy come from the old discordgo build (line numbers match `3626910`): UNIQUE on original participant, shutdown timeout, EndSession "not found". The stale-participant fix covers the UNIQUE error.
- Fixes on the M01 build. All are done and deployed on 2026-11-20. Build, vet, and `go test -race ./...` pass.
  - `nnlxrqnr`: the sender now gates DAVE audio on `ShouldHoldFrames` instead of `Ready()`.
  - `uultvtvk`: `Shutdown()` unlocks `cacheMu` before `wg.Wait()`. The update loop checks `ctx.Err()` before each tick.
  - `umznrpom`: `sessionCache.Get` no longer waits on a session lock while holding `cacheMu`, and it re-checks the cache once it has the lock. `Remove` now releases the session lock. Before this, a tick racing `EndSession`, or a Join press during `/start`, could deadlock the whole cache. A temporary race test hung on the old code and passed on the new code. This may explain the ungraceful recreate at 20:37.
  - `xqkyvtul`: if inserting the starter as a participant fails, `StartSession` ends the session and returns an error, so the user sees "Failed to start session." Previously the session started and then ended silently on the first tick.
  - To verify in dev or prod: V03, V06, V15, V16, V17 in FOLLOWUP-01.
- Unexplained ops events: a double start at 20:27 and an ungraceful recreate at 20:37. Check `docker logs` (panics go to stderr, not the log file) and `journalctl -u docker` on starling.

## Prod report 2026-11-22 (assessed and fixed 2026-11-23, not deployed)

- bot0 spike (3698× `main.go:217` 50001, plus 10004 at `autoshush.go:36`): session `8513f4a5` is orphaned. The bot most likely lost access to the guild, so no leave events arrive, the participants are never removed, and the session never goes empty. Once the fixes are deployed, the restart restores it and it ends after 3 failed edits (`ended unreachable session`). If deploying is delayed, end it in the DB.
- Fix stack above the STATE change (build, vet, and `go test -race ./...` pass at each change):
  - `tpltzpox`: disconnects detach without attempting a restore (no more 40032 at `hooks.go:47`). The join hook ignores disconnects (`hooks.go:76`). 50013 on restore is logged at Warn.
  - `onqmmktq`: clearer Join/Start/Skip errors. Join while not in voice links the channel (bots can't pull users into voice). A failed move or a missing permission gets its own message. Join or Skip on an ended session no longer errors. Adds `disgo.ErrVoiceStateNotFound`.
  - `znukpxmt`: autoshush detaches participants whose voice state is missing. `GetVoiceState` only returns `ErrVoiceStateNotFound` once the guild is loaded (this avoids the startup race listed in FOLLOWUP).
  - `womypomz`: sessions end on `GuildLeave` (`EndGuildSessions`), and when timer-message edits fail with 10003/10004/10008, or with 50001 on 3 consecutive ticks. Logs use `textCID` instead of `channelID`=VoiceCID.
  - `tzmprzyv` (above the docs change): logs `added to guild` / `removed from guild` (disgo `GuildJoin`/`GuildLeave`, so outages aren't counted). The report prompt in `~/home-server/agent/pomomo-report.md` now counts these lines (jj change `qktuxkso` there, not committed).
- To verify: V20–V25 in FOLLOWUP-01. Not pushed or deployed.

## Context

- jj stack: M01 through T17–T19 is on `main` and deployed. Above it: prod triage → DAVE fix → shutdown fix → cache lock fix → starter-insert fix (`@`). All pushed to `origin/main` and deployed.
- Local test image `pomomo-bot:m01-test` exists (not pushed); safe to `docker rmi`.
