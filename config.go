package pomomo

import (
	"os"

	"github.com/benjamonnguyen/deadsimple/config"
	"github.com/benjamonnguyen/deadsimple/config/env"
	"github.com/charmbracelet/log"
)

const (
	DatabaseURLKey config.Key = "POMOMO_DB_URL"
	BotNameKey     config.Key = "POMOMO_BOT_NAME"
	BotTokenKey    config.Key = "POMOMO_BOT_TOKEN"
	ShardIDKey     config.Key = "POMOMO_SHARD_ID"
	ShardCountKey  config.Key = "POMOMO_SHARD_COUNT"
	LogLevelKey    config.Key = "POMOMO_LOG_LEVEL"
)

func LoadConfig() (config.Config, error) {
	entries := []env.Entry{
		{
			Key:      DatabaseURLKey,
			Required: true,
		},
		{
			Key:      BotNameKey,
			Default:  "Pomomo",
			Required: true,
		},
		{
			Key:      BotTokenKey,
			Required: true,
		},
		{
			Key:      ShardIDKey,
			Required: false,
		},
		{
			Key:      ShardCountKey,
			Required: false,
		},
		{
			Key:      LogLevelKey,
			Required: false,
		},
	}

	cfgPath := os.Getenv("POMOMO_CONFIG_PATH")
	if cfgPath == "" {
		log.Fatal("missing POMOMO_CONFIG_PATH")
	}
	log.Info("loading config", "path", cfgPath)
	return env.NewConfig(cfgPath, entries...)
}
