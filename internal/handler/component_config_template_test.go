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

func TestComponentConfigTemplatesSeedBuiltInsAndList(t *testing.T) {
	router := setupComponentConfigTemplateTest(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/component-config-templates", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data []componentConfigTemplateResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) < 6 {
		t.Fatalf("expected built-in templates, got %#v", body.Data)
	}
	var spring componentConfigTemplateResponse
	for _, item := range body.Data {
		if item.Key == "springboot-postgres-redis" {
			spring = item
			break
		}
	}
	if spring.Key == "" || !spring.IsBuiltin || spring.Framework != "springboot" {
		t.Fatalf("missing springboot built-in template: %#v", spring)
	}
	if len(spring.Fields) == 0 || len(spring.Env) == 0 || len(spring.ConfigMaps) == 0 || len(spring.Files) == 0 {
		t.Fatalf("springboot template must expose fields and real outputs: %#v", spring)
	}
}

func TestComponentConfigTemplatesCreateAndDeleteCustom(t *testing.T) {
	router := setupComponentConfigTemplateTest(t)

	payload := []byte(`{
		"name":"Custom Gin Runtime",
		"framework":"go",
		"componentTypes":["backend"],
		"fields":[{"key":"redis.addr","label":"Redis 地址","type":"serviceRef","target":"redis"}],
		"env":[{"name":"REDIS_ADDR","source":"value","value":"redis-master:6379"}]
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/component-config-templates", bytes.NewReader(payload))
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Data componentConfigTemplateResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.Data.Key != "custom-custom-gin-runtime" || created.Data.IsBuiltin {
		t.Fatalf("unexpected custom template: %#v", created.Data)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/component-config-templates/1", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("built-in delete status = %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/component-config-templates/"+stringID(created.Data.ID), nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("custom delete status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func stringID(id uint) string {
	data, _ := json.Marshal(id)
	return string(data)
}

func setupComponentConfigTemplateTest(t *testing.T) *gin.Engine {
	t.Helper()
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ComponentConfigTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	SeedComponentConfigTemplates()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRouter(router)
	return router
}
