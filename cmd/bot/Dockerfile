# Build stage
FROM golang:1.25.3-alpine AS build
LABEL org.opencontainers.image.source=https://github.com/benjamonnguyen/pomomo-go

WORKDIR /app

# Copy replace packages
COPY discordgo ../discordgo
COPY deadsimple ../deadsimple

# go mod download
COPY pomomo-go/go.mod pomomo-go/go.sum ./
RUN go mod download

COPY pomomo-go .
RUN go build -o /app/bot ./cmd/bot

# Final stage
FROM alpine:3.21

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ffmpeg

# Copy binary from builder
COPY --from=build /app/bot ./bot

# Copy migrations (embedded but needed for reference)
COPY --from=build /app/cmd/bot/migrations ./migrations

# Copy sounds
COPY --from=build /app/cmd/bot/sounds ./sounds

# Create directories for data
RUN mkdir -p ./data

# Default config path (mount your .env file here)
ENV POMOMO_CONFIG_PATH=/app/.env

# Volume for persistent data and config
VOLUME ./data

# Expose no ports (Discord bot only)
# Run the bot
ENTRYPOINT ["/app/bot"]
