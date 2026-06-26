package handler

import (
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
)

func parseApplicationID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return 0, false
	}
	return uint(id), true
}

func parseMemberID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("memberId"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member id"})
		return 0, false
	}
	return uint(id), true
}

// ListApplicationMembers returns all members of an application.
func ListApplicationMembers(c *gin.Context) {
	appID, ok := parseApplicationID(c)
	if !ok {
		return
	}
	var members []model.AppMember
	if err := database.DB.Where("application_id = ?", appID).
		Preload("User").
		Order("id asc").
		Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type memberItem struct {
		ID     uint   `json:"id"`
		UserID uint   `json:"userId"`
		Role   string `json:"role"`
		User   *struct {
			ID       uint   `json:"id"`
			Username string `json:"username"`
			Email    string `json:"email"`
		} `json:"user,omitempty"`
	}
	items := make([]memberItem, 0, len(members))
	for _, m := range members {
		item := memberItem{
			ID:     m.ID,
			UserID: m.UserID,
			Role:   m.Role,
		}
		if m.User.ID != 0 {
			item.User = &struct {
				ID       uint   `json:"id"`
				Username string `json:"username"`
				Email    string `json:"email"`
			}{
				ID:       m.User.ID,
				Username: m.User.Username,
				Email:    m.User.Email,
			}
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// InviteApplicationMember adds a user to an application.
func InviteApplicationMember(c *gin.Context) {
	appID, ok := parseApplicationID(c)
	if !ok {
		return
	}
	var req struct {
		Username string `json:"username" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var user model.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	var existing int64
	database.DB.Model(&model.AppMember{}).Where("application_id = ? AND user_id = ?", appID, user.ID).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "user is already a member of this application"})
		return
	}
	member := model.AppMember{
		ApplicationID: appID,
		UserID:        user.ID,
		Role:          req.Role,
	}
	if err := database.DB.Create(&member).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": member})
}

// UpdateApplicationMemberRole changes a member's role.
func UpdateApplicationMemberRole(c *gin.Context) {
	appID, ok := parseApplicationID(c)
	if !ok {
		return
	}
	memberID, ok := parseMemberID(c)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := database.DB.Model(&model.AppMember{}).
		Where("id = ? AND application_id = ?", memberID, appID).
		Update("role", req.Role)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"ok": true}})
}

// RemoveApplicationMember removes a member from an application.
func RemoveApplicationMember(c *gin.Context) {
	appID, ok := parseApplicationID(c)
	if !ok {
		return
	}
	memberID, ok := parseMemberID(c)
	if !ok {
		return
	}
	result := database.DB.Where("id = ? AND application_id = ?", memberID, appID).Delete(&model.AppMember{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"ok": true}})
}
