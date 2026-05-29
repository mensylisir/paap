package model

import (
	"time"

	"gorm.io/gorm"
)

// ServiceCatalog defines available tool and infra service types
type ServiceCatalog struct {
	gorm.Model
	Type        string `gorm:"uniqueIndex;size:30;not null" json:"type"`
	Name        string `gorm:"size:50;not null" json:"name"`
	Category    string `gorm:"size:20;not null" json:"category"` // tool | infra
	Description string `gorm:"size:200" json:"description"`
	Icon        string `gorm:"size:50" json:"icon"`
	Enabled     bool   `gorm:"default:true" json:"enabled"`
}

// InfraInstallation tracks what infrastructure is installed in an environment
type InfraInstallation struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	EnvironmentID uint           `gorm:"not null;index" json:"environmentId"`
	InfraType     string         `gorm:"size:30;not null" json:"infraType"` // postgresql, redis, rabbitmq...
	Status        string         `gorm:"size:20;default:pending" json:"status"`
	Config        string         `gorm:"type:text" json:"config"` // JSON: version, storage, credentials
	ErrorMessage  string         `gorm:"type:text" json:"errorMessage"`
}
