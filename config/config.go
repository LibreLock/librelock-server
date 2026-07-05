package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DBPath        string
	TokenTTL      int
	AppEnv        string
	AllowedOrigin string
}

func Load() *Config {
	return &Config{
		Port:          getEnv("PORT", "8000"),
		DBPath:        getEnv("DB_PATH", "data/librelock.db"),
		TokenTTL:      getEnvInt("TOKEN_TTL", 3600),
		AppEnv:        getEnv("APP_ENV", "development"),
		AllowedOrigin: getEnv("ALLOWED_ORIGIN", "http://localhost:1401"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
