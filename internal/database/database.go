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
		Logger: logger.Default.LogMode(logger.Info),
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
	return DB.AutoMigrate(
		&model.User{},
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.EnvTemplate{},
		&model.ServiceTemplate{},
		&model.ServiceCatalog{},
		&model.ServiceInstallation{},
		&model.InfraInstallation{},
		&model.Component{},
		&model.ServiceInstance{},
	)
}

func Close() {
	if DB != nil {
		sqlDB, err := DB.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}
