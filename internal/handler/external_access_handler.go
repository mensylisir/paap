package handler

import (
	"context"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
)

func SetComponentExternalAccess(c *gin.Context) {
	envID, err := strconv.Atoi(c.Param("id"))
	if err != nil || envID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return
	}
	componentID, err := strconv.Atoi(c.Param("componentId"))
	if err != nil || componentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid component id"})
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	env, app, ok := loadEnvironmentAndApp(c, uint(envID))
	if !ok {
		return
	}
	if !requireApplicationAdminAccess(c, app.ID) {
		return
	}
	var comp model.Component
	if err := database.DB.Where("id = ? AND environment_id = ?", componentID, envID).First(&comp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
		return
	}
	if err := k8s.SetComponentExternalAccess(context.Background(), env.Namespace, comp.Name, req.Enabled); err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"enabled": req.Enabled}})
}

func SetComponentNodePortAccess(c *gin.Context) {
	envID, err := strconv.Atoi(c.Param("id"))
	if err != nil || envID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return
	}
	componentID, err := strconv.Atoi(c.Param("componentId"))
	if err != nil || componentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid component id"})
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	env, app, ok := loadEnvironmentAndApp(c, uint(envID))
	if !ok {
		return
	}
	if !requireApplicationAdminAccess(c, app.ID) {
		return
	}
	var comp model.Component
	if err := database.DB.Where("id = ? AND environment_id = ?", componentID, envID).First(&comp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
		return
	}
	if err := k8s.SetComponentNodePortAccess(context.Background(), env.Namespace, comp.Name, req.Enabled); err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"enabled": req.Enabled}})
}
