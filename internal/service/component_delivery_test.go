package service

import (
	"strings"
	"testing"

	"paap/internal/model"
)

func TestCreateComponentAllowsSourceDraftWithoutVersion(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	comp, err := CreateComponent(db, env.ID, CreateComponentInput{
		Name:          "Orders API",
		Type:          "backend",
		DeliveryMode:  "source",
		SourceRepoURL: "https://git.example.com/team/orders.git",
	})
	if err != nil {
		t.Fatalf("create source component: %v", err)
	}
	if comp.DeliveryMode != "source" || comp.Status != "draft" || comp.PipelineStatus != "draft" {
		t.Fatalf("unexpected source draft state: delivery=%q status=%q pipeline=%q", comp.DeliveryMode, comp.Status, comp.PipelineStatus)
	}
	if comp.Version != "" || comp.Image != "" || comp.RegistryImage != "" {
		t.Fatalf("source draft should not precompute build output: version=%q image=%q registry=%q", comp.Version, comp.Image, comp.RegistryImage)
	}
	if comp.SourceBranch != "main" || comp.BuildContext != "." {
		t.Fatalf("source defaults not applied: branch=%q context=%q", comp.SourceBranch, comp.BuildContext)
	}
}

func TestCreateComponentReadsImageTagOnlyFromLastPathSegment(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	cases := []struct {
		name    string
		image   string
		version string
		want    string
	}{
		{name: "port-only", image: "registry.local:5000/order", version: "manual", want: "manual"},
		{name: "registry-port-and-tag", image: "registry.local:5000/order:v1.0.0", want: "v1.0.0"},
		{name: "short-tag", image: "order:v1.0.0", want: "v1.0.0"},
		{name: "no-tag", image: "order", version: "manual", want: "manual"},
	}

	for _, tc := range cases {
		comp, err := CreateComponent(db, env.ID, CreateComponentInput{
			Name:    tc.name,
			Type:    "backend",
			Image:   tc.image,
			Version: tc.version,
		})
		if err != nil {
			t.Fatalf("create component %s: %v", tc.name, err)
		}
		if comp.Version != tc.want {
			t.Fatalf("component %s version = %q, want %q", tc.name, comp.Version, tc.want)
		}
	}
}

func TestUpdateComponentSwitchesBetweenSourceAndImage(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	comp := model.Component{EnvironmentID: env.ID, Name: "Orders API", Type: "backend", DeliveryMode: "image", Image: "registry.local/orders:v1", Version: "v1", Replicas: 1}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	updated, err := UpdateComponent(db, comp.ID, UpdateComponentInput{
		DeliveryMode:  "source",
		SourceRepoURL: "https://git.example.com/team/orders.git",
	})
	if err != nil {
		t.Fatalf("switch to source: %v", err)
	}
	if updated.DeliveryMode != "source" || updated.JenkinsJob != "shop-dev-orders-api-build" || updated.PipelineStatus != "planned" {
		t.Fatalf("unexpected source state: delivery=%q job=%q pipeline=%q", updated.DeliveryMode, updated.JenkinsJob, updated.PipelineStatus)
	}

	updated, err = UpdateComponent(db, comp.ID, UpdateComponentInput{
		DeliveryMode: "image",
		Image:        "registry.local/orders:v2",
	})
	if err != nil {
		t.Fatalf("switch to image: %v", err)
	}
	if updated.DeliveryMode != "image" || updated.SourceRepoURL != "" || updated.JenkinsJob != "" || updated.PipelineStatus != "" {
		t.Fatalf("source metadata not cleared for image flow: %#v", updated)
	}
	if updated.Image != "registry.local/orders:v2" || updated.Version != "v2" {
		t.Fatalf("image update not applied: image=%q version=%q", updated.Image, updated.Version)
	}
}

func TestUpdateSourceComponentRecomputesRuntimeRegistryImageFromSelectedTarget(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{app}-{env}.paap.local")
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	registry := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceName:   "dev-registry",
		ServiceType:   "registry",
		Status:        "running",
		Namespace:     "shop-dev-registry",
		ReleaseName:   "shop-dev-registry",
	}
	if err := db.Create(&registry).Error; err != nil {
		t.Fatalf("create registry: %v", err)
	}
	comp := model.Component{
		EnvironmentID:  env.ID,
		Name:           "Gateway",
		Type:           "frontend",
		DeliveryMode:   "source",
		SourceRepoURL:  "https://github.com/sqshq/piggymetrics.git",
		SourceBranch:   "master",
		BuildContext:   "gateway",
		Image:          "10.96.190.247:5000/shop-dev/gateway:6bb2cf9",
		RegistryImage:  "10.96.190.247:5000/shop-dev/gateway:6bb2cf9",
		Version:        "6bb2cf9",
		PipelineStatus: "planned",
		JenkinsJob:     "shop-dev-gateway-build",
		Replicas:       1,
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	cfg := &model.ComponentConfig{
		RegistryTarget: &model.ComponentRegistryTarget{
			Key:         "service:1",
			Source:      model.CapabilitySourceManaged,
			Host:        "registry.shop-dev.paap.local",
			ServiceID:   registry.ID,
			ServiceType: "registry",
			Name:        "dev-registry",
		},
	}
	updated, err := UpdateComponent(db, comp.ID, UpdateComponentInput{
		DeliveryMode:  "source",
		SourceRepoURL: "https://github.com/sqshq/piggymetrics.git",
		SourceBranch:  "master",
		BuildContext:  ".",
		BuildModule:   "gateway",
		Image:         comp.Image,
		Version:       "6bb2cf9",
		Config:        cfg,
	})
	if err != nil {
		t.Fatalf("update component: %v", err)
	}

	wantImage := "registry.shop-dev.paap.local/shop-dev/gateway:6bb2cf9"
	if updated.Image != wantImage || updated.RegistryImage != wantImage {
		t.Fatalf("source image not recomputed from runtime registry: image=%q registry=%q want=%q", updated.Image, updated.RegistryImage, wantImage)
	}
	if updated.BuildContext != "." || updated.BuildModule != "gateway" {
		t.Fatalf("build fields not saved: context=%q module=%q", updated.BuildContext, updated.BuildModule)
	}
	if updated.PipelineStatus != "planned" {
		t.Fatalf("pipeline status = %q, want planned", updated.PipelineStatus)
	}
}

func TestUpdateImageComponentRecomputesRuntimeRegistryImageFromSelectedTarget(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{app}-{env}.paap.local")
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	registry := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceName:   "dev-registry",
		ServiceType:   "registry",
		Status:        "running",
		Namespace:     "shop-dev-registry",
		ReleaseName:   "shop-dev-registry",
	}
	if err := db.Create(&registry).Error; err != nil {
		t.Fatalf("create registry: %v", err)
	}
	comp := model.Component{
		EnvironmentID: env.ID,
		Name:          "Auth Service",
		Type:          "backend",
		DeliveryMode:  "image",
		Image:         "10.96.190.247:5000/shop-dev/auth-service:6bb2cf9",
		RegistryImage: "10.96.190.247:5000/shop-dev/auth-service:6bb2cf9",
		Version:       "6bb2cf9",
		Replicas:      1,
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	cfg := &model.ComponentConfig{
		RegistryTarget: &model.ComponentRegistryTarget{
			Key:         "service:1",
			Source:      model.CapabilitySourceManaged,
			Host:        "registry.shop-dev.paap.local",
			ServiceID:   registry.ID,
			ServiceType: "registry",
			Name:        "dev-registry",
		},
	}
	updated, err := UpdateComponent(db, comp.ID, UpdateComponentInput{
		DeliveryMode: "image",
		Image:        comp.Image,
		Version:      "6bb2cf9",
		Config:       cfg,
	})
	if err != nil {
		t.Fatalf("update component: %v", err)
	}

	wantImage := "registry.shop-dev.paap.local/shop-dev/auth-service:6bb2cf9"
	if updated.Image != wantImage || updated.RegistryImage != wantImage {
		t.Fatalf("image delivery should use runtime registry host: image=%q registry=%q want=%q", updated.Image, updated.RegistryImage, wantImage)
	}
	if updated.PipelineStatus != "built" {
		t.Fatalf("pipeline status = %q, want built", updated.PipelineStatus)
	}
}

func TestPreferredSourceRegistryServiceTypePrefersRunningHarbor(t *testing.T) {
	services := []model.ServiceInstallation{
		{ServiceType: "registry", Status: "running"},
		{ServiceType: "harbor", Status: "running"},
	}

	if got := preferredSourceRegistryServiceType(services); got != "harbor" {
		t.Fatalf("registry service type = %q, want harbor", got)
	}
}

func TestLoadComponentDeliveryServiceInstallationsIncludesSharedCapabilityRefs(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.EnvironmentCapability{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	sharedApp := model.Application{Name: SystemSharedApplicationName, Identifier: SystemSharedApplicationIdentifier, IsSystem: true}
	if err := db.Create(&sharedApp).Error; err != nil {
		t.Fatalf("create shared app: %v", err)
	}
	sharedEnv := model.Environment{ApplicationID: sharedApp.ID, Name: SystemSharedEnvironmentName, Identifier: SystemSharedEnvironmentIdentifier, IsSystem: true}
	if err := db.Create(&sharedEnv).Error; err != nil {
		t.Fatalf("create shared env: %v", err)
	}

	sharedGit := model.ServiceInstallation{EnvironmentID: sharedEnv.ID, ServiceType: "git", Status: "running", Namespace: "default-shared-git"}
	localDeploy := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "shop-dev-deploy"}
	if err := db.Create(&sharedGit).Error; err != nil {
		t.Fatalf("create shared git: %v", err)
	}
	if err := db.Create(&localDeploy).Error; err != nil {
		t.Fatalf("create local deploy: %v", err)
	}
	if err := db.Create(&model.EnvironmentCapability{
		EnvironmentID:     env.ID,
		Capability:        "git",
		CapabilityKey:     "git-shared",
		Source:            model.CapabilitySourceShared,
		ServiceType:       "git",
		RefServiceID:      &sharedGit.ID,
		ValidationStatus:  "linked",
		ValidationMessage: "linked",
	}).Error; err != nil {
		t.Fatalf("create shared capability: %v", err)
	}

	services, err := loadComponentDeliveryServiceInstallations(db, env.ID)
	if err != nil {
		t.Fatalf("load delivery services: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("services = %#v, want shared git and local deploy", services)
	}
	if services[0].Namespace != "shop-dev-deploy" || services[1].Namespace != "default-shared-git" {
		t.Fatalf("services should prefer local resources and then shared capability refs, got %#v", services)
	}
	namespaces := componentToolNamespacesFromServices(services)
	if namespaces.Gitea != "default-shared-git" || namespaces.ArgoCD != "shop-dev-deploy" {
		t.Fatalf("tool namespaces = %#v", namespaces)
	}
}

func TestApplyComponentDeployVersionReplacesExistingImageTag(t *testing.T) {
	comp := model.Component{
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		Version:       "v1.2.3",
		RegistryImage: "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
	}

	updated := applyComponentDeployVersion(comp, "17")

	wantImage := "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:17"
	if updated.Image != wantImage {
		t.Fatalf("image = %q, want %q", updated.Image, wantImage)
	}
	if updated.Version != "17" {
		t.Fatalf("version = %q, want 17", updated.Version)
	}
	if updated.RegistryImage != wantImage {
		t.Fatalf("registry image = %q, want %q", updated.RegistryImage, wantImage)
	}
	if updated.PipelineStatus != "built" {
		t.Fatalf("pipeline status = %q, want built", updated.PipelineStatus)
	}
}

func TestApplyComponentDeployVersionRecomputesSourceRegistryHost(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.paap.local:5000")
	comp := model.Component{
		Name:          "source-smoke",
		DeliveryMode:  "source",
		Image:         "registry.test-staging.paap.local:5000/test-staging/source-smoke:manual",
		Version:       "manual",
		RegistryImage: "registry.test-staging.paap.local:5000/test-staging/source-smoke:manual",
	}

	updated := applyComponentDeployVersionForRuntimeRegistry(
		model.Application{Identifier: "test"},
		model.Environment{Identifier: "staging"},
		comp,
		"source-smoke",
		"manual",
		"registry",
	)

	wantImage := "registry.paap.local:5000/test-staging/source-smoke:manual"
	if updated.Image != wantImage {
		t.Fatalf("image = %q, want %q", updated.Image, wantImage)
	}
	if updated.RegistryImage != wantImage {
		t.Fatalf("registry image = %q, want %q", updated.RegistryImage, wantImage)
	}
	if updated.PipelineStatus != "built" {
		t.Fatalf("pipeline status = %q, want built", updated.PipelineStatus)
	}
}

func TestPrepareComponentSourceBuildVersionDoesNotMarkImageBuilt(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{app}-{env}.paap.local:5000")
	comp := model.Component{
		Name:           "Orders API",
		Type:           "backend",
		DeliveryMode:   "source",
		PipelineStatus: "planned",
	}

	prepared := prepareComponentSourceBuildVersion(
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		comp,
		"orders-api",
		"v1.2.3",
		"registry",
	)

	wantImage := "registry.shop-dev.paap.local:5000/shop-dev/orders-api:v1.2.3"
	if prepared.Image != wantImage || prepared.RegistryImage != wantImage {
		t.Fatalf("source build image not prepared: image=%q registry=%q want=%q", prepared.Image, prepared.RegistryImage, wantImage)
	}
	if prepared.PipelineStatus != "planned" {
		t.Fatalf("source build preparation must not mark image built, got %q", prepared.PipelineStatus)
	}
	if prepared.JenkinsJob != "shop-dev-orders-api-build" {
		t.Fatalf("jenkins job = %q, want shop-dev-orders-api-build", prepared.JenkinsJob)
	}
}

func TestValidateComponentSourceBuildPreflightRequiresDeliveryDependencies(t *testing.T) {
	err := validateComponentSourceBuildPreflight(t.Context(), gitOpsFlowClient(t), nil)
	if err == nil {
		t.Fatalf("expected source build preflight to fail")
	}
	message := err.Error()
	for _, want := range []string{
		"Git service (Gitea)",
		"CI service (Jenkins)",
		"image registry service (registry/Harbor)",
		"CD service (ArgoCD)",
		"kpack is not ready",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("preflight error missing %q: %s", want, message)
		}
	}
}

func TestValidateComponentImageDeploymentPreflightRequiresRegistryAndGitOpsDependencies(t *testing.T) {
	err := validateComponentImageDeploymentPreflight(t.Context(), componentDeploymentContext{}, componentDeliveryTargetsFromServices([]model.ServiceInstallation{
		{ServiceType: "git", Status: "running", Namespace: "shop-dev-git"},
	}), "argocd")
	if err == nil {
		t.Fatalf("expected image delivery preflight to fail when registry and ArgoCD are missing")
	}
	message := err.Error()
	if !strings.Contains(message, "CD service (ArgoCD)") {
		t.Fatalf("preflight error should mention ArgoCD: %s", message)
	}
	if !strings.Contains(message, "image registry service (registry/Harbor)") {
		t.Fatalf("preflight error should mention registry: %s", message)
	}
	for _, unexpected := range []string{"CI service", "kpack"} {
		if strings.Contains(message, unexpected) {
			t.Fatalf("image delivery preflight should not require %s: %s", unexpected, message)
		}
	}
}

func TestValidateComponentImageDeploymentPreflightRejectsExternalImage(t *testing.T) {
	deployment := componentDeploymentContext{
		App:       model.Application{Identifier: "shop"},
		Env:       model.Environment{Identifier: "dev"},
		Component: model.Component{DeliveryMode: "image", Image: "sqshq/piggymetrics-gateway:fd5ee3c"},
	}
	services := []model.ServiceInstallation{
		{ServiceType: "git", Status: "running", Namespace: "shop-dev-git"},
		{ServiceType: "registry", Status: "running", Namespace: "shop-dev-registry"},
		{ServiceType: "deploy", Status: "running", Namespace: "shop-dev-deploy"},
	}
	targets := componentDeliveryTargetsFromServices(services)

	err := validateComponentImageDeploymentPreflight(t.Context(), deployment, targets, "argocd")
	if err == nil || !strings.Contains(err.Error(), "current environment registry") {
		t.Fatalf("expected external image to be rejected, got %v", err)
	}

	deployment.Component.Image = "registry.shop-dev.paap.local/shop-dev/gateway:v1"
	err = validateComponentImageDeploymentPreflight(t.Context(), deployment, targets, "argocd")
	if err == nil || !strings.Contains(err.Error(), "registry health check failed") {
		t.Fatalf("expected registry health check to be performed and fail in test environment, got %v", err)
	}
}

func TestValidateComponentSourceBuildPreflightPassesWhenDependenciesAreReady(t *testing.T) {
	services := []model.ServiceInstallation{
		{ServiceType: "git", Status: "running", Namespace: "shop-dev-git"},
		{ServiceType: "ci", Status: "running", Namespace: "shop-dev-ci"},
		{ServiceType: "harbor", Status: "running", Namespace: "shop-dev-harbor"},
		{ServiceType: "deploy", Status: "running", Namespace: "shop-dev-deploy"},
	}

	if err := validateComponentSourceBuildPreflight(t.Context(), kpackReadyClient(t), componentDeliveryTargetsFromServices(services)); err != nil {
		t.Fatalf("source build preflight should pass: %v", err)
	}
}

func TestPreferredComponentDeliveryTargetPrioritizesManagedSharedExternal(t *testing.T) {
	shared := model.ServiceInstallation{ID: 2, ServiceType: "registry", Status: "running", Namespace: "shared-registry"}
	local := model.ServiceInstallation{ID: 1, ServiceType: "registry", Status: "running", Namespace: "shop-dev-registry"}
	targets := []componentDeliveryTarget{
		{Capability: "registry", Source: model.CapabilitySourceExternal, ServiceType: "registry", ExternalEndpoint: "registry.external.example.com", ValidationStatus: "success"},
		{Capability: "registry", Source: model.CapabilitySourceShared, ServiceType: "registry", Service: &shared},
		{Capability: "registry", Source: model.CapabilitySourceManaged, ServiceType: "registry", Service: &local},
	}

	target, ok := preferredComponentDeliveryTarget(targets, "registry", []string{"registry"})

	if !ok || target.Service == nil || target.Service.Namespace != "shop-dev-registry" {
		t.Fatalf("target = %#v, want managed local registry", target)
	}
}

func TestPreferredComponentDeliveryTargetFallsBackToSharedThenExternal(t *testing.T) {
	shared := model.ServiceInstallation{ID: 2, ServiceType: "registry", Status: "running", Namespace: "shared-registry"}
	targets := []componentDeliveryTarget{
		{Capability: "registry", Source: model.CapabilitySourceExternal, ServiceType: "registry", ExternalEndpoint: "registry.external.example.com", ValidationStatus: "success"},
		{Capability: "registry", Source: model.CapabilitySourceShared, ServiceType: "registry", Service: &shared},
	}

	target, ok := preferredComponentDeliveryTarget(targets, "registry", []string{"registry"})

	if !ok || target.Service == nil || target.Service.Namespace != "shared-registry" {
		t.Fatalf("target = %#v, want shared registry", target)
	}

	targets = []componentDeliveryTarget{{Capability: "registry", Source: model.CapabilitySourceExternal, ServiceType: "registry", ExternalEndpoint: "registry.external.example.com", ValidationStatus: "success"}}
	target, ok = preferredComponentDeliveryTarget(targets, "registry", []string{"registry"})
	if !ok || target.Source != model.CapabilitySourceExternal || target.ExternalEndpoint != "registry.external.example.com" {
		t.Fatalf("target = %#v, want external registry", target)
	}
}

func TestPreferredComponentRegistryDeliveryTargetUsesSelectedService(t *testing.T) {
	local := model.ServiceInstallation{ID: 1, ServiceType: "registry", Status: "running", Namespace: "shop-dev-registry"}
	shared := model.ServiceInstallation{ID: 2, ServiceType: "registry", Status: "running", Namespace: "shared-registry"}
	targets := []componentDeliveryTarget{
		{Capability: "registry", Source: model.CapabilitySourceManaged, ServiceType: "registry", Service: &local},
		{Capability: "registry", CapabilityID: 9, Source: model.CapabilitySourceShared, ServiceType: "registry", Service: &shared},
	}

	target, ok := preferredComponentRegistryDeliveryTarget(targets, &model.ComponentRegistryTarget{Key: "service:2", ServiceID: 2})

	if !ok || target.Service == nil || target.Service.ID != 2 {
		t.Fatalf("target = %#v, want selected shared registry", target)
	}
}

func TestPreferredComponentRegistryDeliveryTargetUsesSelectedExternalCapability(t *testing.T) {
	local := model.ServiceInstallation{ID: 1, ServiceType: "registry", Status: "running", Namespace: "shop-dev-registry"}
	targets := []componentDeliveryTarget{
		{Capability: "registry", Source: model.CapabilitySourceManaged, ServiceType: "registry", Service: &local},
		{Capability: "registry", CapabilityID: 11, Source: model.CapabilitySourceExternal, ServiceType: "registry", ExternalEndpoint: "https://registry.external.example.com", ValidationStatus: "success"},
	}

	target, ok := preferredComponentRegistryDeliveryTarget(targets, &model.ComponentRegistryTarget{Key: "capability:11", CapabilityID: 11})

	if !ok || target.Source != model.CapabilitySourceExternal || target.CapabilityID != 11 {
		t.Fatalf("target = %#v, want selected external registry", target)
	}
}
