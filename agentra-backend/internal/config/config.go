package config

import "os"

type Config struct {
	DatabaseURL string
	Port        string
	CORSOrigins string
}

func Load() Config {
	return Config{
		DatabaseURL: env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agentra?sslmode=disable"),
		Port:        env("PORT", "8080"),
		CORSOrigins: env("CORS_ORIGINS", "*"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
