package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the gam CLI.
type Config struct {
	DatabaseURL string
	RedisURL    string
	ProjectRoot string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() (*Config, error) {
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	cfg := &Config{
		DatabaseURL: getEnv("GAM_DATABASE_URL", "postgres://localhost:5432/gamsync?sslmode=disable"),
		RedisURL:    getEnv("GAM_REDIS_URL", "redis://localhost:6379/0"),
		ProjectRoot: getEnv("GAM_PROJECT_ROOT", projectRoot),
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
