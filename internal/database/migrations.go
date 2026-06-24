package database

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"paap/migration"

	"gorm.io/gorm"
)

func RunSQLMigrations() error {
	if DB == nil {
		return fmt.Errorf("database is not initialized")
	}
	return RunSQLMigrationsWithFS(DB, migration.Files)
}

func RunSQLMigrationsWithFS(db *gorm.DB, migrations fs.FS) error {
	if err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
	version TEXT PRIMARY KEY,
	applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`).Error; err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	paths, err := fs.Glob(migrations, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(paths)

	for _, path := range paths {
		version := filepath.Base(path)

		var applied int64
		if err := db.Raw("SELECT COUNT(1) FROM schema_migrations WHERE version = ?", version).Scan(&applied).Error; err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if applied > 0 {
			continue
		}

		content, err := fs.ReadFile(migrations, path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}
		sql := strings.TrimSpace(string(content))
		if sql == "" {
			continue
		}

		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(sql).Error; err != nil {
				return err
			}
			return tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version).Error
		}); err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
	}

	return nil
}
