package model

import (
	"time"

	"gorm.io/gorm"
)

// Component represents a business service component in an environment
type Component struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	EnvironmentID uint           `gorm:"not null;index" json:"environmentId"`
	Name          string         `gorm:"size:50;not null" json:"name"`
	Type          string         `gorm:"size:20;not null" json:"type"` // frontend, backend, database, middleware, custom
	Image         string         `gorm:"size:200" json:"image"`
	Version       string         `gorm:"size:50;default:latest" json:"version"`
	Replicas      int            `gorm:"default:1" json:"replicas"`
	CPU           string         `gorm:"size:20;default:0.5核" json:"cpu"`
	Memory        string         `gorm:"size:20;default:512MB" json:"memory"`
	Status        string         `gorm:"size:20;default:stopped" json:"status"` // running, stopped, error
	ErrorMessage  string         `gorm:"type:text" json:"errorMessage"`
}

// ServiceInstance represents a tool service instance in an application (deprecated, use ServiceInstallation)
type ServiceInstance struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	ApplicationID uint           `gorm:"not null;index" json:"applicationId"`
	Application   Application    `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	ServiceType   string         `gorm:"size:20;not null" json:"serviceType"` // deploy, ci, monitor, log
	Status        string         `gorm:"size:20;default:pending" json:"status"` // pending, running, error
	Config        string         `gorm:"type:text" json:"config"` // JSON config
}
