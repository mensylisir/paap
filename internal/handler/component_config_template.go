package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"paap/internal/database"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const componentConfigTemplateSyntax = `Use [[paap:<field>]] placeholders in env values or config files. Common options: default=<value>, secret=true, output=configMap|secret. PAAP also supports {{componentName}}, {{configMapName}}, and {{secretName}} runtime tokens.`

type componentConfigTemplateRequest struct {
	Key            string                   `json:"key"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	Framework      string                   `json:"framework"`
	BindingMode    string                   `json:"bindingMode"`
	ComponentTypes []string                 `json:"componentTypes"`
	Syntax         string                   `json:"syntax"`
	Fields         []map[string]interface{} `json:"fields"`
	Env            []map[string]interface{} `json:"env"`
	ConfigMaps     []map[string]interface{} `json:"configMaps"`
	Secrets        []map[string]interface{} `json:"secrets"`
	Files          []map[string]interface{} `json:"files"`
	Command        []string                 `json:"command"`
	Args           []string                 `json:"args"`
	Enabled        *bool                    `json:"enabled"`
}

type componentConfigTemplateResponse struct {
	ID             uint                     `json:"id"`
	Key            string                   `json:"key"`
	Name           string                   `json:"name"`
	Description    string                   `json:"description"`
	Framework      string                   `json:"framework"`
	BindingMode    string                   `json:"bindingMode"`
	ComponentTypes []string                 `json:"componentTypes"`
	Syntax         string                   `json:"syntax"`
	Fields         []map[string]interface{} `json:"fields"`
	Env            []map[string]interface{} `json:"env"`
	ConfigMaps     []map[string]interface{} `json:"configMaps"`
	Secrets        []map[string]interface{} `json:"secrets"`
	Files          []map[string]interface{} `json:"files"`
	Command        []string                 `json:"command"`
	Args           []string                 `json:"args"`
	IsBuiltin      bool                     `json:"isBuiltin"`
	SortOrder      int                      `json:"sortOrder"`
	Enabled        bool                     `json:"enabled"`
}

// ListComponentConfigTemplates returns global component runtime configuration
// templates. Built-ins are seeded at server startup and user templates are
// stored in the same table.
func ListComponentConfigTemplates(c *gin.Context) {
	var templates []model.ComponentConfigTemplate
	if err := database.DB.Where("enabled = ?", true).Order("sort_order ASC, name ASC").Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]componentConfigTemplateResponse, 0, len(templates))
	for _, tmpl := range templates {
		items = append(items, componentConfigTemplateToResponse(tmpl))
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

// CreateComponentConfigTemplate creates or updates a custom component config
// template. Built-in templates are immutable through this endpoint.
func CreateComponentConfigTemplate(c *gin.Context) {
	var req componentConfigTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tmpl, err := componentConfigTemplateFromRequest(req, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing model.ComponentConfigTemplate
	err = database.DB.Where("key = ?", tmpl.Key).First(&existing).Error
	if err == nil {
		if existing.IsBuiltin {
			c.JSON(http.StatusConflict, gin.H{"error": "built-in component config template key is reserved"})
			return
		}
		tmpl.ID = existing.ID
		if err := database.DB.Model(&existing).Updates(componentConfigTemplateUpdateMap(tmpl)).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := database.DB.First(&existing, existing.ID).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": componentConfigTemplateToResponse(existing)})
		return
	}
	if err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Create(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": componentConfigTemplateToResponse(tmpl)})
}

// UpdateComponentConfigTemplate updates a custom component config template.
func UpdateComponentConfigTemplate(c *gin.Context) {
	var existing model.ComponentConfigTemplate
	if err := database.DB.First(&existing, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	if existing.IsBuiltin {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in component config templates cannot be edited"})
		return
	}
	var req componentConfigTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tmpl, err := componentConfigTemplateFromRequest(req, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tmpl.ID = existing.ID
	if err := database.DB.Model(&existing).Updates(componentConfigTemplateUpdateMap(tmpl)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.First(&existing, existing.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": componentConfigTemplateToResponse(existing)})
}

// DeleteComponentConfigTemplate deletes a custom component config template.
func DeleteComponentConfigTemplate(c *gin.Context) {
	var tmpl model.ComponentConfigTemplate
	if err := database.DB.First(&tmpl, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
		return
	}
	if tmpl.IsBuiltin {
		c.JSON(http.StatusForbidden, gin.H{"error": "built-in component config templates cannot be deleted"})
		return
	}
	if err := database.DB.Unscoped().Delete(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// SyncBuiltinComponentConfigTemplates refreshes the built-in component runtime
// config templates.
func SyncBuiltinComponentConfigTemplates(c *gin.Context) {
	updated := SeedComponentConfigTemplates()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"updated": updated}})
}

func SeedComponentConfigTemplates() int {
	templates := builtinComponentConfigTemplates()
	updated := 0
	for _, tmpl := range templates {
		var existing model.ComponentConfigTemplate
		if err := database.DB.Where("key = ?", tmpl.Key).Assign(tmpl).FirstOrCreate(&existing).Error; err != nil {
			log.Printf("[SeedComponentConfigTemplates] failed to seed %s: %v", tmpl.Key, err)
			continue
		}
		updated++
	}
	return updated
}

func componentConfigTemplateFromRequest(req componentConfigTemplateRequest, builtin bool) (model.ComponentConfigTemplate, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.ComponentConfigTemplate{}, fmt.Errorf("template name is required")
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		key = "custom-" + slugifyTemplateKey(name)
	}
	if key == "custom-" {
		return model.ComponentConfigTemplate{}, fmt.Errorf("template key is required")
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	syntax := strings.TrimSpace(req.Syntax)
	if syntax == "" {
		syntax = componentConfigTemplateSyntax
	}
	return model.ComponentConfigTemplate{
		Key:            key,
		Name:           name,
		Description:    strings.TrimSpace(req.Description),
		Framework:      firstNonEmpty(strings.TrimSpace(req.Framework), "auto"),
		BindingMode:    firstNonEmpty(strings.TrimSpace(req.BindingMode), "recommended"),
		ComponentTypes: mustJSON(req.ComponentTypes),
		Syntax:         syntax,
		FieldsJSON:     mustJSON(req.Fields),
		EnvJSON:        mustJSON(req.Env),
		ConfigJSON:     mustJSON(req.ConfigMaps),
		SecretJSON:     mustJSON(req.Secrets),
		FileJSON:       mustJSON(req.Files),
		CommandJSON:    mustJSON(req.Command),
		ArgsJSON:       mustJSON(req.Args),
		IsBuiltin:      builtin,
		SortOrder:      1000,
		Enabled:        enabled,
	}, nil
}

func componentConfigTemplateUpdateMap(tmpl model.ComponentConfigTemplate) map[string]interface{} {
	return map[string]interface{}{
		"key":             tmpl.Key,
		"name":            tmpl.Name,
		"description":     tmpl.Description,
		"framework":       tmpl.Framework,
		"binding_mode":    tmpl.BindingMode,
		"component_types": tmpl.ComponentTypes,
		"syntax":          tmpl.Syntax,
		"fields_json":     tmpl.FieldsJSON,
		"env_json":        tmpl.EnvJSON,
		"config_json":     tmpl.ConfigJSON,
		"secret_json":     tmpl.SecretJSON,
		"file_json":       tmpl.FileJSON,
		"command_json":    tmpl.CommandJSON,
		"args_json":       tmpl.ArgsJSON,
		"enabled":         tmpl.Enabled,
	}
}

func componentConfigTemplateToResponse(tmpl model.ComponentConfigTemplate) componentConfigTemplateResponse {
	return componentConfigTemplateResponse{
		ID:             tmpl.ID,
		Key:            tmpl.Key,
		Name:           tmpl.Name,
		Description:    tmpl.Description,
		Framework:      firstNonEmpty(tmpl.Framework, "auto"),
		BindingMode:    firstNonEmpty(tmpl.BindingMode, "recommended"),
		ComponentTypes: decodeStringArray(tmpl.ComponentTypes),
		Syntax:         firstNonEmpty(tmpl.Syntax, componentConfigTemplateSyntax),
		Fields:         decodeObjectArray(tmpl.FieldsJSON),
		Env:            decodeObjectArray(tmpl.EnvJSON),
		ConfigMaps:     decodeObjectArray(tmpl.ConfigJSON),
		Secrets:        decodeObjectArray(tmpl.SecretJSON),
		Files:          decodeObjectArray(tmpl.FileJSON),
		Command:        decodeStringArray(tmpl.CommandJSON),
		Args:           decodeStringArray(tmpl.ArgsJSON),
		IsBuiltin:      tmpl.IsBuiltin,
		SortOrder:      tmpl.SortOrder,
		Enabled:        tmpl.Enabled,
	}
}

var templateKeyCleaner = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyTemplateKey(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	key = templateKeyCleaner.ReplaceAllString(key, "-")
	key = strings.Trim(key, "-")
	if len(key) > 56 {
		key = strings.Trim(key[:56], "-")
	}
	return key
}

func mustJSON(value interface{}) string {
	if value == nil {
		return "[]"
	}
	data, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func decodeObjectArray(raw string) []map[string]interface{} {
	if strings.TrimSpace(raw) == "" {
		return []map[string]interface{}{}
	}
	var out []map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []map[string]interface{}{}
	}
	if out == nil {
		return []map[string]interface{}{}
	}
	return out
}

func decodeStringArray(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []string{}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func builtinComponentConfigTemplates() []model.ComponentConfigTemplate {
	template := func(key, name, description, framework string, componentTypes []string, sortOrder int, fields, env, configMaps, secrets, files []map[string]interface{}, command, args []string) model.ComponentConfigTemplate {
		return model.ComponentConfigTemplate{
			Key:            key,
			Name:           name,
			Description:    description,
			Framework:      framework,
			BindingMode:    "recommended",
			ComponentTypes: mustJSON(componentTypes),
			Syntax:         componentConfigTemplateSyntax,
			FieldsJSON:     mustJSON(fields),
			EnvJSON:        mustJSON(env),
			ConfigJSON:     mustJSON(configMaps),
			SecretJSON:     mustJSON(secrets),
			FileJSON:       mustJSON(files),
			CommandJSON:    mustJSON(command),
			ArgsJSON:       mustJSON(args),
			IsBuiltin:      true,
			SortOrder:      sortOrder,
			Enabled:        true,
		}
	}
	field := func(key, label, typ, target, output, def string, required bool) map[string]interface{} {
		item := map[string]interface{}{"key": key, "label": label, "type": typ}
		if target != "" {
			item["target"] = target
		}
		if output != "" {
			item["output"] = output
		}
		if def != "" {
			item["default"] = def
		}
		if required {
			item["required"] = true
		}
		return item
	}
	env := func(name, source, value, refName, refKey string) map[string]interface{} {
		item := map[string]interface{}{"name": name, "source": source}
		if value != "" {
			item["value"] = value
		}
		if refName != "" {
			item["refName"] = refName
		}
		if refKey != "" {
			item["refKey"] = refKey
		}
		return item
	}
	configMap := func(name string, data map[string]string) map[string]interface{} {
		return map[string]interface{}{"name": name, "data": data}
	}
	file := func(name, configMapName, key, mountPath string) map[string]interface{} {
		return map[string]interface{}{"name": name, "configMapName": configMapName, "key": key, "mountPath": mountPath, "readOnly": true}
	}

	return []model.ComponentConfigTemplate{
		template(
			"nginx-spa-api-proxy",
			"Nginx 静态前端 + API 代理",
			"为 Vue/React/Vite 静态前端生成 nginx default.conf，并把 /api 代理到后端组件。",
			"nginx",
			[]string{"frontend"},
			10,
			[]map[string]interface{}{
				field("backend.url", "后端地址", "serviceRef", "backend", "configMap", "http://backend", true),
				field("listen.port", "监听端口", "number", "", "configMap", "80", false),
			},
			[]map[string]interface{}{
				env("BACKEND_URL", "value", "[[paap:backend.url default=http://backend]]", "", ""),
			},
			[]map[string]interface{}{
				configMap("{{configMapName}}", map[string]string{"default.conf": strings.Join([]string{
					"server {",
					"  listen [[paap:listen.port default=80]];",
					"  server_name _;",
					"  root /usr/share/nginx/html;",
					"  index index.html;",
					"",
					"  location / {",
					"    try_files $uri $uri/ /index.html;",
					"  }",
					"",
					"  location /api/ {",
					"    proxy_pass [[paap:backend.url default=http://backend]];",
					"    proxy_http_version 1.1;",
					"    proxy_set_header Host $host;",
					"    proxy_set_header X-Real-IP $remote_addr;",
					"    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
					"    proxy_set_header X-Forwarded-Proto $scheme;",
					"  }",
					"}",
					"",
				}, "\n")}),
			},
			nil,
			[]map[string]interface{}{file("nginx-default-conf", "{{configMapName}}", "default.conf", "/etc/nginx/conf.d/default.conf")},
			nil,
			nil,
		),
		template(
			"springboot-postgres-redis",
			"Spring Boot + PostgreSQL + Redis",
			"生成 Spring Boot 外部 application-paap.yml，并把数据库和 Redis 密码作为 Secret 注入。",
			"springboot",
			[]string{"backend"},
			20,
			[]map[string]interface{}{
				field("database.jdbcUrl", "数据库 JDBC URL", "serviceRef", "postgresql|mysql", "configMap", "jdbc:postgresql://postgresql:5432/postgres", true),
				field("database.username", "数据库用户名", "text", "", "configMap", "postgres", true),
				field("database.password", "数据库密码", "password", "", "secret", "", true),
				field("redis.host", "Redis 地址", "serviceRef", "redis", "configMap", "redis-master", false),
				field("redis.password", "Redis 密码", "password", "", "secret", "", false),
			},
			[]map[string]interface{}{
				env("SPRING_PROFILES_ACTIVE", "value", "prod", "", ""),
				env("SPRING_CONFIG_ADDITIONAL_LOCATION", "value", "file:/etc/paap/", "", ""),
				env("SPRING_DATASOURCE_PASSWORD", "secret", "", "{{secretName}}", "SPRING_DATASOURCE_PASSWORD"),
				env("REDIS_PASSWORD", "secret", "", "{{secretName}}", "REDIS_PASSWORD"),
			},
			[]map[string]interface{}{
				configMap("{{configMapName}}", map[string]string{"application-paap.yml": strings.Join([]string{
					"spring:",
					"  datasource:",
					"    url: [[paap:database.jdbcUrl default=jdbc:postgresql://postgresql:5432/postgres]]",
					"    username: [[paap:database.username default=postgres]]",
					"    password: ${SPRING_DATASOURCE_PASSWORD}",
					"  data:",
					"    redis:",
					"      host: [[paap:redis.host default=redis-master]]",
					"      port: [[paap:redis.port default=6379]]",
					"      password: ${REDIS_PASSWORD}",
					"",
				}, "\n")}),
			},
			[]map[string]interface{}{
				configMap("{{secretName}}", map[string]string{
					"SPRING_DATASOURCE_PASSWORD": "[[paap:database.password]]",
					"REDIS_PASSWORD":             "[[paap:redis.password]]",
				}),
			},
			[]map[string]interface{}{file("spring-application-paap", "{{configMapName}}", "application-paap.yml", "/etc/paap/application-paap.yml")},
			nil,
			nil,
		),
		template(
			"node-express-api",
			"Node.js / Express API",
			"Node、Express、NestJS 常用运行变量，包含数据库、Redis 和 JWT Secret。",
			"node",
			[]string{"backend"},
			30,
			[]map[string]interface{}{
				field("database.url", "DATABASE_URL", "serviceRef", "postgresql|mysql|mongodb", "secret", "", false),
				field("redis.url", "REDIS_URL", "serviceRef", "redis", "secret", "", false),
				field("jwt.secret", "JWT Secret", "password", "", "secret", "", true),
			},
			[]map[string]interface{}{
				env("NODE_ENV", "value", "production", "", ""),
				env("PORT", "value", "3000", "", ""),
				env("DATABASE_URL", "secret", "", "{{secretName}}", "DATABASE_URL"),
				env("REDIS_URL", "secret", "", "{{secretName}}", "REDIS_URL"),
				env("JWT_SECRET", "secret", "", "{{secretName}}", "JWT_SECRET"),
			},
			nil,
			[]map[string]interface{}{
				configMap("{{secretName}}", map[string]string{
					"DATABASE_URL": "[[paap:database.url]]",
					"REDIS_URL":    "[[paap:redis.url]]",
					"JWT_SECRET":   "[[paap:jwt.secret]]",
				}),
			},
			nil,
			nil,
			nil,
		),
		template(
			"go-gin-api",
			"Go Gin API",
			"Gin/Fiber/标准 Go API 常用运行变量，包含数据库、Redis 和 JWT Secret。",
			"go",
			[]string{"backend"},
			40,
			[]map[string]interface{}{
				field("database.url", "数据库连接串", "serviceRef", "postgresql|mysql", "secret", "", false),
				field("redis.addr", "Redis 地址", "serviceRef", "redis", "configMap", "redis-master:6379", false),
				field("jwt.secret", "JWT Secret", "password", "", "secret", "", false),
			},
			[]map[string]interface{}{
				env("APP_ENV", "value", "production", "", ""),
				env("PORT", "value", "8080", "", ""),
				env("DATABASE_URL", "secret", "", "{{secretName}}", "DATABASE_URL"),
				env("REDIS_ADDR", "value", "[[paap:redis.addr default=redis-master:6379]]", "", ""),
				env("JWT_SECRET", "secret", "", "{{secretName}}", "JWT_SECRET"),
			},
			nil,
			[]map[string]interface{}{
				configMap("{{secretName}}", map[string]string{
					"DATABASE_URL": "[[paap:database.url]]",
					"JWT_SECRET":   "[[paap:jwt.secret]]",
				}),
			},
			nil,
			nil,
			nil,
		),
		template(
			"python-django-fastapi",
			"Python Django / FastAPI",
			"Django、FastAPI、Flask 常用运行变量，包含数据库、Redis 和应用密钥。",
			"python",
			[]string{"backend"},
			50,
			[]map[string]interface{}{
				field("database.url", "DATABASE_URL", "serviceRef", "postgresql|mysql", "secret", "", false),
				field("redis.url", "REDIS_URL", "serviceRef", "redis", "secret", "", false),
				field("app.secret", "应用 Secret", "password", "", "secret", "", true),
			},
			[]map[string]interface{}{
				env("APP_ENV", "value", "production", "", ""),
				env("PORT", "value", "8000", "", ""),
				env("DATABASE_URL", "secret", "", "{{secretName}}", "DATABASE_URL"),
				env("REDIS_URL", "secret", "", "{{secretName}}", "REDIS_URL"),
				env("DJANGO_SECRET_KEY", "secret", "", "{{secretName}}", "DJANGO_SECRET_KEY"),
				env("SECRET_KEY", "secret", "", "{{secretName}}", "SECRET_KEY"),
			},
			nil,
			[]map[string]interface{}{
				configMap("{{secretName}}", map[string]string{
					"DATABASE_URL":      "[[paap:database.url]]",
					"REDIS_URL":         "[[paap:redis.url]]",
					"DJANGO_SECRET_KEY": "[[paap:app.secret]]",
					"SECRET_KEY":        "[[paap:app.secret]]",
				}),
			},
			nil,
			nil,
			nil,
		),
		template(
			"frontend-runtime-api-vars",
			"前端运行 API 变量",
			"React/Vue/Vite/Next.js 常用 API 地址变量，适合非 nginx 运行时前端。",
			"node",
			[]string{"frontend"},
			60,
			[]map[string]interface{}{
				field("backend.url", "后端地址", "serviceRef", "backend", "configMap", "http://backend", true),
			},
			[]map[string]interface{}{
				env("BACKEND_URL", "value", "[[paap:backend.url default=http://backend]]", "", ""),
				env("API_BASE_URL", "value", "[[paap:backend.url default=http://backend]]", "", ""),
				env("VITE_API_BASE_URL", "value", "[[paap:backend.url default=http://backend]]", "", ""),
				env("NEXT_PUBLIC_API_URL", "value", "[[paap:backend.url default=http://backend]]", "", ""),
			},
			nil,
			nil,
			nil,
			nil,
			nil,
		),
	}
}
