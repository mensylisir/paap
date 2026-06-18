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
	if len(spring.NativeConfigs) == 0 {
		t.Fatalf("springboot template must keep native template preview source: %#v", spring)
	}
	if len(spring.Files) == 0 || spring.Files[0]["recommendedMountPath"] == "" {
		t.Fatalf("springboot template must expose only recommended mount path hints: %#v", spring.Files)
	}
	if _, exists := spring.Files[0]["mountPath"]; exists {
		t.Fatalf("springboot template must not bind runtime mountPath at template level: %#v", spring.Files[0])
	}
	if !bytes.Contains([]byte(spring.NativeConfigs[0]["content"].(string)), []byte("__TEMPLATE__JDBC_URL__数据库地址__")) {
		t.Fatalf("springboot native preview must use ordinary template syntax: %#v", spring.NativeConfigs)
	}
}

func TestBuiltInNginxTemplateDoesNotAssumeBusinessRoute(t *testing.T) {
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
	var nginx componentConfigTemplateResponse
	for _, item := range body.Data {
		if item.Key == "nginx-spa-api-proxy" {
			nginx = item
			break
		}
	}
	if nginx.Key == "" {
		t.Fatalf("missing nginx built-in template")
	}
	if len(nginx.NativeConfigs) == 0 {
		t.Fatalf("nginx template must keep native preview source")
	}
	content, _ := nginx.NativeConfigs[0]["content"].(string)
	for _, unwanted := range []string{"location /api/", "BACKEND_URL", "后端地址", "更多代理路由"} {
		if bytes.Contains([]byte(content), []byte(unwanted)) {
			t.Fatalf("nginx built-in template must not assume %q: %s", unwanted, content)
		}
	}
	if !bytes.Contains([]byte(content), []byte("__TEMPLATE__FOR__LOCATION_LIST__代理路由__")) {
		t.Fatalf("nginx built-in template must expose neutral proxy route list: %s", content)
	}
	if len(nginx.Files) == 0 || nginx.Files[0]["recommendedMountPath"] != "/etc/nginx/conf.d/default.conf" {
		t.Fatalf("nginx built-in template must provide a recommended mount path hint: %#v", nginx.Files)
	}
	if _, exists := nginx.Files[0]["mountPath"]; exists {
		t.Fatalf("nginx built-in template must not bind runtime mountPath at template level: %#v", nginx.Files[0])
	}
}

func TestParseNativeComponentConfigTemplate(t *testing.T) {
	parsed := parseNativeComponentConfigTemplate(`server {
  listen __TEMPLATE__LISTEN_PORT__监听端口__DEFAULT__80__;
  __TEMPLATE__FOR__LOCATION_LIST__位置块列表__
  location __TEMPLATE__ITEM_PATH__匹配路径__ {
    proxy_pass __TEMPLATE__ITEM_PROXY_PASS__转发地址__;
  }
  __TEMPLATE__END__LOCATION_LIST__
}`, nativeComponentConfigTemplateOptions{
		Framework: "nginx",
		FileName:  "default.conf",
	})

	if len(parsed.fields) != 2 {
		t.Fatalf("fields = %#v", parsed.fields)
	}
	if parsed.fields[0]["key"] != "LISTEN_PORT" || parsed.fields[0]["label"] != "监听端口" || parsed.fields[0]["default"] != "80" {
		t.Fatalf("listen field not parsed from ordinary syntax: %#v", parsed.fields[0])
	}
	list, ok := parsed.fields[1]["itemFields"].([]map[string]interface{})
	if !ok || len(list) != 2 {
		t.Fatalf("list item fields not parsed: %#v", parsed.fields[1])
	}
	if list[1]["key"] != "PROXY_PASS" || list[1]["type"] != "serviceRef" || list[1]["target"] != "backend" {
		t.Fatalf("proxy_pass item field must be a backend service reference: %#v", list[1])
	}
	content := parsed.configMaps[0]["data"].(map[string]string)["default.conf"]
	if !bytes.Contains([]byte(content), []byte("[[paap:LISTEN_PORT default=80]]")) {
		t.Fatalf("value token not converted: %s", content)
	}
	if !bytes.Contains([]byte(content), []byte("[[paap:for LOCATION_LIST]]")) {
		t.Fatalf("for token not converted: %s", content)
	}
	if len(parsed.files) != 1 || parsed.files[0]["recommendedMountPath"] != "/etc/nginx/conf.d/default.conf" {
		t.Fatalf("parsed native template must expose recommended mount path: %#v", parsed.files)
	}
	if _, exists := parsed.files[0]["mountPath"]; exists {
		t.Fatalf("parsed native template must not bind runtime mountPath: %#v", parsed.files[0])
	}
}

func TestComponentConfigTemplatesCreateAndDeleteCustom(t *testing.T) {
	router := setupComponentConfigTemplateTest(t)

	payload := []byte(`{
		"name":"Custom Gin Runtime",
		"framework":"go",
		"componentTypes":["backend"],
		"fields":[{"key":"redis.addr","label":"Redis 地址","type":"serviceRef","target":"redis"}],
		"env":[{"name":"REDIS_ADDR","source":"value","value":"redis-master:6379"}],
		"files":[{"name":"app.env","configMapName":"{{configMapName}}","key":"app.env","mountPath":"/etc/app/app.env"}]
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
	if len(created.Data.Files) != 1 || created.Data.Files[0]["recommendedMountPath"] != "/etc/app/app.env" {
		t.Fatalf("custom template must keep legacy mountPath only as a recommendation: %#v", created.Data.Files)
	}
	if _, exists := created.Data.Files[0]["mountPath"]; exists {
		t.Fatalf("custom template must not store runtime mountPath in template files: %#v", created.Data.Files[0])
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
