package service

import (
	"context"
	"errors"
	"strings"

	paapv1 "paap/api/v1"
	"paap/internal/model"

	"gorm.io/gorm"
)

var ErrSystemApplicationEnvironmentCreate = errors.New("system applications cannot create additional environments")

type CreateEnvironmentInput struct {
	Name         string
	Identifier   string
	TemplateID   uint
	FromEmpty    bool
	Blank        bool
	Capabilities []EnvironmentCapabilityRequest
	CreatedBy    uint
}

type CreatedEnvironment struct {
	Application   model.Application
	Environment   model.Environment
	ResourceQuota *paapv1.ResourceQuotaSpec
}

func CreateEnvironment(ctx context.Context, db *gorm.DB, appID uint, input CreateEnvironmentInput) (CreatedEnvironment, error) {
	app, err := findApplication(db, appID)
	if err != nil {
		return CreatedEnvironment{}, err
	}
	if app.IsSystem {
		return CreatedEnvironment{}, ErrSystemApplicationEnvironmentCreate
	}

	identifier, err := uniqueIdentifierWithFallback(db, firstNonEmpty(input.Identifier, input.Name), "env", 50, func(db *gorm.DB, candidate string) (bool, error) {
		var count int64
		if err := db.Model(&model.Environment{}).Where("application_id = ? AND identifier = ?", appID, candidate).Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	})
	if err != nil {
		return CreatedEnvironment{}, err
	}

	env := model.Environment{
		ApplicationID: app.ID,
		Name:          input.Name,
		Identifier:    identifier,
		TemplateID:    input.TemplateID,
		Status:        "creating",
		Namespace:     app.Identifier + "-" + identifier,
	}
	if input.FromEmpty || input.Blank {
		env.TemplateID = 0
	}

	resourceQuota, err := EnvironmentTemplateResourceQuota(db, env.TemplateID)
	if err != nil {
		return CreatedEnvironment{}, err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&env).Error; err != nil {
			return err
		}
		if len(input.Capabilities) > 0 {
			if err := CreateInitialEnvironmentCapabilities(ctx, tx, env, input.Capabilities, input.CreatedBy); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return CreatedEnvironment{}, err
	}

	return CreatedEnvironment{Application: app, Environment: env, ResourceQuota: resourceQuota}, nil
}

func EnvironmentTemplateResourceQuota(db *gorm.DB, templateID uint) (*paapv1.ResourceQuotaSpec, error) {
	if templateID == 0 {
		return nil, nil
	}
	tmpl, err := GetTemplate(db, templateID)
	if err != nil {
		if errors.Is(err, ErrTemplateNotFound) {
			return nil, nil
		}
		return nil, err
	}
	cpu := strings.TrimSpace(tmpl.ResourceCPU)
	memory := strings.TrimSpace(tmpl.ResourceMem)
	storage := strings.TrimSpace(tmpl.ResourceDisk)
	if cpu == "" && memory == "" && storage == "" {
		return nil, nil
	}
	return &paapv1.ResourceQuotaSpec{
		CPU:     cpu,
		Memory:  memory,
		Storage: storage,
	}, nil
}

func UpdateEnvironmentStatus(db *gorm.DB, envID uint, status string, errorMessage string) (model.Environment, error) {
	updates := map[string]interface{}{"status": status}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	} else {
		updates["error_message"] = ""
	}
	if err := db.Model(&model.Environment{}).Where("id = ?", envID).Updates(updates).Error; err != nil {
		return model.Environment{}, err
	}
	return findEnvironment(db, envID)
}
