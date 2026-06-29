package database

import (
	"fmt"
	"log"
	"strings"

	"paap/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dsn string) error {
	var err error

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
	} else {
		return fmt.Errorf("PostgreSQL DATABASE_URL is required")
	}
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := autoMigrate(); err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	if err := RunSQLMigrations(); err != nil {
		return fmt.Errorf("failed to run sql migrations: %w", err)
	}

	log.Println("Database connected and migrated successfully")
	return nil
}

func autoMigrate() error {
	if err := migrateServiceInstallationProvisionMode(); err != nil {
		return err
	}
	if err := deduplicateServiceInstallations(); err != nil {
		return err
	}
	if err := migrateEnvironmentCapabilities(); err != nil {
		return err
	}
	if err := migrateServiceTemplateManifestRBAC(); err != nil {
		return err
	}
	return DB.AutoMigrate(
		&model.User{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.EnvironmentCanvasState{},
		&model.EnvironmentCapability{},
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

func migrateServiceTemplateManifestRBAC() error {
	return DB.Exec(`
ALTER TABLE IF EXISTS service_templates
	DROP COLUMN IF EXISTS workload_role_policy,
	DROP COLUMN IF EXISTS environment_role_policy
`).Error
}

func migrateServiceInstallationProvisionMode() error {
	if !DB.Migrator().HasTable(&model.ServiceInstallation{}) {
		return nil
	}
	if !DB.Migrator().HasColumn(&model.ServiceInstallation{}, "ProvisionMode") {
		if err := DB.Exec(`ALTER TABLE service_installations ADD COLUMN provision_mode varchar(20) DEFAULT 'managed'`).Error; err != nil {
			return err
		}
	}
	if err := DB.Exec(`UPDATE service_installations SET provision_mode = 'managed' WHERE provision_mode IS NULL OR provision_mode = ''`).Error; err != nil {
		return err
	}
	if DB.Migrator().HasIndex(&model.ServiceInstallation{}, "idx_service_installation_env_type") {
		if err := DB.Migrator().DropIndex(&model.ServiceInstallation{}, "idx_service_installation_env_type"); err != nil {
			return err
		}
	}
	return nil
}

func migrateEnvironmentCapabilities() error {
	if !DB.Migrator().HasTable(&model.EnvironmentCapability{}) {
		return nil
	}
	if err := DB.Unscoped().Where("deleted_at IS NOT NULL").Delete(&model.EnvironmentCapability{}).Error; err != nil {
		return err
	}
	if !DB.Migrator().HasColumn(&model.EnvironmentCapability{}, "CapabilityKey") {
		if err := DB.Exec(`ALTER TABLE environment_capabilities ADD COLUMN capability_key varchar(100)`).Error; err != nil {
			return err
		}
	}
	if err := DB.Exec(`
UPDATE environment_capabilities
SET capability_key = capability
WHERE capability_key IS NULL OR capability_key = ''
`).Error; err != nil {
		return err
	}
	if DB.Migrator().HasIndex(&model.EnvironmentCapability{}, "idx_environment_capability") {
		if err := DB.Migrator().DropIndex(&model.EnvironmentCapability{}, "idx_environment_capability"); err != nil {
			return err
		}
	}
	return nil
}

func deduplicateServiceInstallations() error {
	if !DB.Migrator().HasTable(&model.ServiceInstallation{}) {
		return nil
	}

	if err := DB.Unscoped().Where("deleted_at IS NOT NULL").Delete(&model.ServiceInstallation{}).Error; err != nil {
		return err
	}

	return DB.Exec(`
DELETE FROM service_installations
WHERE id IN (
	SELECT duplicate.id
	FROM service_installations AS duplicate
	JOIN (
		SELECT environment_id, service_type, provision_mode, MIN(id) AS keep_id
		FROM service_installations
		WHERE deleted_at IS NULL
		GROUP BY environment_id, service_type, provision_mode
		HAVING COUNT(*) > 1
	) AS grouped
	ON duplicate.environment_id = grouped.environment_id
	AND duplicate.service_type = grouped.service_type
	AND duplicate.provision_mode = grouped.provision_mode
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
