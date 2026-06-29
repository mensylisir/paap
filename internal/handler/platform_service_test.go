package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"paap/internal/database"
	"paap/internal/middleware"
	"paap/internal/model"
	"paap/internal/permission"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type platformServiceUsageFixture struct {
	Features      string
	App           model.Application
	Env           model.Environment
	SharedApp     model.Application
	SharedEnv     model.Environment
	Managed       model.ServiceInstallation
	KubeVirt      model.ServiceInstallation
	Monitor       model.ServiceInstallation
	SharedService model.ServiceInstallation
	SharedCap     model.EnvironmentCapability
	ExternalCap   model.EnvironmentCapability
	Component     model.Component
}

func openPlatformServiceUsageTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Application{},
		&model.Environment{},
		&model.ServiceTemplate{},
		&model.ServiceCatalog{},
		&model.ServiceInstallation{},
		&model.EnvironmentCapability{},
		&model.Component{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedPlatformServiceUsageFixture(t *testing.T, db *gorm.DB) platformServiceUsageFixture {
	t.Helper()
	features := serviceFeatureMatrixJSON("postgresql", "infra")
	manifest := model.PlatformManifest{
		Name:        "postgresql",
		Version:     "15.4.0",
		Description: "fixture postgres manifest",
		Catalog: &model.CatalogSpec{
			Docs: model.CatalogDocsSpec{
				Overview:   "# PostgreSQL Fixture\n\n模板提供的服务介绍。",
				Install:    "## 安装\n\n模板提供的安装说明。",
				Quickstart: "## Quick Start\n\n模板提供的接入说明。",
			},
			Architecture: []model.CatalogArchitectureSpec{
				{ID: "primary", Type: "postgres-primary", Label: "Primary"},
				{ID: "replica", Type: "postgres-replica", Label: "Replica"},
			},
		},
		Observability: &model.ObservabilitySpec{
			DashboardUID:     "fixture-postgres-dashboard",
			DashboardTitle:   "Fixture PostgreSQL 大盘",
			LogQueryTemplate: `{namespace="$namespace"} |~ "(?i)(fixture-postgres-error|slow)"`,
			MetricCards: []model.ObservabilityMetricCardSpec{
				{Key: "fixture_connections", Title: "模板连接数", Unit: "count", Description: "模板声明的连接数", PromQL: `sum(fixture_pg_connections{namespace="$namespace"})`},
				{Key: "fixture_slow", Title: "模板慢查询", Unit: "qps", Description: "模板声明的慢查询", PromQL: `sum(rate(fixture_pg_slow_queries{namespace="$namespace"}[5m]))`},
			},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	kubeVirtManifest := model.PlatformManifest{
		Name:        "postgresql",
		Version:     "kubevirt-postgresql-16",
		Description: "fixture kubevirt postgres manifest",
		Catalog: &model.CatalogSpec{
			Docs: model.CatalogDocsSpec{
				Overview:   "# PostgreSQL KubeVirt Fixture\n\n不应该覆盖普通 PostgreSQL 服务介绍。",
				Install:    "## KubeVirt 安装\n\n不应该覆盖普通 PostgreSQL 安装说明。",
				Quickstart: "## KubeVirt Quick Start\n\n不应该覆盖普通 PostgreSQL 接入说明。",
			},
		},
	}
	kubeVirtManifestJSON, err := json.Marshal(kubeVirtManifest)
	if err != nil {
		t.Fatalf("marshal kubevirt manifest: %v", err)
	}
	if err := db.Create(&model.ServiceTemplate{
		Type:                 "postgresql",
		Name:                 "PostgreSQL KubeVirt",
		Category:             "database",
		Installer:            "raw-yaml",
		ProvisionMode:        model.ServiceProvisionModeKubeVirt,
		RuntimeSpec:          `{"image":"docker.io/library/postgres:16","ports":[{"name":"postgresql","port":5432}]}`,
		SupportedFeatures:    serviceFeatureMatrixJSON("postgresql", "database"),
		PlatformManifestJSON: string(kubeVirtManifestJSON),
		Enabled:              true,
	}).Error; err != nil {
		t.Fatalf("seed kubevirt template: %v", err)
	}
	if err := db.Create(&model.ServiceTemplate{
		Type:                 "postgresql",
		Name:                 "PostgreSQL",
		Category:             "infra",
		Installer:            "helm",
		AppVersion:           "15.4.0",
		SupportedFeatures:    features,
		PlatformManifestJSON: string(manifestJSON),
		Enabled:              true,
	}).Error; err != nil {
		t.Fatalf("seed template: %v", err)
	}
	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("seed env: %v", err)
	}
	sharedApp := model.Application{Name: "Default", Identifier: "default", OwnerID: 1, IsSystem: true}
	if err := db.Create(&sharedApp).Error; err != nil {
		t.Fatalf("seed shared app: %v", err)
	}
	sharedEnv := model.Environment{ApplicationID: sharedApp.ID, Name: "Shared", Identifier: "shared", Namespace: "default-shared", IsSystem: true}
	if err := db.Create(&sharedEnv).Error; err != nil {
		t.Fatalf("seed shared env: %v", err)
	}
	managed := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "postgresql",
		ServiceName:   "billing-postgresql",
		Status:        "running",
		Namespace:     "billing-dev-postgresql",
		Values: `primary:
  resources:
    requests:
      cpu: 750m
      memory: 1536Mi
  persistence:
    size: 30Gi
`,
	}
	if err := db.Create(&managed).Error; err != nil {
		t.Fatalf("seed managed service: %v", err)
	}
	kubeVirt := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "postgresql",
		ServiceName:   "billing-postgresql-vm",
		Status:        "running",
		Namespace:     "billing-dev-postgresql-vm",
		ProvisionMode: model.ServiceProvisionModeKubeVirt,
		RuntimeSpec:   `{"kind":"VirtualMachine","template":"postgresql-vm-template"}`,
	}
	if err := db.Create(&kubeVirt).Error; err != nil {
		t.Fatalf("seed kubevirt service: %v", err)
	}
	monitor := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "monitor", ServiceName: "billing-monitor", Status: "running", Namespace: "billing-dev-monitor"}
	if err := db.Create(&monitor).Error; err != nil {
		t.Fatalf("seed monitor service: %v", err)
	}
	sharedService := model.ServiceInstallation{EnvironmentID: sharedEnv.ID, ServiceType: "postgresql", ServiceName: "shared-postgresql", Status: "running", Namespace: "default-shared-postgresql"}
	if err := db.Create(&sharedService).Error; err != nil {
		t.Fatalf("seed shared service: %v", err)
	}
	sharedCap := model.EnvironmentCapability{EnvironmentID: env.ID, Capability: "database", Source: model.CapabilitySourceShared, ServiceType: "postgresql", RefServiceID: &sharedService.ID, ValidationStatus: "linked"}
	if err := db.Create(&sharedCap).Error; err != nil {
		t.Fatalf("seed shared capability: %v", err)
	}
	externalCap := model.EnvironmentCapability{EnvironmentID: env.ID, Capability: "database", Source: model.CapabilitySourceExternal, ServiceType: "postgresql", Provider: "external-postgres", ExternalEndpoint: "postgres.example:5432", ValidationStatus: "pending"}
	if err := db.Create(&externalCap).Error; err != nil {
		t.Fatalf("seed external capability: %v", err)
	}
	componentConfig, err := model.ComponentConfig{
		Bindings: []model.ComponentBinding{
			{TargetKey: "service:" + strconv.FormatUint(uint64(managed.ID), 10), TargetKind: "service", TargetName: managed.ServiceName, TargetType: "postgresql", Role: "database"},
			{TargetKey: "capability:" + strconv.FormatUint(uint64(sharedCap.ID), 10), TargetKind: "capability", TargetName: sharedCap.CapabilityKey, TargetType: "postgresql", Role: "database"},
			{TargetKey: "capability:" + strconv.FormatUint(uint64(externalCap.ID), 10), TargetKind: "capability", TargetName: externalCap.CapabilityKey, TargetType: "postgresql", Role: "database"},
		},
	}.JSON()
	if err != nil {
		t.Fatalf("component config: %v", err)
	}
	component := model.Component{EnvironmentID: env.ID, Name: "billing-api", Type: "backend", Config: componentConfig}
	if err := db.Create(&component).Error; err != nil {
		t.Fatalf("seed component: %v", err)
	}
	return platformServiceUsageFixture{
		Features:      features,
		App:           app,
		Env:           env,
		SharedApp:     sharedApp,
		SharedEnv:     sharedEnv,
		Managed:       managed,
		KubeVirt:      kubeVirt,
		Monitor:       monitor,
		SharedService: sharedService,
		SharedCap:     sharedCap,
		ExternalCap:   externalCap,
		Component:     component,
	}
}

func TestBuildPlatformServiceStatsAggregatesInstallationsAndCapabilities(t *testing.T) {
	db := openPlatformServiceUsageTestDB(t)
	fixture := seedPlatformServiceUsageFixture(t, db)

	stats, err := service.ListPlatformServiceStats(db)
	if err != nil {
		t.Fatalf("build stats: %v", err)
	}
	byType := map[string]service.PlatformServiceStat{}
	for _, item := range stats {
		byType[item.Type] = item
	}
	postgres := byType["postgresql"]
	if postgres.ManagedInstances != 2 || postgres.RunningInstances != 3 {
		t.Fatalf("managed/running stats = %d/%d, want 2/3", postgres.ManagedInstances, postgres.RunningInstances)
	}
	if postgres.KubeVirtInstances != 1 {
		t.Fatalf("kubevirt stats = %d, want 1", postgres.KubeVirtInstances)
	}
	if postgres.SharedReferences != 1 || postgres.ExternalConnections != 1 {
		t.Fatalf("shared/external stats = %d/%d, want 1/1", postgres.SharedReferences, postgres.ExternalConnections)
	}
	if postgres.ApplicationCount != 2 || postgres.EnvironmentCount != 2 {
		t.Fatalf("app/env counts = %d/%d, want 2/2", postgres.ApplicationCount, postgres.EnvironmentCount)
	}
	if postgres.Features != fixture.Features {
		t.Fatalf("features = %q, want %q", postgres.Features, fixture.Features)
	}
}

func TestListCatalogServiceProductsReturnsProductStatsAndEnvironmentTemplates(t *testing.T) {
	db := openPlatformServiceUsageTestDB(t)
	fixture := seedPlatformServiceUsageFixture(t, db)
	envTemplate := model.EnvTemplate{
		Name:         "轻量开发环境",
		Description:  "基础环境服务模板",
		ResourceCPU:  "2核",
		ResourceMem:  "4GB",
		ResourceDisk: "20GB",
	}
	if err := db.Create(&envTemplate).Error; err != nil {
		t.Fatalf("seed env template: %v", err)
	}

	products, err := service.ListCatalogServiceProducts(db)
	if err != nil {
		t.Fatalf("catalog service products: %v", err)
	}
	byType := map[string]service.CatalogServiceProduct{}
	for _, item := range products {
		byType[item.Type] = item
	}
	postgres := byType["postgresql"]
	if postgres.Name != "PostgreSQL" || postgres.Features != fixture.Features {
		t.Fatalf("unexpected postgres product identity: %#v", postgres)
	}
	if len(postgres.Versions) != 1 || postgres.Versions[0] != "15.4.0" {
		t.Fatalf("postgres versions = %#v, want [15.4.0]", postgres.Versions)
	}
	if postgres.ManagedInstances != 2 || postgres.KubeVirtInstances != 1 || postgres.PublicInstances != 1 {
		t.Fatalf("managed/kubevirt/public = %d/%d/%d, want 2/1/1", postgres.ManagedInstances, postgres.KubeVirtInstances, postgres.PublicInstances)
	}
	if postgres.SharedReferences != 1 || postgres.ExternalConnections != 1 || postgres.RunningInstances != 3 {
		t.Fatalf("shared/external/running = %d/%d/%d, want 1/1/3", postgres.SharedReferences, postgres.ExternalConnections, postgres.RunningInstances)
	}
	environment := byType["environment:"+strconv.FormatUint(uint64(envTemplate.ID), 10)]
	if environment.Category != "environment" || environment.Name != "轻量开发环境" || environment.Description != "基础环境服务模板" {
		t.Fatalf("unexpected environment product: %#v", environment)
	}
}

func TestBuildPlatformServiceInstancesReturnsManagedAndExternalInstances(t *testing.T) {
	db := openPlatformServiceUsageTestDB(t)
	fixture := seedPlatformServiceUsageFixture(t, db)

	instances, err := service.ListPlatformServiceInstances(db, "postgresql")
	if err != nil {
		t.Fatalf("build instances: %v", err)
	}
	byID := map[string]service.PlatformServiceInstance{}
	for _, item := range instances {
		byID[item.ID] = item
	}
	managed := byID["managed:"+strconv.FormatUint(uint64(fixture.Managed.ID), 10)]
	if managed.Source != model.CapabilitySourceManaged || managed.ProvisionMode != model.ServiceProvisionModeManaged || managed.ApplicationName != "Billing" || managed.MonitoringTarget != "namespace:billing-dev-postgresql" {
		t.Fatalf("unexpected managed instance: %#v", managed)
	}
	wantMonitoringURL := "/api/v1/environments/" + strconv.FormatUint(uint64(fixture.Env.ID), 10) + "/services/" + strconv.FormatUint(uint64(fixture.Monitor.ID), 10) + "/proxy/d/fixture-postgres-dashboard?orgId=1&theme=light&var-namespace=billing-dev-postgresql"
	if managed.MonitoringURL != wantMonitoringURL {
		t.Fatalf("managed monitoring URL = %q, want %q", managed.MonitoringURL, wantMonitoringURL)
	}
	if !strings.Contains(managed.ErrorLogsURL, "/explore?") || !strings.Contains(managed.ErrorLogsURL, "billing-dev-postgresql") {
		t.Fatalf("managed error logs URL = %q, want grafana explore URL scoped to namespace", managed.ErrorLogsURL)
	}
	kubeVirt := byID["managed:"+strconv.FormatUint(uint64(fixture.KubeVirt.ID), 10)]
	if kubeVirt.Source != model.CapabilitySourceManaged || kubeVirt.ProvisionMode != model.ServiceProvisionModeKubeVirt || kubeVirt.ServiceType != "postgresql" {
		t.Fatalf("unexpected kubevirt instance: %#v", kubeVirt)
	}
	shared := byID["managed:"+strconv.FormatUint(uint64(fixture.SharedService.ID), 10)]
	if shared.UsageCount != 3 || shared.ApplicationName != "Default" {
		t.Fatalf("unexpected shared managed instance usage: %#v", shared)
	}
	external := byID["capability:"+strconv.FormatUint(uint64(fixture.ExternalCap.ID), 10)]
	if external.Source != model.CapabilitySourceExternal || external.Endpoint != "postgres.example:5432" || external.UsageCount != 2 {
		t.Fatalf("unexpected external instance: %#v", external)
	}
	if _, ok := byID["capability:"+strconv.FormatUint(uint64(fixture.SharedCap.ID), 10)]; ok {
		t.Fatalf("shared capability reference must not be duplicated as a service instance")
	}
}

func TestBuildPlatformServiceUsageReturnsSharedAndExternalRelations(t *testing.T) {
	db := openPlatformServiceUsageTestDB(t)
	fixture := seedPlatformServiceUsageFixture(t, db)

	usage, err := service.ListPlatformServiceUsage(db, "postgresql")
	if err != nil {
		t.Fatalf("build usage: %v", err)
	}
	byID := map[string]service.PlatformServiceUsage{}
	for _, item := range usage {
		byID[item.ID] = item
	}
	shared := byID["capability:"+strconv.FormatUint(uint64(fixture.SharedCap.ID), 10)]
	if shared.Source != model.CapabilitySourceShared || shared.RefServiceID != fixture.SharedService.ID || shared.ServiceInstanceID != "managed:"+strconv.FormatUint(uint64(fixture.SharedService.ID), 10) {
		t.Fatalf("unexpected shared usage: %#v", shared)
	}
	external := byID["capability:"+strconv.FormatUint(uint64(fixture.ExternalCap.ID), 10)]
	if external.Source != model.CapabilitySourceExternal || external.Endpoint != "postgres.example:5432" || external.ServiceInstanceID != "capability:"+strconv.FormatUint(uint64(fixture.ExternalCap.ID), 10) {
		t.Fatalf("unexpected external usage: %#v", external)
	}
	managed := byID["managed:"+strconv.FormatUint(uint64(fixture.Managed.ID), 10)]
	if managed.Source != model.CapabilitySourceManaged || managed.ApplicationName != "Billing" {
		t.Fatalf("unexpected managed usage: %#v", managed)
	}
	kubeVirt := byID["managed:"+strconv.FormatUint(uint64(fixture.KubeVirt.ID), 10)]
	if kubeVirt.ProvisionMode != model.ServiceProvisionModeKubeVirt || kubeVirt.ServiceType != "postgresql" {
		t.Fatalf("unexpected kubevirt usage: %#v", kubeVirt)
	}
	componentManaged := byID["component:"+strconv.FormatUint(uint64(fixture.Component.ID), 10)+":binding:0:service-"+strconv.FormatUint(uint64(fixture.Managed.ID), 10)]
	if componentManaged.ComponentName != "billing-api" || componentManaged.ServiceInstanceID != "managed:"+strconv.FormatUint(uint64(fixture.Managed.ID), 10) || componentManaged.Source != model.CapabilitySourceManaged {
		t.Fatalf("unexpected component managed usage: %#v", componentManaged)
	}
	componentShared := byID["component:"+strconv.FormatUint(uint64(fixture.Component.ID), 10)+":binding:1:capability-"+strconv.FormatUint(uint64(fixture.SharedCap.ID), 10)]
	if componentShared.ComponentID != fixture.Component.ID || componentShared.Source != model.CapabilitySourceShared || componentShared.RefServiceID != fixture.SharedService.ID {
		t.Fatalf("unexpected component shared usage: %#v", componentShared)
	}
	componentExternal := byID["component:"+strconv.FormatUint(uint64(fixture.Component.ID), 10)+":binding:2:capability-"+strconv.FormatUint(uint64(fixture.ExternalCap.ID), 10)]
	if componentExternal.ComponentType != "backend" || componentExternal.Source != model.CapabilitySourceExternal || componentExternal.Endpoint != "postgres.example:5432" {
		t.Fatalf("unexpected component external usage: %#v", componentExternal)
	}
}

func TestCatalogServiceDetailReturnsDocsAndInstallMethods(t *testing.T) {
	db := openPlatformServiceUsageTestDB(t)
	fixture := seedPlatformServiceUsageFixture(t, db)

	detail, err := service.GetCatalogServiceDetail(db, "postgresql")
	if err != nil {
		t.Fatalf("catalog service detail: %v", err)
	}
	if detail.Product.Name != "PostgreSQL" || detail.Product.Features != fixture.Features {
		t.Fatalf("unexpected detail product: %#v", detail.Product)
	}
	if !strings.Contains(detail.Docs.Overview.Markdown, "模板提供的服务介绍") {
		t.Fatalf("overview markdown = %q, want template-provided overview", detail.Docs.Overview.Markdown)
	}
	if !strings.Contains(detail.Docs.Install.Markdown, "模板提供的安装说明") || !strings.Contains(detail.Docs.Quickstart.Markdown, "模板提供的接入说明") {
		t.Fatalf("docs = %#v, want template-provided install and quickstart", detail.Docs)
	}
	byKey := map[string]service.CatalogInstallPath{}
	for _, item := range detail.InstallMethods {
		byKey[item.Key] = item
	}
	if !byKey["managed"].Enabled || !byKey["shared"].Enabled || !byKey["external"].Enabled || !byKey["kubevirt"].Enabled {
		t.Fatalf("unexpected install methods: %#v", detail.InstallMethods)
	}
}

func TestCatalogServiceResourcesTopologyAndObservability(t *testing.T) {
	db := openPlatformServiceUsageTestDB(t)
	fixture := seedPlatformServiceUsageFixture(t, db)

	resources, err := service.GetCatalogServiceResources(db, "postgresql")
	if err != nil {
		t.Fatalf("catalog service resources: %v", err)
	}
	if resources.Total.Instances != 4 || resources.Total.RunningInstances != 3 {
		t.Fatalf("resource total = %#v, want 4 instances and 3 running", resources.Total)
	}
	if resources.Total.CPURequestMillicores < 750 || resources.Total.StorageRequestBytes < 30*1024*1024*1024 {
		t.Fatalf("resource total = %#v, want install-values footprint included", resources.Total)
	}
	if len(resources.Groups) == 0 || len(resources.Instances) == 0 {
		t.Fatalf("resources missing groups or instances: %#v", resources)
	}
	foundInstallValues := false
	for _, item := range resources.Instances {
		if item.ID == "managed:"+strconv.FormatUint(uint64(fixture.Managed.ID), 10) && item.SnapshotSource == "install-values" {
			foundInstallValues = true
			break
		}
	}
	if !foundInstallValues {
		t.Fatalf("resources instances = %#v, want managed instance sourced from install values", resources.Instances)
	}

	topology, err := service.GetCatalogServiceTopology(db, "postgresql")
	if err != nil {
		t.Fatalf("catalog service topology: %v", err)
	}
	nodeIDs := map[string]struct{}{}
	for _, node := range topology.Nodes {
		nodeIDs[node.ID] = struct{}{}
	}
	if _, ok := nodeIDs["service:postgresql"]; !ok {
		t.Fatalf("topology missing product node: %#v", topology.Nodes)
	}
	if _, ok := nodeIDs["component:"+strconv.FormatUint(uint64(fixture.Component.ID), 10)]; ok {
		t.Fatalf("topology should not include application dependency nodes: %#v", topology.Nodes)
	}
	if _, ok := nodeIDs["instance:managed:"+strconv.FormatUint(uint64(fixture.Managed.ID), 10)+":primary"]; !ok {
		t.Fatalf("topology missing template architecture primary node: %#v", topology.Nodes)
	}
	if _, ok := nodeIDs["instance:managed:"+strconv.FormatUint(uint64(fixture.Managed.ID), 10)+":replica"]; !ok {
		t.Fatalf("topology missing template architecture replica node: %#v", topology.Nodes)
	}
	for _, edge := range topology.Edges {
		if edge.Type == "uses" {
			t.Fatalf("topology should only describe service-internal topology, got application uses edge: %#v", edge)
		}
	}

	observability, err := service.GetCatalogServiceObservability(db, "postgresql")
	if err != nil {
		t.Fatalf("catalog service observability: %v", err)
	}
	if observability.DashboardUID != "fixture-postgres-dashboard" {
		t.Fatalf("dashboard uid = %q, want fixture-postgres-dashboard", observability.DashboardUID)
	}
	if len(observability.MetricCards) == 0 || observability.MetricCards[1].Title != "模板慢查询" {
		t.Fatalf("metric cards = %#v, want template-provided metric cards", observability.MetricCards)
	}
	foundManaged := false
	for _, item := range observability.Instances {
		if item.InstanceID == "managed:"+strconv.FormatUint(uint64(fixture.Managed.ID), 10) {
			foundManaged = strings.Contains(item.DashboardURL, "fixture-postgres-dashboard") && strings.Contains(item.ErrorLogsURL, "fixture-postgres-error")
			break
		}
	}
	if !foundManaged {
		t.Fatalf("observability instances = %#v, want managed instance dashboard and error log URLs", observability.Instances)
	}
}

func TestPlatformServiceAPIsRequirePlatformPermission(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db := openPlatformServiceUsageTestDB(t)
	seedApplicationRBACForTest(t, db)
	admin := model.User{Username: "admin", Password: "x"}
	normal := model.User{Username: "normal", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := db.Create(&normal).Error; err != nil {
		t.Fatalf("create normal: %v", err)
	}
	bindSystemRoleForTest(t, db, admin.ID, model.RolePlatformAdmin)
	bindSystemRoleForTest(t, db, normal.ID, model.RoleUser)
	seedPlatformServiceUsageFixture(t, db)
	database.DB = db

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/platform/services/stats", withTestAuthUserContext(normal.ID), middleware.RequireSystemPermission(permission.SystemSharedPoolManage), ListPlatformServiceStats)
	router.GET("/api/v1/platform/services/:type/instances", withTestAuthUserContext(admin.ID), middleware.RequireSystemPermission(permission.SystemSharedPoolManage), ListPlatformServiceInstances)

	forbidden := httptest.NewRecorder()
	router.ServeHTTP(forbidden, httptest.NewRequest(http.MethodGet, "/api/v1/platform/services/stats", nil))
	if forbidden.Code != http.StatusForbidden {
		t.Fatalf("normal user status = %d, want 403; body=%s", forbidden.Code, forbidden.Body.String())
	}

	ok := httptest.NewRecorder()
	router.ServeHTTP(ok, httptest.NewRequest(http.MethodGet, "/api/v1/platform/services/postgresql/instances", nil))
	if ok.Code != http.StatusOK {
		t.Fatalf("admin status = %d, want %d; body=%s", ok.Code, http.StatusOK, ok.Body.String())
	}
	var response struct {
		Data []service.PlatformServiceInstance `json:"data"`
	}
	if err := json.Unmarshal(ok.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode instances response: %v", err)
	}
	if len(response.Data) == 0 {
		t.Fatalf("instances response is empty")
	}
}

func TestCatalogServicesAPIAllowsAuthenticatedUsers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db := openPlatformServiceUsageTestDB(t)
	user := model.User{Username: "normal", Password: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	seedPlatformServiceUsageFixture(t, db)
	database.DB = db

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/catalog/services", withTestAuthUserContext(user.ID), ListCatalogServices)
	router.GET("/api/v1/catalog/services/:type/detail", withTestAuthUserContext(user.ID), GetCatalogServiceDetail)

	ok := httptest.NewRecorder()
	router.ServeHTTP(ok, httptest.NewRequest(http.MethodGet, "/api/v1/catalog/services", nil))
	if ok.Code != http.StatusOK {
		t.Fatalf("catalog services status = %d, want %d; body=%s", ok.Code, http.StatusOK, ok.Body.String())
	}
	var response struct {
		Data []service.CatalogServiceProduct `json:"data"`
	}
	if err := json.Unmarshal(ok.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode catalog services response: %v", err)
	}
	if len(response.Data) == 0 {
		t.Fatalf("catalog services response is empty")
	}

	detail := httptest.NewRecorder()
	router.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/v1/catalog/services/postgresql/detail", nil))
	if detail.Code != http.StatusOK {
		t.Fatalf("catalog service detail status = %d, want %d; body=%s", detail.Code, http.StatusOK, detail.Body.String())
	}
	var detailResponse struct {
		Data service.CatalogServiceDetail `json:"data"`
	}
	if err := json.Unmarshal(detail.Body.Bytes(), &detailResponse); err != nil {
		t.Fatalf("decode catalog service detail response: %v", err)
	}
	if detailResponse.Data.Product.Type != "postgresql" || detailResponse.Data.Docs.Quickstart.Markdown == "" {
		t.Fatalf("unexpected catalog service detail response: %#v", detailResponse.Data)
	}
	if strings.Contains(detailResponse.Data.Product.Description, "KubeVirt") {
		t.Fatalf("catalog service product should prefer managed template description, got %#v", detailResponse.Data.Product.Description)
	}
	if strings.Contains(detailResponse.Data.Docs.Overview.Markdown, "KubeVirt Fixture") {
		t.Fatalf("catalog service detail should prefer managed template docs, got %#v", detailResponse.Data.Docs.Overview.Markdown)
	}
}
