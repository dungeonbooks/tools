package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Env            string
	ExaAPIKey      string
	HardcoverToken string // raw Authorization header value
	GoogleBooksKey string // optional; keyless works but is throttled
}

func Load() Config {
	_ = godotenv.Load() // best-effort: load .env if present; real env vars win
	return Config{
		Env:            env("ENV", "dev"),
		ExaAPIKey:      os.Getenv("EXA_API_KEY"),
		HardcoverToken: os.Getenv("HARDCOVER_API_TOKEN"),
		GoogleBooksKey: os.Getenv("GOOGLE_BOOKS_API_KEY"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
