package database

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInitRejectsNonPostgresDSN(t *testing.T) {
	err := Init(filepath.Join(t.TempDir(), "paap.db"))

	if err == nil {
		t.Fatalf("Init accepted a non-PostgreSQL DSN")
	}
	if !strings.Contains(err.Error(), "PostgreSQL DATABASE_URL") {
		t.Fatalf("Init error = %q, want PostgreSQL DATABASE_URL message", err.Error())
	}
}
