package handler

import (
	"context"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CreateAppRequest struct {
	Name        string `json:"name" binding:"required"`
	Identifier  string `json:"identifier" binding:"required"`
	Description string `json:"description"`
}

type UpdateAppRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ListApplications returns all applications for the current user
func ListApplications(c *gin.Context) {
	var apps []model.Application
	if err := database.DB.Find(&apps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": apps})
}

// CreateApplication creates a new application and its K8s CR
func CreateApplication(c *gin.Context) {
	var req CreateAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	app := model.Application{
		Name:        req.Name,
		Identifier:  req.Identifier,
		Description: req.Description,
		OwnerID:     1, // demo user
	}

	if err := database.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建 K8s Application CR
	ctx := context.Background()
	if err := k8s.CreateApplicationCR(ctx, req.Name, req.Identifier, req.Description); err != nil {
		// CR 创建失败不阻塞，记录日志
		c.Header("X-CR-Warning", "Application CR creation failed: "+err.Error())
	}

	// Create app member record for owner
	member := model.AppMember{
		ApplicationID: app.ID,
		UserID:        1,
		Role:          "admin",
	}
	database.DB.Create(&member)

	c.JSON(http.StatusCreated, gin.H{"data": app})
}

// GetApplication returns application details with environments
func GetApplication(c *gin.Context) {
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

	var envs []model.Environment
	database.DB.Where("application_id = ?", app.ID).Find(&envs)
	var members []model.AppMember
	database.DB.Where("application_id = ?", app.ID).Preload("User").Find(&members)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"application":  app,
			"environments": envs,
			"members":      members,
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

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	if err := database.DB.Model(&model.Application{}).Where("id = ?", id).Updates(updates).Error; err != nil {
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

	// 删除 K8s Application CR（会触发 Operator 级联删除）
	ctx := context.Background()
	if err := k8s.DeleteApplicationCR(ctx, app.Identifier); err != nil {
		c.Header("X-CR-Warning", "Application CR deletion failed: "+err.Error())
	}

	// 删除数据库记录（硬删除，避免唯一约束冲突）
	database.DB.Unscoped().Where("application_id = ?", app.ID).Delete(&model.Environment{})
	database.DB.Unscoped().Where("application_id = ?", app.ID).Delete(&model.AppMember{})
	database.DB.Unscoped().Delete(&app)

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
