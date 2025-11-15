package pomomo

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string
	BotName     string
	BotToken    string
}

func LoadConfig() (Config, error) {
	isProd := flag.Bool("p", false, "is production environment")
	homeDir, _ := os.UserHomeDir()
	envPath := ""
	if *isProd {
		log.Println("Environment: PROD")
		envPath = path.Join(homeDir, ".pomomo", ".env")
	} else {
		log.Println("Environment: DEV")
		envPath = path.Join(homeDir, ".pomomo", ".env.dev")
	}

	log.Println("loading config from", envPath)
	_ = godotenv.Load(envPath)
	config := Config{
		DatabaseURL: os.Getenv("POMOMO_DB_PATH"),
		BotName:     os.Getenv("POMOMO_BOT_NAME"),
		BotToken:    os.Getenv("POMOMO_BOT_TOKEN"),
	}

	if strings.HasPrefix(config.DatabaseURL, "~/") {
		config.DatabaseURL = strings.Replace(config.DatabaseURL, "~", homeDir, 1)
	}

	if config.BotToken == "" {
		return Config{}, fmt.Errorf("required environment variable: POMOMO_BOT_TOKEN")
	}

	if config.BotName == "" {
		config.BotName = "Pomomo"
	}

	return config, nil
}
