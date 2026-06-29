package service

import (
	"fmt"
	"strings"

	"paap/internal/k8s"
	"paap/internal/model"
)

type KubeVirtServiceRuntimeContext struct {
	App          model.Application
	Env          model.Environment
	Installation model.ServiceInstallation
	Template     model.ServiceTemplate
}

func BuildKubeVirtServiceRuntimeResources(ctx KubeVirtServiceRuntimeContext) (k8s.KubeVirtServiceResources, error) {
	inst := ctx.Installation
	tmpl := ctx.Template
	serviceType := strings.TrimSpace(inst.ServiceType)
	if serviceType == "" {
		serviceType = strings.TrimSpace(tmpl.Type)
	}
	if serviceType == "" {
		return k8s.KubeVirtServiceResources{}, fmt.Errorf("service type is required")
	}
	if NormalizeServiceProvisionMode(inst.ProvisionMode) != model.ServiceProvisionModeKubeVirt &&
		NormalizeServiceProvisionMode(tmpl.ProvisionMode) != model.ServiceProvisionModeKubeVirt {
		return k8s.KubeVirtServiceResources{}, fmt.Errorf("service %s is not a kubevirt provisioned service", serviceType)
	}

	runtimeSpec := strings.TrimSpace(inst.RuntimeSpec)
	if runtimeSpec == "" {
		runtimeSpec = strings.TrimSpace(tmpl.RuntimeSpec)
	}
	if runtimeSpec == "" {
		return k8s.KubeVirtServiceResources{}, fmt.Errorf("kubevirt runtime spec is required")
	}

	namespace := strings.TrimSpace(inst.Namespace)
	if namespace == "" {
		namespace = normalizeIdentifier(ctx.App.Identifier+"-"+ctx.Env.Identifier+"-"+serviceType, serviceType, 63)
	}
	serviceName := strings.TrimSpace(inst.ServiceName)
	if serviceName == "" {
		serviceName = namespace
	}

	return k8s.BuildKubeVirtServiceResources(k8s.KubeVirtServiceResourceInput{
		AppIdentifier: strings.TrimSpace(ctx.App.Identifier),
		EnvIdentifier: strings.TrimSpace(ctx.Env.Identifier),
		ServiceType:   serviceType,
		ServiceName:   serviceName,
		Namespace:     namespace,
		RuntimeSpec:   runtimeSpec,
		Labels: map[string]string{
			"paap.io/category": strings.TrimSpace(tmpl.Category),
			"paap.io/tool":     normalizeIdentifier(strings.TrimSpace(tmpl.Name), serviceType, 50),
		},
	})
}
