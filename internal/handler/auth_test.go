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
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoginReturnsSignedJWTAndMeAcceptsBearerToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	passwordHash, err := hashPassword("admin123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := db.Create(&model.User{Username: "admin", Email: "admin@example.test", Password: passwordHash, Role: "admin"}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/auth/login", Login)
	router.GET("/api/v1/auth/me", GetCurrentUser)

	loginBody := bytes.NewBufferString(`{"username":"admin","password":"admin123"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody)
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
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
	if parts := strings.Split(loginResponse.Data.Token, "."); len(parts) != 3 {
		t.Fatalf("token = %q, want compact JWT with three segments", loginResponse.Data.Token)
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
			Username string `json:"username"`
			Role     string `json:"role"`
		} `json:"data"`
	}
	if err := json.Unmarshal(meRec.Body.Bytes(), &meResponse); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResponse.Data.Username != "admin" || meResponse.Data.Role != "admin" {
		t.Fatalf("current user = %#v, want admin/admin", meResponse.Data)
	}
}
