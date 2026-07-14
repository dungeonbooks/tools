package config

import (
	"os"

	"github.com/joho/godotenv"
)

// SharedEnvPath holds credentials outside any checkout, so they survive a git
// pull and are shared by every agent on the box. The repo's own .env is
// gitignored and takes precedence when present.
const SharedEnvPath = "/etc/dungeonbooks/marty.env"

type Config struct {
	Env            string
	ExaAPIKey      string
	HardcoverToken string
	GoogleBooksKey string
	CachePath      string
}

// Load reads configuration from the environment, backfilling from env files in
// order of precedence: the real environment, then ./.env, then $MARTY_ENV, then
// SharedEnvPath. godotenv never overwrites a variable that is already set, so
// earlier sources win. Later sources matter for the MCP server, which a client
// may launch from any working directory with a sparse environment.
func Load() Config {
	_ = godotenv.Load()
	for _, path := range []string{os.Getenv("MARTY_ENV"), SharedEnvPath} {
		if path != "" {
			_ = godotenv.Load(path)
		}
	}
	return Config{
		Env:            env("ENV", "dev"),
		ExaAPIKey:      os.Getenv("EXA_API_KEY"),
		HardcoverToken: os.Getenv("HARDCOVER_API_TOKEN"),
		GoogleBooksKey: os.Getenv("GOOGLE_BOOKS_API_KEY"),
		CachePath:      os.Getenv("MARTY_CACHE"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
