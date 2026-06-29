package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"paap/internal/authz"
	"paap/internal/database"
	"paap/internal/middleware"
	"paap/internal/model"
	"paap/internal/permission"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func withTestAuthUser(userID uint, next gin.HandlerFunc) gin.HandlerFunc {
	return withTestAuthUserRoles(userID, []string{model.RoleUser}, next)
}

func withTestAuthUserRole(userID uint, role string, next gin.HandlerFunc) gin.HandlerFunc {
	return withTestAuthUserRoles(userID, []string{role}, next)
}

func withTestAuthUserRoles(userID uint, roles []string, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.ContextUserIDKey, userID)
		c.Set(middleware.ContextUserRolesKey, roles)
		next(c)
	}
}

func withTestAuthUserContext(userID uint) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.ContextUserIDKey, userID)
		c.Set(middleware.ContextUserRolesKey, []string{model.RoleUser})
		c.Next()
	}
}

func seedApplicationRBACForTest(t *testing.T, db *gorm.DB) {
	t.Helper()

	permissions := []model.Permission{
		{Code: permission.SystemUserRead, Name: "查看用户", ScopeType: model.ScopeSystem, Resource: "user", Action: "read", Enabled: true},
		{Code: permission.SystemUserManage, Name: "管理用户", ScopeType: model.ScopeSystem, Resource: "user", Action: "manage", Enabled: true},
		{Code: permission.SystemRoleManage, Name: "管理角色", ScopeType: model.ScopeSystem, Resource: "role", Action: "manage", Enabled: true},
		{Code: permission.SystemTemplateManage, Name: "管理模板", ScopeType: model.ScopeSystem, Resource: "template", Action: "manage", Enabled: true},
		{Code: permission.SystemSharedPoolManage, Name: "管理共享资源池", ScopeType: model.ScopeSystem, Resource: "shared_pool", Action: "manage", Enabled: true},
		{Code: permission.AppCreate, Name: "创建应用", ScopeType: model.ScopeSystem, Resource: "app", Action: "create", Enabled: true},
		{Code: permission.AppRead, Name: "查看应用", ScopeType: model.ScopeApp, Resource: "app", Action: "read", Enabled: true},
		{Code: permission.AppUpdate, Name: "编辑应用", ScopeType: model.ScopeApp, Resource: "app", Action: "update", Enabled: true},
		{Code: permission.AppDelete, Name: "删除应用", ScopeType: model.ScopeApp, Resource: "app", Action: "delete", Enabled: true},
		{Code: permission.AppMemberRead, Name: "查看成员", ScopeType: model.ScopeApp, Resource: "member", Action: "read", Enabled: true},
		{Code: permission.AppMemberManage, Name: "管理成员", ScopeType: model.ScopeApp, Resource: "member", Action: "manage", Enabled: true},
		{Code: permission.EnvCreate, Name: "创建环境", ScopeType: model.ScopeApp, Resource: "env", Action: "create", Enabled: true},
		{Code: permission.EnvRead, Name: "查看环境", ScopeType: model.ScopeEnv, Resource: "env", Action: "read", Enabled: true},
		{Code: permission.EnvManage, Name: "管理环境", ScopeType: model.ScopeEnv, Resource: "env", Action: "manage", Enabled: true},
		{Code: permission.EnvDelete, Name: "删除环境", ScopeType: model.ScopeEnv, Resource: "env", Action: "delete", Enabled: true},
		{Code: permission.ServiceRead, Name: "查看服务", ScopeType: model.ScopeEnv, Resource: "service", Action: "read", Enabled: true},
		{Code: permission.ServiceInstall, Name: "安装服务", ScopeType: model.ScopeEnv, Resource: "service", Action: "install", Enabled: true},
		{Code: permission.ServiceManage, Name: "管理服务", ScopeType: model.ScopeEnv, Resource: "service", Action: "manage", Enabled: true},
		{Code: permission.ComponentRead, Name: "查看组件", ScopeType: model.ScopeEnv, Resource: "component", Action: "read", Enabled: true},
		{Code: permission.ComponentCreate, Name: "创建组件", ScopeType: model.ScopeEnv, Resource: "component", Action: "create", Enabled: true},
		{Code: permission.ComponentDeploy, Name: "部署组件", ScopeType: model.ScopeEnv, Resource: "component", Action: "deploy", Enabled: true},
		{Code: permission.ComponentManage, Name: "管理组件", ScopeType: model.ScopeEnv, Resource: "component", Action: "manage", Enabled: true},
	}
	if err := db.Create(&permissions).Error; err != nil {
		t.Fatalf("seed permissions: %v", err)
	}

	roles := []model.Role{
		{Code: model.RolePlatformAdmin, Name: "平台管理员", ScopeType: model.ScopeSystem, Builtin: true, Editable: false, Enabled: true},
		{Code: model.RoleAppAdmin, Name: "应用管理员", ScopeType: model.ScopeSystem, Builtin: true, Editable: false, Enabled: true},
		{Code: model.RoleUser, Name: "普通用户", ScopeType: model.ScopeSystem, Builtin: true, Editable: false, Enabled: true},
		{Code: model.AppRoleAdmin, Name: "应用管理员", ScopeType: model.ScopeApp, Builtin: true, Editable: false, Enabled: true},
		{Code: model.AppRoleMember, Name: "应用成员", ScopeType: model.ScopeApp, Builtin: true, Editable: false, Enabled: true},
		{Code: model.AppRoleViewer, Name: "只读成员", ScopeType: model.ScopeApp, Builtin: true, Editable: false, Enabled: true},
	}
	if err := db.Create(&roles).Error; err != nil {
		t.Fatalf("seed roles: %v", err)
	}

	permissionIDs := map[string]uint{}
	for _, item := range permissions {
		permissionIDs[item.Code] = item.ID
	}
	rolePermissions := make([]model.RolePermission, 0, 6)
	for _, role := range roles {
		switch {
		case role.Code == model.RolePlatformAdmin:
			for _, perm := range permissions {
				rolePermissions = append(rolePermissions, model.RolePermission{RoleID: role.ID, PermissionID: perm.ID})
			}
		case role.Code == model.RoleAppAdmin && role.ScopeType == model.ScopeSystem:
			rolePermissions = append(rolePermissions, model.RolePermission{RoleID: role.ID, PermissionID: permissionIDs[permission.AppCreate]})
		case role.Code == model.AppRoleAdmin && role.ScopeType == model.ScopeApp:
			for _, perm := range permissions {
				if perm.ScopeType == model.ScopeApp || perm.ScopeType == model.ScopeEnv {
					rolePermissions = append(rolePermissions, model.RolePermission{RoleID: role.ID, PermissionID: perm.ID})
				}
			}
		case role.Code == model.AppRoleMember && role.ScopeType == model.ScopeApp:
			for _, code := range []string{
				permission.AppRead, permission.AppMemberRead, permission.EnvCreate,
				permission.EnvRead, permission.EnvManage, permission.ServiceRead, permission.ServiceInstall, permission.ServiceManage,
				permission.ComponentRead, permission.ComponentCreate, permission.ComponentDeploy, permission.ComponentManage,
			} {
				rolePermissions = append(rolePermissions, model.RolePermission{RoleID: role.ID, PermissionID: permissionIDs[code]})
			}
		case role.Code == model.AppRoleViewer && role.ScopeType == model.ScopeApp:
			for _, code := range []string{permission.AppRead, permission.AppMemberRead, permission.EnvRead, permission.ServiceRead, permission.ComponentRead} {
				rolePermissions = append(rolePermissions, model.RolePermission{RoleID: role.ID, PermissionID: permissionIDs[code]})
			}
		}
	}
	if err := db.Create(&rolePermissions).Error; err != nil {
		t.Fatalf("seed role permissions: %v", err)
	}
}

func bindSystemRoleForTest(t *testing.T, db *gorm.DB, userID uint, roleCode string) {
	t.Helper()
	if err := authz.BindRole(db, userID, roleCode, authz.SystemScope(), userID); err != nil {
		t.Fatalf("bind %s role: %v", roleCode, err)
	}
}

func TestCreateApplicationRequiresAppCreatePermission(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.AppMember{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedApplicationRBACForTest(t, db)
	database.DB = db
	if err := authz.BindRole(db, 1, model.RoleAppAdmin, authz.SystemScope(), 0); err != nil {
		t.Fatalf("bind app_admin: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(
		"/api/v1/applications",
		withTestAuthUserContext(1),
		middleware.RequireSystemPermission(permission.AppCreate),
		CreateApplication,
	)

	rec := httptest.NewRecorder()
	body, _ := json.Marshal(CreateAppRequest{Name: "有权限应用"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("app_admin status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	body, _ = json.Marshal(CreateAppRequest{Name: "无权限应用"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router = gin.New()
	router.POST(
		"/api/v1/applications",
		withTestAuthUserContext(2),
		middleware.RequireSystemPermission(permission.AppCreate),
		CreateApplication,
	)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("regular user status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestListApplicationsIncludesEnvironmentCounts(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
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

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
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

func TestListApplicationsIncludesSystemAppsForPlatformAdmins(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
		&model.User{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	seedApplicationRBACForTest(t, db)

	admin := model.User{Username: "admin", Email: "admin@example.com", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	bindSystemRoleForTest(t, db, admin.ID, model.RolePlatformAdmin)
	normal := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&normal).Error; err != nil {
		t.Fatalf("create normal app: %v", err)
	}
	systemApp := model.Application{Name: "共享资源池", Identifier: "default", OwnerID: admin.ID, IsSystem: true}
	if err := db.Create(&systemApp).Error; err != nil {
		t.Fatalf("create system app: %v", err)
	}
	systemEnv := model.Environment{ApplicationID: systemApp.ID, Name: "共享环境", Identifier: "shared", Namespace: "default-shared", IsSystem: true}
	if err := db.Create(&systemEnv).Error; err != nil {
		t.Fatalf("create system env: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/applications", withTestAuthUserRole(admin.ID, model.RolePlatformAdmin, ListApplications))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Data []struct {
			Name             string `json:"name"`
			Identifier       string `json:"identifier"`
			IsSystem         bool   `json:"isSystem"`
			EnvironmentCount int    `json:"environmentCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) < 2 || body.Data[0].Identifier != "default" || body.Data[0].Name != "共享资源池" || !body.Data[0].IsSystem {
		t.Fatalf("first application = %#v, want shared resource pool first", body.Data)
	}
	seen := map[string]struct {
		IsSystem         bool
		EnvironmentCount int
	}{}
	for _, app := range body.Data {
		seen[app.Identifier] = struct {
			IsSystem         bool
			EnvironmentCount int
		}{IsSystem: app.IsSystem, EnvironmentCount: app.EnvironmentCount}
	}
	if !seen["default"].IsSystem || seen["default"].EnvironmentCount != 1 {
		t.Fatalf("default system app missing or malformed: %#v", body.Data)
	}
	if _, ok := seen["billing"]; !ok {
		t.Fatalf("normal app missing: %#v", body.Data)
	}
}

func TestListApplicationsDoesNotCreateSharedResourcePool(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
		&model.User{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	seedApplicationRBACForTest(t, db)

	admin := model.User{Username: "admin", Email: "admin@example.com", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	bindSystemRoleForTest(t, db, admin.ID, model.RolePlatformAdmin)
	normal := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&normal).Error; err != nil {
		t.Fatalf("create normal app: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/applications", withTestAuthUserRole(admin.ID, model.RolePlatformAdmin, ListApplications))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Data []struct {
			Identifier string `json:"identifier"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Identifier != "billing" {
		t.Fatalf("applications = %#v, want only existing business app", body.Data)
	}

	var appCount int64
	if err := db.Model(&model.Application{}).Where("identifier = ?", "default").Count(&appCount).Error; err != nil {
		t.Fatalf("count shared app: %v", err)
	}
	if appCount != 0 {
		t.Fatalf("shared app count = %d, want ListApplications to be read-only", appCount)
	}
	var envCount int64
	if err := db.Model(&model.Environment{}).Where("identifier = ?", "shared").Count(&envCount).Error; err != nil {
		t.Fatalf("count shared env: %v", err)
	}
	if envCount != 0 {
		t.Fatalf("shared env count = %d, want ListApplications to be read-only", envCount)
	}
}

func TestCreateApplicationGeneratesIdentifierWhenMissing(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.AppMember{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedApplicationRBACForTest(t, db)
	database.DB = db

	body, _ := json.Marshal(CreateAppRequest{Name: "订单服务"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/applications", withTestAuthUserRole(1, model.RoleAppAdmin, CreateApplication))
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

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.AppMember{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedApplicationRBACForTest(t, db)
	database.DB = db

	body, _ := json.Marshal(CreateAppRequest{Name: "结算服务"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/applications", withTestAuthUserRole(42, model.RoleAppAdmin, CreateApplication))
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

func TestCreateApplicationRejectsRegularUser(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.AppMember{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seedApplicationRBACForTest(t, db)
	database.DB = db

	body, _ := json.Marshal(CreateAppRequest{Name: "普通用户应用"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(
		"/api/v1/applications",
		withTestAuthUserContext(42),
		middleware.RequireSystemPermission(permission.AppCreate),
		CreateApplication,
	)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "permission denied") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestGetApplicationIncludesEnvironmentCounts(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
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

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
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

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
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

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
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

func TestDeleteApplicationRejectsSystemApplications(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
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

	app := model.Application{Name: "default", Identifier: "default", OwnerID: 1, IsSystem: true}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/applications/:id", withTestAuthUserRole(1, model.RolePlatformAdmin, DeleteApplication))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/applications/1", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "system applications cannot be deleted") {
		t.Fatalf("body = %s, want system app delete guard", rec.Body.String())
	}
}

func TestListApplicationMembersReturnsUsersForAppMembers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
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
		{Username: "owner", Email: "owner@example.com", Password: "x"},
		{Username: "alice", Email: "alice@example.com", Password: "x"},
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

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	owner := model.User{Username: "owner", Email: "owner@example.com", Password: "x"}
	target := model.User{Username: "bob", Email: "bob@example.com", Password: "x"}
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

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	admin := model.User{Username: "owner", Email: "owner@example.com", Password: "x"}
	memberUser := model.User{Username: "alice", Email: "alice@example.com", Password: "x"}
	targetUser := model.User{Username: "bob", Email: "bob@example.com", Password: "x"}
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

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	admin := model.User{Username: "owner", Email: "owner@example.com", Password: "x"}
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
