package handler

import (
	"errors"
	"net/http"

	"paap/internal/database"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

func GetCatalogServiceDetail(c *gin.Context) {
	detail, err := service.GetCatalogServiceDetail(database.DB, c.Param("type"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": detail})
}

func GetCatalogServiceResources(c *gin.Context) {
	resources, err := service.GetCatalogServiceResources(database.DB, c.Param("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resources})
}

func GetCatalogServiceTopology(c *gin.Context) {
	topology, err := service.GetCatalogServiceTopology(database.DB, c.Param("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": topology})
}

func GetCatalogServiceObservability(c *gin.Context) {
	observability, err := service.GetCatalogServiceObservability(database.DB, c.Param("type"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": observability})
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
