# State

**Current milestone:** M01 — Migrate from discordgo to disgo
**Position:** All phases (T01–T19) implemented and **deployed to prod 2026-11-19 19:39 (starling.lan, both shards)** at the user's request, skipping dev verification. Both shards connected and restored cleanly; no interactions observed yet. Next: watch prod logs / run FOLLOWUP-01 checks in prod (V03, V06, V18 first).

## Deploy / rollback

- Image: `ghcr.io/benjamonnguyen/pomomo-bot:latest` = `:m01-disgo` (sha256:fc892ef91bf0…, local id; starling container image b304a023a2fa…).
- Rollback image on starling: `ghcr.io/benjamonnguyen/pomomo-bot:pre-m01` (02e698df90b5, discordgo build from 2026-02-05). Rollback: `docker tag …:pre-m01 …:latest && docker compose up -d` in `~/stacks/pomomo-bot` (no `pull`).
- DB backups taken right before deploy: `~/stacks/pomomo-bot/backups/bot{0,1}-2026-11-19-1938-pre-m01.db`. No schema changes in M01.

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
- Fixes on the M01 build. All are done and none are deployed. Build, vet, and `go test -race ./...` pass.
  - `nnlxrqnr`: the sender now gates DAVE audio on `ShouldHoldFrames` instead of `Ready()`.
  - `uultvtvk`: `Shutdown()` unlocks `cacheMu` before `wg.Wait()`. The update loop checks `ctx.Err()` before each tick.
  - `umznrpom`: `sessionCache.Get` no longer waits on a session lock while holding `cacheMu`, and it re-checks the cache once it has the lock. `Remove` now releases the session lock. Before this, a tick racing `EndSession`, or a Join press during `/start`, could deadlock the whole cache. A temporary race test hung on the old code and passed on the new code. This may explain the ungraceful recreate at 20:37.
  - `xqkyvtul`: if inserting the starter as a participant fails, `StartSession` ends the session and returns an error, so the user sees "Failed to start session." Previously the session started and then ended silently on the first tick.
  - To verify in dev or prod: V03, V06, V15, V16, V17 in FOLLOWUP-01.
- Unexplained ops events: a double start at 20:27 and an ungraceful recreate at 20:37. Check `docker logs` (panics go to stderr, not the log file) and `journalctl -u docker` on starling.

## Context

- jj stack: M01 through T17–T19 is on `main` and deployed. Above it: prod triage → DAVE fix → shutdown fix → cache lock fix → starter-insert fix (`@`). None of these are deployed.
- Local test image `pomomo-bot:m01-test` exists (not pushed); safe to `docker rmi`.
