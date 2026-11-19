# State

**Current milestone:** M01 — Migrate from discordgo to disgo
**Position:** Plan finalized; `milestones/M01/TASKS.md` generated (T01–T19, 4 phases). Next: execute Phase 1.

## Decisions

- Target library: `github.com/disgoorg/disgo` v0.20.0 (chosen over cherry-picking discordgo PR #1694 alone, and over `sfossl/gda` which lacks voice + state cache).
- DAVE: pure-Go `thomas-vilte/dave-go` first; fall back to cgo `golibdave` on a Debian-slim image if it fails verification.
- No interim discordgo hotfix; stale-participant fix ships with M01.
- Image built locally from the parent dir and pushed manually; `../discordgo` dropped from the build.
- One voice conn per guild for the session's lifetime (not per alert).
- Verify with a single dev bot (`.env.dev`); no multi-shard testing.

## Blockers

- None.

## Context

- Unreleased change `qrlrrwyu` "fix stale participants blocking new sessions" (parent of the M01 planning change) — not yet deployed to starling.lan.
- Prod remains on discordgo until M01 deploys; gateway deadlocks may still need manual restarts.
