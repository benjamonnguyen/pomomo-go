package pomomo

import (
	"os"

	"github.com/benjamonnguyen/deadsimple/cfg"
	"github.com/benjamonnguyen/deadsimple/cfg/env"
	"github.com/charmbracelet/log"
)

const (
	DatabaseURLKey cfg.Key = "POMOMO_DB_URL"
	BotNameKey     cfg.Key = "POMOMO_BOT_NAME"
	BotTokenKey    cfg.Key = "POMOMO_BOT_TOKEN"
	ShardIDKey     cfg.Key = "POMOMO_SHARD_ID"
	ShardCountKey  cfg.Key = "POMOMO_SHARD_COUNT"
	LogLevelKey    cfg.Key = "POMOMO_LOG_LEVEL"
	LogFileKey     cfg.Key = "POMOMO_LOG_FILE"
)

func LoadConfig() (cfg.Config, error) {
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
		{
			Key:      LogFileKey,
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
