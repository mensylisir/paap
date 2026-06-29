package service

import (
	"encoding/json"
	"testing"

	"paap/internal/model"
)

func TestLoadServiceWorkloadRoleDefaultsToNoProjectedPermissions(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if got := LoadServiceWorkloadRole(db, "missing"); len(got.Rules) != 0 {
		t.Fatalf("missing template should not get projected workload permissions: %#v", got.Rules)
	}

	if err := db.Create(&model.ServiceTemplate{Type: "redis"}).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	if got := LoadServiceWorkloadRole(db, "redis"); len(got.Rules) != 0 {
		t.Fatalf("empty workload policy should not get projected workload permissions: %#v", got.Rules)
	}
}

func TestLoadServiceEnvironmentRoleReadsCustomTemplatePolicy(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	manifestJSON := serviceTemplateManifestJSON(t, model.PlatformManifest{
		Name:    "custom-monitor",
		Version: "v1",
		Permissions: model.PermissionsSpec{
			EnvironmentNamespaces: model.NamespacePermissionsSpec{
				Rules: []model.PolicyRuleSpec{
					{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
				},
			},
		},
	})
	if err := db.Create(&model.ServiceTemplate{
		Type:                 "custom-monitor",
		PlatformManifestJSON: manifestJSON,
	}).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}

	if got := LoadServiceWorkloadRole(db, "custom-monitor"); len(got.Rules) != 0 {
		t.Fatalf("custom monitor should not get workload permissions: %#v", got.Rules)
	}
	got := LoadServiceEnvironmentRole(db, "custom-monitor")
	if got == nil || len(got.Rules) != 1 {
		t.Fatalf("custom monitor should get environment permissions, got %#v", got)
	}
	if got.Rules[0].APIGroups[0] != "*" || got.Rules[0].Resources[0] != "*" || got.Rules[0].Verbs[0] != "*" {
		t.Fatalf("unexpected environment policy: %#v", got.Rules[0])
	}
}

func TestServiceTemplateModelDoesNotMigrateLegacyRolePolicyColumns(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, column := range []string{"workload_role_policy", "environment_role_policy"} {
		if db.Migrator().HasColumn(&model.ServiceTemplate{}, column) {
			t.Fatalf("legacy service template column %s must not be migrated", column)
		}
	}
	if !db.Migrator().HasColumn(&model.ServiceTemplate{}, "platform_manifest_json") {
		t.Fatalf("platform_manifest_json must remain the service template RBAC source")
	}
}

func serviceTemplateManifestJSON(t *testing.T, manifest model.PlatformManifest) string {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal platform manifest: %v", err)
	}
	return string(data)
}
