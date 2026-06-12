package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port          string
	DSN           string
	TokenTTL      int
	AppEnv        string
	AllowedOrigin string
}

func Load() *Config {
	return &Config{
		Port:          getEnv("PORT", "8000"),
		DSN:           buildDSN(),
		TokenTTL:      getEnvInt("TOKEN_TTL", 3600),
		AppEnv:        getEnv("APP_ENV", "development"),
		AllowedOrigin: getEnv("ALLOWED_ORIGIN", "http://localhost:1401"),
	}
}

func buildDSN() string {
	if dsn := os.Getenv("DATABASE_URL"); dsn != "" {
		return dsn
	}
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		getEnv("DB_HOST", "127.0.0.1"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "user"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "librelock"),
	)
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
