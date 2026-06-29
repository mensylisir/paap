package service

import (
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

	if err := db.Create(&model.ServiceTemplate{Type: "redis", WorkloadRolePolicy: ""}).Error; err != nil {
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

	policy := `[{"apiGroups":["*"],"resources":["*"],"verbs":["*"]}]`
	if err := db.Create(&model.ServiceTemplate{
		Type:                  "custom-monitor",
		WorkloadRolePolicy:    "[]",
		EnvironmentRolePolicy: policy,
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
