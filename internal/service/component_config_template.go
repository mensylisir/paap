package service

import (
	"errors"

	"paap/internal/model"

	"gorm.io/gorm"
)

var (
	ErrComponentConfigTemplateNotFound = errors.New("template not found")
)

func ListComponentConfigTemplates(db *gorm.DB) ([]model.ComponentConfigTemplate, error) {
	var templates []model.ComponentConfigTemplate
	if err := db.Where("enabled = ?", true).Order("sort_order ASC, name ASC").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func CreateComponentConfigTemplate(db *gorm.DB, tmpl model.ComponentConfigTemplate) (model.ComponentConfigTemplate, bool, error) {
	var existing model.ComponentConfigTemplate
	err := db.Where("key = ?", tmpl.Key).First(&existing).Error
	if err == nil {
		tmpl.ID = existing.ID
		if err := db.Model(&existing).Updates(componentConfigTemplateUpdateMap(tmpl)).Error; err != nil {
			return model.ComponentConfigTemplate{}, false, err
		}
		if err := db.First(&existing, existing.ID).Error; err != nil {
			return model.ComponentConfigTemplate{}, false, err
		}
		return existing, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ComponentConfigTemplate{}, false, err
	}
	if err := db.Create(&tmpl).Error; err != nil {
		return model.ComponentConfigTemplate{}, false, err
	}
	return tmpl, true, nil
}

func UpdateComponentConfigTemplate(db *gorm.DB, id uint, tmpl model.ComponentConfigTemplate) (model.ComponentConfigTemplate, error) {
	var existing model.ComponentConfigTemplate
	if err := db.First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ComponentConfigTemplate{}, ErrComponentConfigTemplateNotFound
		}
		return model.ComponentConfigTemplate{}, err
	}
	tmpl.ID = existing.ID
	if err := db.Model(&existing).Updates(componentConfigTemplateUpdateMap(tmpl)).Error; err != nil {
		return model.ComponentConfigTemplate{}, err
	}
	if err := db.First(&existing, existing.ID).Error; err != nil {
		return model.ComponentConfigTemplate{}, err
	}
	return existing, nil
}

func DeleteComponentConfigTemplate(db *gorm.DB, id uint) error {
	var tmpl model.ComponentConfigTemplate
	if err := db.First(&tmpl, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrComponentConfigTemplateNotFound
		}
		return err
	}
	return db.Unscoped().Delete(&tmpl).Error
}

func componentConfigTemplateUpdateMap(tmpl model.ComponentConfigTemplate) map[string]interface{} {
	return map[string]interface{}{
		"key":             tmpl.Key,
		"name":            tmpl.Name,
		"description":     tmpl.Description,
		"framework":       tmpl.Framework,
		"binding_mode":    tmpl.BindingMode,
		"component_types": tmpl.ComponentTypes,
		"s3_bucket":       tmpl.S3Bucket,
		"s3_key":          tmpl.S3Key,
		"syntax":          tmpl.Syntax,
		"native_json":     tmpl.NativeJSON,
		"fields_json":     tmpl.FieldsJSON,
		"env_json":        tmpl.EnvJSON,
		"config_json":     tmpl.ConfigJSON,
		"secret_json":     tmpl.SecretJSON,
		"file_json":       tmpl.FileJSON,
		"command_json":    tmpl.CommandJSON,
		"args_json":       tmpl.ArgsJSON,
		"is_builtin":      tmpl.IsBuiltin,
		"sort_order":      tmpl.SortOrder,
		"enabled":         tmpl.Enabled,
	}
}
