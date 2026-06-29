package handler

import (
	"net/http"

	"paap/internal/database"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

func ListPlatformServiceStats(c *gin.Context) {
	stats, err := service.ListPlatformServiceStats(database.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": stats})
}

func ListCatalogServices(c *gin.Context) {
	products, err := service.ListCatalogServiceProducts(database.DB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": products})
}

func ListPlatformServiceInstances(c *gin.Context) {
	instances, err := service.ListPlatformServiceInstances(database.DB, c.Param("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": instances})
}

func ListPlatformServiceUsage(c *gin.Context) {
	usage, err := service.ListPlatformServiceUsage(database.DB, c.Param("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": usage})
}
