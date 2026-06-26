package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/model"

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
		&model.UserRole{},
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.EnvironmentCapability{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	return db
}

func createCapabilityTestPlatformAdmin(t *testing.T, db *gorm.DB) model.User {
	t.Helper()
	admin := model.User{Username: "admin", Email: "admin@example.test", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := model.ReplaceUserRoles(db, admin.ID, []string{model.RolePlatformAdmin, model.RoleAppAdmin}); err != nil {
		t.Fatalf("create admin roles: %v", err)
	}
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

func TestUpsertEnvironmentCapabilityUpdatesExistingSharedReference(t *testing.T) {
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

	var count int64
	if err := db.Model(&model.EnvironmentCapability{}).Where("environment_id = ? AND capability = ?", env.ID, "database").Count(&count).Error; err != nil {
		t.Fatalf("count capability: %v", err)
	}
	if count != 1 {
		t.Fatalf("capability count = %d, want one updated row", count)
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
	if len(response.Data) != 1 || response.Data[0].RefServiceID == nil || *response.Data[0].RefServiceID != secondSharedDatabase.ID {
		t.Fatalf("capability list = %#v, want active row linked to second database", response.Data)
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
	capability, ok := normalizeCapability("custom")
	if !ok || capability != "custom" {
		t.Fatalf("normalize custom capability = %q, %v; want custom, true", capability, ok)
	}
}
