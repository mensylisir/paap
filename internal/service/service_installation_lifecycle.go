package service

import (
	"encoding/json"
	"errors"
	"log"
	"strings"

	"paap/internal/model"

	"gorm.io/gorm"
)

var ErrServiceNamespaceNotReady = errors.New("service namespace is not ready")

type ServiceInstallTemplateQuery struct {
	ChartVersion  string
	AppVersion    string
	ProvisionMode string
	EnabledOnly   bool
}

type ServiceInstallTemplateContext struct {
	App      model.Application
	Env      model.Environment
	Template model.ServiceTemplate
}

type ServiceInstallationRuntimeInput struct {
	ServiceName   string
	Namespace     string
	ReleaseName   string
	ProvisionMode string
	RuntimeSpec   string
	Values        string
	ErrorMessage  string
}

type ServiceInstallationUpdateContext struct {
	App          model.Application
	Env          model.Environment
	Installation model.ServiceInstallation
	Template     model.ServiceTemplate
}

type ServiceExternalAccessContext struct {
	Env          model.Environment
	Installation model.ServiceInstallation
}

type EnvironmentServiceInstallPlan struct {
	Services []string
}

func LoadServiceInstallTemplateContext(db *gorm.DB, envID uint, serviceType string, query ServiceInstallTemplateQuery) (ServiceInstallTemplateContext, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return ServiceInstallTemplateContext{}, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return ServiceInstallTemplateContext{}, err
	}

	dbQuery := db.Where("type = ?", serviceType)
	if query.EnabledOnly {
		dbQuery = dbQuery.Where("enabled = ?", true)
	}
	if strings.TrimSpace(query.ChartVersion) != "" {
		dbQuery = dbQuery.Where("chart_version = ?", strings.TrimSpace(query.ChartVersion))
	}
	if strings.TrimSpace(query.AppVersion) != "" {
		dbQuery = dbQuery.Where("app_version = ?", strings.TrimSpace(query.AppVersion))
	}
	provisionMode := NormalizeServiceProvisionMode(query.ProvisionMode)
	if provisionMode == model.ServiceProvisionModeManaged {
		dbQuery = dbQuery.Where("(provision_mode = ? OR provision_mode = '' OR provision_mode IS NULL)", model.ServiceProvisionModeManaged)
	} else {
		dbQuery = dbQuery.Where("provision_mode = ?", provisionMode)
	}

	var tmpl model.ServiceTemplate
	if err := dbQuery.Order("install_order").First(&tmpl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ServiceInstallTemplateContext{}, ErrTemplateNotFound
		}
		return ServiceInstallTemplateContext{}, err
	}
	return ServiceInstallTemplateContext{App: app, Env: env, Template: tmpl}, nil
}

func BuildEnvironmentServiceInstallPlan(db *gorm.DB, templateID uint, foundation []string) (EnvironmentServiceInstallPlan, error) {
	services := appendServiceTypes(nil, foundation...)
	if templateID == 0 {
		return EnvironmentServiceInstallPlan{Services: services}, nil
	}
	tmpl, err := GetTemplate(db, templateID)
	if err != nil {
		return EnvironmentServiceInstallPlan{}, err
	}
	services = appendServiceTypes(services, templateInstallServiceTypes(tmpl)...)
	return EnvironmentServiceInstallPlan{Services: services}, nil
}

func LoadServiceTemplateForInstall(db *gorm.DB, serviceType string) (model.ServiceTemplate, error) {
	return loadServiceTemplateByType(db, serviceType, false)
}

func SaveServiceDraft(db *gorm.DB, env model.Environment, serviceType string, input ServiceInstallationRuntimeInput) (model.ServiceInstallation, bool, error) {
	provisionMode := NormalizeServiceProvisionMode(input.ProvisionMode)
	var inst model.ServiceInstallation
	err := db.Unscoped().
		Where("environment_id = ? AND service_type = ? AND provision_mode = ?", env.ID, serviceType, provisionMode).
		Order("CASE WHEN deleted_at IS NULL THEN 0 ELSE 1 END, id").
		First(&inst).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ServiceInstallation{}, false, err
	}

	created := false
	if err == nil {
		if inst.DeletedAt.Valid {
			inst.DeletedAt = gorm.DeletedAt{}
			created = true
		}
		switch strings.ToLower(strings.TrimSpace(inst.Status)) {
		case "running", "installing", "deleting":
			if err := db.Save(&inst).Error; err != nil {
				return model.ServiceInstallation{}, false, err
			}
			return inst, false, nil
		}
	} else {
		created = true
		inst = model.ServiceInstallation{
			EnvironmentID: env.ID,
			ServiceType:   serviceType,
		}
	}

	inst.Status = "draft"
	inst.ServiceName = input.ServiceName
	inst.Namespace = input.Namespace
	inst.ReleaseName = input.ReleaseName
	inst.ProvisionMode = provisionMode
	inst.RuntimeSpec = strings.TrimSpace(input.RuntimeSpec)
	inst.ErrorMessage = input.ErrorMessage
	inst.Values = input.Values
	if err := db.Save(&inst).Error; err != nil {
		return model.ServiceInstallation{}, false, err
	}
	if err := deleteDuplicateServiceInstallations(db, env.ID, serviceType, provisionMode, inst.ID); err != nil {
		return model.ServiceInstallation{}, false, err
	}
	return inst, created, nil
}

func SaveServiceInstalling(db *gorm.DB, env model.Environment, serviceType string, input ServiceInstallationRuntimeInput) (model.ServiceInstallation, error) {
	provisionMode := NormalizeServiceProvisionMode(input.ProvisionMode)
	if err := db.Unscoped().
		Where("environment_id = ? AND service_type = ? AND provision_mode = ? AND deleted_at IS NOT NULL", env.ID, serviceType, provisionMode).
		Delete(&model.ServiceInstallation{}).Error; err != nil {
		return model.ServiceInstallation{}, err
	}

	var inst model.ServiceInstallation
	err := db.Where("environment_id = ? AND service_type = ? AND provision_mode = ?", env.ID, serviceType, provisionMode).First(&inst).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ServiceInstallation{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		inst = model.ServiceInstallation{
			EnvironmentID: env.ID,
			ServiceType:   serviceType,
		}
	}

	inst.Status = "installing"
	inst.ServiceName = input.ServiceName
	inst.Namespace = input.Namespace
	inst.ReleaseName = input.ReleaseName
	inst.ProvisionMode = provisionMode
	inst.RuntimeSpec = strings.TrimSpace(input.RuntimeSpec)
	inst.ErrorMessage = input.ErrorMessage
	inst.Values = input.Values
	if err := db.Save(&inst).Error; err != nil {
		return model.ServiceInstallation{}, err
	}
	if err := deleteDuplicateServiceInstallations(db, env.ID, serviceType, provisionMode, inst.ID); err != nil {
		return model.ServiceInstallation{}, err
	}
	return inst, nil
}

func LoadServiceInstallationUpdateContext(db *gorm.DB, envID uint, serviceID uint) (ServiceInstallationUpdateContext, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return ServiceInstallationUpdateContext{}, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return ServiceInstallationUpdateContext{}, err
	}
	inst, err := findServiceInstallation(db, envID, serviceID)
	if err != nil {
		return ServiceInstallationUpdateContext{}, err
	}
	tmpl, err := loadServiceTemplateByType(db, inst.ServiceType, false)
	if err != nil {
		return ServiceInstallationUpdateContext{}, err
	}
	return ServiceInstallationUpdateContext{App: app, Env: env, Installation: inst, Template: tmpl}, nil
}

func SaveServiceInstallationRuntime(db *gorm.DB, inst model.ServiceInstallation, input ServiceInstallationRuntimeInput) (model.ServiceInstallation, error) {
	inst.ServiceName = input.ServiceName
	inst.Namespace = input.Namespace
	inst.ReleaseName = input.ReleaseName
	inst.ProvisionMode = NormalizeServiceProvisionMode(input.ProvisionMode)
	inst.RuntimeSpec = strings.TrimSpace(input.RuntimeSpec)
	inst.Values = input.Values
	if strings.TrimSpace(inst.Status) == "" || strings.EqualFold(inst.Status, "pending") {
		inst.Status = "draft"
	}
	if strings.EqualFold(inst.Status, "draft") {
		inst.ErrorMessage = ""
	} else {
		inst.ErrorMessage = input.ErrorMessage
	}
	if err := db.Save(&inst).Error; err != nil {
		return model.ServiceInstallation{}, err
	}
	return inst, nil
}

func MarkServiceInstallationFailed(db *gorm.DB, inst model.ServiceInstallation, message string) (model.ServiceInstallation, error) {
	inst.Status = "failed"
	inst.ErrorMessage = message
	if err := db.Save(&inst).Error; err != nil {
		return model.ServiceInstallation{}, err
	}
	return inst, nil
}

func MarkServiceInstallationInstalling(db *gorm.DB, inst model.ServiceInstallation) (model.ServiceInstallation, error) {
	inst.Status = "installing"
	if err := db.Save(&inst).Error; err != nil {
		return model.ServiceInstallation{}, err
	}
	return inst, nil
}

func MarkServiceInstallationRunning(db *gorm.DB, inst model.ServiceInstallation) (model.ServiceInstallation, error) {
	inst.Status = "running"
	inst.ErrorMessage = ""
	if err := db.Save(&inst).Error; err != nil {
		return model.ServiceInstallation{}, err
	}
	return inst, nil
}

func LoadServiceExternalAccessContext(db *gorm.DB, envID uint, serviceID uint) (ServiceExternalAccessContext, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return ServiceExternalAccessContext{}, err
	}
	inst, err := findServiceInstallation(db, env.ID, serviceID)
	if err != nil {
		return ServiceExternalAccessContext{}, err
	}
	if strings.TrimSpace(inst.Namespace) == "" {
		return ServiceExternalAccessContext{}, ErrServiceNamespaceNotReady
	}
	return ServiceExternalAccessContext{Env: env, Installation: inst}, nil
}

func ListEnvironmentServiceInstallations(db *gorm.DB, envID uint) ([]model.ServiceInstallation, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return nil, err
	}
	var services []model.ServiceInstallation
	if err := db.Where("environment_id = ?", env.ID).Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

func FindEnvironmentServiceByType(db *gorm.DB, envID uint, serviceType string) (model.ServiceInstallation, bool, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return model.ServiceInstallation{}, false, err
	}
	var inst model.ServiceInstallation
	if err := db.Where("environment_id = ? AND service_type = ?", env.ID, serviceType).First(&inst).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ServiceInstallation{}, false, nil
		}
		return model.ServiceInstallation{}, false, err
	}
	return inst, true, nil
}

func CreateInfraInstallation(db *gorm.DB, envID uint, infraType string) (model.Environment, model.InfraInstallation, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return model.Environment{}, model.InfraInstallation{}, err
	}
	inst := model.InfraInstallation{
		EnvironmentID: env.ID,
		InfraType:     infraType,
		Status:        "installing",
	}
	if err := db.Create(&inst).Error; err != nil {
		return model.Environment{}, model.InfraInstallation{}, err
	}
	return env, inst, nil
}

func SaveInfraInstallationStatus(db *gorm.DB, inst model.InfraInstallation, status string, message string) error {
	inst.Status = status
	inst.ErrorMessage = message
	return db.Save(&inst).Error
}

func deleteDuplicateServiceInstallations(db *gorm.DB, envID uint, serviceType string, provisionMode string, keepID uint) error {
	return db.Unscoped().
		Where("environment_id = ? AND service_type = ? AND provision_mode = ? AND id <> ?", envID, serviceType, NormalizeServiceProvisionMode(provisionMode), keepID).
		Delete(&model.ServiceInstallation{}).Error
}

func loadServiceTemplateByType(db *gorm.DB, serviceType string, enabledOnly bool) (model.ServiceTemplate, error) {
	query := db.
		Where("type = ?", serviceType).
		Where("(provision_mode = ? OR provision_mode = '' OR provision_mode IS NULL)", model.ServiceProvisionModeManaged)
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	var tmpl model.ServiceTemplate
	if err := query.Order("install_order").First(&tmpl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ServiceTemplate{}, ErrTemplateNotFound
		}
		return model.ServiceTemplate{}, err
	}
	return tmpl, nil
}

func appendServiceTypes(base []string, extra ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(base)+len(extra))
	for _, value := range append(base, extra...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func templateInstallServiceTypes(tmpl model.EnvTemplate) []string {
	seen := map[string]bool{}
	result := make([]string, 0)
	appendList := func(raw, field string) {
		var values []string
		if strings.TrimSpace(raw) == "" {
			return
		}
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			log.Printf("[installTemplateServices] Failed to unmarshal %s: %v", field, err)
			return
		}
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			result = append(result, value)
		}
	}
	appendList(tmpl.Services, "services")
	appendList(tmpl.Infra, "infra")
	return result
}
