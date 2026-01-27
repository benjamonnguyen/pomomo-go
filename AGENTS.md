# Agent Development Guide

This document provides guidelines for agentic coding tools working in the pomomo-go repository.

## Project Overview

pomomo-go is a Discord bot that facilitates Pomodoro sessions

- `pomomo` (root): Shared library with commands, models, interfaces, and configuration
- `cmd/bot`: Main Discord bot application with handlers and session management
- `cmd/register`: Registration tool for Discord commands
- `sqlite`: Database repository implementations

## sqlite
- always use `migrate create -ext sql -dir cmd/bot/migrations -seq <name>` to create migrations

## Rules
- Only edit referenced files. Prompt user for permission if trying to edit other files.

## Directives
- `@implement`: When present in referenced file(s), only act on these directives. For example, all TODO comments should be ignore for the scope of this prompt.

## Database Patterns

- **Transactions**: Use transactor pattern for atomic operations
```go
err := m.tx.WithinTransaction(ctx, func(ctx context.Context) error {
    // database operations
    return nil
})
```

## Commands

### Building
```bash
go build -o bin/bot <target>
```

### Development
```bash
# Run the bot (requires .env configuration)
POMOMO_CONFIG_PATH=$HOME/.pomomo/.env.dev go run ./cmd/bot
```

