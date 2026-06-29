package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"paap/internal/authz"
	"paap/internal/database"
	"paap/internal/middleware"
	"paap/internal/permission"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
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

type AppListItem = service.ApplicationListItem
type EnvironmentListItem = service.EnvironmentListItem
type ServiceStatusListItem = service.ServiceStatusListItem

// ListApplications returns all applications for the current user
func ListApplications(c *gin.Context) {
	syncClusterStateIfPossible()

	platformAdmin := authenticatedUserIsPlatformAdmin(c)
	userID, ok := authenticatedUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing authenticated user"})
		return
	}
	items, err := service.ListApplications(database.DB, userID, platformAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": items})
}

// CreateApplication creates a new application and its K8s CR
func CreateApplication(c *gin.Context) {
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
	app, warning, err := service.CreateApplication(c.Request.Context(), database.DB, service.CreateApplicationInput{
		Name:        req.Name,
		Identifier:  req.Identifier,
		Description: req.Description,
		OwnerID:     userID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if warning != "" {
		c.Header("X-CR-Warning", warning)
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

func authenticatedUserIsPlatformAdmin(c *gin.Context) bool {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return false
	}
	allowed, err := authz.Can(database.DB, userID, authz.SystemScope(), permission.SystemUserManage)
	return err == nil && allowed
}

// GetApplication returns application details with environments
func GetApplication(c *gin.Context) {
	syncClusterStateIfPossible()

	id, _ := strconv.Atoi(c.Param("id"))
	detail, err := service.GetApplication(database.DB, uint(id))
	if err != nil {
		respondApplicationServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"application":  detail.Application,
			"environments": detail.Environments,
			"members":      detail.Members,
		},
	})
}

// UpdateApplication updates application info
func UpdateApplication(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req UpdateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := service.UpdateApplication(database.DB, uint(id), service.UpdateApplicationInput{
		Name:        req.Name,
		Description: req.Description,
	}); err != nil {
		respondApplicationServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// DeleteApplication deletes an application and its K8s CR
func DeleteApplication(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	warnings, err := service.DeleteApplication(c.Request.Context(), database.DB, uint(id))
	if err != nil {
		respondApplicationServiceError(c, err)
		return
	}
	if len(warnings) > 0 {
		c.Header("X-CR-Warning", strings.Join(warnings, "; "))
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func respondApplicationServiceError(c *gin.Context, err error) {
	var validationErr service.ValidationError
	switch {
	case errors.As(err, &validationErr):
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error()})
	case errors.Is(err, service.ErrApplicationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
	case errors.Is(err, service.ErrSystemApplicationDelete):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
