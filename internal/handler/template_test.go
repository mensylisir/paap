package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"paap/internal/database"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

func TestListServiceCatalogHidesUnsupportedPlaceholders(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceCatalog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	for _, item := range []model.ServiceCatalog{
		{Type: "postgresql", Name: "PostgreSQL", Category: "infra", Enabled: true},
		{Type: "kingbase", Name: "人大金仓", Category: "infra", Enabled: true},
		{Type: "nacos", Name: "Nacos", Category: "infra", Enabled: true},
		{Type: "eureka", Name: "Eureka", Category: "infra", Enabled: true},
		{Type: "disabled-demo", Name: "Disabled", Category: "infra", Enabled: false},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("seed %s: %v", item.Type, err)
		}
	}
	if err := db.Model(&model.ServiceCatalog{}).Where("type = ?", "disabled-demo").Update("enabled", false).Error; err != nil {
		t.Fatalf("disable demo catalog: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/catalog", ListServiceCatalog)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/catalog", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []model.ServiceCatalog `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	got := map[string]bool{}
	for _, item := range body.Data {
		got[item.Type] = true
	}
	for _, visible := range []string{"postgresql", "nacos", "eureka"} {
		if !got[visible] {
			t.Fatalf("expected %s in catalog, got %#v", visible, got)
		}
	}
	for _, hidden := range []string{"kingbase", "disabled-demo"} {
		if got[hidden] {
			t.Fatalf("%s should be hidden from service catalog, got %#v", hidden, got)
		}
	}
}

func TestBuiltInServiceTemplatesExposeFeatureMatrix(t *testing.T) {
	postgres, ok := builtInServiceTemplateByType("postgresql")
	if !ok {
		t.Fatalf("postgresql template not found")
	}
	git, ok := builtInServiceTemplateByType("git")
	if !ok {
		t.Fatalf("git template not found")
	}
	nacos, ok := builtInServiceTemplateByType("nacos")
	if !ok {
		t.Fatalf("nacos template not found")
	}
	eureka, ok := builtInServiceTemplateByType("eureka")
	if !ok {
		t.Fatalf("eureka template not found")
	}

	postgresFeatures := decodeServiceFeatureMatrix(t, postgres.SupportedFeatures)
	gitFeatures := decodeServiceFeatureMatrix(t, git.SupportedFeatures)

	for _, key := range []string{"managed", "shared", "external", "kubevirt"} {
		if _, ok := postgresFeatures[key]; !ok {
			t.Fatalf("postgres features missing %s: %#v", key, postgresFeatures)
		}
	}
	if !postgresFeatures["managed"] || !postgresFeatures["shared"] || !postgresFeatures["external"] || !postgresFeatures["kubevirt"] {
		t.Fatalf("postgres features = %#v, want all core feature modes enabled", postgresFeatures)
	}
	if !gitFeatures["managed"] || !gitFeatures["shared"] || !gitFeatures["external"] {
		t.Fatalf("git features = %#v, want managed/shared/external enabled", gitFeatures)
	}
	if gitFeatures["kubevirt"] {
		t.Fatalf("git features = %#v, kubevirt should be disabled for tool services", gitFeatures)
	}
	if postgres.Category != "database" {
		t.Fatalf("postgres category = %q, want database", postgres.Category)
	}
	if git.Category != "middleware" {
		t.Fatalf("git category = %q, want middleware", git.Category)
	}
	if nacos.S3Key != "charts/nacos.tar.gz" || eureka.S3Key != "charts/eureka.tar.gz" {
		t.Fatalf("nacos/eureka chart keys = %q/%q", nacos.S3Key, eureka.S3Key)
	}
	if nacos.Category != "middleware" || eureka.Category != "middleware" {
		t.Fatalf("nacos/eureka categories = %q/%q, want middleware", nacos.Category, eureka.Category)
	}
}

func TestBuiltInKubeVirtServiceTemplatesAreSeedable(t *testing.T) {
	templates := builtInKubeVirtServiceTemplates()
	byType := map[string]model.ServiceTemplate{}
	for _, tmpl := range templates {
		byType[tmpl.Type] = tmpl
	}
	for _, serviceType := range []string{"postgresql", "mysql", "redis"} {
		tmpl, ok := byType[serviceType]
		if !ok {
			t.Fatalf("missing kubevirt template for %s; got %#v", serviceType, byType)
		}
		if tmpl.ProvisionMode != model.ServiceProvisionModeKubeVirt {
			t.Fatalf("%s provisionMode = %q, want kubevirt", serviceType, tmpl.ProvisionMode)
		}
		if tmpl.Installer != "raw-yaml" {
			t.Fatalf("%s installer = %q, want raw-yaml", serviceType, tmpl.Installer)
		}
		if tmpl.S3Key == "" || !strings.Contains(tmpl.S3Key, "kubevirt/"+serviceType) {
			t.Fatalf("%s s3 key = %q, want stable kubevirt key", serviceType, tmpl.S3Key)
		}
		var runtimeSpec struct {
			Image string `json:"image"`
			Ports []struct {
				Port int `json:"port"`
			} `json:"ports"`
		}
		if err := json.Unmarshal([]byte(tmpl.RuntimeSpec), &runtimeSpec); err != nil {
			t.Fatalf("%s runtime spec is not json: %v", serviceType, err)
		}
		if runtimeSpec.Image == "" || len(runtimeSpec.Ports) == 0 || runtimeSpec.Ports[0].Port == 0 {
			t.Fatalf("%s runtime spec incomplete: %#v", serviceType, runtimeSpec)
		}
		features := decodeServiceFeatureMatrix(t, tmpl.SupportedFeatures)
		if !features["kubevirt"] {
			t.Fatalf("%s features = %#v, want kubevirt enabled", serviceType, features)
		}
	}
}

func TestServiceCatalogSeedNormalizesProductCategories(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceCatalog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := service.SeedServiceCatalogEntries(db, []model.ServiceCatalog{
		{Type: "ci", Name: "CI", Category: "tool", Enabled: true},
		{Type: "deploy", Name: "CD", Category: "tool", Enabled: true},
		{Type: "postgresql", Name: "PostgreSQL", Category: "infra", Enabled: true},
		{Type: "redis", Name: "Redis", Category: "infra", Enabled: true},
	}); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	var items []model.ServiceCatalog
	if err := db.Find(&items).Error; err != nil {
		t.Fatalf("list catalog: %v", err)
	}
	byType := map[string]string{}
	for _, item := range items {
		byType[item.Type] = item.Category
	}
	want := map[string]string{
		"ci":         "ci",
		"deploy":     "cd",
		"postgresql": "database",
		"redis":      "middleware",
	}
	for serviceType, category := range want {
		if byType[serviceType] != category {
			t.Fatalf("%s category = %q, want %q; all=%#v", serviceType, byType[serviceType], category, byType)
		}
	}
}

func TestNacosAndEurekaBuiltInChartArchivesParse(t *testing.T) {
	for _, item := range []struct {
		serviceType string
		chartName   string
	}{
		{serviceType: "nacos", chartName: "nacos"},
		{serviceType: "eureka", chartName: "eureka"},
	} {
		t.Run(item.serviceType, func(t *testing.T) {
			archivePath := filepath.Join("..", "..", "data", "charts", item.chartName+".tar.gz")
			manifest, err := extractManifestFromTar(archivePath)
			if err != nil {
				t.Fatalf("extract manifest from %s: %v", archivePath, err)
			}
			if manifest.Name != item.serviceType {
				t.Fatalf("manifest name = %q, want %q", manifest.Name, item.serviceType)
			}
			chartVersion, appVersion, err := extractChartYamlMeta(archivePath)
			if err != nil {
				t.Fatalf("extract chart metadata from %s: %v", archivePath, err)
			}
			if chartVersion == "" || appVersion == "" {
				t.Fatalf("chart metadata missing: chartVersion=%q appVersion=%q", chartVersion, appVersion)
			}
		})
	}
}

func decodeServiceFeatureMatrix(t *testing.T, raw string) map[string]bool {
	t.Helper()
	var rows []struct {
		Key     string `json:"key"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatalf("decode feature matrix %q: %v", raw, err)
	}
	out := map[string]bool{}
	for _, row := range rows {
		out[row.Key] = row.Enabled
	}
	return out
}

func TestEnvironmentTemplateCRUDRoutesAreMounted(t *testing.T) {
	t.Setenv("JWT_SECRET", "template-route-test-secret")

	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Permission{}, &model.Role{}, &model.RolePermission{}, &model.RoleBinding{}, &model.EnvTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	seedApplicationRBACForTest(t, db)

	passwordHash, err := hashPassword("Def@u1tpwd")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := model.User{Username: "admin", Email: "admin@example.test", Password: passwordHash}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	bindSystemRoleForTest(t, db, user.ID, model.RolePlatformAdmin)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRouter(router)
	token := loginForTemplateRouteTest(t, router)

	createBody := `{"name":"API 模板","description":"route test","services":["git","registry"],"infra":["redis"],"resourceCpu":"2核","resourceMem":"4GB","resourceDisk":"20GB"}`
	createRec := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/templates", bytes.NewBufferString(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var createResponse struct {
		Data model.EnvTemplate `json:"data"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResponse.Data.ID == 0 {
		t.Fatalf("created template missing id: %s", createRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/templates/1", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	updateRec := httptest.NewRecorder()
	updateReq := httptest.NewRequest(http.MethodPut, "/api/v1/templates/1", bytes.NewBufferString(`{"description":"updated","resourceMem":"6GB"}`))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d; body=%s", updateRec.Code, http.StatusOK, updateRec.Body.String())
	}

	var updated model.EnvTemplate
	if err := db.First(&updated, createResponse.Data.ID).Error; err != nil {
		t.Fatalf("find updated template: %v", err)
	}
	if updated.Description != "updated" || updated.ResourceMem != "6GB" {
		t.Fatalf("updated template = %#v", updated)
	}

	clearRec := httptest.NewRecorder()
	clearReq := httptest.NewRequest(http.MethodPut, "/api/v1/templates/1", bytes.NewBufferString(`{"services":[],"infra":[]}`))
	clearReq.Header.Set("Content-Type", "application/json")
	clearReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(clearRec, clearReq)
	if clearRec.Code != http.StatusOK {
		t.Fatalf("clear lists status = %d, want %d; body=%s", clearRec.Code, http.StatusOK, clearRec.Body.String())
	}
	if err := db.First(&updated, createResponse.Data.ID).Error; err != nil {
		t.Fatalf("find cleared template: %v", err)
	}
	if updated.Services != "[]" || updated.Infra != "[]" {
		t.Fatalf("cleared template lists = services:%q infra:%q, want []", updated.Services, updated.Infra)
	}

	deleteRec := httptest.NewRecorder()
	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/templates/1", nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want %d; body=%s", deleteRec.Code, http.StatusOK, deleteRec.Body.String())
	}
	var count int64
	if err := db.Model(&model.EnvTemplate{}).Where("id = ?", createResponse.Data.ID).Count(&count).Error; err != nil {
		t.Fatalf("count template: %v", err)
	}
	if count != 0 {
		t.Fatalf("template count = %d, want deleted", count)
	}
}

func loginForTemplateRouteTest(t *testing.T, router *gin.Engine) string {
	t.Helper()

	loginRec := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{"username":"admin","password":"Def@u1tpwd"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body=%s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}

	var loginResponse struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResponse.Data.Token == "" {
		t.Fatalf("login response missing token: %s", loginRec.Body.String())
	}
	return loginResponse.Data.Token
}
