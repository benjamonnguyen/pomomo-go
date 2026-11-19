# Milestone 1 — Migrate from discordgo to disgo

**Goal:** Replace `github.com/bwmarrin/discordgo` (local fork at `../discordgo`) with `github.com/disgoorg/disgo` so that the bot (1) survives gateway reconnects without deadlocking and (2) can play interval alerts again under Discord's mandatory DAVE (E2EE) voice protocol.

## Background

- **Gateway deadlock:** discordgo `Open()` holds the session lock while reading the first post-handshake packet. If Discord answers with Op 7/9 (common after an outage), `onEvent` → `CloseWithCode` re-locks → permanent deadlock. Observed 11/13 on both shards: bots silently offline ("The application did not respond") until manual restart; shutdown hangs in `cl.Close()`. Upstream: issues #513/#544 open since 2018, fix PR #1694 unmerged.
- **Voice broken:** every alert fails with `close 4017: E2EE/DAVE protocol required` / `timeout waiting for voice`. discordgo has no DAVE support (PR #1704 unmerged).
- **disgo v0.20.0** (2026-11-06): handles Op 7/9 in the read loop, zombie-connection detection, voice-state cache, voice events with `OldVoiceState`, DAVE via a pluggable `godave.Session` (cgo `golibdave` or pure-Go `thomas-vilte/dave-go`).

## Requirements

- Feature parity with current behavior:
  - `/start` slash command with all options; registration via `cmd/register`
  - Join / End / Skip buttons (Components v2 session message: greeting, settings container, timer bar, action row)
  - session message edit loop, pin on start / unpin on end
  - autoshush: server mute/deafen participants during pomodoro, restore on break / leave / end / shutdown
  - move user into session voice channel on Join
  - detached-participant restore on voice channel rejoin
  - interval alert audio from embedded `.dca` Opus frames, **working with DAVE**
  - sharding via `POMOMO_SHARD_ID` / `POMOMO_SHARD_COUNT` (one process per shard, separate DBs)
  - graceful shutdown restores participant voice state, finishes within the 10s docker stop window
- Gateway recovers from Op 7 / Op 9 / dropped connections without a manual restart.
- No changes to the DB schema or session/participant domain logic beyond what the API swap requires.
- Domain types (`pomomo.TextChannelID`, `VoiceChannelID`, guild/user IDs as `string`) stay unchanged; snowflake ↔ string conversion happens only in adapter code.
- Docker image builds for linux/amd64 and runs on starling.lan.

## Scope

### In scope

- `go.mod`: drop discordgo + `replace ../discordgo`; add disgo, snowflake, dave-go (exact versions pinned)
- `commands.go`: `StartCommand` → `discord.SlashCommandCreate`
- `cmd/register`: bulk overwrite global commands via disgo REST (no gateway connection needed)
- `cmd/bot/discordgo.go` → `cmd/bot/disgo.go`: `DiscordMessenger` + component builders (`SessionMessageComponents`, `TextDisplay`, buttons) on disgo types; `followup` func type
- `discordgo/` package → `disgo/` adapter:
  - `UpdateVoiceState` → `Rest.UpdateMember` with `*bool` mute/deaf
  - `GetVoiceState` → voice-state cache
  - `SendOpusAudio` → reuse the guild's voice conn (see Decisions), wait (bounded) for `DAVE().Ready()`, write frames to `conn.UDP()` at 20ms ticks
  - voice conn lifecycle: open lazily on first alert, close when the guild has no sessions
- `cmd/bot/discord_event_hooks.go`: handler signatures → disgo events
  - `StartSession` → `*events.ApplicationCommandInteractionCreate`
  - `SkipInterval` / `EndSession` / `JoinSession` → `*events.ComponentInteractionCreate`
  - voice hooks → `GuildVoiceJoin` / `GuildVoiceMove` / `GuildVoiceLeave` (replaces `BeforeUpdate` checks)
- `cmd/bot/main.go`: client construction (intents, cache flags, shard, HTTP timeout, user agent, logger, DAVE session func), voice conn close when guild has no sessions, `client.Close(ctx)` in shutdown
- Logging: route disgo's `slog` output through charmbracelet/log (it implements `slog.Handler`)
- `cmd/bot/Dockerfile`: remove `COPY discordgo ../discordgo`; keep Alpine + `CGO_ENABLED=0`

### Out of scope

- Interim discordgo hotfix (PR #1694 cherry-pick / watchdog) — not shipping; prod stays as-is until M01 deploys
- Merging shards into one process with disgo's `sharding.ShardManager` (DBs are per-shard)
- Multi-shard verification (config path only; see Verification)
- HTTP interactions endpoint (keep gateway interactions)
- CI image builds
- New features (stats, pomctl, multisession)
- Fixing pre-existing lock-ordering hazards in `participantsMgr.AcquireVoiceChannelLock` / `sessionCache.Get` (follow-up milestone)

## Decisions

- **DAVE: `thomas-vilte/dave-go` first, `golibdave` as fallback.**
  - Start with pure-Go `dave-go`: keeps the Alpine image, `CGO_ENABLED=0`, static binary, and plain `go run` / `go test` locally.
  - Wire it through a single `voice.WithDaveSessionCreateFunc(...)` call in `main.go` so swapping implementations is a one-line change; no DAVE-specific code elsewhere.
  - Fall back to cgo `golibdave` (Discord's official libdave) if dave-go fails verification: no/garbled audio, voice gateway rejects the session, DAVE never becomes ready, or panics/leaks. Fallback means switching the Docker build + runtime images to Debian-slim with prebuilt `libdave` v1.3.0 (Discord ships glibc builds only), and documenting the libdave install for local dev.
  - Pin the exact `dave-go` version; record which implementation shipped in STATE.md.
- **No interim fix.** The stale-participant fix and the migration deploy together.
- **Image build stays local:** `docker build` from the parent dir (context includes `pomomo-go/` and `deadsimple/`), push to `ghcr.io/benjamonnguyen/pomomo-bot` manually. `../discordgo` is dropped from the build context.
- **Voice connection per session, not per alert:** one conn per guild, opened on the first alert and kept for the session's lifetime; closed when the guild's session count hits 0 (same trigger as today's `conn.Disconnect()`) and on shutdown. Avoids DAVE handshake latency before every alert. Must handle the conn having dropped/reconnected between alerts (re-open if not connected).

## Verification

- Single dev bot via `POMOMO_CONFIG_PATH=$HOME/.pomomo/.env.dev` in a test guild; no multi-shard testing (shard ID/count config is passed through but only exercised with count 1).
- Listen to alerts in a real voice channel with a DAVE-capable client.

## Risks / Pitfalls

- **disgo is pre-1.0** — pin exact version; minor bumps can break APIs.
- **dave-go is experimental** — a protocol bug would surface as silent/garbled audio or rejected voice sessions rather than a clear error; verification must include listening to alerts in a real voice channel with a DAVE-capable client, across a channel move and a voice reconnect.
- **DAVE readiness:** frames written before `conn.DAVE().Ready()` are dropped/garbled; alert must wait (bounded) for readiness after join and after members join/leave (epoch changes).
- **Long-lived voice conn:** must survive voice server changes / reconnects between alerts; check connection state before sending and re-open on failure.
- **Interaction 3s deadline:** `StartSession` responds before DB work today — preserve ordering; `Respond(wait=true)` needs `GetInteractionResponse` to obtain the message ID for pinning/editing.
- **Voice state cache needs `GUILD_CREATE` voice states** — requires `IntentGuilds | IntentGuildVoiceStates` and `cache.FlagVoiceStates` (+ guilds); verify the cache is populated after restart for `RestoreCache`'d participants.
- **Shutdown ordering:** voice-state restores → close voice conns → `client.Close(ctx)`, all within the 10s budget.
- **Custom IDs / message IDs** are strings in the DB — parse with `snowflake.Parse` and handle errors instead of panicking.
- **charmbracelet/log ↔ slog** level mapping; avoid double timestamps in `bot.log`.
- **Prod stays on discordgo until M01 ships** — gateway deadlocks may still require manual restarts in the meantime.
