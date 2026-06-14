package model

import (
	"time"

	"gorm.io/gorm"
)

// ComponentConfigTemplate defines reusable runtime configuration for business
// components. It stores the template shape used by the UI and expands into the
// existing ComponentConfig model when applied to a component.
type ComponentConfigTemplate struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`

	Key            string `gorm:"uniqueIndex;size:80;not null" json:"key"`
	Name           string `gorm:"size:80;not null" json:"name"`
	Description    string `gorm:"size:500" json:"description"`
	Framework      string `gorm:"size:40;default:auto" json:"framework"`
	BindingMode    string `gorm:"size:40;default:recommended" json:"bindingMode"`
	ComponentTypes string `gorm:"type:text" json:"componentTypes"`

	// Syntax documents the supported placeholder syntax. FieldsJSON declares the
	// user-facing inputs generated from the template, while the remaining JSON
	// columns map directly to the component drawer's config model.
	Syntax      string `gorm:"type:text" json:"syntax"`
	FieldsJSON  string `gorm:"type:text" json:"fieldsJson"`
	EnvJSON     string `gorm:"type:text" json:"envJson"`
	ConfigJSON  string `gorm:"type:text" json:"configJson"`
	SecretJSON  string `gorm:"type:text" json:"secretJson"`
	FileJSON    string `gorm:"type:text" json:"fileJson"`
	CommandJSON string `gorm:"type:text" json:"commandJson"`
	ArgsJSON    string `gorm:"type:text" json:"argsJson"`

	IsBuiltin bool `gorm:"default:false" json:"isBuiltin"`
	SortOrder int  `gorm:"default:100" json:"sortOrder"`
	Enabled   bool `gorm:"default:true" json:"enabled"`
}
