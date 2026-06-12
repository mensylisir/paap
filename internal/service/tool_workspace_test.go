package service

import (
	"strings"
	"testing"

	"paap/internal/model"
)

func TestBuildToolWorkspaceReturnsGiteaRepositoriesFromComponents(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "dev"}
	inst := model.ServiceInstallation{ServiceType: "git", Status: "running", Namespace: "billing-dev-git", ReleaseName: "billing-dev-git"}
	components := []model.Component{
		{Name: "api", GitRepoURL: "http://gitea/paap/billing-dev-components.git", GitPath: "components/api"},
		{Name: "web", GitRepoURL: "http://gitea/paap/billing-dev-components.git", GitPath: "components/web"},
	}

	workspace := BuildToolWorkspace(app, env, inst, components)

	if workspace.Kind != "repository" || workspace.Title != "代码仓库" {
		t.Fatalf("unexpected workspace: %#v", workspace)
	}
	if len(workspace.Resources) != 1 {
		t.Fatalf("expected 1 real repo resource, got %d: %#v", len(workspace.Resources), workspace.Resources)
	}
	if workspace.Resources[0].Name != "billing-dev-components" {
		t.Fatalf("unexpected repo resource: %#v", workspace.Resources[0])
	}
	componentPaths, ok := workspace.Resources[0].Annotations["componentPaths"].([]string)
	if !ok {
		t.Fatalf("expected componentPaths annotation, got %#v", workspace.Resources[0].Annotations["componentPaths"])
	}
	if strings.Join(componentPaths, ",") != "components/api,components/web" {
		t.Fatalf("unexpected component paths: %#v", componentPaths)
	}
	if _, ok := workspace.Resources[0].Annotations["path"]; ok {
		t.Fatalf("real repository card must not force a component path as repository root: %#v", workspace.Resources[0].Annotations)
	}
	if workspace.Resources[0].ExternalURL == "" {
		t.Fatalf("expected repository external URL")
	}
	actionKeys := map[string]bool{}
	for _, action := range workspace.Actions {
		actionKeys[action.Key] = true
	}
	if !actionKeys["reconcile_gitops"] {
		t.Fatalf("expected reconcile action, got %#v", workspace.Actions)
	}
}

func TestBuildToolWorkspaceExposesGiteaRepositoryAndKeyActions(t *testing.T) {
	workspace := BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "dev"},
		model.ServiceInstallation{ID: 3, EnvironmentID: 2, ServiceType: "git", Status: "running", Namespace: "billing-dev-git"},
		nil,
	)

	actionKeys := map[string]bool{}
	for _, action := range workspace.Actions {
		actionKeys[action.Key] = true
		if strings.Contains(action.Label, "创建/修复") {
			t.Fatalf("git workspace action must not expose internal repair wording: %#v", action)
		}
		switch action.Key {
		case "create_gitea_repository":
			assertActionFields(t, action, "name", "description", "private")
		case "add_gitea_user_key":
			assertActionFields(t, action, "title", "key")
		case "add_gitea_deploy_key":
			assertActionFields(t, action, "repository", "title", "key", "readOnly")
		case "reconcile_gitops":
			if action.Label != "同步代码仓" {
				t.Fatalf("unexpected reconcile action label: %#v", action)
			}
		}
	}
	for _, want := range []string{"create_gitea_repository", "add_gitea_user_key", "add_gitea_deploy_key"} {
		if !actionKeys[want] {
			t.Fatalf("git workspace missing action %s: %#v", want, workspace.Actions)
		}
	}
	if !workspaceConfigContains(workspace.Config, "平台代理入口", "/api/v1/environments/2/services/3/proxy/") {
		t.Fatalf("workspace config must expose PAAP proxy entry, got %#v", workspace.Config)
	}
	if !workspaceConfigContains(workspace.Config, "集群内地址", "http://billing-dev-git.billing-dev-git.svc.cluster.local:3000") {
		t.Fatalf("workspace config must keep internal service address separate, got %#v", workspace.Config)
	}
}

func TestBuildToolWorkspaceDoesNotInventRepositoryResourcesWithoutComponents(t *testing.T) {
	workspace := BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "empty"},
		model.ServiceInstallation{ServiceType: "git", Status: "running", Namespace: "billing-empty-git"},
		nil,
	)

	if len(workspace.Resources) != 0 {
		t.Fatalf("git workspace must not invent repository resources without components, got %#v", workspace.Resources)
	}
}

func TestBuildToolWorkspaceReturnsArgoCDApplicationsFromComponents(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}
	inst := model.ServiceInstallation{ServiceType: "deploy", Status: "running", Namespace: "billing-prod-deploy"}
	components := []model.Component{
		{Name: "api", Status: "syncing", ArgoCDApp: "billing-prod-api"},
	}

	workspace := BuildToolWorkspace(app, env, inst, components)

	if workspace.Kind != "gitops" || workspace.Title != "ArgoCD Applications" {
		t.Fatalf("unexpected workspace: %#v", workspace)
	}
	if len(workspace.Resources) != 1 {
		t.Fatalf("expected 1 application resource, got %d", len(workspace.Resources))
	}
	if workspace.Resources[0].Name != "billing-prod-api" || workspace.Resources[0].Type != "Application" {
		t.Fatalf("unexpected application resource: %#v", workspace.Resources[0])
	}
	actionKeys := map[string]bool{}
	for _, action := range workspace.Resources[0].Actions {
		actionKeys[action.Key] = true
	}
	for _, want := range []string{"sync_argocd_application", "delete_argocd_application"} {
		if !actionKeys[want] {
			t.Fatalf("expected application action %s: %#v", want, workspace.Resources[0].Actions)
		}
	}
}

func TestBuildToolWorkspaceArgoCDApplicationActionUsesAutomaticProject(t *testing.T) {
	workspace := BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "prod"},
		model.ServiceInstallation{ServiceType: "deploy", Status: "running", Namespace: "billing-prod-deploy"},
		nil,
	)

	var action ToolWorkspaceAction
	for _, candidate := range workspace.Actions {
		if candidate.Key == "apply_argocd_application" {
			action = candidate
			break
		}
	}
	if action.Key == "" {
		t.Fatalf("missing apply_argocd_application action: %#v", workspace.Actions)
	}
	if !strings.Contains(action.Description, "Project 自动绑定") {
		t.Fatalf("action description must explain automatic project binding, got %q", action.Description)
	}
	for _, field := range action.Fields {
		if field.Name == "project" {
			t.Fatalf("project must not be user-fillable: %#v", action.Fields)
		}
	}
	assertActionFields(t, action, "name", "repoURL", "path", "targetRevision", "destinationNamespace", "automated")
}

func TestBuildToolWorkspaceArgoCDApplicationSetActionUsesAutomaticProject(t *testing.T) {
	workspace := BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "prod"},
		model.ServiceInstallation{ServiceType: "deploy", Status: "running", Namespace: "billing-prod-deploy"},
		nil,
	)

	var action ToolWorkspaceAction
	for _, candidate := range workspace.Actions {
		if candidate.Key == "apply_argocd_applicationset" {
			action = candidate
			break
		}
	}
	if action.Key == "" {
		t.Fatalf("missing apply_argocd_applicationset action: %#v", workspace.Actions)
	}
	if !strings.Contains(action.Description, "Project 自动绑定") {
		t.Fatalf("action description must explain automatic project binding, got %q", action.Description)
	}
	for _, field := range action.Fields {
		if field.Name == "project" {
			t.Fatalf("project must not be user-fillable: %#v", action.Fields)
		}
	}
	assertActionFields(t, action, "name", "repoURL", "path", "targetRevision", "destinationNamespace")
}

func TestBuildToolWorkspaceReturnsMonitorCoverage(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}
	inst := model.ServiceInstallation{ServiceType: "monitor", Status: "running", Namespace: "billing-prod-monitor"}
	components := []model.Component{{Name: "api"}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	if workspace.Kind != "observability" {
		t.Fatalf("unexpected kind: %s", workspace.Kind)
	}
	types := resourceTypes(workspace.Resources)
	for _, wantType := range []string{"Monitor Subject", "Dashboard"} {
		if !types[wantType] {
			t.Fatalf("monitor workspace missing %s resource: %#v", wantType, workspace.Resources)
		}
	}
	for _, forbiddenType := range []string{"Alert", "Rule"} {
		if types[forbiddenType] {
			t.Fatalf("monitor workspace must not invent %s resources before live data is loaded: %#v", forbiddenType, workspace.Resources)
		}
	}
	dashboardUIDs := map[interface{}]bool{}
	for _, resource := range workspace.Resources {
		if resource.Type == "Dashboard" && resource.Annotations != nil {
			dashboardUIDs[resource.Annotations["dashboardUid"]] = true
		}
	}
	for _, wantUID := range []string{"paap-billing-prod-overview", "paap-pod-workload", "paap-tool-workload", "paap-middleware-workload"} {
		if !dashboardUIDs[wantUID] {
			t.Fatalf("missing built-in dashboard %s in %#v", wantUID, workspace.Resources)
		}
	}
	if workspace.Config[0].Label != "命名空间" || workspace.Config[0].Value != "billing-prod-monitor" {
		t.Fatalf("unexpected config: %#v", workspace.Config)
	}
}

func TestBuildToolWorkspaceExposesHealthActionsForOperationalTools(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}

	cases := map[string]string{
		"monitor":  "check_grafana_health",
		"log":      "check_loki_health",
		"ci":       "check_jenkins_health",
		"registry": "check_registry_health",
	}

	for serviceType, wantAction := range cases {
		workspace := BuildToolWorkspace(app, env, model.ServiceInstallation{ServiceType: serviceType, Namespace: "ns"}, nil)
		found := false
		for _, action := range workspace.Actions {
			if action.Key == wantAction {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s workspace missing action %s: %#v", serviceType, wantAction, workspace.Actions)
		}
	}
}

func TestBuildToolWorkspaceUsesJenkinsJobDirectoryURL(t *testing.T) {
	workspace := BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "dev"},
		model.ServiceInstallation{ID: 10, EnvironmentID: 1, ServiceType: "ci", Status: "running", Namespace: "billing-dev-ci"},
		[]model.Component{{Name: "api", JenkinsJob: "billing-dev-api-build"}},
	)

	if len(workspace.Resources) != 1 {
		t.Fatalf("expected one Jenkins job resource, got %#v", workspace.Resources)
	}
	if got, want := workspace.Resources[0].ExternalURL, "/api/v1/environments/1/services/10/proxy/job/billing-dev-api-build/"; got != want {
		t.Fatalf("Jenkins job URL = %q, want %q", got, want)
	}
}

func TestBuildToolWorkspaceExposesOperationalActions(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}

	cases := map[string]string{
		"monitor": "list_grafana_dashboards",
		"log":     "query_loki_streams",
		"ci":      "trigger_jenkins_build",
	}

	for serviceType, wantAction := range cases {
		workspace := BuildToolWorkspace(app, env, model.ServiceInstallation{ServiceType: serviceType, Namespace: "ns"}, nil)
		found := false
		for _, action := range workspace.Actions {
			if action.Key == wantAction {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s workspace missing action %s: %#v", serviceType, wantAction, workspace.Actions)
		}
	}
}

func TestBuildToolWorkspaceExposesDatabaseManagementActions(t *testing.T) {
	workspace := BuildToolWorkspace(model.Application{}, model.Environment{}, model.ServiceInstallation{ServiceType: "mysql", Namespace: "ns"}, nil)

	want := map[string]bool{
		"check_database_connection": false,
		"list_databases":            false,
	}
	for _, action := range workspace.Actions {
		if _, ok := want[action.Key]; ok {
			want[action.Key] = true
		}
	}
	for action, found := range want {
		if !found {
			t.Fatalf("mysql workspace missing action %s: %#v", action, workspace.Actions)
		}
	}
}

func TestBuildToolWorkspaceDoesNotExposeDemoPlaceholders(t *testing.T) {
	serviceTypes := []string{"mysql", "postgresql", "mongodb"}

	for _, serviceType := range serviceTypes {
		workspace := BuildToolWorkspace(model.Application{}, model.Environment{}, model.ServiceInstallation{ServiceType: serviceType, Namespace: "ns"}, nil)
		for _, action := range workspace.Actions {
			assertActionFieldsDoNotContain(t, action, "demo")
		}
	}
}

func TestBuildToolWorkspaceDoesNotInventDataResourcesBeforeLiveDataReturns(t *testing.T) {
	serviceTypes := []string{"mysql", "postgresql", "redis", "mongodb", "rabbitmq", "kafka", "minio"}

	for _, serviceType := range serviceTypes {
		workspace := BuildToolWorkspace(model.Application{}, model.Environment{}, model.ServiceInstallation{
			ServiceType: serviceType,
			Status:      "running",
			Namespace:   "billing-dev-" + serviceType,
			ReleaseName: "billing-dev-" + serviceType,
		}, nil)
		if len(workspace.Resources) != 1 || workspace.Resources[0].Type != "Connection" {
			t.Fatalf("%s workspace must only expose the real connection resource before live data is loaded, got %#v", serviceType, workspace.Resources)
		}
		forbidden := map[string]bool{
			"appdb":               true,
			"users":               true,
			"session:*":           true,
			"jobs":                true,
			"amq.topic":           true,
			"events":              true,
			"artifacts":           true,
			"releases/app.tar.gz": true,
		}
		for _, resource := range workspace.Resources {
			if forbidden[resource.Name] {
				t.Fatalf("%s workspace exposed synthetic resource %#v", serviceType, resource)
			}
		}
	}
}

func TestBuildToolWorkspaceReturnsGiteaFallbackWithoutSyntheticFileTree(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "dev"}
	inst := model.ServiceInstallation{ServiceType: "git", Status: "running", Namespace: "billing-dev-git"}
	components := []model.Component{{
		Name:         "api",
		Type:         "backend",
		GitRepoURL:   "http://gitea/paap/billing-dev-components.git",
		GitPath:      "components/api",
		SourceBranch: "develop",
	}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	repo := workspace.Resources[0]
	if repo.Type != "Repository" {
		t.Fatalf("expected Repository resource, got %#v", repo)
	}
	assertAnnotation(t, repo, "branch", "develop")
	assertAnnotation(t, repo, "private", true)
	assertAnnotation(t, repo, "language", "Container/Kubernetes")
	if len(repo.Children) != 0 {
		t.Fatalf("fallback repository must not expose synthetic file tree children: %#v", repo.Children)
	}
}

func TestBuildToolWorkspaceDoesNotInventArgoCDTopologyChildren(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}
	inst := model.ServiceInstallation{ServiceType: "deploy", Status: "running", Namespace: "billing-prod-deploy"}
	components := []model.Component{{
		Name:       "api",
		Type:       "backend",
		Status:     "running",
		GitRepoURL: "http://gitea/paap/billing-prod-components.git",
		GitPath:    "components/api",
		ArgoCDApp:  "billing-prod-api",
		Replicas:   2,
	}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	appResource := workspace.Resources[0]
	assertAnnotation(t, appResource, "syncStatus", "Synced")
	assertAnnotation(t, appResource, "health", "Healthy")
	assertAnnotation(t, appResource, "repoURL", "http://gitea/paap/billing-prod-components.git")
	assertAnnotation(t, appResource, "path", "components/api")
	assertAnnotation(t, appResource, "namespace", "billing-prod")
	assertAnnotation(t, appResource, "server", "https://kubernetes.default.svc")
	if len(appResource.Children) != 0 {
		t.Fatalf("must not expose synthetic ArgoCD topology children before live data is loaded, got %#v", appResource.Children)
	}
}

func TestBuildToolWorkspaceReturnsMonitorOperationalResources(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}
	inst := model.ServiceInstallation{ID: 9, EnvironmentID: 3, ServiceType: "monitor", Status: "running", Namespace: "billing-prod-monitor"}
	components := []model.Component{{Name: "api", Type: "backend", Status: "running"}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	types := resourceTypes(workspace.Resources)
	for _, wantType := range []string{"Monitor Subject", "Dashboard"} {
		if !types[wantType] {
			t.Fatalf("monitor workspace missing %s resource: %#v", wantType, workspace.Resources)
		}
	}
	for _, forbiddenType := range []string{"Alert", "Rule"} {
		if types[forbiddenType] {
			t.Fatalf("monitor workspace must not invent %s resources before live data is loaded: %#v", forbiddenType, workspace.Resources)
		}
	}
	subjects := 0
	var componentSubject ToolWorkspaceResource
	for _, resource := range workspace.Resources {
		if resource.Type == "Monitor Subject" {
			subjects++
			if resource.Name == "api" {
				componentSubject = resource
			}
		}
	}
	if subjects < 2 {
		t.Fatalf("expected environment and component monitor subjects, got %#v", workspace.Resources)
	}
	assertAnnotation(t, componentSubject, "dashboardUid", "paap-pod-workload")
	assertAnnotation(t, componentSubject, "dashboardPath", "/d/paap-pod-workload")
	assertAnnotation(t, componentSubject, "logQuery", `{namespace="billing-prod", pod=~"api.*"}`)
	var dashboard ToolWorkspaceResource
	for _, resource := range workspace.Resources {
		if resource.Type == "Dashboard" {
			dashboard = resource
			break
		}
	}
	if dashboard.ExternalURL != "/api/v1/environments/3/services/9/proxy/d/paap-billing-prod-overview" {
		t.Fatalf("dashboard external URL = %q", dashboard.ExternalURL)
	}
	dashboardUIDs := map[interface{}]bool{}
	for _, resource := range workspace.Resources {
		if resource.Type == "Dashboard" && resource.Annotations != nil {
			dashboardUIDs[resource.Annotations["dashboardUid"]] = true
		}
	}
	for _, wantUID := range []string{"paap-billing-prod-overview", "paap-pod-workload", "paap-tool-workload", "paap-middleware-workload"} {
		if !dashboardUIDs[wantUID] {
			t.Fatalf("missing built-in dashboard %s in %#v", wantUID, workspace.Resources)
		}
	}
}

func TestBuildToolWorkspaceReturnsRegistryImageReferenceWithoutFakeArtifactMetadata(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{app}-{env}.example.com:5443")
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}
	inst := model.ServiceInstallation{ID: 7, EnvironmentID: 3, ServiceType: "registry", Status: "running", Namespace: "billing-prod-registry"}
	components := []model.Component{{
		Name:          "api",
		Type:          "backend",
		Version:       "v1.2.3",
		RegistryImage: "registry.billing-prod.example.com:5443/billing-prod/api:v1.2.3",
	}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	repo := workspace.Resources[0]
	assertAnnotation(t, repo, "project", "billing-prod")
	tags, ok := repo.Annotations["tags"].([]string)
	if !ok || len(tags) != 1 || tags[0] != "v1.2.3" {
		t.Fatalf("expected tags annotation with v1.2.3, got %#v", repo.Annotations)
	}
	for _, forbidden := range []string{"artifactCount", "digest"} {
		if _, ok := repo.Annotations[forbidden]; ok {
			t.Fatalf("registry resource must not expose synthetic %s annotation: %#v", forbidden, repo.Annotations)
		}
	}
	if repo.ExternalURL == "" {
		t.Fatalf("expected registry repository external URL")
	}
	if repo.ExternalURL != "/api/v1/environments/3/services/7/proxy/v2/billing-prod/api/tags/list" {
		t.Fatalf("registry external URL = %q", repo.ExternalURL)
	}
	foundTrust := false
	for _, resource := range workspace.Resources {
		if resource.Type == "Runtime Trust" {
			foundTrust = true
			assertAnnotation(t, resource, "registryHost", "registry.billing-prod.example.com:5443")
			assertAnnotation(t, resource, "registryEndpoint", "https://registry.billing-prod.example.com:5443")
			assertAnnotation(t, resource, "certificateURL", "/api/v1/environments/3/services/7/registry-ca.crt")
			if resource.Status != "Action Required" {
				t.Fatalf("expected runtime trust action required, got %#v", resource)
			}
		}
	}
	if !foundTrust {
		t.Fatalf("expected Runtime Trust resource, got %#v", workspace.Resources)
	}
}

func TestBuildToolWorkspaceReturnsJenkinsJobMetadataFallback(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}
	inst := model.ServiceInstallation{ID: 10, EnvironmentID: 3, ServiceType: "ci", Status: "running", Namespace: "billing-prod-ci"}
	components := []model.Component{{
		Name:           "api",
		Type:           "backend",
		JenkinsJob:     "billing-prod-api-build",
		PipelineStatus: "running",
	}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	job := workspace.Resources[0]
	if job.Type != "Job" {
		t.Fatalf("expected Jenkins Job fallback, got %#v", job)
	}
	assertAnnotation(t, job, "color", "blue_anime")
	assertAnnotation(t, job, "component", "api")
	if job.ExternalURL == "" {
		t.Fatalf("expected Jenkins job external URL")
	}
	if job.ExternalURL != "/api/v1/environments/3/services/10/proxy/job/billing-prod-api-build/" {
		t.Fatalf("jenkins job external URL = %q", job.ExternalURL)
	}
	if len(job.Actions) != 1 || job.Actions[0].Target != "billing-prod-api-build" {
		t.Fatalf("expected Jenkins trigger action to target the concrete job, got %#v", job.Actions)
	}
}

func TestBuildToolWorkspaceRecomputesLegacySourceRegistryImageForDisplay(t *testing.T) {
	app := model.Application{Identifier: "test"}
	env := model.Environment{Identifier: "staging"}
	inst := model.ServiceInstallation{ID: 6, EnvironmentID: 1, ServiceType: "registry", Status: "running", Namespace: "test-staging-registry"}
	components := []model.Component{{
		Name:          "source-smoke",
		Type:          "backend",
		Version:       "14",
		DeliveryMode:  "source",
		RegistryImage: "registry.paap.local:5000/test-staging/source-smoke:14",
	}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	assertAnnotation(t, workspace.Resources[0], "image", "registry.test-staging.paap.local:5000/test-staging/source-smoke:14")
}

func TestBuildToolWorkspaceDoesNotLinkExternalImagesToEnvironmentRegistry(t *testing.T) {
	app := model.Application{Identifier: "test"}
	env := model.Environment{Identifier: "staging"}
	inst := model.ServiceInstallation{ID: 6, EnvironmentID: 1, ServiceType: "registry", Status: "running", Namespace: "test-staging-registry"}
	components := []model.Component{{
		Name:          "orders",
		Type:          "backend",
		Version:       "2.8.3",
		DeliveryMode:  "image",
		RegistryImage: "registry:2.8.3",
	}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	if workspace.Resources[0].Type != "External Image" {
		t.Fatalf("external image resource type = %q, want External Image", workspace.Resources[0].Type)
	}
	if workspace.Resources[0].ExternalURL != "" {
		t.Fatalf("external image should not link to environment registry, got %q", workspace.Resources[0].ExternalURL)
	}
	assertAnnotation(t, workspace.Resources[0], "image", "registry:2.8.3")
}

func TestBuildToolWorkspaceDoesNotExposeDraftLatestAsRegistryRepository(t *testing.T) {
	app := model.Application{Identifier: "test"}
	env := model.Environment{Identifier: "staging"}
	inst := model.ServiceInstallation{ID: 6, EnvironmentID: 1, ServiceType: "registry", Status: "running", Namespace: "test-staging-registry"}
	components := []model.Component{{
		Name:          "frontend-1",
		Type:          "frontend",
		Status:        "draft",
		Version:       "latest",
		RegistryImage: "registry.test-staging.paap.local:5000/test-staging/frontend-1:latest",
	}}

	workspace := BuildToolWorkspace(app, env, inst, components)

	for _, resource := range workspace.Resources {
		if resource.Type == "Image Repository" || resource.Type == "Harbor Repository" || resource.Type == "Repository" {
			t.Fatalf("draft/latest component must not create registry repository resource: %#v", resource)
		}
	}
	if len(workspace.Resources) != 1 || workspace.Resources[0].Type != "Runtime Trust" {
		t.Fatalf("expected only Runtime Trust resource, got %#v", workspace.Resources)
	}
}

func assertAnnotation(t *testing.T, resource ToolWorkspaceResource, key string, want interface{}) {
	t.Helper()
	if resource.Annotations == nil {
		t.Fatalf("resource %s has no annotations", resource.Name)
	}
	if got := resource.Annotations[key]; got != want {
		t.Fatalf("annotation %s = %#v, want %#v on %#v", key, got, want, resource)
	}
}

func assertActionFields(t *testing.T, action ToolWorkspaceAction, wantNames ...string) {
	t.Helper()
	got := map[string]bool{}
	for _, field := range action.Fields {
		got[field.Name] = true
	}
	for _, want := range wantNames {
		if !got[want] {
			t.Fatalf("action %s missing field %s: %#v", action.Key, want, action.Fields)
		}
	}
}

func assertActionFieldsDoNotContain(t *testing.T, action ToolWorkspaceAction, forbidden string) {
	t.Helper()
	for _, field := range action.Fields {
		if strings.Contains(strings.ToLower(field.Placeholder), strings.ToLower(forbidden)) ||
			strings.Contains(strings.ToLower(field.Default), strings.ToLower(forbidden)) {
			t.Fatalf("action %s field %s exposes forbidden placeholder/default %q: %#v", action.Key, field.Name, forbidden, field)
		}
	}
}

func workspaceConfigContains(config []ToolWorkspaceConfig, label, value string) bool {
	for _, item := range config {
		if item.Label == label && item.Value == value {
			return true
		}
	}
	return false
}

func resourceTypes(resources []ToolWorkspaceResource) map[string]bool {
	types := map[string]bool{}
	for _, resource := range resources {
		types[resource.Type] = true
	}
	return types
}
