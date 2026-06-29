package service

import (
	"strings"

	"paap/internal/model"

	"gorm.io/gorm"
)

type ComponentRuntimeContext struct {
	Env        model.Environment
	Component  model.Component
	Identifier string
}

func LoadComponentRuntimeContext(db *gorm.DB, envID uint, componentID uint) (ComponentRuntimeContext, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return ComponentRuntimeContext{}, err
	}
	comp, err := findEnvironmentComponent(db, env.ID, componentID)
	if err != nil {
		return ComponentRuntimeContext{}, err
	}
	return ComponentRuntimeContext{
		Env:        env,
		Component:  comp,
		Identifier: ComponentIdentifier(comp.Name, comp.Type, comp.ID),
	}, nil
}

func MonitorNamespaceForEnvironment(db *gorm.DB, envID uint) string {
	var inst model.ServiceInstallation
	if err := db.
		Where("environment_id = ? AND service_type = ?", envID, "monitor").
		Order("id DESC").
		First(&inst).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(inst.Namespace)
}
