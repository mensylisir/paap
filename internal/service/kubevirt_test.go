package service

import (
	"strings"
	"testing"

	"paap/internal/model"
)

func TestBuildKubeVirtServiceRuntimeResourcesUsesInstallationRuntimeSpec(t *testing.T) {
	resources, err := BuildKubeVirtServiceRuntimeResources(KubeVirtServiceRuntimeContext{
		App: model.Application{Identifier: "billing"},
		Env: model.Environment{Identifier: "dev"},
		Installation: model.ServiceInstallation{
			ServiceType:   "postgresql",
			ServiceName:   "billing-dev-postgresql",
			Namespace:     "billing-dev-postgresql",
			ProvisionMode: model.ServiceProvisionModeKubeVirt,
			RuntimeSpec:   `{"image":"postgres:16","ports":[{"name":"postgresql","port":5432}],"credentials":{"username":"app","password":"secret"}}`,
		},
		Template: model.ServiceTemplate{
			Type:          "postgresql",
			Category:      "database",
			ProvisionMode: model.ServiceProvisionModeKubeVirt,
			RuntimeSpec:   `{"image":"postgres:15","ports":[{"port":15432}]}`,
		},
	})
	if err != nil {
		t.Fatalf("BuildKubeVirtServiceRuntimeResources() error = %v", err)
	}
	if resources.VirtualMachine.GetName() != "billing-dev-postgresql" {
		t.Fatalf("vm name = %q", resources.VirtualMachine.GetName())
	}
	if resources.VirtualMachine.GetLabels()["paap.io/service-type"] != "postgresql" {
		t.Fatalf("vm labels = %#v", resources.VirtualMachine.GetLabels())
	}
	if resources.Service.Spec.Ports[0].Port != 5432 {
		t.Fatalf("service port = %d", resources.Service.Spec.Ports[0].Port)
	}
	if string(resources.Secret.Data["username"]) != "app" {
		t.Fatalf("secret username = %q", string(resources.Secret.Data["username"]))
	}
}

func TestBuildKubeVirtServiceRuntimeResourcesRejectsManagedService(t *testing.T) {
	_, err := BuildKubeVirtServiceRuntimeResources(KubeVirtServiceRuntimeContext{
		App: model.Application{Identifier: "billing"},
		Env: model.Environment{Identifier: "dev"},
		Installation: model.ServiceInstallation{
			ServiceType:   "postgresql",
			ProvisionMode: model.ServiceProvisionModeManaged,
		},
		Template: model.ServiceTemplate{
			Type:          "postgresql",
			ProvisionMode: model.ServiceProvisionModeManaged,
			RuntimeSpec:   `{"image":"postgres:16","ports":[{"port":5432}]}`,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "not a kubevirt") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildKubeVirtServiceRuntimeResourcesFallsBackToTemplateSpecAndNamespace(t *testing.T) {
	resources, err := BuildKubeVirtServiceRuntimeResources(KubeVirtServiceRuntimeContext{
		App: model.Application{Identifier: "billing"},
		Env: model.Environment{Identifier: "stage"},
		Installation: model.ServiceInstallation{
			ServiceType:   "redis",
			ProvisionMode: model.ServiceProvisionModeKubeVirt,
		},
		Template: model.ServiceTemplate{
			Type:          "redis",
			Name:          "Redis VM",
			Category:      "middleware",
			ProvisionMode: model.ServiceProvisionModeKubeVirt,
			RuntimeSpec:   `{"image":"redis:7","ports":[{"port":6379}]}`,
		},
	})
	if err != nil {
		t.Fatalf("BuildKubeVirtServiceRuntimeResources() error = %v", err)
	}
	if resources.Service.Namespace != "billing-stage-redis" {
		t.Fatalf("service namespace = %q", resources.Service.Namespace)
	}
	if resources.VirtualMachine.GetLabels()["paap.io/tool"] != "redis-vm" {
		t.Fatalf("vm labels = %#v", resources.VirtualMachine.GetLabels())
	}
}
