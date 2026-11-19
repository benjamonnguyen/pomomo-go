# M01 Follow-up 01 — Verification & fallback

Everything below is **unverified against Discord**. Automated coverage so far: `go build`, `go vet`, `go test -race ./...` (ID helpers, slog bridge, messenger HTTP payloads, voice adapter with fake Manager/Conn, voice before/after logic, session message JSON), static `CGO_ENABLED=0` build, local `docker build` that boots to config load.

## Recommended verification (single dev bot, test guild)

Run: `POMOMO_CONFIG_PATH=$HOME/.pomomo/.env.dev go run ./cmd/bot` (set `POMOMO_LOG_LEVEL=DEBUG` to see disgo/voice logs).

| #   | Status | Check | How | Pass criteria |
| --- | ------ | ----- | --- | ------------- |
| V01 | | Command registration | `POMOMO_CONFIG_PATH=... go run ./cmd/register` | Prints `start: <id>`; `/start` shows all 6 options with same descriptions/limits |
| V02 | | Gateway connect + restore | Start bot with an existing dev DB | `Pomomo running` logged; `restored participantMgr cache` / `restored pending sessions` as before; `disgo:` lines have a single timestamp |
| V03 | | `/start` | Run in a text channel while in a voice channel | Session message (greeting, green settings container, timer bar, Join/End/Skip) appears and is pinned; starter is server-muted/deafened per settings |
| V04 | | `/start` guards | Not in voice; second session in same guild; DM | Correct error text; DM no longer panics |
| V05 | | Timer loop | Wait ≥ 40s | Timer bar updates every ~20s |
| V06 | | **Interval alert (DAVE)** | Use a short pomodoro (`pomodoro:1 short_break:1`), listen with a current Discord client | Alert audible and clean at each interval change; no `timeout playing audio` / `4017` in logs |
| V07 | | Alert across intervals reuses conn | Let 2–3 intervals pass | Bot stays in voice between alerts (no leave/rejoin per alert) |
| V08 | | Alert after voice disruption | Move the bot to another channel / change voice region, then wait for next alert | Bot reconnects to the session channel and plays the alert |
| V09 | | DAVE epoch change | Have a second user join/leave voice mid-session | Next alert still audible for everyone |
| V10 | | Skip | Press Skip | Interval advances, message updates, no alert on skip |
| V11 | | Join | Second user (in another voice channel) presses Join | Moved into session channel, ephemeral "Joined session!", muted per settings |
| V12 | | Join twice | Same user presses Join again | "You are already in this session." |
| V13 | | Leave voice mid-session | Participant leaves voice | Removed (or detached if restore fails); on rejoining voice in that guild, original mute/deaf restored |
| V14 | | Stale participant (prior fix) | Seed a leftover participant row for the starter from an ended session, then `/start` | Session starts; log `replaced stale participant`; doesn't end immediately |
| V15 | | End | Press End | Farewell message, unpinned, participants' voice state restored, bot leaves voice |
| V16 | | Empty session auto-end | All participants leave voice | Session ends on next tick; bot leaves voice |
| V17 | | Graceful shutdown | Ctrl-C with an active session | Participants restored; exits < 10s with no `failed to shut down gracefully` |
| V18 | | Gateway resilience | Drop network for ~1–2 min (e.g. disable Wi-Fi) then restore | Bot reconnects by itself and keeps handling interactions — **the original outage** |
| V19 | | Docker image | Build from parent dir as usual, run with `.env.dev` | Same as V02/V06 inside the container |

## golibdave fallback (if V06/V08/V09 fail with dave-go)

Symptoms that warrant it: no or garbled audio, voice gateway closes with 4017/400x DAVE errors, `timeout playing audio, dave ready: false`, panics in `dave-go`.

1. `go get github.com/disgoorg/godave/golibdave@<version matching godave v0.3.0>`; drop `thomas-vilte/dave-go`.
2. `cmd/bot/main.go`: replace `voice.WithDaveSessionCreateFunc(davesession.CreateFunc())` with `voice.WithDaveSessionCreateFunc(golibdave.NewSession)`.
3. `cmd/bot/Dockerfile`:
   - builder `golang:1.26-bookworm`; install libdave v1.3.0 with godave's `scripts/libdave_install.sh v1.3.0` (audit first); set `PKG_CONFIG_PATH` to the install dir; build with `CGO_ENABLED=1`.
   - runtime `debian:bookworm-slim` (+ `ca-certificates`); copy `libdave.so` and run `ldconfig`.
4. Local dev: run the same install script once; `go build`/`go test` then need cgo + pkg-config.
5. Record the switch in `milestones/STATE.md` (Decisions).

## Known gaps / candidates for follow-up

- **Startup voice-state race:** `RestoreSessions` can run before `GUILD_CREATE` voice states land in the cache, so autoshush may log `voice state not found` right after a restart (prod already logged the discordgo equivalent `state cache not found` ~275 times).
- **ffmpeg** is still installed in the runtime image but nothing in the bot shells out to it (sounds are pre-encoded `.dca`); removing it would shrink the 158MB image.
- **Docker build context** is all of `~/GitHub` with no `.dockerignore` at that level — slow builds; consider a dedicated context dir or a parent-level `.dockerignore`.
- disgo open issue #648 (Close during reconnect backoff doesn't stop reconnect) — could affect shutdown during an outage.
- Pre-existing lock-ordering hazards in `participantsMgr.AcquireVoiceChannelLock` / `sessionCache.Get` (out of scope for M01).
