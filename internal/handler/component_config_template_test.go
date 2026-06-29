package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"paap/internal/database"
	"paap/internal/middleware"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
)

func TestComponentConfigTemplatesListStartsEmpty(t *testing.T) {
	router, token := setupComponentConfigTemplateTest(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/component-config-templates", nil)
	req.Header.Set("Authorization", "Bearer "+token)
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
	if len(body.Data) != 0 {
		t.Fatalf("component config templates must be user/import data, got %#v", body.Data)
	}
}

func TestParseNativeComponentConfigTemplate(t *testing.T) {
	parsed := service.ParseNativeComponentConfigTemplate(`server {
  listen __TEMPLATE__LISTEN_PORT__监听端口__DEFAULT__80__;
  __TEMPLATE__FOR__LOCATION_LIST__位置块列表__
  location __TEMPLATE__ITEM_PATH__匹配路径__ {
    proxy_pass __TEMPLATE__ITEM_PROXY_PASS__转发地址__;
  }
  __TEMPLATE__END__LOCATION_LIST__
}`, service.NativeComponentConfigTemplateOptions{
		Framework: "nginx",
		FileName:  "default.conf",
	})

	if len(parsed.Fields) != 2 {
		t.Fatalf("fields = %#v", parsed.Fields)
	}
	if parsed.Fields[0]["key"] != "LISTEN_PORT" || parsed.Fields[0]["label"] != "监听端口" || parsed.Fields[0]["default"] != "80" {
		t.Fatalf("listen field not parsed from ordinary syntax: %#v", parsed.Fields[0])
	}
	list, ok := parsed.Fields[1]["itemFields"].([]map[string]interface{})
	if !ok || len(list) != 2 {
		t.Fatalf("list item fields not parsed: %#v", parsed.Fields[1])
	}
	if list[1]["key"] != "PROXY_PASS" || list[1]["type"] != "serviceRef" || list[1]["target"] != "backend" {
		t.Fatalf("proxy_pass item field must be a backend service reference: %#v", list[1])
	}
	content := parsed.ConfigMaps[0]["data"].(map[string]string)["default.conf"]
	if !bytes.Contains([]byte(content), []byte("[[paap:LISTEN_PORT default=80]]")) {
		t.Fatalf("value token not converted: %s", content)
	}
	if !bytes.Contains([]byte(content), []byte("[[paap:for LOCATION_LIST]]")) {
		t.Fatalf("for token not converted: %s", content)
	}
	if len(parsed.Files) != 1 || parsed.Files[0]["recommendedMountPath"] != "/etc/nginx/conf.d/default.conf" {
		t.Fatalf("parsed native template must expose recommended mount path: %#v", parsed.Files)
	}
	if _, exists := parsed.Files[0]["mountPath"]; exists {
		t.Fatalf("parsed native template must not bind runtime mountPath: %#v", parsed.Files[0])
	}
}

func TestParseNativeComponentConfigTemplateInfersTextareaItems(t *testing.T) {
	parsed := service.ParseNativeComponentConfigTemplate(`server {
  __TEMPLATE__FOR__LOCATION_LIST__Location 规则__
  location __TEMPLATE__ITEM_MATCH__匹配规则__DEFAULT__/__ {
    __TEMPLATE__ITEM_DIRECTIVES__指令块__
  }
  __TEMPLATE__END__LOCATION_LIST__
}`, service.NativeComponentConfigTemplateOptions{
		Framework: "nginx",
		FileName:  "default.conf",
	})

	if len(parsed.Fields) != 1 {
		t.Fatalf("fields = %#v", parsed.Fields)
	}
	list, ok := parsed.Fields[0]["itemFields"].([]map[string]interface{})
	if !ok || len(list) != 2 {
		t.Fatalf("list item fields not parsed: %#v", parsed.Fields[0])
	}
	if list[0]["key"] != "MATCH" || list[0]["type"] != "text" {
		t.Fatalf("match item field not parsed: %#v", list[0])
	}
	if list[1]["key"] != "DIRECTIVES" || list[1]["type"] != "textarea" {
		t.Fatalf("directives item field must be textarea: %#v", list[1])
	}
}

func TestParseUploadedNativeJSONConfigTemplateFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "appsettings.json")
	if err := os.WriteFile(filePath, []byte(`{
  "fields": {
    "note": "this is an application config field, not a PAAP template field"
  },
  "schema": {
    "version": "2020-12"
  },
  "ConnectionStrings": {
    "Default": "__TEMPLATE__DATABASE_URL__数据库地址__"
  },
  "JwtSecret": "__TEMPLATE__JWT_SECRET__JWT密钥__"
}`), 0644); err != nil {
		t.Fatalf("write native json: %v", err)
	}

	tmpl, err := service.ParseUploadedComponentConfigTemplateFile(filePath, "appsettings.json", service.ComponentConfigTemplateUploadOptions{
		Mode:           "native",
		Name:           "ASP.NET Runtime",
		Framework:      "custom",
		ComponentTypes: []string{"backend"},
	})
	if err != nil {
		t.Fatalf("parse native json upload: %v", err)
	}
	if tmpl.Name != "ASP.NET Runtime" {
		t.Fatalf("native json must not be decoded as advanced template: %#v", tmpl)
	}
	fields := service.DecodeObjectArray(tmpl.FieldsJSON)
	if len(fields) != 2 {
		t.Fatalf("native json fields = %#v", fields)
	}
	configs := service.DecodeObjectArray(tmpl.ConfigJSON)
	data := configs[0]["data"].(map[string]interface{})
	if !strings.Contains(data["appsettings.json"].(string), "[[paap:DATABASE_URL]]") {
		t.Fatalf("native json must be stored as converted config file: %#v", data)
	}
	if tmpl.Syntax != service.NativeComponentConfigTemplateSyntax {
		t.Fatalf("native json must keep native syntax, got %q", tmpl.Syntax)
	}
}

func TestParseUploadedConfigTemplateDefaultsToOrdinaryConfig(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(filePath, []byte(`{
  "schema": {
    "note": "this belongs to the user's application config"
  },
  "fields": {
    "note": "this is also application data, not PAAP template metadata"
  },
  "database": {
    "host": "__TEMPLATE__DATABASE_HOST__数据库地址__"
  }
}`), 0644); err != nil {
		t.Fatalf("write ordinary json config: %v", err)
	}

	tmpl, err := service.ParseUploadedComponentConfigTemplateFile(filePath, "schema.json", service.ComponentConfigTemplateUploadOptions{
		Name:      "Ordinary JSON Config",
		Framework: "custom",
	})
	if err != nil {
		t.Fatalf("parse ordinary config with default mode: %v", err)
	}
	if tmpl.Name != "Ordinary JSON Config" || tmpl.Syntax != service.NativeComponentConfigTemplateSyntax {
		t.Fatalf("default upload must be ordinary config mode: %#v", tmpl)
	}
	fields := service.DecodeObjectArray(tmpl.FieldsJSON)
	if len(fields) != 1 || fields[0]["key"] != "DATABASE_HOST" {
		t.Fatalf("ordinary JSON config fields = %#v", fields)
	}
	configs := service.DecodeObjectArray(tmpl.ConfigJSON)
	data := configs[0]["data"].(map[string]interface{})
	content := data["schema.json"].(string)
	if !strings.Contains(content, `"schema"`) || !strings.Contains(content, "[[paap:DATABASE_HOST]]") {
		t.Fatalf("ordinary JSON config must be kept as config content: %s", content)
	}
}

func TestParseUploadedConfigTemplateAutoModeDoesNotGuessAdvancedTemplate(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "template.json")
	if err := os.WriteFile(filePath, []byte(`{
  "template": {
    "note": "this is an application-owned config object"
  },
  "schema": {
    "fields": ["application", "data"]
  },
  "endpoint": "__TEMPLATE__API_ENDPOINT__API地址__"
}`), 0644); err != nil {
		t.Fatalf("write ordinary json config: %v", err)
	}

	tmpl, err := service.ParseUploadedComponentConfigTemplateFile(filePath, "template.json", service.ComponentConfigTemplateUploadOptions{
		Mode:      "auto",
		Name:      "普通业务 template.json",
		Framework: "custom",
	})
	if err != nil {
		t.Fatalf("parse auto mode ordinary config: %v", err)
	}
	if tmpl.Name != "普通业务 template.json" || tmpl.Syntax != service.NativeComponentConfigTemplateSyntax {
		t.Fatalf("auto mode must fall back to ordinary config semantics: %#v", tmpl)
	}
	fields := service.DecodeObjectArray(tmpl.FieldsJSON)
	if len(fields) != 1 || fields[0]["key"] != "API_ENDPOINT" {
		t.Fatalf("ordinary template.json fields = %#v", fields)
	}
	configs := service.DecodeObjectArray(tmpl.ConfigJSON)
	data := configs[0]["data"].(map[string]interface{})
	content := data["template.json"].(string)
	if !strings.Contains(content, `"template"`) || !strings.Contains(content, "[[paap:API_ENDPOINT]]") {
		t.Fatalf("ordinary template.json must remain config content: %s", content)
	}
}

func TestParseNativeComponentConfigTemplateFilesCombinesFiles(t *testing.T) {
	tmpl, err := service.ParseNativeComponentConfigTemplateFiles([]service.NativeComponentConfigTemplateFile{
		{Name: "application.yml", Content: "spring:\n  data:\n    mongodb:\n      host: __TEMPLATE__MONGODB_HOST__MongoDB地址__\n"},
		{Name: "bootstrap.yml", Content: "eureka:\n  client:\n    serviceUrl:\n      defaultZone: __TEMPLATE__EUREKA_URL__Eureka地址__\n"},
	}, service.ComponentConfigTemplateUploadOptions{
		Mode:           "native",
		Name:           "Piggy Component Runtime",
		Framework:      "springboot",
		ComponentTypes: []string{"backend"},
	})
	if err != nil {
		t.Fatalf("parse multiple native files: %v", err)
	}
	nativeConfigs := service.DecodeObjectArray(tmpl.NativeJSON)
	if len(nativeConfigs) != 2 {
		t.Fatalf("native configs = %#v", nativeConfigs)
	}
	fields := service.DecodeObjectArray(tmpl.FieldsJSON)
	if len(fields) != 2 {
		t.Fatalf("fields = %#v", fields)
	}
	if fields[0]["type"] != "serviceRef" || fields[0]["target"] != "mongodb" {
		t.Fatalf("mongodb host must infer serviceRef: %#v", fields[0])
	}
	if fields[1]["type"] != "serviceRef" || fields[1]["target"] != "eureka" {
		t.Fatalf("eureka url must infer serviceRef: %#v", fields[1])
	}
	configs := service.DecodeObjectArray(tmpl.ConfigJSON)
	data := configs[0]["data"].(map[string]interface{})
	if _, ok := data["application.yml"]; !ok {
		t.Fatalf("application.yml missing from combined config map: %#v", data)
	}
	if _, ok := data["bootstrap.yml"]; !ok {
		t.Fatalf("bootstrap.yml missing from combined config map: %#v", data)
	}
	files := service.DecodeObjectArray(tmpl.FileJSON)
	if len(files) != 2 {
		t.Fatalf("file hints = %#v", files)
	}
}

func TestParseNativeComponentConfigTemplateAllowsChineseNameWithoutKey(t *testing.T) {
	tmpl, err := service.ParseNativeComponentConfigTemplateFiles([]service.NativeComponentConfigTemplateFile{
		{Name: "application.yml", Content: "server:\n  port: __TEMPLATE__SERVER_PORT__服务端口__DEFAULT__8080__\n"},
	}, service.ComponentConfigTemplateUploadOptions{
		Mode:      "native",
		Name:      "中文配置模板",
		Framework: "springboot",
	})
	if err != nil {
		t.Fatalf("parse native template with chinese name: %v", err)
	}
	if !strings.HasPrefix(tmpl.Key, "custom-template-") {
		t.Fatalf("unexpected fallback key: %s", tmpl.Key)
	}
}

func TestComponentConfigTemplatesCreateAndDeleteCustom(t *testing.T) {
	router, token := setupComponentConfigTemplateTest(t)

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
	req.Header.Set("Authorization", "Bearer "+token)
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
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/component-config-templates/"+stringID(created.Data.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("custom delete status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestParseComponentConfigTemplatePackageFile(t *testing.T) {
	archivePath := writeComponentConfigTemplatePackage(t, map[string]string{
		"template.json": `{
			"key":"nginx-default-conf",
			"name":"Nginx default.conf",
			"framework":"nginx",
			"componentTypes":["frontend"],
			"nativeConfigs":[{"name":"default.conf","path":"files/default.conf"}],
			"secrets":[{"name":"{{secretName}}","data":{"TOKEN":"__TEMPLATE__TOKEN__Token__"}}]
		}`,
		"schema.json":        `{"fields":[{"key":"LISTEN_PORT","label":"监听端口","type":"number","default":"80"},{"key":"TOKEN","label":"Token","type":"password","output":"secret"}]}`,
		"files/default.conf": "server { listen __TEMPLATE__LISTEN_PORT__监听端口__DEFAULT__80__; }",
	})

	tmpl, err := service.ParseComponentConfigTemplatePackageFile(archivePath)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}
	if tmpl.Key != "nginx-default-conf" || tmpl.Framework != "nginx" {
		t.Fatalf("unexpected template identity: %#v", tmpl)
	}
	configs := service.DecodeObjectArray(tmpl.ConfigJSON)
	if len(configs) != 1 {
		t.Fatalf("generated config maps = %#v", configs)
	}
	data, ok := configs[0]["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("config data not decoded: %#v", configs[0])
	}
	if !bytes.Contains([]byte(data["default.conf"].(string)), []byte("[[paap:LISTEN_PORT default=80]]")) {
		t.Fatalf("native placeholders not converted: %#v", data)
	}
	secrets := service.DecodeObjectArray(tmpl.SecretJSON)
	secretData := secrets[0]["data"].(map[string]interface{})
	if secretData["TOKEN"] != "[[paap:TOKEN]]" {
		t.Fatalf("secret placeholders not converted: %#v", secretData)
	}
}

func TestParsePackagedBuiltInComponentConfigTemplates(t *testing.T) {
	paths, err := filepath.Glob("../../data/config-templates/*.tar.gz")
	if err != nil {
		t.Fatalf("glob packaged templates: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected packaged built-in config templates")
	}
	for _, archivePath := range paths {
		t.Run(filepath.Base(archivePath), func(t *testing.T) {
			tmpl, err := service.ParseComponentConfigTemplatePackageFile(archivePath)
			if err != nil {
				t.Fatalf("parse packaged template: %v", err)
			}
			if tmpl.Key == "" || tmpl.Name == "" {
				t.Fatalf("template identity missing: %#v", tmpl)
			}
			if strings.TrimSpace(tmpl.FieldsJSON) == "" {
				t.Fatalf("fields json missing for %s", archivePath)
			}
		})
	}
}

func TestBuiltInComponentConfigTemplateArchivePathsIncludesNginxDefault(t *testing.T) {
	paths, err := service.BuiltInComponentConfigTemplateArchivePaths()
	if err != nil {
		t.Fatalf("list built-in config template archives: %v", err)
	}
	found := false
	for _, archivePath := range paths {
		if filepath.Base(archivePath) == "nginx-default-conf.tar.gz" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("nginx-default-conf.tar.gz not found in built-in config template archives: %#v", paths)
	}
}

func TestComponentConfigTemplatesUpdateAndDeleteOldBuiltinData(t *testing.T) {
	router, token := setupComponentConfigTemplateTest(t)

	tmpl := model.ComponentConfigTemplate{
		Key:            "legacy-built-in-template",
		Name:           "Legacy Built In Template",
		Framework:      "go",
		BindingMode:    "recommended",
		ComponentTypes: service.MustJSON([]string{"backend"}),
		FieldsJSON:     "[]",
		EnvJSON:        "[]",
		ConfigJSON:     "[]",
		SecretJSON:     "[]",
		FileJSON:       "[]",
		CommandJSON:    "[]",
		ArgsJSON:       "[]",
		IsBuiltin:      true,
		Enabled:        true,
	}
	if err := database.DB.Create(&tmpl).Error; err != nil {
		t.Fatalf("create legacy template: %v", err)
	}

	payload := []byte(`{
		"name":"Edited Runtime Template",
		"framework":"go",
		"componentTypes":["backend"],
		"env":[{"name":"APP_ENV","source":"value","value":"prod"}]
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/component-config-templates/"+stringID(tmpl.ID), bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update legacy template status = %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/component-config-templates/"+stringID(tmpl.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete legacy template status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func stringID(id uint) string {
	data, _ := json.Marshal(id)
	return string(data)
}

func setupComponentConfigTemplateTest(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Permission{}, &model.Role{}, &model.RolePermission{}, &model.RoleBinding{}, &model.ComponentConfigTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	seedApplicationRBACForTest(t, db)

	passwordHash, err := hashPassword("admin123")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := model.User{Username: "admin", Email: "admin@example.test", Password: passwordHash}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	bindSystemRoleForTest(t, db, user.ID, model.RolePlatformAdmin)
	token, err := middleware.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRouter(router)
	return router, token
}

func writeComponentConfigTemplatePackage(t *testing.T, files map[string]string) string {
	t.Helper()
	archivePath := filepath.Join(t.TempDir(), "template.tar.gz")
	out, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		data := []byte(content)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close package: %v", err)
	}
	return archivePath
}
