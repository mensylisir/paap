package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"paap/internal/k8s"
	"paap/internal/model"

	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrInvalidComponentRequest = errors.New("invalid component request")
	ErrComponentDeliveryFailed = errors.New("component delivery failed")
	ErrComponentCRUpsertFailed = errors.New("component CR upsert failed")
)

var componentSourceDeliveryWatchers sync.Map

type CreateComponentInput struct {
	Name           string
	Type           string
	Image          string
	Version        string
	Replicas       int
	CPU            string
	Memory         string
	DeliveryMode   string
	DraftOnly      bool
	SourceRepoURL  string
	SourceBranch   string
	BuildContext   string
	BuildModule    string
	DockerfilePath string
	Config         *model.ComponentConfig
}

type UpdateComponentInput struct {
	Name           string
	Type           string
	Image          string
	Version        string
	Replicas       int
	CPU            string
	Memory         string
	DeliveryMode   string
	SourceRepoURL  string
	SourceBranch   string
	BuildContext   string
	BuildModule    string
	DockerfilePath string
	Config         *model.ComponentConfig
}

type componentDeploymentContext struct {
	App              model.Application
	Env              model.Environment
	Component        model.Component
	Identifier       string
	PrimaryNamespace string
	Config           model.ComponentConfig
}

type componentDeliveryTarget struct {
	Capability       string
	CapabilityID     uint
	Source           string
	ServiceType      string
	Service          *model.ServiceInstallation
	App              model.Application
	Env              model.Environment
	ExternalEndpoint string
	ValidationStatus string
}

func CreateComponent(db *gorm.DB, envID uint, input CreateComponentInput) (model.Component, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return model.Component{}, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return model.Component{}, err
	}
	if model.IsSystemSharedEnvironment(app, env) {
		return model.Component{}, ComponentValidationError{Message: "system shared environments only support installing tools and middleware"}
	}
	if err := validateComponentDeliveryInput(input); err != nil {
		return model.Component{}, err
	}

	version := strings.TrimSpace(input.Version)
	if version == "" {
		version = imageReferenceTag(input.Image)
	}
	comp := model.Component{
		EnvironmentID: env.ID,
		Name:          input.Name,
		Type:          input.Type,
		Image:         input.Image,
		Version:       version,
		Replicas:      input.Replicas,
		CPU:           input.CPU,
		Memory:        input.Memory,
		Status:        "draft",
		DeliveryMode:  componentDeliveryMode(input.DeliveryMode),
	}
	if comp.Replicas == 0 {
		comp.Replicas = 1
	}
	if comp.DeliveryMode == "source" {
		comp.SourceRepoURL = strings.TrimSpace(input.SourceRepoURL)
		comp.SourceBranch = valueOrDefaultString(input.SourceBranch, "main")
		comp.BuildContext = valueOrDefaultString(input.BuildContext, ".")
		comp.BuildModule = strings.TrimSpace(input.BuildModule)
		comp.DockerfilePath = strings.TrimSpace(input.DockerfilePath)
		comp.Image = ""
		comp.RegistryImage = ""
		comp.PipelineStatus = "draft"
	}
	if input.Config != nil {
		configJSON, err := input.Config.JSON()
		if err != nil {
			return model.Component{}, ComponentValidationError{Message: err.Error()}
		}
		comp.Config = configJSON
	}
	if err := db.Create(&comp).Error; err != nil {
		return model.Component{}, err
	}
	return comp, nil
}

func UpdateComponent(db *gorm.DB, componentID uint, input UpdateComponentInput) (model.Component, error) {
	var comp model.Component
	if err := db.First(&comp, componentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Component{}, ErrComponentNotFound
		}
		return model.Component{}, err
	}
	env, err := findEnvironment(db, comp.EnvironmentID)
	if err != nil {
		return model.Component{}, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return model.Component{}, err
	}

	if strings.TrimSpace(input.Name) != "" {
		comp.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.Type) != "" {
		comp.Type = strings.TrimSpace(input.Type)
	}
	if strings.TrimSpace(input.DeliveryMode) != "" {
		mode := strings.ToLower(strings.TrimSpace(input.DeliveryMode))
		switch mode {
		case "source":
			if strings.TrimSpace(input.SourceRepoURL) == "" && strings.TrimSpace(comp.SourceRepoURL) == "" {
				return model.Component{}, ComponentValidationError{Message: "source repository URL is required"}
			}
			comp.DeliveryMode = "source"
			if strings.TrimSpace(input.SourceRepoURL) != "" {
				comp.SourceRepoURL = strings.TrimSpace(input.SourceRepoURL)
			}
			comp.SourceBranch = valueOrDefaultString(input.SourceBranch, valueOrDefaultString(comp.SourceBranch, "main"))
			comp.BuildContext = valueOrDefaultString(input.BuildContext, valueOrDefaultString(comp.BuildContext, "."))
			comp.BuildModule = strings.TrimSpace(input.BuildModule)
			comp.DockerfilePath = strings.TrimSpace(input.DockerfilePath)
			identifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
			comp.JenkinsJob = fmt.Sprintf("%s-%s-%s-build", app.Identifier, env.Identifier, identifier)
			comp.Image = ""
			comp.RegistryImage = ""
			comp.PipelineStatus = "planned"
		case "image":
			comp.DeliveryMode = "image"
			comp.SourceRepoURL = ""
			comp.SourceMirrorRepoURL = ""
			comp.SourceBranch = ""
			comp.BuildContext = ""
			comp.BuildModule = ""
			comp.DockerfilePath = ""
			comp.JenkinsJob = ""
			if comp.PipelineStatus == "planned" || comp.PipelineStatus == "pending" {
				comp.PipelineStatus = ""
			}
		default:
			return model.Component{}, ComponentValidationError{Message: "deliveryMode must be image or source"}
		}
	}
	if strings.TrimSpace(input.Image) != "" {
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(input.Image)), ":latest") {
			return model.Component{}, ComponentValidationError{Message: "component image tag must be explicit; latest is not allowed"}
		}
		comp.Image = strings.TrimSpace(input.Image)
		if componentUsesDirectImageReference(comp) {
			comp.RegistryImage = comp.Image
		}
		if tag := imageReferenceTag(comp.Image); tag != "" {
			comp.Version = tag
		}
	}
	if strings.TrimSpace(input.Version) != "" {
		if strings.EqualFold(input.Version, "latest") {
			return model.Component{}, ComponentValidationError{Message: "component image tag must be explicit; latest is not allowed"}
		}
		comp.Version = strings.TrimSpace(input.Version)
		if componentUsesDirectImageReference(comp) {
			if strings.TrimSpace(comp.RegistryImage) != "" {
				comp.RegistryImage = ImageWithTag(comp.RegistryImage, comp.Version)
				comp.Image = comp.RegistryImage
			} else if strings.TrimSpace(comp.Image) != "" {
				comp.Image = ImageWithTag(comp.Image, comp.Version)
			}
		}
	}
	if input.Replicas > 0 {
		comp.Replicas = input.Replicas
	}
	if strings.TrimSpace(input.CPU) != "" {
		comp.CPU = strings.TrimSpace(input.CPU)
	}
	if strings.TrimSpace(input.Memory) != "" {
		comp.Memory = strings.TrimSpace(input.Memory)
	}
	if input.Config != nil {
		configJSON, err := input.Config.JSON()
		if err != nil {
			return model.Component{}, ComponentValidationError{Message: err.Error()}
		}
		comp.Config = configJSON
	}
	if comp.DeliveryMode == "source" && strings.TrimSpace(comp.Version) != "" {
		identifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
		cfg, _ := model.ParseComponentConfig(comp.Config)
		targets, _ := loadComponentDeliveryTargets(db, env.ID)
		registryTarget, _ := preferredComponentRegistryDeliveryTarget(targets, cfg.RegistryTarget)
		comp = prepareComponentSourceBuildVersionForTarget(app, env, comp, identifier, comp.Version, registryTarget)
	}
	if comp.DeliveryMode != "source" && strings.TrimSpace(comp.Version) != "" {
		identifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
		cfg, _ := model.ParseComponentConfig(comp.Config)
		targets, _ := loadComponentDeliveryTargets(db, env.ID)
		if registryTarget, ok := preferredComponentRegistryDeliveryTarget(targets, cfg.RegistryTarget); ok {
			comp = applyComponentImageDeployVersionForRuntimeRegistryTarget(app, env, comp, identifier, comp.Version, registryTarget)
		}
	}

	if err := db.Save(&comp).Error; err != nil {
		return model.Component{}, err
	}
	return comp, nil
}

func DeployComponent(ctx context.Context, db *gorm.DB, k8sClient client.Client, componentID uint, version string) (model.Component, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return model.Component{}, ComponentValidationError{Message: "version is required"}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	deployment, err := loadComponentDeploymentContext(db, componentID)
	if err != nil {
		return model.Component{}, err
	}
	if err := prepareComponentDeploymentVersion(db, &deployment, version); err != nil {
		return model.Component{}, err
	}
	if err := validateComponentDeliveryPreflight(ctx, db, k8sClient, deployment); err != nil {
		return model.Component{}, err
	}
	if err := detectAndApplyComponentRuntimeConfig(ctx, db, &deployment); err != nil {
		return model.Component{}, err
	}
	if err := saveComponentDeploymentDraft(db, deployment.Component); err != nil {
		return model.Component{}, err
	}

	result, err := runComponentDeliveryFlow(ctx, db, k8sClient, deployment.App, deployment.Env, deployment.Component, deployment.Identifier, deployment.PrimaryNamespace)
	if err != nil {
		return model.Component{}, errors.Join(ErrComponentDeliveryFailed, err)
	}
	deployment.Component = applyComponentGitOpsResult(deployment.Component, result)
	if componentDeploymentWaitsForSourceBuild(deployment.Component) {
		if err := db.Save(&deployment.Component).Error; err != nil {
			return model.Component{}, err
		}
		startComponentSourceDeliveryWatcher(db, k8sClient, deployment.Component.ID)
		return deployment.Component, nil
	}

	if err := upsertComponentRuntimeCR(ctx, deployment); err != nil {
		return model.Component{}, errors.Join(ErrComponentCRUpsertFailed, err)
	}
	deployment.Component.Status = "syncing"
	return deployment.Component, db.Save(&deployment.Component).Error
}

func startComponentSourceDeliveryWatcher(db *gorm.DB, k8sClient client.Client, componentID uint) {
	if db == nil || k8sClient == nil || componentID == 0 {
		return
	}
	if _, loaded := componentSourceDeliveryWatchers.LoadOrStore(componentID, struct{}{}); loaded {
		return
	}
	go func() {
		defer componentSourceDeliveryWatchers.Delete(componentID)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			done, err := completePendingComponentSourceDelivery(ctx, db, k8sClient, componentID)
			if err != nil {
				log.Printf("component source delivery watcher failed component=%d: %v", componentID, err)
			}
			if done {
				return
			}
			select {
			case <-ctx.Done():
				log.Printf("component source delivery watcher timed out component=%d", componentID)
				return
			case <-ticker.C:
			}
		}
	}()
}

func completePendingComponentSourceDelivery(ctx context.Context, db *gorm.DB, k8sClient client.Client, componentID uint) (bool, error) {
	deployment, err := loadComponentDeploymentContext(db, componentID)
	if err != nil {
		return true, err
	}
	comp := deployment.Component
	if comp.DeliveryMode != "source" {
		return true, nil
	}
	if strings.EqualFold(strings.TrimSpace(comp.PipelineStatus), "built") {
		return true, nil
	}
	targets, err := loadComponentDeliveryTargets(db, deployment.Env.ID)
	if err != nil {
		return false, err
	}
	flow := newComponentFlowContext(ctx, k8sClient, deployment.App, deployment.Env, comp, deployment.Identifier, deployment.PrimaryNamespace, targets)
	ready, warning, err := actionDetectSourceBuildCompletion(ctx, flow)
	if err != nil {
		return false, err
	}
	if !ready {
		if sourceBuildWarningIsFailed(warning) {
			comp.PipelineStatus = "failed"
			comp.Status = "error"
			comp.ErrorMessage = warning
			return true, db.Save(&comp).Error
		}
		if strings.TrimSpace(warning) != "" {
			comp.PipelineStatus = "running"
			comp.Status = "building"
			comp.ErrorMessage = warning
			if err := db.Save(&comp).Error; err != nil {
				return false, err
			}
		}
		return false, nil
	}
	comp.PipelineStatus = "built"
	if target, ok := preferredComponentRegistryDeliveryTarget(targets, deployment.Config.RegistryTarget); ok {
		comp = applyComponentDeployVersionForRuntimeRegistryTarget(deployment.App, deployment.Env, comp, deployment.Identifier, comp.Version, target)
	}
	result, err := RunComponentGitOpsDeploymentFlow(ctx, newComponentFlowContext(ctx, k8sClient, deployment.App, deployment.Env, comp, deployment.Identifier, deployment.PrimaryNamespace, targets))
	if err != nil {
		return false, err
	}
	comp = applyComponentGitOpsResult(comp, result)
	deployment.Component = comp
	if err := upsertComponentRuntimeCR(ctx, deployment); err != nil {
		return false, errors.Join(ErrComponentCRUpsertFailed, err)
	}
	return true, db.Save(&comp).Error
}

func loadComponentDeploymentContext(db *gorm.DB, componentID uint) (componentDeploymentContext, error) {
	var comp model.Component
	if err := db.First(&comp, componentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return componentDeploymentContext{}, ErrComponentNotFound
		}
		return componentDeploymentContext{}, err
	}
	env, err := findEnvironment(db, comp.EnvironmentID)
	if err != nil {
		return componentDeploymentContext{}, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return componentDeploymentContext{}, err
	}
	cfg, err := model.ParseComponentConfig(comp.Config)
	if err != nil {
		return componentDeploymentContext{}, ComponentValidationError{Message: "component config invalid: " + err.Error()}
	}
	identifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	return componentDeploymentContext{
		App:              app,
		Env:              env,
		Component:        comp,
		Identifier:       identifier,
		PrimaryNamespace: fmt.Sprintf("%s-%s", app.Identifier, env.Identifier),
		Config:           cfg,
	}, nil
}

func prepareComponentDeploymentVersion(db *gorm.DB, deployment *componentDeploymentContext, version string) error {
	comp := deployment.Component
	comp.Version = version
	if comp.DeliveryMode == "source" {
		targets, _ := loadComponentDeliveryTargets(db, deployment.Env.ID)
		registryTarget, _ := preferredComponentRegistryDeliveryTarget(targets, deployment.Config.RegistryTarget)
		if strings.EqualFold(strings.TrimSpace(comp.PipelineStatus), "built") {
			comp = applyComponentDeployVersionForRuntimeRegistryTarget(deployment.App, deployment.Env, comp, deployment.Identifier, version, registryTarget)
		} else {
			comp = prepareComponentSourceBuildVersionForTarget(deployment.App, deployment.Env, comp, deployment.Identifier, version, registryTarget)
		}
	} else {
		targets, _ := loadComponentDeliveryTargets(db, deployment.Env.ID)
		if registryTarget, ok := preferredComponentRegistryDeliveryTarget(targets, deployment.Config.RegistryTarget); ok {
			comp = applyComponentImageDeployVersionForRuntimeRegistryTarget(deployment.App, deployment.Env, comp, deployment.Identifier, version, registryTarget)
		} else {
			comp = applyComponentDeployVersion(comp, version)
		}
	}
	deployment.Component = comp
	return nil
}

func loadEnvironmentRegistryInstallations(db *gorm.DB, envID uint) []model.ServiceInstallation {
	var registryInstallations []model.ServiceInstallation
	_ = db.Where("environment_id = ? AND service_type IN ?", envID, []string{"registry", "harbor"}).
		Find(&registryInstallations).Error
	return registryInstallations
}

type componentDeliveryDependency struct {
	label        string
	serviceTypes []string
}

func validateComponentDeliveryPreflight(ctx context.Context, db *gorm.DB, k8sClient client.Client, deployment componentDeploymentContext) error {
	targets, err := loadComponentDeliveryTargets(db, deployment.Env.ID)
	if err != nil {
		return err
	}
	if componentSourceBuildIsPending(deployment.Component) {
		return validateComponentSourceBuildPreflight(ctx, k8sClient, targets)
	}
	if deployment.Component.DeliveryMode == "source" {
		return validateComponentBuiltSourceDeploymentPreflight(targets)
	}
	return validateComponentImageDeploymentPreflight(ctx, deployment, targets)
}

func loadComponentDeliveryServiceInstallations(db *gorm.DB, envID uint) ([]model.ServiceInstallation, error) {
	targets, err := loadComponentDeliveryTargets(db, envID)
	if err != nil {
		return nil, err
	}
	services := make([]model.ServiceInstallation, 0, len(targets))
	seen := map[uint]bool{}
	for _, target := range targets {
		if target.Service == nil || target.Service.ID == 0 || seen[target.Service.ID] {
			continue
		}
		seen[target.Service.ID] = true
		services = append(services, *target.Service)
	}
	return services, nil
}

func loadComponentDeliveryTargets(db *gorm.DB, envID uint) ([]componentDeliveryTarget, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return nil, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return nil, err
	}
	var localServices []model.ServiceInstallation
	if err := db.Where("environment_id = ?", envID).Find(&localServices).Error; err != nil {
		return nil, err
	}

	var capabilities []model.EnvironmentCapability
	if err := db.Where("environment_id = ?", envID).Preload("RefService").Find(&capabilities).Error; err != nil {
		return nil, err
	}

	targets := make([]componentDeliveryTarget, 0, len(localServices)+len(capabilities))
	seen := map[uint]bool{}
	appendService := func(source string, capabilityID uint, inst model.ServiceInstallation, targetApp model.Application, targetEnv model.Environment) {
		if inst.ID != 0 {
			if seen[inst.ID] {
				return
			}
			seen[inst.ID] = true
		}
		targets = append(targets, componentDeliveryTarget{
			Capability:   CapabilityForServiceType(inst.ServiceType),
			CapabilityID: capabilityID,
			Source:       source,
			ServiceType:  inst.ServiceType,
			Service:      &inst,
			App:          targetApp,
			Env:          targetEnv,
		})
	}

	for _, inst := range localServices {
		appendService(model.CapabilitySourceManaged, 0, inst, app, env)
	}

	for _, capability := range capabilities {
		if capability.Source == model.CapabilitySourceExternal {
			targets = append(targets, componentDeliveryTarget{
				Capability:       capability.Capability,
				CapabilityID:     capability.ID,
				Source:           model.CapabilitySourceExternal,
				ServiceType:      firstNonEmpty(capability.ServiceType, capabilityServiceTypeFallback(capability.Capability)),
				ExternalEndpoint: capability.ExternalEndpoint,
				ValidationStatus: capability.ValidationStatus,
			})
			continue
		}
		if capability.RefService == nil {
			continue
		}
		targetEnv := env
		targetApp := app
		if capability.RefService.EnvironmentID != env.ID {
			if loadedEnv, loadErr := findEnvironment(db, capability.RefService.EnvironmentID); loadErr == nil {
				targetEnv = loadedEnv
				if loadedApp, appErr := findApplication(db, loadedEnv.ApplicationID); appErr == nil {
					targetApp = loadedApp
				}
			}
		}
		appendService(capability.Source, capability.ID, *capability.RefService, targetApp, targetEnv)
	}
	return targets, nil
}

func validateComponentSourceBuildPreflight(ctx context.Context, k8sClient client.Client, targets []componentDeliveryTarget) error {
	missing := missingComponentDeliveryDependencies(targets, []componentDeliveryDependency{
		{label: "Git service (Gitea)", serviceTypes: []string{"git"}},
		{label: "CI service (Jenkins)", serviceTypes: []string{"ci"}},
		{label: "image registry service (registry/Harbor)", serviceTypes: []string{"harbor", "registry"}},
		{label: "CD service (ArgoCD)", serviceTypes: []string{"deploy"}},
	})
	if err := validateComponentKpackPreflight(ctx, k8sClient); err != nil {
		missing = append(missing, err.Error())
	}
	if len(missing) > 0 {
		return ComponentValidationError{Message: "source delivery prerequisites are not ready: " + strings.Join(missing, "; ")}
	}
	return nil
}

func validateComponentBuiltSourceDeploymentPreflight(targets []componentDeliveryTarget) error {
	missing := missingComponentDeliveryDependencies(targets, []componentDeliveryDependency{
		{label: "Git service (Gitea)", serviceTypes: []string{"git"}},
		{label: "image registry service (registry/Harbor)", serviceTypes: []string{"harbor", "registry"}},
		{label: "CD service (ArgoCD)", serviceTypes: []string{"deploy"}},
	})
	if len(missing) > 0 {
		return ComponentValidationError{Message: "built source deployment prerequisites are not ready: " + strings.Join(missing, "; ")}
	}
	return nil
}

func validateComponentImageDeploymentPreflight(ctx context.Context, deployment componentDeploymentContext, targets []componentDeliveryTarget) error {
	missing := missingComponentDeliveryDependencies(targets, []componentDeliveryDependency{
		{label: "Git service (Gitea)", serviceTypes: []string{"git"}},
		{label: "image registry service (registry/Harbor)", serviceTypes: []string{"harbor", "registry"}},
		{label: "CD service (ArgoCD)", serviceTypes: []string{"deploy"}},
	})
	if len(missing) > 0 {
		return ComponentValidationError{Message: "image delivery prerequisites are not ready: " + strings.Join(missing, "; ")}
	}
	if !componentImageUsesEnvironmentRegistry(ctx, deployment, targets) {
		return ComponentValidationError{Message: "image delivery requires an image from the current environment registry"}
	}
	return nil
}

func componentImageUsesEnvironmentRegistry(ctx context.Context, deployment componentDeploymentContext, targets []componentDeliveryTarget) bool {
	image := strings.TrimSpace(deployment.Component.Image)
	if image == "" {
		image = strings.TrimSpace(deployment.Component.RegistryImage)
	}
	if image == "" {
		return false
	}
	for _, target := range targets {
		if !componentDeliveryTargetMatches(target, "registry", []string{"registry", "harbor"}) {
			continue
		}
		for _, host := range environmentRegistryPullHosts(ctx, deployment.App, deployment.Env, target) {
			if strings.HasPrefix(image, strings.Trim(host, "/")+"/") {
				return true
			}
		}
	}
	return false
}

func environmentRegistryPullHosts(ctx context.Context, app model.Application, env model.Environment, target componentDeliveryTarget) []string {
	host := componentRegistryRuntimeHost(app, env, target)
	hosts := []string{}
	if strings.TrimSpace(host) != "" {
		hosts = append(hosts, host)
	}
	if target.Service != nil && strings.EqualFold(strings.TrimSpace(target.ServiceType), "registry") {
		if network, err := k8s.DiscoverRegistryServiceNetwork(ctx, target.Service.Namespace, target.Service.ReleaseName); err == nil && network != nil && strings.TrimSpace(network.ClusterIP) != "" {
			hosts = append(hosts, strings.TrimSpace(network.ClusterIP)+":5000")
		}
	}
	return hosts
}

func missingComponentDeliveryDependencies(targets []componentDeliveryTarget, deps []componentDeliveryDependency) []string {
	missing := make([]string, 0)
	for _, dep := range deps {
		if _, ok := findReadyComponentDeliveryTarget(targets, "", dep.serviceTypes); ok {
			continue
		}
		missing = append(missing, dep.label+" is required and must be running")
	}
	return missing
}

func componentDeliveryTargetsFromServices(services []model.ServiceInstallation) []componentDeliveryTarget {
	targets := make([]componentDeliveryTarget, 0, len(services))
	for _, inst := range services {
		targets = append(targets, componentDeliveryTarget{
			Capability:  CapabilityForServiceType(inst.ServiceType),
			Source:      model.CapabilitySourceManaged,
			ServiceType: inst.ServiceType,
			Service:     &inst,
		})
	}
	return targets
}

func findReadyComponentDeliveryService(services []model.ServiceInstallation, serviceTypes []string) (model.ServiceInstallation, bool) {
	for _, serviceType := range serviceTypes {
		for _, inst := range services {
			if !strings.EqualFold(strings.TrimSpace(inst.ServiceType), serviceType) {
				continue
			}
			if componentDeliveryServiceIsReady(inst) {
				return inst, true
			}
		}
	}
	return model.ServiceInstallation{}, false
}

func findReadyComponentDeliveryTarget(targets []componentDeliveryTarget, capability string, serviceTypes []string) (componentDeliveryTarget, bool) {
	for _, target := range targets {
		if !componentDeliveryTargetMatches(target, capability, serviceTypes) || !componentDeliveryTargetReady(target) {
			continue
		}
		return target, true
	}
	return componentDeliveryTarget{}, false
}

func preferredComponentDeliveryTarget(targets []componentDeliveryTarget, capability string, serviceTypes []string) (componentDeliveryTarget, bool) {
	sourceOrder := []string{model.CapabilitySourceManaged, model.CapabilitySourceShared, model.CapabilitySourceExternal}
	for _, readyOnly := range []bool{true, false} {
		for _, source := range sourceOrder {
			for _, target := range targets {
				if !strings.EqualFold(strings.TrimSpace(target.Source), source) {
					continue
				}
				if !componentDeliveryTargetMatches(target, capability, serviceTypes) {
					continue
				}
				if readyOnly && !componentDeliveryTargetReady(target) {
					continue
				}
				return target, true
			}
		}
	}
	return componentDeliveryTarget{}, false
}

func preferredComponentRegistryDeliveryTarget(targets []componentDeliveryTarget, selected *model.ComponentRegistryTarget) (componentDeliveryTarget, bool) {
	if selected != nil {
		for _, target := range targets {
			if !componentDeliveryTargetMatches(target, "registry", []string{"harbor", "registry"}) {
				continue
			}
			if componentDeliveryTargetMatchesRegistrySelection(target, selected) {
				return target, true
			}
		}
	}
	return preferredComponentDeliveryTarget(targets, "registry", []string{"harbor", "registry"})
}

func componentDeliveryTargetMatchesRegistrySelection(target componentDeliveryTarget, selected *model.ComponentRegistryTarget) bool {
	if selected == nil {
		return false
	}
	key := strings.TrimSpace(selected.Key)
	if key != "" {
		if target.Service != nil && key == fmt.Sprintf("service:%d", target.Service.ID) {
			return true
		}
		if target.CapabilityID > 0 && key == fmt.Sprintf("capability:%d", target.CapabilityID) {
			return true
		}
	}
	if selected.ServiceID > 0 && target.Service != nil && target.Service.ID == selected.ServiceID {
		return true
	}
	if selected.CapabilityID > 0 && target.CapabilityID == selected.CapabilityID {
		return true
	}
	if selected.Host != "" {
		selectedHost := externalEndpointHost(selected.Host)
		if selectedHost != "" && selectedHost == externalEndpointHost(target.ExternalEndpoint) {
			return true
		}
	}
	if selected.Source != "" && !strings.EqualFold(strings.TrimSpace(selected.Source), strings.TrimSpace(target.Source)) {
		return false
	}
	if selected.ServiceType != "" && strings.EqualFold(strings.TrimSpace(selected.ServiceType), strings.TrimSpace(target.ServiceType)) {
		return true
	}
	return false
}

func componentDeliveryTargetMatches(target componentDeliveryTarget, capability string, serviceTypes []string) bool {
	if strings.TrimSpace(capability) != "" && strings.EqualFold(strings.TrimSpace(target.Capability), strings.TrimSpace(capability)) {
		return true
	}
	for _, serviceType := range serviceTypes {
		if strings.EqualFold(strings.TrimSpace(target.ServiceType), strings.TrimSpace(serviceType)) {
			return true
		}
	}
	return false
}

func componentDeliveryTargetReady(target componentDeliveryTarget) bool {
	if target.Source == model.CapabilitySourceExternal {
		return strings.TrimSpace(target.ExternalEndpoint) != "" && !strings.EqualFold(strings.TrimSpace(target.ValidationStatus), "failed")
	}
	return target.Service != nil && componentDeliveryServiceIsReady(*target.Service)
}

func componentDeliveryServiceIsReady(inst model.ServiceInstallation) bool {
	return strings.EqualFold(strings.TrimSpace(inst.Status), "running") && strings.TrimSpace(inst.Namespace) != ""
}

func validateComponentKpackPreflight(ctx context.Context, k8sClient client.Client) error {
	if k8sClient == nil {
		return fmt.Errorf("kpack client is not initialized")
	}
	missing := missingComponentKpackPrerequisites(ctx, k8sClient)
	if len(missing) > 0 {
		return fmt.Errorf("kpack is not ready: %s", strings.Join(missing, ", "))
	}
	return nil
}

func missingComponentKpackPrerequisites(ctx context.Context, k8sClient client.Client) []string {
	requiredCRDs := []string{
		"builds.kpack.io",
		"builders.kpack.io",
		"clusterstores.kpack.io",
		"clusterstacks.kpack.io",
		"clusterlifecycles.kpack.io",
		"images.kpack.io",
		"sourceresolvers.kpack.io",
	}
	missing := make([]string, 0)
	for _, name := range requiredCRDs {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name}, crd); err != nil {
			missing = append(missing, name+" CRD")
		}
	}
	controller := &appsv1.Deployment{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: k8s.KpackSystemNamespace, Name: k8s.KpackControllerDeploymentName}, controller); err != nil {
		if apierrors.IsNotFound(err) {
			missing = append(missing, "kpack-controller deployment")
		} else {
			missing = append(missing, "kpack-controller deployment")
		}
	} else if controller.Status.ReadyReplicas < 1 && controller.Status.AvailableReplicas < 1 {
		missing = append(missing, "kpack-controller deployment ready replica")
	}
	return missing
}

func detectAndApplyComponentRuntimeConfig(ctx context.Context, db *gorm.DB, deployment *componentDeploymentContext) error {
	detected := detectComponentContainerPort(ctx, db, deployment.Env, deployment.Identifier, deployment.Component.Image, deployment.Config)
	if detected <= 0 {
		return nil
	}
	deployment.Config.ContainerPort = detected
	configJSON, err := deployment.Config.JSON()
	if err != nil {
		return ComponentValidationError{Message: "component config invalid: " + err.Error()}
	}
	deployment.Component.Config = configJSON
	return nil
}

func saveComponentDeploymentDraft(db *gorm.DB, comp model.Component) error {
	return db.Save(&comp).Error
}

func componentDeploymentWaitsForSourceBuild(comp model.Component) bool {
	return comp.DeliveryMode == "source" && !strings.EqualFold(strings.TrimSpace(comp.PipelineStatus), "built")
}

func upsertComponentRuntimeCR(ctx context.Context, deployment componentDeploymentContext) error {
	envVars, err := model.ComponentEnvVars(deployment.Component.Config)
	if err != nil {
		return ComponentValidationError{Message: "component config invalid: " + err.Error()}
	}
	comp := deployment.Component
	return k8s.UpsertComponentCR(
		ctx,
		deployment.App.Identifier,
		deployment.Env.Identifier,
		comp.Name,
		deployment.Identifier,
		comp.Type,
		comp.Image,
		comp.Version,
		int32(comp.Replicas),
		deployment.PrimaryNamespace,
		"argocd",
		deployment.Config,
		envVars,
	)
}

func componentDeliveryMode(mode string) string {
	if strings.ToLower(strings.TrimSpace(mode)) == "source" {
		return "source"
	}
	return "image"
}

func validateComponentDeliveryInput(input CreateComponentInput) error {
	if input.DraftOnly {
		if strings.TrimSpace(input.Name) == "" {
			return ComponentValidationError{Message: "component name is required"}
		}
		if strings.TrimSpace(input.Type) == "" {
			return ComponentValidationError{Message: "component type is required"}
		}
		if strings.EqualFold(strings.TrimSpace(input.Version), "latest") || strings.HasSuffix(strings.ToLower(strings.TrimSpace(input.Image)), ":latest") {
			return ComponentValidationError{Message: "component image tag must be explicit; latest is not allowed"}
		}
		return nil
	}
	version := strings.TrimSpace(input.Version)
	if strings.EqualFold(version, "latest") {
		return ComponentValidationError{Message: "component image tag must be explicit; latest is not allowed"}
	}
	if componentDeliveryMode(input.DeliveryMode) == "source" {
		if strings.TrimSpace(input.SourceRepoURL) == "" {
			return ComponentValidationError{Message: "source repository URL is required"}
		}
		return nil
	}
	if strings.TrimSpace(input.Image) == "" {
		return ComponentValidationError{Message: "component image is required"}
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(input.Image)), ":latest") {
		return ComponentValidationError{Message: "component image tag must be explicit; latest is not allowed"}
	}
	if version == "" && imageReferenceTag(input.Image) == "" {
		return ComponentValidationError{Message: "component version is required when image has no tag"}
	}
	return nil
}

func preferredSourceRegistryServiceType(services []model.ServiceInstallation) string {
	serviceType, _, _ := preferredSourceRegistrySelection(services)
	return serviceType
}

func preferredSourceRegistrySelection(services []model.ServiceInstallation) (string, model.ServiceInstallation, bool) {
	for _, status := range []string{"running", ""} {
		for _, serviceType := range []string{"harbor", "registry"} {
			for _, inst := range services {
				if inst.ServiceType != serviceType {
					continue
				}
				if status == "" || strings.EqualFold(inst.Status, status) {
					return serviceType, inst, true
				}
			}
		}
	}
	return "registry", model.ServiceInstallation{}, false
}

func valueOrDefaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func applyComponentDeployVersion(comp model.Component, version string) model.Component {
	version = strings.TrimSpace(version)
	if version == "" {
		return comp
	}
	comp.Version = version
	if strings.TrimSpace(comp.RegistryImage) != "" {
		comp.RegistryImage = ImageWithTag(comp.RegistryImage, version)
		comp.Image = comp.RegistryImage
	} else if strings.TrimSpace(comp.Image) != "" {
		comp.Image = ImageWithTag(comp.Image, version)
	}
	if comp.DeliveryMode == "source" || comp.JenkinsJob != "" || comp.RegistryImage != "" {
		comp.PipelineStatus = "built"
	}
	return comp
}

func componentUsesDirectImageReference(comp model.Component) bool {
	return strings.TrimSpace(comp.DeliveryMode) != "source" && strings.TrimSpace(comp.JenkinsJob) == ""
}

func prepareComponentSourceBuildVersion(app model.Application, env model.Environment, comp model.Component, identifier, version, registryServiceType string) model.Component {
	target := componentDeliveryTarget{Capability: "registry", Source: model.CapabilitySourceManaged, ServiceType: registryServiceType, App: app, Env: env}
	return prepareComponentSourceBuildVersionForTarget(app, env, comp, identifier, version, target)
}

func prepareComponentSourceBuildVersionForTarget(app model.Application, env model.Environment, comp model.Component, identifier, version string, target componentDeliveryTarget) model.Component {
	version = strings.TrimSpace(version)
	if version == "" {
		return comp
	}
	repository := fmt.Sprintf("%s-%s/%s", app.Identifier, env.Identifier, identifier)
	comp.Version = version
	comp.RegistryImage = componentRegistryRuntimeImage(app, env, target, repository, version)
	comp.Image = comp.RegistryImage
	if comp.JenkinsJob == "" {
		comp.JenkinsJob = fmt.Sprintf("%s-%s-%s-build", app.Identifier, env.Identifier, identifier)
	}
	comp.PipelineStatus = "planned"
	return comp
}

func applyComponentDeployVersionForRuntimeRegistry(app model.Application, env model.Environment, comp model.Component, identifier, version, registryServiceType string) model.Component {
	target := componentDeliveryTarget{Capability: "registry", Source: model.CapabilitySourceManaged, ServiceType: registryServiceType, App: app, Env: env}
	return applyComponentDeployVersionForRuntimeRegistryTarget(app, env, comp, identifier, version, target)
}

func applyComponentDeployVersionForRuntimeRegistryTarget(app model.Application, env model.Environment, comp model.Component, identifier, version string, target componentDeliveryTarget) model.Component {
	version = strings.TrimSpace(version)
	if version == "" {
		return comp
	}
	repository := fmt.Sprintf("%s-%s/%s", app.Identifier, env.Identifier, identifier)
	comp.Version = version
	comp.RegistryImage = componentRegistryRuntimeImage(app, env, target, repository, version)
	comp.Image = comp.RegistryImage
	if comp.DeliveryMode == "source" || comp.JenkinsJob != "" || comp.RegistryImage != "" {
		comp.PipelineStatus = "built"
	}
	return comp
}

func applyComponentImageDeployVersionForRuntimeRegistryTarget(app model.Application, env model.Environment, comp model.Component, identifier, version string, target componentDeliveryTarget) model.Component {
	version = strings.TrimSpace(version)
	if version == "" {
		return comp
	}
	image := strings.TrimSpace(comp.RegistryImage)
	if image == "" {
		image = strings.TrimSpace(comp.Image)
	}
	repository, _ := imageRepositoryAndReference(image)
	if strings.TrimSpace(repository) == "" {
		repository = fmt.Sprintf("%s-%s/%s", app.Identifier, env.Identifier, identifier)
	}
	comp.Version = version
	comp.RegistryImage = componentRegistryRuntimeImage(app, env, target, repository, version)
	comp.Image = comp.RegistryImage
	comp.PipelineStatus = "built"
	return comp
}

func componentRegistryRuntimeImage(app model.Application, env model.Environment, target componentDeliveryTarget, repository, tag string) string {
	host := componentRegistryRuntimeHost(app, env, target)
	repository = strings.Trim(repository, "/")
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "latest"
	}
	return fmt.Sprintf("%s/%s:%s", strings.TrimRight(host, "/"), repository, tag)
}

func componentRegistryRuntimeHost(app model.Application, env model.Environment, target componentDeliveryTarget) string {
	if strings.EqualFold(strings.TrimSpace(target.Source), model.CapabilitySourceExternal) {
		return externalEndpointHost(target.ExternalEndpoint)
	}
	serviceType := strings.TrimSpace(target.ServiceType)
	if serviceType == "" {
		serviceType = "registry"
	}
	targetApp := target.App
	targetEnv := target.Env
	if strings.TrimSpace(targetApp.Identifier) == "" {
		targetApp = app
	}
	if strings.TrimSpace(targetEnv.Identifier) == "" {
		targetEnv = env
	}
	return RuntimeRegistryHost(targetApp, targetEnv, serviceType)
}

func externalEndpointHost(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Host != "" {
		return strings.TrimRight(parsed.Host, "/")
	}
	return strings.Trim(strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://"), "/")
}

func detectComponentContainerPort(ctx context.Context, db *gorm.DB, env model.Environment, identifier, image string, cfg model.ComponentConfig) int32 {
	if cfg.ContainerPort > 0 {
		return 0
	}
	if detected := detectComponentImageContainerPort(ctx, db, env.ID, image); detected > 0 {
		return detected
	}
	if runtime, err := k8s.DiscoverComponentRuntimeConfig(ctx, env.Namespace, identifier); err == nil && runtime != nil && len(runtime.Ports) > 0 {
		return runtime.Ports[0]
	}
	return 0
}

func detectComponentImageContainerPort(ctx context.Context, db *gorm.DB, envID uint, image string) int32 {
	repository, reference := imageRepositoryAndReference(image)
	if repository == "" || reference == "" {
		return 0
	}
	var registries []model.ServiceInstallation
	if err := db.Where("environment_id = ? AND service_type IN ?", envID, []string{"registry", "harbor"}).Find(&registries).Error; err != nil {
		return 0
	}
	for _, inst := range registries {
		ports, err := k8s.NewRegistryClient(inst.Namespace).ExposedPorts(ctx, repository, reference)
		if err != nil || len(ports) == 0 {
			continue
		}
		return ports[0]
	}
	return 0
}

func imageReferenceTag(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	colon := strings.LastIndex(last, ":")
	if colon < 0 || colon == len(last)-1 {
		return ""
	}
	return last[colon+1:]
}

func imageRepositoryAndReference(image string) (string, string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	if digestAt := strings.Index(image, "@"); digestAt >= 0 {
		repository := strings.Trim(strings.TrimSpace(image[:digestAt]), "/")
		reference := strings.TrimSpace(image[digestAt+1:])
		if slash := strings.Index(repository, "/"); slash >= 0 && imageReferenceFirstSegmentIsRegistry(repository[:slash]) {
			repository = repository[slash+1:]
		}
		return repository, reference
	}
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	colon := strings.LastIndex(last, ":")
	if colon < 0 || colon == len(last)-1 {
		return "", ""
	}
	reference := last[colon+1:]
	parts[len(parts)-1] = last[:colon]
	if len(parts) > 1 && imageReferenceFirstSegmentIsRegistry(parts[0]) {
		parts = parts[1:]
	}
	return strings.Trim(strings.Join(parts, "/"), "/"), reference
}

func imageReferenceFirstSegmentIsRegistry(segment string) bool {
	segment = strings.ToLower(strings.TrimSpace(segment))
	return segment == "localhost" || strings.Contains(segment, ".") || strings.Contains(segment, ":")
}
