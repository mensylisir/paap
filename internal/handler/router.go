package handler

import (
	"net/http"
	"paap/internal/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {
	r.Use(middleware.CORS())

	r.GET("/health", HealthCheck)
	r.GET("/healthz", HealthCheck)
	r.GET("/ws", HandleWebSocket)

	// 前端静态文件。开发调试时经常替换镜像内前端 bundle，禁用缓存避免浏览器继续使用旧 chunk。
	noCacheStatic := func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
	}
	assets := r.Group("/assets", noCacheStatic)
	assets.StaticFS("", http.Dir("./frontend/dist/assets"))
	r.GET("/favicon.svg", noCacheStatic, func(c *gin.Context) { c.File("./frontend/dist/favicon.svg") })
	r.GET("/icons.svg", noCacheStatic, func(c *gin.Context) { c.File("./frontend/dist/icons.svg") })
	r.NoRoute(func(c *gin.Context) {
		if len(c.Request.URL.Path) >= 8 && c.Request.URL.Path[:8] == "/assets/" {
			c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
			return
		}
		// SPA fallback：非 API 路径都返回 index.html
		if len(c.Request.URL.Path) < 4 || c.Request.URL.Path[:4] != "/api" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
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
		api.POST("/service-templates/upload", UploadTemplate)     // BYO custom template upload
		api.POST("/service-templates/sync", SyncBuiltinTemplates) // Force re-sync built-in templates to S3 + DB
		api.GET("/service-templates/:id", GetServiceTemplate)
		api.POST("/service-templates", CreateServiceTemplate)
		api.PUT("/service-templates/:id", UpdateServiceTemplate)
		api.DELETE("/service-templates/:id", DeleteServiceTemplate)
		api.GET("/component-config-templates", ListComponentConfigTemplates)
		api.POST("/component-config-templates", CreateComponentConfigTemplate)
		api.POST("/component-config-templates/sync", SyncBuiltinComponentConfigTemplates)
		api.PUT("/component-config-templates/:id", UpdateComponentConfigTemplate)
		api.DELETE("/component-config-templates/:id", DeleteComponentConfigTemplate)

		// Applications
		api.GET("/applications", ListApplications)
		api.POST("/applications", CreateApplication)
		api.GET("/applications/:id", GetApplication)
		api.PUT("/applications/:id", UpdateApplication)
		api.DELETE("/applications/:id", DeleteApplication)

		// Application Environments
		api.GET("/applications/:id/environments", ListApplicationEnvironments)
		api.POST("/applications/:id/environments", CreateEnvironment)

		// Environment (standalone routes)
		api.GET("/environments/:id", GetEnvironment)
		api.GET("/environments/:id/canvas-state", GetEnvironmentCanvasState)
		api.PUT("/environments/:id/canvas-state", SaveEnvironmentCanvasState)
		api.DELETE("/environments/:id", DeleteEnvironment)
		api.GET("/environments/:id/services", ListServiceInstances)
		api.GET("/environments/:id/services/:serviceId", GetServiceInstance)
		api.GET("/environments/:id/services/:serviceId/credentials", GetServiceCredentials)
		api.GET("/environments/:id/services/:serviceId/workspace", GetServiceWorkspace)
		api.GET("/environments/:id/services/:serviceId/runtime-metrics", GetServiceRuntimeMetrics)
		api.GET("/environments/:id/services/:serviceId/runtime-logs", GetServiceRuntimeLogs)
		api.GET("/environments/:id/services/:serviceId/console", HandleServiceConsole)
		api.Any("/environments/:id/services/:serviceId/proxy/*path", ProxyServiceInstance)
		api.GET("/environments/:id/services/:serviceId/registry-ca.crt", DownloadRegistryCACertificate)
		api.POST("/environments/:id/services/:serviceId/workspace/actions", RunServiceWorkspaceAction)
		api.POST("/environments/:id/services/drafts", CreateServiceDraft)
		api.POST("/environments/:id/services", InstallService)
		api.PUT("/environments/:id/services/:serviceId", UpdateService)
		api.PUT("/environments/:id/services/:serviceId/external-access", SetServiceExternalAccess)
		api.DELETE("/environments/:id/services/:serviceId", UninstallService)

		// Environment Components
		api.GET("/environments/:id/components", ListEnvironmentComponents)
		api.POST("/environments/:id/components", CreateComponent)
		api.GET("/environments/:id/components/:componentId/runtime-metrics", GetComponentRuntimeMetrics)
		api.GET("/environments/:id/components/:componentId/runtime-logs", GetComponentRuntimeLogs)
		api.GET("/environments/:id/components/:componentId/console", HandleComponentConsole)
		api.Any("/environments/:id/components/:componentId/proxy/*path", ProxyComponent)
		api.GET("/environments/:id/adoptable-resources", ListAdoptableResources)
		api.POST("/environments/:id/adoptable-resources", AdoptResource)

		// Component deploy / version bump (used by CI callback)
		api.PUT("/components/:id", UpdateComponent)
		api.POST("/components/:id/deploy", DeployComponent)
		// Component delete
		api.DELETE("/components/:id", DeleteComponent)
	}
}
