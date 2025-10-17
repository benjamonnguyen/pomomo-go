package pomomo

import (
	"flag"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	isProd := flag.Bool("p", false, "is production environment")
	if *isProd {
		godotenv.Load(".env")
	} else {
		godotenv.Load(".env.dev")
	}
}
