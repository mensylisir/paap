package service

import (
	"strings"
	"testing"

	"paap/internal/model"
)

func TestUpsertSeedServiceTemplateSeparatesProvisionModes(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	managed := model.ServiceTemplate{
		Type:          "postgresql",
		Name:          "PostgreSQL Helm",
		Category:      "infra",
		Installer:     "helm",
		ProvisionMode: model.ServiceProvisionModeManaged,
		S3Key:         "charts/postgresql.tar.gz",
		InstallOrder:  100,
		Enabled:       true,
	}
	kubevirt := model.ServiceTemplate{
		Type:          "postgresql",
		Name:          "PostgreSQL KubeVirt",
		Category:      "database",
		Installer:     "raw-yaml",
		ProvisionMode: model.ServiceProvisionModeKubeVirt,
		RuntimeSpec:   `{"image":"postgres:16","ports":[{"port":5432}]}`,
		S3Key:         "service-templates/kubevirt/postgresql.json",
		InstallOrder:  100,
		Enabled:       true,
	}
	if err := UpsertSeedServiceTemplate(db, managed); err != nil {
		t.Fatalf("upsert managed: %v", err)
	}
	if err := UpsertSeedServiceTemplate(db, kubevirt); err != nil {
		t.Fatalf("upsert kubevirt: %v", err)
	}
	kubevirt.Name = "PostgreSQL VM"
	if err := UpsertSeedServiceTemplate(db, kubevirt); err != nil {
		t.Fatalf("refresh kubevirt: %v", err)
	}

	var count int64
	if err := db.Model(&model.ServiceTemplate{}).Where("type = ?", "postgresql").Count(&count).Error; err != nil {
		t.Fatalf("count templates: %v", err)
	}
	if count != 2 {
		t.Fatalf("postgresql template count = %d, want managed+kubevirt", count)
	}

	app := model.Application{Name: "Billing", Identifier: "billing"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	managedCtx, err := LoadServiceInstallTemplateContext(db, env.ID, "postgresql", ServiceInstallTemplateQuery{EnabledOnly: true})
	if err != nil {
		t.Fatalf("load managed context: %v", err)
	}
	if managedCtx.Template.ProvisionMode != model.ServiceProvisionModeManaged || managedCtx.Template.Installer != "helm" {
		t.Fatalf("managed context template = %#v", managedCtx.Template)
	}

	kubevirtCtx, err := LoadServiceInstallTemplateContext(db, env.ID, "postgresql", ServiceInstallTemplateQuery{
		ProvisionMode: model.ServiceProvisionModeKubeVirt,
		EnabledOnly:   true,
	})
	if err != nil {
		t.Fatalf("load kubevirt context: %v", err)
	}
	if kubevirtCtx.Template.ProvisionMode != model.ServiceProvisionModeKubeVirt || kubevirtCtx.Template.Installer != "raw-yaml" {
		t.Fatalf("kubevirt context template = %#v", kubevirtCtx.Template)
	}
	if !strings.Contains(kubevirtCtx.Template.RuntimeSpec, "postgres:16") {
		t.Fatalf("kubevirt runtime spec = %q", kubevirtCtx.Template.RuntimeSpec)
	}
}

func TestCreateServiceTemplateValidatesKubeVirtRuntimeSpec(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	_, err = CreateServiceTemplate(db, SaveServiceTemplateInput{
		Type:          "postgresql",
		Name:          "Bad PostgreSQL VM",
		Category:      "database",
		Installer:     "raw-yaml",
		ProvisionMode: model.ServiceProvisionModeKubeVirt,
		RuntimeSpec:   `{"ports":[{"port":5432}]}`,
	})
	if err == nil || !strings.Contains(err.Error(), "image is required") {
		t.Fatalf("error = %v, want kubevirt image validation", err)
	}

	tmpl, err := CreateServiceTemplate(db, SaveServiceTemplateInput{
		Type:          "postgresql",
		Name:          "PostgreSQL VM",
		Category:      "database",
		Installer:     "raw-yaml",
		ProvisionMode: model.ServiceProvisionModeKubeVirt,
		RuntimeSpec:   `{"image":"postgres:16","ports":[{"port":5432}],"readiness":{"type":"tcp","port":5432},"monitoring":{"enabled":true,"port":9187},"backupPolicy":{"enabled":true,"schedule":"0 2 * * *"}}`,
	})
	if err != nil {
		t.Fatalf("create valid kubevirt template: %v", err)
	}
	if tmpl.ProvisionMode != model.ServiceProvisionModeKubeVirt || !strings.Contains(tmpl.RuntimeSpec, `"backupPolicy"`) {
		t.Fatalf("saved kubevirt template = %#v", tmpl)
	}
}
