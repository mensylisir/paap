package model

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPlatformManifestRoleJSONSeparatesToolWorkloadAndEnvironmentPermissions(t *testing.T) {
	manifest := PlatformManifest{
		Name:    "argocd",
		Version: "v2.13.3",
		Permissions: PermissionsSpec{
			ClusterResources: ClusterResourcePermissionsSpec{
				Rules: []PolicyRuleSpec{
					{APIGroups: []string{"apiregistration.k8s.io"}, Resources: []string{"apiservices"}, Verbs: []string{"get", "list", "watch"}},
				},
			},
			ToolNamespace: NamespacePermissionsSpec{
				Rules: []PolicyRuleSpec{
					{APIGroups: []string{"argoproj.io"}, Resources: []string{"applications"}, Verbs: []string{"get", "list"}},
				},
			},
			WorkloadNamespaces: NamespacePermissionsSpec{
				Rules: []PolicyRuleSpec{
					{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
				},
			},
			EnvironmentNamespaces: NamespacePermissionsSpec{
				Rules: []PolicyRuleSpec{
					{APIGroups: []string{""}, Resources: []string{"pods", "services"}, Verbs: []string{"get", "list", "watch"}},
				},
			},
		},
	}

	assertRoleJSON(t, manifest.ToToolNamespaceRoleJSON(), []map[string]interface{}{
		{"apiGroups": []interface{}{"argoproj.io"}, "resources": []interface{}{"applications"}, "verbs": []interface{}{"get", "list"}},
	})
	assertRoleJSON(t, manifest.ToWorkloadRoleJSON(), []map[string]interface{}{
		{"apiGroups": []interface{}{"*"}, "resources": []interface{}{"*"}, "verbs": []interface{}{"*"}},
	})
	assertRoleJSON(t, manifest.ToEnvironmentRoleJSON(), []map[string]interface{}{
		{"apiGroups": []interface{}{""}, "resources": []interface{}{"pods", "services"}, "verbs": []interface{}{"get", "list", "watch"}},
	})
	assertRoleJSON(t, manifest.ToClusterRoleJSON(), []map[string]interface{}{
		{"apiGroups": []interface{}{"apiregistration.k8s.io"}, "resources": []interface{}{"apiservices"}, "verbs": []interface{}{"get", "list", "watch"}},
	})
}

func TestPlatformManifestValidateAllowsToolOnlyWithToolNamespaceRules(t *testing.T) {
	manifest := PlatformManifest{
		Name:    "harbor",
		Version: "v2.12.0",
		Permissions: PermissionsSpec{
			ToolNamespace: NamespacePermissionsSpec{
				Rules: []PolicyRuleSpec{
					{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}},
				},
			},
		},
	}

	if err := manifest.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestPlatformManifestValidateRejectsLegacyPermissionShape(t *testing.T) {
	var manifest PlatformManifest
	err := yaml.Unmarshal([]byte(`
name: legacy
version: v1
permissions:
  scope: environment-wide
  rules:
    - apiGroups: [""]
      resources: ["pods"]
      verbs: ["get"]
`), &manifest)
	if err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate() succeeded for legacy permissions shape")
	}
}

func TestPlatformManifestValidateRejectsLegacyScopedNamespacePermissions(t *testing.T) {
	var manifest PlatformManifest
	err := yaml.Unmarshal([]byte(`
name: legacy-monitor
version: v1
permissions:
  toolNamespace:
    rules:
      - apiGroups: [""]
        resources: ["pods"]
        verbs: ["get"]
  workloadNamespaces:
    scope: environment-wide
    rules:
      - apiGroups: [""]
        resources: ["pods"]
        verbs: ["get", "list", "watch"]
`), &manifest)
	if err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	err = manifest.Validate()
	if err == nil {
		t.Fatal("Validate() succeeded for legacy workloadNamespaces.scope")
	}
	if !strings.Contains(err.Error(), "workloadNamespaces.scope") {
		t.Fatalf("Validate() error = %v, want workloadNamespaces.scope guidance", err)
	}
}

func TestPlatformManifestValidateRejectsWritableClusterResourceRules(t *testing.T) {
	manifest := PlatformManifest{
		Name:    "unsafe-monitor",
		Version: "v1",
		Permissions: PermissionsSpec{
			ClusterResources: ClusterResourcePermissionsSpec{
				Rules: []PolicyRuleSpec{
					{APIGroups: []string{""}, Resources: []string{"nodes"}, Verbs: []string{"get", "list", "watch", "delete"}},
				},
			},
		},
	}

	err := manifest.Validate()
	if err == nil {
		t.Fatal("Validate() succeeded for writable clusterResources rule")
	}
	if !strings.Contains(err.Error(), "clusterResources") || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("Validate() error = %v, want clusterResources read-only error", err)
	}
}

func TestPlatformManifestValidateRejectsNamespacedResourcesInClusterResourceRules(t *testing.T) {
	manifest := PlatformManifest{
		Name:    "unsafe-monitor",
		Version: "v1",
		Permissions: PermissionsSpec{
			ClusterResources: ClusterResourcePermissionsSpec{
				Rules: []PolicyRuleSpec{
					{APIGroups: []string{""}, Resources: []string{"pods", "secrets"}, Verbs: []string{"get", "list", "watch"}},
				},
			},
		},
	}

	err := manifest.Validate()
	if err == nil {
		t.Fatal("Validate() succeeded for namespaced resources in clusterResources")
	}
	if !strings.Contains(err.Error(), "workloadNamespaces") {
		t.Fatalf("Validate() error = %v, want workloadNamespaces guidance", err)
	}
}

func assertRoleJSON(t *testing.T, got string, want []map[string]interface{}) {
	t.Helper()
	var gotRules []map[string]interface{}
	if err := json.Unmarshal([]byte(got), &gotRules); err != nil {
		t.Fatalf("role JSON did not unmarshal: %v", err)
	}
	if len(gotRules) != len(want) {
		t.Fatalf("got %d rules, want %d: %s", len(gotRules), len(want), got)
	}
	for i := range want {
		for key, wantValue := range want[i] {
			gotValue, ok := gotRules[i][key]
			if !ok {
				t.Fatalf("rule %d missing key %q in %v", i, key, gotRules[i])
			}
			if !jsonValuesEqual(gotValue, wantValue) {
				t.Fatalf("rule %d key %q = %#v, want %#v", i, key, gotValue, wantValue)
			}
		}
	}
}

func jsonValuesEqual(a, b interface{}) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
