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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"paap/internal/database"
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
	Type             string `json:"type" binding:"required"`
	Name             string `json:"name" binding:"required"`
	Category         string `json:"category" binding:"required"`
	Description      string `json:"description"`
	Installer        string `json:"installer" binding:"required"`
	ChartRepo        string `json:"chartRepo"`
	ChartName        string `json:"chartName"`
	ChartVersion     string `json:"chartVersion"`
	DefaultValues    string `json:"defaultValues"`
	ConfigurableParams string `json:"configurableParams"`
	RawYamlTemplate  string `json:"rawYamlTemplate"`
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
		"name":               req.Name,
		"description":        req.Description,
		"installer":          req.Installer,
		"chart_repo":         req.ChartRepo,
		"chart_name":         req.ChartName,
		"chart_version":      req.ChartVersion,
		"default_values":     req.DefaultValues,
		"configurable_params": req.ConfigurableParams,
		"raw_yaml_template":  req.RawYamlTemplate,
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

// ListServiceCatalog returns all available service types
func ListServiceCatalog(c *gin.Context) {
	var services []model.ServiceCatalog
	if err := database.DB.Where("enabled = ?", true).Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": services})
}

// SeedServiceCatalog creates default service types and templates
func SeedServiceCatalog() {
	var count int64
	database.DB.Model(&model.ServiceCatalog{}).Count(&count)
	if count > 0 {
		return
	}

	services := []model.ServiceCatalog{
		// Tools
		{Type: "deploy", Name: "部署服务", Category: "tool", Description: "ArgoCD: 管理应用的部署、版本、回滚", Icon: "rocket"},
		{Type: "ci", Name: "CI 服务", Category: "tool", Description: "Tekton/Jenkins: 自动构建和测试代码", Icon: "flow"},
		{Type: "monitor", Name: "监控服务", Category: "tool", Description: "Prometheus+Grafana: 资源监控与告警", Icon: "chart-line"},
		{Type: "log", Name: "日志服务", Category: "tool", Description: "Loki: 日志收集与查询", Icon: "document"},
		{Type: "registry", Name: "镜像仓库", Category: "tool", Description: "Harbor: 容器镜像管理", Icon: "cube"},
		// Infra - Database
		{Type: "postgresql", Name: "PostgreSQL", Category: "infra", Description: "关系型数据库", Icon: "database"},
		{Type: "mysql", Name: "MySQL", Category: "infra", Description: "关系型数据库", Icon: "database"},
		{Type: "kingbase", Name: "人大金仓", Category: "infra", Description: "国产关系型数据库", Icon: "database"},
		{Type: "mongodb", Name: "MongoDB", Category: "infra", Description: "文档型数据库", Icon: "database"},
		// Infra - Cache
		{Type: "redis", Name: "Redis", Category: "infra", Description: "缓存服务", Icon: "cloud"},
		// Infra - MQ
		{Type: "rabbitmq", Name: "RabbitMQ", Category: "infra", Description: "消息队列", Icon: "network"},
		{Type: "kafka", Name: "Kafka", Category: "infra", Description: "消息队列", Icon: "network"},
		// Infra - Storage
		{Type: "minio", Name: "MinIO", Category: "infra", Description: "对象存储", Icon: "data-base"},
		// Infra - Service Discovery
		{Type: "nacos", Name: "Nacos", Category: "infra", Description: "注册中心与配置中心", Icon: "server"},
	}

	for _, s := range services {
		database.DB.Create(&s)
	}

	// Seed ServiceTemplate entries for built-in tools with Helm chart info
	SeedServiceTemplates()
}

// SeedServiceTemplates creates ServiceTemplate entries for built-in tools
// Stores chart archives in S3 (MinIO)
func SeedServiceTemplates() {
	var count int64
	database.DB.Model(&model.ServiceTemplate{}).Count(&count)
	if count > 0 {
		return
	}

	templates := []model.ServiceTemplate{
		// ArgoCD (deploy)
		{
			Type:                 "deploy",
			Name:                 "ArgoCD (官方)",
			Category:             "tool",
			Description:          "ArgoCD - GitOps 持续部署工具",
			Icon:                 "rocket",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/argocd.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"argocd","version":"v2.13.3","description":"ArgoCD - GitOps 持续部署工具","permissions":{"scope":"environment-wide","rules":[{"apiGroups":[""],"resources":["pods","services","configmaps","secrets","persistentvolumeclaims","serviceaccounts"],"verbs":["get","list","watch","create","update","patch","delete"]},{"apiGroups":["apps"],"resources":["deployments","statefulsets","replicasets"],"verbs":["get","list","watch","create","update","patch","delete"]},{"apiGroups":["networking.k8s.io"],"resources":["ingresses"],"verbs":["get","list","watch","create","update","patch","delete"]},{"apiGroups":["autoscaling"],"resources":["horizontalpodautoscalers"],"verbs":["get","list","watch","create","update","patch","delete"]}]},"observability":{"metrics":{"port":8083,"path":"/metrics"}}}`,
			WorkloadRolePolicy:   `[{"apiGroups":[""],"resources":["pods","services","configmaps","secrets","persistentvolumeclaims","serviceaccounts"],"verbs":["*"]},{"apiGroups":["apps"],"resources":["deployments","statefulsets","replicasets"],"verbs":["*"]},{"apiGroups":["networking.k8s.io"],"resources":["ingresses"],"verbs":["*"]},{"apiGroups":["autoscaling"],"resources":["horizontalpodautoscalers"],"verbs":["*"]}]`,
			Enabled:              true,
		},
		// Jenkins (ci)
		{
			Type:                 "ci",
			Name:                 "Jenkins (官方)",
			Category:             "tool",
			Description:          "Jenkins - CI/CD 服务器",
			Icon:                 "flow",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/jenkins.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"jenkins","version":"v2.440.3","description":"Jenkins - CI/CD 服务器","permissions":{"scope":"environment-wide","rules":[{"apiGroups":[""],"resources":["pods","pods/log","services","configmaps","secrets","serviceaccounts"],"verbs":["get","list","watch","create","update","patch","delete"]},{"apiGroups":["apps"],"resources":["deployments","replicasets"],"verbs":["get","list","watch","create","update","patch","delete"]},{"apiGroups":["batch"],"resources":["jobs"],"verbs":["get","list","watch","create","update","patch","delete"]},{"apiGroups":["networking.k8s.io"],"resources":["ingresses"],"verbs":["get","list","watch","create","update","patch","delete"]}]},"observability":{"metrics":{"port":8080,"path":"/metrics"}}}`,
			WorkloadRolePolicy:   `[{"apiGroups":[""],"resources":["pods","pods/log","services","configmaps","secrets","serviceaccounts"],"verbs":["*"]},{"apiGroups":["apps"],"resources":["deployments","replicasets"],"verbs":["*"]},{"apiGroups":["batch"],"resources":["jobs"],"verbs":["*"]},{"apiGroups":["networking.k8s.io"],"resources":["ingresses"],"verbs":["*"]}]`,
			Enabled:              true,
		},
		// Prometheus+Grafana (monitor)
		{
			Type:                 "monitor",
			Name:                 "Prometheus+Grafana (官方)",
			Category:             "tool",
			Description:          "Prometheus + Grafana - 全栈监控",
			Icon:                 "chart-line",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/monitor.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"kube-prometheus-stack","version":"v0.78.2","description":"Prometheus + Grafana + Alertmanager 全栈监控","permissions":{"scope":"environment-wide","rules":[{"apiGroups":[""],"resources":["pods","services","endpoints","nodes","namespaces","configmaps","secrets"],"verbs":["get","list","watch"]},{"apiGroups":[""],"resources":["nodes/metrics","nodes/proxy"],"verbs":["get","list","watch"]},{"apiGroups":["apps"],"resources":["deployments","daemonsets","statefulsets","replicasets"],"verbs":["get","list","watch"]},{"apiGroups":["networking.k8s.io"],"resources":["ingresses"],"verbs":["get","list","watch"]},{"apiGroups":["autoscaling"],"resources":["horizontalpodautoscalers"],"verbs":["get","list","watch"]}]},"observability":{"metrics":{"port":9090,"path":"/metrics"}}}`,
			WorkloadRolePolicy:   `[{"apiGroups":[""],"resources":["pods","services","endpoints","nodes","namespaces","configmaps","secrets"],"verbs":["get","list","watch"]},{"apiGroups":[""],"resources":["nodes/metrics","nodes/proxy"],"verbs":["get","list","watch"]},{"apiGroups":["apps"],"resources":["deployments","daemonsets","statefulsets","replicasets"],"verbs":["get","list","watch"]},{"apiGroups":["networking.k8s.io"],"resources":["ingresses"],"verbs":["get","list","watch"]},{"apiGroups":["autoscaling"],"resources":["horizontalpodautoscalers"],"verbs":["get","list","watch"]}]`,
			Enabled:              true,
		},
		// Loki (log)
		{
			Type:                 "log",
			Name:                 "Loki+Promtail (官方)",
			Category:             "tool",
			Description:          "Loki + Promtail - 日志收集与查询",
			Icon:                 "document",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/loki.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"loki","version":"v2.9.4","description":"Loki + Promtail - 日志收集与查询","permissions":{"scope":"environment-wide","rules":[{"apiGroups":[""],"resources":["pods","pods/log"],"verbs":["get","list","watch"]}]},"observability":{"metrics":{"port":3100,"path":"/metrics"}}}`,
			WorkloadRolePolicy:   `[{"apiGroups":[""],"resources":["pods","pods/log"],"verbs":["get","list","watch"]}]`,
			Enabled:              true,
		},
		// Harbor (registry)
		{
			Type:                 "registry",
			Name:                 "Harbor (官方)",
			Category:             "tool",
			Description:          "Harbor - 企业级容器镜像仓库",
			Icon:                 "cube",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/harbor.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"harbor","version":"v2.12.0","description":"Harbor - 企业级容器镜像仓库","permissions":{"scope":"tool-only","rules":[]},"observability":{"metrics":{"port":9090,"path":"/metrics"}}}`,
			WorkloadRolePolicy:   `[]`,
			Enabled:              true,
		},
		// PostgreSQL
		{
			Type:                 "postgresql",
			Name:                 "PostgreSQL (Bitnami)",
			Category:             "infra",
			Description:          "PostgreSQL - 关系型数据库",
			Icon:                 "database",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/postgresql.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"postgresql","version":"12.12.10","description":"PostgreSQL - 关系型数据库","permissions":{"scope":"tool-only","rules":[]}}`,
			WorkloadRolePolicy:   `[]`,
			Enabled:              true,
		},
		// MySQL
		{
			Type:                 "mysql",
			Name:                 "MySQL (Bitnami)",
			Category:             "infra",
			Description:          "MySQL - 关系型数据库",
			Icon:                 "database",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/mysql.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"mysql","version":"9.12.0","description":"MySQL - 关系型数据库","permissions":{"scope":"tool-only","rules":[]}}`,
			WorkloadRolePolicy:   `[]`,
			Enabled:              true,
		},
		// MongoDB
		{
			Type:                 "mongodb",
			Name:                 "MongoDB (Bitnami)",
			Category:             "infra",
			Description:          "MongoDB - 文档型数据库",
			Icon:                 "database",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/mongodb.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"mongodb","version":"14.0.0","description":"MongoDB - 文档型数据库","permissions":{"scope":"tool-only","rules":[]}}`,
			WorkloadRolePolicy:   `[]`,
			Enabled:              true,
		},
		// Redis
		{
			Type:                 "redis",
			Name:                 "Redis (Bitnami)",
			Category:             "infra",
			Description:          "Redis - 缓存服务",
			Icon:                 "cloud",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/redis.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"redis","version":"18.6.0","description":"Redis - 缓存服务","permissions":{"scope":"tool-only","rules":[]}}`,
			WorkloadRolePolicy:   `[]`,
			Enabled:              true,
		},
		// RabbitMQ
		{
			Type:                 "rabbitmq",
			Name:                 "RabbitMQ (Bitnami)",
			Category:             "infra",
			Description:          "RabbitMQ - 消息队列",
			Icon:                 "network",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/rabbitmq.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"rabbitmq","version":"12.0.0","description":"RabbitMQ - 消息队列","permissions":{"scope":"tool-only","rules":[]}}`,
			WorkloadRolePolicy:   `[]`,
			Enabled:              true,
		},
		// Kafka
		{
			Type:                 "kafka",
			Name:                 "Kafka (Bitnami)",
			Category:             "infra",
			Description:          "Apache Kafka - 流处理平台",
			Icon:                 "network",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/kafka.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"kafka","version":"26.0.0","description":"Apache Kafka - 流处理平台","permissions":{"scope":"tool-only","rules":[]}}`,
			WorkloadRolePolicy:   `[]`,
			Enabled:              true,
		},
		// MinIO
		{
			Type:                 "minio",
			Name:                 "MinIO (Bitnami)",
			Category:             "infra",
			Description:          "MinIO - 对象存储",
			Icon:                 "data-base",
			Installer:            "helm",
			S3Bucket:             "paap-charts",
			S3Key:                "charts/minio.tar.gz",
			IsCustom:             false,
			PlatformManifestJSON: `{"name":"minio","version":"13.0.0","description":"MinIO - 对象存储","permissions":{"scope":"tool-only","rules":[]}}`,
			WorkloadRolePolicy:   `[]`,
			Enabled:              true,
		},
	}

	for _, t := range templates {
		database.DB.Create(&t)
	}

	// Upload built-in chart packages to S3
	SeedBuiltinChartsToS3()
}

// SeedBuiltinChartsToS3 uploads built-in chart packages to S3 if they don't exist.
// This ensures all built-in templates are available in S3 for installation.
func SeedBuiltinChartsToS3() {
	log.Printf("[SeedBuiltinChartsToS3] Starting seed process...")

	s3, err := getOrCreateS3Client()
	if err != nil {
		log.Printf("[SeedBuiltinChartsToS3] S3 client not available, skipping: %v", err)
		return
	}

	log.Printf("[SeedBuiltinChartsToS3] S3 client created successfully")

	chartTypes := []string{
		"argocd", "jenkins", "monitor", "loki", "harbor",
		"postgresql", "mysql", "mongodb", "redis", "rabbitmq", "kafka", "minio",
	}

	ctx := context.Background()
	for _, ct := range chartTypes {
		s3Key := fmt.Sprintf("charts/%s.tar.gz", ct)
		if s3.ObjectExists(ctx, s3Key) {
			log.Printf("[SeedBuiltinChartsToS3] %s already exists in S3, skipping", s3Key)
			continue
		}

		localPath := fmt.Sprintf("/charts/%s.tar.gz", ct)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			log.Printf("[SeedBuiltinChartsToS3] local file not found: %s, skipping", localPath)
			continue
		}

		if err := s3.UploadFile(ctx, s3Key, localPath, "application/gzip"); err != nil {
			log.Printf("[SeedBuiltinChartsToS3] failed to upload %s: %v", ct, err)
		} else {
			log.Printf("[SeedBuiltinChartsToS3] uploaded %s to S3", ct)
		}
	}

	log.Printf("[SeedBuiltinChartsToS3] Seed process completed")
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

	// Check if type already exists
	var existing model.ServiceTemplate
	if err := database.DB.Where("type = ?", typ).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("template type '%s' already exists", typ)})
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

	// Store chart in S3
	s3, err := getOrCreateS3Client()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("S3 client error: %v", err)})
		return
	}
	s3Key := fmt.Sprintf("charts/%s.tar.gz", typ)
	if err := s3.UploadFile(context.Background(), s3Key, tmpFile.Name(), "application/gzip"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("S3 upload error: %v", err)})
		return
	}

	manifestJSON, _ := json.Marshal(manifest)
	if category == "" {
		category = "tool"
	}

	tmpl := model.ServiceTemplate{
		Type:                 typ,
		Name:                 name,
		Category:             category,
		Description:          description,
		Icon:                 "puzzle",
		Installer:            "helm",
		WorkloadRolePolicy:   manifest.ToWorkloadRoleJSON(),
		IsCustom:             true,
		PlatformManifestJSON: string(manifestJSON),
		S3Bucket:             s3BucketName,
		S3Key:                s3Key,
		PresetValues:         presetValues,
		Enabled:              true,
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
// It extracts the manifest, preset-values, dashboard JSONs, and uses helm template to
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

	// Render chart with helm template and scan output for forbidden kinds
	chartPath := filepath.Join(tmpDir, "chart")
	args := []string{"template", "validate-release", chartPath, "--namespace", "paap-validate"}
	if presetValues != "" {
		presetPath := filepath.Join(tmpDir, "preset-values.yaml")
		args = append(args, "-f", presetPath)
	}

	cmd := exec.Command("helm", args...)
	rendered, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", nil, nil, fmt.Errorf("helm template failed: %s", string(rendered))
	}

	// Scan rendered output for ClusterRole or ClusterRoleBinding
	renderedStr := string(rendered)
	for _, line := range strings.Split(renderedStr, "\n") {
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
