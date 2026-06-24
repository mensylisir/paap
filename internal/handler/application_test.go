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
	router.GET("/api/v1/applications/:id", withTestAuthUserRole(1, "admin", GetApplication))
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

func TestGetApplicationRejectsNonMembers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.AppMember{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	hidden := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&hidden).Error; err != nil {
		t.Fatalf("create hidden app: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/applications/:id", withTestAuthUser(2, GetApplication))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/1", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestUpdateApplicationRejectsNonMembers(t *testing.T) {
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

	app := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}

	body, _ := json.Marshal(UpdateAppRequest{Name: "非法更新"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/applications/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/applications/:id", withTestAuthUser(2, UpdateApplication))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	var got model.Application
	if err := db.First(&got, app.ID).Error; err != nil {
		t.Fatalf("find app: %v", err)
	}
	if got.Name != "隐藏应用" {
		t.Fatalf("name = %q, want unchanged", got.Name)
	}
}

func TestDeleteApplicationRejectsNonMembers(t *testing.T) {
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
		&model.InfraInstallation{},
		&model.Component{},
		&model.EnvironmentCanvasState{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/applications/1", nil)
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/applications/:id", withTestAuthUser(2, DeleteApplication))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	var count int64
	if err := db.Model(&model.Application{}).Where("id = ?", app.ID).Count(&count).Error; err != nil {
		t.Fatalf("count app: %v", err)
	}
	if count != 1 {
		t.Fatalf("app count = %d, want unchanged", count)
	}
}

func TestListApplicationMembersReturnsUsersForAppMembers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	users := []model.User{
		{Username: "owner", Email: "owner@example.com", Password: "x", Role: "user"},
		{Username: "alice", Email: "alice@example.com", Password: "x", Role: "user"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := db.Create(&[]model.AppMember{
		{ApplicationID: app.ID, UserID: users[0].ID, Role: "admin"},
		{ApplicationID: app.ID, UserID: users[1].ID, Role: "member"},
	}).Error; err != nil {
		t.Fatalf("create members: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/applications/:id/members", withTestAuthUser(users[1].ID, ListApplicationMembers))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/1/members", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Data []model.AppMember `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 2 {
		t.Fatalf("members = %#v, want two", body.Data)
	}
	if body.Data[0].User.Username != "owner" || body.Data[1].User.Username != "alice" {
		t.Fatalf("member users = %#v", body.Data)
	}
}

func TestInviteApplicationMemberAddsExistingUserByUsername(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	owner := model.User{Username: "owner", Email: "owner@example.com", Password: "x", Role: "user"}
	target := model.User{Username: "bob", Email: "bob@example.com", Password: "x", Role: "user"}
	if err := db.Create(&[]model.User{owner, target}).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := db.Where("username = ?", "owner").First(&owner).Error; err != nil {
		t.Fatalf("find owner: %v", err)
	}
	if err := db.Where("username = ?", "bob").First(&target).Error; err != nil {
		t.Fatalf("find target: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: owner.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create owner member: %v", err)
	}

	body, _ := json.Marshal(InviteAppMemberRequest{Username: "bob", Role: "viewer"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/1/members", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/applications/:id/members", withTestAuthUser(owner.ID, InviteApplicationMember))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var member model.AppMember
	if err := db.Where("application_id = ? AND user_id = ?", app.ID, target.ID).First(&member).Error; err != nil {
		t.Fatalf("find invited member: %v", err)
	}
	if member.Role != "viewer" {
		t.Fatalf("role = %q, want viewer", member.Role)
	}
}

func TestUpdateApplicationMemberRoleRequiresAppAdmin(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	admin := model.User{Username: "owner", Email: "owner@example.com", Password: "x", Role: "user"}
	memberUser := model.User{Username: "alice", Email: "alice@example.com", Password: "x", Role: "user"}
	targetUser := model.User{Username: "bob", Email: "bob@example.com", Password: "x", Role: "user"}
	if err := db.Create(&[]model.User{admin, memberUser, targetUser}).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	var users []model.User
	if err := db.Order("username").Find(&users).Error; err != nil {
		t.Fatalf("find users: %v", err)
	}
	byName := map[string]model.User{}
	for _, user := range users {
		byName[user.Username] = user
	}
	targetMember := model.AppMember{ApplicationID: app.ID, UserID: byName["bob"].ID, Role: "member"}
	if err := db.Create(&[]model.AppMember{
		{ApplicationID: app.ID, UserID: byName["owner"].ID, Role: "admin"},
		{ApplicationID: app.ID, UserID: byName["alice"].ID, Role: "member"},
		targetMember,
	}).Error; err != nil {
		t.Fatalf("create members: %v", err)
	}
	if err := db.Where("application_id = ? AND user_id = ?", app.ID, byName["bob"].ID).First(&targetMember).Error; err != nil {
		t.Fatalf("find target member: %v", err)
	}

	body, _ := json.Marshal(UpdateAppMemberRoleRequest{Role: "viewer"})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/applications/1/members/3", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/applications/:id/members/:memberId", withTestAuthUser(byName["alice"].ID, UpdateApplicationMemberRole))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	var got model.AppMember
	if err := db.First(&got, targetMember.ID).Error; err != nil {
		t.Fatalf("find target: %v", err)
	}
	if got.Role != "member" {
		t.Fatalf("role = %q, want unchanged", got.Role)
	}
}

func TestRemoveApplicationMemberPreventsDeletingLastAdmin(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	admin := model.User{Username: "owner", Email: "owner@example.com", Password: "x", Role: "user"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	member := model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/applications/1/members/1", nil)
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/applications/:id/members/:memberId", withTestAuthUser(admin.ID, RemoveApplicationMember))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var count int64
	if err := db.Model(&model.AppMember{}).Where("application_id = ?", app.ID).Count(&count).Error; err != nil {
		t.Fatalf("count members: %v", err)
	}
	if count != 1 {
		t.Fatalf("member count = %d, want unchanged", count)
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
