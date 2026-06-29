package handler

import (
	"errors"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/service"

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

func parseAppMemberID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("memberId"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid member id"})
		return 0, false
	}
	return uint(id), true
}

// ListApplicationMembers returns members for an application visible to the current user.
func ListApplicationMembers(c *gin.Context) {
	appID, ok := parseApplicationID(c)
	if !ok {
		return
	}
	members, err := service.ListApplicationMembers(database.DB, appID)
	if err != nil {
		respondAppMemberServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": members})
}

// InviteApplicationMember adds an existing platform user to an application.
func InviteApplicationMember(c *gin.Context) {
	appID, ok := parseApplicationID(c)
	if !ok {
		return
	}
	var req InviteAppMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	createdBy, _ := authenticatedUserID(c)
	member, err := service.InviteApplicationMember(database.DB, appID, req.Username, req.Role, createdBy)
	if err != nil {
		respondAppMemberServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": member})
}

// UpdateApplicationMemberRole changes a member's application role.
func UpdateApplicationMemberRole(c *gin.Context) {
	appID, ok := parseApplicationID(c)
	if !ok {
		return
	}
	memberID, ok := parseAppMemberID(c)
	if !ok {
		return
	}
	var req UpdateAppMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	createdBy, _ := authenticatedUserID(c)
	member, err := service.UpdateApplicationMemberRole(database.DB, appID, memberID, req.Role, createdBy)
	if err != nil {
		respondAppMemberServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": member})
}

// RemoveApplicationMember removes a member from an application.
func RemoveApplicationMember(c *gin.Context) {
	appID, ok := parseApplicationID(c)
	if !ok {
		return
	}
	memberID, ok := parseAppMemberID(c)
	if !ok {
		return
	}
	if err := service.RemoveApplicationMember(database.DB, appID, memberID); err != nil {
		respondAppMemberServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

func respondAppMemberServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrApplicationNotFound),
		errors.Is(err, service.ErrAppMemberNotFound),
		errors.Is(err, service.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrAppMemberExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrInvalidAppRole),
		errors.Is(err, service.ErrLastAppAdmin):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
