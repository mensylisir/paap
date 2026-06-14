package model

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDocsExamplesPlatformManifestsUseExplicitNamespacePermissionTypes(t *testing.T) {
	paths := findExamplePlatformManifests(t)
	if len(paths) == 0 {
		t.Fatal("expected docs/examples platform manifests")
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read platform manifest: %v", err)
			}
			if strings.Contains(string(data), "scope:") {
				t.Fatalf("platform manifest must use explicit toolNamespace/workloadNamespaces/environmentNamespaces sections, not legacy scope:\n%s", path)
			}
			var manifest PlatformManifest
			if err := yaml.Unmarshal(data, &manifest); err != nil {
				t.Fatalf("unmarshal platform manifest: %v", err)
			}
			if err := manifest.Validate(); err != nil {
				t.Fatalf("validate platform manifest: %v", err)
			}
		})
	}
}

func findExamplePlatformManifests(t *testing.T) []string {
	t.Helper()
	var paths []string
	root := "../../docs/examples"
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == "platform-manifest.yaml" {
			paths = append(paths, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return paths
}

func TestArgoCDPresetValuesKeepRepoServerRBACAsList(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/argocd/preset-values.yaml")
	if err != nil {
		t.Fatalf("read argocd preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal argocd preset values: %v", err)
	}
	repoServer, ok := values["repoServer"].(map[string]interface{})
	if !ok {
		t.Fatalf("repoServer values missing or wrong type: %#v", values["repoServer"])
	}
	if _, ok := repoServer["rbac"].([]interface{}); !ok {
		t.Fatalf("repoServer.rbac must stay a list because the upstream chart expects a list, got %#v", repoServer["rbac"])
	}
}

func TestArgoCDPresetValuesEnableApplicationSetWithoutClusterScope(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/argocd/preset-values.yaml")
	if err != nil {
		t.Fatalf("read argocd preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal argocd preset values: %v", err)
	}

	if got := values["createClusterRoles"]; got != false {
		t.Fatalf("argocd createClusterRoles must stay false so ArgoCD does not get cluster-wide workload permissions, got %#v", got)
	}
	if got := values["createAggregateRoles"]; got != false {
		t.Fatalf("argocd createAggregateRoles must stay false so ArgoCD does not extend cluster roles, got %#v", got)
	}

	applicationSet, ok := values["applicationSet"].(map[string]interface{})
	if !ok {
		t.Fatalf("applicationSet values missing or wrong type: %#v", values["applicationSet"])
	}
	replicas, ok := applicationSet["replicas"].(int)
	if !ok {
		t.Fatalf("applicationSet.replicas missing or wrong type: %#v", applicationSet["replicas"])
	}
	if replicas < 1 {
		t.Fatalf("applicationSet.replicas must be at least 1 so ApplicationSet reconciliation is available, got %d", replicas)
	}
	if got := applicationSet["allowAnyNamespace"]; got != false {
		t.Fatalf("applicationSet.allowAnyNamespace must stay false so the chart does not create cluster-scoped ApplicationSet RBAC, got %#v", got)
	}
	rbac, ok := applicationSet["rbac"].(map[string]interface{})
	if !ok {
		t.Fatalf("applicationSet.rbac values missing or wrong type: %#v", applicationSet["rbac"])
	}
	if got := rbac["create"]; got != false {
		t.Fatalf("applicationSet.rbac.create must stay false because platform-manifest.yaml owns namespaced RBAC, got %#v", got)
	}
}

func TestArgoCDPlatformManifestKeepsApplicationSetNamespaced(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/argocd/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read argocd platform manifest: %v", err)
	}

	var manifest PlatformManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal argocd platform manifest: %v", err)
	}

	if len(manifest.Permissions.ClusterResources.Rules) != 0 {
		t.Fatalf("argocd must not request clusterResources; runtime RBAC is projected only into environment namespaces, got %#v", manifest.Permissions.ClusterResources.Rules)
	}
	if !manifestHasRuleForResource(manifest.Permissions.WorkloadNamespaces.Rules, "*", "*") {
		t.Fatalf("argocd workloadNamespaces must allow all resources inside the environment namespace boundary, got %#v", manifest.Permissions.WorkloadNamespaces.Rules)
	}
	for _, want := range []string{
		"applicationsets",
		"applicationsets/status",
		"applicationsets/finalizers",
	} {
		if !manifestHasRuleForResource(manifest.Permissions.ToolNamespace.Rules, "argoproj.io", want) {
			t.Fatalf("argocd toolNamespace role must include argoproj.io/%s for the namespaced ApplicationSet controller", want)
		}
	}
	if !manifestHasWriteRuleForResource(manifest.Permissions.ToolNamespace.Rules, "argoproj.io", "appprojects") {
		t.Fatalf("argocd toolNamespace role must allow namespaced AppProject writes so argocd-server can bootstrap its default project")
	}
}

func TestArgoCDPresetValuesLimitClusterCacheResources(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/argocd/preset-values.yaml")
	if err != nil {
		t.Fatalf("read argocd preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal argocd preset values: %v", err)
	}
	configs, ok := values["configs"].(map[string]interface{})
	if !ok {
		t.Fatalf("configs values missing or wrong type: %#v", values["configs"])
	}
	cm, ok := configs["cm"].(map[string]interface{})
	if !ok {
		t.Fatalf("configs.cm values missing or wrong type: %#v", configs["cm"])
	}
	inclusions, ok := cm["resource.inclusions"].(string)
	if !ok || strings.TrimSpace(inclusions) == "" {
		t.Fatalf("configs.cm.resource.inclusions must limit ArgoCD cluster cache, got %#v", cm["resource.inclusions"])
	}
	for _, required := range []string{"ConfigMap", "Secret"} {
		if !strings.Contains(inclusions, "- "+required) {
			t.Fatalf("argocd resource.inclusions must include %s so component config resources can sync:\n%s", required, inclusions)
		}
	}
	for _, forbidden := range []string{"Environment", "ServiceInstance", "ResourceClaimTemplate", "NetworkPolicy", "ReplicationController", "DaemonSet"} {
		if strings.Contains(inclusions, "- "+forbidden) {
			t.Fatalf("argocd resource.inclusions must not include %s in cluster cache:\n%s", forbidden, inclusions)
		}
	}
}

func TestArgoCDPlatformManifestInjectsNamespaceScopedControllerParams(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/argocd/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read argocd platform manifest: %v", err)
	}

	var manifest PlatformManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal argocd platform manifest: %v", err)
	}

	mapped := manifest.BuildHelmValues(map[string]string{
		"env_namespaces":      "test-staging,test-staging-app,test-staging-monitor",
		"workload_namespaces": "test-staging,test-staging-app",
	})
	for _, key := range []string{
		"configs.params.application.namespaces",
		"configs.params.applicationsetcontroller.namespaces",
	} {
		if got := mapped[key]; got != "test-staging,test-staging-app" {
			t.Fatalf("argocd manifest must map workload_namespaces to %s, got %q", key, got)
		}
	}
}

func TestBuiltInToolPresetsExposeUserFacingServices(t *testing.T) {
	type presetCheck struct {
		name     string
		path     string
		selector []string
	}
	checks := []presetCheck{
		{name: "gitea", path: "../../docs/examples/built-in-templates/gitea/preset-values.yaml", selector: []string{"service"}},
		{name: "argocd", path: "../../docs/examples/built-in-templates/argocd/preset-values.yaml", selector: []string{"server", "service"}},
		{name: "monitor", path: "../../docs/examples/built-in-templates/monitor/preset-values.yaml", selector: []string{"grafana", "service"}},
		{name: "loki", path: "../../docs/examples/built-in-templates/loki/preset-values.yaml", selector: []string{"loki", "service"}},
	}

	for _, check := range checks {
		data, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s preset values: %v", check.name, err)
		}
		var values map[string]interface{}
		if err := yaml.Unmarshal(data, &values); err != nil {
			t.Fatalf("unmarshal %s preset values: %v", check.name, err)
		}
		current := interface{}(values)
		for _, key := range check.selector {
			next, ok := current.(map[string]interface{})[key]
			if !ok {
				t.Fatalf("%s preset missing %s in selector %v", check.name, key, check.selector)
			}
			current = next
		}
		service, ok := current.(map[string]interface{})
		if !ok {
			t.Fatalf("%s selected service values wrong type: %#v", check.name, current)
		}
		if got := service["type"]; got != "NodePort" {
			t.Fatalf("%s user-facing service must default to NodePort for local kind access, got %#v", check.name, got)
		}
		if check.name == "argocd" {
			if service["nodePortHttp"] != nil || service["nodePortHttps"] != nil {
				t.Fatalf("argocd preset must leave nodePortHttp/nodePortHttps empty so multiple environments do not collide, got %#v", service)
			}
		}
	}
}

func TestBuiltInMiddlewarePresetsDisableChartSpecificPersistenceByDefault(t *testing.T) {
	checks := map[string][]string{
		"../../docs/examples/built-in-templates/redis/preset-values.yaml": {
			"master.persistence.enabled",
			"replica.persistence.enabled",
			"sentinel.persistence.enabled",
		},
		"../../docs/examples/built-in-templates/mysql/preset-values.yaml": {
			"primary.persistence.enabled",
			"secondary.persistence.enabled",
		},
		"../../docs/examples/built-in-templates/postgresql/preset-values.yaml": {
			"primary.persistence.enabled",
			"readReplicas.persistence.enabled",
		},
		"../../docs/examples/built-in-templates/kafka/preset-values.yaml": {
			"controller.persistence.enabled",
			"broker.persistence.enabled",
			"zookeeper.persistence.enabled",
		},
	}

	for path, keys := range checks {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read preset values: %v", err)
			}
			var values map[string]interface{}
			if err := yaml.Unmarshal(data, &values); err != nil {
				t.Fatalf("unmarshal preset values: %v", err)
			}
			for _, key := range keys {
				if got := nestedBool(values, key); got != false {
					t.Fatalf("%s must default to false in %s, got %v", key, path, got)
				}
			}
		})
	}
}

func TestBuiltInMiddlewarePresetsUsePlatformServiceAccounts(t *testing.T) {
	paths := []string{
		"../../docs/examples/built-in-templates/redis/preset-values.yaml",
		"../../docs/examples/built-in-templates/postgresql/preset-values.yaml",
		"../../docs/examples/built-in-templates/mysql/preset-values.yaml",
		"../../docs/examples/built-in-templates/rabbitmq/preset-values.yaml",
		"../../docs/examples/built-in-templates/mongodb/preset-values.yaml",
		"../../docs/examples/built-in-templates/kafka/preset-values.yaml",
		"../../docs/examples/built-in-templates/minio/preset-values.yaml",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read preset values: %v", err)
			}
			var values map[string]interface{}
			if err := yaml.Unmarshal(data, &values); err != nil {
				t.Fatalf("unmarshal preset values: %v", err)
			}
			if got := nestedBool(values, "serviceAccount.create"); got != false {
				t.Fatalf("serviceAccount.create must be false in %s because PAAP creates the ServiceAccount and RBAC, got %v", path, got)
			}
		})
	}
}

func TestBuiltInDatabaseTemplatesUseResolvableLegacyBitnamiImages(t *testing.T) {
	checks := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "postgresql values",
			path: "../../docs/examples/built-in-templates/postgresql/chart/values.yaml",
			want: []string{
				"repository: bitnamilegacy/postgresql",
				"repository: bitnamilegacy/postgres-exporter",
			},
		},
		{
			name: "postgresql chart metadata",
			path: "../../docs/examples/built-in-templates/postgresql/chart/Chart.yaml",
			want: []string{
				"image: docker.io/bitnamilegacy/postgresql:15.4.0-debian-11-r45",
				"image: docker.io/bitnamilegacy/postgres-exporter:0.14.0-debian-11-r2",
			},
		},
		{
			name: "redis values",
			path: "../../docs/examples/built-in-templates/redis/chart/values.yaml",
			want: []string{
				"repository: bitnamilegacy/redis",
				"repository: bitnamilegacy/redis-sentinel",
				"repository: bitnamilegacy/redis-exporter",
			},
		},
		{
			name: "redis chart metadata",
			path: "../../docs/examples/built-in-templates/redis/chart/Chart.yaml",
			want: []string{
				"image: docker.io/bitnamilegacy/redis:7.2.3-debian-11-r1",
				"image: docker.io/bitnamilegacy/redis-sentinel:7.2.3-debian-11-r1",
				"image: docker.io/bitnamilegacy/redis-exporter:1.55.0-debian-11-r2",
			},
		},
	}

	for _, check := range checks {
		data, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.name, err)
		}
		text := string(data)
		for _, want := range check.want {
			if !strings.Contains(text, want) {
				t.Fatalf("%s must contain %q", check.name, want)
			}
		}
		if strings.Contains(text, "repository: bitnami/postgresql") ||
			strings.Contains(text, "repository: bitnami/postgres-exporter") ||
			strings.Contains(text, "repository: bitnami/redis") ||
			strings.Contains(text, "repository: bitnami/redis-sentinel") ||
			strings.Contains(text, "repository: bitnami/redis-exporter") ||
			strings.Contains(text, "image: docker.io/bitnami/postgresql:") ||
			strings.Contains(text, "image: docker.io/bitnami/postgres-exporter:") ||
			strings.Contains(text, "image: docker.io/bitnami/redis:") ||
			strings.Contains(text, "image: docker.io/bitnami/redis-sentinel:") ||
			strings.Contains(text, "image: docker.io/bitnami/redis-exporter:") {
			t.Fatalf("%s still references deprecated bitnami image repositories", check.name)
		}
	}
}

func TestLokiPresetRaisesPromtailInotifyLimits(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/loki/preset-values.yaml")
	if err != nil {
		t.Fatalf("read loki preset values: %v", err)
	}
	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal loki preset values: %v", err)
	}
	promtail, ok := values["promtail"].(map[string]interface{})
	if !ok {
		t.Fatalf("promtail values missing or wrong type: %#v", values["promtail"])
	}
	initContainers, ok := promtail["initContainer"].([]interface{})
	if !ok || len(initContainers) == 0 {
		t.Fatalf("promtail must include an initContainer that raises inotify limits for local kind and single-node clusters, got %#v", promtail["initContainer"])
	}
	rendered, err := yaml.Marshal(initContainers)
	if err != nil {
		t.Fatalf("marshal promtail initContainer: %v", err)
	}
	text := string(rendered)
	for _, want := range []string{
		"raise-inotify-limits",
		"privileged: true",
		"fs.inotify.max_user_watches=1048576",
		"fs.inotify.max_user_instances=1024",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("promtail inotify initContainer missing %q:\n%s", want, text)
		}
	}
	config, ok := promtail["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("promtail config missing or wrong type: %#v", promtail["config"])
	}
	clients, ok := config["clients"].([]interface{})
	if !ok || len(clients) == 0 {
		t.Fatalf("promtail clients missing or wrong type: %#v", config["clients"])
	}
	firstClient, ok := clients[0].(map[string]interface{})
	if !ok {
		t.Fatalf("promtail client missing or wrong type: %#v", clients[0])
	}
	if got, want := firstClient["url"], "http://{{ .Release.Name }}:3100/loki/api/v1/push"; got != want {
		t.Fatalf("promtail must push to the actual Loki service name, got %q want %q", got, want)
	}
}

func TestArgoCDChartRolesLeaveAppProjectWritesToPlatformRole(t *testing.T) {
	for _, path := range []string{
		"../../docs/examples/built-in-templates/argocd/chart/templates/argocd-server/role.yaml",
		"../../docs/examples/built-in-templates/argocd/chart/templates/argocd-application-controller/role.yaml",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		if templateRoleWritesResource(text, "appprojects") {
			t.Fatalf("%s must not grant write verbs on appprojects; PAAP platform manifest owns the scoped AppProject write role:\n%s", path, text)
		}
	}
}

func TestMonitorPlatformManifestKeepsClusterScopedResourcesOutOfToolNamespaceRole(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read monitor platform manifest: %v", err)
	}

	var manifest PlatformManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal monitor platform manifest: %v", err)
	}

	clusterScoped := map[string]map[string]bool{
		"": {
			"nodes":             true,
			"nodes/proxy":       true,
			"nodes/metrics":     true,
			"nodes/stats":       true,
			"namespaces":        true,
			"persistentvolumes": true,
		},
		"certificates.k8s.io": {
			"certificatesigningrequests": true,
		},
		"storage.k8s.io": {
			"storageclasses":    true,
			"volumeattachments": true,
		},
	}

	for _, rule := range manifest.Permissions.ToolNamespace.Rules {
		for _, group := range rule.APIGroups {
			for _, resource := range rule.Resources {
				if clusterScoped[group][resource] {
					t.Fatalf("cluster-scoped resource %s/%s must not be in toolNamespace role", group, resource)
				}
			}
		}
	}

	for group, resources := range clusterScoped {
		for resource := range resources {
			if !manifestHasRuleForResource(manifest.Permissions.ClusterResources.Rules, group, resource) {
				t.Fatalf("cluster-scoped resource %s/%s must be declared under clusterResources", group, resource)
			}
		}
	}
}

func TestMonitorPlatformManifestUsesEnvironmentNamespacesForEnvironmentWideRBAC(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read monitor platform manifest: %v", err)
	}

	var manifest PlatformManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal monitor platform manifest: %v", err)
	}

	if len(manifest.Permissions.WorkloadNamespaces.Rules) != 0 {
		t.Fatalf("monitor must not put environment-wide access in workloadNamespaces, got %#v", manifest.Permissions.WorkloadNamespaces.Rules)
	}
	if !manifestHasRuleForResource(manifest.Permissions.EnvironmentNamespaces.Rules, "*", "*") {
		t.Fatalf("monitor environmentNamespaces must allow all resources inside the environment namespace boundary, got %#v", manifest.Permissions.EnvironmentNamespaces.Rules)
	}
}

func TestMonitorPresetValuesEnableNamespaceCollector(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/preset-values.yaml")
	if err != nil {
		t.Fatalf("read monitor preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal monitor preset values: %v", err)
	}
	ksm, ok := values["kube-state-metrics"].(map[string]interface{})
	if !ok {
		t.Fatalf("kube-state-metrics values missing or wrong type: %#v", values["kube-state-metrics"])
	}
	collectors, ok := ksm["collectors"].([]interface{})
	if !ok {
		t.Fatalf("kube-state-metrics.collectors missing or wrong type: %#v", ksm["collectors"])
	}
	for _, collector := range collectors {
		if collector == "namespaces" {
			return
		}
	}
	t.Fatalf("kube-state-metrics.collectors must include namespaces, got %#v", collectors)
}

func TestMonitorPlatformManifestAllowsPrometheusOperatorStatusControllersInToolNamespace(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read monitor platform manifest: %v", err)
	}

	var manifest PlatformManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal monitor platform manifest: %v", err)
	}

	for _, want := range []string{
		"alertmanagerconfigs",
		"alertmanagers/status",
		"prometheusagents",
		"prometheusagents/status",
		"prometheuses/status",
		"thanosrulers/status",
	} {
		if !manifestHasRuleForResource(manifest.Permissions.ToolNamespace.Rules, "monitoring.coreos.com", want) {
			t.Fatalf("monitor toolNamespace role must include monitoring.coreos.com/%s so prometheus-operator can start its namespaced controllers", want)
		}
	}
}

func TestMonitorPresetValuesScrapeKubeletCadvisorThroughAPIServerProxy(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/preset-values.yaml")
	if err != nil {
		t.Fatalf("read monitor preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal monitor preset values: %v", err)
	}

	operator, ok := values["prometheusOperator"].(map[string]interface{})
	if !ok {
		t.Fatalf("prometheusOperator values missing or wrong type: %#v", values["prometheusOperator"])
	}
	kubeletService, ok := operator["kubeletService"].(map[string]interface{})
	if !ok {
		t.Fatalf("prometheusOperator.kubeletService values missing or wrong type: %#v", operator["kubeletService"])
	}
	if kubeletService["enabled"] != false {
		t.Fatalf("prometheusOperator.kubeletService.enabled must stay false because this local monitor preset disables kubelet cluster scraping, got %#v", kubeletService["enabled"])
	}
	namespaces, ok := operator["namespaces"].(map[string]interface{})
	if !ok {
		t.Fatalf("prometheusOperator.namespaces values missing or wrong type: %#v", operator["namespaces"])
	}
	if namespaces["releaseNamespace"] != true {
		t.Fatalf("prometheusOperator.namespaces.releaseNamespace must stay true so the operator does not require cluster-wide monitoring.coreos.com RBAC, got %#v", namespaces["releaseNamespace"])
	}
	if operator["kubeletEndpointsEnabled"] != false {
		t.Fatalf("prometheusOperator.kubeletEndpointsEnabled must stay false so the operator does not need kube-system endpoints write access, got %#v", operator["kubeletEndpointsEnabled"])
	}
	if operator["kubeletEndpointSliceEnabled"] != false {
		t.Fatalf("prometheusOperator.kubeletEndpointSliceEnabled must stay false so the operator does not need kube-system endpointslice write access, got %#v", operator["kubeletEndpointSliceEnabled"])
	}

	prometheus, ok := values["prometheus"].(map[string]interface{})
	if !ok {
		t.Fatalf("prometheus values missing or wrong type: %#v", values["prometheus"])
	}
	spec, ok := prometheus["prometheusSpec"].(map[string]interface{})
	if !ok {
		t.Fatalf("prometheus.prometheusSpec values missing or wrong type: %#v", prometheus["prometheusSpec"])
	}
	scrapeConfig, ok := spec["additionalScrapeConfigs"].(string)
	if !ok {
		t.Fatalf("prometheus.prometheusSpec.additionalScrapeConfigs missing or wrong type: %#v", spec["additionalScrapeConfigs"])
	}
	for _, want := range []string{
		"job_name: paap-kubelet-cadvisor",
		"role: node",
		"replacement: kubernetes.default.svc:443",
		"replacement: /api/v1/nodes/${1}/proxy/metrics/cadvisor",
	} {
		if !strings.Contains(scrapeConfig, want) {
			t.Fatalf("monitor additionalScrapeConfigs must contain %q so cAdvisor container metrics are available:\n%s", want, scrapeConfig)
		}
	}
}

func TestMonitorPresetValuesKeepGrafanaSidecarsNamespaced(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/preset-values.yaml")
	if err != nil {
		t.Fatalf("read monitor preset values: %v", err)
	}
	manifestData, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read monitor platform manifest: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal monitor preset values: %v", err)
	}
	var manifest PlatformManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal monitor platform manifest: %v", err)
	}
	mapped := manifest.BuildHelmValues(map[string]string{"tool_namespace": "test-staging-monitor"})
	if got := mapped["paap.toolNamespace"]; got != "test-staging-monitor" {
		t.Fatalf("monitor manifest must map tool_namespace to paap.toolNamespace, got %q", got)
	}
	if got := mapped["global.paap.toolNamespace"]; got != "test-staging-monitor" {
		t.Fatalf("monitor manifest must map tool_namespace to global.paap.toolNamespace for Grafana subchart tpl, got %q", got)
	}
	grafana, ok := values["grafana"].(map[string]interface{})
	if !ok {
		t.Fatalf("grafana values missing or wrong type: %#v", values["grafana"])
	}
	sidecar, ok := grafana["sidecar"].(map[string]interface{})
	if !ok {
		t.Fatalf("grafana.sidecar values missing or wrong type: %#v", grafana["sidecar"])
	}
	for _, name := range []string{"dashboards", "datasources"} {
		cfg, ok := sidecar[name].(map[string]interface{})
		if !ok {
			t.Fatalf("grafana.sidecar.%s values missing or wrong type: %#v", name, sidecar[name])
		}
		items, ok := cfg["searchNamespace"].([]interface{})
		if !ok || len(items) != 1 || items[0] != "{{ .Values.global.paap.toolNamespace }}" {
			t.Fatalf("grafana sidecar %s searchNamespace must template to global.paap.toolNamespace, got %#v", name, cfg["searchNamespace"])
		}
	}
}

func TestMonitorPlatformManifestMapsAllWorkloadServiceAccounts(t *testing.T) {
	manifestData, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read monitor platform manifest: %v", err)
	}

	var manifest PlatformManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal monitor platform manifest: %v", err)
	}
	mapped := manifest.BuildHelmValues(map[string]string{"tool_namespace": "test-staging-monitor"})

	for _, key := range []string{
		"prometheusOperator.serviceAccount.name",
		"prometheus.serviceAccount.name",
		"prometheus.prometheusSpec.serviceAccountName",
		"alertmanager.serviceAccount.name",
		"alertmanager.alertmanagerSpec.serviceAccountName",
		"grafana.serviceAccount.name",
		"kube-state-metrics.serviceAccount.name",
		"prometheus-node-exporter.serviceAccount.name",
	} {
		if got := mapped[key]; got != "test-staging-monitor" {
			t.Fatalf("monitor manifest must map tool_namespace to %s so chart workloads use the PAAP-managed service account, got %q", key, got)
		}
	}
}

func TestLokiPlatformManifestMapsAllWorkloadServiceAccounts(t *testing.T) {
	manifestData, err := os.ReadFile("../../docs/examples/built-in-templates/loki/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read loki platform manifest: %v", err)
	}

	var manifest PlatformManifest
	if err := yaml.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal loki platform manifest: %v", err)
	}
	mapped := manifest.BuildHelmValues(map[string]string{"tool_namespace": "test-staging-log"})

	for _, key := range []string{
		"serviceAccount.name",
		"loki.serviceAccount.name",
		"promtail.serviceAccount.name",
	} {
		if got := mapped[key]; got != "test-staging-log" {
			t.Fatalf("loki manifest must map tool_namespace to %s so chart workloads use the PAAP-managed service account, got %q", key, got)
		}
	}
}

func TestMonitorPresetValuesDisableStockGrafanaDashboards(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/preset-values.yaml")
	if err != nil {
		t.Fatalf("read monitor preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal monitor preset values: %v", err)
	}
	grafana, ok := values["grafana"].(map[string]interface{})
	if !ok {
		t.Fatalf("grafana values missing or wrong type: %#v", values["grafana"])
	}
	if got := grafana["defaultDashboardsEnabled"]; got != false {
		t.Fatalf("grafana.defaultDashboardsEnabled must be false so built-in monitor installs only PAAP dashboards, got %#v", got)
	}
	nodeExporter, ok := values["nodeExporter"].(map[string]interface{})
	if !ok {
		t.Fatalf("nodeExporter values missing or wrong type: %#v", values["nodeExporter"])
	}
	if got := nodeExporter["forceDeployDashboards"]; got != false {
		t.Fatalf("nodeExporter.forceDeployDashboards must be false so stock node dashboards are not rendered, got %#v", got)
	}
}

func TestMonitorPresetValuesAvoidNodeExporterHostPortConflict(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/preset-values.yaml")
	if err != nil {
		t.Fatalf("read monitor preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal monitor preset values: %v", err)
	}
	nodeExporter, ok := values["prometheus-node-exporter"].(map[string]interface{})
	if !ok {
		t.Fatalf("prometheus-node-exporter values missing or wrong type: %#v", values["prometheus-node-exporter"])
	}
	if got := nodeExporter["hostNetwork"]; got != false {
		t.Fatalf("node exporter hostNetwork must be false in multi-env kind clusters to avoid one-node host port conflicts, got %#v", got)
	}
	if got := nodeExporter["hostPID"]; got != false {
		t.Fatalf("node exporter hostPID must be false when hostNetwork is disabled, got %#v", got)
	}
}

func TestMonitorPresetValuesKeepAnnotatedPodScrapesToAnnotatedPort(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/monitor/preset-values.yaml")
	if err != nil {
		t.Fatalf("read monitor preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal monitor preset values: %v", err)
	}
	prometheus, ok := values["prometheus"].(map[string]interface{})
	if !ok {
		t.Fatalf("prometheus values missing or wrong type: %#v", values["prometheus"])
	}
	spec, ok := prometheus["prometheusSpec"].(map[string]interface{})
	if !ok {
		t.Fatalf("prometheus.prometheusSpec values missing or wrong type: %#v", prometheus["prometheusSpec"])
	}
	scrapeConfig, ok := spec["additionalScrapeConfigs"].(string)
	if !ok {
		t.Fatalf("prometheus.prometheusSpec.additionalScrapeConfigs missing or wrong type: %#v", spec["additionalScrapeConfigs"])
	}

	for _, want := range []string{
		"job_name: paap-annotated-pods-named-port",
		"job_name: paap-annotated-pods-numbered-port",
		"action: keepequal",
		"target_label: __meta_kubernetes_pod_container_port_name",
		"target_label: __meta_kubernetes_pod_container_port_number",
	} {
		if !strings.Contains(scrapeConfig, want) {
			t.Fatalf("monitor additionalScrapeConfigs must contain %q so named and numeric prometheus.io/port annotations only scrape the selected pod port:\n%s", want, scrapeConfig)
		}
	}
}

func TestJenkinsPlatformManifestAllowsKubernetesPluginExecIntoAgents(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/jenkins/platform-manifest.yaml")
	if err != nil {
		t.Fatalf("read jenkins platform manifest: %v", err)
	}

	var manifest PlatformManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal jenkins platform manifest: %v", err)
	}

	if !manifestHasRuleForResource(manifest.Permissions.ToolNamespace.Rules, "", "pods/exec") {
		t.Fatalf("jenkins toolNamespace role must include pods/exec so the Kubernetes plugin can run steps inside agent containers")
	}
}

func TestJenkinsPresetValuesKeepOfflineKindInstallReproducible(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/jenkins/preset-values.yaml")
	if err != nil {
		t.Fatalf("read jenkins preset values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal jenkins preset values: %v", err)
	}
	controller, ok := values["controller"].(map[string]interface{})
	if !ok {
		t.Fatalf("controller values missing or wrong type: %#v", values["controller"])
	}
	if controller["installLatestPlugins"] != false {
		t.Fatalf("controller.installLatestPlugins must stay false to avoid pulling Jenkins plugins incompatible with the pinned core, got %#v", controller["installLatestPlugins"])
	}
	if controller["installLatestSpecifiedPlugins"] != false {
		t.Fatalf("controller.installLatestSpecifiedPlugins must stay false to keep plugin resolution reproducible, got %#v", controller["installLatestSpecifiedPlugins"])
	}
	if controller["javaOpts"] != "-Xms256m -Xmx1024m" {
		t.Fatalf("controller.javaOpts must cap Jenkins heap for the local kind environment, got %#v", controller["javaOpts"])
	}
}

func TestJenkinsChartPinsPipelineDependenciesForPinnedCore(t *testing.T) {
	data, err := os.ReadFile("../../docs/examples/built-in-templates/jenkins/chart/values.yaml")
	if err != nil {
		t.Fatalf("read jenkins chart values: %v", err)
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("unmarshal jenkins chart values: %v", err)
	}
	controller, ok := values["controller"].(map[string]interface{})
	if !ok {
		t.Fatalf("controller values missing or wrong type: %#v", values["controller"])
	}
	plugins, ok := controller["installPlugins"].([]interface{})
	if !ok {
		t.Fatalf("controller.installPlugins must be a list, got %#v", controller["installPlugins"])
	}

	for _, want := range []string{
		"script-security:1265.va_fb_290b_4b_d34",
		"workflow-scm-step:415.v434365564324",
	} {
		if !containsInterfaceString(plugins, want) {
			t.Fatalf("controller.installPlugins must pin %s so workflow-cps loads on Jenkins 2.414.3, got %#v", want, plugins)
		}
	}
}

func TestPAAPServerRBACAllowsKpackBootstrap(t *testing.T) {
	data, err := os.ReadFile("../../deploy/k8s/paap-server.yaml")
	if err != nil {
		t.Fatalf("read paap server manifest: %v", err)
	}

	role := readClusterRoleFromManifest(t, data, "paap-server-cr-manager")
	for _, want := range []struct {
		group    string
		resource string
	}{
		{"kpack.io", "clusterbuilders"},
		{"kpack.io", "clusterbuildpacks"},
		{"kpack.io", "clusterstores"},
		{"kpack.io", "clusterstacks"},
		{"kpack.io", "clusterlifecycles"},
		{"kpack.io", "builders"},
		{"kpack.io", "images"},
		{"kpack.io", "builds"},
		{"kpack.io", "sourceresolvers"},
	} {
		if !manifestHasRuleForResource(role.Rules, want.group, want.resource) {
			t.Fatalf("paap-server ClusterRole must allow %s/%s for source component kpack bootstrap", want.group, want.resource)
		}
	}
}

func TestPAAPServerRBACAllowsExternalAccessDiscovery(t *testing.T) {
	data, err := os.ReadFile("../../deploy/k8s/paap-server.yaml")
	if err != nil {
		t.Fatalf("read paap server manifest: %v", err)
	}

	role := readClusterRoleFromManifest(t, data, "paap-server-cr-manager")
	for _, want := range []struct {
		group    string
		resource string
	}{
		{"", "nodes"},
		{"", "services"},
		{"networking.k8s.io", "ingresses"},
		{"gateway.networking.k8s.io", "gateways"},
		{"gateway.networking.k8s.io", "httproutes"},
	} {
		if !manifestHasRuleForResource(role.Rules, want.group, want.resource) {
			t.Fatalf("paap-server ClusterRole must allow %s/%s for external access discovery", want.group, want.resource)
		}
	}
}

func TestPAAPServerRBACAllowsRuntimeConsole(t *testing.T) {
	data, err := os.ReadFile("../../deploy/k8s/paap-server.yaml")
	if err != nil {
		t.Fatalf("read paap server manifest: %v", err)
	}

	role := readClusterRoleFromManifest(t, data, "paap-server-cr-manager")
	for _, want := range []string{"pods", "pods/log", "pods/attach", "pods/exec", "pods/ephemeralcontainers"} {
		if !manifestHasRuleForResource(role.Rules, "", want) {
			t.Fatalf("paap-server ClusterRole must allow core/%s for runtime console and diagnostics", want)
		}
	}
	for _, verb := range []string{"update", "patch"} {
		if !manifestHasVerbForResource(role.Rules, "", "pods/ephemeralcontainers", verb) {
			t.Fatalf("paap-server ClusterRole must allow %s on core/pods/ephemeralcontainers for debug console fallback", verb)
		}
	}
	if manifestHasVerbForResource(role.Rules, "", "pods", "create") {
		t.Fatalf("paap-server runtime console must not grant create on core/pods")
	}
}

type manifestClusterRole struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Rules []PolicyRuleSpec `yaml:"rules"`
}

func readClusterRoleFromManifest(t *testing.T, data []byte, name string) manifestClusterRole {
	t.Helper()
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var role manifestClusterRole
		err := decoder.Decode(&role)
		if err != nil {
			break
		}
		if role.Kind == "ClusterRole" && role.Metadata.Name == name {
			return role
		}
	}
	t.Fatalf("ClusterRole %s not found", name)
	return manifestClusterRole{}
}

func manifestHasRuleForResource(rules []PolicyRuleSpec, group, resource string) bool {
	for _, rule := range rules {
		if manifestRuleHasResource(rule, group, resource) {
			return true
		}
	}
	return false
}

func manifestRuleHasResource(rule PolicyRuleSpec, group, resource string) bool {
	return containsString(rule.APIGroups, group) && containsString(rule.Resources, resource)
}

func manifestHasWriteRuleForResource(rules []PolicyRuleSpec, group, resource string) bool {
	writeVerbs := map[string]bool{
		"*":      true,
		"create": true,
		"update": true,
		"patch":  true,
		"delete": true,
	}
	for _, rule := range rules {
		if !containsString(rule.APIGroups, group) || !containsString(rule.Resources, resource) {
			continue
		}
		for _, verb := range rule.Verbs {
			if writeVerbs[verb] {
				return true
			}
		}
	}
	return false
}

func manifestHasVerbForResource(rules []PolicyRuleSpec, group, resource, verb string) bool {
	for _, rule := range rules {
		if !containsString(rule.APIGroups, group) || !containsString(rule.Resources, resource) {
			continue
		}
		if containsString(rule.Verbs, verb) || containsString(rule.Verbs, "*") {
			return true
		}
	}
	return false
}

func templateRoleWritesResource(text, resource string) bool {
	for _, rule := range strings.Split(text, "\n- apiGroups:") {
		if !strings.Contains(rule, "- "+resource) {
			continue
		}
		for _, verb := range []string{"- '*'", "- *", "- create", "- update", "- patch", "- delete"} {
			if strings.Contains(rule, verb) {
				return true
			}
		}
	}
	return false
}

func containsString(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

func containsInterfaceString(values []interface{}, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}

func nestedBool(values map[string]interface{}, key string) bool {
	var current interface{} = values
	for _, part := range strings.Split(key, ".") {
		next, ok := current.(map[string]interface{})[part]
		if !ok {
			return true
		}
		current = next
	}
	result, ok := current.(bool)
	if !ok {
		return true
	}
	return result
}
