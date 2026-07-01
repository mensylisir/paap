package handler

import (
	"net/http"
	"paap/internal/middleware"
	"paap/internal/permission"

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
		api.GET("/auth/keycloak/login", KeycloakLogin)
		api.GET("/auth/keycloak/callback", KeycloakCallback)
	}

	protected := api.Group("", middleware.AuthRequired())
	{
		protected.GET("/auth/me", GetCurrentUser)
		protected.GET("/auth/permissions", GetCurrentPermissions)
		protected.GET("/roles", ListAssignableRoles)
		protected.GET("/permissions/tree", middleware.RequireSystemPermission(permission.SystemRoleManage), ListPermissionTree)

		// Templates
		protected.GET("/templates", ListTemplates)
		protected.GET("/templates/:id", GetTemplate)
		protected.POST("/templates", middleware.RequireSystemPermission(permission.SystemTemplateManage), CreateTemplate)
		protected.PUT("/templates/:id", middleware.RequireSystemPermission(permission.SystemTemplateManage), UpdateTemplate)
		protected.DELETE("/templates/:id", middleware.RequireSystemPermission(permission.SystemTemplateManage), DeleteTemplate)
		protected.GET("/service-templates", ListServiceTemplates)
		protected.GET("/catalog/services", ListCatalogServices)
		protected.GET("/catalog/services/:type/detail", GetCatalogServiceDetail)
		protected.GET("/catalog/services/:type/resources", GetCatalogServiceResources)
		protected.GET("/catalog/services/:type/topology", GetCatalogServiceTopology)
		protected.GET("/catalog/services/:type/observability", GetCatalogServiceObservability)
		protected.POST("/service-templates/upload", middleware.RequireSystemPermission(permission.SystemTemplateManage), UploadTemplate)     // BYO custom template upload
		protected.POST("/service-templates/sync", middleware.RequireSystemPermission(permission.SystemTemplateManage), SyncBuiltinTemplates) // Force re-sync built-in templates to S3 + DB
		protected.GET("/service-templates/:id", GetServiceTemplate)
		protected.POST("/service-templates", middleware.RequireSystemPermission(permission.SystemTemplateManage), CreateServiceTemplate)
		protected.PUT("/service-templates/:id", middleware.RequireSystemPermission(permission.SystemTemplateManage), UpdateServiceTemplate)
		protected.DELETE("/service-templates/:id", middleware.RequireSystemPermission(permission.SystemTemplateManage), DeleteServiceTemplate)
		protected.GET("/component-config-templates", ListComponentConfigTemplates)
		protected.POST("/component-config-templates/upload", middleware.RequireSystemPermission(permission.SystemTemplateManage), UploadComponentConfigTemplate)
		protected.POST("/component-config-templates", middleware.RequireSystemPermission(permission.SystemTemplateManage), CreateComponentConfigTemplate)
		protected.PUT("/component-config-templates/:id", middleware.RequireSystemPermission(permission.SystemTemplateManage), UpdateComponentConfigTemplate)
		protected.DELETE("/component-config-templates/:id", middleware.RequireSystemPermission(permission.SystemTemplateManage), DeleteComponentConfigTemplate)

		// Applications
		protected.GET("/applications", ListApplications)
		protected.POST("/applications", middleware.RequireSystemPermission(permission.AppCreate), CreateApplication)
		protected.GET("/applications/:id", middleware.RequireAppPermission(permission.AppRead), GetApplication)
		protected.PUT("/applications/:id", middleware.RequireAppPermission(permission.AppUpdate), UpdateApplication)
		protected.DELETE("/applications/:id", middleware.RequireAppPermission(permission.AppDelete), DeleteApplication)
		protected.GET("/applications/:id/members", middleware.RequireAppPermission(permission.AppMemberRead), ListApplicationMembers)
		protected.POST("/applications/:id/members", middleware.RequireAppPermission(permission.AppMemberManage), InviteApplicationMember)
		protected.PUT("/applications/:id/members/:memberId", middleware.RequireAppPermission(permission.AppMemberManage), UpdateApplicationMemberRole)
		protected.DELETE("/applications/:id/members/:memberId", middleware.RequireAppPermission(permission.AppMemberManage), RemoveApplicationMember)

		// Application Environments
		protected.GET("/applications/:id/environments", middleware.RequireAppPermission(permission.AppRead), ListApplicationEnvironments)
		protected.POST("/applications/:id/environments", middleware.RequireAppPermission(permission.EnvCreate), CreateEnvironment)

		// Environment (standalone routes)
		protected.GET("/environments/:id", middleware.RequireEnvPermission(permission.EnvRead), GetEnvironment)
		protected.GET("/environments/:id/capabilities", middleware.RequireEnvPermission(permission.EnvRead), ListEnvironmentCapabilities)
		protected.PUT("/environments/:id/capabilities/:capability", middleware.RequireEnvPermission(permission.EnvManage), UpsertEnvironmentCapability)
		protected.POST("/environments/:id/capabilities/:capability/validate", middleware.RequireEnvPermission(permission.EnvManage), ValidateEnvironmentCapability)
		protected.GET("/environments/:id/capabilities/:capability/credentials", middleware.RequireEnvPermission(permission.EnvRead), GetEnvironmentCapabilityCredentials)
		protected.GET("/environments/:id/canvas-state", middleware.RequireEnvPermission(permission.EnvRead), GetEnvironmentCanvasState)
		protected.PUT("/environments/:id/canvas-state", middleware.RequireEnvPermission(permission.EnvManage), SaveEnvironmentCanvasState)
		protected.DELETE("/environments/:id", middleware.RequireEnvPermission(permission.EnvDelete), DeleteEnvironment)
		protected.GET("/environments/:id/services", middleware.RequireEnvPermission(permission.ServiceRead), ListServiceInstances)
		protected.GET("/environments/:id/services/:serviceId", middleware.RequireEnvPermission(permission.ServiceRead), GetServiceInstance)
		protected.GET("/environments/:id/services/:serviceId/credentials", middleware.RequireEnvPermission(permission.ServiceRead), GetServiceCredentials)
		protected.GET("/environments/:id/services/:serviceId/workspace", middleware.RequireEnvPermission(permission.ServiceRead), GetServiceWorkspace)
		protected.GET("/environments/:id/services/:serviceId/runtime-metrics", middleware.RequireEnvPermission(permission.ServiceRead), GetServiceRuntimeMetrics)
		protected.GET("/environments/:id/services/:serviceId/runtime-logs", middleware.RequireEnvPermission(permission.ServiceRead), GetServiceRuntimeLogs)
		protected.GET("/environments/:id/services/:serviceId/console", middleware.RequireEnvPermission(permission.ServiceRead), HandleServiceConsole)
		protected.Any("/environments/:id/services/:serviceId/proxy/*path", middleware.RequireEnvPermission(permission.ServiceRead), ProxyServiceInstance)
		protected.GET("/environments/:id/services/:serviceId/registry-ca.crt", middleware.RequireEnvPermission(permission.ServiceRead), DownloadRegistryCACertificate)
		protected.POST("/environments/:id/services/:serviceId/workspace/actions", middleware.RequireEnvPermission(permission.ServiceManage), RunServiceWorkspaceAction)
		protected.POST("/environments/:id/services/drafts", middleware.RequireEnvPermission(permission.ServiceInstall), CreateServiceDraft)
		protected.POST("/environments/:id/services", middleware.RequireEnvPermission(permission.ServiceInstall), InstallService)
		protected.PUT("/environments/:id/services/:serviceId", middleware.RequireEnvPermission(permission.ServiceManage), UpdateService)
		protected.PUT("/environments/:id/services/:serviceId/external-access", middleware.RequireEnvPermission(permission.ServiceManage), SetServiceExternalAccess)
		protected.DELETE("/environments/:id/services/:serviceId", middleware.RequireEnvPermission(permission.ServiceManage), UninstallService)

		// Environment Components
		protected.GET("/environments/:id/components", middleware.RequireEnvPermission(permission.ComponentRead), ListEnvironmentComponents)
		protected.POST("/environments/:id/components", middleware.RequireEnvPermission(permission.ComponentCreate), CreateComponent)
		protected.GET("/environments/:id/components/:componentId/runtime-metrics", middleware.RequireEnvPermission(permission.ComponentRead), GetComponentRuntimeMetrics)
		protected.GET("/environments/:id/components/:componentId/runtime-logs", middleware.RequireEnvPermission(permission.ComponentRead), GetComponentRuntimeLogs)
		protected.GET("/environments/:id/components/:componentId/console", middleware.RequireEnvPermission(permission.ComponentRead), HandleComponentConsole)
		protected.Any("/environments/:id/components/:componentId/proxy/*path", middleware.RequireEnvPermission(permission.ComponentRead), ProxyComponent)
		protected.GET("/environments/:id/adoptable-resources", middleware.RequireEnvPermission(permission.ComponentRead), ListAdoptableResources)
		protected.POST("/environments/:id/adoptable-resources", middleware.RequireEnvPermission(permission.ComponentCreate), AdoptResource)

		// Component deploy / version bump (used by CI callback)
		protected.PUT("/components/:id", middleware.RequireComponentPermission(permission.ComponentManage), UpdateComponent)
		protected.POST("/components/:id/deploy", middleware.RequireComponentPermission(permission.ComponentDeploy), DeployComponent)
		// Component external access toggle
		protected.PUT("/environments/:id/components/:componentId/external-access", middleware.RequireEnvPermission(permission.ComponentDeploy), SetComponentExternalAccess)
		protected.PUT("/environments/:id/components/:componentId/nodeport-access", middleware.RequireEnvPermission(permission.ComponentDeploy), SetComponentNodePortAccess)
		// Component delete
		protected.DELETE("/components/:id", middleware.RequireComponentPermission(permission.ComponentManage), DeleteComponent)
		protected.DELETE("/environments/:id/capabilities/:capability", middleware.RequireEnvPermission(permission.EnvManage), DeleteEnvironmentCapability)
		protected.GET("/capabilities/shared-resources", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), ListSharedCapabilityResources)
		protected.GET("/platform/services/stats", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), ListPlatformServiceStats)
		protected.GET("/platform/services/:type/instances", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), ListPlatformServiceInstances)
		protected.GET("/platform/services/:type/usage", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), ListPlatformServiceUsage)
		protected.GET("/platform-addons", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), ListPlatformAddons)
		protected.GET("/platform-addons/:name", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), GetPlatformAddon)
		protected.POST("/platform-addons/sync", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), SyncPlatformAddons)
		protected.POST("/platform-addons/upload", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), UploadPlatformAddon)
		protected.POST("/platform-addons/:name/enable", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), EnablePlatformAddon)
		protected.POST("/platform-addons/:name/disable", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), DisablePlatformAddon)
		protected.POST("/platform-addons/:name/check", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), CheckPlatformAddon)

		// Platform admin routes
		admin := protected.Group("/admin")
		{
			admin.GET("/users", middleware.RequireSystemPermission(permission.SystemUserManage), ListUsers)
			admin.PUT("/users/:id/role", middleware.RequireSystemPermission(permission.SystemUserManage), UpdateUserRole)
			admin.GET("/roles", middleware.RequireSystemPermission(permission.SystemRoleManage), ListRoles)
			admin.POST("/roles", middleware.RequireSystemPermission(permission.SystemRoleManage), CreateRole)
			admin.PUT("/roles/:id", middleware.RequireSystemPermission(permission.SystemRoleManage), UpdateRole)
			admin.DELETE("/roles/:id", middleware.RequireSystemPermission(permission.SystemRoleManage), DeleteRole)
			admin.GET("/shared-resource-pool", middleware.RequireSystemPermission(permission.SystemSharedPoolManage), GetSharedResourcePool)
		}
	}
}
