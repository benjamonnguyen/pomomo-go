# MVP
## autoshush
btn
func (h *commandHandler) JoinSession(s *discordgo.Session, m *discordgo.InteractionCreate) {
  // move user into voice channel
  // noop if already in channel - should leave channel to leave session
	panic("not implemented")
}
noDeafen start option

## msg
start, end

## permissions
minPermission set to user that starts session

## refactor session_manager: extract session_provider

## bugs
- message not being cleanedup

## testing
- commands.go integration tests
- update online docs, top.gg, support server url

# TODO
- stats
  - display at end or during
  - persist for premium

pomctl
commands: register-cmds, purge, kill, rest, broadcast

shard_manager
- rebalance

