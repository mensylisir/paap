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

func TestMissingAssetDoesNotUseSPAFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRouter(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/missing-old-chunk.js", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "<!doctype html") {
		t.Fatalf("missing asset must not return SPA HTML fallback: %s", rec.Body.String())
	}
}

func TestAPIRoutesRequireAuthExceptLogin(t *testing.T) {
	t.Setenv("JWT_SECRET", "router-test-secret")

	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.UserRole{},
		&model.Application{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	passwordHash, err := hashPassword("Def@u1tpwd")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := model.User{Username: "admin", Email: "admin@example.test", Password: passwordHash}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := model.ReplaceUserRoles(db, user.ID, []string{model.RolePlatformAdmin, model.RoleAppAdmin}); err != nil {
		t.Fatalf("create roles: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRouter(router)

	noAuthRec := httptest.NewRecorder()
	noAuthReq := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	router.ServeHTTP(noAuthRec, noAuthReq)
	if noAuthRec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated applications status = %d, want %d; body=%s", noAuthRec.Code, http.StatusUnauthorized, noAuthRec.Body.String())
	}

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
		t.Fatalf("login response did not include token: %s", loginRec.Body.String())
	}

	withAuthRec := httptest.NewRecorder()
	withAuthReq := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	withAuthReq.Header.Set("Authorization", "Bearer "+loginResponse.Data.Token)
	router.ServeHTTP(withAuthRec, withAuthReq)
	if withAuthRec.Code != http.StatusOK {
		t.Fatalf("authenticated applications status = %d, want %d; body=%s", withAuthRec.Code, http.StatusOK, withAuthRec.Body.String())
	}
}
