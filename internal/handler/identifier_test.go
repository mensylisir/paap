package handler

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNormalizeIdentifier(t *testing.T) {
	if got := normalizeIdentifier("中文应用", "app", 50); got != "app" {
		t.Fatalf("normalizeIdentifier fallback = %q, want %q", got, "app")
	}
	if got := normalizeIdentifier("Order_Service 01", "app", 50); got != "order-service-01" {
		t.Fatalf("normalizeIdentifier = %q", got)
	}
}

func TestUniqueIdentifierAddsSuffixForDuplicates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	exists := func(db *gorm.DB, candidate string) (bool, error) {
		return candidate == "test" || candidate == "test-2", nil
	}

	got, err := uniqueIdentifier(db, "test", 50, exists)
	if err != nil {
		t.Fatalf("uniqueIdentifier error: %v", err)
	}
	if got != "test-3" {
		t.Fatalf("uniqueIdentifier = %q, want test-3", got)
	}
}
