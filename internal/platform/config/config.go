package config

import "os"

type Config struct {
	Env       string
	Port      string
	ExaAPIKey string
}

func Load() Config {
	return Config{
		Env:       env("ENV", "dev"),
		Port:      env("PORT", "8080"),
		ExaAPIKey: os.Getenv("EXA_API_KEY"),
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
