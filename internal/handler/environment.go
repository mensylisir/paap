package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

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
	ctx := context.Background()

	var tmpl model.EnvTemplate
	if err := database.DB.First(&tmpl, templateID).Error; err != nil {
		database.DB.Model(env).Update("status", "error").Update("error_message", "template not found")
		return
	}

	// Install tools (创建 ServiceInstance CR)
	var services []string
	if err := json.Unmarshal([]byte(tmpl.Services), &services); err != nil {
		services = []string{}
	}

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

		// 渲染模板并创建 ConfigMap
		manifestsRef := renderAndStoreManifests(ctx, crNS, svc, app, env, envIdentifier, toolNS, nil)

		workloadRole := paapv1.RoleSpec{
			Rules: []paapv1.PolicyRule{
				{
					APIGroups: []string{"", "apps", "batch", "networking.k8s.io", "autoscaling"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
		}

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

	// 发现负载 namespace
	primaryNS := app.Identifier + "-" + envIdentifier
	namespaces := []string{primaryNS}

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

	// 创建 ServiceInstance CR
	ctx := context.Background()
	crNS := fmt.Sprintf("paap-app-%s", app.Identifier)
	manifestsRef := renderAndStoreManifests(ctx, crNS, req.ServiceType, &app, &env, env.Identifier, toolNS, nil)

	workloadRole := paapv1.RoleSpec{
		Rules: []paapv1.PolicyRule{
			{
				APIGroups: []string{"", "apps", "batch", "networking.k8s.io", "autoscaling"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		},
	}

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
