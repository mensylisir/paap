package config

import "os"

type Config struct {
	Port                         string
	DatabaseURL                  string
	JWTSecret                    string
	KeycloakIssuerURL            string
	KeycloakBackchannelIssuerURL string
	KeycloakClientID             string
	KeycloakClientSecret         string
	KeycloakRedirectURL          string
}

func Load() *Config {
	return &Config{
		Port:                         getEnv("PORT", "9090"),
		DatabaseURL:                  getEnv("DATABASE_URL", ""),
		JWTSecret:                    getEnv("JWT_SECRET", "paap-dev-secret-change-in-prod"),
		KeycloakIssuerURL:            getEnv("KEYCLOAK_ISSUER_URL", ""),
		KeycloakBackchannelIssuerURL: getEnv("KEYCLOAK_BACKCHANNEL_ISSUER_URL", ""),
		KeycloakClientID:             getEnv("KEYCLOAK_CLIENT_ID", ""),
		KeycloakClientSecret:         getEnv("KEYCLOAK_CLIENT_SECRET", ""),
		KeycloakRedirectURL:          getEnv("KEYCLOAK_REDIRECT_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
