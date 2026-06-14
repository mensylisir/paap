package database

import (
	"fmt"
	"log"
	"strings"

	"paap/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dsn string) error {
	var err error
	var dialector gorm.Dialector

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		dialector = postgres.Open(dsn)
	} else {
		dialector = sqlite.Open(dsn)
	}

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := autoMigrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("Database connected and migrated successfully")
	return nil
}

func autoMigrate() error {
	if err := deduplicateServiceInstallations(); err != nil {
		return err
	}
	return DB.AutoMigrate(
		&model.User{},
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.EnvironmentCanvasState{},
		&model.EnvTemplate{},
		&model.ServiceTemplate{},
		&model.ServiceCatalog{},
		&model.ServiceInstallation{},
		&model.InfraInstallation{},
		&model.Component{},
		&model.ComponentConfigTemplate{},
		&model.ServiceInstance{},
	)
}

func deduplicateServiceInstallations() error {
	if !DB.Migrator().HasTable(&model.ServiceInstallation{}) {
		return nil
	}

	return DB.Exec(`
DELETE FROM service_installations
WHERE id IN (
	SELECT duplicate.id
	FROM service_installations AS duplicate
	JOIN (
		SELECT environment_id, service_type, MIN(id) AS keep_id
		FROM service_installations
		WHERE deleted_at IS NULL
		GROUP BY environment_id, service_type
		HAVING COUNT(*) > 1
	) AS grouped
	ON duplicate.environment_id = grouped.environment_id
	AND duplicate.service_type = grouped.service_type
	AND duplicate.id <> grouped.keep_id
	WHERE duplicate.deleted_at IS NULL
)`).Error
}

func Close() {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}
