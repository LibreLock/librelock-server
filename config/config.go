package config

import (
	"os"
	"strconv"
	"strings"
)

// Deployment modes
// The active mode lives in the DB (app_state), read via appmode
const (
	ModePersonal     = "personal"
	ModeOrganization = "organization"
)

type Config struct {
	Port          string
	DBPath        string
	TokenTTL      int
	AppEnv        string
	AllowedOrigin string
	// Proxies whose X-Forwarded-For is believed, as a comma-separated list of IPs or CIDRs
	// Empty means trust none, which is right for a directly exposed server; behind a reverse proxy
	// it has to be set or every client looks like the proxy and they all share one rate-limit bucket
	TrustedProxies []string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8000"),
		DBPath:         getEnv("DB_PATH", "data/librelock.db"),
		TokenTTL:       getEnvInt("TOKEN_TTL", 3600),
		AppEnv:         getEnv("APP_ENV", "development"),
		AllowedOrigin:  getEnv("ALLOWED_ORIGIN", "http://localhost:1401"),
		TrustedProxies: getEnvList("TRUSTED_PROXIES"),
	}
}

func getEnvList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
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
