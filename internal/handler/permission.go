package handler

import (
	"errors"
	"net/http"
	"strconv"

	"paap/internal/authz"
	"paap/internal/database"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

func GetCurrentPermissions(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing authenticated user"})
		return
	}
	scope, ok := permissionScopeFromRequest(c)
	if !ok {
		return
	}
	result, err := service.CurrentPermissionsForUser(database.DB, userID, scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result})
}

func ListPermissionTree(c *gin.Context) {
	groups, err := service.ListPermissionTree(database.DB, c.Query("scopeType"))
	if err != nil {
		var validation service.ValidationError
		if errors.As(err, &validation) {
			c.JSON(http.StatusBadRequest, gin.H{"error": validation.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": groups})
}

func permissionScopeFromRequest(c *gin.Context) (authz.Scope, bool) {
	scope := authz.SystemScope()
	scopeType := c.Query("scopeType")
	if scopeType == "" {
		return scope, true
	}
	normalized, err := model.NormalizeScopeType(scopeType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return authz.Scope{}, false
	}
	scope.Type = normalized
	if normalized == model.ScopeSystem {
		return scope, true
	}
	scopeID, err := strconv.Atoi(c.Query("scopeId"))
	if err != nil || scopeID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scopeId is required"})
		return authz.Scope{}, false
	}
	scope.ID = uint(scopeID)
	return scope, true
}
