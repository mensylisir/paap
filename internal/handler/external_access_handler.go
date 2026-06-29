package handler

import (
	"errors"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

func SetComponentExternalAccess(c *gin.Context) {
	envID, componentID, ok := parseComponentAccessRoute(c)
	if !ok {
		return
	}
	var req ServiceExternalAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := service.SetComponentExternalAccess(c.Request.Context(), database.DB, envID, componentID, req.Enabled)
	if err != nil {
		respondComponentAccessServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result.View, "externalAccess": result.ExternalAccess})
}

func SetComponentNodePortAccess(c *gin.Context) {
	envID, componentID, ok := parseComponentAccessRoute(c)
	if !ok {
		return
	}
	var req ServiceExternalAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := service.SetComponentNodePortAccess(c.Request.Context(), database.DB, envID, componentID, req.Enabled)
	if err != nil {
		respondComponentAccessServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result.View, "externalAccess": result.ExternalAccess})
}

func parseComponentAccessRoute(c *gin.Context) (uint, uint, bool) {
	envID, err := strconv.Atoi(c.Param("id"))
	if err != nil || envID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return 0, 0, false
	}
	componentID, err := strconv.Atoi(c.Param("componentId"))
	if err != nil || componentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid component id"})
		return 0, 0, false
	}
	return uint(envID), uint(componentID), true
}

func respondComponentAccessServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrEnvironmentNotFound),
		errors.Is(err, service.ErrComponentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrEnvironmentNotReady):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrComponentAccessPatch),
		errors.Is(err, service.ErrComponentNodePortPatch):
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
