package handler

import (
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
)

// ListUsers returns all platform users.
// Only platform_admin can call this.
func ListUsers(c *gin.Context) {
	var users []model.User
	if err := database.DB.Order("id asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type userItem struct {
		ID       uint     `json:"id"`
		Username string   `json:"username"`
		Email    string   `json:"email"`
		Roles    []string `json:"roles"`
	}
	items := make([]userItem, 0, len(users))
	for _, u := range users {
		roles, err := model.UserRoleValues(database.DB, u.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items = append(items, userItem{
			ID:       u.ID,
			Username: u.Username,
			Email:    u.Email,
			Roles:    roles,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// UpdateUserRole changes a user's platform role.
// Only platform_admin can call this.
func UpdateUserRole(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req struct {
		Roles []string `json:"roles" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user model.User
	if err := database.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	roles, err := model.ReplaceUserRoles(database.DB, id, req.Roles)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"roles":    roles,
		},
	})
}

// RequirePlatformAdmin is a middleware that rejects non-platform-admin requests.
func RequirePlatformAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !authenticatedUserIsPlatformAdmin(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "platform admin access required"})
			return
		}
		c.Next()
	}
}

func parseUserID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return 0, false
	}
	return uint(id), true
}
