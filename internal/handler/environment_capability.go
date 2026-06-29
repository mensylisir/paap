package handler

import (
	"errors"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	systemSharedApplicationIdentifier = service.SystemSharedApplicationIdentifier
	systemSharedEnvironmentIdentifier = service.SystemSharedEnvironmentIdentifier
	systemSharedApplicationName       = service.SystemSharedApplicationName
	systemSharedEnvironmentName       = service.SystemSharedEnvironmentName
)

type EnvironmentCapabilityRequest = service.EnvironmentCapabilityRequest
type SharedCapabilityResource = service.SharedCapabilityResource

func ListEnvironmentCapabilities(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}

	capabilities, err := service.ListEnvironmentCapabilities(database.DB, envID)
	if err != nil {
		writeEnvironmentCapabilityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": capabilities})
}

func GetSharedResourcePool(c *gin.Context) {
	app, env, err := service.LoadSystemSharedEnvironment(database.DB)
	if err != nil {
		writeEnvironmentCapabilityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"application": app,
		"environment": env,
	}})
}

func UpsertEnvironmentCapability(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}

	var req EnvironmentCapabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var userID uint
	if id, ok := authenticatedUserID(c); ok {
		userID = id
	}
	capability, err := service.UpsertEnvironmentCapability(c.Request.Context(), database.DB, envID, c.Param("capability"), req, userID)
	if err != nil {
		writeEnvironmentCapabilityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": capability})
}

func DeleteEnvironmentCapability(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}
	row, err := service.DeleteEnvironmentCapability(c.Request.Context(), database.DB, envID, c.Param("capability"))
	if err != nil {
		writeEnvironmentCapabilityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true, "capability": row.Capability, "capabilityKey": row.CapabilityKey}})
}

func ValidateEnvironmentCapability(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}
	row, err := service.ValidateEnvironmentCapability(c.Request.Context(), database.DB, envID, c.Param("capability"))
	if err != nil {
		writeEnvironmentCapabilityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": row})
}

func GetEnvironmentCapabilityCredentials(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}
	credentials, err := service.GetEnvironmentCapabilityCredentials(c.Request.Context(), database.DB, envID, c.Param("capability"))
	if err != nil {
		writeEnvironmentCapabilityCredentialError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"credentials": credentials}})
}

func ListSharedCapabilityResources(c *gin.Context) {
	resources, err := service.ListSharedCapabilityResources(database.DB)
	if err != nil {
		writeEnvironmentCapabilityError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resources})
}

func rejectSystemSharedEnvironmentMutation(c *gin.Context, env model.Environment, message string) bool {
	if !service.EnvironmentIsSystemSharedPool(database.DB, env) {
		return false
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": message})
	return true
}

func parseEnvironmentID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return 0, false
	}
	return uint(id), true
}

func writeEnvironmentCapabilityCredentialError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrKubernetesClientNotInitialized):
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrCredentialSecretNotFound),
		errors.Is(err, service.ErrSharedServiceNotFound),
		errors.Is(err, service.ErrEnvironmentCapabilityNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrCredentialSecretForbidden),
		errors.Is(err, service.ErrEnvironmentCapabilitySecretRefInvalid):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrCapabilityCredentialsUnsupported):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
	}
}

func writeEnvironmentCapabilityError(c *gin.Context, err error) {
	var validationErr service.ValidationError
	switch {
	case errors.Is(err, service.ErrEnvironmentNotFound),
		errors.Is(err, service.ErrSharedResourcePoolNotFound),
		errors.Is(err, service.ErrEnvironmentCapabilityNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.As(err, &validationErr),
		errors.Is(err, service.ErrSystemSharedEnvironmentMutation),
		errors.Is(err, service.ErrEnvironmentCapabilityInvalid),
		errors.Is(err, service.ErrEnvironmentCapabilitySourceInvalid),
		errors.Is(err, service.ErrCapabilityValidationUnsupported),
		errors.Is(err, service.ErrCapabilityCredentialsUnsupported):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrCredentialSecretForbidden),
		errors.Is(err, service.ErrEnvironmentCapabilitySecretRefInvalid):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrCredentialSecretNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, service.ErrKubernetesClientNotInitialized):
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	}
}
