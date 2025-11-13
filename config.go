package pomomo

import (
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	BotName     string
	BotToken    string
}

func LoadConfig() (Config, error) {
	isProd := flag.Bool("p", false, "is production environment")
	if *isProd {
		_ = godotenv.Load(".env")
	} else {
		_ = godotenv.Load(".env.dev")
	}

	config := Config{
		DatabaseURL: os.Getenv("POMOMO_DB_PATH"),
		BotName:     os.Getenv("POMOMO_BOT_NAME"),
		BotToken:    os.Getenv("POMOMO_BOT_TOKEN"),
	}

	if config.BotToken == "" {
		return Config{}, fmt.Errorf("required environment variable: POMOMO_BOT_TOKEN")
	}

	if config.BotName == "" {
		config.BotName = "Pomomo"
	}

	return config, nil
}
