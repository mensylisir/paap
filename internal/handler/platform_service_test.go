package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	if err := db.Create(&model.ServiceTemplate{
		Type:              "postgresql",
		Name:              "PostgreSQL",
		Category:          "infra",
		Installer:         "helm",
		AppVersion:        "15.4.0",
		SupportedFeatures: features,
		Enabled:           true,
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
	managed := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "postgresql", ServiceName: "billing-postgresql", Status: "running", Namespace: "billing-dev-postgresql"}
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
	wantMonitoringURL := "/api/v1/environments/" + strconv.FormatUint(uint64(fixture.Env.ID), 10) + "/services/" + strconv.FormatUint(uint64(fixture.Monitor.ID), 10) + "/proxy/d/paap-middleware-workload?orgId=1&theme=light&var-namespace=billing-dev-postgresql"
	if managed.MonitoringURL != wantMonitoringURL {
		t.Fatalf("managed monitoring URL = %q, want %q", managed.MonitoringURL, wantMonitoringURL)
	}
	kubeVirt := byID["managed:"+strconv.FormatUint(uint64(fixture.KubeVirt.ID), 10)]
	if kubeVirt.Source != model.CapabilitySourceManaged || kubeVirt.ProvisionMode != model.ServiceProvisionModeKubeVirt || kubeVirt.ServiceType != "postgresql" {
		t.Fatalf("unexpected kubevirt instance: %#v", kubeVirt)
	}
	shared := byID["managed:"+strconv.FormatUint(uint64(fixture.SharedService.ID), 10)]
	if shared.UsageCount != 2 || shared.ApplicationName != "Default" {
		t.Fatalf("unexpected shared managed instance usage: %#v", shared)
	}
	external := byID["capability:"+strconv.FormatUint(uint64(fixture.ExternalCap.ID), 10)]
	if external.Source != model.CapabilitySourceExternal || external.Endpoint != "postgres.example:5432" || external.UsageCount != 1 {
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
}
