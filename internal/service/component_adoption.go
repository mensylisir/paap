package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	"paap/internal/k8s"
	"paap/internal/model"

	"gorm.io/gorm"
)

var (
	ErrAdoptableResourceNotFound    = errors.New("adoptable resource not found")
	ErrSystemSharedEnvironmentAdopt = errors.New("system shared environments cannot adopt workload resources")
)

type AdoptedComponentResult struct {
	Component     model.Component
	RuntimeConfig *k8s.RuntimeConfig
}

func ListAdoptableResources(ctx context.Context, db *gorm.DB, envID uint) ([]k8s.AdoptableResource, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return nil, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return nil, err
	}
	return discoverAdoptableResourcesForEnvironment(ctx, db, app, env)
}

func AdoptResourceAsComponent(ctx context.Context, db *gorm.DB, envID uint, key string) (AdoptedComponentResult, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return AdoptedComponentResult{}, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return AdoptedComponentResult{}, err
	}
	if model.IsSystemSharedEnvironment(app, env) {
		return AdoptedComponentResult{}, ErrSystemSharedEnvironmentAdopt
	}

	resources, err := discoverAdoptableResourcesForEnvironment(ctx, db, app, env)
	if err != nil {
		return AdoptedComponentResult{}, err
	}
	var selected *k8s.AdoptableResource
	for i := range resources {
		if resources[i].Key == key {
			selected = &resources[i]
			break
		}
	}
	if selected == nil {
		return AdoptedComponentResult{}, ErrAdoptableResourceNotFound
	}

	comp, err := componentFromAdoptableResource(env, *selected)
	if err != nil {
		return AdoptedComponentResult{}, ComponentValidationError{Message: err.Error()}
	}
	if err := db.Create(&comp).Error; err != nil {
		return AdoptedComponentResult{}, err
	}
	return AdoptedComponentResult{Component: comp, RuntimeConfig: selected.RuntimeConfig}, nil
}

func discoverAdoptableResourcesForEnvironment(ctx context.Context, db *gorm.DB, app model.Application, env model.Environment) ([]k8s.AdoptableResource, error) {
	namespaces := adoptableEnvironmentNamespaces(env)
	var services []model.ServiceInstallation
	if err := db.Where("environment_id = ?", env.ID).Find(&services).Error; err != nil {
		return nil, err
	}
	var components []model.Component
	if err := db.Where("environment_id = ?", env.ID).Find(&components).Error; err != nil {
		return nil, err
	}

	managed := managedAdoptableResourceKeys(env, services, components)
	out := make([]k8s.AdoptableResource, 0)
	seen := map[string]struct{}{}
	for _, namespace := range uniqueStrings(namespaces) {
		items, err := k8s.ListNamespaceAdoptableResources(ctx, namespace)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !adoptableResourceBelongsToEnvironment(item, app, env) {
				continue
			}
			if _, exists := managed[item.Key]; exists {
				continue
			}
			if _, exists := seen[item.Key]; exists {
				continue
			}
			seen[item.Key] = struct{}{}
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func adoptableEnvironmentNamespaces(env model.Environment) []string {
	base := strings.TrimSpace(env.Namespace)
	if base == "" {
		return nil
	}
	return []string{base, base + "-app"}
}

func managedAdoptableResourceKeys(env model.Environment, services []model.ServiceInstallation, components []model.Component) map[string]struct{} {
	managed := map[string]struct{}{}
	for _, comp := range components {
		identifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
		for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet"} {
			managed[strings.ToLower(env.Namespace+"/"+kind+"/"+identifier)] = struct{}{}
			managed[strings.ToLower(env.Namespace+"/"+kind+"/"+comp.Name)] = struct{}{}
		}
	}
	for _, inst := range services {
		name := strings.TrimSpace(inst.ReleaseName)
		if name == "" {
			name = strings.TrimSpace(inst.ServiceName)
		}
		if name == "" {
			name = strings.TrimSpace(inst.ServiceType)
		}
		for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet"} {
			managed[strings.ToLower(inst.Namespace+"/"+kind+"/"+name)] = struct{}{}
			managed[strings.ToLower(inst.Namespace+"/"+kind+"/"+inst.ServiceType)] = struct{}{}
		}
	}
	return managed
}

func adoptableResourceBelongsToEnvironment(item k8s.AdoptableResource, app model.Application, env model.Environment) bool {
	namespace := strings.TrimSpace(item.Namespace)
	if namespace == "" {
		return false
	}
	base := strings.TrimSpace(env.Namespace)
	if base != "" && (namespace == base || strings.HasPrefix(namespace, base+"-")) {
		return true
	}
	prefix := strings.Trim(strings.TrimSpace(app.Identifier)+"-"+strings.TrimSpace(env.Identifier), "-")
	return prefix != "" && (namespace == prefix || strings.HasPrefix(namespace, prefix+"-"))
}

func componentFromAdoptableResource(env model.Environment, item k8s.AdoptableResource) (model.Component, error) {
	cfg := componentConfigFromRuntimeConfig(item.RuntimeConfig)
	configJSON, err := cfg.JSON()
	if err != nil {
		return model.Component{}, err
	}
	image := runtimeImageOrDescription(item)
	replicas := int32(1)
	if item.RuntimeConfig != nil && item.RuntimeConfig.Replicas != nil {
		replicas = *item.RuntimeConfig.Replicas
	}
	if replicas < 0 {
		replicas = 0
	}
	comp := model.Component{
		EnvironmentID: env.ID,
		Name:          item.Name,
		Type:          valueOrDefaultString(item.ComponentType, "backend"),
		Image:         image,
		Version:       imageReferenceTag(image),
		Replicas:      int(replicas),
		Status:        "draft",
		DeliveryMode:  "image",
		Config:        configJSON,
	}
	if comp.Replicas == 0 && !strings.EqualFold(item.Status, "stopped") {
		comp.Replicas = 1
	}
	if item.RuntimeConfig != nil {
		comp.CPU = item.RuntimeConfig.Resources.Requests["cpu"]
		comp.Memory = item.RuntimeConfig.Resources.Requests["memory"]
	}
	return comp, nil
}

func componentConfigFromRuntimeConfig(cfg *k8s.RuntimeConfig) model.ComponentConfig {
	if cfg == nil {
		return model.ComponentConfig{}
	}
	out := model.ComponentConfig{
		Command: append([]string{}, cfg.Command...),
		Args:    append([]string{}, cfg.Args...),
		Env:     make([]model.ComponentEnvVar, 0, len(cfg.Env)),
		Files:   make([]model.ComponentConfigFile, 0, len(cfg.Files)),
	}
	if len(cfg.Ports) > 0 {
		out.ContainerPort = cfg.Ports[0]
	}
	for _, env := range cfg.Env {
		out.Env = append(out.Env, model.ComponentEnvVar{
			Name:          env.Name,
			Value:         env.Value,
			SecretName:    env.SecretName,
			SecretKey:     env.SecretKey,
			ConfigMapName: env.ConfigMapName,
			ConfigMapKey:  env.ConfigMapKey,
		})
	}
	for _, file := range cfg.Files {
		if file.Kind != "configMap" || strings.TrimSpace(file.ObjectName) == "" || strings.TrimSpace(file.Key) == "" || strings.TrimSpace(file.MountPath) == "" {
			continue
		}
		out.Files = append(out.Files, model.ComponentConfigFile{
			Name:          strings.Trim(strings.TrimSpace(file.ObjectName)+"-"+strings.TrimSpace(file.Key), "-"),
			ConfigMapName: strings.TrimSpace(file.ObjectName),
			Key:           strings.TrimSpace(file.Key),
			MountPath:     strings.TrimSpace(file.MountPath),
			ReadOnly:      true,
		})
	}
	return out
}

func runtimeImageOrDescription(item k8s.AdoptableResource) string {
	if item.RuntimeConfig != nil && strings.TrimSpace(item.RuntimeConfig.Image) != "" {
		return strings.TrimSpace(item.RuntimeConfig.Image)
	}
	return strings.TrimSpace(item.Description)
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
