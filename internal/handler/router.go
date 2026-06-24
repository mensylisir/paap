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
	}

	protected := api.Group("", middleware.AuthRequired())
	{
		protected.GET("/auth/me", GetCurrentUser)

		// Templates
		protected.GET("/templates", ListTemplates)
		protected.GET("/templates/:id", GetTemplate)
		protected.POST("/templates", CreateTemplate)
		protected.PUT("/templates/:id", UpdateTemplate)
		protected.DELETE("/templates/:id", DeleteTemplate)
		protected.GET("/service-templates", ListServiceTemplates)
		protected.POST("/service-templates/upload", UploadTemplate)     // BYO custom template upload
		protected.POST("/service-templates/sync", SyncBuiltinTemplates) // Force re-sync built-in templates to S3 + DB
		protected.GET("/service-templates/:id", GetServiceTemplate)
		protected.POST("/service-templates", CreateServiceTemplate)
		protected.PUT("/service-templates/:id", UpdateServiceTemplate)
		protected.DELETE("/service-templates/:id", DeleteServiceTemplate)
		protected.GET("/component-config-templates", ListComponentConfigTemplates)
		protected.POST("/component-config-templates", CreateComponentConfigTemplate)
		protected.POST("/component-config-templates/sync", SyncBuiltinComponentConfigTemplates)
		protected.PUT("/component-config-templates/:id", UpdateComponentConfigTemplate)
		protected.DELETE("/component-config-templates/:id", DeleteComponentConfigTemplate)

		// Applications
		protected.GET("/applications", ListApplications)
		protected.POST("/applications", CreateApplication)
		protected.GET("/applications/:id", GetApplication)
		protected.PUT("/applications/:id", UpdateApplication)
		protected.DELETE("/applications/:id", DeleteApplication)
		protected.GET("/applications/:id/members", ListApplicationMembers)
		protected.POST("/applications/:id/members", InviteApplicationMember)
		protected.PUT("/applications/:id/members/:memberId", UpdateApplicationMemberRole)
		protected.DELETE("/applications/:id/members/:memberId", RemoveApplicationMember)

		// Application Environments
		protected.GET("/applications/:id/environments", ListApplicationEnvironments)
		protected.POST("/applications/:id/environments", CreateEnvironment)

		// Environment (standalone routes)
		protected.GET("/environments/:id", GetEnvironment)
		protected.GET("/environments/:id/canvas-state", GetEnvironmentCanvasState)
		protected.PUT("/environments/:id/canvas-state", SaveEnvironmentCanvasState)
		protected.DELETE("/environments/:id", DeleteEnvironment)
		protected.GET("/environments/:id/services", ListServiceInstances)
		protected.GET("/environments/:id/services/:serviceId", GetServiceInstance)
		protected.GET("/environments/:id/services/:serviceId/credentials", GetServiceCredentials)
		protected.GET("/environments/:id/services/:serviceId/workspace", GetServiceWorkspace)
		protected.GET("/environments/:id/services/:serviceId/runtime-metrics", GetServiceRuntimeMetrics)
		protected.GET("/environments/:id/services/:serviceId/runtime-logs", GetServiceRuntimeLogs)
		protected.GET("/environments/:id/services/:serviceId/console", HandleServiceConsole)
		protected.Any("/environments/:id/services/:serviceId/proxy/*path", ProxyServiceInstance)
		protected.GET("/environments/:id/services/:serviceId/registry-ca.crt", DownloadRegistryCACertificate)
		protected.POST("/environments/:id/services/:serviceId/workspace/actions", RunServiceWorkspaceAction)
		protected.POST("/environments/:id/services/drafts", CreateServiceDraft)
		protected.POST("/environments/:id/services", InstallService)
		protected.PUT("/environments/:id/services/:serviceId", UpdateService)
		protected.PUT("/environments/:id/services/:serviceId/external-access", SetServiceExternalAccess)
		protected.DELETE("/environments/:id/services/:serviceId", UninstallService)

		// Environment Components
		protected.GET("/environments/:id/components", ListEnvironmentComponents)
		protected.POST("/environments/:id/components", CreateComponent)
		protected.GET("/environments/:id/components/:componentId/runtime-metrics", GetComponentRuntimeMetrics)
		protected.GET("/environments/:id/components/:componentId/runtime-logs", GetComponentRuntimeLogs)
		protected.GET("/environments/:id/components/:componentId/console", HandleComponentConsole)
		protected.Any("/environments/:id/components/:componentId/proxy/*path", ProxyComponent)
		protected.GET("/environments/:id/adoptable-resources", ListAdoptableResources)
		protected.POST("/environments/:id/adoptable-resources", AdoptResource)

		// Component deploy / version bump (used by CI callback)
		protected.PUT("/components/:id", UpdateComponent)
		protected.POST("/components/:id/deploy", DeployComponent)
		// Component external access toggle
		protected.PUT("/environments/:id/components/:componentId/external-access", SetComponentExternalAccess)
		protected.PUT("/environments/:id/components/:componentId/nodeport-access", SetComponentNodePortAccess)
		// Component delete
		protected.DELETE("/components/:id", DeleteComponent)
	}
}
