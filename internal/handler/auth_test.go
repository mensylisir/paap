package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
)

func TestLoginReturnsSignedJWTAndMeAcceptsBearerToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.UserRole{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	passwordHash, err := hashPassword("Def@u1tpwd")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.Create(&model.User{Username: "admin", Email: "admin@example.test", Password: passwordHash}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("find admin: %v", err)
	}
	if _, err := model.ReplaceUserRoles(db, admin.ID, []string{model.RolePlatformAdmin, model.RoleAppAdmin}); err != nil {
		t.Fatalf("create roles: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/login", Login)
	router.GET("/api/v1/auth/me", GetCurrentUser)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"Def@u1tpwd"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d; body=%s", loginRec.Code, http.StatusOK, loginRec.Body.String())
	}

	var loginResponse struct {
		Data struct {
			Token string   `json:"token"`
			Roles []string `json:"roles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if parts := strings.Split(loginResponse.Data.Token, "."); len(parts) != 3 {
		t.Fatalf("token = %q, want compact JWT with three segments", loginResponse.Data.Token)
	}
	if !model.HasUserRole(loginResponse.Data.Roles, model.RolePlatformAdmin) || !model.HasUserRole(loginResponse.Data.Roles, model.RoleAppAdmin) {
		t.Fatalf("login roles = %#v, want platform_admin and app_admin", loginResponse.Data.Roles)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResponse.Data.Token)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("me status = %d, want %d; body=%s", meRec.Code, http.StatusOK, meRec.Body.String())
	}

	var meResponse struct {
		Data struct {
			Username string   `json:"username"`
			Roles    []string `json:"roles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &meResponse); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResponse.Data.Username != "admin" || !model.HasUserRole(meResponse.Data.Roles, model.RolePlatformAdmin) || !model.HasUserRole(meResponse.Data.Roles, model.RoleAppAdmin) {
		t.Fatalf("current user = %#v, want admin with platform_admin and app_admin", meResponse.Data)
	}
}

func TestPlatformAdminMigrationSeedsPlatformAdminWithHardenedPassword(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	if err := database.RunSQLMigrations(); err != nil {
		t.Fatalf("run sql migrations: %v", err)
	}

	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("find admin: %v", err)
	}
	roles, err := model.UserRoleValues(db, admin.ID)
	if err != nil {
		t.Fatalf("load admin roles: %v", err)
	}
	if !model.HasUserRole(roles, model.RolePlatformAdmin) || !model.HasUserRole(roles, model.RoleAppAdmin) {
		t.Fatalf("seeded admin roles = %#v, want platform_admin and app_admin", roles)
	}
	if !checkPasswordHash("Def@u1tpwd", admin.Password) {
		t.Fatalf("fresh migration admin password must match hardened default")
	}
	if checkPasswordHash("admin123", admin.Password) {
		t.Fatalf("fresh migration admin password must not accept legacy default")
	}
}

func TestPlatformAdminMigrationUpdatesExistingAdminPassword(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	legacyHash, err := hashPassword("admin123")
	if err != nil {
		t.Fatalf("hash legacy password: %v", err)
	}
	if err := db.Create(&model.User{Username: "admin", Email: "admin@paap.local", Password: legacyHash}).Error; err != nil {
		t.Fatalf("create legacy admin: %v", err)
	}

	if err := database.RunSQLMigrations(); err != nil {
		t.Fatalf("run sql migrations: %v", err)
	}

	var admin model.User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("find admin: %v", err)
	}
	if !checkPasswordHash("Def@u1tpwd", admin.Password) {
		t.Fatalf("legacy admin password must be rotated to hardened default")
	}
	if checkPasswordHash("admin123", admin.Password) {
		t.Fatalf("legacy admin password must stop working after seed rotation")
	}
}
