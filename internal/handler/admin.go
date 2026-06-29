package handler

import (
	"errors"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

// ListUsers returns all platform users.
// Only platform_admin can call this.
func ListUsers(c *gin.Context) {
	items, err := service.ListPlatformUsers(database.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
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

	createdBy, _ := authenticatedUserID(c)
	user, err := service.UpdatePlatformUserRoles(database.DB, id, req.Roles, createdBy)
	if err != nil {
		respondUserAdminServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"roles":    user.Roles,
		},
	})
}

func parseUserID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return 0, false
	}
	return uint(id), true
}

func respondUserAdminServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
