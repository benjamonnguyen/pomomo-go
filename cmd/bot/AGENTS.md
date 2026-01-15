# Overview
- `cmd/bot/dgutils`: Discord API utility functions
- `cmd/bot/migrations`: SQL migration files

# sqlite
- always use `migrate create -ext sql -dir cmd/bot/migrations -seq <name>` to create migrations

# Database Patterns

- **Transactions**: Use transactor pattern for atomic operations
```go
err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
    // database operations
    return nil
})
```

# Additional considerations
1. **Context cancellation**: Always respect context cancellation in long-running goroutines
2. **Mutex deadlocks**: Be careful with nested locks and defer unlock appropriately
3. **Transaction boundaries**: Keep transactions short and focused
4. **Graceful shutdown**: Use WaitGroup and context cancellation for clean termination



