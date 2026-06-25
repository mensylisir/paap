package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	CapabilitySourceManaged  = "managed"
	CapabilitySourceShared   = "shared"
	CapabilitySourceExternal = "external"
	CapabilitySourceDeferred = "deferred"
)

type EnvironmentCapability struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`

	EnvironmentID uint        `gorm:"not null;uniqueIndex:idx_environment_capability" json:"environmentId"`
	Environment   Environment `gorm:"foreignKey:EnvironmentID" json:"environment,omitempty"`

	Capability string `gorm:"size:40;not null;uniqueIndex:idx_environment_capability" json:"capability"`
	Source     string `gorm:"size:20;not null;default:managed" json:"source"`
	Provider   string `gorm:"size:50" json:"provider"`

	ServiceType  string               `gorm:"size:30" json:"serviceType,omitempty"`
	RefServiceID *uint                `gorm:"index" json:"refServiceId,omitempty"`
	RefService   *ServiceInstallation `gorm:"foreignKey:RefServiceID" json:"refService,omitempty"`

	ExternalEndpoint      string `gorm:"size:500" json:"externalEndpoint,omitempty"`
	CredentialSecretRef   string `gorm:"size:200" json:"credentialSecretRef,omitempty"`
	TLSInsecureSkipVerify bool   `gorm:"default:false" json:"tlsInsecureSkipVerify,omitempty"`

	ValidationStatus  string `gorm:"size:20;default:pending" json:"validationStatus"`
	ValidationMessage string `gorm:"type:text" json:"validationMessage,omitempty"`
	CreatedBy         uint   `gorm:"index" json:"createdBy,omitempty"`
}
