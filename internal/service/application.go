package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"paap/internal/authz"
	"paap/internal/k8s"
	"paap/internal/model"
	"paap/internal/permission"

	"gorm.io/gorm"
)

var (
	ErrSystemApplicationDelete = errors.New("system applications cannot be deleted")
)

type CreateApplicationInput struct {
	Name        string
	Identifier  string
	Description string
	OwnerID     uint
}

type UpdateApplicationInput struct {
	Name        string
	Description string
}

type ApplicationListItem struct {
	model.Application
	Environments     []EnvironmentListItem `json:"environments"`
	EnvironmentCount int                   `json:"environmentCount"`
}

type EnvironmentListItem struct {
	model.Environment
	ToolCount      int                     `json:"toolCount"`
	ComponentCount int                     `json:"componentCount"`
	Services       []ServiceStatusListItem `json:"services"`
}

type ServiceStatusListItem struct {
	ServiceType  string `json:"serviceType"`
	Status       string `json:"status"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
}

type ApplicationDetail struct {
	Application  model.Application     `json:"application"`
	Environments []EnvironmentListItem `json:"environments"`
	Members      []model.AppMember     `json:"members"`
}

func ListApplications(db *gorm.DB, userID uint, platformAdmin bool) ([]ApplicationListItem, error) {
	query := db.Model(&model.Application{})
	if !platformAdmin {
		query = query.Where("applications.is_system = ?", false)
		query = query.Joins(
			`JOIN role_bindings ON role_bindings.scope_id = applications.id
				AND role_bindings.scope_type = 'app'
				AND role_bindings.user_id = ?
				AND role_bindings.deleted_at IS NULL
			JOIN roles ON roles.id = role_bindings.role_id
				AND roles.deleted_at IS NULL
				AND roles.enabled = TRUE
			JOIN role_permissions ON role_permissions.role_id = roles.id
				AND role_permissions.deleted_at IS NULL
			JOIN permissions ON permissions.id = role_permissions.permission_id
				AND permissions.deleted_at IS NULL
				AND permissions.enabled = TRUE
				AND permissions.code = ?`,
			userID,
			permission.AppRead,
		)
	}

	var apps []model.Application
	if err := query.Order("applications.is_system DESC").Order("applications.id").Find(&apps).Error; err != nil {
		return nil, err
	}

	items := make([]ApplicationListItem, 0, len(apps))
	for _, app := range apps {
		envItems, err := listApplicationEnvironmentItems(db, app.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, ApplicationListItem{
			Application:      app,
			Environments:     envItems,
			EnvironmentCount: len(envItems),
		})
	}
	return items, nil
}

func CreateApplication(ctx context.Context, db *gorm.DB, input CreateApplicationInput) (model.Application, string, error) {
	if input.OwnerID == 0 {
		return model.Application{}, "", ValidationError{Message: "invalid or missing authenticated user"}
	}
	identifier, err := uniqueIdentifierWithFallback(db, firstNonEmpty(input.Identifier, input.Name), "app", 50, func(db *gorm.DB, candidate string) (bool, error) {
		var count int64
		if err := db.Model(&model.Application{}).Where("identifier = ?", candidate).Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	})
	if err != nil {
		return model.Application{}, "", err
	}

	app := model.Application{
		Name:        input.Name,
		Identifier:  identifier,
		Description: input.Description,
		OwnerID:     input.OwnerID,
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&app).Error; err != nil {
			return err
		}
		member := model.AppMember{
			ApplicationID: app.ID,
			UserID:        input.OwnerID,
			Role:          model.AppRoleAdmin,
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		return authz.BindRole(tx, input.OwnerID, model.AppRoleAdmin, authz.AppScope(app.ID), input.OwnerID)
	}); err != nil {
		return model.Application{}, "", err
	}

	warning := ""
	if err := k8s.CreateApplicationCR(ctx, input.Name, identifier, input.Description); err != nil {
		warning = "Application CR creation failed: " + err.Error()
	}
	return app, warning, nil
}

func GetApplication(db *gorm.DB, appID uint) (ApplicationDetail, error) {
	app, err := findApplication(db, appID)
	if err != nil {
		return ApplicationDetail{}, err
	}
	envItems, err := listApplicationEnvironmentItems(db, app.ID)
	if err != nil {
		return ApplicationDetail{}, err
	}
	var members []model.AppMember
	if err := db.Where("application_id = ?", app.ID).Preload("User").Find(&members).Error; err != nil {
		return ApplicationDetail{}, err
	}
	return ApplicationDetail{Application: app, Environments: envItems, Members: members}, nil
}

func UpdateApplication(db *gorm.DB, appID uint, input UpdateApplicationInput) error {
	app, err := findApplication(db, appID)
	if err != nil {
		return err
	}
	updates := make(map[string]interface{})
	if strings.TrimSpace(input.Name) != "" {
		updates["name"] = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.Description) != "" {
		updates["description"] = strings.TrimSpace(input.Description)
	}
	if len(updates) == 0 {
		return nil
	}
	return db.Model(&app).Updates(updates).Error
}

func DeleteApplication(ctx context.Context, db *gorm.DB, appID uint) ([]string, error) {
	app, err := findApplication(db, appID)
	if err != nil {
		return nil, err
	}
	if app.IsSystem {
		return nil, ErrSystemApplicationDelete
	}

	warnings := make([]string, 0)
	appendWarning := func(prefix string, err error) {
		if err != nil {
			warnings = append(warnings, prefix+": "+err.Error())
		}
	}

	var envs []model.Environment
	if err := db.Where("application_id = ?", app.ID).Find(&envs).Error; err != nil {
		return nil, err
	}
	envIDs := make([]uint, 0, len(envs))
	for _, env := range envs {
		envIDs = append(envIDs, env.ID)
		appendWarning("Environment cluster cleanup failed", k8s.DeleteEnvironmentScopedResources(ctx, app.Identifier, env.Identifier))
	}

	appendWarning("Application CR deletion failed", k8s.DeleteApplicationCR(ctx, app.Identifier))
	appendWarning("Application namespace cleanup failed", k8s.DeleteApplicationScopedResources(ctx, app.Identifier))

	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(envIDs) > 0 {
			if err := tx.Unscoped().Where("environment_id IN ?", envIDs).Delete(&model.ServiceInstallation{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("environment_id IN ?", envIDs).Delete(&model.InfraInstallation{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("environment_id IN ?", envIDs).Delete(&model.Component{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("environment_id IN ?", envIDs).Delete(&model.EnvironmentCanvasState{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Unscoped().Where("application_id = ?", app.ID).Delete(&model.Environment{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("application_id = ?", app.ID).Delete(&model.AppMember{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("scope_type = ? AND scope_id = ?", model.ScopeApp, app.ID).Delete(&model.RoleBinding{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&app).Error
	}); err != nil {
		return nil, err
	}
	authz.InvalidateAll()
	return warnings, nil
}

func listApplicationEnvironmentItems(db *gorm.DB, appID uint) ([]EnvironmentListItem, error) {
	var envs []model.Environment
	if err := db.Where("application_id = ?", appID).Find(&envs).Error; err != nil {
		return nil, err
	}
	return buildEnvironmentListItems(db, envs)
}

func buildEnvironmentListItems(db *gorm.DB, envs []model.Environment) ([]EnvironmentListItem, error) {
	envItems := make([]EnvironmentListItem, 0, len(envs))
	for _, env := range envs {
		var toolCount int64
		if err := db.Model(&model.ServiceInstallation{}).
			Where("environment_id = ?", env.ID).
			Count(&toolCount).Error; err != nil {
			return nil, err
		}
		var services []model.ServiceInstallation
		if err := db.Where("environment_id = ?", env.ID).
			Order("service_type").
			Find(&services).Error; err != nil {
			return nil, err
		}
		var componentCount int64
		if err := db.Model(&model.Component{}).
			Where("environment_id = ?", env.ID).
			Count(&componentCount).Error; err != nil {
			return nil, err
		}
		envItems = append(envItems, EnvironmentListItem{
			Environment:    env,
			ToolCount:      int(toolCount),
			ComponentCount: int(componentCount),
			Services:       buildServiceStatusItems(services),
		})
	}
	return envItems, nil
}

func buildServiceStatusItems(services []model.ServiceInstallation) []ServiceStatusListItem {
	items := make([]ServiceStatusListItem, 0, len(services))
	for _, svc := range services {
		items = append(items, ServiceStatusListItem{
			ServiceType:  svc.ServiceType,
			Status:       svc.Status,
			ErrorMessage: svc.ErrorMessage,
			Namespace:    svc.Namespace,
		})
	}
	return items
}

var identifierInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)

func normalizeIdentifier(input, fallbackPrefix string, maxLen int) string {
	candidate := strings.ToLower(strings.TrimSpace(input))
	candidate = strings.ReplaceAll(candidate, "_", "-")
	candidate = identifierInvalidChars.ReplaceAllString(candidate, "-")
	candidate = strings.Trim(candidate, "-")
	if candidate == "" || candidate[0] < 'a' || candidate[0] > 'z' {
		candidate = fallbackPrefix
	}
	if maxLen > 0 && len(candidate) > maxLen {
		candidate = strings.Trim(candidate[:maxLen], "-")
	}
	if candidate == "" {
		return fallbackPrefix
	}
	return candidate
}

func uniqueIdentifierWithFallback(db *gorm.DB, base, fallbackPrefix string, maxLen int, exists func(*gorm.DB, string) (bool, error)) (string, error) {
	base = normalizeIdentifier(base, fallbackPrefix, maxLen)
	candidate := base
	for i := 2; ; i++ {
		found, err := exists(db, candidate)
		if err != nil {
			return "", err
		}
		if !found {
			return candidate, nil
		}
		suffix := fmt.Sprintf("-%d", i)
		prefix := base
		if maxLen > 0 && len(prefix)+len(suffix) > maxLen {
			prefix = strings.Trim(prefix[:maxLen-len(suffix)], "-")
		}
		if prefix == "" {
			prefix = fallbackPrefix
		}
		candidate = prefix + suffix
	}
}
