package model

import (
	"time"

	"gorm.io/gorm"
)

// ServiceTemplate defines HOW to install a specific tool or infrastructure service.
// This is the implementation layer - it specifies the helm chart, default values,
// configurable parameters, install/uninstall steps, etc.
type ServiceTemplate struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`

	Type        string `gorm:"uniqueIndex;size:30;not null" json:"type"` // e.g. "postgresql", "deploy", "ci"
	Name        string `gorm:"size:50;not null" json:"name"`             // e.g. "PostgreSQL", "部署服务"
	Category    string `gorm:"size:20;not null" json:"category"`         // "tool" | "infra" | "middleware"
	Description string `gorm:"size:500" json:"description"`
	Icon        string `gorm:"size:50" json:"icon"`

	// Installer configuration
	Installer    string `gorm:"size:20;not null" json:"installer"` // "helm" | "kubectl" | "raw-yaml"
	ChartRepo    string `gorm:"size:200" json:"chartRepo"`         // "https://charts.bitnami.com/bitnami"
	ChartName    string `gorm:"size:100" json:"chartName"`         // "bitnami/postgresql"
	ChartVersion string `gorm:"size:30" json:"chartVersion"`       // "12.12.10"

	// Default values (JSON)
	DefaultValues string `gorm:"type:text" json:"defaultValues"`

	// Configurable parameters (JSON array of param definitions)
	// e.g. [{"key":"auth.username","label":"用户名","type":"string","required":true}, ...]
	ConfigurableParams string `gorm:"type:text" json:"configurableParams"`

	// Raw YAML template for kubectl apply mode
	RawYamlTemplate string `gorm:"type:text" json:"rawYamlTemplate"`

	// Per-tool workload namespace RBAC rules (JSON array of PolicyRule).
	// Defines the permissions projected into environment workload namespaces.
	// Example: [{"apiGroups":[""],"resources":["pods","services"],"verbs":["get","list","watch"]}]
	WorkloadRolePolicy string `gorm:"type:text" json:"workloadRolePolicy"`

	// Per-tool environment namespace RBAC rules (JSON array of PolicyRule).
	// Defines the permissions projected into non-self namespaces in the same environment,
	// including middleware and other tool namespaces.
	EnvironmentRolePolicy string `gorm:"type:text" json:"environmentRolePolicy"`

	// Custom template (BYO) fields
	// IsCustom indicates this is a user-uploaded template (not a built-in one).
	IsCustom bool `gorm:"default:false" json:"isCustom"`

	// PlatformManifestJSON stores the parsed platform-manifest.yaml as JSON.
	// For custom templates, this declares permissions and observability requirements.
	// Example: {"name":"custom-monitor","permissions":{"environmentNamespaces":{"rules":[...]}}}
	PlatformManifestJSON string `gorm:"type:text" json:"platformManifestJSON"`

	// ChartArchivePath is the filesystem path to the uploaded chart archive (tar.gz).
	// Only set for custom templates. The chart is stored under data/charts/{template_type}/
	ChartArchivePath string `gorm:"size:500" json:"chartArchivePath,omitempty"`

	// S3 object key for the chart archive
	// If set, the chart is stored in S3 and will be downloaded at install time
	S3Bucket string `gorm:"size:100" json:"s3Bucket,omitempty"`
	S3Key    string `gorm:"size:500" json:"s3Key,omitempty"`

	// PresetValues stores the content of preset-values.yaml from the template archive.
	// These are applied BEFORE platform context variables and user parameters.
	// Typically used to disable built-in RBAC: rbac.create=false, serviceAccount.create=false
	PresetValues string `gorm:"type:text" json:"presetValues,omitempty"`

	// Installation order hint (lower = earlier)
	InstallOrder int `gorm:"default:0" json:"installOrder"`

	Enabled bool `gorm:"default:true" json:"enabled"`
}

// EnvTemplate defines WHAT services an environment contains.
// This is the orchestration layer - it references ServiceTemplates by type.
type EnvTemplate struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`

	Name        string `gorm:"size:50;not null" json:"name"`
	Description string `gorm:"size:500" json:"description"`

	// List of service types to install (references ServiceTemplate.Type)
	// JSON array: ["deploy","ci","monitor"]
	Services string `gorm:"type:text" json:"services"`

	// List of infra types to install
	// JSON array: ["postgresql","redis"]
	Infra string `gorm:"type:text" json:"infra"`

	// Resource quota for the environment
	ResourceCPU  string `gorm:"size:20;default:4核" json:"resourceCpu"`
	ResourceMem  string `gorm:"size:20;default:8GB" json:"resourceMem"`
	ResourceDisk string `gorm:"size:20;default:50GB" json:"resourceDisk"`
}

// ServiceInstallation tracks an installed service in an environment
type ServiceInstallation struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`

	EnvironmentID uint   `gorm:"not null;uniqueIndex:idx_service_installation_env_type" json:"environmentId"`
	ServiceType   string `gorm:"size:30;not null;uniqueIndex:idx_service_installation_env_type" json:"serviceType"` // references ServiceTemplate.Type
	ServiceName   string `gorm:"size:50" json:"serviceName"`                                                        // instance name
	ReleaseName   string `gorm:"size:50" json:"releaseName"`                                                        // helm release name

	Status       string `gorm:"size:20;default:pending" json:"status"` // pending, installing, running, failed, deleting
	ErrorMessage string `gorm:"type:text" json:"errorMessage"`

	// Actual values used for this installation (merged default + user override)
	Values string `gorm:"type:text" json:"values"`

	Namespace string `gorm:"size:100" json:"namespace"`
}

// EnvTemplateServiceRef is a join table for EnvTemplate -> ServiceTemplate relations
// (used for more flexible template composition)
type EnvTemplateServiceRef struct {
	ID            uint   `gorm:"primarykey" json:"id"`
	EnvTemplateID uint   `gorm:"not null;index" json:"envTemplateId"`
	ServiceType   string `gorm:"size:30;not null" json:"serviceType"`
	InstallOrder  int    `gorm:"default:0" json:"installOrder"`
}
