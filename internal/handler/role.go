package handler

import (
	"errors"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

type saveRoleRequest struct {
	Code          string `json:"code"`
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	ScopeType     string `json:"scopeType"`
	Enabled       *bool  `json:"enabled"`
	PermissionIDs []uint `json:"permissionIds"`
}

func ListAssignableRoles(c *gin.Context) {
	items, err := service.ListAssignableRoles(database.DB, c.Query("scopeType"))
	if err != nil {
		writeRoleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func ListRoles(c *gin.Context) {
	items, err := service.ListRoles(database.DB, c.Query("scopeType"))
	if err != nil {
		writeRoleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func CreateRole(c *gin.Context) {
	var req saveRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := service.CreateRole(database.DB, saveRoleInput(req))
	if err != nil {
		writeRoleServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": item})
}

func UpdateRole(c *gin.Context) {
	roleID, ok := parseRoleID(c)
	if !ok {
		return
	}
	var req saveRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	item, err := service.UpdateRole(database.DB, roleID, saveRoleInput(req))
	if err != nil {
		writeRoleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": item})
}

func DeleteRole(c *gin.Context) {
	roleID, ok := parseRoleID(c)
	if !ok {
		return
	}
	if err := service.DeleteRole(database.DB, roleID); err != nil {
		writeRoleServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func saveRoleInput(req saveRoleRequest) service.SaveRoleInput {
	return service.SaveRoleInput{
		Code:          req.Code,
		Name:          req.Name,
		Description:   req.Description,
		ScopeType:     req.ScopeType,
		Enabled:       req.Enabled,
		PermissionIDs: req.PermissionIDs,
	}
}

func parseRoleID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role id"})
		return 0, false
	}
	return uint(id), true
}

func writeRoleServiceError(c *gin.Context, err error) {
	var validation service.ValidationError
	switch {
	case errors.As(err, &validation):
		c.JSON(http.StatusBadRequest, gin.H{"error": validation.Error()})
	case errors.Is(err, service.ErrRoleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrRoleNotEditable), errors.Is(err, service.ErrRoleNotDeletable):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrRoleAssigned):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
