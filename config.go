package pomomo

import (
	"os"

	"github.com/benjamonnguyen/deadsimple/config"
	"github.com/benjamonnguyen/deadsimple/config/env"
	"github.com/charmbracelet/log"
)

const (
	DatabaseURLKey         config.Key = "POMOMO_DB_URL"
	BotNameKey             config.Key = "POMOMO_BOT_NAME"
	BotTokenKey            config.Key = "POMOMO_BOT_TOKEN"
	PomodoroSoundPathKey   config.Key = "POMOMO_POMODORO_SOUND_PATH"
	LongBreakSoundPathKey  config.Key = "POMOMO_LONG_BREAK_SOUND_PATH"
	ShortBreakSoundPathKey config.Key = "POMOMO_SHORT_BREAK_SOUND_PATH"
	IdleSoundPathKey       config.Key = "POMOMO_IDLE_SOUND_PATH"
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
			Key:      PomodoroSoundPathKey,
			Required: false,
		},
		{
			Key:      LongBreakSoundPathKey,
			Required: false,
		},
		{
			Key:      ShortBreakSoundPathKey,
			Required: false,
		},
		{
			Key:      IdleSoundPathKey,
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
