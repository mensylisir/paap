package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/middleware"
	"paap/internal/model"
	"paap/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func setupCapabilityTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Permission{},
		&model.Role{},
		&model.RolePermission{},
		&model.RoleBinding{},
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.EnvironmentCapability{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	seedApplicationRBACForTest(t, db)
	return db
}

func createCapabilityTestPlatformAdmin(t *testing.T, db *gorm.DB) model.User {
	t.Helper()
	admin := model.User{Username: "admin", Email: "admin@example.test", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	bindSystemRoleForTest(t, db, admin.ID, model.RolePlatformAdmin)
	bindSystemRoleForTest(t, db, admin.ID, model.RoleAppAdmin)
	return admin
}

func createCapabilityTestSharedEnvironment(t *testing.T, db *gorm.DB, ownerID uint) (model.Application, model.Environment) {
	t.Helper()
	app := model.Application{
		Name:        systemSharedApplicationName,
		Identifier:  systemSharedApplicationIdentifier,
		Description: "PAAP platform shared resource pool",
		OwnerID:     ownerID,
		IsSystem:    true,
	}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create shared app: %v", err)
	}
	env := model.Environment{
		ApplicationID: app.ID,
		Name:          systemSharedEnvironmentName,
		Identifier:    systemSharedEnvironmentIdentifier,
		Status:        "empty",
		Namespace:     app.Identifier + "-" + systemSharedEnvironmentIdentifier,
		IsSystem:      true,
	}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create shared env: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: ownerID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create shared app member: %v", err)
	}
	return app, env
}

func TestGetSharedResourcePoolDoesNotCreateMissingResourcePool(t *testing.T) {
	db := setupCapabilityTestDB(t)
	admin := createCapabilityTestPlatformAdmin(t, db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/admin/shared-resource-pool", withTestAuthUserRole(admin.ID, model.RolePlatformAdmin, GetSharedResourcePool))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/shared-resource-pool", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rec.Code, rec.Body.String())
	}
	var appCount int64
	if err := db.Model(&model.Application{}).Where("identifier = ?", systemSharedApplicationIdentifier).Count(&appCount).Error; err != nil {
		t.Fatalf("count shared app: %v", err)
	}
	if appCount != 0 {
		t.Fatalf("shared app count = %d, want read endpoint to avoid creating DB rows", appCount)
	}
}

func TestSharedCapabilityResourcesRequireSharedPoolPermission(t *testing.T) {
	t.Setenv("JWT_SECRET", "shared-capability-route-test-secret")
	db := setupCapabilityTestDB(t)
	admin := createCapabilityTestPlatformAdmin(t, db)
	_, sharedEnv := createCapabilityTestSharedEnvironment(t, db, admin.ID)
	if err := db.Create(&model.ServiceInstallation{
		EnvironmentID: sharedEnv.ID,
		ServiceType:   "postgresql",
		ServiceName:   "shared-postgresql",
		Status:        "running",
		Namespace:     "paap-shared-postgresql",
	}).Error; err != nil {
		t.Fatalf("create shared service: %v", err)
	}
	user := model.User{Username: "viewer", Email: "viewer@example.test", Password: "x"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create viewer: %v", err)
	}
	viewerToken, err := middleware.GenerateToken(user.ID)
	if err != nil {
		t.Fatalf("generate viewer token: %v", err)
	}
	adminToken, err := middleware.GenerateToken(admin.ID)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	SetupRouter(router)

	forbiddenRec := httptest.NewRecorder()
	forbiddenReq := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/shared-resources", nil)
	forbiddenReq.Header.Set("Authorization", "Bearer "+viewerToken)
	router.ServeHTTP(forbiddenRec, forbiddenReq)
	if forbiddenRec.Code != http.StatusForbidden {
		t.Fatalf("viewer status = %d, want %d; body=%s", forbiddenRec.Code, http.StatusForbidden, forbiddenRec.Body.String())
	}

	okRec := httptest.NewRecorder()
	okReq := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities/shared-resources", nil)
	okReq.Header.Set("Authorization", "Bearer "+adminToken)
	router.ServeHTTP(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("admin status = %d, want %d; body=%s", okRec.Code, http.StatusOK, okRec.Body.String())
	}
	if !strings.Contains(okRec.Body.String(), "shared-postgresql") {
		t.Fatalf("admin response missing shared resource: %s", okRec.Body.String())
	}
}

func TestGetSharedResourcePoolReturnsSystemCanvasTarget(t *testing.T) {
	db := setupCapabilityTestDB(t)
	admin := createCapabilityTestPlatformAdmin(t, db)
	createCapabilityTestSharedEnvironment(t, db, admin.ID)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/admin/shared-resource-pool", withTestAuthUserRole(admin.ID, model.RolePlatformAdmin, GetSharedResourcePool))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/shared-resource-pool", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data struct {
			Application model.Application `json:"application"`
			Environment model.Environment `json:"environment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Data.Application.Identifier != systemSharedApplicationIdentifier || !body.Data.Application.IsSystem {
		t.Fatalf("application = %#v, want system shared app", body.Data.Application)
	}
	if body.Data.Environment.Identifier != systemSharedEnvironmentIdentifier || !body.Data.Environment.IsSystem {
		t.Fatalf("environment = %#v, want system shared environment", body.Data.Environment)
	}
}

func TestEnvironmentCapabilityCanUseManagedSharedAndExternalSources(t *testing.T) {
	db := setupCapabilityTestDB(t)

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	_, sharedEnv := createCapabilityTestSharedEnvironment(t, db, admin.ID)
	sharedHarbor := model.ServiceInstallation{
		EnvironmentID: sharedEnv.ID,
		ServiceType:   "harbor",
		ServiceName:   "shared-harbor",
		Status:        "running",
		Namespace:     "default-shared-harbor",
	}
	if err := db.Create(&sharedHarbor).Error; err != nil {
		t.Fatalf("create shared harbor: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/capabilities/:capability", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, UpsertEnvironmentCapability))
	router.GET("/api/v1/environments/:id/capabilities/:capability/credentials", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, GetEnvironmentCapabilityCredentials))
	router.GET("/api/v1/environments/:id/capabilities", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, ListEnvironmentCapabilities))

	sharedBody := bytes.NewBufferString(fmt.Sprintf(`{"source":"shared","provider":"harbor","serviceType":"harbor","refServiceId":%d}`, sharedHarbor.ID))
	sharedReq := httptest.NewRequest(http.MethodPut, "/api/v1/environments/1/capabilities/registry", sharedBody)
	sharedReq.Header.Set("Content-Type", "application/json")
	sharedRec := httptest.NewRecorder()
	router.ServeHTTP(sharedRec, sharedReq)
	if sharedRec.Code != http.StatusOK {
		t.Fatalf("shared capability status = %d, body=%s", sharedRec.Code, sharedRec.Body.String())
	}

	externalBody := bytes.NewBufferString(`{"source":"external","provider":"gitlab","externalEndpoint":"https://gitlab.example.com","credentialSecretRef":"paap/gitlab-token"}`)
	externalReq := httptest.NewRequest(http.MethodPut, "/api/v1/environments/1/capabilities/git", externalBody)
	externalReq.Header.Set("Content-Type", "application/json")
	externalRec := httptest.NewRecorder()
	router.ServeHTTP(externalRec, externalReq)
	if externalRec.Code != http.StatusOK {
		t.Fatalf("external capability status = %d, body=%s", externalRec.Code, externalRec.Body.String())
	}

	managedBody := bytes.NewBufferString(`{"source":"managed","provider":"postgresql","serviceType":"postgresql"}`)
	managedReq := httptest.NewRequest(http.MethodPut, "/api/v1/environments/1/capabilities/database", managedBody)
	managedReq.Header.Set("Content-Type", "application/json")
	managedRec := httptest.NewRecorder()
	router.ServeHTTP(managedRec, managedReq)
	if managedRec.Code != http.StatusOK {
		t.Fatalf("managed capability status = %d, body=%s", managedRec.Code, managedRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/capabilities", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}

	var response struct {
		Data []model.EnvironmentCapability `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(response.Data) != 3 {
		t.Fatalf("capabilities = %#v, want three rows", response.Data)
	}
	byCapability := map[string]model.EnvironmentCapability{}
	for _, capability := range response.Data {
		byCapability[capability.Capability] = capability
	}
	if byCapability["registry"].Source != model.CapabilitySourceShared || byCapability["registry"].RefServiceID == nil || *byCapability["registry"].RefServiceID != sharedHarbor.ID {
		t.Fatalf("registry capability = %#v, want shared harbor ref", byCapability["registry"])
	}
	if byCapability["git"].Source != model.CapabilitySourceExternal || byCapability["git"].ExternalEndpoint != "https://gitlab.example.com" {
		t.Fatalf("git capability = %#v, want external gitlab endpoint", byCapability["git"])
	}
	if byCapability["database"].Source != model.CapabilitySourceManaged || byCapability["database"].ServiceType != "postgresql" {
		t.Fatalf("database capability = %#v, want managed postgresql", byCapability["database"])
	}
}

func TestUpsertEnvironmentCapabilityAllowsMultipleSharedReferencesForSameCapability(t *testing.T) {
	db := setupCapabilityTestDB(t)

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	_, sharedEnv := createCapabilityTestSharedEnvironment(t, db, admin.ID)
	firstSharedPostgres := model.ServiceInstallation{
		EnvironmentID: sharedEnv.ID,
		ServiceType:   "postgresql",
		ServiceName:   "shared-postgres-a",
		Status:        "running",
		Namespace:     "default-shared-postgres-a",
	}
	secondSharedDatabase := model.ServiceInstallation{
		EnvironmentID: sharedEnv.ID,
		ServiceType:   "mysql",
		ServiceName:   "shared-mysql",
		Status:        "running",
		Namespace:     "default-shared-mysql",
	}
	if err := db.Create(&firstSharedPostgres).Error; err != nil {
		t.Fatalf("create first shared postgres: %v", err)
	}
	if err := db.Create(&secondSharedDatabase).Error; err != nil {
		t.Fatalf("create second shared database: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/capabilities/:capability", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, UpsertEnvironmentCapability))
	router.GET("/api/v1/environments/:id/capabilities", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, ListEnvironmentCapabilities))

	for _, svc := range []model.ServiceInstallation{firstSharedPostgres, secondSharedDatabase} {
		body := bytes.NewBufferString(fmt.Sprintf(`{"source":"shared","provider":"%s","serviceType":"%s","refServiceId":%d}`, svc.ServiceType, svc.ServiceType, svc.ID))
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/environments/%d/capabilities/database", env.ID), body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("shared capability upsert for service %d status = %d, body=%s", svc.ID, rec.Code, rec.Body.String())
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/environments/%d/capabilities", env.ID), nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}
	var response struct {
		Data []model.EnvironmentCapability `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(response.Data) != 2 {
		t.Fatalf("capability list = %#v, want two database references", response.Data)
	}
	seen := map[uint]bool{}
	keys := map[string]bool{}
	for _, item := range response.Data {
		if item.RefServiceID == nil {
			t.Fatalf("capability missing ref service: %#v", item)
		}
		seen[*item.RefServiceID] = true
		if item.CapabilityKey == "" || keys[item.CapabilityKey] {
			t.Fatalf("capability key not unique: %#v", response.Data)
		}
		keys[item.CapabilityKey] = true
	}
	if !seen[firstSharedPostgres.ID] || !seen[secondSharedDatabase.ID] {
		t.Fatalf("capability list = %#v, want refs to both shared services", response.Data)
	}
}

func TestUpsertEnvironmentCapabilityRestoresSoftDeletedSharedReference(t *testing.T) {
	db := setupCapabilityTestDB(t)

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	_, sharedEnv := createCapabilityTestSharedEnvironment(t, db, admin.ID)
	sharedPostgres := model.ServiceInstallation{
		EnvironmentID: sharedEnv.ID,
		ServiceType:   "postgresql",
		ServiceName:   "shared-postgres",
		Status:        "running",
		Namespace:     "default-shared-postgres",
	}
	if err := db.Create(&sharedPostgres).Error; err != nil {
		t.Fatalf("create shared postgres: %v", err)
	}
	softDeleted := model.EnvironmentCapability{
		EnvironmentID:     env.ID,
		Capability:        "database",
		Source:            model.CapabilitySourceShared,
		Provider:          "postgresql",
		ServiceType:       "postgresql",
		RefServiceID:      &sharedPostgres.ID,
		ValidationStatus:  "linked",
		ValidationMessage: "old hidden row",
		CreatedBy:         admin.ID,
	}
	if err := db.Create(&softDeleted).Error; err != nil {
		t.Fatalf("create capability: %v", err)
	}
	if err := db.Delete(&softDeleted).Error; err != nil {
		t.Fatalf("soft delete capability: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/capabilities/:capability", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, UpsertEnvironmentCapability))
	router.GET("/api/v1/environments/:id/capabilities", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, ListEnvironmentCapabilities))

	body := bytes.NewBufferString(fmt.Sprintf(`{"source":"shared","provider":"postgresql","serviceType":"postgresql","refServiceId":%d}`, sharedPostgres.ID))
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/environments/%d/capabilities/database", env.ID), body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore shared capability status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var allRows []model.EnvironmentCapability
	if err := db.Unscoped().Where("environment_id = ? AND capability = ?", env.ID, "database").Find(&allRows).Error; err != nil {
		t.Fatalf("load all capability rows: %v", err)
	}
	if len(allRows) != 1 || allRows[0].DeletedAt.Valid {
		t.Fatalf("allRows = %#v, want one restored active row", allRows)
	}
	listReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/environments/%d/capabilities", env.ID), nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}
	var response struct {
		Data []model.EnvironmentCapability `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].ID != softDeleted.ID {
		t.Fatalf("capability list = %#v, want restored capability id %d", response.Data, softDeleted.ID)
	}
}

func TestGetEnvironmentCapabilityCredentialsReadsSharedRefServiceSecrets(t *testing.T) {
	db := setupCapabilityTestDB(t)
	previousK8sClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousK8sClient) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-postgresql", Namespace: "default-shared-postgresql"},
		Data: map[string][]byte{
			"postgres-password": []byte("shared-secret"),
		},
	}).Build())

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	_, sharedEnv := createCapabilityTestSharedEnvironment(t, db, admin.ID)
	sharedSvc := model.ServiceInstallation{
		EnvironmentID: sharedEnv.ID,
		ServiceType:   "postgresql",
		ServiceName:   "shared-postgresql",
		Status:        "running",
		Namespace:     "default-shared-postgresql",
	}
	if err := db.Create(&sharedSvc).Error; err != nil {
		t.Fatalf("create shared service: %v", err)
	}
	if err := db.Create(&model.EnvironmentCapability{
		EnvironmentID:    env.ID,
		Capability:       "database",
		Source:           model.CapabilitySourceShared,
		Provider:         "postgresql",
		ServiceType:      "postgresql",
		RefServiceID:     &sharedSvc.ID,
		ValidationStatus: "linked",
		CreatedBy:        admin.ID,
	}).Error; err != nil {
		t.Fatalf("create capability: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/environments/:id/capabilities/:capability/credentials", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, GetEnvironmentCapabilityCredentials))
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/environments/%d/capabilities/database/credentials", env.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var response struct {
		Data struct {
			Credentials []ServiceCredential `json:"credentials"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data.Credentials) != 1 || response.Data.Credentials[0].Key != "postgres-password" || response.Data.Credentials[0].Value != "shared-secret" {
		t.Fatalf("unexpected credentials: %#v", response.Data.Credentials)
	}
}

func TestDeleteEnvironmentCapabilityRemovesCapabilityCard(t *testing.T) {
	db := setupCapabilityTestDB(t)

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	row := model.EnvironmentCapability{
		EnvironmentID:         env.ID,
		Capability:            "git",
		Source:                model.CapabilitySourceExternal,
		Provider:              "gitlab",
		ServiceType:           "git",
		ExternalEndpoint:      "https://gitlab.example.com",
		ValidationStatus:      "pending",
		ValidationMessage:     "external connection has not been validated",
		CredentialSecretRef:   "billing-prod/paap-external-git-credentials",
		TLSInsecureSkipVerify: false,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create capability: %v", err)
	}
	capabilityKey := fmt.Sprintf("capability:%d", row.ID)
	componentConfig, err := (model.ComponentConfig{Bindings: []model.ComponentBinding{
		{TargetKey: capabilityKey, TargetKind: "capability", TargetName: row.CapabilityKey, TargetType: row.ServiceType},
		{TargetKey: "service:99", TargetKind: "service", TargetName: "keep"},
	}}).JSON()
	if err != nil {
		t.Fatalf("build component config: %v", err)
	}
	component := model.Component{EnvironmentID: env.ID, Name: "api", Type: "backend", Config: componentConfig}
	if err := db.Create(&component).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	canvas := model.EnvironmentCanvasState{
		EnvironmentID: env.ID,
		Positions:     fmt.Sprintf(`{"%s":{"x":12,"y":24},"environment:%s":{"x":20,"y":30},"component:%d":{"x":1,"y":2}}`, capabilityKey, capabilityKey, component.ID),
		Edges:         fmt.Sprintf(`[{"fromKey":"component:%d","toKey":"%s"},{"fromKey":"component:%d","toKey":"service:99"}]`, component.ID, capabilityKey, component.ID),
		Names:         fmt.Sprintf(`{"%s":"外部 Git","environment:%s":"环境视图 Git","component:%d":"API"}`, capabilityKey, capabilityKey, component.ID),
	}
	if err := db.Create(&canvas).Error; err != nil {
		t.Fatalf("create canvas state: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/environments/:id/capabilities/:capability", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, DeleteEnvironmentCapability))

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/environments/1/capabilities/git", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", rec.Code, rec.Body.String())
	}

	var count int64
	if err := db.Model(&model.EnvironmentCapability{}).Where("environment_id = ? AND capability = ?", env.ID, "git").Count(&count).Error; err != nil {
		t.Fatalf("count capability: %v", err)
	}
	if count != 0 {
		t.Fatalf("capability count = %d, want deleted", count)
	}

	var savedCanvas model.EnvironmentCanvasState
	if err := db.First(&savedCanvas, "environment_id = ?", env.ID).Error; err != nil {
		t.Fatalf("load canvas state: %v", err)
	}
	if strings.Contains(savedCanvas.Positions, capabilityKey) || strings.Contains(savedCanvas.Edges, capabilityKey) || strings.Contains(savedCanvas.Names, capabilityKey) {
		t.Fatalf("canvas state still references deleted capability: positions=%s edges=%s names=%s", savedCanvas.Positions, savedCanvas.Edges, savedCanvas.Names)
	}
	if !strings.Contains(savedCanvas.Positions, fmt.Sprintf("component:%d", component.ID)) || !strings.Contains(savedCanvas.Edges, "service:99") {
		t.Fatalf("unrelated canvas state was removed: positions=%s edges=%s", savedCanvas.Positions, savedCanvas.Edges)
	}

	var savedComponent model.Component
	if err := db.First(&savedComponent, component.ID).Error; err != nil {
		t.Fatalf("load component: %v", err)
	}
	savedConfig, err := model.ParseComponentConfig(savedComponent.Config)
	if err != nil {
		t.Fatalf("parse saved component config: %v", err)
	}
	if len(savedConfig.Bindings) != 1 || savedConfig.Bindings[0].TargetKey != "service:99" {
		t.Fatalf("component bindings were not cleaned: %#v", savedConfig.Bindings)
	}
}

func TestExternalEnvironmentCapabilityStoresCredentialsInKubernetesSecret(t *testing.T) {
	db := setupCapabilityTestDB(t)
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "billing-prod"},
	}).Build())

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/capabilities/:capability", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, UpsertEnvironmentCapability))
	router.GET("/api/v1/environments/:id/capabilities/:capability/credentials", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, GetEnvironmentCapabilityCredentials))

	body := bytes.NewBufferString(`{"source":"external","provider":"gitlab","serviceType":"git","externalEndpoint":"https://gitlab.example.com","authType":"basic","username":"paap","password":"secret-pass"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/1/capabilities/git", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("external capability status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret-pass") {
		t.Fatalf("response leaked plaintext password: %s", rec.Body.String())
	}

	var response struct {
		Data model.EnvironmentCapability `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.CredentialSecretRef == "" {
		t.Fatalf("credentialSecretRef is empty, response=%#v", response.Data)
	}

	var capability model.EnvironmentCapability
	if err := db.Where("environment_id = ? AND capability = ?", env.ID, "git").First(&capability).Error; err != nil {
		t.Fatalf("load capability: %v", err)
	}
	if capability.CredentialSecretRef != response.Data.CredentialSecretRef {
		t.Fatalf("stored credential ref = %q, response = %q", capability.CredentialSecretRef, response.Data.CredentialSecretRef)
	}

	parts := strings.SplitN(capability.CredentialSecretRef, "/", 2)
	if len(parts) != 2 {
		t.Fatalf("credentialSecretRef = %q, want namespace/name", capability.CredentialSecretRef)
	}
	secret := &corev1.Secret{}
	if err := k8s.GetClient().Get(t.Context(), client.ObjectKey{Namespace: parts[0], Name: parts[1]}, secret); err != nil {
		t.Fatalf("load generated secret: %v", err)
	}
	if string(secret.Data["username"]) != "paap" || string(secret.Data["password"]) != "secret-pass" {
		t.Fatalf("generated secret data = %#v", secret.Data)
	}
	if string(secret.Data["endpoint"]) != "https://gitlab.example.com" || string(secret.Data["authType"]) != "basic" {
		t.Fatalf("generated secret metadata data = %#v", secret.Data)
	}

	credentialReq := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/capabilities/git/credentials", nil)
	credentialRec := httptest.NewRecorder()
	router.ServeHTTP(credentialRec, credentialReq)
	if credentialRec.Code != http.StatusOK {
		t.Fatalf("credential status = %d, body=%s", credentialRec.Code, credentialRec.Body.String())
	}
	var credentialResponse struct {
		Data struct {
			Credentials []ServiceCredential `json:"credentials"`
		} `json:"data"`
	}
	if err := json.Unmarshal(credentialRec.Body.Bytes(), &credentialResponse); err != nil {
		t.Fatalf("decode credentials: %v", err)
	}
	credentials := map[string]string{}
	for _, credential := range credentialResponse.Data.Credentials {
		credentials[credential.Key] = credential.Value
	}
	if credentials["endpoint"] != "https://gitlab.example.com" || credentials["username"] != "paap" || credentials["password"] != "secret-pass" {
		t.Fatalf("credentials = %#v", credentials)
	}
}

func TestValidateExternalEnvironmentCapabilityStoresValidStatus(t *testing.T) {
	db := setupCapabilityTestDB(t)
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "billing-prod"}},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "external-db-credentials", Namespace: "billing-prod"},
			Data:       map[string][]byte{"username": []byte("paap"), "password": []byte("secret")},
		},
	).Build())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	capability := model.EnvironmentCapability{
		EnvironmentID:         env.ID,
		Capability:            "database",
		CapabilityKey:         "database-external-test",
		Source:                model.CapabilitySourceExternal,
		Provider:              "postgresql",
		ServiceType:           "postgresql",
		ExternalEndpoint:      listener.Addr().String(),
		CredentialSecretRef:   "billing-prod/external-db-credentials",
		ValidationStatus:      "pending",
		ValidationMessage:     "not validated",
		TLSInsecureSkipVerify: false,
	}
	if err := db.Create(&capability).Error; err != nil {
		t.Fatalf("create capability: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/capabilities/:capability/validate", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, ValidateEnvironmentCapability))

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/environments/%d/capabilities/database-external-test/validate", env.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Data model.EnvironmentCapability `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ValidationStatus != "valid" {
		t.Fatalf("response validation status = %q, body=%s", response.Data.ValidationStatus, rec.Body.String())
	}
	var saved model.EnvironmentCapability
	if err := db.First(&saved, capability.ID).Error; err != nil {
		t.Fatalf("load capability: %v", err)
	}
	if saved.ValidationStatus != "valid" || !strings.Contains(saved.ValidationMessage, "reachable") {
		t.Fatalf("saved validation = %q/%q", saved.ValidationStatus, saved.ValidationMessage)
	}
}

func TestValidateExternalHTTPCapabilityChecksCredentials(t *testing.T) {
	db := setupCapabilityTestDB(t)
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "billing-prod"}},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "external-git-credentials", Namespace: "billing-prod"},
			Data:       map[string][]byte{"username": []byte("paap"), "password": []byte("secret")},
		},
	).Build())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/version" {
			http.NotFound(w, r)
			return
		}
		username, password, ok := r.BasicAuth()
		if !ok || username != "paap" || password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	t.Cleanup(server.Close)

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	capability := model.EnvironmentCapability{
		EnvironmentID:       env.ID,
		Capability:          "git",
		CapabilityKey:       "external-gitea",
		Source:              model.CapabilitySourceExternal,
		Provider:            "gitea",
		ServiceType:         "git",
		ExternalEndpoint:    server.URL,
		CredentialSecretRef: "billing-prod/external-git-credentials",
		ValidationStatus:    "pending",
	}
	if err := db.Create(&capability).Error; err != nil {
		t.Fatalf("create capability: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/capabilities/:capability/validate", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, ValidateEnvironmentCapability))

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/environments/%d/capabilities/external-gitea/validate", env.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var saved model.EnvironmentCapability
	if err := db.First(&saved, capability.ID).Error; err != nil {
		t.Fatalf("load capability: %v", err)
	}
	if saved.ValidationStatus != "valid" || !strings.Contains(saved.ValidationMessage, "HTTP credentials") {
		t.Fatalf("saved validation = %q/%q", saved.ValidationStatus, saved.ValidationMessage)
	}
}

func TestValidateExternalEnvironmentCapabilityStoresFailedStatus(t *testing.T) {
	db := setupCapabilityTestDB(t)
	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	capability := model.EnvironmentCapability{
		EnvironmentID:     env.ID,
		Capability:        "registry",
		CapabilityKey:     "registry-external-test",
		Source:            model.CapabilitySourceExternal,
		Provider:          "harbor",
		ServiceType:       "registry",
		ValidationStatus:  "pending",
		ValidationMessage: "not validated",
	}
	if err := db.Create(&capability).Error; err != nil {
		t.Fatalf("create capability: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/capabilities/:capability/validate", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, ValidateEnvironmentCapability))

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/environments/%d/capabilities/registry-external-test/validate", env.ID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var saved model.EnvironmentCapability
	if err := db.First(&saved, capability.ID).Error; err != nil {
		t.Fatalf("load capability: %v", err)
	}
	if saved.ValidationStatus != "failed" || !strings.Contains(saved.ValidationMessage, "endpoint") {
		t.Fatalf("saved validation = %q/%q", saved.ValidationStatus, saved.ValidationMessage)
	}
}

func TestExternalEnvironmentCapabilityAllowsMultipleConnectionsForSameCapability(t *testing.T) {
	db := setupCapabilityTestDB(t)
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "billing-prod"},
	}).Build())

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/capabilities/:capability", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, UpsertEnvironmentCapability))
	router.GET("/api/v1/environments/:id/capabilities/:capability/credentials", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, GetEnvironmentCapabilityCredentials))

	cases := []struct {
		key      string
		endpoint string
		password string
	}{
		{key: "prod-postgres", endpoint: "postgres.prod.example:5432", password: "prod-secret"},
		{key: "report-postgres", endpoint: "postgres.report.example:5432", password: "report-secret"},
	}
	for _, tc := range cases {
		body := bytes.NewBufferString(fmt.Sprintf(`{"source":"external","capabilityKey":%q,"provider":"postgresql","serviceType":"postgresql","externalEndpoint":%q,"authType":"basic","username":"paap","password":%q}`, tc.key, tc.endpoint, tc.password))
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/environments/%d/capabilities/database", env.ID), body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("external capability %s status = %d, body=%s", tc.key, rec.Code, rec.Body.String())
		}
	}

	var rows []model.EnvironmentCapability
	if err := db.Where("environment_id = ? AND capability = ?", env.ID, "database").Order("capability_key").Find(&rows).Error; err != nil {
		t.Fatalf("load capabilities: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want two external database connections", rows)
	}
	secretRefs := map[string]string{}
	for _, row := range rows {
		secretRefs[row.CapabilityKey] = row.CredentialSecretRef
	}
	if secretRefs["prod-postgres"] == "" || secretRefs["report-postgres"] == "" || secretRefs["prod-postgres"] == secretRefs["report-postgres"] {
		t.Fatalf("secret refs were not unique: %#v", secretRefs)
	}

	credentialReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/environments/%d/capabilities/report-postgres/credentials", env.ID), nil)
	credentialRec := httptest.NewRecorder()
	router.ServeHTTP(credentialRec, credentialReq)
	if credentialRec.Code != http.StatusOK {
		t.Fatalf("credential status = %d, body=%s", credentialRec.Code, credentialRec.Body.String())
	}
	if !strings.Contains(credentialRec.Body.String(), "report-secret") || strings.Contains(credentialRec.Body.String(), "prod-secret") {
		t.Fatalf("credential response did not select the requested capability key: %s", credentialRec.Body.String())
	}
}

func TestEnvironmentCapabilityRejectsSharedServiceOutsideSystemPool(t *testing.T) {
	db := setupCapabilityTestDB(t)

	admin := createCapabilityTestPlatformAdmin(t, db)
	app := model.Application{Name: "业务应用", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: app.ID, UserID: admin.ID, Role: "admin"}).Error; err != nil {
		t.Fatalf("create app member: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	nonShared := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "harbor", ServiceName: "local-harbor", Status: "running"}
	if err := db.Create(&nonShared).Error; err != nil {
		t.Fatalf("create local harbor: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/capabilities/:capability", withTestAuthUserRole(admin.ID, model.RoleAppAdmin, UpsertEnvironmentCapability))

	body := bytes.NewBufferString(fmt.Sprintf(`{"source":"shared","provider":"harbor","serviceType":"harbor","refServiceId":%d}`, nonShared.ID))
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/1/capabilities/registry", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "shared service must belong to the system shared environment") {
		t.Fatalf("body = %s, want shared service validation message", rec.Body.String())
	}
}

func TestNormalizeCapabilityAcceptsCustomExternalResources(t *testing.T) {
	capability, ok := service.NormalizeCapability("custom")
	if !ok || capability != "custom" {
		t.Fatalf("normalize custom capability = %q, %v; want custom, true", capability, ok)
	}
}
