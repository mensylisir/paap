package model

import "encoding/json"

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
	// Scope and Rules are rejected legacy fields from the former two-scope model.
	Scope string           `json:"scope,omitempty" yaml:"scope,omitempty"`
	Rules []PolicyRuleSpec `json:"rules,omitempty" yaml:"rules,omitempty"`

	// ClusterResources declares cluster-scoped read-only permissions the tool needs.
	// These permissions are applied as a ClusterRoleBinding to the tool's ServiceAccount.
	ClusterResources ClusterResourcePermissionsSpec `json:"clusterResources,omitempty" yaml:"clusterResources,omitempty"`

	// ToolNamespace declares the permissions the tool needs inside its own namespace.
	// These rules are bound only in the tool namespace.
	ToolNamespace NamespacePermissionsSpec `json:"toolNamespace,omitempty" yaml:"toolNamespace,omitempty"`

	// WorkloadNamespaces declares whether and how permissions are projected into
	// the environment's workload namespaces.
	WorkloadNamespaces NamespacePermissionsSpec `json:"workloadNamespaces,omitempty" yaml:"workloadNamespaces,omitempty"`

	// EnvironmentNamespaces declares permissions projected into all other namespaces
	// owned by the same environment, such as middleware and tool namespaces.
	EnvironmentNamespaces NamespacePermissionsSpec `json:"environmentNamespaces,omitempty" yaml:"environmentNamespaces,omitempty"`
}

// NamespacePermissionsSpec declares Role rules for a single namespace class.
type NamespacePermissionsSpec struct {
	// Scope is a rejected legacy field. Namespace permission classes are now
	// explicit: toolNamespace, workloadNamespaces, and environmentNamespaces.
	Scope string           `json:"scope,omitempty" yaml:"scope,omitempty"`
	Rules []PolicyRuleSpec `json:"rules,omitempty" yaml:"rules,omitempty"`
}

// ClusterResourcePermissionsSpec declares ClusterRole rules for cluster-scoped read-only access.
type ClusterResourcePermissionsSpec struct {
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
	rules := m.Permissions.WorkloadNamespaces.Rules
	if len(rules) == 0 {
		return "[]" // No workload permissions needed
	}
	return policyRulesToJSON(rules)
}

// ToEnvironmentRoleJSON converts environment namespace permissions into JSON.
func (m *PlatformManifest) ToEnvironmentRoleJSON() string {
	return policyRulesToJSON(m.Permissions.EnvironmentNamespaces.Rules)
}

// ToToolNamespaceRoleJSON converts tool-namespace permissions into JSON.
func (m *PlatformManifest) ToToolNamespaceRoleJSON() string {
	return policyRulesToJSON(m.Permissions.ToolNamespace.Rules)
}

// ToClusterRoleJSON converts cluster-scoped permissions into JSON.
func (m *PlatformManifest) ToClusterRoleJSON() string {
	return policyRulesToJSON(m.Permissions.ClusterResources.Rules)
}

func policyRulesToJSON(policyRules []PolicyRuleSpec) string {
	if len(policyRules) == 0 {
		return "[]"
	}
	rules := make([]map[string]interface{}, 0, len(policyRules))
	for _, r := range policyRules {
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
	if m.Permissions.Scope != "" || len(m.Permissions.Rules) > 0 {
		return &ValidationError{Field: "permissions.scope", Message: "legacy permissions.scope/rules are no longer supported; use toolNamespace, workloadNamespaces, and environmentNamespaces"}
	}
	if err := validateNamespacePermissionClass("permissions.toolNamespace", m.Permissions.ToolNamespace); err != nil {
		return err
	}
	if err := validateNamespacePermissionClass("permissions.workloadNamespaces", m.Permissions.WorkloadNamespaces); err != nil {
		return err
	}
	if err := validateNamespacePermissionClass("permissions.environmentNamespaces", m.Permissions.EnvironmentNamespaces); err != nil {
		return err
	}
	if err := validateClusterResourceRules(m.Permissions.ClusterResources.Rules); err != nil {
		return err
	}
	return nil
}

func validateNamespacePermissionClass(field string, spec NamespacePermissionsSpec) error {
	if spec.Scope != "" {
		return &ValidationError{Field: field + ".scope", Message: "scope is no longer supported; declare rules directly under toolNamespace, workloadNamespaces, or environmentNamespaces"}
	}
	for ruleIndex, rule := range spec.Rules {
		if len(rule.APIGroups) == 0 || len(rule.Resources) == 0 || len(rule.Verbs) == 0 {
			return &ValidationError{
				Field:   field + ".rules",
				Message: field + " rule " + jsonNumber(ruleIndex) + " must include apiGroups, resources, and verbs",
			}
		}
	}
	return nil
}

func validateClusterResourceRules(rules []PolicyRuleSpec) error {
	for ruleIndex, rule := range rules {
		for _, verb := range rule.Verbs {
			if !readOnlyClusterResourceVerbs[verb] {
				return &ValidationError{
					Field:   "permissions.clusterResources.rules",
					Message: "clusterResources rules must be read-only (get, list, watch)",
				}
			}
		}
		for _, apiGroup := range rule.APIGroups {
			allowedResources, ok := clusterScopedResources[apiGroup]
			if !ok {
				return &ValidationError{
					Field:   "permissions.clusterResources.rules",
					Message: "clusterResources rule references an unsupported API group; put namespaced permissions under workloadNamespaces or environmentNamespaces",
				}
			}
			for _, resource := range rule.Resources {
				if !allowedResources[resource] {
					return &ValidationError{
						Field:   "permissions.clusterResources.rules",
						Message: "clusterResources rule references namespaced or unsupported resource " + resource + "; put namespaced permissions under workloadNamespaces or environmentNamespaces",
					}
				}
			}
		}
		if len(rule.APIGroups) == 0 || len(rule.Resources) == 0 || len(rule.Verbs) == 0 {
			return &ValidationError{
				Field:   "permissions.clusterResources.rules",
				Message: "clusterResources rule " + jsonNumber(ruleIndex) + " must include apiGroups, resources, and verbs",
			}
		}
	}
	return nil
}

var readOnlyClusterResourceVerbs = map[string]bool{
	"get":   true,
	"list":  true,
	"watch": true,
}

var clusterScopedResources = map[string]map[string]bool{
	"": {
		"namespaces":        true,
		"nodes":             true,
		"nodes/metrics":     true,
		"nodes/proxy":       true,
		"nodes/stats":       true,
		"persistentvolumes": true,
	},
	"apiregistration.k8s.io": {
		"apiservices": true,
	},
	"certificates.k8s.io": {
		"certificatesigningrequests": true,
	},
	"storage.k8s.io": {
		"storageclasses":       true,
		"volumeattachments":    true,
		"csidrivers":           true,
		"csinodes":             true,
		"csistoragecapacities": true,
	},
}

func jsonNumber(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
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
