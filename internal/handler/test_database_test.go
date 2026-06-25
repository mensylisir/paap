package handler

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var handlerTestDBSeq uint64

func openTestDB(t *testing.T) (*gorm.DB, error) {
	t.Helper()
	return openPostgresTestDB(t, atomic.AddUint64(&handlerTestDBSeq, 1))
}

func openPostgresTestDB(t *testing.T, seq uint64) (*gorm.DB, error) {
	t.Helper()

	dsn := os.Getenv("PAAP_TEST_DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		dsn = os.Getenv("TEST_DATABASE_URL")
	}
	if strings.TrimSpace(dsn) == "" {
		t.Skip("set PAAP_TEST_DATABASE_URL or TEST_DATABASE_URL to run PostgreSQL database tests")
	}

	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	schema := fmt.Sprintf("paap_test_%d_%d", time.Now().UnixNano(), seq)
	if err := base.Exec(`CREATE SCHEMA "` + schema + `"`).Error; err != nil {
		closeGormDB(base)
		return nil, err
	}

	db, err := gorm.Open(postgres.Open(dsnWithSearchPath(dsn, schema)), &gorm.Config{})
	if err != nil {
		_ = base.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
		closeGormDB(base)
		return nil, err
	}

	t.Cleanup(func() {
		closeGormDB(db)
		_ = base.Exec(`DROP SCHEMA IF EXISTS "` + schema + `" CASCADE`).Error
		closeGormDB(base)
	})

	return db, nil
}

func dsnWithSearchPath(dsn, schema string) string {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func closeGormDB(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
