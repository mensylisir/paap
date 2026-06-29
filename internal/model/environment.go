package model

import (
	"time"

	"gorm.io/gorm"
)

const (
	SystemSharedApplicationIdentifier = "default"
	SystemSharedEnvironmentIdentifier = "shared"
)

type Environment struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	ApplicationID uint           `gorm:"not null;index" json:"applicationId"`
	Application   Application    `gorm:"foreignKey:ApplicationID" json:"application,omitempty"`
	Name          string         `gorm:"size:50;not null" json:"name"`
	Identifier    string         `gorm:"size:50;not null" json:"identifier"`
	TemplateID    uint           `gorm:"default:0" json:"templateId"`
	Status        string         `gorm:"size:20;default:empty" json:"status"` // empty, running, stopped, creating, error
	Namespace     string         `gorm:"size:100" json:"namespace"`
	ErrorMessage  string         `gorm:"type:text" json:"errorMessage,omitempty"` // 环境创建/运行过程中的错误信息
	IsSystem      bool           `gorm:"default:false;index" json:"isSystem"`
}

func IsSystemSharedEnvironment(app Application, env Environment) bool {
	return app.IsSystem &&
		app.Identifier == SystemSharedApplicationIdentifier &&
		env.IsSystem &&
		env.Identifier == SystemSharedEnvironmentIdentifier
}

type EnvironmentCanvasState struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	EnvironmentID uint           `gorm:"not null;uniqueIndex" json:"environmentId"`
	Positions     string         `gorm:"type:text" json:"positions"`
	Edges         string         `gorm:"type:text" json:"edges"`
	Names         string         `gorm:"type:text" json:"names"`
}

type EnvironmentTemplate struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	Name         string         `gorm:"size:50;not null" json:"name"`
	Description  string         `gorm:"size:200" json:"description"`
	Services     string         `gorm:"type:text" json:"services"` // JSON array of service types
	ResourceCPU  string         `gorm:"size:20;default:4核" json:"resourceCpu"`
	ResourceMem  string         `gorm:"size:20;default:8GB" json:"resourceMem"`
	ResourceDisk string         `gorm:"size:20;default:50GB" json:"resourceDisk"`
}
