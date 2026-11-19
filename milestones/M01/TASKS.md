# Phase 1 — Dependencies & foundations

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T01 | Done | Add disgo v0.20.0, `disgoorg/snowflake/v2`, `thomas-vilte/dave-go` (exact versions) to `go.mod` | | disgo v0.20.0, snowflake v2.0.3, dave-go v0.5.1 (+ mls-go v1.6.0); godave resolves to v0.3.0 and dave-go's session compiles against it. `go` directive bumped 1.25.3 → 1.26.0 (dave-go requirement) — Dockerfile builder image must be bumped in T18. Deps are `// indirect` until imported; don't `go mod tidy` before T17 |
| T02 | Done | Snowflake ↔ string helpers for `pomomo.*ID` / guild / user / message IDs | T01 | `disgo/ids.go`: generic `ParseID[T ~string]`, `ParseIDs`, `FormatID[T]`; errors on empty/invalid, with tests |
| T03 | Done | slog bridge: disgo logger → charmbracelet/log with level mapping | T01 | `disgo.NewLogger(l)` = `slog.New(l.WithPrefix("disgo"))`; charm levels == slog levels. Test asserts 1 timestamp/line, debug filtered at INFO, caller = slog call site. Must be created **after** `log.SetOutput`/`SetLevel` in main (T13) since `WithPrefix` snapshots them |

# Phase 2 — Adapters on disgo types

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T04 | Done | Port `commands.go` `StartCommand` to `discord.SlashCommandCreate` | T01 | Done together with T16 (register is its only typed consumer) to keep the tree building. JSON payload verified identical (types, names, min/max) |
| T05 | | Port component builders (`SessionMessageComponents`, `TextDisplay`, buttons, container, accent color) to disgo Components v2 | T01 | Keep `InteractionID` custom-ID format unchanged |
| T06 | Done | Port `DiscordMessenger` (`EditChannelMessage`, `Respond`, `EditResponse`, `DeferMessageCreate`, `DeferMessageUpdate`) to disgo REST/interaction APIs | T02, T05 | `disgo.Messenger` (REST-only, takes `discord.Interaction`). DeferMessageUpdate followup edits `@original` (= component's message). httptest-verified paths, response types, CV2/ephemeral flags |
| T07 | Done | New `disgo/` voice adapter: `UpdateVoiceState` (`Rest.UpdateMember`), `GetVoiceState` (cache) | T02 | `disgo.Adapter`: `UpdateVoiceState` (`*bool` so `mute:false` is sent), `MoveMember`, `GetVoiceState` (cache; now also returns `ChannelID` — added `ChannelID` to `pomomo.VoiceState`) |
| T08 | Done | Voice conn manager: one conn per guild, lazy open on first alert, `Close(guildID)`, re-open if dropped | T01 | Per-guild lock; reuses conn if same channel + voice gateway `StatusReady`, else close + `CreateConn` + `Open` (10s); `CloseVoice(gid)` |
| T09 | Done | `SendOpusAudio` on the managed conn: bounded wait for `DAVE().Ready()`, write `.dca` frames to `conn.UDP()` at 20ms ticks, speaking flag on/off | T08 | Uses disgo's `AudioSender` via `SetOpusFrameProvider` (handles 20ms pacing, speaking flags, silence frames, DAVE gating) with a one-shot `frameProvider`; timeout = alert length + 10s DAVE wait. Tested with fake Manager/Conn under `-race` |

# Phase 3 — Event wiring & client

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T10 | | Port `StartSession` hook to `*events.ApplicationCommandInteractionCreate` | T04, T06 | Preserve respond-before-DB ordering (3s deadline); pin via REST |
| T11 | | Port `SkipInterval` / `EndSession` / `JoinSession` to `*events.ComponentInteractionCreate` | T06, T07 | Join: voice state from cache, move via `UpdateMember` `ChannelID` |
| T12 | | Port voice hooks to `GuildVoiceJoin` / `GuildVoiceMove` / `GuildVoiceLeave` using `OldVoiceState` | T07 | Keep guild-scoped detached-participant restore |
| T13 | | `main.go`: build disgo client (intents Guilds + GuildVoiceStates, cache flags, shard ID/count, HTTP timeout, user agent, logger, `voice.WithDaveSessionCreateFunc(dave-go)`), register listeners | T03, T09–T12 | DAVE impl set in exactly one place |
| T14 | | `main.go` session-end path: unpin via REST, close guild voice conn when session count hits 0 | T08, T13 | Replaces `cl.VoiceConnections[...].Disconnect()` |
| T15 | | Shutdown ordering: voice-state restores → close voice conns → `client.Close(ctx)` within 10s | T13, T14 | |
| T16 | Done | Port `cmd/register` to disgo REST bulk overwrite (no gateway) | T04 | `rest.New` + `GetCurrentApplication` + `SetGlobalCommands`; shard config no longer read. Not yet run against Discord |

# Phase 4 — Cleanup & build

| #   | Status | Task | Dependencies | Notes |
| --- | ------ | ---- | ------------ | ----- |
| T17 | | Remove discordgo: `discordgo/` package, `cmd/bot/discordgo.go`, `replace ../discordgo`, `go mod tidy`; `go build` + `go vet` clean | T13–T16 | |
| T18 | | Dockerfile: drop `COPY discordgo ../discordgo`, keep Alpine + `CGO_ENABLED=0`; local `docker build` from parent dir succeeds | T17 | |
| T19 | | Write `milestones/M01/FOLLOWUP-01.md` with verification steps and golibdave fallback procedure | T18 | |
