package service

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	paapv1 "paap/api/v1"
	"paap/internal/model"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestClusterSyncOwnerIDUsesUserRolesWhenAdminUsernameMissing(t *testing.T) {
	db, mock := openMockClusterSyncDB(t)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM information_schema.tables WHERE table_schema = CURRENT_SCHEMA() AND table_name = $1 AND table_type = $2`)).
		WithArgs("users", "BASE TABLE").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "users" WHERE username = $1 AND "users"."deleted_at" IS NULL ORDER BY "users"."id" LIMIT $2`)).
		WithArgs("admin", 1).
		WillReturnRows(userSQLRows())
	mock.ExpectQuery(`SELECT .* FROM "users" JOIN user_roles .*user_roles\.role = \$1.*`).
		WithArgs(model.RolePlatformAdmin, 1).
		WillReturnRows(userSQLRows().AddRow(42, time.Now(), time.Now(), nil, "platform-owner", "owner@example.local", "x"))

	ownerID, err := clusterSyncOwnerID(db)
	if err != nil {
		t.Fatalf("cluster sync owner: %v", err)
	}
	if ownerID != 42 {
		t.Fatalf("ownerID = %d, want 42", ownerID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func openMockClusterSyncDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open sql mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open gorm sql mock: %v", err)
	}
	return db, mock
}

func userSQLRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "created_at", "updated_at", "deleted_at", "username", "email", "password"})
}

func TestClusterSyncObjectIsDeleting(t *testing.T) {
	deletingAt := metav1.Now()
	if !clusterSyncObjectIsDeleting(&paapv1.Environment{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletingAt}}) {
		t.Fatalf("object with deletionTimestamp should be treated as deleting")
	}
	if clusterSyncObjectIsDeleting(&paapv1.Environment{}) {
		t.Fatalf("object without deletionTimestamp should not be treated as deleting")
	}
}

func TestSyncClusterStateRestoresDBFromExistingCRs(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	if err := db.Create(&model.User{Username: "developer", Email: "developer@example.local", Password: "x"}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	admin := model.User{Username: "admin", Email: "admin@example.local", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec: paapv1.ApplicationSpec{
					Name:        "测试服务",
					Identifier:  "test",
					Description: "from cr",
				},
				Status: paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "staging",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "预发",
					Identifier:       "staging",
					PrimaryNamespace: "test-staging",
				},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
			&paapv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-argocd",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app":          "test",
						"paap.io/env":          "staging",
						"paap.io/service-type": "deploy",
						"paap.io/tool":         "argocd",
					},
				},
				Spec: paapv1.ServiceInstanceSpec{
					EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
					Type:           "deploy",
					ToolNamespace:  "test-staging-argocd",
					Helm: &paapv1.HelmInstallSpec{
						ReleaseName: "test-staging-argocd",
						Namespace:   "test-staging-argocd",
					},
				},
				Status: paapv1.ServiceInstanceStatus{
					Phase: "Running",
					Conditions: []metav1.Condition{{
						Type:    "Ready",
						Status:  metav1.ConditionFalse,
						Reason:  "ImagePullBackOff",
						Message: "Back-off pulling image",
					}},
				},
			},
			&paapv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-order-api",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app":       "test",
						"paap.io/env":       "staging",
						"paap.io/component": "order-api",
					},
				},
				Spec: paapv1.ComponentSpec{
					EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
					Name:           "订单服务",
					Identifier:     "order-api",
					Type:           "backend",
					ManagedBy:      "argocd",
					ArgoCDAppRef:   &paapv1.ObjectReference{Name: "test-staging-order-api"},
					Deployment: paapv1.DeploymentSpec{
						Namespace: "test-staging",
						Image:     "registry:2.8.3",
						Tag:       "2.8.3",
						Replicas:  1,
					},
				},
				Status: paapv1.ComponentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var app model.Application
	if err := db.Where("identifier = ?", "test").First(&app).Error; err != nil {
		t.Fatalf("application not restored: %v", err)
	}
	if app.Name != "测试服务" || app.Description != "from cr" || app.OwnerID != admin.ID {
		t.Fatalf("unexpected application: %#v", app)
	}

	var member model.AppMember
	if err := db.Where("application_id = ? AND user_id = ?", app.ID, admin.ID).First(&member).Error; err != nil {
		t.Fatalf("owner member not restored: %v", err)
	}

	var env model.Environment
	if err := db.Where("application_id = ? AND identifier = ?", app.ID, "staging").First(&env).Error; err != nil {
		t.Fatalf("environment not restored: %v", err)
	}
	if env.Name != "预发" || env.Namespace != "test-staging" || env.Status != "running" {
		t.Fatalf("unexpected environment: %#v", env)
	}

	var install model.ServiceInstallation
	if err := db.Where("environment_id = ? AND service_type = ?", env.ID, "deploy").First(&install).Error; err != nil {
		t.Fatalf("service installation not restored: %v", err)
	}
	if install.ServiceName != "staging-argocd" || install.Namespace != "test-staging-argocd" || install.ReleaseName != "test-staging-argocd" || install.Status != "running" {
		t.Fatalf("unexpected service installation: %#v", install)
	}
	if install.ErrorMessage != "Back-off pulling image" {
		t.Fatalf("expected synced error message, got %q", install.ErrorMessage)
	}

	var comp model.Component
	if err := db.Where("environment_id = ? AND name = ?", env.ID, "订单服务").First(&comp).Error; err != nil {
		t.Fatalf("component not restored: %v", err)
	}
	if comp.Type != "backend" || comp.Image != "registry:2.8.3" || comp.Version != "2.8.3" || comp.Replicas != 1 || comp.Status != "running" {
		t.Fatalf("unexpected component: %#v", comp)
	}
	if comp.ArgoCDApp != "test-staging-order-api" {
		t.Fatalf("expected argocd app synced, got %#v", comp)
	}
}

func TestSyncClusterStateMarksSharedPoolCRsAsSystem(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	admin := model.User{Username: "admin", Email: "admin@example.local", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "paap-system"},
				Spec: paapv1.ApplicationSpec{
					Name:        "共享资源池",
					Identifier:  "default",
					Description: "PAAP platform shared resource pool",
				},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shared",
					Namespace: "paap-app-default",
					Labels: map[string]string{
						"paap.io/app": "default",
						"paap.io/env": "shared",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "共享环境",
					Identifier:       "shared",
					PrimaryNamespace: "default-shared",
				},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var app model.Application
	if err := db.Where("identifier = ?", "default").First(&app).Error; err != nil {
		t.Fatalf("load shared app: %v", err)
	}
	if !app.IsSystem || app.Name != "共享资源池" {
		t.Fatalf("shared app = %#v, want system shared app from CR", app)
	}
	var env model.Environment
	if err := db.Where("application_id = ? AND identifier = ?", app.ID, "shared").First(&env).Error; err != nil {
		t.Fatalf("load shared env: %v", err)
	}
	if !env.IsSystem || env.Name != "共享环境" || env.Namespace != "default-shared" {
		t.Fatalf("shared env = %#v, want system shared environment from CR", env)
	}
}

func TestSyncClusterStateIgnoresLegacyServiceTypeNamedServiceInstances(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec:       paapv1.ApplicationSpec{Name: "测试服务", Identifier: "test"},
				Status:     paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels:    map[string]string{"paap.io/app": "test", "paap.io/env": "staging"},
				},
				Spec:   paapv1.EnvironmentSpec{Name: "预发", Identifier: "staging", PrimaryNamespace: "test-staging"},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
			&paapv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-deploy",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app":          "test",
						"paap.io/env":          "staging",
						"paap.io/service-type": "deploy",
						"paap.io/tool":         "argocd",
					},
				},
				Spec: paapv1.ServiceInstanceSpec{
					EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
					Type:           "deploy",
					ToolNamespace:  "test-staging-argocd",
					Helm: &paapv1.HelmInstallSpec{
						ReleaseName: "test-staging-argocd",
						Namespace:   "test-staging-argocd",
					},
				},
				Status: paapv1.ServiceInstanceStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var count int64
	if err := db.Model(&model.ServiceInstallation{}).Count(&count).Error; err != nil {
		t.Fatalf("count service installations: %v", err)
	}
	if count != 0 {
		t.Fatalf("legacy service-type named ServiceInstance should be ignored, got %d installs", count)
	}
}

func TestSyncClusterStateKeepsEnvironmentEmptyWithoutServicesOrComponents(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	app := model.Application{Name: "测试服务", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{
		ApplicationID: app.ID,
		Name:          "开发环境",
		Identifier:    "dev",
		TemplateID:    0,
		Status:        "empty",
		Namespace:     "test-dev",
	}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec: paapv1.ApplicationSpec{
					Name:       "测试服务",
					Identifier: "test",
				},
				Status: paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dev",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "dev",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "开发环境",
					Identifier:       "dev",
					PrimaryNamespace: "test-dev",
				},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var synced model.Environment
	if err := db.Where("application_id = ? AND identifier = ?", app.ID, "dev").First(&synced).Error; err != nil {
		t.Fatalf("environment not synced: %v", err)
	}
	if synced.Status != "empty" {
		t.Fatalf("empty environment status = %q, want empty", synced.Status)
	}
	if synced.TemplateID != 0 {
		t.Fatalf("empty environment template id = %d, want 0", synced.TemplateID)
	}
}

func TestSyncClusterStateRestoresEmptyStatusAfterRunningDrift(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	app := model.Application{Name: "测试服务", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{
		ApplicationID: app.ID,
		Name:          "开发环境",
		Identifier:    "dev",
		TemplateID:    0,
		Status:        "running",
		Namespace:     "test-dev",
	}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec: paapv1.ApplicationSpec{
					Name:       "测试服务",
					Identifier: "test",
				},
				Status: paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dev",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "dev",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "开发环境",
					Identifier:       "dev",
					PrimaryNamespace: "test-dev",
				},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var synced model.Environment
	if err := db.Where("application_id = ? AND identifier = ?", app.ID, "dev").First(&synced).Error; err != nil {
		t.Fatalf("environment not synced: %v", err)
	}
	if synced.Status != "empty" {
		t.Fatalf("empty environment status = %q, want empty", synced.Status)
	}
	if synced.TemplateID != 0 {
		t.Fatalf("empty environment template id = %d, want 0", synced.TemplateID)
	}
}

func TestSyncClusterStateUpdatesComponentRegistryImageFromCR(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}
	app := model.Application{Name: "测试服务", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "预发", Identifier: "staging", Namespace: "test-staging", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.Component{
		EnvironmentID:  env.ID,
		Name:           "source-smoke",
		Type:           "backend",
		Image:          "registry.paap.local:5000/test-staging/source-smoke:14",
		RegistryImage:  "registry.paap.local:5000/test-staging/source-smoke:14",
		Version:        "14",
		Replicas:       1,
		Status:         "running",
		DeliveryMode:   "source",
		ArgoCDApp:      "test-staging-source-smoke",
		JenkinsJob:     "test-staging-source-smoke-build",
		PipelineStatus: "configured",
	}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec:       paapv1.ApplicationSpec{Name: "测试服务", Identifier: "test"},
				Status:     paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels:    map[string]string{"paap.io/app": "test", "paap.io/env": "staging"},
				},
				Spec:   paapv1.EnvironmentSpec{Name: "预发", Identifier: "staging", PrimaryNamespace: "test-staging"},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
			&paapv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-source-smoke",
					Namespace: "paap-app-test",
					Labels:    map[string]string{"paap.io/app": "test", "paap.io/env": "staging", "paap.io/component": "source-smoke"},
				},
				Spec: paapv1.ComponentSpec{
					EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
					Name:           "source-smoke",
					Identifier:     "source-smoke",
					Type:           "backend",
					ManagedBy:      "argocd",
					ArgoCDAppRef:   &paapv1.ObjectReference{Name: "test-staging-source-smoke"},
					Deployment: paapv1.DeploymentSpec{
						Namespace: "test-staging",
						Image:     "registry.test-staging.paap.local:5000/test-staging/source-smoke:14",
						Tag:       "14",
						Replicas:  1,
					},
				},
				Status: paapv1.ComponentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var updated model.Component
	if err := db.Where("environment_id = ? AND name = ?", env.ID, "source-smoke").First(&updated).Error; err != nil {
		t.Fatalf("component not found: %v", err)
	}
	wantImage := "registry.test-staging.paap.local:5000/test-staging/source-smoke:14"
	if updated.Image != wantImage {
		t.Fatalf("image = %q, want %q", updated.Image, wantImage)
	}
	if updated.RegistryImage != wantImage {
		t.Fatalf("registry image = %q, want %q", updated.RegistryImage, wantImage)
	}
	if updated.DeliveryMode != "source" || updated.JenkinsJob != "test-staging-source-smoke-build" || updated.PipelineStatus != "configured" {
		t.Fatalf("existing source delivery metadata was not preserved: %#v", updated)
	}
}

func TestSyncClusterStatePreservesHighLevelComponentConfig(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}
	app := model.Application{Name: "test", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "预发", Identifier: "staging", Namespace: "test-staging", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	cfgJSON, err := (model.ComponentConfig{
		Framework: "springboot",
		Env: []model.ComponentEnvVar{
			{Name: "OLD", Value: "stale"},
		},
		ConfigMaps: []model.ComponentConfigMap{{
			Name: "api-config",
			Data: map[string]string{"application-paap.yml": "spring: {}\n"},
		}},
		Secrets: []model.ComponentSecret{{
			Name: "api-secret",
			Data: map[string]string{"POSTGRES_PASSWORD": "secret"},
		}},
		Files: []model.ComponentConfigFile{{
			Name:          "spring-config",
			ConfigMapName: "api-config",
			Key:           "application-paap.yml",
			MountPath:     "/etc/paap/application-paap.yml",
		}},
		Bindings: []model.ComponentBinding{{
			TargetKey:  "service:1",
			TargetName: "postgresql",
			TargetType: "postgresql",
			Role:       "database",
			Mode:       "springboot-file",
		}},
		Dependencies: []string{"postgresql"},
	}).JSON()
	if err != nil {
		t.Fatalf("config json: %v", err)
	}
	if err := db.Create(&model.Component{
		EnvironmentID: env.ID,
		Name:          "api",
		Type:          "backend",
		Image:         "registry/api:v1",
		Version:       "v1",
		Replicas:      1,
		Status:        "running",
		Config:        cfgJSON,
	}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec:       paapv1.ApplicationSpec{Name: "测试服务", Identifier: "test"},
				Status:     paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels:    map[string]string{"paap.io/app": "test", "paap.io/env": "staging"},
				},
				Spec:   paapv1.EnvironmentSpec{Name: "预发", Identifier: "staging", PrimaryNamespace: "test-staging"},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
			&paapv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-api",
					Namespace: "paap-app-test",
					Labels:    map[string]string{"paap.io/app": "test", "paap.io/env": "staging", "paap.io/component": "api"},
				},
				Spec: paapv1.ComponentSpec{
					EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
					Name:           "api",
					Identifier:     "api",
					Type:           "backend",
					ManagedBy:      "argocd",
					Deployment: paapv1.DeploymentSpec{
						Namespace: "test-staging",
						Image:     "registry/api:v2",
						Tag:       "v2",
						Replicas:  1,
						Env: []paapv1.EnvVar{{
							Name:  "RUNTIME",
							Value: "fresh",
						}},
					},
				},
				Status: paapv1.ComponentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var updated model.Component
	if err := db.Where("environment_id = ? AND name = ?", env.ID, "api").First(&updated).Error; err != nil {
		t.Fatalf("component not found: %v", err)
	}
	if updated.Image != "registry/api:v1" || updated.Version != "v1" {
		t.Fatalf("sync must not overwrite image delivery edits from runtime CR, got image=%q version=%q", updated.Image, updated.Version)
	}
	merged, err := model.ParseComponentConfig(updated.Config)
	if err != nil {
		t.Fatalf("parse merged config: %v", err)
	}
	if merged.Framework != "springboot" || len(merged.ConfigMaps) != 1 || len(merged.Secrets) != 1 || len(merged.Files) != 1 || len(merged.Bindings) != 1 {
		t.Fatalf("high-level config was not preserved: %#v", merged)
	}
	if len(merged.Env) != 1 || merged.Env[0].Name != "OLD" || merged.Env[0].Value != "stale" {
		t.Fatalf("platform env config should not be overwritten by stale runtime env: %#v", merged.Env)
	}

	adopted := mergeComponentRuntimeConfig("", `{"env":[{"name":"RUNTIME","value":"fresh"}]}`)
	adoptedCfg, err := model.ParseComponentConfig(adopted)
	if err != nil {
		t.Fatalf("parse adopted runtime config: %v", err)
	}
	if len(adoptedCfg.Env) != 1 || adoptedCfg.Env[0].Name != "RUNTIME" || adoptedCfg.Env[0].Value != "fresh" {
		t.Fatalf("empty platform config should still adopt runtime env: %#v", adoptedCfg.Env)
	}
}

func TestSyncClusterStatePrunesDBRecordsMissingFromCluster(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	staleApp := model.Application{Name: "stale", Identifier: "stale", OwnerID: 1}
	if err := db.Create(&staleApp).Error; err != nil {
		t.Fatalf("create stale app: %v", err)
	}
	staleEnv := model.Environment{
		ApplicationID: staleApp.ID,
		Name:          "stale",
		Identifier:    "stale",
		Status:        "running",
		Namespace:     "stale-stale",
	}
	if err := db.Create(&staleEnv).Error; err != nil {
		t.Fatalf("create stale env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{
		EnvironmentID: staleEnv.ID,
		ServiceType:   "deploy",
		Status:        "running",
		Namespace:     "stale-stale-deploy",
	}).Error; err != nil {
		t.Fatalf("create stale service: %v", err)
	}
	if err := db.Create(&model.Component{
		EnvironmentID: staleEnv.ID,
		Name:          "stale-api",
		Type:          "backend",
		Image:         "registry:2.8.3",
		Version:       "2.8.3",
		Replicas:      1,
		Status:        "running",
	}).Error; err != nil {
		t.Fatalf("create stale component: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: staleApp.ID, UserID: 1, Role: "admin"}).Error; err != nil {
		t.Fatalf("create stale member: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var appCount int64
	if err := db.Model(&model.Application{}).Count(&appCount).Error; err != nil {
		t.Fatalf("count apps: %v", err)
	}
	if appCount != 0 {
		t.Fatalf("expected stale app pruned, got %d apps", appCount)
	}

	var envCount int64
	if err := db.Model(&model.Environment{}).Count(&envCount).Error; err != nil {
		t.Fatalf("count envs: %v", err)
	}
	if envCount != 0 {
		t.Fatalf("expected stale env pruned, got %d envs", envCount)
	}

	var serviceCount int64
	if err := db.Model(&model.ServiceInstallation{}).Count(&serviceCount).Error; err != nil {
		t.Fatalf("count services: %v", err)
	}
	if serviceCount != 0 {
		t.Fatalf("expected stale service pruned, got %d services", serviceCount)
	}

	var componentCount int64
	if err := db.Model(&model.Component{}).Count(&componentCount).Error; err != nil {
		t.Fatalf("count components: %v", err)
	}
	if componentCount != 0 {
		t.Fatalf("expected stale component pruned, got %d components", componentCount)
	}
}

func TestSyncClusterStateIgnoresDeletingEnvironmentResources(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate db: %v", err)
	}
	admin := model.User{Username: "admin", Email: "admin@example.local", Password: "x"}
	if err := db.Create(&admin).Error; err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: admin.ID}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("seed env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "redis", Status: "running", Namespace: "billing-prod-redis"}).Error; err != nil {
		t.Fatalf("seed service: %v", err)
	}
	if err := db.Create(&model.Component{EnvironmentID: env.ID, Name: "api", Type: "backend", Image: "registry/api:v1", Version: "v1", Status: "running"}).Error; err != nil {
		t.Fatalf("seed component: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	deletingAt := metav1.Now()
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "billing", Namespace: "paap-system"},
				Spec:       paapv1.ApplicationSpec{Name: "Billing", Identifier: "billing"},
				Status:     paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "prod",
					Namespace:         "paap-app-billing",
					Labels:            map[string]string{"paap.io/app": "billing", "paap.io/env": "prod"},
					DeletionTimestamp: &deletingAt,
					Finalizers:        []string{"paap.io/environment"},
				},
				Spec:   paapv1.EnvironmentSpec{Name: "生产", Identifier: "prod", PrimaryNamespace: "billing-prod"},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
			&paapv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "prod-redis",
					Namespace:         "paap-app-billing",
					Labels:            map[string]string{"paap.io/app": "billing", "paap.io/env": "prod", "paap.io/tool": "redis"},
					DeletionTimestamp: &deletingAt,
					Finalizers:        []string{"paap.io/service"},
				},
				Spec: paapv1.ServiceInstanceSpec{
					EnvironmentRef: paapv1.ObjectReference{Name: "prod"},
					Type:           "redis",
					ToolNamespace:  "billing-prod-redis",
				},
				Status: paapv1.ServiceInstanceStatus{Phase: "Running"},
			},
			&paapv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "prod-api",
					Namespace:         "paap-app-billing",
					Labels:            map[string]string{"paap.io/app": "billing", "paap.io/env": "prod", "paap.io/component": "api"},
					DeletionTimestamp: &deletingAt,
					Finalizers:        []string{"paap.io/component"},
				},
				Spec: paapv1.ComponentSpec{
					EnvironmentRef: paapv1.ObjectReference{Name: "prod"},
					Name:           "api",
					Identifier:     "api",
					Type:           "backend",
					Deployment:     paapv1.DeploymentSpec{Namespace: "billing-prod", Image: "registry/api:v1", Tag: "v1", Replicas: 1},
				},
				Status: paapv1.ComponentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	for _, table := range []interface{}{&model.Environment{}, &model.ServiceInstallation{}, &model.Component{}} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatalf("count %T: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%T rows after deleting CR sync = %d, want 0", table, count)
		}
	}
}

func TestSyncClusterStatePrunesSystemSharedPoolMissingFromCluster(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	systemApp := model.Application{Name: "共享资源池", Identifier: "default", OwnerID: 1, IsSystem: true}
	if err := db.Create(&systemApp).Error; err != nil {
		t.Fatalf("create system app: %v", err)
	}
	systemEnv := model.Environment{
		ApplicationID: systemApp.ID,
		Name:          "共享环境",
		Identifier:    "shared",
		Status:        "empty",
		Namespace:     "default-shared",
		IsSystem:      true,
	}
	if err := db.Create(&systemEnv).Error; err != nil {
		t.Fatalf("create system env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{
		EnvironmentID: systemEnv.ID,
		ServiceType:   "git",
		Status:        "running",
		Namespace:     "default-shared-git",
	}).Error; err != nil {
		t.Fatalf("create system service: %v", err)
	}
	if err := db.Create(&model.Component{
		EnvironmentID: systemEnv.ID,
		Name:          "system-helper",
		Type:          "backend",
		Image:         "registry.local/helper:v1",
		Version:       "v1",
		Replicas:      1,
		Status:        "running",
	}).Error; err != nil {
		t.Fatalf("create system component: %v", err)
	}
	if err := db.Create(&model.AppMember{ApplicationID: systemApp.ID, UserID: 1, Role: "admin"}).Error; err != nil {
		t.Fatalf("create system member: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var appCount int64
	if err := db.Model(&model.Application{}).Where("identifier = ? AND is_system = ?", "default", true).Count(&appCount).Error; err != nil {
		t.Fatalf("count system app: %v", err)
	}
	if appCount != 0 {
		t.Fatalf("system app count = %d, want CR-backed shared pool pruned when CR is missing", appCount)
	}
	var envCount int64
	if err := db.Model(&model.Environment{}).Where("application_id = ? AND identifier = ? AND is_system = ?", systemApp.ID, "shared", true).Count(&envCount).Error; err != nil {
		t.Fatalf("count system env: %v", err)
	}
	if envCount != 0 {
		t.Fatalf("system env count = %d, want CR-backed shared environment pruned when CR is missing", envCount)
	}
	var serviceCount int64
	if err := db.Model(&model.ServiceInstallation{}).Where("environment_id = ?", systemEnv.ID).Count(&serviceCount).Error; err != nil {
		t.Fatalf("count system services: %v", err)
	}
	if serviceCount != 0 {
		t.Fatalf("system service count = %d, want pruned with missing shared environment CR", serviceCount)
	}
	var componentCount int64
	if err := db.Model(&model.Component{}).Where("environment_id = ?", systemEnv.ID).Count(&componentCount).Error; err != nil {
		t.Fatalf("count system components: %v", err)
	}
	if componentCount != 0 {
		t.Fatalf("system component count = %d, want pruned with missing shared environment CR", componentCount)
	}
}

func TestSyncClusterStatePreservesDraftComponentsMissingFromCluster(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	app := model.Application{Name: "测试服务", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{
		ApplicationID: app.ID,
		Name:          "预发",
		Identifier:    "staging",
		Status:        "running",
		Namespace:     "test-staging",
	}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	draft := model.Component{
		EnvironmentID: env.ID,
		Name:          "frontend-1",
		Type:          "frontend",
		Replicas:      1,
		Status:        "draft",
		DeliveryMode:  "image",
	}
	if err := db.Create(&draft).Error; err != nil {
		t.Fatalf("create draft component: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec: paapv1.ApplicationSpec{
					Name:       "测试服务",
					Identifier: "test",
				},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "staging",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "预发",
					Identifier:       "staging",
					PrimaryNamespace: "test-staging",
				},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var saved model.Component
	if err := db.Where("environment_id = ? AND name = ?", env.ID, "frontend-1").First(&saved).Error; err != nil {
		t.Fatalf("draft component should be preserved: %v", err)
	}
	if saved.Status != "draft" {
		t.Fatalf("status = %q, want draft", saved.Status)
	}
}

func TestSyncClusterStatePreservesServiceCardsMissingFromCluster(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	app := model.Application{Name: "测试服务", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{
		ApplicationID: app.ID,
		Name:          "预发",
		Identifier:    "staging",
		Status:        "running",
		Namespace:     "test-staging",
	}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	for _, item := range []model.ServiceInstallation{
		{EnvironmentID: env.ID, ServiceType: "rabbitmq", Status: "draft", Namespace: "test-staging-rabbitmq"},
		{EnvironmentID: env.ID, ServiceType: "kafka", Status: "failed", Namespace: "test-staging-kafka", ErrorMessage: "image missing"},
		{EnvironmentID: env.ID, ServiceType: "redis", Status: "running", Namespace: "test-staging-redis"},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create service %s: %v", item.ServiceType, err)
		}
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec: paapv1.ApplicationSpec{
					Name:       "测试服务",
					Identifier: "test",
				},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "staging",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "预发",
					Identifier:       "staging",
					PrimaryNamespace: "test-staging",
				},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var installs []model.ServiceInstallation
	if err := db.Order("service_type").Find(&installs).Error; err != nil {
		t.Fatalf("list services: %v", err)
	}
	got := make([]string, 0, len(installs))
	for _, item := range installs {
		got = append(got, item.ServiceType+":"+item.Status)
	}
	want := []string{"kafka:failed", "rabbitmq:draft"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("services after sync = %#v, want %#v", got, want)
	}
}

func TestSyncClusterStateClearsEnvironmentErrorWhenRunning(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	app := model.Application{Name: "test", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.Environment{
		ApplicationID: app.ID,
		Name:          "staging",
		Identifier:    "staging",
		Status:        "error",
		Namespace:     "test-staging",
		ErrorMessage:  "stale error",
	}).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec:       paapv1.ApplicationSpec{Name: "test", Identifier: "test"},
				Status:     paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "staging",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "staging",
					Identifier:       "staging",
					PrimaryNamespace: "test-staging",
				},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var env model.Environment
	if err := db.Where("application_id = ? AND identifier = ?", app.ID, "staging").First(&env).Error; err != nil {
		t.Fatalf("load env: %v", err)
	}
	if env.Status != "empty" {
		t.Fatalf("expected empty status without services or components, got %q", env.Status)
	}
	if env.ErrorMessage != "" {
		t.Fatalf("expected cleared error message, got %q", env.ErrorMessage)
	}
}

func TestSyncClusterStateDeduplicatesServiceInstallations(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}
	if err := db.Exec("DROP INDEX IF EXISTS idx_service_installation_env_type").Error; err != nil {
		t.Fatalf("drop unique index: %v", err)
	}

	app := model.Application{Name: "test", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{
		ApplicationID: app.ID,
		Name:          "staging",
		Identifier:    "staging",
		Status:        "running",
		Namespace:     "test-staging",
	}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	running := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "git",
		ServiceName:   "staging-git",
		ReleaseName:   "test-staging-git",
		Status:        "running",
		Namespace:     "test-staging-git",
	}
	if err := db.Create(&running).Error; err != nil {
		t.Fatalf("create running service: %v", err)
	}
	duplicate := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "git",
		Status:        "installing",
		Namespace:     "test-staging-git",
	}
	if err := db.Create(&duplicate).Error; err != nil {
		t.Fatalf("create duplicate service: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec: paapv1.ApplicationSpec{
					Name:       "test",
					Identifier: "test",
				},
				Status: paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "staging",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "staging",
					Identifier:       "staging",
					PrimaryNamespace: "test-staging",
				},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
			&paapv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-git",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app":  "test",
						"paap.io/env":  "staging",
						"paap.io/tool": "git",
					},
				},
				Spec: paapv1.ServiceInstanceSpec{
					EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
					Type:           "git",
					ToolNamespace:  "test-staging-git",
					Helm: &paapv1.HelmInstallSpec{
						ReleaseName: "test-staging-git",
						Namespace:   "test-staging-git",
					},
				},
				Status: paapv1.ServiceInstanceStatus{Phase: "Running"},
			},
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var installs []model.ServiceInstallation
	if err := db.Where("environment_id = ? AND service_type = ?", env.ID, "git").Find(&installs).Error; err != nil {
		t.Fatalf("list service installations: %v", err)
	}
	if len(installs) != 1 {
		t.Fatalf("expected one git service installation after sync, got %d: %#v", len(installs), installs)
	}
	if installs[0].Status != "running" || installs[0].ServiceName != "staging-git" {
		t.Fatalf("unexpected deduplicated service installation: %#v", installs[0])
	}
}

func TestSyncClusterStateDeletesObsoleteDockerRegistryServiceInstances(t *testing.T) {
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
		t.Fatalf("migrate db: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	obsolete := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "staging-docker-registry",
			Namespace: "paap-app-test",
			Labels: map[string]string{
				"paap.io/app":  "test",
				"paap.io/env":  "staging",
				"paap.io/tool": "docker-registry",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
			Type:           "docker-registry",
			ToolNamespace:  "test-staging-docker-registry",
			Helm: &paapv1.HelmInstallSpec{
				ReleaseName: "test-staging-docker-registry",
				Namespace:   "test-staging-docker-registry",
			},
		},
		Status: paapv1.ServiceInstanceStatus{Phase: "Running"},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&paapv1.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "paap-system"},
				Spec: paapv1.ApplicationSpec{
					Name:       "test",
					Identifier: "test",
				},
				Status: paapv1.ApplicationStatus{Phase: "Active"},
			},
			&paapv1.Environment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "staging",
					},
				},
				Spec: paapv1.EnvironmentSpec{
					Name:             "staging",
					Identifier:       "staging",
					PrimaryNamespace: "test-staging",
				},
				Status: paapv1.EnvironmentStatus{Phase: "Running"},
			},
			obsolete,
		).
		Build()

	if err := SyncClusterState(context.Background(), db, k8sClient); err != nil {
		t.Fatalf("sync cluster state: %v", err)
	}

	var got paapv1.ServiceInstance
	err = k8sClient.Get(context.Background(), client.ObjectKeyFromObject(obsolete), &got)
	if err == nil {
		t.Fatalf("expected obsolete docker-registry CR to be deleted")
	}

	var count int64
	if err := db.Model(&model.ServiceInstallation{}).Where("service_type = ?", "docker-registry").Count(&count).Error; err != nil {
		t.Fatalf("count docker-registry installs: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no obsolete docker-registry DB installs, got %d", count)
	}
}
