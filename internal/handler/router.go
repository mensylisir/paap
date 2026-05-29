package handler

import (
	"paap/internal/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	r.Use(middleware.CORS())

	r.GET("/health", HealthCheck)
	r.GET("/healthz", HealthCheck)
	r.GET("/ws", HandleWebSocket)

	// 前端静态文件
	r.Static("/assets", "./frontend/dist/assets")
	r.StaticFile("/favicon.svg", "./frontend/dist/favicon.svg")
	r.StaticFile("/icons.svg", "./frontend/dist/icons.svg")
	r.NoRoute(func(c *gin.Context) {
		// SPA fallback：非 API 路径都返回 index.html
		if len(c.Request.URL.Path) < 4 || c.Request.URL.Path[:4] != "/api" {
			c.File("./frontend/dist/index.html")
			return
		}
		c.JSON(404, gin.H{"error": "not found"})
	})

	api := r.Group("/api/v1")
	{
		// Auth
		api.POST("/auth/register", Register)
		api.POST("/auth/login", Login)
		api.GET("/auth/me", GetCurrentUser)

		// Templates
		api.GET("/templates", ListTemplates)
		api.GET("/service-templates", ListServiceTemplates)
		api.GET("/service-templates/:id", GetServiceTemplate)
		api.POST("/service-templates", CreateServiceTemplate)
		api.PUT("/service-templates/:id", UpdateServiceTemplate)
		api.DELETE("/service-templates/:id", DeleteServiceTemplate)

		// Applications
		api.GET("/applications", ListApplications)
		api.POST("/applications", CreateApplication)
		api.GET("/applications/:id", GetApplication)
		api.PUT("/applications/:id", UpdateApplication)
		api.DELETE("/applications/:id", DeleteApplication)

		// Application Environments
		api.GET("/applications/:id/environments", ListApplicationEnvironments)
		api.POST("/applications/:id/environments", CreateEnvironment)

		// Application Services
		api.GET("/applications/:id/services", ListServiceInstances)
		api.POST("/applications/:id/services", InstallService)

		// Environment (standalone routes)
		api.GET("/environments/:id", GetEnvironment)
		api.DELETE("/environments/:id", DeleteEnvironment)

		// Environment Components
		api.GET("/environments/:id/components", ListEnvironmentComponents)
		api.POST("/environments/:id/components", CreateComponent)

		// Component delete
		api.DELETE("/components/:id", DeleteComponent)
	}
}
