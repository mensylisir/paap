package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	systemSharedApplicationIdentifier = "default"
	systemSharedEnvironmentIdentifier = "shared"
	systemSharedApplicationName       = "共享资源池"
	systemSharedEnvironmentName       = "共享环境"
)

type EnvironmentCapabilityRequest struct {
	Capability            string `json:"capability"`
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

func loadSystemSharedEnvironment() (model.Application, model.Environment, error) {
	var app model.Application
	if err := database.DB.
		Where("identifier = ? AND is_system = ?", systemSharedApplicationIdentifier, true).
		First(&app).Error; err != nil {
		return model.Application{}, model.Environment{}, err
	}
	var env model.Environment
	if err := database.DB.
		Where("application_id = ? AND identifier = ? AND is_system = ?", app.ID, systemSharedEnvironmentIdentifier, true).
		First(&env).Error; err != nil {
		return model.Application{}, model.Environment{}, err
	}
	return app, env, nil
}

func ListEnvironmentCapabilities(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}
	env, _, ok := loadEnvironmentAndApp(c, envID)
	if !ok {
		return
	}

	var capabilities []model.EnvironmentCapability
	if err := database.DB.Where("environment_id = ?", env.ID).
		Preload("RefService").
		Order("capability").
		Find(&capabilities).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": capabilities})
}

func GetSharedResourcePool(c *gin.Context) {
	app, env, err := loadSystemSharedEnvironment()
	if err != nil {
		if errorsIsRecordNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "shared resource pool not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"application": app,
		"environment": env,
	}})
}

func UpsertEnvironmentCapability(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}
	env, app, ok := loadEnvironmentAndApp(c, envID)
	if !ok {
		return
	}
	if rejectSystemSharedEnvironmentMutation(c, env, "system shared environments cannot add shared or external resource cards") {
		return
	}
	if !requireApplicationAdminAccess(c, app.ID) {
		return
	}

	capability, ok := normalizeCapability(c.Param("capability"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid capability"})
		return
	}

	var req EnvironmentCapabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	source, ok := normalizeCapabilitySource(req.Source)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid capability source"})
		return
	}

	capabilityRow, err := buildEnvironmentCapability(c.Request.Context(), env, capability, source, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if userID, ok := authenticatedUserID(c); ok {
		capabilityRow.CreatedBy = userID
	}

	if err := upsertEnvironmentCapability(&capabilityRow); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := database.DB.Preload("RefService").First(&capabilityRow, capabilityRow.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": capabilityRow})
}

func upsertEnvironmentCapability(capabilityRow *model.EnvironmentCapability) error {
	return database.DB.Clauses(
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "environment_id"},
				{Name: "capability"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
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

func DeleteEnvironmentCapability(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}
	env, app, ok := loadEnvironmentAndApp(c, envID)
	if !ok {
		return
	}
	if !requireApplicationAdminAccess(c, app.ID) {
		return
	}
	capability, ok := normalizeCapability(c.Param("capability"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid capability"})
		return
	}

	var row model.EnvironmentCapability
	err := database.DB.Where("environment_id = ? AND capability = ?", env.ID, capability).First(&row).Error
	if errorsIsRecordNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "capability not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	deleteGeneratedExternalCapabilitySecret(c.Request.Context(), env, row)
	if err := database.DB.Delete(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true, "capability": capability}})
}

func GetEnvironmentCapabilityCredentials(c *gin.Context) {
	envID, ok := parseEnvironmentID(c)
	if !ok {
		return
	}
	env, _, ok := loadEnvironmentAndApp(c, envID)
	if !ok {
		return
	}
	capability, ok := normalizeCapability(c.Param("capability"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid capability"})
		return
	}

	var row model.EnvironmentCapability
	err := database.DB.Where("environment_id = ? AND capability = ?", env.ID, capability).First(&row).Error
	if errorsIsRecordNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "capability not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if row.Source == model.CapabilitySourceShared {
		var svc model.ServiceInstallation
		if row.RefServiceID == nil || *row.RefServiceID == 0 {
			c.JSON(http.StatusOK, gin.H{"data": gin.H{"credentials": []ServiceCredential{}}})
			return
		}
		if err := database.DB.First(&svc, *row.RefServiceID).Error; errorsIsRecordNotFound(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "shared service not found"})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		credentials, err := discoverServiceCredentials(c.Request.Context(), svc.Namespace)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"credentials": credentials}})
		return
	}

	if row.Source != model.CapabilitySourceExternal {
		c.JSON(http.StatusBadRequest, gin.H{"error": "capability is not external or shared"})
		return
	}
	if strings.TrimSpace(row.CredentialSecretRef) == "" {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"credentials": []ServiceCredential{}}})
		return
	}
	namespace, name, err := parseEnvironmentCapabilitySecretRef(env, row.CredentialSecretRef)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	cl := k8s.GetClient()
	if cl == nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": "kubernetes client is not initialized"})
		return
	}
	secret := &corev1.Secret{}
	if err := cl.Get(c.Request.Context(), ctrlclient.ObjectKey{Namespace: namespace, Name: name}, secret); apierrors.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "credential secret not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
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
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"credentials": credentials}})
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

func ListSharedCapabilityResources(c *gin.Context) {
	_, sharedEnv, err := loadSystemSharedEnvironment()
	if errorsIsRecordNotFound(err) {
		c.JSON(http.StatusOK, gin.H{"data": []SharedCapabilityResource{}})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var services []model.ServiceInstallation
	if err := database.DB.Where("environment_id = ?", sharedEnv.ID).Order("service_type, id").Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resources := make([]SharedCapabilityResource, 0, len(services))
	for _, svc := range services {
		resources = append(resources, SharedCapabilityResource{
			ID:          svc.ID,
			Capability:  capabilityForServiceType(svc.ServiceType),
			Provider:    providerForServiceType(svc.ServiceType),
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
	c.JSON(http.StatusOK, gin.H{"data": resources})
}

func buildEnvironmentCapability(ctx context.Context, env model.Environment, capability, source string, req EnvironmentCapabilityRequest) (model.EnvironmentCapability, error) {
	row := model.EnvironmentCapability{
		EnvironmentID: env.ID,
		Capability:    capability,
		Source:        source,
		Provider:      strings.TrimSpace(req.Provider),
		ServiceType:   strings.TrimSpace(req.ServiceType),
	}
	if row.Provider == "" && row.ServiceType != "" {
		row.Provider = providerForServiceType(row.ServiceType)
	}

	switch source {
	case model.CapabilitySourceManaged:
		return buildManagedCapability(env, row, req)
	case model.CapabilitySourceShared:
		return buildSharedCapability(row, req)
	case model.CapabilitySourceExternal:
		return buildExternalCapability(ctx, env, row, req)
	case model.CapabilitySourceDeferred:
		row.ValidationStatus = "pending"
		row.ValidationMessage = "capability is configured later"
		return row, nil
	default:
		return model.EnvironmentCapability{}, fmt.Errorf("invalid capability source")
	}
}

func CreateInitialEnvironmentCapabilities(env model.Environment, requests []EnvironmentCapabilityRequest, createdBy uint) error {
	for _, req := range requests {
		capability, ok := normalizeCapability(req.Capability)
		if !ok {
			return fmt.Errorf("invalid capability %q", req.Capability)
		}
		source, ok := normalizeCapabilitySource(req.Source)
		if !ok {
			return fmt.Errorf("invalid capability source for %s", capability)
		}
		row, err := buildEnvironmentCapability(context.Background(), env, capability, source, req)
		if err != nil {
			return err
		}
		row.CreatedBy = createdBy
		if err := database.DB.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func buildManagedCapability(env model.Environment, row model.EnvironmentCapability, req EnvironmentCapabilityRequest) (model.EnvironmentCapability, error) {
	if req.RefServiceID != nil {
		var svc model.ServiceInstallation
		if err := database.DB.First(&svc, *req.RefServiceID).Error; err != nil {
			return model.EnvironmentCapability{}, fmt.Errorf("managed service not found")
		}
		if svc.EnvironmentID != env.ID {
			return model.EnvironmentCapability{}, fmt.Errorf("managed service must belong to current environment")
		}
		row.RefServiceID = &svc.ID
		row.ServiceType = svc.ServiceType
		if row.Provider == "" {
			row.Provider = providerForServiceType(svc.ServiceType)
		}
	}
	row.ValidationStatus = "pending"
	if row.RefServiceID != nil {
		row.ValidationStatus = "linked"
	}
	return row, nil
}

func buildSharedCapability(row model.EnvironmentCapability, req EnvironmentCapabilityRequest) (model.EnvironmentCapability, error) {
	if req.RefServiceID == nil || *req.RefServiceID == 0 {
		return model.EnvironmentCapability{}, fmt.Errorf("shared capability requires refServiceId")
	}
	var svc model.ServiceInstallation
	if err := database.DB.First(&svc, *req.RefServiceID).Error; err != nil {
		return model.EnvironmentCapability{}, fmt.Errorf("shared service not found")
	}
	var env model.Environment
	if err := database.DB.First(&env, svc.EnvironmentID).Error; err != nil {
		return model.EnvironmentCapability{}, fmt.Errorf("shared service environment not found")
	}
	if !environmentIsSystemSharedPool(env) {
		return model.EnvironmentCapability{}, fmt.Errorf("shared service must belong to the system shared environment")
	}
	row.RefServiceID = &svc.ID
	row.ServiceType = svc.ServiceType
	if row.Provider == "" {
		row.Provider = providerForServiceType(svc.ServiceType)
	}
	row.ValidationStatus = "linked"
	return row, nil
}

func environmentIsSystemSharedPool(env model.Environment) bool {
	if !env.IsSystem || env.Identifier != systemSharedEnvironmentIdentifier {
		return false
	}
	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		return false
	}
	return app.IsSystem && app.Identifier == systemSharedApplicationIdentifier
}

func rejectSystemSharedEnvironmentMutation(c *gin.Context, env model.Environment, message string) bool {
	if !environmentIsSystemSharedPool(env) {
		return false
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": message})
	return true
}

func buildExternalCapability(ctx context.Context, env model.Environment, row model.EnvironmentCapability, req EnvironmentCapabilityRequest) (model.EnvironmentCapability, error) {
	row.ExternalEndpoint = strings.TrimSpace(req.ExternalEndpoint)
	row.CredentialSecretRef = strings.TrimSpace(req.CredentialSecretRef)
	row.TLSInsecureSkipVerify = req.TLSInsecureSkipVerify
	row.ValidationStatus = "pending"
	if row.ExternalEndpoint == "" {
		row.ValidationMessage = "external endpoint is not configured"
	} else {
		row.ValidationMessage = "external connection has not been validated"
	}
	if externalCredentialPayload(req) {
		secretRef, err := upsertExternalCapabilityCredentialSecret(ctx, env, row.Capability, row, req)
		if err != nil {
			return model.EnvironmentCapability{}, err
		}
		row.CredentialSecretRef = secretRef
	}
	return row, nil
}

func externalCredentialPayload(req EnvironmentCapabilityRequest) bool {
	return strings.TrimSpace(req.Username) != "" ||
		strings.TrimSpace(req.Password) != "" ||
		strings.TrimSpace(req.Token) != ""
}

func upsertExternalCapabilityCredentialSecret(ctx context.Context, env model.Environment, capability string, row model.EnvironmentCapability, req EnvironmentCapabilityRequest) (string, error) {
	namespace := strings.TrimSpace(env.Namespace)
	if namespace == "" {
		return "", fmt.Errorf("environment namespace is required for external credentials")
	}
	cl := k8s.GetClient()
	if cl == nil {
		return "", fmt.Errorf("kubernetes client is not initialized")
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
		return "", "", fmt.Errorf("invalid credentialSecretRef")
	}
	if namespace != strings.TrimSpace(env.Namespace) {
		return "", "", fmt.Errorf("credential secret must belong to current environment namespace")
	}
	return namespace, name, nil
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
	if kind, ok := credentialKeyKind(key); ok {
		return kind
	}
	return "metadata"
}

func parseEnvironmentID(c *gin.Context) (uint, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return 0, false
	}
	return uint(id), true
}

func normalizeCapability(value string) (string, bool) {
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

func normalizeCapabilitySource(value string) (string, bool) {
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

func capabilityForServiceType(serviceType string) string {
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

func providerForServiceType(serviceType string) string {
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

func errorsIsRecordNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}
