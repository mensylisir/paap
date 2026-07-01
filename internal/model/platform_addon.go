package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	PlatformAddonDesiredEnabled  = "enabled"
	PlatformAddonDesiredDisabled = "disabled"

	PlatformAddonSourceBuiltin = "builtin"
	PlatformAddonSourceCustom  = "custom"

	PlatformAddonStatusUnknown     = "unknown"
	PlatformAddonStatusInstalling  = "installing"
	PlatformAddonStatusAvailable   = "available"
	PlatformAddonStatusUnavailable = "unavailable"
	PlatformAddonStatusDegraded    = "degraded"
	PlatformAddonStatusDisabled    = "disabled"
	PlatformAddonStatusFailed      = "failed"
)

// ClusterAddon tracks cluster-level PAAP platform capabilities such as kpack,
// KEDA, KubeVirt, CDI, and metrics-server.
type ClusterAddon struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`

	ClusterID    uint   `gorm:"default:0;uniqueIndex:idx_cluster_addon_name" json:"clusterId"`
	Name         string `gorm:"size:64;not null;uniqueIndex:idx_cluster_addon_name" json:"name"`
	DisplayName  string `gorm:"size:128;not null" json:"displayName"`
	Category     string `gorm:"size:64;not null;default:platform" json:"category"`
	Source       string `gorm:"size:32;not null;default:builtin" json:"source"`
	Namespace    string `gorm:"size:128" json:"namespace"`
	Version      string `gorm:"size:64" json:"version"`
	InstallMode  string `gorm:"size:32;not null;default:manifest" json:"installMode"`
	S3Bucket     string `gorm:"size:100" json:"s3Bucket,omitempty"`
	S3Key        string `gorm:"size:500" json:"s3Key,omitempty"`
	DependsOn    string `gorm:"type:text" json:"dependsOn,omitempty"`
	Capabilities string `gorm:"type:text" json:"capabilities,omitempty"`

	DesiredState string `gorm:"size:32;not null;default:disabled" json:"desiredState"`
	Status       string `gorm:"size:32;not null;default:unknown" json:"status"`
	Config       string `gorm:"type:text" json:"config,omitempty"`
	Conditions   string `gorm:"type:text" json:"conditions,omitempty"`
	ErrorMessage string `gorm:"type:text" json:"errorMessage,omitempty"`
	Description  string `gorm:"type:text" json:"description,omitempty"`
	Readme       string `gorm:"type:text" json:"readme,omitempty"`

	InstalledAt   *time.Time `json:"installedAt,omitempty"`
	LastCheckedAt *time.Time `json:"lastCheckedAt,omitempty"`
}
