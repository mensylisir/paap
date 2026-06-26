package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	paapv1 "paap/api/v1"
	"paap/internal/model"

	"gorm.io/gorm"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const legacyClusterSyncOwnerID uint = 1

const (
	systemSharedApplicationIdentifier = "default"
	systemSharedEnvironmentIdentifier = "shared"
)

// SyncClusterState restores the server database from PAAP custom resources.
// The Kubernetes CRs are the durable control-plane state; the DB is the UI/API index.
func SyncClusterState(ctx context.Context, db *gorm.DB, k8sClient client.Client) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if k8sClient == nil {
		return fmt.Errorf("k8s client is nil")
	}

	ownerID, err := clusterSyncOwnerID(db)
	if err != nil {
		return err
	}

	appsByIdentifier, err := syncApplications(ctx, db, k8sClient, ownerID)
	if err != nil {
		return err
	}

	envsByKey, err := syncEnvironments(ctx, db, k8sClient, appsByIdentifier, ownerID)
	if err != nil {
		return err
	}

	serviceKeys, err := syncServiceInstances(ctx, db, k8sClient, appsByIdentifier, envsByKey, ownerID)
	if err != nil {
		return err
	}

	componentKeys, err := syncComponents(ctx, db, k8sClient, appsByIdentifier, envsByKey, ownerID)
	if err != nil {
		return err
	}

	if err := reconcileEnvironmentStatuses(db, envsByKey, serviceKeys, componentKeys); err != nil {
		return err
	}

	return pruneMissingClusterState(db, appsByIdentifier, envsByKey, serviceKeys, componentKeys)
}

func clusterSyncOwnerID(db *gorm.DB) (uint, error) {
	if !db.Migrator().HasTable(&model.User{}) {
		return legacyClusterSyncOwnerID, nil
	}

	var user model.User
	if err := db.Where("username = ?", "admin").First(&user).Error; err == nil {
		return user.ID, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("find platform admin user: %w", err)
	}

	if err := db.
		Joins("JOIN user_roles ON user_roles.user_id = users.id AND user_roles.role = ? AND user_roles.deleted_at IS NULL", model.RolePlatformAdmin).
		Where("users.deleted_at IS NULL").
		Order("users.id").
		First(&user).Error; err == nil {
		return user.ID, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("find platform admin role user: %w", err)
	}

	return legacyClusterSyncOwnerID, nil
}

func syncApplications(ctx context.Context, db *gorm.DB, k8sClient client.Client, ownerID uint) (map[string]model.Application, error) {
	var appList paapv1.ApplicationList
	if err := k8sClient.List(ctx, &appList); err != nil {
		return nil, fmt.Errorf("list application CRs: %w", err)
	}

	appsByIdentifier := make(map[string]model.Application, len(appList.Items))
	for _, appCR := range appList.Items {
		if clusterSyncObjectIsDeleting(&appCR) {
			continue
		}
		identifier := firstNonEmpty(appCR.Spec.Identifier, appCR.Name)
		if identifier == "" {
			continue
		}

		app := model.Application{
			Name:        firstNonEmpty(appCR.Spec.Name, identifier),
			Identifier:  identifier,
			Description: appCR.Spec.Description,
			OwnerID:     ownerID,
			IsSystem:    identifier == systemSharedApplicationIdentifier,
		}
		if err := upsertApplication(db, &app); err != nil {
			return nil, err
		}
		if err := ensureOwnerMember(db, app.ID, ownerID); err != nil {
			return nil, err
		}
		appsByIdentifier[identifier] = app
	}

	return appsByIdentifier, nil
}

func syncEnvironments(ctx context.Context, db *gorm.DB, k8sClient client.Client, appsByIdentifier map[string]model.Application, ownerID uint) (map[string]model.Environment, error) {
	var envList paapv1.EnvironmentList
	if err := k8sClient.List(ctx, &envList); err != nil {
		return nil, fmt.Errorf("list environment CRs: %w", err)
	}

	envsByKey := make(map[string]model.Environment, len(envList.Items))
	for _, envCR := range envList.Items {
		if clusterSyncObjectIsDeleting(&envCR) {
			continue
		}
		appIdentifier := firstNonEmpty(envCR.Labels["paap.io/app"], strings.TrimPrefix(envCR.Namespace, "paap-app-"))
		envIdentifier := firstNonEmpty(envCR.Spec.Identifier, envCR.Labels["paap.io/env"], envCR.Name)
		if appIdentifier == "" || envIdentifier == "" {
			continue
		}

		app, err := ensureApplication(db, appsByIdentifier, appIdentifier, ownerID)
		if err != nil {
			return nil, err
		}

		env := model.Environment{
			ApplicationID: app.ID,
			Name:          firstNonEmpty(envCR.Spec.Name, envIdentifier),
			Identifier:    envIdentifier,
			Status:        normalizePhase(envCR.Status.Phase, "empty"),
			Namespace:     firstNonEmpty(envCR.Spec.PrimaryNamespace, envIdentifier),
			IsSystem:      appIdentifier == systemSharedApplicationIdentifier && envIdentifier == systemSharedEnvironmentIdentifier,
		}
		if env.Status != "error" {
			env.ErrorMessage = ""
		}
		if err := upsertEnvironment(db, &env); err != nil {
			return nil, err
		}
		envsByKey[envKey(appIdentifier, envIdentifier)] = env
	}

	return envsByKey, nil
}

func syncServiceInstances(ctx context.Context, db *gorm.DB, k8sClient client.Client, appsByIdentifier map[string]model.Application, envsByKey map[string]model.Environment, ownerID uint) (map[string]struct{}, error) {
	var svcList paapv1.ServiceInstanceList
	if err := k8sClient.List(ctx, &svcList); err != nil {
		return nil, fmt.Errorf("list serviceinstance CRs: %w", err)
	}

	serviceKeys := make(map[string]struct{}, len(svcList.Items))
	for _, svcCR := range svcList.Items {
		if clusterSyncObjectIsDeleting(&svcCR) {
			continue
		}
		appIdentifier := firstNonEmpty(svcCR.Labels["paap.io/app"], strings.TrimPrefix(svcCR.Namespace, "paap-app-"))
		envIdentifier := firstNonEmpty(svcCR.Labels["paap.io/env"], svcCR.Spec.EnvironmentRef.Name)
		serviceType := firstNonEmpty(svcCR.Spec.Type, svcCR.Labels["paap.io/tool"])
		if appIdentifier == "" || envIdentifier == "" || serviceType == "" {
			continue
		}
		if !isCurrentServiceInstanceCR(svcCR, envIdentifier, serviceType) {
			continue
		}
		if serviceType == "docker-registry" {
			if err := k8sClient.Delete(ctx, &svcCR); err != nil && !apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("delete obsolete docker-registry serviceinstance %s/%s: %w", svcCR.Namespace, svcCR.Name, err)
			}
			continue
		}

		if _, err := ensureApplication(db, appsByIdentifier, appIdentifier, ownerID); err != nil {
			return nil, err
		}
		env, err := ensureEnvironment(db, envsByKey, appIdentifier, envIdentifier)
		if err != nil {
			return nil, err
		}

		values, err := json.Marshal(serviceInstanceStoredValues(svcCR))
		if err != nil {
			return nil, fmt.Errorf("marshal service parameters %s/%s: %w", svcCR.Namespace, svcCR.Name, err)
		}

		releaseName := svcCR.Name
		namespace := svcCR.Spec.ToolNamespace
		if svcCR.Spec.Helm != nil {
			releaseName = firstNonEmpty(svcCR.Spec.Helm.ReleaseName, releaseName)
			namespace = firstNonEmpty(namespace, svcCR.Spec.Helm.Namespace)
		}

		install := model.ServiceInstallation{
			EnvironmentID: env.ID,
			ServiceType:   serviceType,
			ServiceName:   svcCR.Name,
			ReleaseName:   releaseName,
			Status:        normalizePhase(svcCR.Status.Phase, "installing"),
			ErrorMessage:  serviceInstanceErrorMessage(svcCR),
			Values:        string(values),
			Namespace:     namespace,
		}
		if err := upsertServiceInstallation(db, &install); err != nil {
			return nil, err
		}
		serviceKeys[serviceKey(install.EnvironmentID, install.ServiceType)] = struct{}{}
	}

	return serviceKeys, nil
}

func isCurrentServiceInstanceCR(svcCR paapv1.ServiceInstance, envIdentifier, serviceType string) bool {
	identity := strings.TrimSpace(svcCR.Labels["paap.io/tool"])
	if identity == "" || strings.EqualFold(identity, serviceType) {
		if svcCR.Spec.Helm != nil {
			identity = firstNonEmpty(
				serviceIdentityFromRuntimeName(envIdentifier, svcCR.Spec.Helm.ReleaseName),
				serviceIdentityFromRuntimeName(envIdentifier, svcCR.Spec.Helm.Namespace),
				identity,
			)
		}
	}
	if identity == "" {
		identity = serviceType
	}
	return svcCR.Name == dnsLabelPart(envIdentifier+"-"+identity)
}

func serviceIdentityFromRuntimeName(envIdentifier, runtimeName string) string {
	runtimeName = dnsLabelPart(runtimeName)
	envIdentifier = dnsLabelPart(envIdentifier)
	if runtimeName == "" || envIdentifier == "" {
		return ""
	}
	suffix := "-" + envIdentifier + "-"
	if idx := strings.Index(runtimeName, suffix); idx >= 0 {
		return strings.Trim(runtimeName[idx+len(suffix):], "-")
	}
	if strings.HasPrefix(runtimeName, envIdentifier+"-") {
		return strings.TrimPrefix(runtimeName, envIdentifier+"-")
	}
	return ""
}

func dnsLabelPart(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	b.Grow(len(value))
	lastDash := false
	for _, r := range value {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func serviceInstanceStoredValues(svcCR paapv1.ServiceInstance) map[string]string {
	if svcCR.Spec.Helm != nil && len(svcCR.Spec.Helm.Values) > 0 {
		return svcCR.Spec.Helm.Values
	}
	return svcCR.Spec.Parameters
}

func syncComponents(ctx context.Context, db *gorm.DB, k8sClient client.Client, appsByIdentifier map[string]model.Application, envsByKey map[string]model.Environment, ownerID uint) (map[string]struct{}, error) {
	var compList paapv1.ComponentList
	if err := k8sClient.List(ctx, &compList); err != nil {
		return nil, fmt.Errorf("list component CRs: %w", err)
	}

	componentKeys := make(map[string]struct{}, len(compList.Items))
	for _, compCR := range compList.Items {
		if clusterSyncObjectIsDeleting(&compCR) {
			continue
		}
		appIdentifier := firstNonEmpty(compCR.Labels["paap.io/app"], strings.TrimPrefix(compCR.Namespace, "paap-app-"))
		envIdentifier := firstNonEmpty(compCR.Labels["paap.io/env"], compCR.Spec.EnvironmentRef.Name)
		componentIdentifier := firstNonEmpty(compCR.Spec.Identifier, compCR.Labels["paap.io/component"], compCR.Name)
		if appIdentifier == "" || envIdentifier == "" || componentIdentifier == "" {
			continue
		}

		if _, err := ensureApplication(db, appsByIdentifier, appIdentifier, ownerID); err != nil {
			return nil, err
		}
		env, err := ensureEnvironment(db, envsByKey, appIdentifier, envIdentifier)
		if err != nil {
			return nil, err
		}

		replicas := int(compCR.Spec.Deployment.Replicas)
		if replicas <= 0 {
			replicas = 1
		}

		comp := model.Component{
			EnvironmentID: env.ID,
			Name:          firstNonEmpty(compCR.Spec.Name, componentIdentifier),
			Type:          firstNonEmpty(compCR.Spec.Type, "custom"),
			Image:         compCR.Spec.Deployment.Image,
			Version:       compCR.Spec.Deployment.Tag,
			Replicas:      replicas,
			Status:        normalizePhase(compCR.Status.Phase, "creating"),
			GitPath:       fmt.Sprintf("components/%s", componentIdentifier),
		}
		if cfgJSON, err := model.ComponentConfigFromEnvVars(compCR.Spec.Deployment.Env).JSON(); err == nil {
			comp.Config = cfgJSON
		}
		if compCR.Spec.ArgoCDAppRef != nil {
			comp.ArgoCDApp = compCR.Spec.ArgoCDAppRef.Name
		}
		if err := upsertComponent(db, &comp); err != nil {
			return nil, err
		}
		componentKeys[componentKey(comp.EnvironmentID, componentIdentifier)] = struct{}{}
	}

	return componentKeys, nil
}

func clusterSyncObjectIsDeleting(obj metav1.Object) bool {
	if obj == nil {
		return false
	}
	deletionTimestamp := obj.GetDeletionTimestamp()
	return deletionTimestamp != nil && !deletionTimestamp.IsZero()
}

func ensureApplication(db *gorm.DB, appsByIdentifier map[string]model.Application, identifier string, ownerID uint) (model.Application, error) {
	if app, ok := appsByIdentifier[identifier]; ok {
		return app, nil
	}
	app := model.Application{
		Name:       identifier,
		Identifier: identifier,
		OwnerID:    ownerID,
		IsSystem:   identifier == systemSharedApplicationIdentifier,
	}
	if err := upsertApplication(db, &app); err != nil {
		return model.Application{}, err
	}
	if err := ensureOwnerMember(db, app.ID, ownerID); err != nil {
		return model.Application{}, err
	}
	appsByIdentifier[identifier] = app
	return app, nil
}

func ensureEnvironment(db *gorm.DB, envsByKey map[string]model.Environment, appIdentifier, envIdentifier string) (model.Environment, error) {
	key := envKey(appIdentifier, envIdentifier)
	if env, ok := envsByKey[key]; ok {
		return env, nil
	}

	var app model.Application
	if err := db.Where("identifier = ?", appIdentifier).First(&app).Error; err != nil {
		return model.Environment{}, fmt.Errorf("find app %s for environment %s: %w", appIdentifier, envIdentifier, err)
	}

	env := model.Environment{
		ApplicationID: app.ID,
		Name:          envIdentifier,
		Identifier:    envIdentifier,
		Status:        "empty",
		Namespace:     fmt.Sprintf("%s-%s", appIdentifier, envIdentifier),
		IsSystem:      appIdentifier == systemSharedApplicationIdentifier && envIdentifier == systemSharedEnvironmentIdentifier,
	}
	if err := upsertEnvironment(db, &env); err != nil {
		return model.Environment{}, err
	}
	envsByKey[key] = env
	return env, nil
}

func upsertApplication(db *gorm.DB, app *model.Application) error {
	var existing model.Application
	err := db.Unscoped().Where("identifier = ?", app.Identifier).First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(app).Error
	}
	if err != nil {
		return fmt.Errorf("find application %s: %w", app.Identifier, err)
	}
	existing.Name = app.Name
	existing.Description = app.Description
	existing.OwnerID = app.OwnerID
	existing.IsSystem = app.IsSystem
	existing.DeletedAt = gorm.DeletedAt{}
	if err := db.Unscoped().Save(&existing).Error; err != nil {
		return fmt.Errorf("update application %s: %w", app.Identifier, err)
	}
	*app = existing
	return nil
}

func upsertEnvironment(db *gorm.DB, env *model.Environment) error {
	var existing model.Environment
	err := db.Unscoped().
		Where("application_id = ? AND identifier = ?", env.ApplicationID, env.Identifier).
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(env).Error
	}
	if err != nil {
		return fmt.Errorf("find environment %d/%s: %w", env.ApplicationID, env.Identifier, err)
	}
	existing.Name = env.Name
	if strings.EqualFold(existing.Status, "empty") &&
		existing.TemplateID == 0 &&
		strings.EqualFold(env.Status, "running") &&
		strings.TrimSpace(existing.ErrorMessage) == "" {
		env.Status = "empty"
	} else {
		existing.Status = env.Status
	}
	existing.Namespace = env.Namespace
	existing.ErrorMessage = env.ErrorMessage
	existing.IsSystem = env.IsSystem
	existing.DeletedAt = gorm.DeletedAt{}
	if err := db.Unscoped().Save(&existing).Error; err != nil {
		return fmt.Errorf("update environment %d/%s: %w", env.ApplicationID, env.Identifier, err)
	}
	*env = existing
	return nil
}

func upsertServiceInstallation(db *gorm.DB, install *model.ServiceInstallation) error {
	var existing model.ServiceInstallation
	err := db.Unscoped().
		Where("environment_id = ? AND service_type = ?", install.EnvironmentID, install.ServiceType).
		Order("CASE WHEN deleted_at IS NULL THEN 0 ELSE 1 END, id").
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(install).Error
	}
	if err != nil {
		return fmt.Errorf("find service installation %d/%s: %w", install.EnvironmentID, install.ServiceType, err)
	}
	existing.ServiceName = install.ServiceName
	existing.ReleaseName = install.ReleaseName
	existing.Status = install.Status
	existing.ErrorMessage = install.ErrorMessage
	existing.Values = install.Values
	existing.Namespace = install.Namespace
	existing.DeletedAt = gorm.DeletedAt{}
	if err := db.Unscoped().Save(&existing).Error; err != nil {
		return fmt.Errorf("update service installation %d/%s: %w", install.EnvironmentID, install.ServiceType, err)
	}
	if err := db.Unscoped().
		Where("environment_id = ? AND service_type = ? AND id <> ?", install.EnvironmentID, install.ServiceType, existing.ID).
		Delete(&model.ServiceInstallation{}).Error; err != nil {
		return fmt.Errorf("deduplicate service installation %d/%s: %w", install.EnvironmentID, install.ServiceType, err)
	}
	*install = existing
	return nil
}

func upsertComponent(db *gorm.DB, comp *model.Component) error {
	var existing model.Component
	err := db.Unscoped().
		Where("environment_id = ? AND name = ?", comp.EnvironmentID, comp.Name).
		Order("CASE WHEN deleted_at IS NULL THEN 0 ELSE 1 END, id").
		First(&existing).Error
	if err == gorm.ErrRecordNotFound {
		return db.Create(comp).Error
	}
	if err != nil {
		return fmt.Errorf("find component %d/%s: %w", comp.EnvironmentID, comp.Name, err)
	}
	existing.Type = comp.Type
	if syncShouldAdoptRuntimeImage(existing) {
		existing.Image = comp.Image
		existing.RegistryImage = comp.Image
		existing.Version = comp.Version
	} else {
		if strings.TrimSpace(existing.Image) == "" {
			existing.Image = comp.Image
		}
		if strings.TrimSpace(existing.Version) == "" {
			existing.Version = comp.Version
		}
	}
	existing.Replicas = comp.Replicas
	existing.Status = comp.Status
	existing.Config = mergeComponentRuntimeConfig(existing.Config, comp.Config)
	existing.GitPath = firstNonEmpty(existing.GitPath, comp.GitPath)
	existing.ArgoCDApp = firstNonEmpty(existing.ArgoCDApp, comp.ArgoCDApp)
	existing.DeletedAt = gorm.DeletedAt{}
	if err := db.Unscoped().Save(&existing).Error; err != nil {
		return fmt.Errorf("update component %d/%s: %w", comp.EnvironmentID, comp.Name, err)
	}
	if err := db.Unscoped().
		Where("environment_id = ? AND name = ? AND id <> ?", comp.EnvironmentID, comp.Name, existing.ID).
		Delete(&model.Component{}).Error; err != nil {
		return fmt.Errorf("deduplicate component %d/%s: %w", comp.EnvironmentID, comp.Name, err)
	}
	*comp = existing
	return nil
}

func syncShouldAdoptRuntimeImage(existing model.Component) bool {
	return strings.EqualFold(strings.TrimSpace(existing.DeliveryMode), "source") || strings.TrimSpace(existing.JenkinsJob) != ""
}

func mergeComponentRuntimeConfig(existingRaw, runtimeRaw string) string {
	if strings.TrimSpace(existingRaw) == "" {
		return runtimeRaw
	}
	if strings.TrimSpace(runtimeRaw) == "" {
		return existingRaw
	}
	existing, err := model.ParseComponentConfig(existingRaw)
	if err != nil {
		return runtimeRaw
	}
	runtime, err := model.ParseComponentConfig(runtimeRaw)
	if err != nil {
		return existingRaw
	}
	if len(existing.Env) == 0 && !componentConfigHasManagedSurface(existing) {
		existing.Env = runtime.Env
	}
	merged, err := existing.JSON()
	if err != nil {
		return existingRaw
	}
	return merged
}

func componentConfigHasManagedSurface(cfg model.ComponentConfig) bool {
	return strings.TrimSpace(cfg.Framework) != "" ||
		len(cfg.Command) > 0 ||
		len(cfg.Args) > 0 ||
		len(cfg.ConfigMaps) > 0 ||
		len(cfg.Secrets) > 0 ||
		len(cfg.Files) > 0 ||
		len(cfg.Bindings) > 0 ||
		len(cfg.Dependencies) > 0
}

func reconcileEnvironmentStatuses(db *gorm.DB, envsByKey map[string]model.Environment, serviceKeys map[string]struct{}, componentKeys map[string]struct{}) error {
	for _, env := range envsByKey {
		if strings.EqualFold(env.Status, "error") || strings.EqualFold(env.Status, "creating") {
			continue
		}
		if environmentHasSyncedResources(env.ID, serviceKeys, componentKeys) {
			if err := db.Model(&model.Environment{}).
				Where("id = ?", env.ID).
				Update("status", "running").Error; err != nil {
				return fmt.Errorf("mark populated environment %d running: %w", env.ID, err)
			}
			continue
		}
		if err := db.Model(&model.Environment{}).
			Where("id = ?", env.ID).
			Updates(map[string]any{"status": "empty", "template_id": 0}).Error; err != nil {
			return fmt.Errorf("mark empty environment %d: %w", env.ID, err)
		}
	}
	return nil
}

func environmentHasSyncedResources(environmentID uint, serviceKeys map[string]struct{}, componentKeys map[string]struct{}) bool {
	prefix := fmt.Sprintf("%d/", environmentID)
	for key := range serviceKeys {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	for key := range componentKeys {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func ensureOwnerMember(db *gorm.DB, appID uint, ownerID uint) error {
	member := model.AppMember{
		ApplicationID: appID,
		UserID:        ownerID,
		Role:          "admin",
	}
	return db.Where("application_id = ? AND user_id = ?", appID, ownerID).FirstOrCreate(&member).Error
}

func pruneMissingClusterState(db *gorm.DB, appsByIdentifier map[string]model.Application, envsByKey map[string]model.Environment, serviceKeys map[string]struct{}, componentKeys map[string]struct{}) error {
	var installs []model.ServiceInstallation
	if err := db.Find(&installs).Error; err != nil {
		return fmt.Errorf("list service installations for prune: %w", err)
	}
	for _, install := range installs {
		if serviceInstallationCanExistWithoutCR(install.Status) {
			continue
		}
		if _, ok := serviceKeys[serviceKey(install.EnvironmentID, install.ServiceType)]; ok {
			continue
		}
		if err := db.Unscoped().Delete(&install).Error; err != nil {
			return fmt.Errorf("prune service installation %d: %w", install.ID, err)
		}
	}

	var components []model.Component
	if err := db.Find(&components).Error; err != nil {
		return fmt.Errorf("list components for prune: %w", err)
	}
	for _, comp := range components {
		if strings.EqualFold(strings.TrimSpace(comp.Status), "draft") {
			continue
		}
		if _, ok := componentKeys[componentKey(comp.EnvironmentID, componentIdentifierFromModel(comp))]; ok {
			continue
		}
		if err := db.Unscoped().Delete(&comp).Error; err != nil {
			return fmt.Errorf("prune component %d: %w", comp.ID, err)
		}
	}

	appIDToIdentifier := make(map[uint]string, len(appsByIdentifier))
	for identifier, app := range appsByIdentifier {
		appIDToIdentifier[app.ID] = identifier
	}

	var envs []model.Environment
	if err := db.Find(&envs).Error; err != nil {
		return fmt.Errorf("list environments for prune: %w", err)
	}
	for _, env := range envs {
		appIdentifier := appIDToIdentifier[env.ApplicationID]
		if appIdentifier == "" {
			if err := db.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.ServiceInstallation{}).Error; err != nil {
				return fmt.Errorf("prune services for environment %d: %w", env.ID, err)
			}
			if err := db.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.Component{}).Error; err != nil {
				return fmt.Errorf("prune components for environment %d: %w", env.ID, err)
			}
			if err := db.Unscoped().Delete(&env).Error; err != nil {
				return fmt.Errorf("prune environment %d: %w", env.ID, err)
			}
			continue
		}
		if _, ok := envsByKey[envKey(appIdentifier, env.Identifier)]; ok {
			continue
		}
		if err := db.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.ServiceInstallation{}).Error; err != nil {
			return fmt.Errorf("prune services for environment %d: %w", env.ID, err)
		}
		if err := db.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.Component{}).Error; err != nil {
			return fmt.Errorf("prune components for environment %d: %w", env.ID, err)
		}
		if err := db.Unscoped().Delete(&env).Error; err != nil {
			return fmt.Errorf("prune environment %d: %w", env.ID, err)
		}
	}

	var apps []model.Application
	if err := db.Find(&apps).Error; err != nil {
		return fmt.Errorf("list applications for prune: %w", err)
	}
	for _, app := range apps {
		if _, ok := appsByIdentifier[app.Identifier]; ok {
			continue
		}
		if err := db.Unscoped().Where("application_id = ?", app.ID).Delete(&model.Environment{}).Error; err != nil {
			return fmt.Errorf("prune environments for application %d: %w", app.ID, err)
		}
		if err := db.Unscoped().Where("application_id = ?", app.ID).Delete(&model.AppMember{}).Error; err != nil {
			return fmt.Errorf("prune app members for application %d: %w", app.ID, err)
		}
		if err := db.Unscoped().Delete(&app).Error; err != nil {
			return fmt.Errorf("prune application %d: %w", app.ID, err)
		}
	}

	return nil
}

func serviceInstallationCanExistWithoutCR(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "draft", "pending", "failed", "error":
		return true
	default:
		return false
	}
}

func normalizePhase(phase, fallback string) string {
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase == "" {
		return fallback
	}
	return phase
}

func serviceInstanceErrorMessage(svcCR paapv1.ServiceInstance) string {
	for _, cond := range svcCR.Status.Conditions {
		if strings.EqualFold(cond.Type, "Ready") && cond.Status == metav1.ConditionFalse {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func envKey(appIdentifier, envIdentifier string) string {
	return appIdentifier + "/" + envIdentifier
}

func serviceKey(environmentID uint, serviceType string) string {
	return fmt.Sprintf("%d/%s", environmentID, serviceType)
}

func componentKey(environmentID uint, identifier string) string {
	return fmt.Sprintf("%d/%s", environmentID, identifier)
}

func componentIdentifierFromModel(comp model.Component) string {
	gitPath := strings.Trim(comp.GitPath, "/")
	if gitPath != "" {
		parts := strings.Split(gitPath, "/")
		if last := parts[len(parts)-1]; last != "" {
			return last
		}
	}
	return ComponentIdentifier(comp.Name, comp.Type, comp.ID)
}
