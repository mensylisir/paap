package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	paapv1 "paap/api/v1"
	"paap/internal/k8s"
	"paap/internal/model"

	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceCredential struct {
	Secret string `json:"secret"`
	Key    string `json:"key"`
	Value  string `json:"value"`
	Kind   string `json:"kind"`
}

type ServiceInstallationView struct {
	model.ServiceInstallation
	RuntimeConfig      *k8s.RuntimeConfig `json:"runtimeConfig,omitempty"`
	ExternalURL        string             `json:"externalUrl,omitempty"`
	RuntimeServiceName string             `json:"runtimeServiceName,omitempty"`
	RuntimeServiceType string             `json:"runtimeServiceType,omitempty"`
	ClusterIP          string             `json:"clusterIP,omitempty"`
	LoadBalancerIP     string             `json:"loadBalancerIP,omitempty"`
}

type ServiceInstanceDetail struct {
	Installation ServiceInstallationView       `json:"installation"`
	CRStatus     *paapv1.ServiceInstanceStatus `json:"crStatus"`
}

type ServiceWorkspaceContext struct {
	App        model.Application
	Env        model.Environment
	Instance   model.ServiceInstallation
	Components []model.Component
}

func ListServiceInstances(ctx context.Context, db *gorm.DB, envID uint) ([]ServiceInstallationView, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return nil, err
	}

	var services []model.ServiceInstallation
	if err := db.Where("environment_id = ?", env.ID).Find(&services).Error; err != nil {
		return nil, err
	}
	enrichServiceInstallationsWithCRStatus(ctx, db, env, services)
	access := CollectEnvironmentExternalAccess(ctx, db, env)
	return BuildServiceInstallationViews(ctx, services, access), nil
}

func GetServiceInstance(ctx context.Context, db *gorm.DB, envID uint, serviceID uint) (ServiceInstanceDetail, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return ServiceInstanceDetail{}, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return ServiceInstanceDetail{}, err
	}
	inst, err := findServiceInstallation(db, envID, serviceID)
	if err != nil {
		return ServiceInstanceDetail{}, err
	}

	crStatus, err := k8s.GetServiceInstanceCRStatus(ctx, app.Identifier, env.Identifier, inst.ServiceType)
	if NormalizeServiceProvisionMode(inst.ProvisionMode) == model.ServiceProvisionModeKubeVirt {
		crStatus = nil
	} else if err != nil {
		log.Printf("[GetServiceInstance] CR status query warning: %v", err)
	}
	access := CollectEnvironmentExternalAccess(ctx, db, env)
	view := BuildServiceInstallationViews(ctx, []model.ServiceInstallation{inst}, access)[0]
	return ServiceInstanceDetail{Installation: view, CRStatus: crStatus}, nil
}

func GetServiceCredentials(ctx context.Context, db *gorm.DB, envID uint, serviceID uint) ([]ServiceCredential, error) {
	if _, err := findEnvironment(db, envID); err != nil {
		return nil, err
	}
	inst, err := findServiceInstallation(db, envID, serviceID)
	if err != nil {
		return nil, err
	}
	return DiscoverServiceCredentials(ctx, inst.Namespace)
}

func LoadServiceWorkspaceContext(db *gorm.DB, envID uint, serviceID uint) (ServiceWorkspaceContext, error) {
	inst, err := findServiceInstallation(db, envID, serviceID)
	if err != nil {
		return ServiceWorkspaceContext{}, err
	}
	env, err := findEnvironment(db, envID)
	if err != nil {
		return ServiceWorkspaceContext{}, err
	}
	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		return ServiceWorkspaceContext{}, err
	}
	var components []model.Component
	if err := db.Where("environment_id = ?", env.ID).Find(&components).Error; err != nil {
		return ServiceWorkspaceContext{}, err
	}
	return ServiceWorkspaceContext{App: app, Env: env, Instance: inst, Components: components}, nil
}

func BuildServiceInstallationViews(ctx context.Context, services []model.ServiceInstallation, access []EnvironmentExternalAccess) []ServiceInstallationView {
	views := make([]ServiceInstallationView, 0, len(services))
	for _, inst := range services {
		view := ServiceInstallationView{ServiceInstallation: inst}
		if NormalizeServiceProvisionMode(inst.ProvisionMode) == model.ServiceProvisionModeKubeVirt {
			if cfg, err := k8s.DiscoverKubeVirtServiceRuntimeConfig(ctx, inst.Namespace, inst.ServiceType); err == nil {
				view.RuntimeConfig = cfg
			} else if err != nil {
				log.Printf("[service-instance] discover kubevirt runtime config failed service=%s namespace=%s: %v", inst.ServiceType, inst.Namespace, err)
			}
		} else if cfg, err := k8s.DiscoverNamespaceRuntimeConfig(ctx, inst.Namespace); err == nil {
			view.RuntimeConfig = cfg
		} else if err != nil {
			log.Printf("[service-instance] discover service runtime config failed service=%s namespace=%s: %v", inst.ServiceType, inst.Namespace, err)
		}
		if network, err := k8s.DiscoverNamespaceServiceNetwork(ctx, inst.Namespace, inst.ServiceType); err == nil && network != nil {
			view.RuntimeServiceName = network.ServiceName
			view.RuntimeServiceType = network.ServiceType
			view.ClusterIP = network.ClusterIP
			view.LoadBalancerIP = network.LoadBalancerIP
		} else if err != nil {
			log.Printf("[service-instance] discover service network failed service=%s namespace=%s: %v", inst.ServiceType, inst.Namespace, err)
		}
		view.ExternalURL = serviceExternalURL(inst, access)
		views = append(views, view)
	}
	return views
}

func DiscoverServiceCredentials(ctx context.Context, namespace string) ([]ServiceCredential, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, fmt.Errorf("service namespace is empty")
	}
	cl := k8s.GetClient()
	if cl == nil {
		return nil, fmt.Errorf("kubernetes client is not initialized")
	}
	secrets := &corev1.SecretList{}
	if err := cl.List(ctx, secrets, ctrlclient.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	credentials := make([]ServiceCredential, 0)
	for _, secret := range secrets.Items {
		if shouldSkipCredentialSecret(secret) {
			continue
		}
		for key, raw := range secret.Data {
			kind, ok := CredentialKeyKind(key)
			if !ok || len(raw) == 0 {
				continue
			}
			credentials = append(credentials, ServiceCredential{
				Secret: secret.Name,
				Key:    key,
				Value:  string(raw),
				Kind:   kind,
			})
		}
	}
	sort.Slice(credentials, func(i, j int) bool {
		if credentials[i].Secret != credentials[j].Secret {
			return credentials[i].Secret < credentials[j].Secret
		}
		return credentials[i].Key < credentials[j].Key
	})
	return credentials, nil
}

func findServiceInstallation(db *gorm.DB, envID uint, serviceID uint) (model.ServiceInstallation, error) {
	var inst model.ServiceInstallation
	if err := db.Where("id = ? AND environment_id = ?", serviceID, envID).First(&inst).Error; err != nil {
		if errorsIsRecordNotFound(err) {
			return model.ServiceInstallation{}, ErrServiceInstallationNotFound
		}
		return model.ServiceInstallation{}, err
	}
	return inst, nil
}

func enrichServiceInstallationsWithCRStatus(ctx context.Context, db *gorm.DB, env model.Environment, services []model.ServiceInstallation) {
	if len(services) == 0 {
		return
	}

	app, err := findApplication(db, env.ApplicationID)
	if err != nil {
		log.Printf("[ListServiceInstances] application lookup warning: %v", err)
		return
	}

	for i := range services {
		if NormalizeServiceProvisionMode(services[i].ProvisionMode) == model.ServiceProvisionModeKubeVirt {
			status, err := k8s.DiscoverKubeVirtServiceStatus(ctx, services[i].Namespace, services[i].ServiceType)
			if err != nil {
				log.Printf("[ListServiceInstances] KubeVirt status query warning for %s: %v", services[i].ServiceType, err)
				continue
			}
			if status == nil {
				continue
			}
			if phase := normalizeServicePhase(status.Phase, services[i].Status); phase != "" {
				services[i].Status = phase
			}
			services[i].ErrorMessage = strings.TrimSpace(status.Message)
			continue
		}
		crStatus, err := k8s.GetServiceInstanceCRStatus(ctx, app.Identifier, env.Identifier, services[i].ServiceType)
		if err != nil {
			log.Printf("[ListServiceInstances] CR status query warning for %s: %v", services[i].ServiceType, err)
			continue
		}
		if crStatus == nil {
			continue
		}
		if status := normalizeServicePhase(crStatus.Phase, services[i].Status); status != "" {
			services[i].Status = status
		}
		services[i].ErrorMessage = serviceStatusErrorMessage(crStatus)
	}
}

func normalizeServicePhase(phase, fallback string) string {
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase == "" {
		return fallback
	}
	return phase
}

func serviceStatusErrorMessage(status *paapv1.ServiceInstanceStatus) string {
	if status == nil {
		return ""
	}
	for _, cond := range status.Conditions {
		if strings.EqualFold(cond.Type, "Ready") && cond.Status == "False" {
			if strings.TrimSpace(cond.Message) != "" {
				return cond.Message
			}
			if strings.TrimSpace(cond.Reason) != "" {
				return cond.Reason
			}
		}
	}
	return ""
}

func serviceExternalURL(inst model.ServiceInstallation, access []EnvironmentExternalAccess) string {
	for _, item := range access {
		if item.Scope == "service" && item.ServiceID == inst.ID && item.URL != "" {
			return item.URL
		}
	}
	for _, item := range access {
		if item.Scope == "service" && item.ServiceType == inst.ServiceType && item.URL != "" {
			return item.URL
		}
	}
	return ""
}

func shouldSkipCredentialSecret(secret corev1.Secret) bool {
	switch secret.Type {
	case corev1.SecretTypeServiceAccountToken, corev1.SecretTypeTLS, corev1.SecretTypeDockerConfigJson, corev1.SecretTypeDockercfg:
		return true
	default:
		return false
	}
}

func CredentialKeyKind(key string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch {
	case normalized == "username", normalized == "user", strings.HasSuffix(normalized, "-username"):
		return "username", true
	case normalized == "password", normalized == "passwd", normalized == "root-password", strings.Contains(normalized, "password"):
		return "password", true
	case normalized == "accesskey", normalized == "access-key", normalized == "access_key", strings.Contains(normalized, "accesskey"):
		return "accessKey", true
	case normalized == "secretkey", normalized == "secret-key", normalized == "secret_key", strings.Contains(normalized, "secretkey"):
		return "secretKey", true
	default:
		return "", false
	}
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
