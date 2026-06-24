package handler

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	paapv1 "paap/api/v1"
	"paap/internal/database"
	paaphelm "paap/internal/helm"
	"paap/internal/k8s"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// S3 storage configuration
const (
	s3Endpoint   = "minio.paap-system.svc.cluster.local:9000"
	s3AccessKey  = "minioadmin"
	s3SecretKey  = "minioadmin123"
	s3BucketName = "paap-charts"
	s3UseSSL     = false
)

// getOrCreateS3Client returns a cached S3 client or creates a new one
var s3Client *k8s.S3Client

func getOrCreateS3Client() (*k8s.S3Client, error) {
	if s3Client != nil {
		return s3Client, nil
	}
	client, err := k8s.NewS3Client(s3Endpoint, s3AccessKey, s3SecretKey, s3BucketName, s3UseSSL)
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}
	s3Client = client
	return client, nil
}

// Template CRUD

type CreateTemplateRequest struct {
	Name         string   `json:"name" binding:"required"`
	Description  string   `json:"description"`
	Services     []string `json:"services"`
	Infra        []string `json:"infra"`
	ResourceCPU  string   `json:"resourceCpu"`
	ResourceMem  string   `json:"resourceMem"`
	ResourceDisk string   `json:"resourceDisk"`
}

type UpdateTemplateRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Services     []string `json:"services"`
	Infra        []string `json:"infra"`
	ResourceCPU  string   `json:"resourceCpu"`
	ResourceMem  string   `json:"resourceMem"`
	ResourceDisk string   `json:"resourceDisk"`
}

// ListTemplates returns all environment templates
func ListTemplates(c *gin.Context) {
	var templates []model.EnvTemplate
	if err := database.DB.Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

// ListServiceTemplates returns all service templates
func ListServiceTemplates(c *gin.Context) {
	var templates []model.ServiceTemplate
	if err := database.DB.Where("enabled = ?", true).Order("install_order").Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

// GetServiceTemplate returns a single service template
func GetServiceTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var tmpl model.ServiceTemplate
	if err := database.DB.First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tmpl})
}

type CreateServiceTemplateRequest struct {
	Type               string `json:"type" binding:"required"`
	Name               string `json:"name" binding:"required"`
	Category           string `json:"category" binding:"required"`
	Description        string `json:"description"`
	Installer          string `json:"installer" binding:"required"`
	ChartRepo          string `json:"chartRepo"`
	ChartName          string `json:"chartName"`
	ChartVersion       string `json:"chartVersion"`
	DefaultValues      string `json:"defaultValues"`
	ConfigurableParams string `json:"configurableParams"`
	RawYamlTemplate    string `json:"rawYamlTemplate"`
}

// CreateServiceTemplate creates a user-defined service template
func CreateServiceTemplate(c *gin.Context) {
	var req CreateServiceTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tmpl := model.ServiceTemplate{
		Type:               req.Type,
		Name:               req.Name,
		Category:           req.Category,
		Description:        req.Description,
		Installer:          req.Installer,
		ChartRepo:          req.ChartRepo,
		ChartName:          req.ChartName,
		ChartVersion:       req.ChartVersion,
		DefaultValues:      req.DefaultValues,
		ConfigurableParams: req.ConfigurableParams,
		RawYamlTemplate:    req.RawYamlTemplate,
		Enabled:            true,
	}

	if err := database.DB.Create(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": tmpl})
}

// UpdateServiceTemplate updates an existing service template
func UpdateServiceTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var tmpl model.ServiceTemplate
	if err := database.DB.First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	var req CreateServiceTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{
		"name":                req.Name,
		"description":         req.Description,
		"installer":           req.Installer,
		"chart_repo":          req.ChartRepo,
		"chart_name":          req.ChartName,
		"chart_version":       req.ChartVersion,
		"default_values":      req.DefaultValues,
		"configurable_params": req.ConfigurableParams,
		"raw_yaml_template":   req.RawYamlTemplate,
	}

	if err := database.DB.Model(&tmpl).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": tmpl})
}

// DeleteServiceTemplate deletes a service template (hard delete)
func DeleteServiceTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := database.DB.Unscoped().Delete(&model.ServiceTemplate{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// GetTemplate returns a single template
func GetTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var tmpl model.EnvTemplate
	if err := database.DB.First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tmpl})
}

// CreateTemplate creates a new environment template
func CreateTemplate(c *gin.Context) {
	var req CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tmpl := model.EnvTemplate{
		Name:         req.Name,
		Description:  req.Description,
		ResourceCPU:  req.ResourceCPU,
		ResourceMem:  req.ResourceMem,
		ResourceDisk: req.ResourceDisk,
	}
	if len(req.Services) > 0 {
		tmpl.Services = toJSON(req.Services)
	}
	if len(req.Infra) > 0 {
		tmpl.Infra = toJSON(req.Infra)
	}

	if err := database.DB.Create(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": tmpl})
}

// UpdateTemplate updates a template
func UpdateTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var tmpl model.EnvTemplate
	if err := database.DB.First(&tmpl, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.ResourceCPU != "" {
		updates["resource_cpu"] = req.ResourceCPU
	}
	if req.ResourceMem != "" {
		updates["resource_mem"] = req.ResourceMem
	}
	if req.ResourceDisk != "" {
		updates["resource_disk"] = req.ResourceDisk
	}
	if len(req.Services) > 0 {
		updates["services"] = toJSON(req.Services)
	}
	if len(req.Infra) > 0 {
		updates["infra"] = toJSON(req.Infra)
	}

	if err := database.DB.Model(&model.EnvTemplate{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

// DeleteTemplate deletes a template
func DeleteTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := database.DB.Delete(&model.EnvTemplate{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// --- Service Catalog ---

var unsupportedServiceCatalogTypes = []string{"kingbase", "nacos"}

// ListServiceCatalog returns all available service types
func ListServiceCatalog(c *gin.Context) {
	var services []model.ServiceCatalog
	if err := database.DB.Where("enabled = ?", true).Not("type IN ?", unsupportedServiceCatalogTypes).Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": services})
}

// SeedServiceCatalog creates default service types and templates
func SeedServiceCatalog() {
	services := []model.ServiceCatalog{
		// Tools
		{Type: "deploy", Name: "部署服务", Category: "tool", Description: "ArgoCD: 管理应用的部署、版本、回滚", Icon: "rocket", Enabled: true},
		{Type: "ci", Name: "CI 服务", Category: "tool", Description: "Tekton/Jenkins: 自动构建和测试代码", Icon: "flow", Enabled: true},
		{Type: "monitor", Name: "监控服务", Category: "tool", Description: "Prometheus+Grafana: 资源监控与告警", Icon: "chart-line", Enabled: true},
		{Type: "log", Name: "日志服务", Category: "tool", Description: "Loki: 日志收集与查询", Icon: "document", Enabled: true},
		{Type: "registry", Name: "轻量镜像仓库", Category: "tool", Description: "Docker Registry v2: 轻量 OCI 镜像仓库", Icon: "cube", Enabled: true},
		{Type: "harbor", Name: "企业镜像仓库", Category: "tool", Description: "Harbor: 企业级容器镜像管理", Icon: "cube", Enabled: true},
		{Type: "git", Name: "代码仓库", Category: "tool", Description: "Gitea: 轻量 Git 代码仓库", Icon: "document", Enabled: true},
		// Infra - Database
		{Type: "postgresql", Name: "PostgreSQL", Category: "infra", Description: "关系型数据库", Icon: "database", Enabled: true},
		{Type: "mysql", Name: "MySQL", Category: "infra", Description: "关系型数据库", Icon: "database", Enabled: true},
		{Type: "kingbase", Name: "人大金仓", Category: "infra", Description: "国产关系型数据库", Icon: "database", Enabled: false},
		{Type: "mongodb", Name: "MongoDB", Category: "infra", Description: "文档型数据库", Icon: "database", Enabled: true},
		// Infra - Cache
		{Type: "redis", Name: "Redis", Category: "infra", Description: "缓存服务", Icon: "cloud", Enabled: true},
		// Infra - MQ
		{Type: "rabbitmq", Name: "RabbitMQ", Category: "infra", Description: "消息队列", Icon: "network", Enabled: true},
		{Type: "kafka", Name: "Kafka", Category: "infra", Description: "消息队列", Icon: "network", Enabled: true},
		// Infra - Storage
		{Type: "minio", Name: "MinIO", Category: "infra", Description: "对象存储", Icon: "data-base", Enabled: true},
		// Infra - Service Discovery
		{Type: "nacos", Name: "Nacos", Category: "infra", Description: "注册中心与配置中心", Icon: "server", Enabled: false},
	}

	for _, s := range services {
		var existing model.ServiceCatalog
		if err := database.DB.Where("type = ?", s.Type).Assign(s).FirstOrCreate(&existing).Error; err != nil {
			log.Printf("[SeedServiceCatalog] failed to seed %s: %v", s.Type, err)
		}
	}
	if err := database.DB.Model(&model.ServiceCatalog{}).Where("type IN ?", unsupportedServiceCatalogTypes).Update("enabled", false).Error; err != nil {
		log.Printf("[SeedServiceCatalog] failed to disable unsupported catalog entries: %v", err)
	}
	if err := database.DB.Where("type = ?", "docker-registry").Delete(&model.ServiceCatalog{}).Error; err != nil {
		log.Printf("[SeedServiceCatalog] failed to remove obsolete docker-registry catalog: %v", err)
	}

	// Seed ServiceTemplate entries for built-in tools with Helm chart info
	SeedServiceTemplates()
}

func SeedServiceTemplates() {
	migrateServiceTemplateUniqueIndex()
	chartsDir := "data/charts"

	for _, archive := range builtInTemplateArchives() {
		tmplBase, ok := builtInServiceTemplateByType(archive.ServiceType)
		if !ok {
			continue
		}
		tmpl := tmplBase

		// Parse version info from chart/Chart.yaml
		localPath := filepath.Join(chartsDir, archive.ChartName+".tar.gz")
		if chartVersion, appVersion, err := extractChartYamlMeta(localPath); err == nil {
			tmpl.ChartVersion = chartVersion
			tmpl.AppVersion = appVersion
		} else {
			log.Printf("[SeedServiceTemplates] could not read version from %s: %v", localPath, err)
		}

		// Parse manifest from platform-manifest.yaml
		tmpl.PlatformManifestJSON = builtInManifestJSON(archive.ServiceType)
		tmpl.WorkloadRolePolicy = builtInWorkloadRolePolicy(archive.ServiceType)
		tmpl.EnvironmentRolePolicy = builtInEnvironmentRolePolicy(archive.ServiceType)

		// Upsert by type + s3_key (safe even after removing unique index)
		var existing model.ServiceTemplate
		if err := database.DB.Where("type = ? AND s3_key = ?", tmpl.Type, tmpl.S3Key).First(&existing).Error; err != nil {
			if err := database.DB.Create(&tmpl).Error; err != nil {
				log.Printf("[SeedServiceTemplates] failed to create %s: %v", tmpl.Type, err)
			}
		} else {
			tmpl.ID = existing.ID
			if err := database.DB.Model(&existing).Updates(tmpl).Error; err != nil {
				log.Printf("[SeedServiceTemplates] failed to update %s: %v", tmpl.Type, err)
			}
		}
	}
	removeObsoleteDockerRegistryTemplate()
}

func migrateServiceTemplateUniqueIndex() {
	oldIndexName := "idx_service_templates_type"
	if database.DB.Migrator().HasIndex(&model.ServiceTemplate{}, oldIndexName) {
		if err := database.DB.Migrator().DropIndex(&model.ServiceTemplate{}, oldIndexName); err != nil {
			log.Printf("[migrateServiceTemplateUniqueIndex] drop %s: %v", oldIndexName, err)
		} else {
			log.Printf("[migrateServiceTemplateUniqueIndex] dropped %s", oldIndexName)
		}
	}
}

func builtInManifestJSON(templateType string) string {
	manifest, err := readBuiltInManifest(templateType)
	if err != nil {
		log.Printf("[builtInManifestJSON] failed to read %s manifest: %v", templateType, err)
		return ""
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		log.Printf("[builtInManifestJSON] failed to marshal %s manifest: %v", templateType, err)
		return ""
	}
	return string(data)
}

func builtInWorkloadRolePolicy(templateType string) string {
	manifest, err := readBuiltInManifest(templateType)
	if err != nil {
		log.Printf("[builtInWorkloadRolePolicy] failed to read %s manifest: %v", templateType, err)
		return "[]"
	}
	return manifest.ToWorkloadRoleJSON()
}

func builtInEnvironmentRolePolicy(templateType string) string {
	manifest, err := readBuiltInManifest(templateType)
	if err != nil {
		log.Printf("[builtInEnvironmentRolePolicy] failed to read %s manifest: %v", templateType, err)
		return "[]"
	}
	return manifest.ToEnvironmentRoleJSON()
}

func readBuiltInManifest(templateType string) (*model.PlatformManifest, error) {
	examplePath := filepath.Join("docs", "examples", "built-in-templates", templateType, "platform-manifest.yaml")
	if data, err := os.ReadFile(examplePath); err == nil {
		var manifest model.PlatformManifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return nil, err
		}
		return &manifest, nil
	}

	chartPath := filepath.Join("/charts", templateType+".tar.gz")
	return extractManifestFromTar(chartPath)
}

type builtInTemplateArchive struct {
	ServiceType string
	ChartName   string
}

func builtInTemplateArchives() []builtInTemplateArchive {
	return []builtInTemplateArchive{
		{ServiceType: "deploy", ChartName: "argocd"},
		{ServiceType: "ci", ChartName: "jenkins"},
		{ServiceType: "monitor", ChartName: "monitor"},
		{ServiceType: "log", ChartName: "loki"},
		{ServiceType: "registry", ChartName: "registry"},
		{ServiceType: "harbor", ChartName: "harbor"},
		{ServiceType: "git", ChartName: "gitea"},
		{ServiceType: "postgresql", ChartName: "postgresql"},
		{ServiceType: "mysql", ChartName: "mysql"},
		{ServiceType: "mongodb", ChartName: "mongodb"},
		{ServiceType: "redis", ChartName: "redis"},
		{ServiceType: "rabbitmq", ChartName: "rabbitmq"},
		{ServiceType: "kafka", ChartName: "kafka"},
		{ServiceType: "minio", ChartName: "minio"},
	}
}

func builtInChartArchives() []builtInTemplateArchive {
	archives := append([]builtInTemplateArchive{}, builtInTemplateArchives()...)
	archives = append(archives, builtInTemplateArchive{ServiceType: "redis", ChartName: "redis-cluster"})
	archives = append(archives, builtInTemplateArchive{ServiceType: "mysql", ChartName: "mysql-galera"})
	archives = append(archives, builtInTemplateArchive{ServiceType: "postgresql", ChartName: "postgresql-ha"})
	return archives
}

func builtInServiceTemplateByType(serviceType string) (model.ServiceTemplate, bool) {
	templates := map[string]model.ServiceTemplate{
		"deploy": {
			Type:         "deploy",
			Name:         "ArgoCD (官方)",
			Category:     "tool",
			Description:  "ArgoCD - GitOps 持续部署工具",
			Icon:         "rocket",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/argocd.tar.gz",
			IsCustom:     false,
			InstallOrder: 10,
			Enabled:      true,
		},
		"ci": {
			Type:         "ci",
			Name:         "Jenkins (官方)",
			Category:     "tool",
			Description:  "Jenkins - CI/CD 服务器",
			Icon:         "flow",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/jenkins.tar.gz",
			IsCustom:     false,
			InstallOrder: 40,
			Enabled:      true,
		},
		"monitor": {
			Type:         "monitor",
			Name:         "Prometheus+Grafana (官方)",
			Category:     "tool",
			Description:  "Prometheus + Grafana - 全栈监控",
			Icon:         "chart-line",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/monitor.tar.gz",
			IsCustom:     false,
			InstallOrder: 50,
			Enabled:      true,
		},
		"log": {
			Type:         "log",
			Name:         "Loki+Promtail (官方)",
			Category:     "tool",
			Description:  "Loki + Promtail - 日志收集与查询",
			Icon:         "document",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/loki.tar.gz",
			IsCustom:     false,
			InstallOrder: 60,
			Enabled:      true,
		},
		"registry": {
			Type:         "registry",
			Name:         "Docker Registry v2",
			Category:     "tool",
			Description:  "Docker Registry v2 - 轻量 OCI 镜像仓库，适合开发测试环境",
			Icon:         "cube",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/registry.tar.gz",
			IsCustom:     false,
			InstallOrder: 20,
			Enabled:      true,
		},
		"harbor": {
			Type:         "harbor",
			Name:         "Harbor (官方)",
			Category:     "tool",
			Description:  "Harbor - 企业级镜像仓库，组件多、镜像大，建议生产或资源充足环境使用",
			Icon:         "cube",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/harbor.tar.gz",
			IsCustom:     false,
			InstallOrder: 900,
			Enabled:      true,
		},
		"git": {
			Type:         "git",
			Name:         "Gitea",
			Category:     "tool",
			Description:  "Gitea - 轻量 Git 代码仓库",
			Icon:         "document",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/gitea.tar.gz",
			IsCustom:     false,
			InstallOrder: 30,
			Enabled:      true,
		},
		"postgresql": {
			Type:         "postgresql",
			Name:         "PostgreSQL (Bitnami)",
			Category:     "infra",
			Description:  "PostgreSQL - 关系型数据库",
			Icon:         "database",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/postgresql.tar.gz",
			IsCustom:     false,
			InstallOrder: 100,
			Enabled:      true,
		},
		"mysql": {
			Type:         "mysql",
			Name:         "MySQL (Bitnami)",
			Category:     "infra",
			Description:  "MySQL - 关系型数据库",
			Icon:         "database",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/mysql.tar.gz",
			IsCustom:     false,
			InstallOrder: 110,
			Enabled:      true,
		},
		"mongodb": {
			Type:         "mongodb",
			Name:         "MongoDB (Bitnami)",
			Category:     "infra",
			Description:  "MongoDB - 文档型数据库",
			Icon:         "database",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/mongodb.tar.gz",
			IsCustom:     false,
			InstallOrder: 120,
			Enabled:      true,
		},
		"redis": {
			Type:         "redis",
			Name:         "Redis (Bitnami)",
			Category:     "infra",
			Description:  "Redis - 缓存服务",
			Icon:         "cloud",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/redis.tar.gz",
			IsCustom:     false,
			InstallOrder: 130,
			Enabled:      true,
		},
		"rabbitmq": {
			Type:         "rabbitmq",
			Name:         "RabbitMQ (Bitnami)",
			Category:     "infra",
			Description:  "RabbitMQ - 消息队列",
			Icon:         "network",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/rabbitmq.tar.gz",
			IsCustom:     false,
			InstallOrder: 140,
			Enabled:      true,
		},
		"kafka": {
			Type:         "kafka",
			Name:         "Kafka (Bitnami)",
			Category:     "infra",
			Description:  "Apache Kafka - 流处理平台",
			Icon:         "network",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/kafka.tar.gz",
			IsCustom:     false,
			InstallOrder: 150,
			Enabled:      true,
		},
		"minio": {
			Type:         "minio",
			Name:         "MinIO (Bitnami)",
			Category:     "infra",
			Description:  "MinIO - 对象存储",
			Icon:         "data-base",
			Installer:    "helm",
			S3Bucket:     "paap-charts",
			S3Key:        "charts/minio.tar.gz",
			IsCustom:     false,
			InstallOrder: 160,
			Enabled:      true,
		},
	}
	tmpl, ok := templates[serviceType]
	return tmpl, ok
}

func removeObsoleteDockerRegistryTemplate() {
	if err := database.DB.Where("type = ?", "docker-registry").Delete(&model.ServiceTemplate{}).Error; err != nil {
		log.Printf("[SeedServiceTemplates] failed to remove obsolete docker-registry template: %v", err)
	}
	if err := database.DB.Where("service_type = ?", "docker-registry").Delete(&model.ServiceInstallation{}).Error; err != nil {
		log.Printf("[SeedServiceTemplates] failed to remove obsolete docker-registry installs: %v", err)
	}
}

// SeedBuiltinChartsToS3 uploads built-in chart packages to S3.
// If force=true, re-uploads even if already exists (for template updates).
func SeedBuiltinChartsToS3(force bool) {
	log.Printf("[SeedBuiltinChartsToS3] Starting seed process...")

	s3, err := getOrCreateS3Client()
	if err != nil {
		log.Printf("[SeedBuiltinChartsToS3] S3 client not available, skipping: %v", err)
		return
	}

	log.Printf("[SeedBuiltinChartsToS3] S3 client created successfully")

	ctx := context.Background()
	for _, archive := range builtInChartArchives() {
		s3Key := fmt.Sprintf("charts/%s.tar.gz", archive.ChartName)
		if !force && s3.ObjectExists(ctx, s3Key) {
			log.Printf("[SeedBuiltinChartsToS3] %s already exists in S3, skipping", s3Key)
			continue
		}
		if force && s3.ObjectExists(ctx, s3Key) {
			log.Printf("[SeedBuiltinChartsToS3] %s exists in S3, force re-uploading", s3Key)
		}

		localPath := fmt.Sprintf("/charts/%s.tar.gz", archive.ChartName)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			log.Printf("[SeedBuiltinChartsToS3] local file not found: %s, skipping", localPath)
			continue
		}

		if err := s3.UploadFile(ctx, s3Key, localPath, "application/gzip"); err != nil {
			log.Printf("[SeedBuiltinChartsToS3] failed to upload %s: %v", archive.ChartName, err)
		} else {
			log.Printf("[SeedBuiltinChartsToS3] uploaded %s to S3", archive.ChartName)
		}
	}

	log.Printf("[SeedBuiltinChartsToS3] Seed process completed")
}

type BuiltInTemplateSyncResult struct {
	Updated int `json:"updated"`
}

func SyncBuiltinTemplatesNow(ctx context.Context, forceUpload bool) (BuiltInTemplateSyncResult, error) {
	if forceUpload {
		SeedBuiltinChartsToS3(true)
	}
	removeObsoleteDockerRegistryTemplate()

	updated := 0
	for _, archive := range builtInTemplateArchives() {
		localPath := fmt.Sprintf("/charts/%s.tar.gz", archive.ChartName)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			continue
		}

		manifest, err := extractManifestFromTar(localPath)
		if err != nil {
			log.Printf("[SyncBuiltinTemplates] failed to parse %s: %v", archive.ChartName, err)
			continue
		}

		chartVersion, appVersion, _ := extractChartYamlMeta(localPath)

		manifestJSON, _ := json.Marshal(manifest)
		workloadRoleJSON := manifest.ToWorkloadRoleJSON()
		environmentRoleJSON := manifest.ToEnvironmentRoleJSON()

		builtinS3Key := fmt.Sprintf("charts/%s.tar.gz", archive.ChartName)
		updates := map[string]interface{}{
			"platform_manifest_json":  string(manifestJSON),
			"workload_role_policy":    workloadRoleJSON,
			"environment_role_policy": environmentRoleJSON,
			"install_order":           builtInInstallOrder(archive.ServiceType),
			"chart_version":           chartVersion,
			"app_version":             appVersion,
		}
		if tmpl, ok := builtInServiceTemplateByType(archive.ServiceType); ok {
			updates["name"] = tmpl.Name
			updates["category"] = tmpl.Category
			updates["description"] = tmpl.Description
			updates["icon"] = tmpl.Icon
			updates["installer"] = tmpl.Installer
			updates["chart_repo"] = tmpl.ChartRepo
			updates["chart_name"] = tmpl.ChartName
			updates["default_values"] = tmpl.DefaultValues
			updates["configurable_params"] = tmpl.ConfigurableParams
			updates["raw_yaml_template"] = tmpl.RawYamlTemplate
			updates["is_custom"] = tmpl.IsCustom
			updates["chart_archive_path"] = tmpl.ChartArchivePath
			updates["s3_bucket"] = tmpl.S3Bucket
			updates["s3_key"] = builtinS3Key
			updates["preset_values"] = tmpl.PresetValues
			updates["enabled"] = tmpl.Enabled
		} else if description := builtInDescriptionOverride(archive.ServiceType); description != "" {
			updates["description"] = description
		}

		result := database.DB.Model(&model.ServiceTemplate{}).Where("type = ? AND s3_key = ?", archive.ServiceType, builtinS3Key).Updates(updates)
		if result.Error != nil {
			return BuiltInTemplateSyncResult{Updated: updated}, result.Error
		}
		if result.RowsAffected > 0 {
			updated++
			log.Printf("[SyncBuiltinTemplates] updated DB for %s", archive.ServiceType)
		} else if tmpl, ok := builtInServiceTemplateByType(archive.ServiceType); ok {
			tmpl.ChartVersion = chartVersion
			tmpl.AppVersion = appVersion
			tmpl.PlatformManifestJSON = string(manifestJSON)
			tmpl.WorkloadRolePolicy = workloadRoleJSON
			tmpl.EnvironmentRolePolicy = environmentRoleJSON
			if err := database.DB.Create(&tmpl).Error; err != nil {
				return BuiltInTemplateSyncResult{Updated: updated}, err
			}
			updated++
			log.Printf("[SyncBuiltinTemplates] created DB template for %s", archive.ServiceType)
		}

		var refreshedTemplate model.ServiceTemplate
		if err := database.DB.Where("type = ? AND s3_key = ?", archive.ServiceType, builtinS3Key).First(&refreshedTemplate).Error; err != nil {
			return BuiltInTemplateSyncResult{Updated: updated}, err
		}
		if k8s.GetClient() != nil {
			if err := refreshBuiltInServiceInstances(ctx, archive.ServiceType, refreshedTemplate); err != nil {
				return BuiltInTemplateSyncResult{Updated: updated}, err
			}
		}
	}

	return BuiltInTemplateSyncResult{Updated: updated}, nil
}

// SyncBuiltinTemplates forces re-upload of all built-in charts to S3 and updates
// the DB with platform-manifest.yaml parsed from each tar file.
func SyncBuiltinTemplates(c *gin.Context) {
	result, err := SyncBuiltinTemplatesNow(c.Request.Context(), true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "updated": result.Updated})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "sync completed",
		"updated": result.Updated,
	})
}

func refreshBuiltInServiceInstances(ctx context.Context, serviceType string, template model.ServiceTemplate) error {
	var installs []model.ServiceInstallation
	if err := database.DB.Where("service_type = ?", serviceType).Find(&installs).Error; err != nil {
		return err
	}
	for _, inst := range installs {
		var env model.Environment
		if err := database.DB.First(&env, inst.EnvironmentID).Error; err != nil {
			return err
		}
		var app model.Application
		if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
			return err
		}

		helmSpec := buildHelmInstallSpec(&app, &env, &template, serviceType)
		if strings.TrimSpace(inst.Namespace) != "" {
			helmSpec.Namespace = strings.TrimSpace(inst.Namespace)
			rewriteServiceTemplateToolNamespaceValues(helmSpec, helmSpec.Namespace)
		}
		if strings.TrimSpace(inst.ReleaseName) != "" {
			helmSpec.ReleaseName = strings.TrimSpace(inst.ReleaseName)
			if helmSpec.Values != nil {
				if _, ok := helmSpec.Values["fullnameOverride"]; ok {
					helmSpec.Values["fullnameOverride"] = helmSpec.ReleaseName
				}
			}
		}
		workloadRole := getWorkloadRole(serviceType)
		toolNamespaceRole := getToolNamespaceRole(serviceType)
		environmentRole := getEnvironmentRole(serviceType)
		clusterRole := getClusterRole(serviceType)
		resourceLabels := serviceResourceLabels(app.Identifier, env.Identifier, &template, serviceType)
		resourceAnnotations := serviceResourceAnnotations(app.Identifier, env.Identifier, &template, serviceType)
		if err := k8s.UpsertServiceInstanceCR(ctx, app.Identifier, env.Identifier, serviceType, workloadRole, toolNamespaceRole, environmentRole, clusterRole, nil, helmSpec, resourceLabels, resourceAnnotations); err != nil {
			return err
		}

		inst.Namespace = helmSpec.Namespace
		inst.ReleaseName = helmSpec.ReleaseName
		inst.Values = "{}"
		inst.Status = "installing"
		inst.ErrorMessage = ""
		if err := database.DB.Save(&inst).Error; err != nil {
			return err
		}

		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func rewriteServiceTemplateToolNamespaceValues(helmSpec *paapv1.HelmInstallSpec, toolNS string) {
	if helmSpec == nil || helmSpec.Values == nil || strings.TrimSpace(toolNS) == "" {
		return
	}
	for _, key := range []string{
		"tool_namespace",
		"paap.toolNamespace",
		"global.paap.toolNamespace",
		"serviceAccount.name",
		"controller.serviceAccount.name",
		"server.serviceAccount.name",
		"repoServer.serviceAccount.name",
		"redis.serviceAccount.name",
		"applicationSet.serviceAccount.name",
	} {
		if _, ok := helmSpec.Values[key]; ok {
			helmSpec.Values[key] = toolNS
		}
	}
}

func builtInInstallOrder(serviceType string) int {
	orders := map[string]int{
		"deploy":     10,
		"registry":   20,
		"git":        30,
		"ci":         40,
		"monitor":    50,
		"log":        60,
		"postgresql": 100,
		"mysql":      110,
		"mongodb":    120,
		"redis":      130,
		"rabbitmq":   140,
		"kafka":      150,
		"minio":      160,
		"harbor":     900,
	}
	if order, ok := orders[serviceType]; ok {
		return order
	}
	return 500
}

func builtInDescriptionOverride(serviceType string) string {
	descriptions := map[string]string{
		"registry": "Docker Registry v2 - 轻量 OCI 镜像仓库，适合开发测试环境",
		"harbor":   "Harbor - 企业级镜像仓库，组件多、镜像大，建议生产或资源充足环境使用",
	}
	return descriptions[serviceType]
}

// extractManifestFromTar reads a tar.gz and extracts platform-manifest.yaml
func extractManifestFromTar(tarPath string) (*model.PlatformManifest, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err != nil {
			return nil, fmt.Errorf("platform-manifest.yaml not found in tar")
		}
		name := header.Name
		if len(name) > 2 && name[:2] == "./" {
			name = name[2:]
		}
		if name == "platform-manifest.yaml" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			var manifest model.PlatformManifest
			if err := yaml.Unmarshal(data, &manifest); err != nil {
				return nil, err
			}
			return &manifest, nil
		}
	}
}

type chartYamlMeta struct {
	Version    string `yaml:"version"`
	AppVersion string `yaml:"appVersion"`
}

func extractChartYamlMeta(tarPath string) (chartVersion, appVersion string, err error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", "", fmt.Errorf("not a valid gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", err
		}
		name := strings.TrimPrefix(header.Name, "./")
		if name == "chart/Chart.yaml" {
			data, err := io.ReadAll(tr)
			if err != nil {
				return "", "", err
			}
			var meta chartYamlMeta
			if err := yaml.Unmarshal(data, &meta); err != nil {
				return "", "", fmt.Errorf("failed to parse chart/Chart.yaml: %w", err)
			}
			return meta.Version, meta.AppVersion, nil
		}
	}
	return "", "", fmt.Errorf("chart/Chart.yaml not found in archive")
}

// --- BYO Custom Template Upload ---

const chartStorageDir = "data/charts"

// UploadTemplate handles custom template (BYO) upload.
// Accepts a multipart form with:
//   - file: tar.gz archive containing Helm chart + platform-manifest.yaml
//   - type: unique service type identifier
//   - name: display name
//   - category: "tool" or "infra"
//   - description: what this tool does
//
// Validation rules:
//   - Archive MUST contain platform-manifest.yaml at root
//   - Archive MUST NOT contain ClusterRole or ClusterRoleBinding
func UploadTemplate(c *gin.Context) {
	typ := c.PostForm("type")
	name := c.PostForm("name")
	category := c.PostForm("category")
	description := c.PostForm("description")

	if typ == "" || name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type and name are required"})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing 'file' field"})
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".tar.gz") && !strings.HasSuffix(header.Filename, ".tgz") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file must be .tar.gz or .tgz"})
		return
	}

	// Save to temp file
	tmpFile, err := os.CreateTemp("", "paap-chart-*.tar.gz")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()
	io.Copy(tmpFile, file)
	tmpFile.Seek(0, 0)

	// Extract and validate
	manifestYaml, presetValues, _, forbiddenKinds, err := extractAndValidateArchive(tmpFile)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid archive: %v", err)})
		return
	}
	if len(forbiddenKinds) > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":          "archive contains forbidden cluster-scoped RBAC resources in chart/templates/",
			"forbiddenKinds": forbiddenKinds,
			"hint":           "Set rbac.create=false in preset-values.yaml. Platform manages RBAC externally.",
		})
		return
	}
	if manifestYaml == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "archive must contain platform-manifest.yaml at the root level"})
		return
	}

	var manifest model.PlatformManifest
	if err := yaml.Unmarshal([]byte(manifestYaml), &manifest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid platform-manifest.yaml: %v", err)})
		return
	}
	if err := manifest.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse Chart.yaml for version info
	chartVersion, appVersion, _ := extractChartYamlMeta(tmpFile.Name())

	// Store chart in S3
	s3, err := getOrCreateS3Client()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("S3 client error: %v", err)})
		return
	}
	s3Key := fmt.Sprintf("charts/%s-%s.tar.gz", typ, appVersion)
	if appVersion == "" {
		s3Key = fmt.Sprintf("charts/%s.tar.gz", typ)
	}
	if err := s3.UploadFile(context.Background(), s3Key, tmpFile.Name(), "application/gzip"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("S3 upload error: %v", err)})
		return
	}

	manifestJSON, _ := json.Marshal(manifest)
	if category == "" {
		category = "tool"
	}

	tmpl := model.ServiceTemplate{
		Type:                  typ,
		Name:                  name,
		Category:              category,
		Description:           description,
		Icon:                  "puzzle",
		Installer:             "helm",
		ChartVersion:          chartVersion,
		AppVersion:            appVersion,
		WorkloadRolePolicy:    manifest.ToWorkloadRoleJSON(),
		EnvironmentRolePolicy: manifest.ToEnvironmentRoleJSON(),
		IsCustom:              true,
		PlatformManifestJSON:  string(manifestJSON),
		S3Bucket:              s3BucketName,
		S3Key:                 s3Key,
		PresetValues:          presetValues,
		Enabled:               true,
	}

	if err := database.DB.Create(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("db error: %v", err)})
		return
	}

	log.Printf("[UploadTemplate] custom template '%s' (%s) uploaded", name, typ)
	c.JSON(http.StatusCreated, gin.H{"data": tmpl, "manifest": manifest})
}

// extractAndValidateArchive reads a tar.gz with the expected structure:
//
//	archive-root/
//	  platform-manifest.yaml    (required)
//	  preset-values.yaml        (optional - default Helm values to disable built-in RBAC etc.)
//	  chart/                    (required - original third-party Helm chart, unmodified)
//	    Chart.yaml
//	    values.yaml
//	    templates/
//	  dashboards/               (optional - Grafana dashboard JSONs)
//	    main-metrics.json
//
// It extracts the manifest, preset-values, dashboard JSONs, and uses the Helm SDK to
// render the chart with preset-values, then scans the rendered output for forbidden RBAC kinds.
func extractAndValidateArchive(r io.Reader) (manifestYaml, presetValues string, dashboards map[string]string, forbiddenKinds []string, err error) {
	// Create temp directory for extraction
	tmpDir, err := os.MkdirTemp("", "paap-template-validate-*")
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract archive
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("not a valid gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	hasChartDir := false
	dashboards = make(map[string]string)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", nil, nil, fmt.Errorf("read error: %w", err)
		}

		name := header.Name
		name = strings.TrimPrefix(name, "./")
		targetPath := filepath.Join(tmpDir, name)

		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(targetPath, 0755)
			if strings.HasPrefix(name, "chart/") {
				hasChartDir = true
			}
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(targetPath), 0755)
			f, err := os.Create(targetPath)
			if err != nil {
				return "", "", nil, nil, fmt.Errorf("failed to create file: %w", err)
			}
			io.Copy(f, tr)
			f.Close()

			baseName := filepath.Base(name)
			if strings.HasPrefix(name, "chart/") {
				hasChartDir = true
			}

			// Extract platform-manifest.yaml (must be at root level)
			if (baseName == "platform-manifest.yaml" || baseName == "platform-manifest.yml") && !strings.Contains(filepath.Dir(name), "/") {
				content, _ := os.ReadFile(targetPath)
				manifestYaml = string(content)
			}

			// Extract preset-values.yaml (root level)
			if (baseName == "preset-values.yaml" || baseName == "preset-values.yml") && !strings.Contains(filepath.Dir(name), "/") {
				content, _ := os.ReadFile(targetPath)
				presetValues = string(content)
			}

			// Extract dashboard JSONs from dashboards/ directory
			if strings.HasPrefix(name, "dashboards/") && strings.HasSuffix(baseName, ".json") {
				content, _ := os.ReadFile(targetPath)
				dashboards[baseName] = string(content)
			}
		}
	}

	if !hasChartDir {
		return "", "", nil, nil, fmt.Errorf("archive must contain a chart/ directory with the Helm chart")
	}

	// Render chart with Helm SDK and scan output for forbidden kinds
	chartPath := filepath.Join(tmpDir, "chart")
	values, err := paaphelm.BuildValues(presetValues, nil)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("preset-values.yaml parse failed: %w", err)
	}
	rendered, err := paaphelm.NewClient().RenderTemplate("validate-release", "paap-validate", chartPath, values)
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("helm template validation failed: %w", err)
	}

	// Scan rendered output for ClusterRole or ClusterRoleBinding
	for _, line := range strings.Split(rendered, "\n") {
		line = strings.TrimSpace(line)
		if line == "kind: ClusterRole" || strings.Contains(line, "kind: ClusterRoleBinding") {
			if strings.Contains(line, "ClusterRoleBinding") {
				forbiddenKinds = append(forbiddenKinds, "ClusterRoleBinding")
			} else {
				forbiddenKinds = append(forbiddenKinds, "ClusterRole")
			}
		}
	}

	// Deduplicate
	seen := make(map[string]bool)
	deduped := []string{}
	for _, k := range forbiddenKinds {
		if !seen[k] {
			seen[k] = true
			deduped = append(deduped, k)
		}
	}
	forbiddenKinds = deduped

	return manifestYaml, presetValues, dashboards, forbiddenKinds, nil
}
