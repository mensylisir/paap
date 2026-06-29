package middleware

import (
	"net/http"
	"strconv"

	"paap/internal/authz"
	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func RequireSystemPermission(permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requirePermission(c, authz.SystemScope(), permissionCode)
	}
}

func RequireAppPermission(permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		appID, ok := routeUint(c, "id", "invalid application id")
		if !ok {
			return
		}
		requirePermission(c, authz.AppScope(appID), permissionCode)
	}
}

func RequireEnvPermission(permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		envID, ok := routeUint(c, "id", "invalid environment id")
		if !ok {
			return
		}
		requirePermission(c, authz.EnvScope(envID), permissionCode)
	}
}

func RequireComponentPermission(permissionCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		componentID, ok := routeUint(c, "id", "invalid component id")
		if !ok {
			return
		}
		var comp model.Component
		if err := database.DB.Select("id", "environment_id").First(&comp, componentID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "component not found"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		requirePermission(c, authz.EnvScope(comp.EnvironmentID), permissionCode)
	}
}

func requirePermission(c *gin.Context, scope authz.Scope, permissionCode string) {
	userID := c.GetUint(ContextUserIDKey)
	if userID == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing authenticated user"})
		return
	}
	allowed, err := authz.Can(database.DB, userID, scope, permissionCode)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !allowed {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "permission denied"})
		return
	}
	c.Next()
}

func routeUint(c *gin.Context, name string, message string) (uint, bool) {
	value, err := strconv.Atoi(c.Param(name))
	if err != nil || value <= 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": message})
		return 0, false
	}
	return uint(value), true
}
