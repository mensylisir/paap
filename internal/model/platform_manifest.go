package model

import "encoding/json"

// PermissionScope defines the scope of RBAC permissions a custom template requests.
type PermissionScope string

const (
	// PermissionScopeToolOnly means the tool only needs permissions in its own namespace.
	PermissionScopeToolOnly PermissionScope = "tool-only"

	// PermissionScopeEnvironmentWide means the tool needs permissions across all
	// namespaces in the environment (the platform will dynamically inject RoleBindings).
	PermissionScopeEnvironmentWide PermissionScope = "environment-wide"
)

// PlatformManifest is the metadata file (platform-manifest.yaml) that users include
// in their custom Helm chart upload. It declares what the tool "wants" from the platform
// so the platform can inject permissions and observability automatically.
type PlatformManifest struct {
	// Name of the tool (e.g. "custom-monitor", "my-argocd")
	Name string `json:"name" yaml:"name"`

	// Version of the tool (e.g. "v1.2", "2.0.0")
	Version string `json:"version" yaml:"version"`

	// Description of what this tool does
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Permissions declares the RBAC requirements for this tool.
	// The platform uses this to dynamically create Roles and RoleBindings.
	Permissions PermissionsSpec `json:"permissions" yaml:"permissions"`

	// Observability declares how the platform should monitor this tool.
	// +optional
	Observability *ObservabilitySpec `json:"observability,omitempty" yaml:"observability,omitempty"`

	// VariableMapping allows users to map platform context variables to Helm values.
	// This gives users fine-grained control over which platform variables their chart receives.
	// Platform-injected global.* variables are always included; this adds custom mappings on top.
	// +optional
	VariableMapping []VariableMappingEntry `json:"variable_mapping,omitempty" yaml:"variable_mapping,omitempty"`
}

// VariableMappingEntry maps a platform variable to a Helm value key.
type VariableMappingEntry struct {
	// PlatformVar is the platform-side variable name (e.g. "current_env_name", "primary_namespace").
	PlatformVar string `json:"platform_var" yaml:"platform_var"`

	// HelmVar is the Helm values key to set (e.g. "global.envName", "config.namespace").
	HelmVar string `json:"helm_var" yaml:"helm_var"`
}

// PermissionsSpec declares what RBAC permissions the tool needs.
type PermissionsSpec struct {
	// Scope determines the RBAC strategy:
	// - "tool-only": tool only operates in its own namespace (no cross-ns RBAC)
	// - "environment-wide": tool needs access to all namespaces in the environment
	Scope PermissionScope `json:"scope" yaml:"scope"`

	// Rules defines the exact RBAC PolicyRules the tool needs in workload namespaces.
	// Only used when Scope is "environment-wide".
	// The platform will create a Role with these rules in every environment namespace.
	Rules []PolicyRuleSpec `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// PolicyRuleSpec mirrors K8s rbac.PolicyRule but uses simple types for YAML serialization.
type PolicyRuleSpec struct {
	// APIGroups is the API groups the rule applies to (e.g. "", "apps", "batch").
	APIGroups []string `json:"apiGroups" yaml:"apiGroups"`

	// Resources is the Kubernetes resources the rule applies to (e.g. "pods", "deployments").
	Resources []string `json:"resources" yaml:"resources"`

	// Verbs is the operations allowed (e.g. "get", "list", "watch", "create", "update", "delete", "*").
	Verbs []string `json:"verbs" yaml:"verbs"`
}

// ObservabilitySpec declares how the platform should integrate this tool into monitoring.
type ObservabilitySpec struct {
	// Metrics describes how to scrape metrics from this tool.
	// +optional
	Metrics *MetricsSpec `json:"metrics,omitempty" yaml:"metrics,omitempty"`

	// DashboardPath is a path (relative to the archive root) to a Grafana dashboard JSON file.
	// The platform will auto-provision this dashboard into the environment's Grafana instance.
	// Example: "./dashboards/main-metrics.json"
	// +optional
	DashboardPath string `json:"dashboard_path,omitempty" yaml:"dashboard_path,omitempty"`
}

// MetricsSpec describes how to scrape Prometheus metrics from the tool.
type MetricsSpec struct {
	// Port is the container port where metrics are exposed (e.g. 9090).
	Port int `json:"port" yaml:"port"`

	// Path is the HTTP path for the metrics endpoint (e.g. "/metrics").
	Path string `json:"path" yaml:"path"`
}

// ToPolicyRules converts PlatformManifest permissions into paapv1-compatible PolicyRule JSON.
// This is used by the handler to create the ServiceInstance CR with the correct WorkloadRole.
func (m *PlatformManifest) ToWorkloadRoleJSON() string {
	if m.Permissions.Scope != PermissionScopeEnvironmentWide || len(m.Permissions.Rules) == 0 {
		return "[]" // No workload permissions needed
	}
	rules := make([]map[string]interface{}, 0, len(m.Permissions.Rules))
	for _, r := range m.Permissions.Rules {
		rules = append(rules, map[string]interface{}{
			"apiGroups": r.APIGroups,
			"resources": r.Resources,
			"verbs":     r.Verbs,
		})
	}
	b, _ := json.Marshal(rules)
	return string(b)
}

// Validate checks the PlatformManifest for common errors.
func (m *PlatformManifest) Validate() error {
	if m.Name == "" {
		return &ValidationError{Field: "name", Message: "name is required"}
	}
	if m.Version == "" {
		return &ValidationError{Field: "version", Message: "version is required"}
	}
	if m.Permissions.Scope == "" {
		return &ValidationError{Field: "permissions.scope", Message: "permissions.scope is required (tool-only or environment-wide)"}
	}
	if m.Permissions.Scope == PermissionScopeEnvironmentWide && len(m.Permissions.Rules) == 0 {
		return &ValidationError{Field: "permissions.rules", Message: "permissions.rules is required when scope is environment-wide"}
	}
	return nil
}

// BuildHelmValues merges platform context variables with user-defined variable_mapping.
// platformVars contains the platform's built-in context (e.g. global.envNamespaces).
// The manifest's variable_mapping entries are resolved by looking up platformVars[PlatformVar]
// and setting the result at the HelmVar key.
func (m *PlatformManifest) BuildHelmValues(platformVars map[string]string) map[string]string {
	result := make(map[string]string)
	// Start with platform context
	for k, v := range platformVars {
		result[k] = v
	}
	// Apply user-defined variable mappings
	for _, vm := range m.VariableMapping {
		if val, ok := platformVars[vm.PlatformVar]; ok {
			result[vm.HelmVar] = val
		}
	}
	return result
}

// ValidationError represents a validation error in the platform manifest.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return "validation error on field '" + e.Field + "': " + e.Message
}
