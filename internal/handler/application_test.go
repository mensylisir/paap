package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"paap/internal/database"
	"paap/internal/middleware"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func withTestAuthUser(userID uint, next gin.HandlerFunc) gin.HandlerFunc {
	return withTestAuthUserRole(userID, "user", next)
}

func withTestAuthUserRole(userID uint, role string, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.ContextUserIDKey, userID)
		c.Set(middleware.ContextUserRoleKey, role)
		next(c)
	}
}

func TestListApplicationsIncludesEnvironmentCounts(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "预发", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running"}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	if err := db.Create(&model.Component{EnvironmentID: env.ID, Name: "订单服务", Type: "backend", Image: "registry.local/order:v1", Version: "v1", Replicas: 1}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/applications", withTestAuthUserRole(1, "admin", ListApplications))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Data []struct {
			ID           uint `json:"id"`
			Environments []struct {
				ID             uint `json:"id"`
				ToolCount      int  `json:"toolCount"`
				ComponentCount int  `json:"componentCount"`
				Services       []struct {
					ServiceType string `json:"serviceType"`
					Status      string `json:"status"`
				} `json:"services"`
			} `json:"environments"`
			EnvironmentCount int `json:"environmentCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected one app, got %#v", body.Data)
	}
	if body.Data[0].EnvironmentCount != 1 {
		t.Fatalf("environmentCount = %d, want 1", body.Data[0].EnvironmentCount)
	}
	if len(body.Data[0].Environments) != 1 {
		t.Fatalf("expected one environment, got %#v", body.Data[0].Environments)
	}
	if body.Data[0].Environments[0].ToolCount != 1 {
		t.Fatalf("toolCount = %d, want 1", body.Data[0].Environments[0].ToolCount)
	}
	if body.Data[0].Environments[0].ComponentCount != 1 {
		t.Fatalf("componentCount = %d, want 1", body.Data[0].Environments[0].ComponentCount)
	}
	if len(body.Data[0].Environments[0].Services) != 1 {
		t.Fatalf("expected service summary, got %#v", body.Data[0].Environments[0].Services)
	}
	if body.Data[0].Environments[0].Services[0].ServiceType != "deploy" || body.Data[0].Environments[0].Services[0].Status != "running" {
		t.Fatalf("service summary = %#v", body.Data[0].Environments[0].Services[0])
	}
}

func TestListApplicationsFiltersByAppMemberForRegularUsers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	hidden := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	visible := model.Application{Name: "可见应用", Identifier: "visible", OwnerID: 2}
	if err := db.Create(&hidden).Error; err != nil {
		t.Fatalf("create hidden app: %v", err)
	}
	if err := db.Create(&visible).Error; err != nil {
		t.Fatalf("create visible app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: visible.ID, UserID: 2, Role: "member"}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/applications", withTestAuthUser(2, ListApplications))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Data []struct {
			ID         uint   `json:"id"`
			Identifier string `json:"identifier"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Identifier != "visible" {
		t.Fatalf("applications = %#v, want only visible", body.Data)
	}
}

func TestCreateApplicationGeneratesIdentifierWhenMissing(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	body, _ := json.Marshal(CreateAppRequest{Name: "订单服务"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/applications", withTestAuthUser(1, CreateApplication))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got struct {
		Data model.Application `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.Identifier != "app" {
		t.Fatalf("identifier = %q, want app", got.Data.Identifier)
	}
}

func TestCreateApplicationUsesAuthenticatedUserAsOwner(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	body, _ := json.Marshal(CreateAppRequest{Name: "结算服务"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/applications", withTestAuthUser(42, CreateApplication))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got struct {
		Data model.Application `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.OwnerID != 42 {
		t.Fatalf("ownerID = %d, want 42", got.Data.OwnerID)
	}
	var member model.AppMember
	if err := db.Where("application_id = ?", got.Data.ID).First(&member).Error; err != nil {
		t.Fatalf("find member: %v", err)
	}
	if member.UserID != 42 {
		t.Fatalf("member userID = %d, want 42", member.UserID)
	}
}

func TestGetApplicationIncludesEnvironmentCounts(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
		&model.AppMember{},
		&model.User{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "预发", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running"}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	if err := db.Create(&model.Component{EnvironmentID: env.ID, Name: "订单服务", Type: "backend", Image: "registry.local/order:v1", Version: "v1", Replicas: 1}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/applications/:id", GetApplication)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/1", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Data struct {
			Environments []struct {
				ToolCount      int `json:"toolCount"`
				ComponentCount int `json:"componentCount"`
				Services       []struct {
					ServiceType string `json:"serviceType"`
					Status      string `json:"status"`
				} `json:"services"`
			} `json:"environments"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.Environments) != 1 {
		t.Fatalf("expected one environment, got %#v", body.Data.Environments)
	}
	if body.Data.Environments[0].ToolCount != 1 {
		t.Fatalf("toolCount = %d, want 1", body.Data.Environments[0].ToolCount)
	}
	if body.Data.Environments[0].ComponentCount != 1 {
		t.Fatalf("componentCount = %d, want 1", body.Data.Environments[0].ComponentCount)
	}
	if len(body.Data.Environments[0].Services) != 1 {
		t.Fatalf("expected service summary, got %#v", body.Data.Environments[0].Services)
	}
}

func TestBuiltInTemplateOrderingPrefersLightweightRegistry(t *testing.T) {
	if builtInInstallOrder("registry") >= builtInInstallOrder("harbor") {
		t.Fatalf("lightweight registry should be ordered before Harbor")
	}
	if got := builtInDescriptionOverride("registry"); !strings.Contains(got, "轻量") {
		t.Fatalf("registry should describe the lightweight option, got %q", got)
	}
	if got := builtInDescriptionOverride("harbor"); !strings.Contains(got, "企业级") {
		t.Fatalf("harbor should describe the advanced/heavy option, got %q", got)
	}
}
