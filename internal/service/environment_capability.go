package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"paap/internal/k8s"
	"paap/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	SystemSharedApplicationIdentifier = "default"
	SystemSharedEnvironmentIdentifier = "shared"
	SystemSharedApplicationName       = "共享资源池"
	SystemSharedEnvironmentName       = "共享环境"
)

var (
	ErrSharedResourcePoolNotFound            = errors.New("shared resource pool not found")
	ErrEnvironmentCapabilityNotFound         = errors.New("capability not found")
	ErrSystemSharedEnvironmentMutation       = errors.New("system shared environments cannot add shared or external resource cards")
	ErrEnvironmentCapabilityInvalid          = errors.New("invalid capability")
	ErrEnvironmentCapabilitySourceInvalid    = errors.New("invalid capability source")
	ErrCapabilityValidationUnsupported       = errors.New("only external capabilities can be validated")
	ErrCapabilityCredentialsUnsupported      = errors.New("capability is not external or shared")
	ErrSharedServiceNotFound                 = errors.New("shared service not found")
	ErrManagedServiceNotFound                = errors.New("managed service not found")
	ErrCredentialSecretForbidden             = errors.New("credential secret must belong to current environment namespace")
	ErrCredentialSecretNotFound              = errors.New("credential secret not found")
	ErrKubernetesClientNotInitialized        = errors.New("kubernetes client is not initialized")
	ErrEnvironmentCapabilitySecretRefInvalid = errors.New("invalid credentialSecretRef")
)

type EnvironmentCapabilityRequest struct {
	Capability            string `json:"capability"`
	CapabilityKey         string `json:"capabilityKey"`
	Source                string `json:"source"`
	Provider              string `json:"provider"`
	ServiceType           string `json:"serviceType"`
	RefServiceID          *uint  `json:"refServiceId"`
	ExternalEndpoint      string `json:"externalEndpoint"`
	CredentialSecretRef   string `json:"credentialSecretRef"`
	AuthType              string `json:"authType"`
	Username              string `json:"username"`
	Password              string `json:"password"`
	Token                 string `json:"token"`
	TLSInsecureSkipVerify bool   `json:"tlsInsecureSkipVerify"`
}

type SharedCapabilityResource struct {
	ID          uint   `json:"id"`
	Capability  string `json:"capability"`
	Provider    string `json:"provider"`
	ServiceType string `json:"serviceType"`
	ServiceName string `json:"serviceName"`
	Status      string `json:"status"`
	Namespace   string `json:"namespace"`
}

func LoadSystemSharedEnvironment(db *gorm.DB) (model.Application, model.Environment, error) {
	var app model.Application
	if err := db.
		Where("identifier = ? AND is_system = ?", SystemSharedApplicationIdentifier, true).
		First(&app).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Application{}, model.Environment{}, ErrSharedResourcePoolNotFound
		}
		return model.Application{}, model.Environment{}, err
	}

	var env model.Environment
	if err := db.
		Where("application_id = ? AND identifier = ? AND is_system = ?", app.ID, SystemSharedEnvironmentIdentifier, true).
		First(&env).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Application{}, model.Environment{}, ErrSharedResourcePoolNotFound
		}
		return model.Application{}, model.Environment{}, err
	}
	return app, env, nil
}

func ListEnvironmentCapabilities(db *gorm.DB, envID uint) ([]model.EnvironmentCapability, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return nil, err
	}

	var capabilities []model.EnvironmentCapability
	if err := db.Where("environment_id = ?", env.ID).
		Preload("RefService").
		Order("capability").
		Find(&capabilities).Error; err != nil {
		return nil, err
	}
	return capabilities, nil
}

func UpsertEnvironmentCapability(ctx context.Context, db *gorm.DB, envID uint, identifier string, req EnvironmentCapabilityRequest, createdBy uint) (model.EnvironmentCapability, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return model.EnvironmentCapability{}, err
	}
	if EnvironmentIsSystemSharedPool(db, env) {
		return model.EnvironmentCapability{}, ErrSystemSharedEnvironmentMutation
	}

	capability, ok := capabilityFromRouteOrExisting(db, env.ID, identifier)
	if !ok {
		return model.EnvironmentCapability{}, ErrEnvironmentCapabilityInvalid
	}
	source, ok := NormalizeCapabilitySource(req.Source)
	if !ok {
		return model.EnvironmentCapability{}, ErrEnvironmentCapabilitySourceInvalid
	}

	capabilityRow, err := buildEnvironmentCapability(ctx, db, env, capability, source, req)
	if err != nil {
		return model.EnvironmentCapability{}, ValidationError{Message: err.Error()}
	}
	capabilityRow.CreatedBy = createdBy

	if err := upsertEnvironmentCapability(db, &capabilityRow); err != nil {
		return model.EnvironmentCapability{}, err
	}
	if err := db.Preload("RefService").First(&capabilityRow, capabilityRow.ID).Error; err != nil {
		return model.EnvironmentCapability{}, err
	}
	return capabilityRow, nil
}

func DeleteEnvironmentCapability(ctx context.Context, db *gorm.DB, envID uint, identifier string) (model.EnvironmentCapability, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return model.EnvironmentCapability{}, err
	}
	row, err := findEnvironmentCapability(db, env.ID, identifier)
	if err != nil {
		return model.EnvironmentCapability{}, err
	}
	deleteGeneratedExternalCapabilitySecret(ctx, env, row)
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		if err := RemoveCapabilityFromCanvasState(tx, env.ID, row.ID); err != nil {
			return err
		}
		return removeCapabilityReferencesFromComponents(tx, env.ID, row)
	}); err != nil {
		return model.EnvironmentCapability{}, err
	}
	return row, nil
}

func ValidateEnvironmentCapability(ctx context.Context, db *gorm.DB, envID uint, identifier string) (model.EnvironmentCapability, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return model.EnvironmentCapability{}, err
	}
	row, err := findEnvironmentCapability(db, env.ID, identifier)
	if err != nil {
		return model.EnvironmentCapability{}, err
	}
	if row.Source != model.CapabilitySourceExternal {
		return model.EnvironmentCapability{}, ErrCapabilityValidationUnsupported
	}

	status, message := validateExternalEnvironmentCapability(ctx, env, row)
	if err := db.Model(&model.EnvironmentCapability{}).
		Where("id = ?", row.ID).
		Updates(map[string]interface{}{
			"validation_status":  status,
			"validation_message": message,
		}).Error; err != nil {
		return model.EnvironmentCapability{}, err
	}
	row.ValidationStatus = status
	row.ValidationMessage = message
	return row, nil
}

func GetEnvironmentCapabilityCredentials(ctx context.Context, db *gorm.DB, envID uint, identifier string) ([]ServiceCredential, error) {
	env, err := findEnvironment(db, envID)
	if err != nil {
		return nil, err
	}
	row, err := findEnvironmentCapability(db, env.ID, identifier)
	if err != nil {
		return nil, err
	}

	switch row.Source {
	case model.CapabilitySourceShared:
		if row.RefServiceID == nil || *row.RefServiceID == 0 {
			return []ServiceCredential{}, nil
		}
		var svc model.ServiceInstallation
		if err := db.First(&svc, *row.RefServiceID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrSharedServiceNotFound
			}
			return nil, err
		}
		return DiscoverServiceCredentials(ctx, svc.Namespace)
	case model.CapabilitySourceExternal:
		if strings.TrimSpace(row.CredentialSecretRef) == "" {
			return []ServiceCredential{}, nil
		}
		return discoverExternalCapabilityCredentials(ctx, env, row)
	default:
		return nil, ErrCapabilityCredentialsUnsupported
	}
}

func ListSharedCapabilityResources(db *gorm.DB) ([]SharedCapabilityResource, error) {
	_, sharedEnv, err := LoadSystemSharedEnvironment(db)
	if err != nil {
		if errors.Is(err, ErrSharedResourcePoolNotFound) {
			return []SharedCapabilityResource{}, nil
		}
		return nil, err
	}

	var services []model.ServiceInstallation
	if err := db.Where("environment_id = ?", sharedEnv.ID).Order("service_type, id").Find(&services).Error; err != nil {
		return nil, err
	}
	resources := make([]SharedCapabilityResource, 0, len(services))
	for _, svc := range services {
		resources = append(resources, SharedCapabilityResource{
			ID:          svc.ID,
			Capability:  CapabilityForServiceType(svc.ServiceType),
			Provider:    ProviderForServiceType(svc.ServiceType),
			ServiceType: svc.ServiceType,
			ServiceName: svc.ServiceName,
			Status:      svc.Status,
			Namespace:   svc.Namespace,
		})
	}
	sort.SliceStable(resources, func(i, j int) bool {
		if resources[i].Capability == resources[j].Capability {
			return resources[i].ID < resources[j].ID
		}
		return resources[i].Capability < resources[j].Capability
	})
	return resources, nil
}

func CreateInitialEnvironmentCapabilities(ctx context.Context, db *gorm.DB, env model.Environment, requests []EnvironmentCapabilityRequest, createdBy uint) error {
	for _, req := range requests {
		capability, ok := NormalizeCapability(req.Capability)
		if !ok {
			return fmt.Errorf("invalid capability %q", req.Capability)
		}
		source, ok := NormalizeCapabilitySource(req.Source)
		if !ok {
			return fmt.Errorf("invalid capability source for %s", capability)
		}
		row, err := buildEnvironmentCapability(ctx, db, env, capability, source, req)
		if err != nil {
			return err
		}
		row.CreatedBy = createdBy
		if err := db.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func EnvironmentIsSystemSharedPool(db *gorm.DB, env model.Environment) bool {
	if !env.IsSystem || env.Identifier != SystemSharedEnvironmentIdentifier {
		return false
	}
	var app model.Application
	if err := db.First(&app, env.ApplicationID).Error; err != nil {
		return false
	}
	return app.IsSystem && app.Identifier == SystemSharedApplicationIdentifier
}

func upsertEnvironmentCapability(db *gorm.DB, capabilityRow *model.EnvironmentCapability) error {
	return db.Clauses(
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "environment_id"},
				{Name: "capability_key"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"capability":               capabilityRow.Capability,
				"source":                   capabilityRow.Source,
				"provider":                 capabilityRow.Provider,
				"service_type":             capabilityRow.ServiceType,
				"ref_service_id":           capabilityRow.RefServiceID,
				"external_endpoint":        capabilityRow.ExternalEndpoint,
				"credential_secret_ref":    capabilityRow.CredentialSecretRef,
				"tls_insecure_skip_verify": capabilityRow.TLSInsecureSkipVerify,
				"validation_status":        capabilityRow.ValidationStatus,
				"validation_message":       capabilityRow.ValidationMessage,
				"deleted_at":               nil,
				"updated_at":               gorm.Expr("NOW()"),
			}),
		},
		clause.Returning{},
	).Create(capabilityRow).Error
}

func buildEnvironmentCapability(ctx context.Context, db *gorm.DB, env model.Environment, capability, source string, req EnvironmentCapabilityRequest) (model.EnvironmentCapability, error) {
	row := model.EnvironmentCapability{
		EnvironmentID: env.ID,
		Capability:    capability,
		CapabilityKey: normalizedRequestedCapabilityKey(req.CapabilityKey, capability),
		Source:        source,
		Provider:      strings.TrimSpace(req.Provider),
		ServiceType:   strings.TrimSpace(req.ServiceType),
	}
	if row.Provider == "" && row.ServiceType != "" {
		row.Provider = ProviderForServiceType(row.ServiceType)
	}

	switch source {
	case model.CapabilitySourceManaged:
		return buildManagedCapability(db, env, row, req)
	case model.CapabilitySourceShared:
		return buildSharedCapability(db, row, req)
	case model.CapabilitySourceExternal:
		return buildExternalCapability(ctx, env, row, req)
	case model.CapabilitySourceDeferred:
		row.ValidationStatus = "pending"
		row.ValidationMessage = "capability is configured later"
		row.CapabilityKey = capabilityKeyForRow(row)
		return row, nil
	default:
		return model.EnvironmentCapability{}, fmt.Errorf("invalid capability source")
	}
}

func buildManagedCapability(db *gorm.DB, env model.Environment, row model.EnvironmentCapability, req EnvironmentCapabilityRequest) (model.EnvironmentCapability, error) {
	if req.RefServiceID != nil {
		var svc model.ServiceInstallation
		if err := db.First(&svc, *req.RefServiceID).Error; err != nil {
			return model.EnvironmentCapability{}, ErrManagedServiceNotFound
		}
		if svc.EnvironmentID != env.ID {
			return model.EnvironmentCapability{}, fmt.Errorf("managed service must belong to current environment")
		}
		row.RefServiceID = &svc.ID
		row.ServiceType = svc.ServiceType
		if row.Provider == "" {
			row.Provider = ProviderForServiceType(svc.ServiceType)
		}
	}
	row.ValidationStatus = "pending"
	if row.RefServiceID != nil {
		row.ValidationStatus = "linked"
	}
	row.CapabilityKey = capabilityKeyForRow(row)
	return row, nil
}

func buildSharedCapability(db *gorm.DB, row model.EnvironmentCapability, req EnvironmentCapabilityRequest) (model.EnvironmentCapability, error) {
	if req.RefServiceID == nil || *req.RefServiceID == 0 {
		return model.EnvironmentCapability{}, fmt.Errorf("shared capability requires refServiceId")
	}
	var svc model.ServiceInstallation
	if err := db.First(&svc, *req.RefServiceID).Error; err != nil {
		return model.EnvironmentCapability{}, ErrSharedServiceNotFound
	}
	var env model.Environment
	if err := db.First(&env, svc.EnvironmentID).Error; err != nil {
		return model.EnvironmentCapability{}, fmt.Errorf("shared service environment not found")
	}
	if !EnvironmentIsSystemSharedPool(db, env) {
		return model.EnvironmentCapability{}, fmt.Errorf("shared service must belong to the system shared environment")
	}
	row.RefServiceID = &svc.ID
	row.ServiceType = svc.ServiceType
	if row.Provider == "" {
		row.Provider = ProviderForServiceType(svc.ServiceType)
	}
	row.ValidationStatus = "linked"
	row.CapabilityKey = capabilityKeyForRow(row)
	return row, nil
}

func buildExternalCapability(ctx context.Context, env model.Environment, row model.EnvironmentCapability, req EnvironmentCapabilityRequest) (model.EnvironmentCapability, error) {
	row.ExternalEndpoint = strings.TrimSpace(req.ExternalEndpoint)
	row.CredentialSecretRef = strings.TrimSpace(req.CredentialSecretRef)
	row.TLSInsecureSkipVerify = req.TLSInsecureSkipVerify
	row.ValidationStatus = "pending"
	row.CapabilityKey = capabilityKeyForRow(row)
	if row.ExternalEndpoint == "" {
		row.ValidationMessage = "external endpoint is not configured"
	} else {
		row.ValidationMessage = "external connection has not been validated"
	}
	if externalCredentialPayload(req) {
		secretRef, err := upsertExternalCapabilityCredentialSecret(ctx, env, row.CapabilityKey, row, req)
		if err != nil {
			return model.EnvironmentCapability{}, err
		}
		row.CredentialSecretRef = secretRef
	}
	return row, nil
}

func findEnvironmentCapability(db *gorm.DB, environmentID uint, identifier string) (model.EnvironmentCapability, error) {
	clean := strings.TrimSpace(identifier)
	if clean == "" {
		return model.EnvironmentCapability{}, ErrEnvironmentCapabilityNotFound
	}

	var row model.EnvironmentCapability
	err := db.
		Where("environment_id = ? AND (capability_key = ? OR (capability = ? AND capability_key = capability))", environmentID, clean, clean).
		First(&row).Error
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.EnvironmentCapability{}, err
	}

	capability, ok := NormalizeCapability(clean)
	if !ok {
		return model.EnvironmentCapability{}, ErrEnvironmentCapabilityNotFound
	}
	var matches []model.EnvironmentCapability
	if findErr := db.
		Where("environment_id = ? AND capability = ?", environmentID, capability).
		Limit(2).
		Find(&matches).Error; findErr != nil {
		return model.EnvironmentCapability{}, findErr
	}
	if len(matches) != 1 {
		return model.EnvironmentCapability{}, ErrEnvironmentCapabilityNotFound
	}
	return matches[0], nil
}

func capabilityFromRouteOrExisting(db *gorm.DB, environmentID uint, identifier string) (string, bool) {
	if capability, ok := NormalizeCapability(identifier); ok {
		return capability, true
	}
	existing, err := findEnvironmentCapability(db, environmentID, strings.TrimSpace(identifier))
	if err != nil || existing.Capability == "" {
		return "", false
	}
	return existing.Capability, true
}

func validateExternalEnvironmentCapability(ctx context.Context, env model.Environment, row model.EnvironmentCapability) (string, string) {
	endpoint, err := parseExternalCapabilityEndpoint(row)
	if err != nil {
		return "failed", err.Error()
	}
	secretData, err := loadExternalCapabilitySecretData(ctx, env, row)
	if err != nil {
		return "failed", err.Error()
	}

	validateCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if err := validateExternalCapabilityByProtocol(validateCtx, endpoint, row, secretData); err != nil {
		return "failed", err.Error()
	}
	return "valid", externalCapabilityValidationSuccessMessage(row)
}

func loadExternalCapabilitySecretData(ctx context.Context, env model.Environment, row model.EnvironmentCapability) (map[string]string, error) {
	if strings.TrimSpace(row.CredentialSecretRef) == "" {
		return map[string]string{}, nil
	}
	namespace, name, err := parseEnvironmentCapabilitySecretRef(env, row.CredentialSecretRef)
	if err != nil {
		return nil, err
	}
	cl := k8s.GetClient()
	if cl == nil {
		return nil, ErrKubernetesClientNotInitialized
	}
	secret := &corev1.Secret{}
	if err := cl.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: name}, secret); apierrors.IsNotFound(err) {
		return nil, ErrCredentialSecretNotFound
	} else if err != nil {
		return nil, fmt.Errorf("load credential secret: %w", err)
	}
	data := make(map[string]string, len(secret.Data))
	for key, value := range secret.Data {
		data[strings.TrimSpace(key)] = string(value)
	}
	return data, nil
}

func discoverExternalCapabilityCredentials(ctx context.Context, env model.Environment, row model.EnvironmentCapability) ([]ServiceCredential, error) {
	namespace, name, err := parseEnvironmentCapabilitySecretRef(env, row.CredentialSecretRef)
	if err != nil {
		return nil, err
	}
	cl := k8s.GetClient()
	if cl == nil {
		return nil, ErrKubernetesClientNotInitialized
	}
	secret := &corev1.Secret{}
	if err := cl.Get(ctx, ctrlclient.ObjectKey{Namespace: namespace, Name: name}, secret); apierrors.IsNotFound(err) {
		return nil, ErrCredentialSecretNotFound
	} else if err != nil {
		return nil, err
	}

	credentials := make([]ServiceCredential, 0, len(secret.Data))
	for key, raw := range secret.Data {
		if len(raw) == 0 {
			continue
		}
		credentials = append(credentials, ServiceCredential{
			Secret: secret.Name,
			Key:    key,
			Value:  string(raw),
			Kind:   environmentCapabilityCredentialKind(key),
		})
	}
	sort.Slice(credentials, func(i, j int) bool {
		return credentials[i].Key < credentials[j].Key
	})
	return credentials, nil
}

func deleteGeneratedExternalCapabilitySecret(ctx context.Context, env model.Environment, row model.EnvironmentCapability) {
	if row.Source != model.CapabilitySourceExternal || strings.TrimSpace(row.CredentialSecretRef) == "" {
		return
	}
	namespace, name, err := parseEnvironmentCapabilitySecretRef(env, row.CredentialSecretRef)
	if err != nil {
		return
	}
	if !strings.HasPrefix(name, "paap-external-") || !strings.HasSuffix(name, "-credentials") {
		return
	}
	cl := k8s.GetClient()
	if cl == nil {
		return
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
	_ = cl.Delete(ctx, secret)
}

func upsertExternalCapabilityCredentialSecret(ctx context.Context, env model.Environment, capability string, row model.EnvironmentCapability, req EnvironmentCapabilityRequest) (string, error) {
	namespace := strings.TrimSpace(env.Namespace)
	if namespace == "" {
		return "", fmt.Errorf("environment namespace is required for external credentials")
	}
	cl := k8s.GetClient()
	if cl == nil {
		return "", ErrKubernetesClientNotInitialized
	}

	name := normalizeIdentifier("paap-external-"+capability+"-credentials", "external-credentials", 63)
	data := map[string][]byte{
		"endpoint":    []byte(strings.TrimSpace(req.ExternalEndpoint)),
		"provider":    []byte(strings.TrimSpace(row.Provider)),
		"serviceType": []byte(strings.TrimSpace(row.ServiceType)),
		"authType":    []byte(strings.TrimSpace(req.AuthType)),
	}
	if username := strings.TrimSpace(req.Username); username != "" {
		data["username"] = []byte(username)
	}
	if password := strings.TrimSpace(req.Password); password != "" {
		data["password"] = []byte(password)
	}
	if token := strings.TrimSpace(req.Token); token != "" {
		data["token"] = []byte(token)
	}

	secret := &corev1.Secret{}
	key := ctrlclient.ObjectKey{Namespace: namespace, Name: name}
	if err := cl.Get(ctx, key, secret); apierrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"paap.io/managed-by":  "paap",
					"paap.io/capability":  capability,
					"paap.io/credential":  "external-resource",
					"paap.io/environment": env.Identifier,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		if err := cl.Create(ctx, secret); err != nil {
			return "", fmt.Errorf("create external credential secret: %w", err)
		}
		return namespace + "/" + name, nil
	} else if err != nil {
		return "", fmt.Errorf("load external credential secret: %w", err)
	}

	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels["paap.io/managed-by"] = "paap"
	secret.Labels["paap.io/capability"] = capability
	secret.Labels["paap.io/credential"] = "external-resource"
	secret.Labels["paap.io/environment"] = env.Identifier
	secret.Type = corev1.SecretTypeOpaque
	secret.Data = data
	if err := cl.Update(ctx, secret); err != nil {
		return "", fmt.Errorf("update external credential secret: %w", err)
	}
	return namespace + "/" + name, nil
}

func parseEnvironmentCapabilitySecretRef(env model.Environment, ref string) (string, string, error) {
	clean := strings.TrimSpace(ref)
	parts := strings.Split(clean, "/")
	namespace := strings.TrimSpace(env.Namespace)
	name := clean
	if len(parts) == 2 {
		namespace = strings.TrimSpace(parts[0])
		name = strings.TrimSpace(parts[1])
	}
	if namespace == "" || name == "" || len(parts) > 2 {
		return "", "", ErrEnvironmentCapabilitySecretRefInvalid
	}
	if namespace != strings.TrimSpace(env.Namespace) {
		return "", "", ErrCredentialSecretForbidden
	}
	return namespace, name, nil
}

type externalCapabilityEndpoint struct {
	Scheme  string
	Host    string
	Port    string
	Address string
	BaseURL string
}

func externalCapabilityDialAddress(row model.EnvironmentCapability) (string, error) {
	endpoint, err := parseExternalCapabilityEndpoint(row)
	if err != nil {
		return "", err
	}
	return endpoint.Address, nil
}

func parseExternalCapabilityEndpoint(row model.EnvironmentCapability) (externalCapabilityEndpoint, error) {
	endpoint := strings.TrimSpace(row.ExternalEndpoint)
	if endpoint == "" {
		return externalCapabilityEndpoint{}, fmt.Errorf("external endpoint is not configured")
	}

	scheme := ""
	host := ""
	port := ""
	if parsed, err := url.Parse(endpoint); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		scheme = strings.ToLower(parsed.Scheme)
		host = parsed.Hostname()
		port = parsed.Port()
	} else if hostPart, portPart, err := net.SplitHostPort(endpoint); err == nil {
		host = strings.Trim(hostPart, "[]")
		port = portPart
	} else if parsed, err := url.Parse("//" + endpoint); err == nil && parsed.Host != "" {
		host = parsed.Hostname()
		port = parsed.Port()
	}
	if strings.TrimSpace(host) == "" {
		return externalCapabilityEndpoint{}, fmt.Errorf("external endpoint host is invalid")
	}
	if strings.TrimSpace(port) == "" {
		port = defaultExternalCapabilityPort(scheme, row)
	}
	if strings.TrimSpace(port) == "" {
		return externalCapabilityEndpoint{}, fmt.Errorf("external endpoint port is required")
	}
	address := net.JoinHostPort(host, port)
	baseURL := ""
	if scheme == "http" || scheme == "https" {
		baseURL = scheme + "://" + address
	}
	return externalCapabilityEndpoint{Scheme: scheme, Host: host, Port: port, Address: address, BaseURL: baseURL}, nil
}

func validateExternalCapabilityByProtocol(ctx context.Context, endpoint externalCapabilityEndpoint, row model.EnvironmentCapability, secretData map[string]string) error {
	key := externalCapabilityValidationKey(row)
	switch key {
	case "postgresql", "mysql":
		return validateExternalSQL(ctx, endpoint, row, secretData)
	case "mongodb":
		return validateExternalMongoDB(ctx, endpoint, secretData)
	case "redis":
		return validateExternalRedis(ctx, endpoint, secretData)
	case "rabbitmq":
		return validateExternalRabbitMQ(ctx, endpoint, secretData)
	case "kafka":
		return validateExternalKafka(ctx, endpoint, secretData)
	case "minio", "s3", "object-storage", "objectstorage":
		return validateExternalMinIO(ctx, endpoint, row, secretData)
	case "prometheus", "monitor":
		return validateExternalHTTP(ctx, endpoint, row, secretData, []string{"/-/ready", "/api/v1/status/runtimeinfo", "/"})
	case "loki", "log", "logging":
		return validateExternalHTTP(ctx, endpoint, row, secretData, []string{"/ready", "/loki/api/v1/status/buildinfo", "/"})
	case "gitea", "git", "gitlab":
		return validateExternalHTTP(ctx, endpoint, row, secretData, []string{"/api/v1/version", "/api/v4/version", "/"})
	case "harbor", "registry":
		return validateExternalHTTP(ctx, endpoint, row, secretData, []string{"/v2/", "/"})
	case "jenkins", "ci":
		return validateExternalHTTP(ctx, endpoint, row, secretData, []string{"/login", "/"})
	case "argocd", "deploy", "cd":
		return validateExternalHTTP(ctx, endpoint, row, secretData, []string{"/api/version", "/"})
	default:
		return validateExternalTCP(ctx, endpoint)
	}
}

func validateExternalSQL(ctx context.Context, endpoint externalCapabilityEndpoint, row model.EnvironmentCapability, secretData map[string]string) error {
	port, _ := strconv.Atoi(endpoint.Port)
	driver := "mysql"
	defaultUser := "root"
	defaultDatabase := ""
	if externalCapabilityValidationKey(row) == "postgresql" {
		driver = "pgx"
		defaultUser = "postgres"
		defaultDatabase = "postgres"
	}
	db, err := openDatabase(k8s.DatabaseConnectionInfo{
		Driver:   driver,
		Host:     endpoint.Host,
		Port:     port,
		Username: firstNonEmpty(secretValue(secretData, "username", "user"), defaultUser),
		Password: secretValue(secretData, "password", "token"),
		Database: firstNonEmpty(secretValue(secretData, "database", "db"), defaultDatabase),
	})
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("%s credential check failed: %w", driver, err)
	}
	return nil
}

func validateExternalMongoDB(ctx context.Context, endpoint externalCapabilityEndpoint, secretData map[string]string) error {
	port, _ := strconv.Atoi(endpoint.Port)
	client, err := openMongoDB(ctx, k8s.MongoDBConnectionInfo{
		Host:     endpoint.Host,
		Port:     port,
		Username: firstNonEmpty(secretValue(secretData, "username", "user"), "root"),
		Password: secretValue(secretData, "password", "token"),
		Database: firstNonEmpty(secretValue(secretData, "database", "authDatabase", "authSource"), "admin"),
	})
	if err != nil {
		return fmt.Errorf("mongodb credential check failed: %w", err)
	}
	defer client.Disconnect(ctx)
	return nil
}

func validateExternalRedis(ctx context.Context, endpoint externalCapabilityEndpoint, secretData map[string]string) error {
	port, _ := strconv.Atoi(endpoint.Port)
	conn, reader, err := redisConnect(ctx, k8s.RedisConnectionInfo{
		Host:     endpoint.Host,
		Port:     port,
		Password: secretValue(secretData, "password", "token", "redis-password"),
	})
	if err != nil {
		return fmt.Errorf("redis credential check failed: %w", err)
	}
	defer conn.Close()
	if _, err := redisCommand(conn, reader, "PING"); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}

func validateExternalRabbitMQ(ctx context.Context, endpoint externalCapabilityEndpoint, secretData map[string]string) error {
	if endpoint.BaseURL == "" {
		return validateExternalTCP(ctx, endpoint)
	}
	if err := rabbitMQJSON(ctx, k8s.RabbitMQConnectionInfo{
		ManagementURL: endpoint.BaseURL,
		Username:      firstNonEmpty(secretValue(secretData, "username", "user"), "guest"),
		Password:      firstNonEmpty(secretValue(secretData, "password", "token"), "guest"),
	}, http.MethodGet, "/api/overview", nil, nil); err != nil {
		return fmt.Errorf("rabbitmq management credential check failed: %w", err)
	}
	return nil
}

func validateExternalKafka(ctx context.Context, endpoint externalCapabilityEndpoint, secretData map[string]string) error {
	_, err := ListKafkaTopics(ctx, k8s.KafkaConnectionInfo{
		Broker:        endpoint.Address,
		Username:      secretValue(secretData, "username", "user"),
		Password:      secretValue(secretData, "password", "token"),
		SASLMechanism: secretValue(secretData, "saslMechanism", "sasl-mechanism"),
	})
	if err != nil {
		return fmt.Errorf("kafka metadata check failed: %w", err)
	}
	return nil
}

func validateExternalMinIO(ctx context.Context, endpoint externalCapabilityEndpoint, row model.EnvironmentCapability, secretData map[string]string) error {
	useSSL := endpoint.Scheme == "https" || (endpoint.Scheme == "" && strings.EqualFold(row.Provider, "s3"))
	_, err := ListMinIOBuckets(ctx, k8s.MinIOConnectionInfo{
		Endpoint:  endpoint.Address,
		AccessKey: firstNonEmpty(secretValue(secretData, "accessKey", "access-key", "username"), secretValue(secretData, "user")),
		SecretKey: secretValue(secretData, "secretKey", "secret-key", "password", "token"),
		UseSSL:    useSSL,
	})
	if err != nil {
		return fmt.Errorf("object storage credential check failed: %w", err)
	}
	return nil
}

func validateExternalHTTP(ctx context.Context, endpoint externalCapabilityEndpoint, row model.EnvironmentCapability, secretData map[string]string, paths []string) error {
	baseURL := endpoint.BaseURL
	if baseURL == "" {
		scheme := endpoint.Scheme
		if scheme == "" {
			scheme = "https"
			if endpoint.Port == "80" || endpoint.Port == "3000" || endpoint.Port == "8080" || endpoint.Port == "9090" || endpoint.Port == "3100" {
				scheme = "http"
			}
		}
		baseURL = scheme + "://" + endpoint.Address
	}
	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: row.TLSInsecureSkipVerify,
		}},
	}
	var lastErr error
	for _, path := range paths {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+path, nil)
		if err != nil {
			return err
		}
		applyExternalHTTPAuth(req, secretData)
		res, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
		_ = res.Body.Close()
		if res.StatusCode >= 200 && res.StatusCode < 400 {
			return nil
		}
		if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {
			return fmt.Errorf("http credential check failed: status %d", res.StatusCode)
		}
		lastErr = fmt.Errorf("http health check returned status %d for %s", res.StatusCode, path)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no HTTP validation path configured")
}

func validateExternalTCP(ctx context.Context, endpoint externalCapabilityEndpoint) error {
	conn, err := (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, "tcp", endpoint.Address)
	if err != nil {
		return fmt.Errorf("connect %s failed: %w", endpoint.Address, err)
	}
	_ = conn.Close()
	return nil
}

func applyExternalHTTPAuth(req *http.Request, secretData map[string]string) {
	token := secretValue(secretData, "token", "access-token", "authorization")
	if token != "" {
		if strings.HasPrefix(strings.ToLower(token), "bearer ") || strings.HasPrefix(strings.ToLower(token), "basic ") {
			req.Header.Set("Authorization", token)
		} else {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		return
	}
	username := secretValue(secretData, "username", "user")
	password := secretValue(secretData, "password")
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}
}

func externalCapabilityValidationKey(row model.EnvironmentCapability) string {
	for _, value := range []string{row.Provider, row.ServiceType, row.Capability} {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "postgres", "postgresql":
			return "postgresql"
		case "mysql":
			return "mysql"
		case "mongodb", "mongo":
			return "mongodb"
		case "redis", "cache":
			return "redis"
		case "rabbitmq", "amqp":
			return "rabbitmq"
		case "kafka":
			return "kafka"
		case "minio", "s3", "object-storage", "objectstorage":
			return normalized
		case "prometheus", "monitor":
			return "prometheus"
		case "loki", "log", "logging":
			return normalized
		case "gitea", "git", "gitlab":
			return normalized
		case "harbor", "registry":
			return normalized
		case "jenkins", "ci":
			return normalized
		case "argocd", "deploy", "cd":
			return normalized
		}
	}
	return ""
}

func externalCapabilityValidationSuccessMessage(row model.EnvironmentCapability) string {
	switch externalCapabilityValidationKey(row) {
	case "postgresql", "mysql", "mongodb":
		return "endpoint is reachable and database credentials are valid"
	case "redis":
		return "endpoint is reachable and redis ping succeeded"
	case "rabbitmq":
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(row.ExternalEndpoint)), "http") {
			return "endpoint is reachable and rabbitmq management credentials are valid"
		}
		return "endpoint is reachable; use an HTTP management endpoint to validate rabbitmq credentials"
	case "kafka":
		return "endpoint is reachable and kafka metadata is readable"
	case "minio", "s3", "object-storage", "objectstorage":
		return "endpoint is reachable and object storage credentials can list buckets"
	case "prometheus", "monitor", "loki", "log", "logging", "gitea", "git", "gitlab", "harbor", "registry", "jenkins", "ci", "argocd", "deploy", "cd":
		return "endpoint is reachable and HTTP credentials are accepted"
	default:
		return "endpoint is reachable"
	}
}

func secretValue(data map[string]string, keys ...string) string {
	for _, key := range keys {
		for actual, value := range data {
			if strings.EqualFold(strings.TrimSpace(actual), strings.TrimSpace(key)) && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func removeCapabilityReferencesFromComponents(db *gorm.DB, envID uint, capability model.EnvironmentCapability) error {
	var components []model.Component
	if err := db.Where("environment_id = ?", envID).Find(&components).Error; err != nil {
		return err
	}
	for _, component := range components {
		cfg, err := model.ParseComponentConfig(component.Config)
		if err != nil {
			return fmt.Errorf("parse component %d config: %w", component.ID, err)
		}
		filtered := filterCapabilityBindings(cfg.Bindings, capability)
		if len(filtered) == len(cfg.Bindings) {
			continue
		}
		cfg.Bindings = filtered
		nextConfig, err := cfg.JSON()
		if err != nil {
			return fmt.Errorf("marshal component %d config: %w", component.ID, err)
		}
		if err := db.Model(&model.Component{}).
			Where("id = ?", component.ID).
			Update("config", nextConfig).Error; err != nil {
			return err
		}
	}
	return nil
}

func filterCapabilityBindings(bindings []model.ComponentBinding, capability model.EnvironmentCapability) []model.ComponentBinding {
	targetKeys := map[string]struct{}{}
	for _, key := range capabilityReferenceKeys(capability) {
		targetKeys[key] = struct{}{}
	}
	filtered := make([]model.ComponentBinding, 0, len(bindings))
	for _, binding := range bindings {
		if _, ok := targetKeys[strings.TrimSpace(binding.TargetKey)]; ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(binding.TargetKind), "capability") {
			if _, ok := targetKeys["capability:"+strings.TrimSpace(binding.TargetName)]; ok {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(binding.TargetName), capability.CapabilityKey) ||
				strings.EqualFold(strings.TrimSpace(binding.TargetName), capability.Capability) {
				continue
			}
		}
		filtered = append(filtered, binding)
	}
	return filtered
}

func capabilityReferenceKeys(capability model.EnvironmentCapability) []string {
	keys := []string{
		"capability:" + strconv.FormatUint(uint64(capability.ID), 10),
	}
	if clean := strings.TrimSpace(capability.CapabilityKey); clean != "" {
		keys = append(keys, clean, "capability:"+clean)
	}
	if clean := strings.TrimSpace(capability.Capability); clean != "" {
		keys = append(keys, clean, "capability:"+clean)
	}
	return keys
}

func defaultExternalCapabilityPort(scheme string, row model.EnvironmentCapability) string {
	if port := defaultPortForKey(scheme); port != "" {
		return port
	}
	for _, value := range []string{row.Provider, row.ServiceType, row.Capability} {
		if port := defaultPortForKey(value); port != "" {
			return port
		}
	}
	return ""
}

func defaultPortForKey(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "http":
		return "80"
	case "https":
		return "443"
	case "postgres", "postgresql":
		return "5432"
	case "mysql":
		return "3306"
	case "mongodb", "mongo":
		return "27017"
	case "redis", "cache":
		return "6379"
	case "amqp", "rabbitmq":
		return "5672"
	case "kafka":
		return "9092"
	case "minio":
		return "9000"
	case "loki":
		return "3100"
	case "prometheus", "monitor":
		return "9090"
	case "jenkins", "ci":
		return "8080"
	case "argocd", "deploy", "cd":
		return "443"
	case "gitea", "git":
		return "3000"
	case "harbor", "registry", "s3", "object-storage", "objectstorage":
		return "443"
	default:
		return ""
	}
}

func externalCredentialPayload(req EnvironmentCapabilityRequest) bool {
	return strings.TrimSpace(req.Username) != "" ||
		strings.TrimSpace(req.Password) != "" ||
		strings.TrimSpace(req.Token) != ""
}

func normalizedRequestedCapabilityKey(value, capability string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return normalizeIdentifier(value, capability, 100)
}

func capabilityKeyForRow(row model.EnvironmentCapability) string {
	if strings.TrimSpace(row.CapabilityKey) != "" {
		return normalizeIdentifier(row.CapabilityKey, row.Capability, 100)
	}
	parts := []string{row.Capability, row.Source}
	switch row.Source {
	case model.CapabilitySourceShared, model.CapabilitySourceManaged:
		if row.RefServiceID != nil && *row.RefServiceID != 0 {
			parts = append(parts, strconv.FormatUint(uint64(*row.RefServiceID), 10))
		}
	case model.CapabilitySourceExternal:
		parts = append(parts, firstNonEmpty(row.Provider, row.ServiceType, row.ExternalEndpoint, "draft"))
	case model.CapabilitySourceDeferred:
		parts = append(parts, firstNonEmpty(row.Provider, row.ServiceType, "pending"))
	}
	return normalizeIdentifier(strings.Join(parts, "-"), row.Capability, 100)
}

func environmentCapabilityCredentialKind(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch normalized {
	case "endpoint":
		return "endpoint"
	case "authtype", "auth-type":
		return "authType"
	case "token", "access-token":
		return "token"
	case "provider":
		return "provider"
	case "servicetype", "service-type":
		return "serviceType"
	}
	if kind, ok := CredentialKeyKind(key); ok {
		return kind
	}
	return "metadata"
}

func NormalizeCapability(value string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(value))
	key = strings.ReplaceAll(key, "_", "-")
	switch key {
	case "git", "code-repository", "repository":
		return "git", true
	case "registry", "image-registry", "harbor":
		return "registry", true
	case "ci", "continuous-integration":
		return "ci", true
	case "cd", "deploy", "continuous-deployment":
		return "cd", true
	case "monitor", "monitoring", "monitoring-center":
		return "monitor", true
	case "log", "logging", "logging-center":
		return "logging", true
	case "database", "databases", "db":
		return "database", true
	case "cache":
		return "cache", true
	case "mq", "message-queue":
		return "mq", true
	case "objectstorage", "object-storage":
		return "objectStorage", true
	case "custom":
		return "custom", true
	}
	return "", false
}

func NormalizeCapabilitySource(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "managed", "self", "local":
		return model.CapabilitySourceManaged, true
	case "shared":
		return model.CapabilitySourceShared, true
	case "external":
		return model.CapabilitySourceExternal, true
	case "deferred", "later":
		return model.CapabilitySourceDeferred, true
	default:
		return "", false
	}
}

func CapabilityForServiceType(serviceType string) string {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "git":
		return "git"
	case "registry", "harbor":
		return "registry"
	case "ci":
		return "ci"
	case "deploy":
		return "cd"
	case "monitor":
		return "monitor"
	case "log":
		return "logging"
	case "postgresql", "mysql", "mongodb":
		return "database"
	case "redis":
		return "cache"
	case "rabbitmq", "kafka":
		return "mq"
	case "minio":
		return "objectStorage"
	case "custom":
		return "custom"
	default:
		return "platform-tools"
	}
}

func ProviderForServiceType(serviceType string) string {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "deploy":
		return "argocd"
	case "ci":
		return "jenkins"
	case "git":
		return "gitea"
	case "log":
		return "loki"
	case "monitor":
		return "prometheus"
	default:
		return strings.TrimSpace(serviceType)
	}
}
