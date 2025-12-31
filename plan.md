- Handle rate limits with exponential backoff

## Implementation Steps

#### 6.3 Update SQLite Repository (`sqlite/session.go`)
- Update `InsertSession` to include `CurrentInterval` and `IntervalStartedAt`
- Update `UpdateSession` to handle new columns
- Update `GetSession` and other queries to select new columns

## Error Handling
Create helper function withRetries(max int, f func() bool) error
1. **Discord API errors**: Log with session context, implement retry with backoff if 5xx error

## Graceful Shutdown
1. **Context cancellation**: Timer goroutine listens to `ctx.Done()`
2. **Wait for completion**: Use `sync.WaitGroup` to ensure current tick finishes
3. **Cleanup**: Release locks, close channels

- refactor to disgo
- Populate cache on startup
  - if stale, best effort deletion of message
- sound alert
- autoshush

- handle interactions from stale messages
- stats
