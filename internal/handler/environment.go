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

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	paapv1 "paap/api/v1"
	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/model"
	"paap/internal/service"
)

var k8sClient = k8s.NewClient()

type CreateEnvRequest struct {
	Name       string `json:"name" binding:"required"`
	Identifier string `json:"identifier" binding:"required"`
	TemplateID uint   `json:"templateId"`
	FromEmpty  bool   `json:"fromEmpty"`
}

type CreateComponentRequest struct {
	Name     string `json:"name" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Image    string `json:"image"`
	Version  string `json:"version"`
	Replicas int    `json:"replicas"`
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
}

type InstallServiceRequest struct {
	ServiceType string `json:"serviceType" binding:"required"`
}

// getWorkloadRole returns the RBAC whitelist for a given service type,
// read from the ServiceTemplate's WorkloadRolePolicy field.
// Falls back to a safe minimal read-only role if the template is not found or has no policy.
func getWorkloadRole(svcType string) paapv1.RoleSpec {
	var tmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", svcType).First(&tmpl).Error; err != nil {
		return defaultSafeRole()
	}
	if tmpl.WorkloadRolePolicy == "" {
		return defaultSafeRole()
	}
	var rules []paapv1.PolicyRule
	if err := json.Unmarshal([]byte(tmpl.WorkloadRolePolicy), &rules); err != nil {
		return defaultSafeRole()
	}
	return paapv1.RoleSpec{Rules: rules}
}

// defaultSafeRole returns a minimal read-only role as a safe fallback.
func defaultSafeRole() paapv1.RoleSpec {
	return paapv1.RoleSpec{
		Rules: []paapv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "endpoints"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

// mergeDefaultValues parses the template's DefaultValues JSON and merges with user overrides.
func mergeDefaultValues(defaultsJSON string, overrides map[string]string) map[string]string {
	result := make(map[string]string)
	if defaultsJSON != "" {
		json.Unmarshal([]byte(defaultsJSON), &result)
	}
	for k, v := range overrides {
		result[k] = v
	}
	return result
}

// buildContextValues builds platform context variables that are injected into
// every custom template's Helm values. Users can reference these in their charts
// via {{ .Values.global.envNamespaces }} etc.
func buildContextValues(envNS string, allNamespaces []string, envIdentifier string) map[string]string {
	return map[string]string{
		"global.platformManaged": "true",
		"global.environmentId":   envIdentifier,
		"global.envNamespaces":   strings.Join(allNamespaces, ","),
		"global.primaryNamespace": envNS,
	}
}

// installCustomChart extracts a custom template's chart archive and installs it via Helm.
// Values merge order (later wins):
//  1. preset-values.yaml (disable built-in RBAC, set SA name, etc.)
//  2. platform context (global.envNamespaces, global.environmentId, etc.)
//  3. variable_mapping from platform-manifest (custom mappings)
//  4. user parameters (highest priority, not yet implemented)
func installCustomChart(releaseName, toolNS, chartArchivePath, s3Bucket, s3Key, platformManifestJSON, presetValues string, envIdentifier, primaryNS string, allNamespaces []string, extraValues map[string]string) error {
	tmpDir, err := os.MkdirTemp("", "paap-chart-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var localChartPath string
	if s3Bucket != "" && s3Key != "" {
		// Download from S3
		s3Client, err := getOrCreateS3Client()
		if err != nil {
			return fmt.Errorf("failed to create S3 client: %w", err)
		}
		localPath := filepath.Join(tmpDir, "chart.tar.gz")
		if err := s3Client.DownloadFile(context.Background(), s3Key, localPath); err != nil {
			return fmt.Errorf("failed to download chart from S3: %w", err)
		}
		localChartPath = localPath
	} else {
		// Use local file
		localChartPath = chartArchivePath
	}

	if err := extractTarGz(localChartPath, tmpDir); err != nil {
		return fmt.Errorf("failed to extract chart: %w", err)
	}

	chartPath := filepath.Join(tmpDir, "chart")

	// Layer 1: Start with preset-values (e.g. rbac.create=false)
	values := make(map[string]string)

	// First try to read preset-values.yaml from the extracted chart
	presetValuesPath := filepath.Join(tmpDir, "preset-values.yaml")
	if presetValuesData, err := os.ReadFile(presetValuesPath); err == nil {
		log.Printf("[installCustomChart] Read preset-values.yaml: %s", string(presetValuesData))
		values = parseYAMLToMap(string(presetValuesData))
		log.Printf("[installCustomChart] Parsed values: %+v", values)
	} else {
		log.Printf("[installCustomChart] preset-values.yaml not found: %v", err)
		if presetValues != "" {
			// Fallback to database field if file doesn't exist
			values = parseYAMLToMap(presetValues)
		}
	}

	// Layer 2: Add platform context
	for k, v := range buildContextValues(primaryNS, allNamespaces, envIdentifier) {
		values[k] = v
	}

	// Layer 3: Apply variable_mapping from platform-manifest
	if platformManifestJSON != "" {
		var manifest model.PlatformManifest
		if err := json.Unmarshal([]byte(platformManifestJSON), &manifest); err == nil {
			platformVars := map[string]string{
				"current_env_name":  envIdentifier,
				"primary_namespace": primaryNS,
				"all_namespaces":    strings.Join(allNamespaces, ","),
				"tool_namespace":    toolNS,
			}
			for k, v := range manifest.BuildHelmValues(platformVars) {
				values[k] = v
			}
		}
	}

	// Layer 4: Apply extra values (e.g. skip CRDs for ArgoCD)
	for k, v := range extraValues {
		values[k] = v
	}

	if err := k8sClient.InstallHelmChart(releaseName, toolNS, chartPath, values); err != nil {
		return err
	}

	// Provision Grafana dashboards asynchronously
	dashboardsDir := filepath.Join(tmpDir, "dashboards")
	go provisionDashboards(toolNS, dashboardsDir)

	return nil
}

// parseYAMLToMap parses a simple flat YAML string into a map[string]string.
// Only handles top-level key: value pairs (not nested structures).
// Nested keys use dot notation: "rbac.create: false" → {"rbac.create": "false"}
func parseYAMLToMap(yamlContent string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(yamlContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Remove quotes
		val = strings.Trim(val, `"'`)
		if key != "" && val != "" {
			result[key] = val
		}
	}
	return result
}

// provisionDashboards waits for Grafana to be ready and imports all dashboard JSON files.
// Runs as a goroutine with retry logic since Grafana may take time to start.
func provisionDashboards(grafanaNS, dashboardsDir string) {
	if _, err := os.Stat(dashboardsDir); os.IsNotExist(err) {
		return // No dashboards directory
	}

	entries, err := os.ReadDir(dashboardsDir)
	if err != nil || len(entries) == 0 {
		return
	}

	grafana := k8s.NewGrafanaClient(grafanaNS)

	// Wait up to 2 minutes for Grafana to be ready
	for i := 0; i < 24; i++ {
		if err := grafana.HealthCheck(); err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dashboardsDir, entry.Name()))
		if err != nil {
			log.Printf("[provisionDashboards] failed to read %s: %v", entry.Name(), err)
			continue
		}
		if err := grafana.ImportDashboard(string(content), entry.Name()); err != nil {
			log.Printf("[provisionDashboards] failed to import dashboard %s: %v", entry.Name(), err)
		} else {
			log.Printf("[provisionDashboards] imported dashboard: %s", entry.Name())
		}
	}
}

// extractTarGz extracts a tar.gz archive to the target directory.
func extractTarGz(archivePath, targetDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(targetDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			io.Copy(outFile, tr)
			outFile.Close()
		}
	}
	return nil
}

// ListApplicationEnvironments returns environments for an application
func ListApplicationEnvironments(c *gin.Context) {
	appID, _ := strconv.Atoi(c.Param("id"))
	var envs []model.Environment
	if err := database.DB.Where("application_id = ?", appID).Find(&envs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": envs})
}

// CreateEnvironment creates a new environment for an application.
// If template is provided, auto-installs tools and infra. If fromEmpty, creates bare namespace.
func CreateEnvironment(c *gin.Context) {
	appID, _ := strconv.Atoi(c.Param("id"))
	var req CreateEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	env := model.Environment{
		ApplicationID: uint(appID),
		Name:            req.Name,
		Identifier:      req.Identifier,
		TemplateID:      req.TemplateID,
		Status:          "empty",
		Namespace:       req.Identifier,
	}

	if !req.FromEmpty {
		env.Status = "creating"
	}

	if err := database.DB.Create(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 查出应用信息
	var app model.Application
	if err := database.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	// 1) 创建 K8s Environment CR（Operator 会自动创建 namespace + NetworkPolicy + Quota）
	ctx := context.Background()
	primaryNS := app.Identifier + "-" + req.Identifier
	additionalNS := []paapv1.AdditionalNamespace{
		{Suffix: "app", Purpose: "workload"},
	}
	if err := k8s.CreateEnvironmentCR(ctx, app.Identifier, req.Name, req.Identifier, primaryNS, additionalNS); err != nil {
		database.DB.Model(&env).Update("status", "error").Update("error_message", err.Error())
		c.JSON(http.StatusCreated, gin.H{"data": env, "warning": "Environment CR creation failed: " + err.Error()})
		return
	}

	// 2) 如果从模板创建，安装工具（创建 ServiceInstance CR）
	if !req.FromEmpty && req.TemplateID > 0 {
		go installTemplateServices(&env, &app, req.Identifier, req.TemplateID)
		env.Status = "creating"
		database.DB.Model(&env).Update("status", "creating")
	} else {
		env.Status = "empty"
		database.DB.Model(&env).Update("status", "empty")
	}

	c.JSON(http.StatusCreated, gin.H{"data": env})
}

// installTemplateServices creates ServiceInstance CRs for template services
func installTemplateServices(env *model.Environment, app *model.Application, envIdentifier string, templateID uint) {
	log.Printf("[installTemplateServices] Starting for env=%s templateID=%d", envIdentifier, templateID)
	ctx := context.Background()

	var tmpl model.EnvTemplate
	if err := database.DB.First(&tmpl, templateID).Error; err != nil {
		log.Printf("[installTemplateServices] Template %d not found: %v", templateID, err)
		database.DB.Model(env).Update("status", "error").Update("error_message", "template not found")
		return
	}
	log.Printf("[installTemplateServices] Found template: name=%s services=%s", tmpl.Name, tmpl.Services)

	// Install tools (创建 ServiceInstance CR)
	var services []string
	if err := json.Unmarshal([]byte(tmpl.Services), &services); err != nil {
		log.Printf("[installTemplateServices] Failed to unmarshal services: %v", err)
		services = []string{}
	}
	log.Printf("[installTemplateServices] Will install %d services: %v", len(services), services)

	crNS := fmt.Sprintf("paap-app-%s", app.Identifier)

	for _, svc := range services {
		toolNS := fmt.Sprintf("%s-%s-%s", app.Identifier, envIdentifier, svc)
		inst := model.ServiceInstallation{
			EnvironmentID: env.ID,
			ServiceType:   svc,
			Status:        "installing",
			Namespace:     toolNS,
		}
		database.DB.Create(&inst)

		// ArgoCD 需要预装 CRD
		if svc == "deploy" {
			if err := k8sClient.EnsureArgoCDCRDs(); err != nil {
				log.Printf("[installTemplateServices] warning: failed to ensure ArgoCD CRDs: %v", err)
			}
		}

		// 查找模板获取安装方式
		var svcTmpl model.ServiceTemplate
		database.DB.Where("type = ?", svc).First(&svcTmpl)
		log.Printf("[installTemplateServices] Service %s: installer=%s s3Bucket=%s s3Key=%s chartRepo=%s",
			svc, svcTmpl.Installer, svcTmpl.S3Bucket, svcTmpl.S3Key, svcTmpl.ChartRepo)

		var manifestsRef *paapv1.ConfigMapReference

		if svcTmpl.Installer == "helm" {
			releaseName := fmt.Sprintf("%s-%s-%s", app.Identifier, envIdentifier, svc)

			if svcTmpl.S3Bucket != "" || svcTmpl.ChartArchivePath != "" {
				// S3 或本地文件路径
				primaryNS := app.Identifier + "-" + envIdentifier
				allNS := []string{primaryNS, primaryNS + "-app"}
				log.Printf("[installTemplateServices] Installing via S3: release=%s ns=%s s3Key=%s", releaseName, toolNS, svcTmpl.S3Key)
				// ArgoCD 需要跳过 CRD 安装（CRDs 由 EnsureArgoCDCRDs 预装）
				extraValues := map[string]string{}
				if svc == "deploy" {
					extraValues["crds.install"] = "false"
				}
				if err := installCustomChart(releaseName, toolNS, svcTmpl.ChartArchivePath, svcTmpl.S3Bucket, svcTmpl.S3Key, svcTmpl.PlatformManifestJSON, svcTmpl.PresetValues, envIdentifier, primaryNS, allNS, extraValues); err != nil {
					log.Printf("[installTemplateServices] chart install FAILED for %s: %v", svc, err)
					inst.Status = "failed"
					inst.ErrorMessage = err.Error()
					database.DB.Save(&inst)
				} else {
					log.Printf("[installTemplateServices] chart install SUCCEEDED for %s", svc)
				}
			} else {
				// 远程 Helm ChartRepo
				values := mergeDefaultValues(svcTmpl.DefaultValues, nil)
				log.Printf("[installTemplateServices] Installing via Helm repo: release=%s chart=%s/%s:%s", releaseName, svcTmpl.ChartRepo, svcTmpl.ChartName, svcTmpl.ChartVersion)
				if err := k8sClient.InstallToolViaHelm(releaseName, toolNS, svcTmpl.ChartRepo, svcTmpl.ChartName, svcTmpl.ChartVersion, values); err != nil {
					log.Printf("[installTemplateServices] helm install FAILED for %s: %v", svc, err)
					inst.Status = "failed"
					inst.ErrorMessage = err.Error()
					database.DB.Save(&inst)
				} else {
					log.Printf("[installTemplateServices] helm install SUCCEEDED for %s", svc)
				}
			}
		} else {
			// Raw YAML 安装路径
			log.Printf("[installTemplateServices] Installing via raw YAML: %s", svc)
			manifestsRef = renderAndStoreManifests(ctx, crNS, svc, app, env, envIdentifier, toolNS, nil)
		}

		workloadRole := getWorkloadRole(svc)

		if err := k8s.CreateServiceInstanceCR(ctx, app.Identifier, envIdentifier, svc, workloadRole, manifestsRef); err != nil {
			inst.Status = "failed"
			inst.ErrorMessage = err.Error()
			database.DB.Save(&inst)
		} else {
			inst.Status = "running"
			database.DB.Save(&inst)
		}
	}

	database.DB.Model(env).Update("status", "running")
}

// renderAndStoreManifests renders a service template and stores it in a ConfigMap
func renderAndStoreManifests(ctx context.Context, crNS, svcType string, app *model.Application, env *model.Environment, envIdentifier, toolNS string, parameters map[string]string) *paapv1.ConfigMapReference {
	// 查找 ServiceTemplate
	var svcTmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", svcType).First(&svcTmpl).Error; err != nil {
		log.Printf("[renderAndStoreManifests] template not found for type %s: %v", svcType, err)
		return nil
	}

	if svcTmpl.RawYamlTemplate == "" {
		log.Printf("[renderAndStoreManifests] template %s has no rawYamlTemplate", svcType)
		return nil
	}

	// 合并默认参数
	defaultParams := make(map[string]string)
	if svcTmpl.DefaultValues != "" {
		json.Unmarshal([]byte(svcTmpl.DefaultValues), &defaultParams)
	}
	for k, v := range parameters {
		defaultParams[k] = v
	}

	// 构建 namespace 列表（主空间 + 附加空间）
	primaryNS := app.Identifier + "-" + envIdentifier
	namespaces := []string{primaryNS}
	// 默认附加 namespace: {primaryNS}-app (工作负载空间)
	additionalNS := primaryNS + "-app"
	namespaces = append(namespaces, additionalNS)

	// 渲染模板
	renderer := service.NewTemplateRenderer()
	vars := service.BuildVariables(
		app.ID, app.Name, app.Identifier,
		env.ID, env.Name, envIdentifier,
		primaryNS, toolNS, namespaces,
		fmt.Sprintf("%s-%s-%s", app.Identifier, envIdentifier, svcType),
		toolNS,
		defaultParams,
		service.RoleRules{},
	)

	rendered, err := renderer.RenderServiceTemplate(svcTmpl.RawYamlTemplate, vars)
	if err != nil {
		log.Printf("[renderAndStoreManifests] render error for %s: %v", svcType, err)
		return nil
	}

	log.Printf("[renderAndStoreManifests] rendered %d bytes for %s", len(rendered), svcType)

	// 创建 ConfigMap 存储渲染后的 manifests
	cmName := fmt.Sprintf("%s-%s-manifests", envIdentifier, svcType)
	cmLabels := map[string]string{
		"paap.io/app":  app.Identifier,
		"paap.io/env":  envIdentifier,
		"paap.io/tool": svcType,
		"paap.io/type": "manifests",
	}
	if err := k8s.CreateConfigMap(ctx, crNS, cmName, map[string]string{"manifests.yaml": rendered}, cmLabels); err != nil {
		log.Printf("[renderAndStoreManifests] create configmap error: %v", err)
		return nil
	}

	log.Printf("[renderAndStoreManifests] created ConfigMap %s/%s", crNS, cmName)
	return &paapv1.ConfigMapReference{
		Name:      cmName,
		Namespace: crNS,
	}
}

// GetEnvironment returns environment details with components, services, infra
func GetEnvironment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var env model.Environment
	if err := database.DB.First(&env, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var components []model.Component
	database.DB.Where("environment_id = ?", env.ID).Find(&components)
	var services []model.ServiceInstallation
	database.DB.Where("environment_id = ?", env.ID).Find(&services)
	var infra []model.InfraInstallation
	database.DB.Where("environment_id = ?", env.ID).Find(&infra)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"environment": env,
			"components":  components,
			"services":    services,
			"infra":       infra,
		},
	})
}

// DeleteEnvironment deletes an environment and its K8s CR
func DeleteEnvironment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var env model.Environment
	if err := database.DB.First(&env, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	// 查出应用信息
	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err == nil {
		// 删除 K8s Environment CR（Operator 会级联删除 namespace + ServiceInstance CR）
		ctx := context.Background()
		k8s.DeleteEnvironmentCR(ctx, app.Identifier, env.Identifier)
	}

	// 删除数据库记录（硬删除，避免唯一约束冲突）
	database.DB.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.ServiceInstallation{})
	database.DB.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.InfraInstallation{})
	database.DB.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.Component{})
	database.DB.Unscoped().Delete(&env)

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListEnvironmentComponents returns components in an environment
func ListEnvironmentComponents(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	var components []model.Component
	if err := database.DB.Where("environment_id = ?", envID).Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": components})
}

// CreateComponent creates a component via Component CR
func CreateComponent(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var req CreateComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comp := model.Component{
		EnvironmentID: uint(envID),
		Name:          req.Name,
		Type:          req.Type,
		Image:         req.Image,
		Version:       req.Version,
		Replicas:      req.Replicas,
		CPU:           req.CPU,
		Memory:        req.Memory,
		Status:        "creating",
	}

	if comp.Replicas == 0 {
		comp.Replicas = 1
	}
	if comp.Version == "" {
		comp.Version = "latest"
	}
	if comp.Image == "" {
		comp.Image = fmt.Sprintf("nginx:%s", comp.Version)
	}

	if err := database.DB.Create(&comp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 创建 Component CR（Operator 会创建 Deployment + Service）
	ctx := context.Background()
	primaryNS := app.Identifier + "-" + env.Identifier
	replicas := int32(comp.Replicas)
	// 用组件名称的小写英文版本作为标识（去掉中文，用 type 替代）
	compIdentifier := comp.Name
	if compIdentifier == "" || compIdentifier[0] < 'a' || compIdentifier[0] > 'z' {
		compIdentifier = comp.Type + "-" + fmt.Sprintf("%d", comp.ID)
	}

	if err := k8s.CreateComponentCR(ctx, app.Identifier, env.Identifier, comp.Name, compIdentifier, comp.Type, comp.Image, comp.Version, replicas, primaryNS); err != nil {
		comp.Status = "error"
		comp.ErrorMessage = err.Error()
		database.DB.Save(&comp)
		c.JSON(http.StatusCreated, gin.H{"data": comp, "warning": "Component CR creation failed: " + err.Error()})
		return
	}

	comp.Status = "running"
	database.DB.Save(&comp)
	c.JSON(http.StatusCreated, gin.H{"data": comp})
}

// DeleteComponent deletes a component and its K8s CR
func DeleteComponent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var comp model.Component
	if err := database.DB.First(&comp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
		return
	}

	var env model.Environment
	var app model.Application
	if err := database.DB.First(&env, comp.EnvironmentID).Error; err == nil {
		if err := database.DB.First(&app, env.ApplicationID).Error; err == nil {
			// 生成与创建时相同的 identifier
			compIdentifier := comp.Name
			if compIdentifier == "" || compIdentifier[0] < 'a' || compIdentifier[0] > 'z' {
				compIdentifier = comp.Type + "-" + fmt.Sprintf("%d", comp.ID)
			}
			ctx := context.Background()
			k8s.DeleteComponentCR(ctx, app.Identifier, env.Identifier, compIdentifier)
		}
	}

	if err := database.DB.Delete(&comp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListServiceInstances returns service installations for an environment
func ListServiceInstances(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	var services []model.ServiceInstallation
	if err := database.DB.Where("environment_id = ?", envID).Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": services})
}

// InstallService installs a service (tool) in an environment via CR
func InstallService(c *gin.Context) {
	appID, _ := strconv.Atoi(c.Param("id"))

	var env model.Environment
	if err := database.DB.Where("application_id = ?", appID).First(&env).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	var app model.Application
	if err := database.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var req InstallServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	toolNS := fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, req.ServiceType)
	inst := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   req.ServiceType,
		Status:        "installing",
		Namespace:     toolNS,
	}
	database.DB.Create(&inst)

	// ArgoCD 需要预装 CRD
	if req.ServiceType == "deploy" {
		if err := k8sClient.EnsureArgoCDCRDs(); err != nil {
			log.Printf("[InstallService] warning: failed to ensure ArgoCD CRDs: %v", err)
		}
	}

	// 查找模板获取安装方式
	var svcTmpl model.ServiceTemplate
	database.DB.Where("type = ?", req.ServiceType).First(&svcTmpl)

	ctx := context.Background()
	var manifestsRef *paapv1.ConfigMapReference

	if svcTmpl.Installer == "helm" {
		releaseName := fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, req.ServiceType)

		if svcTmpl.S3Bucket != "" || svcTmpl.ChartArchivePath != "" {
			// 自定义模板：从 S3 或本地 chart archive 安装
			primaryNS := app.Identifier + "-" + env.Identifier
			allNS := []string{primaryNS, primaryNS + "-app"}
			extraValues := map[string]string{}
			if req.ServiceType == "deploy" {
				extraValues["crds.install"] = "false"
			}
			if err := installCustomChart(releaseName, toolNS, svcTmpl.ChartArchivePath, svcTmpl.S3Bucket, svcTmpl.S3Key, svcTmpl.PlatformManifestJSON, svcTmpl.PresetValues, env.Identifier, primaryNS, allNS, extraValues); err != nil {
				log.Printf("[InstallService] custom chart install failed for %s: %v", req.ServiceType, err)
				inst.Status = "failed"
				inst.ErrorMessage = err.Error()
				database.DB.Save(&inst)
			}
		} else {
			// 内置 Helm 模板：从远程 repo 安装
			values := mergeDefaultValues(svcTmpl.DefaultValues, nil)
			if err := k8sClient.InstallToolViaHelm(releaseName, toolNS, svcTmpl.ChartRepo, svcTmpl.ChartName, svcTmpl.ChartVersion, values); err != nil {
				log.Printf("[InstallService] helm install failed for %s: %v", req.ServiceType, err)
				inst.Status = "failed"
				inst.ErrorMessage = err.Error()
				database.DB.Save(&inst)
			}
		}
		// manifestsRef 留空，Operator 不会尝试 apply YAML
	} else {
		// Raw YAML 安装路径：渲染模板 → ConfigMap → Operator apply
		crNS := fmt.Sprintf("paap-app-%s", app.Identifier)
		manifestsRef = renderAndStoreManifests(ctx, crNS, req.ServiceType, &app, &env, env.Identifier, toolNS, nil)
	}

	workloadRole := getWorkloadRole(req.ServiceType)

	if err := k8s.CreateServiceInstanceCR(ctx, app.Identifier, env.Identifier, req.ServiceType, workloadRole, manifestsRef); err != nil {
		inst.Status = "failed"
		inst.ErrorMessage = err.Error()
		database.DB.Save(&inst)
		c.JSON(http.StatusCreated, gin.H{"data": inst, "warning": "ServiceInstance CR creation failed: " + err.Error()})
		return
	}

	inst.Status = "running"
	database.DB.Save(&inst)
	c.JSON(http.StatusCreated, gin.H{"data": inst})
}

// InstallInfra installs infrastructure in an environment
func InstallInfra(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		InfraType string            `json:"infraType" binding:"required"`
		Config    map[string]string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	inst := model.InfraInstallation{
		EnvironmentID: env.ID,
		InfraType:     req.InfraType,
		Status:        "installing",
	}
	database.DB.Create(&inst)

	go func() {
		var err error
		switch req.InfraType {
		case "postgresql":
			err = k8sClient.InstallPostgreSQL(env.Namespace, env.Name+"-db")
		case "redis":
			err = k8sClient.InstallRedis(env.Namespace, env.Name+"-redis")
		case "rabbitmq":
			err = k8sClient.InstallRabbitMQ(env.Namespace, env.Name+"-mq")
		case "minio":
			err = k8sClient.InstallMinIO(env.Namespace, env.Name+"-minio")
		case "nacos":
			err = k8sClient.InstallNacos(env.Namespace, env.Name+"-nacos")
		default:
			err = fmt.Errorf("unknown infra type: %s", req.InfraType)
		}

		if err != nil {
			inst.Status = "failed"
			inst.ErrorMessage = err.Error()
		} else {
			inst.Status = "running"
		}
		database.DB.Save(&inst)
	}()

	c.JSON(http.StatusCreated, gin.H{"data": inst})
}

// UninstallService removes a service installation from an environment
func UninstallService(c *gin.Context) {
	appID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var inst model.ServiceInstallation
	if err := database.DB.First(&inst, serviceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service installation not found"})
		return
	}

	var app model.Application
	if err := database.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var env model.Environment
	if err := database.DB.Where("application_id = ? AND id = ?", appID, inst.EnvironmentID).First(&env).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	var svcTmpl model.ServiceTemplate
	database.DB.Where("type = ?", inst.ServiceType).First(&svcTmpl)

	ctx := context.Background()

	// Uninstall based on installer type
	if svcTmpl.Installer == "helm" {
		releaseName := fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, inst.ServiceType)
		if err := k8sClient.DeleteHelmRelease(releaseName, inst.Namespace); err != nil {
			log.Printf("[UninstallService] helm uninstall warning: %v", err)
		}
	} else {
		// Raw YAML: delete ServiceInstance CR, Operator will clean up
		if err := k8s.DeleteServiceInstanceCR(ctx, app.Identifier, env.Identifier, inst.ServiceType); err != nil {
			log.Printf("[UninstallService] CR delete warning: %v", err)
		}
	}

	// Delete the namespace
	if err := k8sClient.DeleteNamespace(inst.Namespace); err != nil {
		log.Printf("[UninstallService] namespace delete warning: %v", err)
	}

	// Delete from database
	database.DB.Delete(&inst)

	c.JSON(http.StatusOK, gin.H{"message": "service uninstalled successfully"})
}
