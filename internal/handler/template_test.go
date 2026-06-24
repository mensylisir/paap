package handler

import (
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
