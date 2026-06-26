package handler

import (
	"context"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/middleware"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateAppRequest struct {
	Name        string `json:"name" binding:"required"`
	Identifier  string `json:"identifier"`
	Description string `json:"description"`
}

type UpdateAppRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type InviteAppMemberRequest struct {
	Username string `json:"username" binding:"required"`
	Role     string `json:"role"`
}

type UpdateAppMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

type AppListItem struct {
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

// ListApplications returns all applications for the current user
func ListApplications(c *gin.Context) {
	syncClusterStateIfPossible()

	platformAdmin := authenticatedUserIsPlatformAdmin(c)
	query := database.DB.Model(&model.Application{})
	if !platformAdmin {
		userID, ok := authenticatedUserID(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing authenticated user"})
			return
		}
		query = query.Where("applications.is_system = ?", false)
		query = query.Joins(
			"JOIN app_members ON app_members.application_id = applications.id AND app_members.user_id = ? AND app_members.deleted_at IS NULL",
			userID,
		)
	}

	var apps []model.Application
	if err := query.Order("applications.is_system DESC").Order("applications.id").Find(&apps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]AppListItem, 0, len(apps))
	for _, app := range apps {
		var envs []model.Environment
		if err := database.DB.Where("application_id = ?", app.ID).Find(&envs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		envItems, err := buildEnvironmentListItems(envs)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		items = append(items, AppListItem{
			Application:      app,
			Environments:     envItems,
			EnvironmentCount: len(envItems),
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": items})
}

// CreateApplication creates a new application and its K8s CR
func CreateApplication(c *gin.Context) {
	if !authenticatedUserCanCreateApp(c) {
		return
	}

	var req CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing authenticated user"})
		return
	}
	identifier, err := uniqueIdentifierWithFallback(database.DB, firstNonEmpty(req.Identifier, req.Name), "app", 50, func(db *gorm.DB, candidate string) (bool, error) {
		var count int64
		if err := db.Model(&model.Application{}).Where("identifier = ?", candidate).Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	app := model.Application{
		Name:        req.Name,
		Identifier:  identifier,
		Description: req.Description,
		OwnerID:     userID,
	}

	if err := database.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建 K8s Application CR
	ctx := context.Background()
	if err := k8s.CreateApplicationCR(ctx, req.Name, identifier, req.Description); err != nil {
		// CR 创建失败不阻塞，记录日志
		c.Header("X-CR-Warning", "Application CR creation failed: "+err.Error())
	}

	// Create app member record for owner
	member := model.AppMember{
		ApplicationID: app.ID,
		UserID:        userID,
		Role:          "admin",
	}
	if err := database.DB.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": app})
}

func authenticatedUserID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(middleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	switch v := value.(type) {
	case uint:
		return v, v > 0
	case int:
		if v > 0 {
			return uint(v), true
		}
	}
	return 0, false
}

func authenticatedUserCanCreateApp(c *gin.Context) bool {
	if !authenticatedUserHasRole(c, model.RoleAppAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "only application administrators can create applications"})
		return false
	}
	return true
}

func authenticatedUserIsPlatformAdmin(c *gin.Context) bool {
	return authenticatedUserHasRole(c, model.RolePlatformAdmin)
}

func authenticatedUserHasRole(c *gin.Context, role string) bool {
	value, exists := c.Get(middleware.ContextUserRolesKey)
	if !exists {
		return false
	}
	switch roles := value.(type) {
	case []string:
		return model.HasUserRole(roles, role)
	case []interface{}:
		for _, candidate := range roles {
			if text, ok := candidate.(string); ok && text == role {
				return true
			}
		}
	}
	return false
}

func requireApplicationAccess(c *gin.Context, appID uint) bool {
	if authenticatedUserIsPlatformAdmin(c) {
		return true
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing authenticated user"})
		return false
	}
	var count int64
	if err := database.DB.Model(&model.AppMember{}).
		Where("application_id = ? AND user_id = ?", appID, userID).
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return false
	}
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "application access denied"})
		return false
	}
	return true
}

func requireApplicationAdminAccess(c *gin.Context, appID uint) bool {
	if authenticatedUserIsPlatformAdmin(c) {
		return true
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing authenticated user"})
		return false
	}
	var count int64
	if err := database.DB.Model(&model.AppMember{}).
		Where("application_id = ? AND user_id = ? AND role = ?", appID, userID, "admin").
		Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return false
	}
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"error": "application admin access required"})
		return false
	}
	return true
}

// GetApplication returns application details with environments
func GetApplication(c *gin.Context) {
	syncClusterStateIfPossible()

	id, _ := strconv.Atoi(c.Param("id"))
	var app model.Application
	if err := database.DB.First(&app, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !requireApplicationAccess(c, app.ID) {
		return
	}

	var envs []model.Environment
	if err := database.DB.Where("application_id = ?", app.ID).Find(&envs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	envItems, err := buildEnvironmentListItems(envs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	var members []model.AppMember
	database.DB.Where("application_id = ?", app.ID).Preload("User").Find(&members)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"application":  app,
			"environments": envItems,
			"members":      members,
		},
	})
}

func buildEnvironmentListItems(envs []model.Environment) ([]EnvironmentListItem, error) {
	envItems := make([]EnvironmentListItem, 0, len(envs))
	for _, env := range envs {
		var toolCount int64
		if err := database.DB.Model(&model.ServiceInstallation{}).
			Where("environment_id = ?", env.ID).
			Count(&toolCount).Error; err != nil {
			return nil, err
		}
		var services []model.ServiceInstallation
		if err := database.DB.Where("environment_id = ?", env.ID).
			Order("service_type").
			Find(&services).Error; err != nil {
			return nil, err
		}
		var componentCount int64
		if err := database.DB.Model(&model.Component{}).
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

// UpdateApplication updates application info
func UpdateApplication(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req UpdateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var app model.Application
	if err := database.DB.First(&app, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !requireApplicationAccess(c, app.ID) {
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	if err := database.DB.Model(&app).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// DeleteApplication deletes an application and its K8s CR
func DeleteApplication(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	// 先查出应用信息（用于删除 CR）
	var app model.Application
	if err := database.DB.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	if !requireApplicationAccess(c, app.ID) {
		return
	}
	if app.IsSystem {
		c.JSON(http.StatusBadRequest, gin.H{"error": "system applications cannot be deleted"})
		return
	}

	ctx := context.Background()
	warn := func(prefix string, err error) {
		if err == nil {
			return
		}
		current := c.Writer.Header().Get("X-CR-Warning")
		if current != "" {
			current += "; "
		}
		c.Header("X-CR-Warning", current+prefix+": "+err.Error())
	}

	var envs []model.Environment
	if err := database.DB.Where("application_id = ?", app.ID).Find(&envs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	envIDs := make([]uint, 0, len(envs))
	for _, env := range envs {
		envIDs = append(envIDs, env.ID)
		warn("Environment cluster cleanup failed", k8s.DeleteEnvironmentScopedResources(ctx, app.Identifier, env.Identifier))
	}

	if err := k8s.DeleteApplicationCR(ctx, app.Identifier); err != nil {
		warn("Application CR deletion failed", err)
	}
	warn("Application namespace cleanup failed", k8s.DeleteApplicationScopedResources(ctx, app.Identifier))

	// 删除数据库记录（硬删除，避免唯一约束冲突）
	if len(envIDs) > 0 {
		database.DB.Unscoped().Where("environment_id IN ?", envIDs).Delete(&model.ServiceInstallation{})
		database.DB.Unscoped().Where("environment_id IN ?", envIDs).Delete(&model.InfraInstallation{})
		database.DB.Unscoped().Where("environment_id IN ?", envIDs).Delete(&model.Component{})
		database.DB.Unscoped().Where("environment_id IN ?", envIDs).Delete(&model.EnvironmentCanvasState{})
	}
	database.DB.Unscoped().Where("application_id = ?", app.ID).Delete(&model.Environment{})
	database.DB.Unscoped().Where("application_id = ?", app.ID).Delete(&model.AppMember{})
	database.DB.Unscoped().Delete(&app)

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
