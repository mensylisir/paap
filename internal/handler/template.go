package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
)

func toJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
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

// DeleteServiceTemplate deletes a service template
func DeleteServiceTemplate(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := database.DB.Delete(&model.ServiceTemplate{}, id).Error; err != nil {
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

// SeedServiceCatalog creates default service types
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
}
