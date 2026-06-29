package service

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"paap/internal/model"

	"gorm.io/gorm"
)

type PlatformServiceStat struct {
	Type                string `json:"type"`
	Name                string `json:"name"`
	Category            string `json:"category"`
	Features            string `json:"features,omitempty"`
	ClusterID           string `json:"clusterId,omitempty"`
	ClusterName         string `json:"clusterName,omitempty"`
	ManagedInstances    int    `json:"managedInstances"`
	KubeVirtInstances   int    `json:"kubevirtInstances"`
	SharedReferences    int    `json:"sharedReferences"`
	ExternalConnections int    `json:"externalConnections"`
	DeferredReferences  int    `json:"deferredReferences"`
	RunningInstances    int    `json:"runningInstances"`
	ApplicationCount    int    `json:"applicationCount"`
	EnvironmentCount    int    `json:"environmentCount"`

	appIDs map[uint]struct{}
	envIDs map[uint]struct{}
}

type PlatformServiceInstance struct {
	ID                    string `json:"id"`
	ServiceType           string `json:"serviceType"`
	ServiceName           string `json:"serviceName"`
	Provider              string `json:"provider,omitempty"`
	Source                string `json:"source"`
	ProvisionMode         string `json:"provisionMode,omitempty"`
	Status                string `json:"status"`
	ApplicationID         uint   `json:"applicationId,omitempty"`
	ApplicationName       string `json:"applicationName,omitempty"`
	EnvironmentID         uint   `json:"environmentId,omitempty"`
	EnvironmentName       string `json:"environmentName,omitempty"`
	EnvironmentIdentifier string `json:"environmentIdentifier,omitempty"`
	Namespace             string `json:"namespace,omitempty"`
	Endpoint              string `json:"endpoint,omitempty"`
	MonitoringTarget      string `json:"monitoringTarget,omitempty"`
	MonitoringURL         string `json:"monitoringUrl,omitempty"`
	ClusterID             string `json:"clusterId,omitempty"`
	ClusterName           string `json:"clusterName,omitempty"`
	UsageCount            int    `json:"usageCount"`
}

type PlatformServiceUsage struct {
	ID                    string `json:"id"`
	ServiceType           string `json:"serviceType"`
	ServiceName           string `json:"serviceName"`
	Provider              string `json:"provider,omitempty"`
	Source                string `json:"source"`
	ProvisionMode         string `json:"provisionMode,omitempty"`
	Status                string `json:"status"`
	ApplicationID         uint   `json:"applicationId,omitempty"`
	ApplicationName       string `json:"applicationName,omitempty"`
	EnvironmentID         uint   `json:"environmentId,omitempty"`
	EnvironmentName       string `json:"environmentName,omitempty"`
	EnvironmentIdentifier string `json:"environmentIdentifier,omitempty"`
	ComponentID           uint   `json:"componentId,omitempty"`
	ComponentName         string `json:"componentName,omitempty"`
	ComponentType         string `json:"componentType,omitempty"`
	Capability            string `json:"capability,omitempty"`
	ServiceInstanceID     string `json:"serviceInstanceId,omitempty"`
	RefServiceID          uint   `json:"refServiceId,omitempty"`
	Endpoint              string `json:"endpoint,omitempty"`
	MonitoringTarget      string `json:"monitoringTarget,omitempty"`
	MonitoringURL         string `json:"monitoringUrl,omitempty"`
	ClusterID             string `json:"clusterId,omitempty"`
	ClusterName           string `json:"clusterName,omitempty"`
}

type CatalogServiceProduct struct {
	Type                string   `json:"type"`
	Name                string   `json:"name"`
	Category            string   `json:"category"`
	Description         string   `json:"description,omitempty"`
	Features            string   `json:"features,omitempty"`
	Versions            []string `json:"versions,omitempty"`
	ClusterID           string   `json:"clusterId,omitempty"`
	ClusterName         string   `json:"clusterName,omitempty"`
	ManagedInstances    int      `json:"managedInstances"`
	KubeVirtInstances   int      `json:"kubevirtInstances"`
	PublicInstances     int      `json:"publicInstances"`
	SharedReferences    int      `json:"sharedReferences"`
	ExternalConnections int      `json:"externalConnections"`
	DeferredReferences  int      `json:"deferredReferences"`
	RunningInstances    int      `json:"runningInstances"`
	ApplicationCount    int      `json:"applicationCount"`
	EnvironmentCount    int      `json:"environmentCount"`
}

func ListPlatformServiceStats(db *gorm.DB) ([]PlatformServiceStat, error) {
	return buildPlatformServiceStats(db)
}

func ListCatalogServiceProducts(db *gorm.DB) ([]CatalogServiceProduct, error) {
	return buildCatalogServiceProducts(db)
}

func ListPlatformServiceInstances(db *gorm.DB, serviceType string) ([]PlatformServiceInstance, error) {
	return buildPlatformServiceInstances(db, serviceType)
}

func ListPlatformServiceUsage(db *gorm.DB, serviceType string) ([]PlatformServiceUsage, error) {
	return buildPlatformServiceUsage(db, serviceType)
}

func buildPlatformServiceStats(db *gorm.DB) ([]PlatformServiceStat, error) {
	stats := map[string]*PlatformServiceStat{}
	ensure := func(serviceType, name, category, features string) *PlatformServiceStat {
		serviceType = strings.TrimSpace(serviceType)
		if serviceType == "" {
			serviceType = "unknown"
		}
		if existing, ok := stats[serviceType]; ok {
			if existing.Name == "" && name != "" {
				existing.Name = name
			}
			if existing.Category == "" && category != "" {
				existing.Category = category
			}
			if existing.Features == "" && features != "" {
				existing.Features = features
			}
			return existing
		}
		if name == "" {
			name = serviceType
		}
		row := &PlatformServiceStat{
			Type:     serviceType,
			Name:     name,
			Category: category,
			Features: features,
			appIDs:   map[uint]struct{}{},
			envIDs:   map[uint]struct{}{},
		}
		stats[serviceType] = row
		return row
	}

	var templates []model.ServiceTemplate
	if err := db.Where("enabled = ?", true).Order("install_order").Find(&templates).Error; err != nil {
		return nil, err
	}
	for _, tmpl := range templates {
		ensure(tmpl.Type, tmpl.Name, tmpl.Category, tmpl.SupportedFeatures)
	}

	var catalog []model.ServiceCatalog
	if err := db.Where("enabled = ?", true).Find(&catalog).Error; err == nil {
		for _, item := range catalog {
			ensure(item.Type, item.Name, item.Category, item.Features)
		}
	}

	var installs []model.ServiceInstallation
	if err := db.Find(&installs).Error; err != nil {
		return nil, err
	}
	envs, err := environmentsByID(db, environmentIDsFromInstallationsAndCapabilities(installs, nil))
	if err != nil {
		return nil, err
	}
	for _, inst := range installs {
		row := ensure(inst.ServiceType, inst.ServiceName, "", "")
		if NormalizeServiceProvisionMode(inst.ProvisionMode) == model.ServiceProvisionModeKubeVirt {
			row.KubeVirtInstances++
		} else {
			row.ManagedInstances++
		}
		if strings.EqualFold(inst.Status, "running") {
			row.RunningInstances++
		}
		row.addEnvironment(envs[inst.EnvironmentID])
	}

	var capabilities []model.EnvironmentCapability
	if err := db.Find(&capabilities).Error; err != nil {
		return nil, err
	}
	envs, err = environmentsByID(db, environmentIDsFromInstallationsAndCapabilities(nil, capabilities))
	if err != nil {
		return nil, err
	}
	for _, capability := range capabilities {
		serviceType := strings.TrimSpace(capability.ServiceType)
		if serviceType == "" {
			serviceType = capabilityServiceTypeFallback(capability.Capability)
		}
		row := ensure(serviceType, serviceType, "", "")
		switch capability.Source {
		case model.CapabilitySourceShared:
			row.SharedReferences++
		case model.CapabilitySourceExternal:
			row.ExternalConnections++
		case model.CapabilitySourceDeferred:
			row.DeferredReferences++
		}
		row.addEnvironment(envs[capability.EnvironmentID])
	}

	out := make([]PlatformServiceStat, 0, len(stats))
	for _, row := range stats {
		row.ApplicationCount = len(row.appIDs)
		row.EnvironmentCount = len(row.envIDs)
		row.appIDs = nil
		row.envIDs = nil
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Type < out[j].Type
	})
	return out, nil
}

func buildCatalogServiceProducts(db *gorm.DB) ([]CatalogServiceProduct, error) {
	stats, err := buildPlatformServiceStats(db)
	if err != nil {
		return nil, err
	}
	products := map[string]*CatalogServiceProduct{}
	ensure := func(serviceType string) *CatalogServiceProduct {
		serviceType = strings.TrimSpace(serviceType)
		if serviceType == "" {
			serviceType = "unknown"
		}
		if existing, ok := products[serviceType]; ok {
			return existing
		}
		row := &CatalogServiceProduct{Type: serviceType, Name: serviceType}
		products[serviceType] = row
		return row
	}

	for _, stat := range stats {
		row := ensure(stat.Type)
		row.Name = firstNonEmpty(stat.Name, row.Name, stat.Type)
		row.Category = firstNonEmpty(stat.Category, row.Category)
		row.Features = firstNonEmpty(stat.Features, row.Features)
		row.ClusterID = firstNonEmpty(stat.ClusterID, row.ClusterID)
		row.ClusterName = firstNonEmpty(stat.ClusterName, row.ClusterName)
		row.ManagedInstances = stat.ManagedInstances
		row.KubeVirtInstances = stat.KubeVirtInstances
		row.SharedReferences = stat.SharedReferences
		row.ExternalConnections = stat.ExternalConnections
		row.DeferredReferences = stat.DeferredReferences
		row.RunningInstances = stat.RunningInstances
		row.ApplicationCount = stat.ApplicationCount
		row.EnvironmentCount = stat.EnvironmentCount
	}

	var templates []model.ServiceTemplate
	if err := db.Where("enabled = ?", true).Not("type IN ?", UnsupportedServiceCatalogTypes()).Order("install_order").Find(&templates).Error; err != nil {
		return nil, err
	}
	for _, tmpl := range templates {
		row := ensure(tmpl.Type)
		row.Name = firstNonEmpty(tmpl.Name, row.Name, tmpl.Type)
		row.Category = firstNonEmpty(tmpl.Category, row.Category)
		row.Description = firstNonEmpty(tmpl.Description, row.Description)
		row.Features = firstNonEmpty(tmpl.SupportedFeatures, row.Features)
		if strings.TrimSpace(tmpl.AppVersion) != "" && !stringSliceContains(row.Versions, tmpl.AppVersion) {
			row.Versions = append(row.Versions, tmpl.AppVersion)
		}
	}

	var catalog []model.ServiceCatalog
	if err := db.Where("enabled = ?", true).Not("type IN ?", UnsupportedServiceCatalogTypes()).Find(&catalog).Error; err != nil {
		return nil, err
	}
	for _, item := range catalog {
		row := ensure(item.Type)
		row.Name = firstNonEmpty(item.Name, row.Name, item.Type)
		row.Category = firstNonEmpty(item.Category, row.Category)
		row.Description = firstNonEmpty(item.Description, row.Description)
		row.Features = firstNonEmpty(item.Features, row.Features)
	}

	var installs []model.ServiceInstallation
	if err := db.Find(&installs).Error; err != nil {
		return nil, err
	}
	envs, err := environmentsByID(db, environmentIDsFromInstallationsAndCapabilities(installs, nil))
	if err != nil {
		return nil, err
	}
	for _, inst := range installs {
		env := envs[inst.EnvironmentID]
		if env.IsSystem {
			ensure(inst.ServiceType).PublicInstances++
		}
	}

	var envTemplates []model.EnvTemplate
	if err := db.Find(&envTemplates).Error; err != nil {
		return nil, err
	}
	for _, tmpl := range envTemplates {
		key := "environment:" + strconvFormatUint(tmpl.ID)
		row := ensure(key)
		row.Name = firstNonEmpty(tmpl.Name, "环境服务")
		row.Category = "environment"
		row.Description = firstNonEmpty(tmpl.Description, "环境服务模板")
		row.Features = jsonString([]ServiceFeatureItem{
			{Key: "environment", Label: "环境服务", Enabled: true},
			{Key: "quota", Label: "资源配额", Enabled: true},
		})
	}

	out := make([]CatalogServiceProduct, 0, len(products))
	for _, row := range products {
		sort.Slice(row.Versions, func(i, j int) bool {
			return row.Versions[i] > row.Versions[j]
		})
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Type < out[j].Type
	})
	return out, nil
}

func buildPlatformServiceInstances(db *gorm.DB, serviceType string) ([]PlatformServiceInstance, error) {
	serviceType = strings.TrimSpace(serviceType)
	installations, capabilities, envs, apps, err := loadPlatformServiceInputs(db, serviceType)
	if err != nil {
		return nil, err
	}
	monitoringServices, err := monitoringServicesByEnvironment(db, environmentIDsFromInstallationsAndCapabilities(installations, capabilities))
	if err != nil {
		return nil, err
	}
	componentCounts, err := componentBindingUsageCounts(db, installations, capabilities)
	if err != nil {
		return nil, err
	}
	refCounts := sharedReferenceCounts(capabilities)
	out := make([]PlatformServiceInstance, 0, len(installations)+len(capabilities))
	for _, inst := range installations {
		env := envs[inst.EnvironmentID]
		app := apps[env.ApplicationID]
		usageCount := 1 + refCounts[inst.ID] + componentCounts[managedServiceInstanceID(inst.ID)]
		out = append(out, PlatformServiceInstance{
			ID:                    managedServiceInstanceID(inst.ID),
			ServiceType:           inst.ServiceType,
			ServiceName:           firstNonEmpty(inst.ServiceName, inst.ServiceType),
			Source:                model.CapabilitySourceManaged,
			ProvisionMode:         NormalizeServiceProvisionMode(inst.ProvisionMode),
			Status:                inst.Status,
			ApplicationID:         app.ID,
			ApplicationName:       app.Name,
			EnvironmentID:         env.ID,
			EnvironmentName:       env.Name,
			EnvironmentIdentifier: env.Identifier,
			Namespace:             inst.Namespace,
			MonitoringTarget:      monitoringTargetForNamespace(inst.Namespace),
			MonitoringURL:         monitoringURLForInstallation(monitoringServices, inst),
			UsageCount:            usageCount,
		})
	}
	for _, capability := range capabilities {
		if capability.Source == model.CapabilitySourceShared {
			continue
		}
		env := envs[capability.EnvironmentID]
		app := apps[env.ApplicationID]
		out = append(out, PlatformServiceInstance{
			ID:                    capabilityServiceInstanceID(capability.ID),
			ServiceType:           platformCapabilityServiceType(capability),
			ServiceName:           firstNonEmpty(capability.Provider, platformCapabilityServiceType(capability)),
			Provider:              capability.Provider,
			Source:                capability.Source,
			ProvisionMode:         model.CapabilitySourceExternal,
			Status:                firstNonEmpty(capability.ValidationStatus, "pending"),
			ApplicationID:         app.ID,
			ApplicationName:       app.Name,
			EnvironmentID:         env.ID,
			EnvironmentName:       env.Name,
			EnvironmentIdentifier: env.Identifier,
			Endpoint:              capability.ExternalEndpoint,
			MonitoringTarget:      monitoringTargetForCapability(capability),
			MonitoringURL:         monitoringURLForCapability(monitoringServices, capability, nil),
			UsageCount:            1 + componentCounts[capabilityServiceInstanceID(capability.ID)],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		if out[i].ApplicationName != out[j].ApplicationName {
			return out[i].ApplicationName < out[j].ApplicationName
		}
		return out[i].ServiceName < out[j].ServiceName
	})
	return out, nil
}

func buildPlatformServiceUsage(db *gorm.DB, serviceType string) ([]PlatformServiceUsage, error) {
	serviceType = strings.TrimSpace(serviceType)
	installations, capabilities, envs, apps, err := loadPlatformServiceInputs(db, serviceType)
	if err != nil {
		return nil, err
	}
	installationsByID := map[uint]model.ServiceInstallation{}
	for _, inst := range installations {
		installationsByID[inst.ID] = inst
	}
	monitoringServices, err := monitoringServicesByEnvironment(db, environmentIDsFromInstallationsAndCapabilities(installations, capabilities))
	if err != nil {
		return nil, err
	}
	out := make([]PlatformServiceUsage, 0, len(installations)+len(capabilities))
	for _, inst := range installations {
		env := envs[inst.EnvironmentID]
		app := apps[env.ApplicationID]
		out = append(out, PlatformServiceUsage{
			ID:                    "managed:" + strconvFormatUint(inst.ID),
			ServiceType:           inst.ServiceType,
			ServiceName:           firstNonEmpty(inst.ServiceName, inst.ServiceType),
			Source:                model.CapabilitySourceManaged,
			ProvisionMode:         NormalizeServiceProvisionMode(inst.ProvisionMode),
			Status:                inst.Status,
			ApplicationID:         app.ID,
			ApplicationName:       app.Name,
			EnvironmentID:         env.ID,
			EnvironmentName:       env.Name,
			EnvironmentIdentifier: env.Identifier,
			ServiceInstanceID:     managedServiceInstanceID(inst.ID),
			MonitoringTarget:      monitoringTargetForNamespace(inst.Namespace),
			MonitoringURL:         monitoringURLForInstallation(monitoringServices, inst),
		})
	}
	for _, capability := range capabilities {
		env := envs[capability.EnvironmentID]
		app := apps[env.ApplicationID]
		refServiceID := uint(0)
		instanceID := capabilityServiceInstanceID(capability.ID)
		serviceName := firstNonEmpty(capability.Provider, platformCapabilityServiceType(capability))
		if capability.RefServiceID != nil {
			refServiceID = *capability.RefServiceID
			instanceID = managedServiceInstanceID(refServiceID)
			if ref, ok := installationsByID[refServiceID]; ok {
				serviceName = firstNonEmpty(ref.ServiceName, ref.ServiceType)
			}
		}
		provisionMode := capability.Source
		if capability.RefServiceID != nil {
			if ref, ok := installationsByID[*capability.RefServiceID]; ok {
				provisionMode = NormalizeServiceProvisionMode(ref.ProvisionMode)
			}
		}
		out = append(out, PlatformServiceUsage{
			ID:                    "capability:" + strconvFormatUint(capability.ID),
			ServiceType:           platformCapabilityServiceType(capability),
			ServiceName:           serviceName,
			Provider:              capability.Provider,
			Source:                capability.Source,
			ProvisionMode:         provisionMode,
			Status:                firstNonEmpty(capability.ValidationStatus, "pending"),
			ApplicationID:         app.ID,
			ApplicationName:       app.Name,
			EnvironmentID:         env.ID,
			EnvironmentName:       env.Name,
			EnvironmentIdentifier: env.Identifier,
			Capability:            capability.Capability,
			ServiceInstanceID:     instanceID,
			RefServiceID:          refServiceID,
			Endpoint:              capability.ExternalEndpoint,
			MonitoringTarget:      monitoringTargetForCapability(capability),
			MonitoringURL:         monitoringURLForCapability(monitoringServices, capability, installationsByID),
		})
	}
	componentUsage, err := buildComponentBindingServiceUsage(db, serviceType, installations, capabilities, envs, apps, monitoringServices, installationsByID)
	if err != nil {
		return nil, err
	}
	out = append(out, componentUsage...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ApplicationName != out[j].ApplicationName {
			return out[i].ApplicationName < out[j].ApplicationName
		}
		if out[i].EnvironmentName != out[j].EnvironmentName {
			return out[i].EnvironmentName < out[j].EnvironmentName
		}
		if out[i].ComponentName != out[j].ComponentName {
			return out[i].ComponentName < out[j].ComponentName
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (s *PlatformServiceStat) addEnvironment(env model.Environment) {
	if env.ID == 0 {
		return
	}
	s.envIDs[env.ID] = struct{}{}
	if env.ApplicationID != 0 {
		s.appIDs[env.ApplicationID] = struct{}{}
	}
}

func loadPlatformServiceInputs(db *gorm.DB, serviceType string) ([]model.ServiceInstallation, []model.EnvironmentCapability, map[uint]model.Environment, map[uint]model.Application, error) {
	var installations []model.ServiceInstallation
	query := db
	if serviceType != "" {
		query = query.Where("service_type = ?", serviceType)
	}
	if err := query.Find(&installations).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	var capabilities []model.EnvironmentCapability
	if err := db.Find(&capabilities).Error; err != nil {
		return nil, nil, nil, nil, err
	}
	if serviceType != "" {
		filtered := capabilities[:0]
		for _, capability := range capabilities {
			if platformCapabilityServiceType(capability) == serviceType {
				filtered = append(filtered, capability)
				continue
			}
			if capability.RefServiceID != nil {
				for _, inst := range installations {
					if inst.ID == *capability.RefServiceID {
						filtered = append(filtered, capability)
						break
					}
				}
			}
		}
		capabilities = filtered
	}

	envs, err := environmentsByID(db, environmentIDsFromInstallationsAndCapabilities(installations, capabilities))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	apps, err := applicationsByEnvironment(db, envs)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return installations, capabilities, envs, apps, nil
}

func applicationsByEnvironment(db *gorm.DB, envs map[uint]model.Environment) (map[uint]model.Application, error) {
	ids := map[uint]struct{}{}
	for _, env := range envs {
		if env.ApplicationID != 0 {
			ids[env.ApplicationID] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return map[uint]model.Application{}, nil
	}
	appIDs := make([]uint, 0, len(ids))
	for id := range ids {
		appIDs = append(appIDs, id)
	}
	var apps []model.Application
	if err := db.Where("id IN ?", appIDs).Find(&apps).Error; err != nil {
		return nil, err
	}
	out := make(map[uint]model.Application, len(apps))
	for _, app := range apps {
		out[app.ID] = app
	}
	return out, nil
}

func sharedReferenceCounts(capabilities []model.EnvironmentCapability) map[uint]int {
	counts := map[uint]int{}
	for _, capability := range capabilities {
		if capability.Source == model.CapabilitySourceShared && capability.RefServiceID != nil {
			counts[*capability.RefServiceID]++
		}
	}
	return counts
}

func componentBindingUsageCounts(db *gorm.DB, installations []model.ServiceInstallation, capabilities []model.EnvironmentCapability) (map[string]int, error) {
	usage, err := buildComponentBindingServiceUsage(db, "", installations, capabilities, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, item := range usage {
		if item.ServiceInstanceID != "" {
			counts[item.ServiceInstanceID]++
		}
	}
	return counts, nil
}

func buildComponentBindingServiceUsage(
	db *gorm.DB,
	serviceType string,
	installations []model.ServiceInstallation,
	capabilities []model.EnvironmentCapability,
	envs map[uint]model.Environment,
	apps map[uint]model.Application,
	monitoringServices map[uint]model.ServiceInstallation,
	installationsByID map[uint]model.ServiceInstallation,
) ([]PlatformServiceUsage, error) {
	serviceType = strings.TrimSpace(serviceType)
	installationsByKey := map[string]model.ServiceInstallation{}
	installationsByIDLocal := map[uint]model.ServiceInstallation{}
	for _, inst := range installations {
		installationsByKey[managedServiceInstanceID(inst.ID)] = inst
		installationsByKey["service:"+strconvFormatUint(inst.ID)] = inst
		installationsByIDLocal[inst.ID] = inst
	}
	capabilitiesByKey := map[string]model.EnvironmentCapability{}
	for _, capability := range capabilities {
		capabilitiesByKey[capabilityServiceInstanceID(capability.ID)] = capability
		capabilitiesByKey["capability:"+strconvFormatUint(capability.ID)] = capability
	}

	envIDs := environmentIDsFromInstallationsAndCapabilities(installations, capabilities)
	if len(envIDs) == 0 {
		return nil, nil
	}
	if envs == nil {
		loaded, err := environmentsByID(db, envIDs)
		if err != nil {
			return nil, err
		}
		envs = loaded
	}
	if apps == nil {
		loaded, err := applicationsByEnvironment(db, envs)
		if err != nil {
			return nil, err
		}
		apps = loaded
	}
	if monitoringServices == nil {
		loaded, err := monitoringServicesByEnvironment(db, envIDs)
		if err != nil {
			return nil, err
		}
		monitoringServices = loaded
	}
	if installationsByID == nil {
		installationsByID = installationsByIDLocal
	}

	var components []model.Component
	if err := db.Where("environment_id IN ?", envIDs).Find(&components).Error; err != nil {
		return nil, err
	}
	out := make([]PlatformServiceUsage, 0)
	seen := map[string]struct{}{}
	for _, comp := range components {
		cfg, err := model.ParseComponentConfig(comp.Config)
		if err != nil {
			return nil, fmt.Errorf("parse component %d config: %w", comp.ID, err)
		}
		env := envs[comp.EnvironmentID]
		app := apps[env.ApplicationID]
		for idx, binding := range cfg.Bindings {
			key := strings.TrimSpace(binding.TargetKey)
			if key == "" {
				continue
			}
			if inst, ok := installationsByKey[key]; ok {
				if serviceType != "" && inst.ServiceType != serviceType {
					continue
				}
				rowID := fmt.Sprintf("component:%d:binding:%d:%s", comp.ID, idx, strings.ReplaceAll(key, ":", "-"))
				if _, exists := seen[rowID]; exists {
					continue
				}
				seen[rowID] = struct{}{}
				out = append(out, PlatformServiceUsage{
					ID:                    rowID,
					ServiceType:           inst.ServiceType,
					ServiceName:           firstNonEmpty(inst.ServiceName, inst.ServiceType),
					Source:                model.CapabilitySourceManaged,
					ProvisionMode:         NormalizeServiceProvisionMode(inst.ProvisionMode),
					Status:                inst.Status,
					ApplicationID:         app.ID,
					ApplicationName:       app.Name,
					EnvironmentID:         env.ID,
					EnvironmentName:       env.Name,
					EnvironmentIdentifier: env.Identifier,
					ComponentID:           comp.ID,
					ComponentName:         comp.Name,
					ComponentType:         comp.Type,
					ServiceInstanceID:     managedServiceInstanceID(inst.ID),
					MonitoringTarget:      monitoringTargetForNamespace(inst.Namespace),
					MonitoringURL:         monitoringURLForInstallation(monitoringServices, inst),
				})
				continue
			}
			capability, ok := capabilitiesByKey[key]
			if !ok {
				continue
			}
			capServiceType := platformCapabilityServiceType(capability)
			if serviceType != "" && capServiceType != serviceType {
				continue
			}
			refServiceID := uint(0)
			instanceID := capabilityServiceInstanceID(capability.ID)
			serviceName := firstNonEmpty(capability.Provider, capServiceType)
			provisionMode := capability.Source
			if capability.RefServiceID != nil {
				refServiceID = *capability.RefServiceID
				instanceID = managedServiceInstanceID(refServiceID)
				if ref, ok := installationsByID[refServiceID]; ok {
					serviceName = firstNonEmpty(ref.ServiceName, ref.ServiceType)
					provisionMode = NormalizeServiceProvisionMode(ref.ProvisionMode)
				}
			}
			rowID := fmt.Sprintf("component:%d:binding:%d:%s", comp.ID, idx, strings.ReplaceAll(key, ":", "-"))
			if _, exists := seen[rowID]; exists {
				continue
			}
			seen[rowID] = struct{}{}
			out = append(out, PlatformServiceUsage{
				ID:                    rowID,
				ServiceType:           capServiceType,
				ServiceName:           serviceName,
				Provider:              capability.Provider,
				Source:                capability.Source,
				ProvisionMode:         provisionMode,
				Status:                firstNonEmpty(capability.ValidationStatus, "pending"),
				ApplicationID:         app.ID,
				ApplicationName:       app.Name,
				EnvironmentID:         env.ID,
				EnvironmentName:       env.Name,
				EnvironmentIdentifier: env.Identifier,
				ComponentID:           comp.ID,
				ComponentName:         comp.Name,
				ComponentType:         comp.Type,
				Capability:            capability.Capability,
				ServiceInstanceID:     instanceID,
				RefServiceID:          refServiceID,
				Endpoint:              capability.ExternalEndpoint,
				MonitoringTarget:      monitoringTargetForCapability(capability),
				MonitoringURL:         monitoringURLForCapability(monitoringServices, capability, installationsByID),
			})
		}
	}
	return out, nil
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func platformCapabilityServiceType(capability model.EnvironmentCapability) string {
	serviceType := strings.TrimSpace(capability.ServiceType)
	if serviceType == "" {
		serviceType = capabilityServiceTypeFallback(capability.Capability)
	}
	if serviceType == "" {
		return "unknown"
	}
	return serviceType
}

func managedServiceInstanceID(id uint) string {
	return "managed:" + strconvFormatUint(id)
}

func capabilityServiceInstanceID(id uint) string {
	return "capability:" + strconvFormatUint(id)
}

func monitoringTargetForNamespace(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return ""
	}
	return "namespace:" + namespace
}

func monitoringTargetForCapability(capability model.EnvironmentCapability) string {
	if strings.TrimSpace(capability.ExternalEndpoint) != "" {
		return "endpoint:" + strings.TrimSpace(capability.ExternalEndpoint)
	}
	if capability.RefServiceID != nil {
		return managedServiceInstanceID(*capability.RefServiceID)
	}
	return ""
}

func monitoringServicesByEnvironment(db *gorm.DB, envIDs []uint) (map[uint]model.ServiceInstallation, error) {
	if len(envIDs) == 0 {
		return map[uint]model.ServiceInstallation{}, nil
	}
	var monitors []model.ServiceInstallation
	if err := db.
		Where("environment_id IN ?", envIDs).
		Where("service_type IN ?", []string{"monitor", "prometheus-grafana"}).
		Where("status = ?", "running").
		Order("id").
		Find(&monitors).Error; err != nil {
		return nil, err
	}
	out := make(map[uint]model.ServiceInstallation, len(monitors))
	for _, monitor := range monitors {
		if _, exists := out[monitor.EnvironmentID]; !exists {
			out[monitor.EnvironmentID] = monitor
		}
	}
	return out, nil
}

func monitoringURLForInstallation(monitors map[uint]model.ServiceInstallation, inst model.ServiceInstallation) string {
	monitor := monitors[inst.EnvironmentID]
	if monitor.ID == 0 {
		return ""
	}
	if strings.EqualFold(inst.ServiceType, "monitor") || strings.EqualFold(inst.ServiceType, "prometheus-grafana") {
		return platformServiceProxyURL(monitor, "")
	}
	return platformServiceProxyURL(monitor, platformServiceDashboardPath(inst.ServiceType, inst.Namespace))
}

func monitoringURLForCapability(monitors map[uint]model.ServiceInstallation, capability model.EnvironmentCapability, installationsByID map[uint]model.ServiceInstallation) string {
	if capability.RefServiceID != nil && installationsByID != nil {
		if ref, ok := installationsByID[*capability.RefServiceID]; ok {
			return monitoringURLForInstallation(monitors, ref)
		}
	}
	monitor := monitors[capability.EnvironmentID]
	if monitor.ID == 0 {
		return ""
	}
	return platformServiceProxyURL(monitor, "")
}

func platformServiceDashboardPath(serviceType, namespace string) string {
	uid := "paap-middleware-workload"
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "git", "deploy", "ci", "registry", "harbor", "log", "loki":
		uid = "paap-tool-workload"
	}
	values := url.Values{}
	values.Set("orgId", "1")
	values.Set("theme", "light")
	if namespace = strings.TrimSpace(namespace); namespace != "" {
		values.Set("var-namespace", namespace)
	}
	return "/d/" + uid + "?" + values.Encode()
}

func platformServiceProxyURL(inst model.ServiceInstallation, path string) string {
	if inst.EnvironmentID == 0 || inst.ID == 0 {
		return ""
	}
	return fmt.Sprintf("/api/v1/environments/%d/services/%d/proxy/%s", inst.EnvironmentID, inst.ID, strings.TrimLeft(path, "/"))
}

func strconvFormatUint(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}

func environmentIDsFromInstallationsAndCapabilities(installs []model.ServiceInstallation, capabilities []model.EnvironmentCapability) []uint {
	seen := map[uint]struct{}{}
	for _, inst := range installs {
		if inst.EnvironmentID != 0 {
			seen[inst.EnvironmentID] = struct{}{}
		}
	}
	for _, capability := range capabilities {
		if capability.EnvironmentID != 0 {
			seen[capability.EnvironmentID] = struct{}{}
		}
	}
	ids := make([]uint, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

func environmentsByID(db *gorm.DB, ids []uint) (map[uint]model.Environment, error) {
	if len(ids) == 0 {
		return map[uint]model.Environment{}, nil
	}
	var envs []model.Environment
	if err := db.Where("id IN ?", ids).Find(&envs).Error; err != nil {
		return nil, err
	}
	out := make(map[uint]model.Environment, len(envs))
	for _, env := range envs {
		out[env.ID] = env
	}
	return out, nil
}

func capabilityServiceTypeFallback(capability string) string {
	switch strings.ToLower(strings.TrimSpace(capability)) {
	case "database":
		return "database"
	case "cache":
		return "redis"
	case "mq":
		return "mq"
	case "objectstorage", "object_storage", "object-storage":
		return "minio"
	case "git":
		return "git"
	case "registry":
		return "registry"
	case "ci":
		return "ci"
	case "cd":
		return "deploy"
	case "monitor":
		return "monitor"
	case "logging", "log":
		return "log"
	default:
		return strings.ToLower(strings.TrimSpace(capability))
	}
}
