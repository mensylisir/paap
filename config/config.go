package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "9090"),
		DatabaseURL: getEnv("DATABASE_URL", "paap.db"),
		JWTSecret:   getEnv("JWT_SECRET", "paap-dev-secret-change-in-prod"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
