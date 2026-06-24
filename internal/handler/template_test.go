package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListServiceCatalogHidesUnsupportedPlaceholders(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
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
	if !got["postgresql"] {
		t.Fatalf("expected postgresql in catalog, got %#v", got)
	}
	for _, hidden := range []string{"kingbase", "nacos", "disabled-demo"} {
		if got[hidden] {
			t.Fatalf("%s should be hidden from service catalog, got %#v", hidden, got)
		}
	}
}

func TestEnvironmentTemplateCRUDRoutesAreMounted(t *testing.T) {
	t.Setenv("JWT_SECRET", "template-route-test-secret")

	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.EnvTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	passwordHash, err := hashPassword("Def@u1tpwd")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.Create(&model.User{Username: "admin", Email: "admin@example.test", Password: passwordHash, Role: "admin"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

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
