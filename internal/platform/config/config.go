package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Env            string
	ExaAPIKey      string
	HardcoverToken string
	GoogleBooksKey string
	CachePath      string
}

func Load() Config {
	_ = godotenv.Load()
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
