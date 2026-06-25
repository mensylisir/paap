package config

import "testing"

func TestLoadDoesNotDefaultDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	cfg := Load()

	if cfg.DatabaseURL != "" {
		t.Fatalf("DatabaseURL = %q, want empty when DATABASE_URL is not set", cfg.DatabaseURL)
	}
}
