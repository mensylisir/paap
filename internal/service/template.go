package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"paap/internal/k8s"
	"paap/internal/model"

	"gorm.io/gorm"
)

var ErrTemplateNotFound = errors.New("template not found")

type CreateTemplateInput struct {
	Name         string
	Description  string
	Services     []string
	Infra        []string
	ResourceCPU  string
	ResourceMem  string
	ResourceDisk string
}

type UpdateTemplateInput struct {
	Name         *string
	Description  *string
	Services     *[]string
	Infra        *[]string
	ResourceCPU  *string
	ResourceMem  *string
	ResourceDisk *string
}

type SaveServiceTemplateInput struct {
	Type               string
	Name               string
	Category           string
	Description        string
	Installer          string
	ProvisionMode      string
	RuntimeSpec        string
	ChartRepo          string
	ChartName          string
	ChartVersion       string
	DefaultValues      string
	ConfigurableParams string
	Features           string
	RawYamlTemplate    string
}

type ServiceFeatureItem struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

type ServiceInstallationTemplateContext struct {
	Application  model.Application
	Environment  model.Environment
	Installation model.ServiceInstallation
}

func ListTemplates(db *gorm.DB) ([]model.EnvTemplate, error) {
	var templates []model.EnvTemplate
	if err := db.Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func GetTemplate(db *gorm.DB, id uint) (model.EnvTemplate, error) {
	var tmpl model.EnvTemplate
	if err := db.First(&tmpl, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.EnvTemplate{}, ErrTemplateNotFound
		}
		return model.EnvTemplate{}, err
	}
	return tmpl, nil
}

func CreateTemplate(db *gorm.DB, input CreateTemplateInput) (model.EnvTemplate, error) {
	tmpl := model.EnvTemplate{
		Name:         input.Name,
		Description:  input.Description,
		ResourceCPU:  input.ResourceCPU,
		ResourceMem:  input.ResourceMem,
		ResourceDisk: input.ResourceDisk,
	}
	if len(input.Services) > 0 {
		tmpl.Services = jsonString(input.Services)
	}
	if len(input.Infra) > 0 {
		tmpl.Infra = jsonString(input.Infra)
	}
	if err := db.Create(&tmpl).Error; err != nil {
		return model.EnvTemplate{}, err
	}
	return tmpl, nil
}

func UpdateTemplate(db *gorm.DB, id uint, input UpdateTemplateInput) error {
	if _, err := GetTemplate(db, id); err != nil {
		return err
	}

	updates := make(map[string]interface{})
	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		updates["name"] = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		updates["description"] = *input.Description
	}
	if input.ResourceCPU != nil && strings.TrimSpace(*input.ResourceCPU) != "" {
		updates["resource_cpu"] = strings.TrimSpace(*input.ResourceCPU)
	}
	if input.ResourceMem != nil && strings.TrimSpace(*input.ResourceMem) != "" {
		updates["resource_mem"] = strings.TrimSpace(*input.ResourceMem)
	}
	if input.ResourceDisk != nil && strings.TrimSpace(*input.ResourceDisk) != "" {
		updates["resource_disk"] = strings.TrimSpace(*input.ResourceDisk)
	}
	if input.Services != nil {
		updates["services"] = jsonString(*input.Services)
	}
	if input.Infra != nil {
		updates["infra"] = jsonString(*input.Infra)
	}
	return db.Model(&model.EnvTemplate{}).Where("id = ?", id).Updates(updates).Error
}

func DeleteTemplate(db *gorm.DB, id uint) error {
	return db.Delete(&model.EnvTemplate{}, id).Error
}

func SeedEnvTemplateEntries(db *gorm.DB, entries []model.EnvTemplate) error {
	var errs []error
	for _, entry := range entries {
		var existing model.EnvTemplate
		if err := db.Where("name = ?", entry.Name).First(&existing).Error; err == nil {
			entry.ID = existing.ID
			if err := db.Model(&existing).Updates(map[string]interface{}{
				"description":   entry.Description,
				"services":      entry.Services,
				"infra":         entry.Infra,
				"resource_cpu":  entry.ResourceCPU,
				"resource_mem":  entry.ResourceMem,
				"resource_disk": entry.ResourceDisk,
			}).Error; err != nil {
				errs = append(errs, fmt.Errorf("refresh env template %s: %w", entry.Name, err))
			}
			continue
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			errs = append(errs, fmt.Errorf("load env template %s: %w", entry.Name, err))
			continue
		}
		if err := db.Create(&entry).Error; err != nil {
			errs = append(errs, fmt.Errorf("seed env template %s: %w", entry.Name, err))
		}
	}
	return errors.Join(errs...)
}

func ListServiceTemplates(db *gorm.DB) ([]model.ServiceTemplate, error) {
	var templates []model.ServiceTemplate
	if err := db.Where("enabled = ?", true).Order("install_order").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func GetServiceTemplate(db *gorm.DB, id uint) (model.ServiceTemplate, error) {
	var tmpl model.ServiceTemplate
	if err := db.First(&tmpl, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ServiceTemplate{}, ErrTemplateNotFound
		}
		return model.ServiceTemplate{}, err
	}
	return tmpl, nil
}

func CreateServiceTemplate(db *gorm.DB, input SaveServiceTemplateInput) (model.ServiceTemplate, error) {
	provisionMode := NormalizeServiceProvisionMode(input.ProvisionMode)
	runtimeSpec := strings.TrimSpace(input.RuntimeSpec)
	if err := validateServiceTemplateRuntimeSpec(provisionMode, runtimeSpec); err != nil {
		return model.ServiceTemplate{}, err
	}
	tmpl := model.ServiceTemplate{
		Type:               input.Type,
		Name:               input.Name,
		Category:           input.Category,
		Description:        input.Description,
		Installer:          input.Installer,
		ProvisionMode:      provisionMode,
		RuntimeSpec:        runtimeSpec,
		ChartRepo:          input.ChartRepo,
		ChartName:          input.ChartName,
		ChartVersion:       input.ChartVersion,
		DefaultValues:      input.DefaultValues,
		ConfigurableParams: input.ConfigurableParams,
		SupportedFeatures:  serviceTemplateFeatures(input.Features, input.Type, input.Category),
		RawYamlTemplate:    input.RawYamlTemplate,
		Enabled:            true,
	}
	if err := db.Create(&tmpl).Error; err != nil {
		return model.ServiceTemplate{}, err
	}
	return tmpl, nil
}

func UpdateServiceTemplate(db *gorm.DB, id uint, input SaveServiceTemplateInput) (model.ServiceTemplate, error) {
	tmpl, err := GetServiceTemplate(db, id)
	if err != nil {
		return model.ServiceTemplate{}, err
	}
	provisionMode := NormalizeServiceProvisionMode(input.ProvisionMode)
	runtimeSpec := strings.TrimSpace(input.RuntimeSpec)
	if err := validateServiceTemplateRuntimeSpec(provisionMode, runtimeSpec); err != nil {
		return model.ServiceTemplate{}, err
	}

	updates := map[string]interface{}{
		"name":                input.Name,
		"description":         input.Description,
		"installer":           input.Installer,
		"provision_mode":      provisionMode,
		"runtime_spec":        runtimeSpec,
		"chart_repo":          input.ChartRepo,
		"chart_name":          input.ChartName,
		"chart_version":       input.ChartVersion,
		"default_values":      input.DefaultValues,
		"configurable_params": input.ConfigurableParams,
		"supported_features":  serviceTemplateFeatures(input.Features, tmpl.Type, tmpl.Category),
		"raw_yaml_template":   input.RawYamlTemplate,
	}
	if err := db.Model(&tmpl).Updates(updates).Error; err != nil {
		return model.ServiceTemplate{}, err
	}
	return tmpl, nil
}

func validateServiceTemplateRuntimeSpec(provisionMode string, runtimeSpec string) error {
	if provisionMode != model.ServiceProvisionModeKubeVirt {
		return nil
	}
	if runtimeSpec == "" {
		return fmt.Errorf("kubevirt runtime spec is required")
	}
	if err := k8s.ValidateKubeVirtRuntimeSpec(runtimeSpec); err != nil {
		return err
	}
	return nil
}

func DeleteServiceTemplate(db *gorm.DB, id uint) error {
	return db.Unscoped().Delete(&model.ServiceTemplate{}, id).Error
}

func ListServiceCatalog(db *gorm.DB) ([]model.ServiceCatalog, error) {
	var services []model.ServiceCatalog
	if err := db.Where("enabled = ?", true).Not("type IN ?", UnsupportedServiceCatalogTypes()).Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

func SeedServiceCatalogEntries(db *gorm.DB, entries []model.ServiceCatalog) error {
	var errs []error
	for _, entry := range entries {
		entry.Category = ProductServiceCategory(entry.Type, entry.Category)
		if strings.TrimSpace(entry.Features) == "" {
			entry.Features = ServiceFeatureMatrixJSON(entry.Type, entry.Category)
		}
		var existing model.ServiceCatalog
		if err := db.Where("type = ?", entry.Type).Assign(entry).FirstOrCreate(&existing).Error; err != nil {
			errs = append(errs, fmt.Errorf("seed service catalog %s: %w", entry.Type, err))
		}
	}
	if err := db.Model(&model.ServiceCatalog{}).Where("type IN ?", UnsupportedServiceCatalogTypes()).Update("enabled", false).Error; err != nil {
		errs = append(errs, fmt.Errorf("disable unsupported service catalog entries: %w", err))
	}
	if err := db.Where("type = ?", "docker-registry").Delete(&model.ServiceCatalog{}).Error; err != nil {
		errs = append(errs, fmt.Errorf("remove obsolete docker-registry catalog: %w", err))
	}
	return errors.Join(errs...)
}

func MigrateServiceTemplateUniqueIndex(db *gorm.DB, oldIndexName string) (bool, error) {
	if !db.Migrator().HasIndex(&model.ServiceTemplate{}, oldIndexName) {
		return false, nil
	}
	if err := db.Migrator().DropIndex(&model.ServiceTemplate{}, oldIndexName); err != nil {
		return true, err
	}
	return true, nil
}

func UpsertSeedServiceTemplate(db *gorm.DB, tmpl model.ServiceTemplate) error {
	tmpl.ProvisionMode = NormalizeServiceProvisionMode(tmpl.ProvisionMode)
	tmpl.Category = ProductServiceCategory(tmpl.Type, tmpl.Category)
	if strings.TrimSpace(tmpl.SupportedFeatures) == "" {
		tmpl.SupportedFeatures = ServiceFeatureMatrixJSON(tmpl.Type, tmpl.Category)
	}
	var existing model.ServiceTemplate
	query := db.Where("type = ?", tmpl.Type)
	if tmpl.ProvisionMode == model.ServiceProvisionModeManaged {
		query = query.Where("(provision_mode = ? OR provision_mode = '' OR provision_mode IS NULL)", model.ServiceProvisionModeManaged)
	} else {
		query = query.Where("provision_mode = ?", tmpl.ProvisionMode)
	}
	if strings.TrimSpace(tmpl.S3Key) != "" {
		query = query.Where("s3_key = ?", tmpl.S3Key)
	} else {
		query = query.Where("(s3_key = '' OR s3_key IS NULL)")
	}
	if err := query.First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Create(&tmpl).Error
		}
		return err
	}
	tmpl.ID = existing.ID
	return db.Model(&existing).Updates(tmpl).Error
}

func RemoveObsoleteDockerRegistryTemplate(db *gorm.DB) error {
	var errs []error
	if err := db.Where("type = ?", "docker-registry").Delete(&model.ServiceTemplate{}).Error; err != nil {
		errs = append(errs, fmt.Errorf("remove obsolete docker-registry template: %w", err))
	}
	if err := db.Where("service_type = ?", "docker-registry").Delete(&model.ServiceInstallation{}).Error; err != nil {
		errs = append(errs, fmt.Errorf("remove obsolete docker-registry installs: %w", err))
	}
	return errors.Join(errs...)
}

func SyncBuiltinServiceTemplateRecord(db *gorm.DB, serviceType, s3Key string, updates map[string]interface{}, fallback model.ServiceTemplate, canCreate bool) (model.ServiceTemplate, bool, error) {
	result := db.Model(&model.ServiceTemplate{}).Where("type = ? AND s3_key = ?", serviceType, s3Key).Updates(updates)
	if result.Error != nil {
		return model.ServiceTemplate{}, false, result.Error
	}
	changed := result.RowsAffected > 0
	if !changed && canCreate {
		if err := db.Create(&fallback).Error; err != nil {
			return model.ServiceTemplate{}, false, err
		}
		changed = true
	}

	var refreshed model.ServiceTemplate
	if err := db.Where("type = ? AND s3_key = ?", serviceType, s3Key).First(&refreshed).Error; err != nil {
		return model.ServiceTemplate{}, changed, err
	}
	return refreshed, changed, nil
}

func ListServiceInstallationTemplateContexts(db *gorm.DB, serviceType string) ([]ServiceInstallationTemplateContext, error) {
	var installs []model.ServiceInstallation
	if err := db.Where("service_type = ?", serviceType).Find(&installs).Error; err != nil {
		return nil, err
	}
	contexts := make([]ServiceInstallationTemplateContext, 0, len(installs))
	for _, inst := range installs {
		env, err := findEnvironment(db, inst.EnvironmentID)
		if err != nil {
			return nil, err
		}
		app, err := findApplication(db, env.ApplicationID)
		if err != nil {
			return nil, err
		}
		contexts = append(contexts, ServiceInstallationTemplateContext{
			Application:  app,
			Environment:  env,
			Installation: inst,
		})
	}
	return contexts, nil
}

func SaveServiceInstallation(db *gorm.DB, inst model.ServiceInstallation) error {
	inst.ProvisionMode = NormalizeServiceProvisionMode(inst.ProvisionMode)
	return db.Save(&inst).Error
}

func CreateUploadedServiceTemplate(db *gorm.DB, tmpl model.ServiceTemplate) (model.ServiceTemplate, error) {
	if err := db.Create(&tmpl).Error; err != nil {
		return model.ServiceTemplate{}, err
	}
	return tmpl, nil
}

func UnsupportedServiceCatalogTypes() []string {
	return []string{"kingbase"}
}

func ProductServiceCategory(serviceType, fallback string) string {
	serviceType = strings.ToLower(strings.TrimSpace(serviceType))
	switch serviceType {
	case "ci", "jenkins", "tekton":
		return "ci"
	case "deploy", "cd", "argocd":
		return "cd"
	case "monitor", "prometheus", "grafana", "kube-prometheus":
		return "monitor"
	case "log", "logging", "loki":
		return "log"
	case "postgresql", "postgresql-ha", "mysql", "mysql-galera", "mongodb", "kingbase":
		return "database"
	case "redis", "redis-cluster", "rabbitmq", "kafka", "minio", "nacos", "eureka", "registry", "harbor", "git":
		return "middleware"
	case "environment":
		return "environment"
	case "kubevirt", "vm":
		return "virtualMachine"
	}
	category := strings.ToLower(strings.TrimSpace(fallback))
	switch category {
	case "ci", "cd", "monitor", "log", "database", "middleware", "environment", "virtualmachine", "other":
		return category
	case "deploy":
		return "cd"
	case "observability":
		return "monitor"
	case "logging":
		return "log"
	case "infra", "tool":
		return "middleware"
	case "vm", "kubevirt":
		return "virtualMachine"
	default:
		return "other"
	}
}

func ServiceFeatureMatrixJSON(serviceType, category string) string {
	serviceType = strings.ToLower(strings.TrimSpace(serviceType))
	category = ProductServiceCategory(serviceType, category)
	external := supportsExternalServiceFeature(serviceType, category)
	kubevirt := supportsKubeVirtServiceFeature(serviceType)
	return jsonString([]ServiceFeatureItem{
		{Key: "managed", Label: "环境内创建", Enabled: true},
		{Key: "shared", Label: "公共服务", Enabled: true},
		{Key: "external", Label: "外部连接", Enabled: external},
		{Key: "kubevirt", Label: "KubeVirt 模板", Enabled: kubevirt},
	})
}

func serviceTemplateFeatures(value, serviceType, category string) string {
	features := strings.TrimSpace(value)
	if features == "" {
		features = ServiceFeatureMatrixJSON(serviceType, category)
	}
	return features
}

func supportsExternalServiceFeature(serviceType, category string) bool {
	switch serviceType {
	case "postgresql", "mysql", "mongodb", "redis", "rabbitmq", "kafka", "minio", "git", "registry", "harbor", "ci", "deploy", "monitor", "log":
		return true
	default:
		return category == "database" || category == "middleware"
	}
}

func supportsKubeVirtServiceFeature(serviceType string) bool {
	switch serviceType {
	case "postgresql", "mysql", "mongodb", "redis":
		return true
	default:
		return false
	}
}

func NormalizeServiceProvisionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case model.ServiceProvisionModeKubeVirt:
		return model.ServiceProvisionModeKubeVirt
	default:
		return model.ServiceProvisionModeManaged
	}
}

func jsonString(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}
