# Agent Development Guide

This document provides guidelines for agentic coding tools working in the pomomo-go repository.

## Project Overview

pomomo-go is a Discord bot that facilitates Pomodoro sessions

- `pomomo` (root): Shared library with commands, models, interfaces, and configuration
- `cmd/bot`: Main Discord bot application with handlers and session management
- `cmd/register`: Registration tool for Discord commands
- `sqlite`: Database repository implementations

## Commands

### Building
```bash
# Build the bot
go build -o bin/bot ./cmd/bot
```

### Development
```bash
# Run the bot (requires .env configuration)
POMOMO_CONFIG_PATH=$HOME/.pomomo/.env.dev go run ./cmd/bot
```

