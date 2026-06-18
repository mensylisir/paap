package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"paap/internal/k8s"
	"paap/internal/model"
)

type fakeComponentJenkinsClient struct {
	baseURL        string
	ensureCalls    int
	buildCalls     int
	buildJobName   string
	buildErr       error
	ensureErr      error
	ensuredJobSpec k8s.JenkinsPipelineJobSpec
}

func (f *fakeComponentJenkinsClient) Base() string {
	return f.baseURL
}

func (f *fakeComponentJenkinsClient) EnsurePipelineJob(_ context.Context, spec k8s.JenkinsPipelineJobSpec) error {
	f.ensureCalls++
	f.ensuredJobSpec = spec
	return f.ensureErr
}

func (f *fakeComponentJenkinsClient) BuildJob(_ context.Context, jobName string) error {
	f.buildCalls++
	f.buildJobName = jobName
	return f.buildErr
}

func TestBuildComponentManifestContainsDeploymentServiceAndPaapLabels(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	comp := model.Component{Name: "订单服务", Type: "backend", Image: "myapp-dev-registry:5000/order", Version: "v1.0.0", Replicas: 2}

	manifest := BuildComponentManifest(app, env, comp, "backend-1", "myapp-dev")

	expected := []string{
		"kind: Deployment",
		"kind: Service",
		"name: backend-1",
		"namespace: myapp-dev",
		"paap.io/app: myapp",
		"paap.io/env: dev",
		"paap.io/component: backend-1",
		"image: myapp-dev-registry:5000/order:v1.0.0",
	}
	for _, want := range expected {
		if !strings.Contains(manifest, want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifest)
		}
	}
}

func TestBuildComponentDeploymentAndServiceManifestsAreSeparateFiles(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	comp := model.Component{Name: "订单服务", Type: "backend", Image: "myapp-dev-registry:5000/order", Version: "v1.0.0", Replicas: 2}

	deployment := BuildComponentDeploymentManifest(app, env, comp, "backend-1", "myapp-dev")
	service := BuildComponentServiceManifest(app, env, comp, "backend-1", "myapp-dev")

	if !strings.Contains(deployment, "kind: Deployment") || strings.Contains(deployment, "kind: Service") {
		t.Fatalf("deployment manifest should contain only Deployment:\n%s", deployment)
	}
	if !strings.Contains(service, "kind: Service") || strings.Contains(service, "kind: Deployment") {
		t.Fatalf("service manifest should contain only Service:\n%s", service)
	}
}

func TestBuildComponentServiceManifestExposesFrontendAsNodePort(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	comp := model.Component{Name: "前端", Type: "frontend", Image: "nginx", Version: "alpine", Replicas: 1}

	service := BuildComponentServiceManifest(app, env, comp, "frontend-1", "myapp-dev")

	if !strings.Contains(service, "type: NodePort") {
		t.Fatalf("frontend service should be exposed as NodePort:\n%s", service)
	}
}

func TestBuildComponentManifestMountsNginxApiProxyConfig(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	cfg := model.ComponentConfig{
		Framework: "nginx",
		ConfigMaps: []model.ComponentConfigMap{{
			Name: "frontend-1-config",
			Data: map[string]string{
				"default.conf": "server {\n  listen 80;\n  location /api/ {\n    proxy_pass http://backend-1;\n  }\n}\n",
			},
		}},
		Files: []model.ComponentConfigFile{{
			Name:          "nginx-api-proxy",
			ConfigMapName: "frontend-1-config",
			Key:           "default.conf",
			MountPath:     "/etc/nginx/conf.d/default.conf",
		}},
		Bindings: []model.ComponentBinding{{
			TargetName: "backend-1",
			TargetType: "backend",
			Role:       "backend",
			Mode:       "env",
			Generated:  map[string]string{"BACKEND_URL": "http://backend-1"},
		}},
		Dependencies: []string{"backend-1"},
	}
	configJSON, err := cfg.JSON()
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	comp := model.Component{Name: "前端", Type: "frontend", Image: "nginx", Version: "alpine", Replicas: 1, Config: configJSON}

	manifest := BuildComponentManifest(app, env, comp, "frontend-1", "myapp-dev")

	expected := []string{
		"kind: ConfigMap",
		"name: frontend-1-config",
		"default.conf: |",
		"proxy_pass http://backend-1;",
		"mountPath: /etc/nginx/conf.d/default.conf",
		"subPath: default.conf",
	}
	for _, want := range expected {
		if !strings.Contains(manifest, want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifest)
		}
	}
}

func TestBuildComponentDeploymentChecksumChangesWhenMountedConfigChanges(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	cfg := model.ComponentConfig{
		ConfigMaps: []model.ComponentConfigMap{{
			Name: "frontend-1-config",
			Data: map[string]string{"default.conf": "server { listen 80; }\n"},
		}},
		Files: []model.ComponentConfigFile{{
			Name:          "nginx-config",
			ConfigMapName: "frontend-1-config",
			Key:           "default.conf",
			MountPath:     "/etc/nginx/conf.d/default.conf",
		}},
	}
	configJSON, err := cfg.JSON()
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	comp := model.Component{Name: "前端", Type: "frontend", Image: "nginx", Version: "alpine", Replicas: 1, Config: configJSON}

	first := deploymentConfigChecksum(t, BuildComponentDeploymentManifest(app, env, comp, "frontend-1", "myapp-dev"))

	cfg.ConfigMaps[0].Data["default.conf"] = "server { listen 81; }\n"
	configJSON, err = cfg.JSON()
	if err != nil {
		t.Fatalf("encode changed config: %v", err)
	}
	comp.Config = configJSON
	second := deploymentConfigChecksum(t, BuildComponentDeploymentManifest(app, env, comp, "frontend-1", "myapp-dev"))

	if first == "" || second == "" {
		t.Fatalf("deployment config checksum must be present, got first=%q second=%q", first, second)
	}
	if first == second {
		t.Fatalf("deployment config checksum must change when mounted config changes, got %q", first)
	}
}

func deploymentConfigChecksum(t *testing.T, manifest string) string {
	t.Helper()
	var deploy appsv1.Deployment
	if err := yaml.Unmarshal([]byte(manifest), &deploy); err != nil {
		t.Fatalf("decode deployment manifest: %v\n%s", err, manifest)
	}
	return deploy.Spec.Template.Annotations["paap.io/config-checksum"]
}

func TestBuildComponentManifestKeepsExplicitImageTag(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	comp := model.Component{Name: "订单服务", Type: "backend", Image: "registry.local:5000/order:v1.2.3", Replicas: 1}

	manifest := BuildComponentManifest(app, env, comp, "backend-1", "myapp-dev")
	if !strings.Contains(manifest, "image: registry.local:5000/order:v1.2.3") {
		t.Fatalf("manifest should keep explicit image tag:\n%s", manifest)
	}
}

func TestBuildComponentManifestScalesSourcePlaceholderToZeroReplicas(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	comp := model.Component{
		Name:           "订单服务",
		Type:           "backend",
		Image:          "registry.myapp-dev.example.com:5443/myapp-dev/orders-api:manual",
		Version:        "manual",
		Replicas:       2,
		DeliveryMode:   "source",
		PipelineStatus: "planned",
	}

	manifest := BuildComponentManifest(app, env, comp, "orders-api", "myapp-dev")
	if !strings.Contains(manifest, "replicas: 0") {
		t.Fatalf("source placeholder deployment should not pull a non-existent image before first build:\n%s", manifest)
	}
}

func TestBuildComponentManifestRestoresSourceReplicasAfterBuild(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	comp := model.Component{
		Name:           "订单服务",
		Type:           "backend",
		Image:          "registry.myapp-dev.example.com:5443/myapp-dev/orders-api:17",
		Version:        "17",
		Replicas:       2,
		DeliveryMode:   "source",
		PipelineStatus: "built",
	}

	manifest := BuildComponentManifest(app, env, comp, "orders-api", "myapp-dev")
	if !strings.Contains(manifest, "replicas: 2") {
		t.Fatalf("built source deployment should restore requested replicas:\n%s", manifest)
	}
	if strings.Contains(manifest, "image: registry.myapp-dev.example.com:5443/myapp-dev/orders-api:manual") {
		t.Fatalf("built source deployment must not keep placeholder image:\n%s", manifest)
	}
}

func TestBuildComponentManifestIncludesConfiguredEnvVars(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	cfg := model.ComponentConfig{
		Env: []model.ComponentEnvVar{
			{Name: "DATABASE_URL", Value: "postgres://orders"},
			{Name: "DB_PASSWORD", SecretName: "orders-db", SecretKey: "password"},
			{Name: "REDIS_HOST", ConfigMapName: "redis-config", ConfigMapKey: "host"},
		},
	}
	configJSON, err := cfg.JSON()
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	comp := model.Component{
		Name:     "订单服务",
		Type:     "backend",
		Image:    "registry.local/orders",
		Version:  "v1",
		Replicas: 1,
		Config:   configJSON,
	}

	manifest := BuildComponentManifest(app, env, comp, "orders-api", "myapp-dev")

	expected := []string{
		"env:",
		"name: DATABASE_URL",
		"value: postgres://orders",
		"name: DB_PASSWORD",
		"secretKeyRef:",
		"name: orders-db",
		"key: password",
		"name: REDIS_HOST",
		"configMapKeyRef:",
		"name: redis-config",
		"key: host",
	}
	for _, want := range expected {
		if !strings.Contains(manifest, want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifest)
		}
	}
}

func TestBuildComponentManifestIncludesGeneratedConfigObjectsAndFileMounts(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	cfg := model.ComponentConfig{
		Framework: "springboot",
		ConfigMaps: []model.ComponentConfigMap{{
			Name: "orders-api-config",
			Data: map[string]string{
				"application-paap.yml": "spring:\n  datasource:\n    url: jdbc:postgresql://postgresql:5432/postgres\n",
			},
		}},
		Secrets: []model.ComponentSecret{{
			Name: "orders-api-secret",
			Data: map[string]string{
				"POSTGRES_PASSWORD": "secret",
			},
		}},
		Files: []model.ComponentConfigFile{{
			Name:          "spring-config",
			ConfigMapName: "orders-api-config",
			Key:           "application-paap.yml",
			MountPath:     "/etc/paap/application-paap.yml",
		}},
		Env: []model.ComponentEnvVar{
			{Name: "SPRING_CONFIG_ADDITIONAL_LOCATION", Value: "file:/etc/paap/"},
			{Name: "POSTGRES_PASSWORD", SecretName: "orders-api-secret", SecretKey: "POSTGRES_PASSWORD"},
		},
		Bindings: []model.ComponentBinding{{
			TargetName: "postgresql",
			TargetType: "postgresql",
			Role:       "database",
			Mode:       "springboot-file",
		}},
		Dependencies: []string{"postgresql"},
	}
	configJSON, err := cfg.JSON()
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	comp := model.Component{
		Name:     "订单服务",
		Type:     "backend",
		Image:    "registry.local/orders",
		Version:  "v1",
		Replicas: 1,
		Config:   configJSON,
	}

	manifest := BuildComponentManifest(app, env, comp, "orders-api", "myapp-dev")

	expected := []string{
		"kind: ConfigMap",
		"name: orders-api-config",
		"application-paap.yml",
		"kind: Secret",
		"name: orders-api-secret",
		"stringData:",
		"POSTGRES_PASSWORD: secret",
		"mountPath: /etc/paap/application-paap.yml",
		"subPath: application-paap.yml",
		"name: SPRING_CONFIG_ADDITIONAL_LOCATION",
		"value: file:/etc/paap/",
	}
	for _, want := range expected {
		if !strings.Contains(manifest, want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifest)
		}
	}
}

func TestPutGiteaFileCreatesNewFileWithPost(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodGet:
			http.NotFound(w, r)
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"content":{"path":"components/backend-1/deployment.yaml"}}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	err := putGiteaFile(t.Context(), server.URL, "test-repo", "components/backend-1/deployment.yaml", "apiVersion: v1", "deploy")
	if err != nil {
		t.Fatalf("put gitea file: %v", err)
	}
	if got := strings.Join(methods, ","); got != "GET,POST" {
		t.Fatalf("expected GET then POST for new file, got %s", got)
	}
}

func TestPutGiteaFileUpdatesExistingFileWithPut(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"sha":"old-sha"}`))
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"content":{"path":"components/backend-1/Jenkinsfile"}}`))
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	err := putGiteaFile(t.Context(), server.URL, "test-repo", "components/backend-1/Jenkinsfile", "pipeline {}", "sync jenkinsfile")
	if err != nil {
		t.Fatalf("put gitea file: %v", err)
	}
	if got := strings.Join(methods, ","); got != "GET,PUT" {
		t.Fatalf("expected GET then PUT for existing file, got %s", got)
	}
}

func TestPutGiteaFileSkipsUpdateWhenContentUnchanged(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"sha":"old-sha","content":"cGlwZWxpbmUge30="}`))
		case http.MethodPut:
			t.Fatalf("unchanged content should not create a new Gitea commit")
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	err := putGiteaFile(t.Context(), server.URL, "test-repo", "components/backend-1/Jenkinsfile", "pipeline {}", "sync jenkinsfile")
	if err != nil {
		t.Fatalf("put gitea file: %v", err)
	}
	if got := strings.Join(methods, ","); got != "GET" {
		t.Fatalf("expected GET only for unchanged file, got %s", got)
	}
}

func TestEnsureArgoCDRepositorySecretUsesHTTPBasicAuth(t *testing.T) {
	k8sClient := fake.NewClientBuilder().WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "paap-repo-test-staging-components",
			Namespace: "test-staging-deploy",
		},
		Data: map[string][]byte{
			"sshPrivateKey": []byte("old-key"),
			"url":           []byte("old"),
		},
	}).Build()

	if err := ensureArgoCDRepositorySecret(t.Context(), k8sClient, "test-staging-deploy", "test-staging-components", "http://gitea/paap/repo.git", "paap", "paap123456"); err != nil {
		t.Fatalf("ensure repo secret: %v", err)
	}
	var got corev1.Secret
	if err := k8sClient.Get(t.Context(), client.ObjectKey{Namespace: "test-staging-deploy", Name: "paap-repo-test-staging-components"}, &got); err != nil {
		t.Fatalf("get secret: %v", err)
	}
	if _, ok := got.Data["sshPrivateKey"]; ok {
		t.Fatalf("expected sshPrivateKey to be removed")
	}
	if string(got.Data["url"]) != "http://gitea/paap/repo.git" || string(got.Data["username"]) != "paap" || string(got.Data["password"]) != "paap123456" {
		t.Fatalf("unexpected secret data: %#v", got.Data)
	}
}

func TestEnsureArgoCDApplicationUsesEnvironmentProject(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev",
			Labels: map[string]string{
				"paap.io/app": "billing",
				"paap.io/env": "dev",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-app",
			Labels: map[string]string{
				"paap.io/app": "billing",
				"paap.io/env": "dev",
			},
		}},
	).Build()

	err := ensureArgoCDProject(
		t.Context(),
		cl,
		"billing-dev-deploy",
		"billing-dev",
		"http://gitea/paap/billing-dev-components.git",
		[]string{"billing-dev", "billing-dev-app"},
	)
	if err != nil {
		t.Fatalf("ensure project: %v", err)
	}
	if err := ensureArgoCDDefaultProjectDenied(t.Context(), cl, "billing-dev-deploy"); err != nil {
		t.Fatalf("deny default project: %v", err)
	}
	if err := ensureArgoCDApplication(
		t.Context(),
		cl,
		"billing-dev-deploy",
		"billing-dev-api",
		"billing-dev",
		"http://gitea/paap/billing-dev-components.git",
		"components/api",
		"billing-dev",
		"billing",
		"dev",
		"api",
	); err != nil {
		t.Fatalf("ensure application: %v", err)
	}

	project := &unstructured.Unstructured{}
	project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev"}, project); err != nil {
		t.Fatalf("get app project: %v", err)
	}
	destinations, _, _ := unstructured.NestedSlice(project.Object, "spec", "destinations")
	if len(destinations) != 2 {
		t.Fatalf("expected two project destinations, got %#v", destinations)
	}
	for _, item := range destinations {
		dest := item.(map[string]interface{})
		if dest["namespace"] == "*" || dest["server"] == "*" {
			t.Fatalf("project must not allow wildcard destinations: %#v", destinations)
		}
	}
	clusterWhitelist, _, _ := unstructured.NestedSlice(project.Object, "spec", "clusterResourceWhitelist")
	if len(clusterWhitelist) != 0 {
		t.Fatalf("project must not whitelist cluster-scoped resources: %#v", clusterWhitelist)
	}
	namespaceWhitelist, _, _ := unstructured.NestedSlice(project.Object, "spec", "namespaceResourceWhitelist")
	for _, want := range []map[string]string{
		{"group": "", "kind": "Pod"},
		{"group": "", "kind": "Endpoints"},
		{"group": "", "kind": "ConfigMap"},
		{"group": "apps", "kind": "Deployment"},
		{"group": "apps", "kind": "ControllerRevision"},
		{"group": "batch", "kind": "Job"},
		{"group": "batch", "kind": "CronJob"},
		{"group": "discovery.k8s.io", "kind": "EndpointSlice"},
	} {
		if !containsArgoCDResourceWhitelist(namespaceWhitelist, want["group"], want["kind"]) {
			t.Fatalf("project namespace whitelist missing %s/%s: %#v", want["group"], want["kind"], namespaceWhitelist)
		}
	}
	defaultProject := &unstructured.Unstructured{}
	defaultProject.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "default"}, defaultProject); err != nil {
		t.Fatalf("get default project: %v", err)
	}
	defaultDestinations, _, _ := unstructured.NestedSlice(defaultProject.Object, "spec", "destinations")
	defaultRepos, _, _ := unstructured.NestedSlice(defaultProject.Object, "spec", "sourceRepos")
	if len(defaultDestinations) != 0 || len(defaultRepos) != 0 {
		t.Fatalf("default project must be denied, destinations=%#v repos=%#v", defaultDestinations, defaultRepos)
	}

	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev-api"}, app); err != nil {
		t.Fatalf("get application: %v", err)
	}
	projectName, _, _ := unstructured.NestedString(app.Object, "spec", "project")
	if projectName != "billing-dev" {
		t.Fatalf("application project = %q, want billing-dev", projectName)
	}
	if got := app.GetAnnotations()["argocd.argoproj.io/refresh"]; got != "hard" {
		t.Fatalf("application refresh annotation = %q, want hard", got)
	}
	if got := app.GetAnnotations()["paap.io/refreshed-at"]; got == "" {
		t.Fatalf("application refreshed-at annotation must be set")
	}
	syncOptions, _, _ := unstructured.NestedStringSlice(app.Object, "spec", "syncPolicy", "syncOptions")
	for _, option := range syncOptions {
		if option == "CreateNamespace=true" {
			t.Fatalf("application must not create arbitrary namespaces: %#v", syncOptions)
		}
	}
}

func TestDiscoverComponentGitOpsNamespacesOnlyIncludesWorkloadNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev",
			Labels: map[string]string{
				"paap.io/app":  "billing",
				"paap.io/env":  "dev",
				"paap.io/role": "workload",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-app",
			Labels: map[string]string{
				"paap.io/app":  "billing",
				"paap.io/env":  "dev",
				"paap.io/role": "workload",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-deploy",
			Labels: map[string]string{
				"paap.io/app":     "billing",
				"paap.io/env":     "dev",
				"paap.io/role":    "tool",
				"paap.io/service": "deploy",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-git",
			Labels: map[string]string{
				"paap.io/app":     "billing",
				"paap.io/env":     "dev",
				"paap.io/role":    "tool",
				"paap.io/service": "git",
			},
		}},
	).Build()

	got := discoverComponentGitOpsNamespaces(t.Context(), cl, "billing", "dev", "billing-dev")
	want := []string{"billing-dev", "billing-dev-app"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("gitops namespaces = %#v, want only workload namespaces %#v", got, want)
	}
}

func TestDiscoverComponentToolNamespacesPreferLabeledToolNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-argocd",
			Labels: map[string]string{
				"paap.io/app":      "billing",
				"paap.io/env":      "dev",
				"paap.io/tool":     "argocd",
				"paap.io/category": "tool",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-gitea",
			Labels: map[string]string{
				"paap.io/app":      "billing",
				"paap.io/env":      "dev",
				"paap.io/tool":     "gitea",
				"paap.io/category": "tool",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-jenkins",
			Labels: map[string]string{
				"paap.io/app":      "billing",
				"paap.io/env":      "dev",
				"paap.io/tool":     "jenkins",
				"paap.io/category": "tool",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-deploy",
			Labels: map[string]string{
				"paap.io/app":          "billing",
				"paap.io/env":          "dev",
				"paap.io/service-type": "deploy",
			},
		}},
	).Build()

	namespaces := discoverComponentToolNamespaces(t.Context(), cl, "billing", "dev")
	if namespaces.ArgoCD != "billing-dev-argocd" || namespaces.Gitea != "billing-dev-gitea" || namespaces.Jenkins != "billing-dev-jenkins" {
		t.Fatalf("unexpected tool namespaces: %#v", namespaces)
	}
}

func TestDiscoverComponentToolNamespacesDefaultToToolIdentityNamespaces(t *testing.T) {
	namespaces := discoverComponentToolNamespaces(t.Context(), nil, "billing", "dev")
	if namespaces.ArgoCD != "billing-dev-argocd" || namespaces.Gitea != "billing-dev-gitea" || namespaces.Jenkins != "billing-dev-jenkins" {
		t.Fatalf("unexpected default tool namespaces: %#v", namespaces)
	}
}

func TestDiscoverComponentToolNamespacesFallbackToLegacyServiceTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-deploy",
			Labels: map[string]string{
				"paap.io/app":          "billing",
				"paap.io/env":          "dev",
				"paap.io/service-type": "deploy",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-git",
			Labels: map[string]string{
				"paap.io/app":          "billing",
				"paap.io/env":          "dev",
				"paap.io/service-type": "git",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-dev-ci",
			Labels: map[string]string{
				"paap.io/app":          "billing",
				"paap.io/env":          "dev",
				"paap.io/service-type": "ci",
			},
		}},
	).Build()

	namespaces := discoverComponentToolNamespaces(t.Context(), cl, "billing", "dev")
	if namespaces.ArgoCD != "billing-dev-deploy" || namespaces.Gitea != "billing-dev-git" || namespaces.Jenkins != "billing-dev-ci" {
		t.Fatalf("unexpected legacy tool namespaces: %#v", namespaces)
	}
}

func TestEnsureArgoCDLocalClusterSecretScopesCacheToEnvironmentNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	err := ensureArgoCDLocalClusterSecret(t.Context(), cl, "billing-dev-argocd", []string{"billing-dev", "billing-dev-app"})
	if err != nil {
		t.Fatalf("ensure local cluster secret: %v", err)
	}
	var secret corev1.Secret
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-argocd", Name: "paap-local-cluster"}, &secret); err != nil {
		t.Fatalf("get local cluster secret: %v", err)
	}
	if secret.Labels["argocd.argoproj.io/secret-type"] != "cluster" {
		t.Fatalf("missing argocd cluster secret label: %#v", secret.Labels)
	}
	if string(secret.StringData["namespaces"]) != "" {
		t.Fatalf("fake client should persist data, not StringData")
	}
	if string(secret.Data["namespaces"]) != "billing-dev,billing-dev-app" {
		t.Fatalf("namespaces = %q", secret.Data["namespaces"])
	}
	if string(secret.Data["clusterResources"]) != "false" {
		t.Fatalf("clusterResources = %q", secret.Data["clusterResources"])
	}
	if !strings.Contains(string(secret.Data["config"]), `"inCluster":true`) {
		t.Fatalf("config should use in-cluster auth, got %s", secret.Data["config"])
	}
}

func containsArgoCDResourceWhitelist(items []interface{}, group, kind string) bool {
	for _, item := range items {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["group"] == group && entry["kind"] == kind {
			return true
		}
	}
	return false
}

func TestBuildComponentJenkinsfileDefaultsToKpackBuildpacks(t *testing.T) {
	comp := model.Component{
		ID:                  42,
		Name:                "Orders API",
		Type:                "backend",
		Image:               "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL:       "https://git.example.com/team/orders.git",
		SourceMirrorRepoURL: "http://gitea/paap/shop-dev-orders-api-source.git",
		SourceBranch:        "main",
		BuildContext:        "",
		DockerfilePath:      "",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	for _, want := range []string{
		"apiVersion: kpack.io/v1alpha2",
		"kind: Image",
		"kubectl apply -f kpack-image.yaml",
		"http://gitea/paap/shop-dev-orders-api-source.git",
		"GITOPS_DEPLOYMENT = \"components/orders-api/deployment.yaml\"",
	} {
		if !strings.Contains(jenkinsfile, want) {
			t.Fatalf("Jenkinsfile missing %q:\n%s", want, jenkinsfile)
		}
	}
	if strings.Contains(jenkinsfile, "docker build") || strings.Contains(jenkinsfile, "docker push") || strings.Contains(jenkinsfile, "pack build") {
		t.Fatalf("default Jenkinsfile must not require Docker daemon:\n%s", jenkinsfile)
	}
	if strings.Contains(jenkinsfile, `SOURCE_REPO = "https://git.example.com/team/orders.git"`) {
		t.Fatalf("Jenkinsfile should build from the environment-local Gitea mirror, not the external source:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfileFallsBackToExternalSourceWhenMirrorMissing(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	if !strings.Contains(jenkinsfile, `SOURCE_REPO = "https://git.example.com/team/orders.git"`) {
		t.Fatalf("Jenkinsfile should fall back to external source when mirror is missing:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfileRunsOnKubernetesKubectlAgent(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	for _, want := range []string{
		"podTemplate(",
		"node(POD_LABEL)",
		"container('kubectl')",
		"serviceAccountName: paap-kpack-build",
		"name: kubectl",
		"image: docker.io/alpine/k8s:1.34.1",
		"name: jnlp",
		"image: docker.io/jenkins/inbound-agent:3107.v665000b_51092-15",
		"stage('Submit Buildpacks Image')",
	} {
		if !strings.Contains(jenkinsfile, want) {
			t.Fatalf("Jenkinsfile missing Kubernetes kubectl agent config %q:\n%s", want, jenkinsfile)
		}
	}
	if strings.Contains(jenkinsfile, "pipeline {") {
		t.Fatalf("Jenkinsfile should use scripted pipeline so it does not require pipeline-model-definition:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfileUsesUntaggedImageNameForBuildNumberTag(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	if strings.Contains(jenkinsfile, `IMAGE_NAME = "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3"`) {
		t.Fatalf("IMAGE_NAME must not include a tag when Jenkins appends BUILD_NUMBER:\n%s", jenkinsfile)
	}
	if !strings.Contains(jenkinsfile, `IMAGE_NAME = "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api"`) {
		t.Fatalf("Jenkinsfile missing untagged IMAGE_NAME:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfileUpdatesGitOpsManifestInsteadOfCallingDeployAPI(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	for _, forbidden := range []string{
		"/api/v1/components/${COMPONENT_ID}/deploy",
		"/api/v1/components/42/deploy",
		"source-webhook",
		"PAAP_SERVER",
		"curl -sf -X POST",
	} {
		if strings.Contains(jenkinsfile, forbidden) {
			t.Fatalf("Jenkinsfile must not call PAAP deploy APIs (%q):\n%s", forbidden, jenkinsfile)
		}
	}
	for _, want := range []string{
		"stage('Update GitOps Manifest')",
		"GITOPS_DEPLOYMENT = \"components/orders-api/deployment.yaml\"",
		"GITOPS_REPO_PUSH_URL = \"http://paap:paap123456@gitea/paap/shop-dev-components.git\"",
		"sed -i -E",
		"git remote set-url origin ${GITOPS_REPO_PUSH_URL}",
		"git add ${GITOPS_DEPLOYMENT}",
		"git commit -m \"build orders-api:${tag}\"",
		"git push origin HEAD:${GITOPS_BRANCH}",
	} {
		if !strings.Contains(jenkinsfile, want) {
			t.Fatalf("Jenkinsfile missing GitOps update step %q:\n%s", want, jenkinsfile)
		}
	}
}

func TestBuildComponentJenkinsfileEscapesGitOpsPushCredentials(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea.internal:3000/paap/shop-dev-components.git", false)

	if !strings.Contains(jenkinsfile, `GITOPS_REPO_PUSH_URL = "http://paap:paap123456@gitea.internal:3000/paap/shop-dev-components.git"`) {
		t.Fatalf("Jenkinsfile missing authenticated GitOps push URL:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfilePassesBuildContextToKpackSource(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
		BuildContext:  "services/orders-api",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	if !strings.Contains(jenkinsfile, `BUILD_CONTEXT = "services/orders-api"`) {
		t.Fatalf("Jenkinsfile missing build context env:\n%s", jenkinsfile)
	}
	if !strings.Contains(jenkinsfile, "subPath: ${BUILD_CONTEXT}") {
		t.Fatalf("kpack Image source must include build context subPath for monorepos:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfilePinsGoBuildpackVersion(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	for _, want := range []string{
		"env:",
		"- name: BP_GO_VERSION",
		"value: \"1.25.*\"",
	} {
		if !strings.Contains(jenkinsfile, want) {
			t.Fatalf("Jenkinsfile must pin Go buildpack version with %q:\n%s", want, jenkinsfile)
		}
	}
}

func TestBuildComponentJenkinsfileMountsKpackRegistryCABinding(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", true)

	for _, want := range []string{
		"apiVersion: kpack.io/v1alpha1",
		"serviceAccount: ${KPACK_SERVICE_ACCOUNT}",
		"build:",
		"cnbBindings:",
		"name: paap-registry-ca",
		"secretRef:",
		"name: paap-kpack-registry-ca",
	} {
		if !strings.Contains(jenkinsfile, want) {
			t.Fatalf("Jenkinsfile missing kpack CA binding %q:\n%s", want, jenkinsfile)
		}
	}
	if strings.Contains(jenkinsfile, "serviceAccountName: ${KPACK_SERVICE_ACCOUNT}") {
		t.Fatalf("kpack v1alpha1 Image must use spec.serviceAccount because serviceAccountName converts to default:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfileSkipsKpackRegistryCABindingWhenUnavailable(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	if strings.Contains(jenkinsfile, "paap-kpack-registry-ca") || strings.Contains(jenkinsfile, "cnbBindings:") {
		t.Fatalf("Jenkinsfile must not reference missing kpack CA binding when PAAP has not synced a CA secret:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsJobSpecUsesComponentJenkinsfile(t *testing.T) {
	app := model.Application{Identifier: "shop"}
	env := model.Environment{Identifier: "dev"}

	spec := buildComponentJenkinsJobSpec(app, env, "orders-api", "http://gitea/paap/shop-dev-components.git")

	if spec.Name != "shop-dev-orders-api-build" {
		t.Fatalf("job name = %q", spec.Name)
	}
	if spec.RepoURL != "http://gitea/paap/shop-dev-components.git" {
		t.Fatalf("repo URL = %q", spec.RepoURL)
	}
	if spec.Branch != "main" {
		t.Fatalf("branch = %q", spec.Branch)
	}
	if spec.ScriptPath != "components/orders-api/Jenkinsfile" {
		t.Fatalf("script path = %q", spec.ScriptPath)
	}
}

func TestBuildComponentKpackSpecUsesEnvironmentRegistry(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{app}-{env}.corp.example.com:5443")
	spec := buildComponentKpackSpec(
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		"orders-api",
	)

	if spec.Namespace != "shop-dev-jenkins" {
		t.Fatalf("namespace = %q", spec.Namespace)
	}
	if spec.RegistryServer != "registry.shop-dev.corp.example.com:5443" {
		t.Fatalf("registry server = %q", spec.RegistryServer)
	}
	if spec.BuilderImage != "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest" {
		t.Fatalf("builder image = %q", spec.BuilderImage)
	}
	if spec.GitServer != "http://shop-dev-gitea.shop-dev-gitea.svc.cluster.local:3000" {
		t.Fatalf("git server = %q", spec.GitServer)
	}
	if spec.GitUsername != "paap" || spec.GitPassword != "paap123456" {
		t.Fatalf("unexpected git credentials username=%q password=%q", spec.GitUsername, spec.GitPassword)
	}
	if spec.StackBuildImage != k8s.PaketoBuildJammyBaseImage || spec.StackRunImage != k8s.PaketoRunJammyBaseImage {
		t.Fatalf("unexpected stack images: %#v", spec)
	}
	if len(spec.BuildpackSources) != 4 ||
		spec.BuildpackSources[0] != k8s.PaketoJavaBuildpackImage ||
		spec.BuildpackSources[1] != k8s.PaketoNodeJSBuildpackImage ||
		spec.BuildpackSources[2] != k8s.PaketoGoBuildpackImage ||
		spec.BuildpackSources[3] != k8s.PaketoPythonBuildpackImage {
		t.Fatalf("unexpected buildpack sources: %#v", spec.BuildpackSources)
	}
}

func TestBuildComponentKpackSpecReadsRegistryCAFromPassedClient(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{app}-{env}.corp.example.com:5443")
	k8sClient := fake.NewClientBuilder().WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "shop-dev-registry-tls", Namespace: "shop-dev-registry"},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt": []byte("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"),
		},
	}).Build()

	spec, warning := buildComponentKpackSpecWithRegistryCA(
		t.Context(),
		k8sClient,
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		"orders-api",
	)

	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if string(spec.RegistryCAPEM) != "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n" {
		t.Fatalf("registry CA = %q", spec.RegistryCAPEM)
	}
	if spec.StackBuildImage != "registry.shop-dev.corp.example.com:5443/shop-dev/paap-build-jammy-base:registry-ca" {
		t.Fatalf("stack build image should use PAAP registry CA image, got %q", spec.StackBuildImage)
	}
	if spec.StackRunImage != "registry.shop-dev.corp.example.com:5443/shop-dev/paap-run-jammy-base:registry-ca" {
		t.Fatalf("stack run image should use PAAP registry CA image, got %q", spec.StackRunImage)
	}
}

func TestBuildComponentKpackSpecUsesComponentRegistryHostAndMatchingCA(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "{service}-{app}-{env}.corp.example.com:5443")
	k8sClient := fake.NewClientBuilder().WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "shop-dev-registry-tls", Namespace: "shop-dev-registry"},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{"ca.crt": []byte("registry-ca")},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "shop-dev-harbor-ingress", Namespace: "shop-dev-harbor"},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{"ca.crt": []byte("harbor-ca")},
		},
	).Build()

	spec, warning := buildComponentKpackSpecWithRegistryCA(
		t.Context(),
		k8sClient,
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		"orders-api",
		"harbor-shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
	)

	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if spec.RegistryServer != "harbor-shop-dev.corp.example.com:5443" {
		t.Fatalf("registry server = %q", spec.RegistryServer)
	}
	if spec.BuilderImage != "harbor-shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest" {
		t.Fatalf("builder image = %q", spec.BuilderImage)
	}
	if spec.RegistryUsername != "admin" || spec.RegistryPassword != "Harbor12345" {
		t.Fatalf("expected Harbor registry credentials, got username=%q password=%q", spec.RegistryUsername, spec.RegistryPassword)
	}
	if string(spec.RegistryCAPEM) != "harbor-ca" {
		t.Fatalf("registry CA = %q", spec.RegistryCAPEM)
	}
}

func TestComponentCIStatusWarnsWhenKpackUsesLightweightHTTPRegistry(t *testing.T) {
	status := k8s.KpackBuildEnvironmentStatus{
		Ready:           true,
		RegistryWarning: "kpack cannot push to the lightweight HTTP registry; use Harbor or a trusted TLS registry",
	}
	ciStatus, ciWarning := componentCIStatusFromKpack(status)

	if ciStatus != "pending" {
		t.Fatalf("ci status = %q", ciStatus)
	}
	if !strings.Contains(ciWarning, "HTTP registry") || !strings.Contains(ciWarning, "trusted TLS") {
		t.Fatalf("warning = %q", ciWarning)
	}
}

func TestKpackPendingWarningIsPreservedBeforeJenkinsSync(t *testing.T) {
	status := k8s.KpackBuildEnvironmentStatus{Warning: "missing kpack CRDs/controller: builders.kpack.io"}
	ciStatus, ciWarning := componentCIStatusFromKpack(status)

	if ciStatus != "pending" {
		t.Fatalf("ci status = %q", ciStatus)
	}
	if !strings.Contains(ciWarning, "builders.kpack.io") {
		t.Fatalf("warning = %q", ciWarning)
	}
}

func TestKpackPendingWarningCombinesRegistryWarning(t *testing.T) {
	status := k8s.KpackBuildEnvironmentStatus{
		Warning:         "missing kpack CRDs/controller: builders.kpack.io",
		RegistryWarning: "kpack cannot push to HTTP registry",
	}
	ciStatus, ciWarning := componentCIStatusFromKpack(status)

	if ciStatus != "pending" {
		t.Fatalf("ci status = %q", ciStatus)
	}
	if !strings.Contains(ciWarning, "builders.kpack.io") || !strings.Contains(ciWarning, "HTTP registry") {
		t.Fatalf("warning should include both causes, got %q", ciWarning)
	}
}

func TestJenkinsNotifyCommitHookURLTargetsRepo(t *testing.T) {
	hookURL := jenkinsNotifyCommitHookURL("http://jenkins.shop-dev-ci.svc.cluster.local:8080", "http://gitea/paap/shop-dev-components.git")

	if !strings.HasPrefix(hookURL, "http://jenkins.shop-dev-ci.svc.cluster.local:8080/git/notifyCommit?url=") {
		t.Fatalf("unexpected hook URL: %s", hookURL)
	}
	if !strings.Contains(hookURL, "http%3A%2F%2Fgitea%2Fpaap%2Fshop-dev-components.git") {
		t.Fatalf("repo URL should be escaped in hook URL: %s", hookURL)
	}
}

func TestBuildComponentReadmeDescribesAutomaticJenkinsSync(t *testing.T) {
	readme := buildComponentReadme(
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		model.Component{Name: "Orders API", Image: "registry/orders:v1", Replicas: 1},
		"orders-api",
		"http://gitea/paap/shop-dev-components.git",
		"components/orders-api",
	)

	for _, forbidden := range []string{"Configure a Gitea webhook", "Install Jenkins in the environment"} {
		if strings.Contains(readme, forbidden) {
			t.Fatalf("README should not tell users to manually configure CI: %q\n%s", forbidden, readme)
		}
	}
	for _, want := range []string{"Jenkins Pipeline Job", "Gitea push webhook", "Buildpacks/kpack"} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing %q:\n%s", want, readme)
		}
	}
}

func TestEnsureGiteaSourceMirrorMigratesExternalRepository(t *testing.T) {
	var createRepoCalled bool
	var migrateCalled bool
	var migratedCloneAddr string
	var migratedRepoName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			createRepoCalled = true
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/migrate":
			migrateCalled = true
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode migrate body: %v", err)
			}
			migratedCloneAddr = body["clone_addr"].(string)
			migratedRepoName = body["repo_name"].(string)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	comp := model.Component{DeliveryMode: "source", SourceRepoURL: "https://git.example.com/team/orders.git"}
	mirrorURL, err := ensureGiteaSourceMirror(t.Context(), server.URL, "shop-dev-orders-api-source", comp)
	if err != nil {
		t.Fatalf("ensure source mirror: %v", err)
	}

	if createRepoCalled {
		t.Fatalf("source mirror should use Gitea migrate instead of creating an empty repository")
	}
	if !migrateCalled {
		t.Fatalf("expected Gitea migrate API to be called")
	}
	if migratedCloneAddr != comp.SourceRepoURL || migratedRepoName != "shop-dev-orders-api-source" {
		t.Fatalf("unexpected migrate payload: clone=%q repo=%q", migratedCloneAddr, migratedRepoName)
	}
	if mirrorURL != server.URL+"/paap/shop-dev-orders-api-source.git" {
		t.Fatalf("mirror URL = %q", mirrorURL)
	}
}

func TestEnsureGiteaSourceMirrorTreatsExistingMirrorAsReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/repos/migrate" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	mirrorURL, err := ensureGiteaSourceMirror(t.Context(), server.URL, "shop-dev-orders-api-source", model.Component{
		DeliveryMode:  "source",
		SourceRepoURL: "https://git.example.com/team/orders.git",
	})
	if err != nil {
		t.Fatalf("existing source mirror should be accepted: %v", err)
	}
	if mirrorURL != server.URL+"/paap/shop-dev-orders-api-source.git" {
		t.Fatalf("mirror URL = %q", mirrorURL)
	}
}

func TestEnsureGiteaSourceMirrorUsesEnvironmentLocalRepoDirectly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("environment-local source repository should not be migrated: %s %s", r.Method, r.URL.Path)
	}))
	defer server.Close()

	sourceURL := server.URL + "/paap/source.git"
	mirrorURL, err := ensureGiteaSourceMirror(t.Context(), server.URL, "shop-dev-orders-api-source", model.Component{
		DeliveryMode:  "source",
		SourceRepoURL: sourceURL,
	})
	if err != nil {
		t.Fatalf("ensure source mirror: %v", err)
	}
	if mirrorURL != sourceURL {
		t.Fatalf("mirror URL = %q", mirrorURL)
	}
}

func TestEnsureGiteaSourceMirrorDoesNotTreatUnprocessableMigrateAsReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/repos/migrate" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"message":"clone failed"}`))
	}))
	defer server.Close()

	_, err := ensureGiteaSourceMirror(t.Context(), server.URL, "shop-dev-orders-api-source", model.Component{
		DeliveryMode:  "source",
		SourceRepoURL: "https://git.example.com/team/orders.git",
	})
	if err == nil || !strings.Contains(err.Error(), "status=422") {
		t.Fatalf("expected migrate failure to be returned, got %v", err)
	}
}

func TestEnsureComponentJenkinsAutomationTriggersInitialBuildForPlannedSourceComponent(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{baseURL: "http://jenkins.shop-dev-ci.svc.cluster.local:8080"}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	server := giteaHookServer(t)
	status, warning := ensureComponentJenkinsAutomation(
		t.Context(),
		kpackReadyClient(t),
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		model.Component{
			Name:           "Orders API",
			Type:           "backend",
			DeliveryMode:   "source",
			JenkinsJob:     "shop-dev-orders-api-build",
			PipelineStatus: "planned",
		},
		"orders-api",
		server.URL,
		"shop-dev-components",
		"http://gitea/paap/shop-dev-components.git",
		k8s.KpackBuildEnvironmentSpec{
			Namespace:      "shop-dev-ci",
			RegistryServer: "registry.shop-dev.corp.example.com:5443",
			BuilderImage:   "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest",
		},
		"",
	)

	if status != "running" {
		t.Fatalf("ci status = %q, want running; warning=%q", status, warning)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if jenkins.ensureCalls != 1 {
		t.Fatalf("expected Jenkins job to be synced once, got %d", jenkins.ensureCalls)
	}
	if jenkins.buildCalls != 1 || jenkins.buildJobName != "shop-dev-orders-api-build" {
		t.Fatalf("expected initial Jenkins build for job, calls=%d job=%q", jenkins.buildCalls, jenkins.buildJobName)
	}
}

func TestEnsureComponentJenkinsAutomationDoesNotRetriggerAfterInitialBuild(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{baseURL: "http://jenkins.shop-dev-ci.svc.cluster.local:8080"}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	server := giteaHookServer(t)
	status, warning := ensureComponentJenkinsAutomation(
		t.Context(),
		kpackReadyClient(t),
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		model.Component{
			Name:           "Orders API",
			Type:           "backend",
			DeliveryMode:   "source",
			JenkinsJob:     "shop-dev-orders-api-build",
			PipelineStatus: "running",
		},
		"orders-api",
		server.URL,
		"shop-dev-components",
		"http://gitea/paap/shop-dev-components.git",
		k8s.KpackBuildEnvironmentSpec{
			Namespace:      "shop-dev-ci",
			RegistryServer: "registry.shop-dev.corp.example.com:5443",
			BuilderImage:   "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest",
		},
		"",
	)

	if status != "configured" {
		t.Fatalf("ci status = %q, want configured; warning=%q", status, warning)
	}
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if jenkins.buildCalls != 0 {
		t.Fatalf("expected no repeated Jenkins build trigger, got %d", jenkins.buildCalls)
	}
}

func TestEnsureComponentJenkinsAutomationCreatesSourceRepositoryWebhook(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{baseURL: "http://jenkins.shop-dev-ci.svc.cluster.local:8080"}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	createdHooks := map[string]map[string]interface{}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks" || r.URL.Path == "/api/v1/repos/paap/shop-dev-orders-api-source/hooks"):
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks" || r.URL.Path == "/api/v1/repos/paap/shop-dev-orders-api-source/hooks"):
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode hook body: %v", err)
			}
			createdHooks[r.URL.Path] = body
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	status, warning := ensureComponentJenkinsAutomation(
		t.Context(),
		kpackReadyClient(t),
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		model.Component{
			ID:                  42,
			Name:                "Orders API",
			Type:                "backend",
			DeliveryMode:        "source",
			JenkinsJob:          "shop-dev-orders-api-build",
			PipelineStatus:      "running",
			SourceMirrorRepoURL: server.URL + "/paap/shop-dev-orders-api-source.git",
		},
		"orders-api",
		server.URL,
		"shop-dev-components",
		server.URL+"/paap/shop-dev-components.git",
		k8s.KpackBuildEnvironmentSpec{
			Namespace:      "shop-dev-ci",
			RegistryServer: "registry.shop-dev.corp.example.com:5443",
			BuilderImage:   "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest",
		},
		"",
	)

	if status != "configured" || warning != "" {
		t.Fatalf("status=%q warning=%q", status, warning)
	}
	sourceHook := createdHooks["/api/v1/repos/paap/shop-dev-orders-api-source/hooks"]
	sourceConfig, ok := sourceHook["config"].(map[string]interface{})
	expectedHookURL := jenkinsRemoteBuildHookURL("http://jenkins.shop-dev-ci.svc.cluster.local:8080", "shop-dev-orders-api-build")
	if !ok || sourceConfig["url"] != expectedHookURL {
		t.Fatalf("source mirror hook should notify Jenkins directly, got %#v", sourceHook)
	}
	if strings.Contains(sourceConfig["url"].(string), "paap-server") || strings.Contains(sourceConfig["url"].(string), "source-webhook") {
		t.Fatalf("source mirror hook must not go through PAAP webhook: %#v", sourceHook)
	}
	componentHook := createdHooks["/api/v1/repos/paap/shop-dev-components/hooks"]
	componentConfig, ok := componentHook["config"].(map[string]interface{})
	expectedComponentHookURL := jenkinsNotifyCommitHookURL("http://jenkins.shop-dev-ci.svc.cluster.local:8080", server.URL+"/paap/shop-dev-components.git")
	if !ok || componentConfig["url"] != expectedComponentHookURL {
		t.Fatalf("component repo should notify Jenkins SCM polling endpoint, component=%#v", componentHook)
	}
	if componentHook == nil {
		t.Fatalf("expected component repository webhook too, got %#v", createdHooks)
	}
}

func TestEnsureGiteaPushWebhookDeletesLegacyPAAPSourceWebhook(t *testing.T) {
	var deletedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/paap/shop-dev-orders-api-source/hooks":
			_, _ = w.Write([]byte(`[
				{"id":7,"config":{"url":"http://paap-server.paap-system.svc.cluster.local:9090/api/v1/components/42/source-webhook"}},
				{"id":8,"config":{"url":"http://jenkins/git/notifyCommit?url=http%3A%2F%2Fgitea%2Frepo.git"}}
			]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/repos/paap/shop-dev-orders-api-source/hooks/7":
			deletedPath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	err := ensureGiteaPushWebhook(t.Context(), server.URL, "shop-dev-orders-api-source", "http://jenkins/git/notifyCommit?url=http%3A%2F%2Fgitea%2Frepo.git")
	if err != nil {
		t.Fatalf("ensure hook: %v", err)
	}
	if deletedPath != "/api/v1/repos/paap/shop-dev-orders-api-source/hooks/7" {
		t.Fatalf("legacy PAAP source webhook was not deleted, got %q", deletedPath)
	}
}

func TestEnsureComponentJenkinsAutomationReportsInitialBuildTriggerFailure(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{
		baseURL:  "http://jenkins.shop-dev-ci.svc.cluster.local:8080",
		buildErr: errors.New("jenkins unavailable"),
	}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	server := giteaHookServer(t)
	status, warning := ensureComponentJenkinsAutomation(
		t.Context(),
		kpackReadyClient(t),
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		model.Component{
			Name:           "Orders API",
			Type:           "backend",
			DeliveryMode:   "source",
			JenkinsJob:     "shop-dev-orders-api-build",
			PipelineStatus: "planned",
		},
		"orders-api",
		server.URL,
		"shop-dev-components",
		"http://gitea/paap/shop-dev-components.git",
		k8s.KpackBuildEnvironmentSpec{
			Namespace:      "shop-dev-ci",
			RegistryServer: "registry.shop-dev.corp.example.com:5443",
			BuilderImage:   "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest",
		},
		"",
	)

	if status != "pending" {
		t.Fatalf("ci status = %q, want pending", status)
	}
	if !strings.Contains(warning, "Jenkins initial build trigger failed") || !strings.Contains(warning, "jenkins unavailable") {
		t.Fatalf("warning should explain build trigger failure, got %q", warning)
	}
	if jenkins.buildCalls != 1 {
		t.Fatalf("expected one initial Jenkins build attempt, got %d", jenkins.buildCalls)
	}
}

func TestEnsureGiteaPushWebhookCreatesHookWhenMissing(t *testing.T) {
	var created map[string]interface{}
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks":
			if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
				t.Fatalf("decode hook body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	err := ensureGiteaPushWebhook(t.Context(), server.URL, "shop-dev-components", "http://jenkins/git/notifyCommit?url=http%3A%2F%2Fgitea%2Frepo.git")
	if err != nil {
		t.Fatalf("ensure hook: %v", err)
	}
	if strings.Join(paths, ",") != "/api/v1/repos/paap/shop-dev-components/hooks,/api/v1/repos/paap/shop-dev-components/hooks" {
		t.Fatalf("unexpected paths: %#v", paths)
	}
	if created["type"] != "gitea" || created["active"] != true {
		t.Fatalf("unexpected hook body: %#v", created)
	}
	config, ok := created["config"].(map[string]interface{})
	if !ok || config["url"] != "http://jenkins/git/notifyCommit?url=http%3A%2F%2Fgitea%2Frepo.git" || config["content_type"] != "json" {
		t.Fatalf("unexpected hook config: %#v", created["config"])
	}
	events, ok := created["events"].([]interface{})
	if !ok || len(events) != 1 || events[0] != "push" {
		t.Fatalf("unexpected hook events: %#v", created["events"])
	}
}

func giteaHookServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks":
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func kpackReadyClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiextensions scheme: %v", err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			kpackTestCRD("builders.kpack.io"),
			kpackTestCRD("builds.kpack.io"),
			kpackTestCRD("clusterstores.kpack.io"),
			kpackTestCRD("clusterstacks.kpack.io"),
			kpackTestCRD("images.kpack.io"),
			kpackTestCRD("sourceresolvers.kpack.io"),
			kpackTestCRD("clusterlifecycles.kpack.io"),
		).
		Build()
}

func kpackTestCRD(name string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "kpack.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural: strings.TrimSuffix(name, ".kpack.io"),
				Kind:   strings.TrimSuffix(name, "s.kpack.io"),
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1alpha2",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object"},
				},
			}},
		},
	}
}
