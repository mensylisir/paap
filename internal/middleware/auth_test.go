package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"paap/internal/authz"
	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
)

func TestAuthRequiredAcceptsRuntimeConsoleWebSocketSubprotocolToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Role{}, &model.RoleBinding{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	user := model.User{Username: "admin", Email: "admin@example.test", Password: "hash"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := db.Create(&model.Role{Code: model.RolePlatformAdmin, Name: "平台管理员", ScopeType: model.ScopeSystem, Builtin: true, Editable: false, Enabled: true}).Error; err != nil {
		t.Fatalf("create platform role: %v", err)
	}
	if err := authz.BindRole(db, user.ID, model.RolePlatformAdmin, authz.SystemScope(), user.ID); err != nil {
		t.Fatalf("bind role: %v", err)
	}
	token, err := GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/console", AuthRequired(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"userID": c.GetUint(ContextUserIDKey), "roles": c.GetStringSlice(ContextUserRolesKey)})
	})

	req := httptest.NewRequest(http.MethodGet, "/console", nil)
	req.Header.Set("Sec-WebSocket-Protocol", RuntimeConsoleWebSocketProtocol+", "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), model.RolePlatformAdmin) {
		t.Fatalf("response must include platform role: %s", rec.Body.String())
	}
}

func TestUserIDFromRequestAcceptsQueryTokenOnlyForProxyRoutes(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	token, err := GenerateToken(42)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	proxyReq := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/services/9/proxy/d/node?paap_token="+url.QueryEscape(token), nil)
	userID, err := UserIDFromRequest(proxyReq)
	if err != nil {
		t.Fatalf("proxy query token should authenticate: %v", err)
	}
	if userID != 42 {
		t.Fatalf("userID = %d, want 42", userID)
	}

	normalReq := httptest.NewRequest(http.MethodGet, "/api/v1/applications?paap_token="+url.QueryEscape(token), nil)
	if _, err := UserIDFromRequest(normalReq); err == nil {
		t.Fatalf("non-proxy query token should not authenticate")
	}
}

func TestUserIDFromRequestAcceptsProxyCookieOnlyForProxyRoutes(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	token, err := GenerateToken(42)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	proxyReq := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/services/9/proxy/public/build/app.js", nil)
	proxyReq.AddCookie(&http.Cookie{Name: EmbeddedProxyAuthCookieName, Value: token})
	userID, err := UserIDFromRequest(proxyReq)
	if err != nil {
		t.Fatalf("proxy cookie token should authenticate: %v", err)
	}
	if userID != 42 {
		t.Fatalf("userID = %d, want 42", userID)
	}

	normalReq := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	normalReq.AddCookie(&http.Cookie{Name: EmbeddedProxyAuthCookieName, Value: token})
	if _, err := UserIDFromRequest(normalReq); err == nil {
		t.Fatalf("non-proxy cookie token should not authenticate")
	}
}
