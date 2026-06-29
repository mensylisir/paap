package service

import (
	"context"
	"errors"
	"log"
	"sort"
	"strings"

	"paap/internal/k8s"
	"paap/internal/model"

	"gorm.io/gorm"
)

var (
	ErrEnvironmentNotFound    = errors.New("environment not found")
	ErrEnvironmentNotReady    = errors.New("environment namespace is not ready")
	ErrComponentNotFound      = errors.New("component not found")
	ErrComponentAccessPatch   = errors.New("component external access patch failed")
	ErrComponentNodePortPatch = errors.New("component node port access patch failed")
)

type EnvironmentExternalAccess struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	URL         string `json:"url"`
	Namespace   string `json:"namespace"`
	Scope       string `json:"scope"`
	ServiceID   uint   `json:"serviceId,omitempty"`
	ServiceType string `json:"serviceType,omitempty"`
}

type ComponentView struct {
	model.Component
	RuntimeConfig *k8s.RuntimeConfig `json:"runtimeConfig,omitempty"`
	ExternalURL   string             `json:"externalUrl,omitempty"`
	IngressURL    string             `json:"ingressUrl"`
	NodePortURL   string             `json:"nodePortUrl"`
}

type ComponentAccessResult struct {
	View           ComponentView
	ExternalAccess []EnvironmentExternalAccess
}

func SetComponentExternalAccess(ctx context.Context, db *gorm.DB, envID uint, componentID uint, enabled bool) (ComponentAccessResult, error) {
	return setComponentAccess(ctx, db, envID, componentID, enabled, func(namespace, identifier string) error {
		if err := k8s.SetComponentExternalAccess(ctx, namespace, identifier, enabled); err != nil {
			return errors.Join(ErrComponentAccessPatch, err)
		}
		return nil
	})
}

func SetComponentNodePortAccess(ctx context.Context, db *gorm.DB, envID uint, componentID uint, enabled bool) (ComponentAccessResult, error) {
	return setComponentAccess(ctx, db, envID, componentID, enabled, func(namespace, identifier string) error {
		if err := k8s.SetComponentNodePortAccess(ctx, namespace, identifier, enabled); err != nil {
			return errors.Join(ErrComponentNodePortPatch, err)
		}
		return nil
	})
}

func setComponentAccess(ctx context.Context, db *gorm.DB, envID uint, componentID uint, enabled bool, patch func(namespace, identifier string) error) (ComponentAccessResult, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return ComponentAccessResult{}, err
	}
	if strings.TrimSpace(env.Namespace) == "" {
		return ComponentAccessResult{}, ErrEnvironmentNotReady
	}
	comp, err := findEnvironmentComponent(db, env.ID, componentID)
	if err != nil {
		return ComponentAccessResult{}, err
	}
	identifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	if err := patch(env.Namespace, identifier); err != nil {
		return ComponentAccessResult{}, err
	}

	access := CollectEnvironmentExternalAccess(ctx, db, env)
	view := BuildComponentView(ctx, env, comp, access)
	return ComponentAccessResult{View: view, ExternalAccess: access}, nil
}

func CollectEnvironmentExternalAccess(ctx context.Context, db *gorm.DB, env model.Environment) []EnvironmentExternalAccess {
	access := make([]EnvironmentExternalAccess, 0)
	seenURLs := map[string]struct{}{}

	appendNamespace := func(namespace, scope string) {
		namespace = strings.TrimSpace(namespace)
		if namespace == "" {
			return
		}
		endpoints, err := k8s.ListNamespaceExternalEndpoints(ctx, namespace)
		if err != nil {
			log.Printf("[external-access] list external access failed namespace=%s: %v", namespace, err)
			return
		}
		for _, endpoint := range endpoints {
			url := strings.TrimSpace(endpoint.URL)
			if url == "" {
				continue
			}
			key := namespace + "\x00" + url
			if _, ok := seenURLs[key]; ok {
				continue
			}
			seenURLs[key] = struct{}{}
			access = append(access, EnvironmentExternalAccess{
				Name:      endpoint.Name,
				Kind:      endpoint.Kind,
				URL:       url,
				Namespace: namespace,
				Scope:     scope,
			})
		}
	}

	appendNamespace(env.Namespace, "environment")
	var services []model.ServiceInstallation
	_ = db.Where("environment_id = ?", env.ID).Find(&services).Error
	for _, svc := range services {
		before := len(access)
		appendNamespace(svc.Namespace, "service")
		for i := before; i < len(access); i++ {
			access[i].ServiceID = svc.ID
			access[i].ServiceType = svc.ServiceType
		}
	}

	sort.Slice(access, func(i, j int) bool {
		if access[i].Scope != access[j].Scope {
			return access[i].Scope < access[j].Scope
		}
		if access[i].Namespace != access[j].Namespace {
			return access[i].Namespace < access[j].Namespace
		}
		if access[i].ServiceType != access[j].ServiceType {
			return access[i].ServiceType < access[j].ServiceType
		}
		if access[i].Name != access[j].Name {
			return access[i].Name < access[j].Name
		}
		return access[i].URL < access[j].URL
	})
	return access
}

func BuildComponentView(ctx context.Context, env model.Environment, comp model.Component, access []EnvironmentExternalAccess) ComponentView {
	view := ComponentView{Component: comp}
	identifier := ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	if cfg, err := k8s.DiscoverComponentRuntimeConfig(ctx, env.Namespace, identifier); err == nil {
		view.RuntimeConfig = cfg
	} else if err != nil {
		log.Printf("[external-access] discover component runtime config failed component=%s namespace=%s: %v", identifier, env.Namespace, err)
	}
	view.ExternalURL = componentExternalURL(comp, identifier, access)
	view.IngressURL = componentIngressURL(identifier, access)
	view.NodePortURL = componentNodePortURL(identifier, access)
	return view
}

func findEnvironment(db *gorm.DB, envID uint) (model.Environment, error) {
	var env model.Environment
	if err := db.First(&env, envID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Environment{}, ErrEnvironmentNotFound
		}
		return model.Environment{}, err
	}
	return env, nil
}

func findEnvironmentComponent(db *gorm.DB, envID uint, componentID uint) (model.Component, error) {
	var comp model.Component
	if err := db.Where("id = ? AND environment_id = ?", componentID, envID).First(&comp).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Component{}, ErrComponentNotFound
		}
		return model.Component{}, err
	}
	return comp, nil
}

func componentIngressURL(identifier string, access []EnvironmentExternalAccess) string {
	ingressName := "comp-" + identifier
	for _, item := range access {
		if item.Kind == "Ingress" && (item.Name == ingressName || strings.HasPrefix(item.Name, identifier+"-")) {
			return item.URL
		}
	}
	return ""
}

func componentNodePortURL(identifier string, access []EnvironmentExternalAccess) string {
	for _, item := range access {
		if item.Kind == "NodePort" && strings.Contains(item.Name, identifier) {
			return item.URL
		}
	}
	return ""
}

func componentExternalURL(comp model.Component, identifier string, access []EnvironmentExternalAccess) string {
	names := []string{identifier, comp.Name}
	matches := make([]EnvironmentExternalAccess, 0)
	for _, item := range access {
		if item.Scope == "service" {
			continue
		}
		for _, name := range names {
			if name != "" && strings.Contains(strings.ToLower(item.Name), strings.ToLower(name)) {
				matches = append(matches, item)
				break
			}
		}
	}
	if len(matches) > 0 {
		sort.SliceStable(matches, func(i, j int) bool {
			if externalAccessKindPriority(matches[i].Kind) != externalAccessKindPriority(matches[j].Kind) {
				return externalAccessKindPriority(matches[i].Kind) < externalAccessKindPriority(matches[j].Kind)
			}
			return matches[i].URL < matches[j].URL
		})
		return matches[0].URL
	}
	if strings.EqualFold(comp.Type, "frontend") {
		for _, item := range access {
			if item.Scope != "service" && strings.TrimSpace(item.URL) != "" {
				return item.URL
			}
		}
	}
	return ""
}

func externalAccessKindPriority(kind string) int {
	switch strings.TrimSpace(kind) {
	case "Gateway":
		return 0
	case "Ingress":
		return 1
	case "LoadBalancer":
		return 2
	case "NodePort":
		return 3
	default:
		return 9
	}
}
