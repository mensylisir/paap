package service

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"paap/internal/model"

	"gopkg.in/yaml.v3"
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
	ErrorLogsURL          string `json:"errorLogsUrl,omitempty"`
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

type CatalogServiceDetail struct {
	Product        CatalogServiceProduct `json:"product"`
	Docs           CatalogServiceDocs    `json:"docs"`
	InstallMethods []CatalogInstallPath  `json:"installMethods"`
}

type CatalogServiceDocs struct {
	Overview   CatalogMarkdownDoc `json:"overview"`
	Install    CatalogMarkdownDoc `json:"install"`
	Quickstart CatalogMarkdownDoc `json:"quickstart"`
}

type CatalogMarkdownDoc struct {
	Title    string `json:"title"`
	Markdown string `json:"markdown"`
}

type CatalogInstallPath struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type CatalogServiceResourceSummary struct {
	ServiceType string                           `json:"serviceType"`
	Total       CatalogServiceResourceFootprint  `json:"total"`
	Groups      []CatalogServiceResourceGroup    `json:"groups"`
	Instances   []CatalogServiceResourceInstance `json:"instances"`
}

type CatalogServiceResourceFootprint struct {
	Instances                   int   `json:"instances"`
	RunningInstances            int   `json:"runningInstances"`
	CPURequestMillicores        int64 `json:"cpuRequestMillicores"`
	CPULimitMillicores          int64 `json:"cpuLimitMillicores"`
	MemoryRequestBytes          int64 `json:"memoryRequestBytes"`
	MemoryLimitBytes            int64 `json:"memoryLimitBytes"`
	StorageRequestBytes         int64 `json:"storageRequestBytes"`
	EstimatedCPUUsageMillicores int64 `json:"estimatedCpuUsageMillicores"`
	EstimatedMemoryUsageBytes   int64 `json:"estimatedMemoryUsageBytes"`
}

type CatalogServiceResourceGroup struct {
	EnvType string `json:"envType"`
	EnvName string `json:"envName,omitempty"`
	CatalogServiceResourceFootprint
}

type CatalogServiceResourceInstance struct {
	ID                    string                          `json:"id"`
	ServiceType           string                          `json:"serviceType"`
	ServiceName           string                          `json:"serviceName"`
	Source                string                          `json:"source"`
	ProvisionMode         string                          `json:"provisionMode,omitempty"`
	Status                string                          `json:"status"`
	ApplicationID         uint                            `json:"applicationId,omitempty"`
	ApplicationName       string                          `json:"applicationName,omitempty"`
	EnvironmentID         uint                            `json:"environmentId,omitempty"`
	EnvironmentName       string                          `json:"environmentName,omitempty"`
	EnvironmentIdentifier string                          `json:"environmentIdentifier,omitempty"`
	EnvType               string                          `json:"envType"`
	Namespace             string                          `json:"namespace,omitempty"`
	SnapshotSource        string                          `json:"snapshotSource"`
	Footprint             CatalogServiceResourceFootprint `json:"footprint"`
}

type CatalogServiceTopology struct {
	ServiceType string                `json:"serviceType"`
	Nodes       []CatalogTopologyNode `json:"nodes"`
	Edges       []CatalogTopologyEdge `json:"edges"`
	Layout      map[string]string     `json:"layout"`
}

type CatalogTopologyNode struct {
	ID     string            `json:"id"`
	Type   string            `json:"type"`
	Label  string            `json:"label"`
	Group  string            `json:"group"`
	Status string            `json:"status,omitempty"`
	Meta   map[string]string `json:"meta,omitempty"`
}

type CatalogTopologyEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Label  string `json:"label,omitempty"`
}

type CatalogServiceObservability struct {
	ServiceType      string                                `json:"serviceType"`
	DashboardUID     string                                `json:"dashboardUid"`
	DashboardTitle   string                                `json:"dashboardTitle"`
	MetricCards      []CatalogServiceMetricCard            `json:"metricCards"`
	LogQueryTemplate string                                `json:"logQueryTemplate"`
	Instances        []CatalogServiceInstanceObservability `json:"instances"`
}

type CatalogServiceMetricCard struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
	PromQL      string `json:"promql"`
}

type CatalogServiceInstanceObservability struct {
	InstanceID       string `json:"instanceId"`
	ServiceName      string `json:"serviceName"`
	EnvironmentName  string `json:"environmentName,omitempty"`
	Namespace        string `json:"namespace,omitempty"`
	MonitoringTarget string `json:"monitoringTarget,omitempty"`
	DashboardURL     string `json:"dashboardUrl,omitempty"`
	ErrorLogsURL     string `json:"errorLogsUrl,omitempty"`
	LogQuery         string `json:"logQuery,omitempty"`
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

func GetCatalogServiceDetail(db *gorm.DB, serviceType string) (CatalogServiceDetail, error) {
	serviceType = strings.TrimSpace(serviceType)
	products, err := buildCatalogServiceProducts(db)
	if err != nil {
		return CatalogServiceDetail{}, err
	}
	var product CatalogServiceProduct
	for _, item := range products {
		if item.Type == serviceType {
			product = item
			break
		}
	}
	if product.Type == "" {
		return CatalogServiceDetail{}, gorm.ErrRecordNotFound
	}
	docs, err := catalogServiceDocs(db, product)
	if err != nil {
		return CatalogServiceDetail{}, err
	}
	return CatalogServiceDetail{
		Product:        product,
		Docs:           docs,
		InstallMethods: catalogInstallPaths(product),
	}, nil
}

func GetCatalogServiceResources(db *gorm.DB, serviceType string) (CatalogServiceResourceSummary, error) {
	return buildCatalogServiceResources(db, serviceType)
}

func GetCatalogServiceTopology(db *gorm.DB, serviceType string) (CatalogServiceTopology, error) {
	return buildCatalogServiceTopology(db, serviceType)
}

func GetCatalogServiceObservability(db *gorm.DB, serviceType string) (CatalogServiceObservability, error) {
	return buildCatalogServiceObservability(db, serviceType)
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
	sortServiceTemplatesForCatalog(templates)
	for _, tmpl := range templates {
		row := ensure(tmpl.Type)
		if strings.TrimSpace(row.Name) == "" || row.Name == row.Type {
			row.Name = firstNonEmpty(tmpl.Name, row.Name, tmpl.Type)
		}
		row.Category = firstNonEmpty(row.Category, tmpl.Category)
		description := strings.TrimSpace(tmpl.Description)
		if manifest, ok, err := catalogManifestFromTemplate(tmpl); err != nil {
			return nil, err
		} else if ok {
			description = firstNonEmpty(manifest.Description, description)
		}
		if strings.TrimSpace(row.Description) == "" {
			row.Description = description
		}
		row.Features = firstNonEmpty(row.Features, tmpl.SupportedFeatures)
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
		row.Description = firstNonEmpty(row.Description, item.Description)
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
	profileFor := catalogServiceObservabilityProfileLookup(db)
	requestedProfile, err := profileFor(serviceType)
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
		observabilityProfile := requestedProfile
		if serviceType == "" {
			observabilityProfile, err = profileFor(inst.ServiceType)
			if err != nil {
				return nil, err
			}
		}
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
			MonitoringURL:         monitoringURLForInstallationWithProfile(monitoringServices, inst, observabilityProfile),
			ErrorLogsURL:          errorLogsURLForInstallationWithProfile(monitoringServices, inst, observabilityProfile),
			UsageCount:            usageCount,
		})
	}
	for _, capability := range capabilities {
		if capability.Source == model.CapabilitySourceShared {
			continue
		}
		env := envs[capability.EnvironmentID]
		app := apps[env.ApplicationID]
		monitoringURL, err := monitoringURLForCapabilityWithProfileLookup(monitoringServices, capability, nil, profileFor)
		if err != nil {
			return nil, err
		}
		errorLogsURL, err := errorLogsURLForCapabilityWithProfileLookup(monitoringServices, capability, nil, profileFor)
		if err != nil {
			return nil, err
		}
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
			MonitoringURL:         monitoringURL,
			ErrorLogsURL:          errorLogsURL,
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
	profileFor := catalogServiceObservabilityProfileLookup(db)
	out := make([]PlatformServiceUsage, 0, len(installations)+len(capabilities))
	for _, inst := range installations {
		observabilityProfile, err := profileFor(inst.ServiceType)
		if err != nil {
			return nil, err
		}
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
			MonitoringURL:         monitoringURLForInstallationWithProfile(monitoringServices, inst, observabilityProfile),
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
		monitoringURL, err := monitoringURLForCapabilityWithProfileLookup(monitoringServices, capability, installationsByID, profileFor)
		if err != nil {
			return nil, err
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
			MonitoringURL:         monitoringURL,
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

func buildCatalogServiceResources(db *gorm.DB, serviceType string) (CatalogServiceResourceSummary, error) {
	serviceType = strings.TrimSpace(serviceType)
	installations, capabilities, envs, apps, err := loadPlatformServiceInputs(db, serviceType)
	if err != nil {
		return CatalogServiceResourceSummary{}, err
	}
	summary := CatalogServiceResourceSummary{ServiceType: serviceType}
	groups := map[string]*CatalogServiceResourceGroup{}
	addGroup := func(env model.Environment, app model.Application, fp CatalogServiceResourceFootprint) string {
		key := environmentClass(app, env)
		group := groups[key]
		if group == nil {
			group = &CatalogServiceResourceGroup{EnvType: key}
			groups[key] = group
		}
		group.CatalogServiceResourceFootprint.add(fp)
		summary.Total.add(fp)
		return key
	}

	for _, inst := range installations {
		env := envs[inst.EnvironmentID]
		app := apps[env.ApplicationID]
		footprint, source := resourceFootprintForInstallation(inst)
		envType := addGroup(env, app, footprint)
		summary.Instances = append(summary.Instances, CatalogServiceResourceInstance{
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
			EnvType:               envType,
			Namespace:             inst.Namespace,
			SnapshotSource:        source,
			Footprint:             footprint,
		})
	}

	for _, capability := range capabilities {
		if capability.Source == model.CapabilitySourceShared {
			continue
		}
		env := envs[capability.EnvironmentID]
		app := apps[env.ApplicationID]
		footprint := CatalogServiceResourceFootprint{Instances: 1}
		envType := addGroup(env, app, footprint)
		summary.Instances = append(summary.Instances, CatalogServiceResourceInstance{
			ID:                    capabilityServiceInstanceID(capability.ID),
			ServiceType:           platformCapabilityServiceType(capability),
			ServiceName:           firstNonEmpty(capability.Provider, platformCapabilityServiceType(capability)),
			Source:                capability.Source,
			ProvisionMode:         capability.Source,
			Status:                firstNonEmpty(capability.ValidationStatus, "pending"),
			ApplicationID:         app.ID,
			ApplicationName:       app.Name,
			EnvironmentID:         env.ID,
			EnvironmentName:       env.Name,
			EnvironmentIdentifier: env.Identifier,
			EnvType:               envType,
			SnapshotSource:        "external-config",
			Footprint:             footprint,
		})
	}

	for _, group := range groups {
		summary.Groups = append(summary.Groups, *group)
	}
	sort.Slice(summary.Groups, func(i, j int) bool { return summary.Groups[i].EnvType < summary.Groups[j].EnvType })
	sort.Slice(summary.Instances, func(i, j int) bool {
		if summary.Instances[i].EnvType != summary.Instances[j].EnvType {
			return summary.Instances[i].EnvType < summary.Instances[j].EnvType
		}
		return summary.Instances[i].ServiceName < summary.Instances[j].ServiceName
	})
	return summary, nil
}

func buildCatalogServiceTopology(db *gorm.DB, serviceType string) (CatalogServiceTopology, error) {
	serviceType = strings.TrimSpace(serviceType)
	instances, err := buildPlatformServiceInstances(db, serviceType)
	if err != nil {
		return CatalogServiceTopology{}, err
	}
	architectureProfile, err := catalogArchitectureProfile(db, serviceType)
	if err != nil {
		return CatalogServiceTopology{}, err
	}
	topology := CatalogServiceTopology{
		ServiceType: serviceType,
		Nodes:       []CatalogTopologyNode{},
		Edges:       []CatalogTopologyEdge{},
		Layout: map[string]string{
			"engine":    "dagre",
			"direction": "LR",
		},
	}
	nodeSeen := map[string]struct{}{}
	edgeSeen := map[string]struct{}{}
	addNode := func(node CatalogTopologyNode) {
		if node.ID == "" {
			return
		}
		if _, exists := nodeSeen[node.ID]; exists {
			return
		}
		nodeSeen[node.ID] = struct{}{}
		topology.Nodes = append(topology.Nodes, node)
	}
	addEdge := func(edge CatalogTopologyEdge) {
		if edge.ID == "" || edge.Source == "" || edge.Target == "" {
			return
		}
		if _, exists := edgeSeen[edge.ID]; exists {
			return
		}
		edgeSeen[edge.ID] = struct{}{}
		topology.Edges = append(topology.Edges, edge)
	}

	productNodeID := "service:" + serviceType
	addNode(CatalogTopologyNode{ID: productNodeID, Type: "service-product", Label: serviceType, Group: "product"})
	for _, inst := range instances {
		instanceNodeID := "instance:" + inst.ID
		addNode(CatalogTopologyNode{
			ID:     instanceNodeID,
			Type:   "service-instance",
			Label:  firstNonEmpty(inst.ServiceName, inst.ID),
			Group:  "instance",
			Status: inst.Status,
			Meta: map[string]string{
				"source":      inst.Source,
				"environment": firstNonEmpty(inst.EnvironmentName, inst.EnvironmentIdentifier),
				"namespace":   inst.Namespace,
			},
		})
		addEdge(CatalogTopologyEdge{ID: productNodeID + "->" + instanceNodeID, Source: productNodeID, Target: instanceNodeID, Type: "has-instance"})
		architectureNodes := architectureProfileNodes(architectureProfile, inst)
		for _, node := range architectureNodes {
			addNode(node)
			addEdge(CatalogTopologyEdge{ID: instanceNodeID + "->" + node.ID, Source: instanceNodeID, Target: node.ID, Type: "contains"})
		}
		if len(architectureNodes) > 1 {
			for _, node := range architectureNodes[1:] {
				addEdge(CatalogTopologyEdge{
					ID:     architectureNodes[0].ID + "->" + node.ID,
					Source: architectureNodes[0].ID,
					Target: node.ID,
					Type:   "replicates",
					Label:  "replication",
				})
			}
		}
	}
	return topology, nil
}

func buildCatalogServiceObservability(db *gorm.DB, serviceType string) (CatalogServiceObservability, error) {
	serviceType = strings.TrimSpace(serviceType)
	profile, err := catalogServiceObservabilityProfile(db, serviceType)
	if err != nil {
		return CatalogServiceObservability{}, err
	}
	instances, err := buildPlatformServiceInstances(db, serviceType)
	if err != nil {
		return CatalogServiceObservability{}, err
	}
	out := CatalogServiceObservability{
		ServiceType:      serviceType,
		DashboardUID:     profile.DashboardUID,
		DashboardTitle:   profile.DashboardTitle,
		MetricCards:      profile.MetricCards,
		LogQueryTemplate: profile.LogQueryTemplate,
	}
	for _, inst := range instances {
		query := serviceLogQuery(profile, inst.Namespace, inst.ServiceName)
		out.Instances = append(out.Instances, CatalogServiceInstanceObservability{
			InstanceID:       inst.ID,
			ServiceName:      inst.ServiceName,
			EnvironmentName:  firstNonEmpty(inst.EnvironmentName, inst.EnvironmentIdentifier),
			Namespace:        inst.Namespace,
			MonitoringTarget: inst.MonitoringTarget,
			DashboardURL:     monitoringURLWithProfile(inst.MonitoringURL, profile, inst.Namespace),
			ErrorLogsURL:     errorLogsURLWithProfile(inst.ErrorLogsURL, profile, inst.Namespace, inst.ServiceName),
			LogQuery:         query,
		})
	}
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

func (f *CatalogServiceResourceFootprint) add(other CatalogServiceResourceFootprint) {
	f.Instances += other.Instances
	f.RunningInstances += other.RunningInstances
	f.CPURequestMillicores += other.CPURequestMillicores
	f.CPULimitMillicores += other.CPULimitMillicores
	f.MemoryRequestBytes += other.MemoryRequestBytes
	f.MemoryLimitBytes += other.MemoryLimitBytes
	f.StorageRequestBytes += other.StorageRequestBytes
	f.EstimatedCPUUsageMillicores += other.EstimatedCPUUsageMillicores
	f.EstimatedMemoryUsageBytes += other.EstimatedMemoryUsageBytes
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
	profileFor := catalogServiceObservabilityProfileLookup(db)

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
				observabilityProfile, err := profileFor(inst.ServiceType)
				if err != nil {
					return nil, err
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
					MonitoringURL:         monitoringURLForInstallationWithProfile(monitoringServices, inst, observabilityProfile),
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
			monitoringURL, err := monitoringURLForCapabilityWithProfileLookup(monitoringServices, capability, installationsByID, profileFor)
			if err != nil {
				return nil, err
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
				MonitoringURL:         monitoringURL,
			})
		}
	}
	return out, nil
}

func catalogServiceManifest(db *gorm.DB, serviceType string) (model.PlatformManifest, bool, error) {
	serviceType = strings.TrimSpace(serviceType)
	if serviceType == "" {
		return model.PlatformManifest{}, false, nil
	}
	var templates []model.ServiceTemplate
	if err := db.
		Where("enabled = ?", true).
		Where("type = ?", serviceType).
		Order("id").
		Find(&templates).Error; err != nil {
		return model.PlatformManifest{}, false, err
	}
	sortServiceTemplatesForCatalog(templates)
	for _, tmpl := range templates {
		manifest, ok, err := catalogManifestFromTemplate(tmpl)
		if err != nil {
			return model.PlatformManifest{}, false, fmt.Errorf("parse service template %d manifest: %w", tmpl.ID, err)
		}
		if ok {
			return manifest, true, nil
		}
	}
	return model.PlatformManifest{}, false, nil
}

func sortServiceTemplatesForCatalog(templates []model.ServiceTemplate) {
	sort.SliceStable(templates, func(i, j int) bool {
		leftMode := NormalizeServiceProvisionMode(templates[i].ProvisionMode)
		rightMode := NormalizeServiceProvisionMode(templates[j].ProvisionMode)
		if leftMode != rightMode {
			return leftMode != model.ServiceProvisionModeKubeVirt
		}
		if templates[i].InstallOrder != templates[j].InstallOrder {
			return templates[i].InstallOrder < templates[j].InstallOrder
		}
		return templates[i].ID < templates[j].ID
	})
}

func catalogManifestFromTemplate(tmpl model.ServiceTemplate) (model.PlatformManifest, bool, error) {
	raw := strings.TrimSpace(tmpl.PlatformManifestJSON)
	if raw == "" {
		return model.PlatformManifest{}, false, nil
	}
	var manifest model.PlatformManifest
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		return model.PlatformManifest{}, false, err
	}
	return manifest, true, nil
}

func catalogArchitectureProfile(db *gorm.DB, serviceType string) ([]model.CatalogArchitectureSpec, error) {
	manifest, ok, err := catalogServiceManifest(db, serviceType)
	if err != nil {
		return nil, err
	}
	if !ok || manifest.Catalog == nil {
		return nil, nil
	}
	return manifest.Catalog.Architecture, nil
}

func catalogServiceDocs(db *gorm.DB, product CatalogServiceProduct) (CatalogServiceDocs, error) {
	manifest, ok, err := catalogServiceManifest(db, product.Type)
	if err != nil {
		return CatalogServiceDocs{}, err
	}
	if !ok {
		return CatalogServiceDocs{}, fmt.Errorf("service template %s missing platform manifest catalog docs", product.Type)
	}
	if err := manifest.ValidateCatalogDocs(); err != nil {
		return CatalogServiceDocs{}, fmt.Errorf("service template %s invalid catalog docs: %w", product.Type, err)
	}
	docs := manifest.Catalog.Docs
	overview := strings.TrimSpace(docs.Overview)
	install := strings.TrimSpace(docs.Install)
	quickstart := strings.TrimSpace(docs.Quickstart)
	return CatalogServiceDocs{
		Overview: CatalogMarkdownDoc{
			Title:    "服务介绍",
			Markdown: overview,
		},
		Install: CatalogMarkdownDoc{
			Title:    "安装方式",
			Markdown: install,
		},
		Quickstart: CatalogMarkdownDoc{
			Title:    "Quick Start",
			Markdown: quickstart,
		},
	}, nil
}

func catalogInstallPaths(product CatalogServiceProduct) []CatalogInstallPath {
	features := parseServiceFeatureItems(product.Features, product.Type, product.Category)
	descriptions := map[string]string{
		"managed":  "在当前环境中通过平台服务模板创建并维护实例。",
		"shared":   "引用共享资源池中的公共实例，业务环境只维护连接关系。",
		"external": "接入集群外部或公司已有服务，平台只验证连接并托管凭据引用。",
		"kubevirt": "通过 KubeVirt 服务模板交付数据库、缓存等专用实例。",
	}
	out := make([]CatalogInstallPath, 0, len(features))
	for _, feature := range features {
		out = append(out, CatalogInstallPath{
			Key:         feature.Key,
			Label:       feature.Label,
			Description: descriptions[feature.Key],
			Enabled:     feature.Enabled,
		})
	}
	return out
}

func parseServiceFeatureItems(raw, serviceType, category string) []ServiceFeatureItem {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = ServiceFeatureMatrixJSON(serviceType, category)
	}
	var items []ServiceFeatureItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil || len(items) == 0 {
		_ = json.Unmarshal([]byte(ServiceFeatureMatrixJSON(serviceType, category)), &items)
	}
	return items
}

func environmentClass(app model.Application, env model.Environment) string {
	if model.IsSystemSharedEnvironment(app, env) {
		return "shared"
	}
	value := strings.ToLower(env.Identifier + " " + env.Name)
	switch {
	case strings.Contains(value, "prod") || strings.Contains(value, "生产"):
		return "prod"
	case strings.Contains(value, "test") || strings.Contains(value, "qa") || strings.Contains(value, "测试"):
		return "test"
	case strings.Contains(value, "stage") || strings.Contains(value, "staging") || strings.Contains(value, "预发"):
		return "staging"
	case strings.Contains(value, "dev") || strings.Contains(value, "开发"):
		return "dev"
	default:
		return firstNonEmpty(env.Identifier, "unknown")
	}
}

func resourceFootprintForInstallation(inst model.ServiceInstallation) (CatalogServiceResourceFootprint, string) {
	fp := defaultResourceFootprint(inst.ServiceType)
	fp.Instances = 1
	if strings.EqualFold(inst.Status, "running") {
		fp.RunningInstances = 1
	}
	source := "service-default"
	if strings.TrimSpace(inst.Values) != "" {
		if overrides, ok := resourceFootprintFromValues(inst.ServiceType, inst.Values); ok {
			fp.CPURequestMillicores = firstPositiveInt64(overrides.CPURequestMillicores, fp.CPURequestMillicores)
			fp.CPULimitMillicores = firstPositiveInt64(overrides.CPULimitMillicores, fp.CPULimitMillicores)
			fp.MemoryRequestBytes = firstPositiveInt64(overrides.MemoryRequestBytes, fp.MemoryRequestBytes)
			fp.MemoryLimitBytes = firstPositiveInt64(overrides.MemoryLimitBytes, fp.MemoryLimitBytes)
			fp.StorageRequestBytes = firstPositiveInt64(overrides.StorageRequestBytes, fp.StorageRequestBytes)
			source = "install-values"
		}
	}
	fp.EstimatedCPUUsageMillicores = fp.CPURequestMillicores * 65 / 100
	fp.EstimatedMemoryUsageBytes = fp.MemoryRequestBytes * 70 / 100
	return fp, source
}

func resourceFootprintFromValues(serviceType, raw string) (CatalogServiceResourceFootprint, bool) {
	var values map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &values); err != nil {
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			return CatalogServiceResourceFootprint{}, false
		}
	}
	out := CatalogServiceResourceFootprint{}
	out.CPURequestMillicores = parseCPUMillicores(valueAtAnyPath(values,
		"resources.requests.cpu",
		"primary.resources.requests.cpu",
		"master.resources.requests.cpu",
		"broker.resources.requests.cpu",
	))
	out.CPULimitMillicores = parseCPUMillicores(valueAtAnyPath(values,
		"resources.limits.cpu",
		"primary.resources.limits.cpu",
		"master.resources.limits.cpu",
		"broker.resources.limits.cpu",
	))
	out.MemoryRequestBytes = parseBytesQuantity(valueAtAnyPath(values,
		"resources.requests.memory",
		"primary.resources.requests.memory",
		"master.resources.requests.memory",
		"broker.resources.requests.memory",
	))
	out.MemoryLimitBytes = parseBytesQuantity(valueAtAnyPath(values,
		"resources.limits.memory",
		"primary.resources.limits.memory",
		"master.resources.limits.memory",
		"broker.resources.limits.memory",
	))
	out.StorageRequestBytes = parseBytesQuantity(valueAtAnyPath(values,
		"persistence.size",
		"primary.persistence.size",
		"master.persistence.size",
		"broker.persistence.size",
		"volumePermissions.size",
	))
	return out, out.CPURequestMillicores > 0 || out.CPULimitMillicores > 0 || out.MemoryRequestBytes > 0 || out.MemoryLimitBytes > 0 || out.StorageRequestBytes > 0
}

func defaultResourceFootprint(serviceType string) CatalogServiceResourceFootprint {
	switch strings.ToLower(strings.TrimSpace(serviceType)) {
	case "kafka":
		return CatalogServiceResourceFootprint{CPURequestMillicores: 500, CPULimitMillicores: 1000, MemoryRequestBytes: 2 * 1024 * 1024 * 1024, MemoryLimitBytes: 4 * 1024 * 1024 * 1024, StorageRequestBytes: 50 * 1024 * 1024 * 1024}
	case "mysql", "postgresql", "mongodb":
		return CatalogServiceResourceFootprint{CPURequestMillicores: 250, CPULimitMillicores: 1000, MemoryRequestBytes: 1 * 1024 * 1024 * 1024, MemoryLimitBytes: 2 * 1024 * 1024 * 1024, StorageRequestBytes: 20 * 1024 * 1024 * 1024}
	case "redis", "redis-cluster":
		return CatalogServiceResourceFootprint{CPURequestMillicores: 100, CPULimitMillicores: 500, MemoryRequestBytes: 256 * 1024 * 1024, MemoryLimitBytes: 512 * 1024 * 1024, StorageRequestBytes: 8 * 1024 * 1024 * 1024}
	case "rabbitmq":
		return CatalogServiceResourceFootprint{CPURequestMillicores: 200, CPULimitMillicores: 1000, MemoryRequestBytes: 512 * 1024 * 1024, MemoryLimitBytes: 2 * 1024 * 1024 * 1024, StorageRequestBytes: 8 * 1024 * 1024 * 1024}
	default:
		return CatalogServiceResourceFootprint{CPURequestMillicores: 100, CPULimitMillicores: 500, MemoryRequestBytes: 256 * 1024 * 1024, MemoryLimitBytes: 512 * 1024 * 1024, StorageRequestBytes: 1 * 1024 * 1024 * 1024}
	}
}

func firstPositiveInt64(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func valueAtAnyPath(values map[string]interface{}, paths ...string) string {
	for _, path := range paths {
		if value := valueAtPath(values, path); value != "" {
			return value
		}
	}
	return ""
}

func valueAtPath(values map[string]interface{}, path string) string {
	var current interface{} = values
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}
	return fmt.Sprint(current)
}

func parseCPUMillicores(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if strings.HasSuffix(value, "m") {
		n, _ := strconv.ParseInt(strings.TrimSuffix(value, "m"), 10, 64)
		return n
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return int64(n * 1000)
}

func parseBytesQuantity(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	units := []struct {
		suffix string
		factor float64
	}{
		{"Ti", 1024 * 1024 * 1024 * 1024}, {"Gi", 1024 * 1024 * 1024}, {"Mi", 1024 * 1024}, {"Ki", 1024},
		{"T", 1000 * 1000 * 1000 * 1000}, {"G", 1000 * 1000 * 1000}, {"M", 1000 * 1000}, {"K", 1000},
	}
	for _, unit := range units {
		if strings.HasSuffix(value, unit.suffix) {
			n, err := strconv.ParseFloat(strings.TrimSuffix(value, unit.suffix), 64)
			if err != nil {
				return 0
			}
			return int64(n * unit.factor)
		}
	}
	n, _ := strconv.ParseInt(value, 10, 64)
	return n
}

func architectureProfileNodes(profile []model.CatalogArchitectureSpec, inst PlatformServiceInstance) []CatalogTopologyNode {
	base := "instance:" + inst.ID + ":"
	meta := map[string]string{"namespace": inst.Namespace}
	if len(profile) == 0 {
		if inst.Namespace == "" {
			return nil
		}
		return []CatalogTopologyNode{{ID: base + "namespace", Type: "namespace", Label: inst.Namespace, Group: "architecture", Status: inst.Status, Meta: meta}}
	}
	out := make([]CatalogTopologyNode, 0, len(profile))
	for _, item := range profile {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = strings.TrimSpace(item.Label)
		}
		id = strings.NewReplacer(" ", "-", "/", "-", ":", "-").Replace(strings.ToLower(id))
		if id == "" {
			continue
		}
		out = append(out, CatalogTopologyNode{
			ID:     base + id,
			Type:   firstNonEmpty(item.Type, "architecture"),
			Label:  firstNonEmpty(item.Label, id),
			Group:  firstNonEmpty(item.Group, "architecture"),
			Status: inst.Status,
			Meta:   meta,
		})
	}
	return out
}

type serviceObservabilityProfileData struct {
	DashboardUID     string
	DashboardTitle   string
	MetricCards      []CatalogServiceMetricCard
	LogQueryTemplate string
}

func catalogServiceObservabilityProfile(db *gorm.DB, serviceType string) (serviceObservabilityProfileData, error) {
	manifest, ok, err := catalogServiceManifest(db, serviceType)
	if err != nil {
		return serviceObservabilityProfileData{}, err
	}
	if ok && manifest.Observability != nil {
		spec := manifest.Observability
		profile := serviceObservabilityProfileData{
			DashboardUID:     strings.TrimSpace(spec.DashboardUID),
			DashboardTitle:   strings.TrimSpace(spec.DashboardTitle),
			LogQueryTemplate: strings.TrimSpace(spec.LogQueryTemplate),
		}
		for _, item := range spec.MetricCards {
			profile.MetricCards = append(profile.MetricCards, CatalogServiceMetricCard{
				Key:         item.Key,
				Title:       item.Title,
				Unit:        item.Unit,
				Description: item.Description,
				PromQL:      item.PromQL,
			})
		}
		return profile, nil
	}
	return serviceObservabilityProfileData{}, nil
}

func catalogServiceObservabilityProfileLookup(db *gorm.DB) func(string) (serviceObservabilityProfileData, error) {
	cache := map[string]serviceObservabilityProfileData{}
	return func(serviceType string) (serviceObservabilityProfileData, error) {
		serviceType = strings.TrimSpace(serviceType)
		if serviceType == "" {
			return serviceObservabilityProfileData{}, nil
		}
		if profile, ok := cache[serviceType]; ok {
			return profile, nil
		}
		profile, err := catalogServiceObservabilityProfile(db, serviceType)
		if err != nil {
			return serviceObservabilityProfileData{}, err
		}
		cache[serviceType] = profile
		return profile, nil
	}
}

func serviceLogQuery(profile serviceObservabilityProfileData, namespace, serviceName string) string {
	query := strings.TrimSpace(profile.LogQueryTemplate)
	if query == "" {
		return ""
	}
	query = strings.ReplaceAll(query, "$namespace", strings.TrimSpace(namespace))
	query = strings.ReplaceAll(query, "$service", strings.TrimSpace(serviceName))
	return query
}

func errorLogsURLForInstallationWithProfile(monitors map[uint]model.ServiceInstallation, inst model.ServiceInstallation, profile serviceObservabilityProfileData) string {
	monitor := monitors[inst.EnvironmentID]
	if monitor.ID == 0 {
		return ""
	}
	explorePath := platformServiceExplorePath(profile, inst.Namespace, inst.ServiceName)
	if explorePath == "" {
		return ""
	}
	return platformServiceProxyURL(monitor, explorePath)
}

func errorLogsURLForCapabilityWithProfileLookup(
	monitors map[uint]model.ServiceInstallation,
	capability model.EnvironmentCapability,
	installationsByID map[uint]model.ServiceInstallation,
	profileFor func(string) (serviceObservabilityProfileData, error),
) (string, error) {
	if capability.RefServiceID == nil || installationsByID == nil {
		return "", nil
	}
	ref, ok := installationsByID[*capability.RefServiceID]
	if !ok {
		return "", nil
	}
	profile, err := profileFor(ref.ServiceType)
	if err != nil {
		return "", err
	}
	return errorLogsURLForInstallationWithProfile(monitors, ref, profile), nil
}

func platformServiceExplorePath(profile serviceObservabilityProfileData, namespace, serviceName string) string {
	query := serviceLogQuery(profile, namespace, serviceName)
	if query == "" {
		return ""
	}
	left := map[string]interface{}{
		"datasource": "Loki",
		"queries": []map[string]string{{
			"refId": "A",
			"expr":  query,
		}},
		"range": map[string]string{
			"from": "now-1h",
			"to":   "now",
		},
	}
	payload, _ := json.Marshal(left)
	values := url.Values{}
	values.Set("orgId", "1")
	values.Set("left", string(payload))
	return "/explore?" + values.Encode()
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

func monitoringURLForInstallationWithProfile(monitors map[uint]model.ServiceInstallation, inst model.ServiceInstallation, profile serviceObservabilityProfileData) string {
	monitor := monitors[inst.EnvironmentID]
	if monitor.ID == 0 {
		return ""
	}
	if strings.EqualFold(inst.ServiceType, "monitor") || strings.EqualFold(inst.ServiceType, "prometheus-grafana") {
		return platformServiceProxyURL(monitor, "")
	}
	if strings.TrimSpace(profile.DashboardUID) == "" {
		return ""
	}
	return platformServiceProxyURL(monitor, platformServiceDashboardPath(profile.DashboardUID, inst.Namespace))
}

func monitoringURLForCapabilityWithProfileLookup(
	monitors map[uint]model.ServiceInstallation,
	capability model.EnvironmentCapability,
	installationsByID map[uint]model.ServiceInstallation,
	profileFor func(string) (serviceObservabilityProfileData, error),
) (string, error) {
	if capability.RefServiceID != nil && installationsByID != nil {
		if ref, ok := installationsByID[*capability.RefServiceID]; ok {
			profile, err := profileFor(ref.ServiceType)
			if err != nil {
				return "", err
			}
			return monitoringURLForInstallationWithProfile(monitors, ref, profile), nil
		}
	}
	return "", nil
}

func monitoringURLWithProfile(existing string, profile serviceObservabilityProfileData, namespace string) string {
	if strings.TrimSpace(existing) == "" {
		return ""
	}
	if strings.TrimSpace(profile.DashboardUID) == "" {
		return ""
	}
	idx := strings.Index(existing, "/proxy/")
	if idx < 0 {
		return existing
	}
	return existing[:idx] + "/proxy/" + strings.TrimLeft(platformServiceDashboardPath(profile.DashboardUID, namespace), "/")
}

func errorLogsURLWithProfile(existing string, profile serviceObservabilityProfileData, namespace, serviceName string) string {
	if strings.TrimSpace(existing) == "" {
		return ""
	}
	explorePath := platformServiceExplorePath(profile, namespace, serviceName)
	if explorePath == "" {
		return ""
	}
	idx := strings.Index(existing, "/proxy/")
	if idx < 0 {
		return existing
	}
	return existing[:idx] + "/proxy/" + strings.TrimLeft(explorePath, "/")
}

func platformServiceDashboardPath(uid, namespace string) string {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return ""
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
