package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"paap/internal/k8s"
	"paap/internal/model"

	"gorm.io/gorm"
)

var (
	ErrSystemEnvironmentDelete     = errors.New("system environments cannot be deleted")
	ErrServiceInstallationNotFound = errors.New("service installation not found")
	ErrServiceInstanceCRDelete     = errors.New("delete service instance cr failed")
)

func DeleteEnvironment(ctx context.Context, db *gorm.DB, envID uint) (string, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return "", err
	}
	if env.IsSystem {
		return "", ErrSystemEnvironmentDelete
	}

	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return "", err
	}

	warning := ""
	if err := k8s.DeleteEnvironmentScopedResources(ctx, app.Identifier, env.Identifier); err != nil {
		warning = "Environment cluster cleanup failed: " + err.Error()
	}

	return warning, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.ServiceInstallation{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.InfraInstallation{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.Component{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.EnvironmentCanvasState{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&env).Error
	})
}

func UninstallService(ctx context.Context, db *gorm.DB, envID uint, serviceID uint) error {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return err
	}

	var inst model.ServiceInstallation
	if err := db.Where("id = ? AND environment_id = ?", serviceID, envID).First(&inst).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrServiceInstallationNotFound
		}
		return err
	}

	if ServiceInstallationRequiresRuntimeDelete(inst) {
		if NormalizeServiceProvisionMode(inst.ProvisionMode) == model.ServiceProvisionModeKubeVirt {
			if err := k8s.DeleteKubeVirtServiceResources(ctx, inst.Namespace, inst.ServiceType); err != nil {
				return fmt.Errorf("%w: %v", ErrServiceInstanceCRDelete, err)
			}
		} else if err := k8s.DeleteServiceInstanceCR(ctx, app.Identifier, env.Identifier, inst.ServiceType); err != nil {
			return fmt.Errorf("%w: %v", ErrServiceInstanceCRDelete, err)
		}
	}

	if NormalizeServiceProvisionMode(inst.ProvisionMode) != model.ServiceProvisionModeKubeVirt {
		if err := k8s.NewClient().DeleteNamespace(inst.Namespace); err != nil {
			log.Printf("[UninstallService] namespace delete warning: %v", err)
		}
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Delete(&inst).Error; err != nil {
			return err
		}
		return MarkEnvironmentEmptyWhenNoResources(tx, env.ID)
	})
}

func ServiceInstallationRequiresRuntimeDelete(inst model.ServiceInstallation) bool {
	return !strings.EqualFold(strings.TrimSpace(inst.Status), "draft")
}

func MarkEnvironmentEmptyWhenNoResources(db *gorm.DB, environmentID uint) error {
	var total int64
	for _, table := range []interface{}{
		&model.ServiceInstallation{},
		&model.Component{},
		&model.InfraInstallation{},
	} {
		var count int64
		if err := db.Model(table).Where("environment_id = ?", environmentID).Count(&count).Error; err != nil {
			return err
		}
		total += count
	}
	if total > 0 {
		return nil
	}
	return db.Model(&model.Environment{}).
		Where("id = ?", environmentID).
		Updates(map[string]any{"status": "empty", "template_id": 0, "error_message": ""}).Error
}
