package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime settings loaded from the environment.
type Config struct {
	Addr            string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
	RateLimitPerMin int
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		Addr:            env("TASKOS_ADDR", ":8080"),
		DatabaseURL:     env("TASKOS_DATABASE_URL", "postgres://localhost:5432/taskos?sslmode=disable"),
		ShutdownTimeout: envDuration("TASKOS_SHUTDOWN_TIMEOUT", 10*time.Second),
		AllowedOrigins:  envCSV("TASKOS_CORS_ORIGINS", "*"),
		RateLimitPerMin: envInt("TASKOS_RATE_LIMIT_PER_MIN", 120),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func envCSV(key, fallback string) []string {
	raw := env(key, fallback)
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
