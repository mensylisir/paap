package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	paapv1 "paap/api/v1"
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

func TestBuildComponentAutoscalingManifestBuildsHPA(t *testing.T) {
	cfg, err := (model.ComponentConfig{
		Autoscaling: &model.ComponentAutoscaling{
			Enabled:     true,
			Mode:        "hpa",
			MinReplicas: 2,
			MaxReplicas: 5,
			TargetCPU:   70,
		},
	}).JSON()
	if err != nil {
		t.Fatalf("config json: %v", err)
	}
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}
	comp := model.Component{Name: "Orders API", Type: "backend", Image: "registry.local/orders:v1", Version: "v1", Replicas: 2, Config: cfg}

	manifest := BuildComponentAutoscalingManifest(app, env, comp, "orders-api", "billing-prod")
	if !strings.Contains(manifest, "kind: HorizontalPodAutoscaler") {
		t.Fatalf("expected HPA manifest:\n%s", manifest)
	}
	var hpa autoscalingv2.HorizontalPodAutoscaler
	if err := yaml.Unmarshal([]byte(manifest), &hpa); err != nil {
		t.Fatalf("unmarshal hpa: %v\n%s", err, manifest)
	}
	if hpa.Spec.MaxReplicas != 5 || hpa.Spec.MinReplicas == nil || *hpa.Spec.MinReplicas != 2 {
		t.Fatalf("replica bounds = %#v", hpa.Spec)
	}
	if len(hpa.Spec.Metrics) != 1 || hpa.Spec.Metrics[0].Resource == nil || hpa.Spec.Metrics[0].Resource.Name != corev1.ResourceCPU {
		t.Fatalf("expected cpu metric, got %#v", hpa.Spec.Metrics)
	}
}

func TestBuildComponentAutoscalingManifestBuildsKEDAScaledObject(t *testing.T) {
	cfg, err := (model.ComponentConfig{
		Autoscaling: &model.ComponentAutoscaling{
			Enabled:         true,
			Mode:            "keda",
			MinReplicas:     1,
			MaxReplicas:     8,
			PollingInterval: 15,
			CooldownPeriod:  120,
			Triggers: []model.ComponentAutoscalingTrigger{{
				Type:  "rabbitmq",
				Value: "100",
				Metadata: map[string]string{
					"queueName":   "orders",
					"queueLength": "100",
				},
			}},
		},
	}).JSON()
	if err != nil {
		t.Fatalf("config json: %v", err)
	}
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "prod"}
	comp := model.Component{Name: "Orders API", Type: "backend", Image: "registry.local/orders:v1", Version: "v1", Replicas: 1, Config: cfg}

	manifest := BuildComponentAutoscalingManifest(app, env, comp, "orders-api", "billing-prod")
	if !strings.Contains(manifest, "kind: ScaledObject") {
		t.Fatalf("expected KEDA ScaledObject manifest:\n%s", manifest)
	}
	var obj unstructured.Unstructured
	if err := yaml.Unmarshal([]byte(manifest), &obj.Object); err != nil {
		t.Fatalf("unmarshal scaledobject: %v\n%s", err, manifest)
	}
	if obj.GetAPIVersion() != "keda.sh/v1alpha1" || obj.GetKind() != "ScaledObject" {
		t.Fatalf("unexpected gvk %s/%s", obj.GetAPIVersion(), obj.GetKind())
	}
	triggers, ok, err := unstructured.NestedSlice(obj.Object, "spec", "triggers")
	if err != nil || !ok || len(triggers) != 1 {
		t.Fatalf("scaledobject triggers missing: ok=%v err=%v obj=%#v", ok, err, obj.Object)
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

func TestBuildComponentManifestUsesConfiguredContainerPort(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	cfg, err := model.ComponentConfig{ContainerPort: 8000}.JSON()
	if err != nil {
		t.Fatalf("config json: %v", err)
	}
	comp := model.Component{Name: "后端", Type: "backend", Image: "registry.local/api", Version: "v1", Replicas: 1, Config: cfg}

	deployment := BuildComponentDeploymentManifest(app, env, comp, "backend-1", "myapp-dev")
	service := BuildComponentServiceManifest(app, env, comp, "backend-1", "myapp-dev")

	if !strings.Contains(deployment, "containerPort: 8000") {
		t.Fatalf("deployment should expose configured container port:\n%s", deployment)
	}
	if !strings.Contains(service, "targetPort: 8000") {
		t.Fatalf("service should target configured container port:\n%s", service)
	}
	if !strings.Contains(service, "port: 80") {
		t.Fatalf("service should keep default service port:\n%s", service)
	}
}

func TestBuildComponentManifestUsesConfiguredServicePort(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	cfg, err := model.ComponentConfig{ContainerPort: 8888, ServicePort: 8888}.JSON()
	if err != nil {
		t.Fatalf("config json: %v", err)
	}
	comp := model.Component{Name: "Config", Type: "backend", Image: "registry.local/config", Version: "v1", Replicas: 1, Config: cfg}

	service := BuildComponentServiceManifest(app, env, comp, "config", "myapp-dev")

	if !strings.Contains(service, "port: 8888") {
		t.Fatalf("service should expose configured service port:\n%s", service)
	}
	if !strings.Contains(service, "targetPort: 8888") {
		t.Fatalf("service should target configured container port:\n%s", service)
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

func TestBuildComponentManifestUsesRegistryImageAsDeploymentSource(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	comp := model.Component{
		Name:          "订单服务",
		Type:          "backend",
		Image:         "docker.io/library/order:v1.2.3",
		RegistryImage: "10.96.190.247:5000/myapp-dev/order:v1.2.3",
		Replicas:      1,
	}

	manifest := BuildComponentManifest(app, env, comp, "backend-1", "myapp-dev")
	if !strings.Contains(manifest, "image: 10.96.190.247:5000/myapp-dev/order:v1.2.3") {
		t.Fatalf("manifest should use the image selected from the environment registry:\n%s", manifest)
	}
	if strings.Contains(manifest, "image: docker.io/library/order:v1.2.3") {
		t.Fatalf("manifest must not use stale component image when registryImage is set:\n%s", manifest)
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

func TestBuildComponentManifestRendersPaapTemplateDefaultsInConfigObjects(t *testing.T) {
	app := model.Application{Name: "测试应用", Identifier: "myapp"}
	env := model.Environment{Name: "开发", Identifier: "dev"}
	cfg := model.ComponentConfig{
		ConfigMaps: []model.ComponentConfigMap{{
			Name: "orders-api-config",
			Data: map[string]string{
				"application.yml": "server:\n  port: [[paap:APP_PORT default=8081]]\n",
			},
		}},
		Secrets: []model.ComponentSecret{{
			Name: "orders-api-secret",
			Data: map[string]string{
				"CONFIG_SERVICE_PASSWORD": "[[paap:CONFIG_SERVICE_PASSWORD default=cfg-pwd-2026]]",
				"EMPTY_REQUIRED":          "[[paap:EMPTY_REQUIRED]]",
			},
		}},
	}
	configJSON, err := cfg.JSON()
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	comp := model.Component{Name: "订单服务", Type: "backend", Image: "registry.local/orders", Version: "v1", Replicas: 1, Config: configJSON}

	manifest := BuildComponentManifest(app, env, comp, "orders-api", "myapp-dev")

	expected := []string{
		"port: 8081",
		"CONFIG_SERVICE_PASSWORD: cfg-pwd-2026",
		"EMPTY_REQUIRED: \"\"",
	}
	for _, want := range expected {
		if !strings.Contains(manifest, want) {
			t.Fatalf("manifest missing %q:\n%s", want, manifest)
		}
	}
	if strings.Contains(manifest, "[[paap:") {
		t.Fatalf("manifest must not leak PAAP template placeholders:\n%s", manifest)
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

func TestDiscoverComponentToolNamespacesDoesNotInventMissingTools(t *testing.T) {
	namespaces := discoverComponentToolNamespaces(t.Context(), nil, "billing", "dev")
	if namespaces.ArgoCD != "" || namespaces.Gitea != "" || namespaces.Jenkins != "" {
		t.Fatalf("tool namespaces must come from real discovered namespaces, got %#v", namespaces)
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
		"READY_STATUS=\\$(kubectl get image.kpack.io/${KPACK_IMAGE}",
		"[ \"\\${READY_STATUS}\" != \"True\" ] && [ -z \"\\${LATEST_IMAGE}\" ]",
		"kubectl delete image.kpack.io/${KPACK_IMAGE} --wait=true",
		"kubectl delete builds.kpack.io -l image.kpack.io/image=${KPACK_IMAGE} --ignore-not-found=true --wait=true",
		"kubectl apply -f kpack-image.yaml",
		"http://gitea/paap/shop-dev-orders-api-source.git",
		"IMAGE_TAG = \"v1.2.3\"",
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

func TestBuildComponentJenkinsfileDoesNotUseExternalSourceWhenMirrorMissing(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	if strings.Contains(jenkinsfile, `SOURCE_REPO = "https://git.example.com/team/orders.git"`) {
		t.Fatalf("Jenkinsfile must not use external source when mirror is missing:\n%s", jenkinsfile)
	}
	if !strings.Contains(jenkinsfile, `SOURCE_REPO = "http://gitea/paap/shop-dev-components.git"`) {
		t.Fatalf("Jenkinsfile should fall back to the internal component repository when mirror is missing:\n%s", jenkinsfile)
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

func TestBuildComponentJenkinsfileUsesPAAPDeployVersionAsBuildTag(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	if strings.Contains(jenkinsfile, "BUILD_NUMBER") {
		t.Fatalf("Jenkinsfile must use the PAAP deploy version rather than Jenkins build number:\n%s", jenkinsfile)
	}
	if !strings.Contains(jenkinsfile, `IMAGE_NAME = "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api"`) {
		t.Fatalf("Jenkinsfile missing untagged IMAGE_NAME:\n%s", jenkinsfile)
	}
	if !strings.Contains(jenkinsfile, `IMAGE_TAG = "v1.2.3"`) || !strings.Contains(jenkinsfile, `tag: ${IMAGE_NAME}:${IMAGE_TAG}`) {
		t.Fatalf("Jenkinsfile should build the image tag selected by PAAP deploy:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfileDoesNotWriteGitOpsManifests(t *testing.T) {
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
		"stage('Update GitOps Manifest')",
		"GITOPS_DEPLOYMENT",
		"GITOPS_REPO_PUSH_URL",
		"sed -i -E",
		"git remote set-url",
		"git add",
		"git commit",
		"git push",
	} {
		if strings.Contains(jenkinsfile, forbidden) {
			t.Fatalf("Jenkinsfile must not own PAAP/GitOps deployment work (%q):\n%s", forbidden, jenkinsfile)
		}
	}
	for _, want := range []string{
		"stage('Submit Buildpacks Image')",
		"kubectl apply -f kpack-image.yaml",
		"kubectl wait image.kpack.io/${KPACK_IMAGE}",
	} {
		if !strings.Contains(jenkinsfile, want) {
			t.Fatalf("Jenkinsfile missing build step %q:\n%s", want, jenkinsfile)
		}
	}
}

func TestBuildComponentJenkinsfileOmitsGitOpsPushCredentials(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea.internal:3000/paap/shop-dev-components.git", false)

	if strings.Contains(jenkinsfile, "paap123456") || strings.Contains(jenkinsfile, "GITOPS_REPO_PUSH_URL") {
		t.Fatalf("Jenkinsfile must not contain GitOps push credentials:\n%s", jenkinsfile)
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

func TestBuildComponentJenkinsfileSupportsMavenBuildModule(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Gateway",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/gateway:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/piggymetrics.git",
		SourceBranch:  "master",
		BuildContext:  ".",
		BuildModule:   "gateway",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "gateway", "http://gitea/paap/shop-dev-components.git", false)

	for _, want := range []string{
		`BUILD_CONTEXT = "."`,
		`BUILD_MODULE = "gateway"`,
		"- name: BP_MAVEN_BUILT_MODULE",
		`value: "${BUILD_MODULE}"`,
		"- name: BP_MAVEN_BUILD_ARGUMENTS",
		`value: "-pl ${BUILD_MODULE} -am -Dmaven.test.skip=true --no-transfer-progress package"`,
		"- name: BP_JVM_VERSION",
		`value: "8.*"`,
	} {
		if !strings.Contains(jenkinsfile, want) {
			t.Fatalf("Jenkinsfile missing Maven module setting %q:\n%s", want, jenkinsfile)
		}
	}
	if strings.Contains(jenkinsfile, "BP_GO_VERSION") {
		t.Fatalf("Maven build must not inherit Go buildpack settings:\n%s", jenkinsfile)
	}
	if strings.Contains(jenkinsfile, "subPath: ${BUILD_CONTEXT}") {
		t.Fatalf("Maven module builds from repository root and must not force source subPath:\n%s", jenkinsfile)
	}
}

func TestBuildComponentJenkinsfileDoesNotGuessBuildpackLanguage(t *testing.T) {
	comp := model.Component{
		ID:            42,
		Name:          "Orders API",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		SourceBranch:  "main",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "orders-api", "http://gitea/paap/shop-dev-components.git", false)

	for _, unwanted := range []string{"BP_GO_VERSION", "BP_MAVEN_BUILT_MODULE", "BP_JVM_VERSION"} {
		if strings.Contains(jenkinsfile, unwanted) {
			t.Fatalf("Jenkinsfile must not inject language-specific buildpack env %q without explicit component build metadata:\n%s", unwanted, jenkinsfile)
		}
	}
}

func TestBuildComponentJenkinsfileInjectsOptionalBuildpackProxy(t *testing.T) {
	t.Setenv("PAAP_BUILDPACK_HTTP_PROXY", "http://172.20.0.1:10808")
	t.Setenv("PAAP_BUILDPACK_HTTPS_PROXY", "http://172.20.0.1:10808")
	t.Setenv("PAAP_BUILDPACK_NO_PROXY", ".svc,.cluster.local,10.0.0.0/8")
	t.Setenv("PAAP_BUILDPACK_JVM_TYPE", "JDK")
	t.Setenv("PAAP_BUILDPACK_LOG_LEVEL", "DEBUG")

	comp := model.Component{
		ID:            42,
		Name:          "Gateway",
		Type:          "backend",
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/gateway:v1.2.3",
		SourceRepoURL: "https://git.example.com/team/piggymetrics.git",
		SourceBranch:  "master",
		BuildContext:  ".",
		BuildModule:   "gateway",
	}

	jenkinsfile := buildComponentJenkinsfile(comp, "gateway", "http://gitea/paap/shop-dev-components.git", false)

	for _, want := range []string{
		"- name: http_proxy",
		`value: "http://172.20.0.1:10808"`,
		"- name: https_proxy",
		"- name: no_proxy",
		`value: ".svc,.cluster.local,10.0.0.0/8"`,
		"- name: BP_JVM_TYPE",
		`value: "JDK"`,
		"- name: BP_LOG_LEVEL",
		`value: "DEBUG"`,
	} {
		if !strings.Contains(jenkinsfile, want) {
			t.Fatalf("Jenkinsfile missing buildpack proxy setting %q:\n%s", want, jenkinsfile)
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

func TestComponentSourceInternalBuildImageUsesClusterIPForManagedRegistry(t *testing.T) {
	previous := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previous) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shop-dev-registry",
			Namespace: "shop-dev-registry",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "shop-dev-registry",
				"app.kubernetes.io/name":     "registry",
				"paap.io/service-type":       "registry",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.96.190.247",
			Ports:     []corev1.ServicePort{{Name: "registry", Port: 5000}},
		},
	}).Build())

	flow := componentFlowContext{
		App:        model.Application{Identifier: "shop"},
		Env:        model.Environment{Identifier: "dev"},
		Identifier: "orders-api",
		Component: model.Component{
			DeliveryMode:  "source",
			Image:         "registry.shop-dev.paap.local/shop-dev/orders-api:v1.2.3",
			RegistryImage: "registry.shop-dev.paap.local/shop-dev/orders-api:v1.2.3",
			Version:       "v1.2.3",
		},
		Services: []model.ServiceInstallation{{
			ServiceType: "registry",
			Status:      "running",
			Namespace:   "shop-dev-registry",
			ReleaseName: "shop-dev-registry",
		}},
	}

	image := componentSourceInternalBuildImage(t.Context(), flow)

	want := "10.96.190.247:5000/shop-dev/orders-api:v1.2.3"
	if image != want {
		t.Fatalf("internal build image = %q, want %q", image, want)
	}
	if flow.Component.Image != "registry.shop-dev.paap.local/shop-dev/orders-api:v1.2.3" {
		t.Fatalf("source helper must not mutate persisted runtime image, got %q", flow.Component.Image)
	}
}

func TestComponentSourceInternalBuildImageBuildsRepositoryWhenSourceImageIsEmpty(t *testing.T) {
	previous := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previous) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shop-dev-registry",
			Namespace: "shop-dev-registry",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "shop-dev-registry",
				"app.kubernetes.io/name":     "registry",
				"paap.io/service-type":       "registry",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.96.190.247",
			Ports:     []corev1.ServicePort{{Name: "registry", Port: 5000}},
		},
	}).Build())

	flow := componentFlowContext{
		App:        model.Application{Identifier: "shop"},
		Env:        model.Environment{Identifier: "dev"},
		Identifier: "gateway",
		Component: model.Component{
			DeliveryMode: "source",
			Version:      "6bb2cf9",
		},
		Services: []model.ServiceInstallation{{
			ServiceType: "registry",
			Status:      "running",
			Namespace:   "shop-dev-registry",
			ReleaseName: "shop-dev-registry",
		}},
	}

	image := componentSourceInternalBuildImage(t.Context(), flow)

	want := "10.96.190.247:5000/shop-dev/gateway:6bb2cf9"
	if image != want {
		t.Fatalf("internal build image = %q, want %q", image, want)
	}
}

func TestComponentSourceInternalBuildImageUsesSelectedSharedRegistry(t *testing.T) {
	previous := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previous) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared-registry",
			Namespace: "shared-registry",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "shared-registry",
				"app.kubernetes.io/name":     "registry",
				"paap.io/service-type":       "registry",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "10.96.10.20",
			Ports:     []corev1.ServicePort{{Name: "registry", Port: 5000}},
		},
	}).Build())

	cfg, err := model.ComponentConfig{
		RegistryTarget: &model.ComponentRegistryTarget{Key: "capability:9", CapabilityID: 9, Source: model.CapabilitySourceShared},
	}.JSON()
	if err != nil {
		t.Fatalf("encode component config: %v", err)
	}
	local := model.ServiceInstallation{ID: 1, ServiceType: "registry", Status: "running", Namespace: "shop-dev-registry", ReleaseName: "shop-dev-registry"}
	shared := model.ServiceInstallation{ID: 2, ServiceType: "registry", Status: "running", Namespace: "shared-registry", ReleaseName: "shared-registry"}
	flow := componentFlowContext{
		App:        model.Application{Identifier: "shop"},
		Env:        model.Environment{Identifier: "dev"},
		Identifier: "orders-api",
		Component: model.Component{
			DeliveryMode:  "source",
			Image:         "registry.shared.paap.local/shop-dev/orders-api:v1.2.3",
			RegistryImage: "registry.shared.paap.local/shop-dev/orders-api:v1.2.3",
			Version:       "v1.2.3",
			Config:        cfg,
		},
		Targets: []componentDeliveryTarget{
			{Capability: "registry", Source: model.CapabilitySourceManaged, ServiceType: "registry", Service: &local},
			{Capability: "registry", CapabilityID: 9, Source: model.CapabilitySourceShared, ServiceType: "registry", Service: &shared},
		},
	}

	image := componentSourceInternalBuildImage(t.Context(), flow)

	want := "10.96.10.20:5000/shop-dev/orders-api:v1.2.3"
	if image != want {
		t.Fatalf("internal build image = %q, want selected shared registry %q", image, want)
	}
}

func TestComponentSourceInternalBuildImageKeepsExternalRegistryEndpoint(t *testing.T) {
	flow := componentFlowContext{
		App:        model.Application{Identifier: "shop"},
		Env:        model.Environment{Identifier: "dev"},
		Identifier: "orders-api",
		Component: model.Component{
			DeliveryMode: "source",
			Image:        "registry.external.example.com/team/orders-api:v1.2.3",
			Version:      "v1.2.3",
		},
	}

	image := componentSourceInternalBuildImage(t.Context(), flow)

	if image != "registry.external.example.com/team/orders-api:v1.2.3" {
		t.Fatalf("external registry image must stay user-configured, got %q", image)
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

	if spec.Namespace != "" {
		t.Fatalf("namespace = %q", spec.Namespace)
	}
	if spec.RegistryServer != "registry.shop-dev.corp.example.com:5443" {
		t.Fatalf("registry server = %q", spec.RegistryServer)
	}
	if spec.BuilderImage != "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest" {
		t.Fatalf("builder image = %q", spec.BuilderImage)
	}
	if spec.GitServer != "" {
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

func TestBuildComponentKpackSpecMirrorsStackAndBuildpacksForInternalRegistry(t *testing.T) {
	spec, warning := buildComponentKpackSpecWithRegistryCAMirrors(
		t.Context(),
		nil,
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		"orders-api",
		true,
		"10.96.190.247:5000/shop-dev/orders-api:v1",
	)

	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if spec.RegistryServer != "10.96.190.247:5000" {
		t.Fatalf("registry server = %q", spec.RegistryServer)
	}
	if spec.BuilderImage != "10.96.190.247:5000/shop-dev/paap-builder:latest" {
		t.Fatalf("builder image = %q", spec.BuilderImage)
	}
	if spec.StackBuildImage != "10.96.190.247:5000/shop-dev/paap-build-jammy-base:0.1.233" {
		t.Fatalf("stack build image = %q", spec.StackBuildImage)
	}
	if spec.StackRunImage != "10.96.190.247:5000/shop-dev/paap-run-jammy-base:0.1.233" {
		t.Fatalf("stack run image = %q", spec.StackRunImage)
	}
	wantBuildpacks := []string{
		"10.96.190.247:5000/shop-dev/paap-buildpack-java:22.0.0",
		"10.96.190.247:5000/shop-dev/paap-buildpack-nodejs:10.3.2",
		"10.96.190.247:5000/shop-dev/paap-buildpack-go:4.19.14",
		"10.96.190.247:5000/shop-dev/paap-buildpack-python:2.49.0",
	}
	if !reflect.DeepEqual(spec.BuildpackSources, wantBuildpacks) {
		t.Fatalf("buildpack sources = %#v", spec.BuildpackSources)
	}
}

func TestActionEnsureKpackMirrorImagesCopiesPaketoImagesForInternalRegistry(t *testing.T) {
	calls := make([]struct {
		pair componentKpackMirrorImagePair
		opts k8s.ContainerImageCopyOptions
	}, 0)
	previousExists := componentKpackMirrorImageExists
	previousStart := startComponentKpackMirrorImage
	componentKpackMirrorImageExists = func(_ context.Context, target string, opts k8s.ContainerImageCopyOptions) (bool, error) {
		return false, nil
	}
	startComponentKpackMirrorImage = func(pair componentKpackMirrorImagePair, opts k8s.ContainerImageCopyOptions) {
		calls = append(calls, struct {
			pair componentKpackMirrorImagePair
			opts k8s.ContainerImageCopyOptions
		}{pair: pair, opts: opts})
	}
	t.Cleanup(func() {
		componentKpackMirrorImageExists = previousExists
		startComponentKpackMirrorImage = previousStart
	})

	flow := componentFlowContext{
		App:       model.Application{Identifier: "shop"},
		Env:       model.Environment{Identifier: "dev"},
		Component: model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "source"},
		Targets: []componentDeliveryTarget{{
			Capability:  "registry",
			ServiceType: "registry",
			Source:      model.CapabilitySourceManaged,
			Service:     &model.ServiceInstallation{ID: 7, ServiceType: "registry", Namespace: "shop-dev-registry"},
		}},
	}
	spec := k8s.KpackBuildEnvironmentSpec{
		StackBuildImage: "10.96.190.247:5000/shop-dev/paap-build-jammy-base:0.1.233",
		StackRunImage:   "10.96.190.247:5000/shop-dev/paap-run-jammy-base:0.1.233",
		BuildpackSources: []string{
			"10.96.190.247:5000/shop-dev/paap-buildpack-java:22.0.0",
			"10.96.190.247:5000/shop-dev/paap-buildpack-nodejs:10.3.2",
			"10.96.190.247:5000/shop-dev/paap-buildpack-go:4.19.14",
			"10.96.190.247:5000/shop-dev/paap-buildpack-python:2.49.0",
		},
	}

	warning := actionEnsureKpackMirrorImages(t.Context(), flow, spec)
	if !strings.Contains(warning, "kpack base images are being mirrored") {
		t.Fatalf("warning = %q", warning)
	}
	if len(calls) != 6 {
		t.Fatalf("copy calls = %d, want 6: %#v", len(calls), calls)
	}
	if calls[0].pair.source != k8s.PaketoBuildJammyBaseImage || calls[0].pair.target != spec.StackBuildImage {
		t.Fatalf("first copy call = %#v", calls[0])
	}
	if !calls[0].opts.TargetInsecure {
		t.Fatalf("docker registry mirror should use insecure HTTP target: %#v", calls[0].opts)
	}
}

func TestActionEnsureKpackMirrorImagesSkipsExternalRegistry(t *testing.T) {
	calls := 0
	previousStart := startComponentKpackMirrorImage
	startComponentKpackMirrorImage = func(pair componentKpackMirrorImagePair, opts k8s.ContainerImageCopyOptions) {
		calls++
	}
	t.Cleanup(func() { startComponentKpackMirrorImage = previousStart })

	flow := componentFlowContext{
		App:       model.Application{Identifier: "shop"},
		Env:       model.Environment{Identifier: "dev"},
		Component: model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "source"},
		Targets: []componentDeliveryTarget{{
			Capability:       "registry",
			ServiceType:      "registry",
			Source:           model.CapabilitySourceExternal,
			ExternalEndpoint: "registry.example.com:5000",
		}},
	}

	warning := actionEnsureKpackMirrorImages(t.Context(), flow, k8s.KpackBuildEnvironmentSpec{
		StackBuildImage: "registry.example.com/shop-dev/paap-build-jammy-base:0.1.233",
	})
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if calls != 0 {
		t.Fatalf("external registry should not be mirrored by PAAP, calls=%d", calls)
	}
}

func TestActionEnsureKpackMirrorImagesSkipsExistingImages(t *testing.T) {
	calls := 0
	previousExists := componentKpackMirrorImageExists
	previousStart := startComponentKpackMirrorImage
	componentKpackMirrorImageExists = func(_ context.Context, target string, opts k8s.ContainerImageCopyOptions) (bool, error) {
		return true, nil
	}
	startComponentKpackMirrorImage = func(pair componentKpackMirrorImagePair, opts k8s.ContainerImageCopyOptions) {
		calls++
	}
	t.Cleanup(func() {
		componentKpackMirrorImageExists = previousExists
		startComponentKpackMirrorImage = previousStart
	})

	flow := componentFlowContext{
		App:       model.Application{Identifier: "shop"},
		Env:       model.Environment{Identifier: "dev"},
		Component: model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "source"},
		Targets: []componentDeliveryTarget{{
			Capability:  "registry",
			ServiceType: "registry",
			Source:      model.CapabilitySourceManaged,
			Service:     &model.ServiceInstallation{ID: 7, ServiceType: "registry", Namespace: "shop-dev-registry"},
		}},
	}

	warning := actionEnsureKpackMirrorImages(t.Context(), flow, k8s.KpackBuildEnvironmentSpec{
		StackBuildImage: "10.96.190.247:5000/shop-dev/paap-build-jammy-base:0.1.233",
	})
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
	if calls != 0 {
		t.Fatalf("existing images should not be scheduled, calls=%d", calls)
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
	wantBuildpacks := []string{
		"registry.shop-dev.corp.example.com:5443/shop-dev/paap-buildpack-java:22.0.0",
		"registry.shop-dev.corp.example.com:5443/shop-dev/paap-buildpack-nodejs:10.3.2",
		"registry.shop-dev.corp.example.com:5443/shop-dev/paap-buildpack-go:4.19.14",
		"registry.shop-dev.corp.example.com:5443/shop-dev/paap-buildpack-python:2.49.0",
	}
	if !reflect.DeepEqual(spec.BuildpackSources, wantBuildpacks) {
		t.Fatalf("buildpacks should use PAAP registry mirrors, got %#v", spec.BuildpackSources)
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
	var payload map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/repos/migrate" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	comp := model.Component{DeliveryMode: "source", SourceRepoURL: "https://git.example.com/team/orders.git"}
	mirrorURL, err := ensureGiteaSourceMirror(t.Context(), server.URL, "shop-dev-orders-api-source", comp)
	if err != nil {
		t.Fatalf("ensure source mirror: %v", err)
	}
	if mirrorURL != server.URL+"/paap/shop-dev-orders-api-source.git" {
		t.Fatalf("mirror URL = %q", mirrorURL)
	}
	if payload["clone_addr"] != comp.SourceRepoURL || payload["repo_name"] != "shop-dev-orders-api-source" || payload["repo_owner"] != "paap" || payload["mirror"] != true {
		t.Fatalf("unexpected migrate payload: %#v", payload)
	}
}

func TestEnsureGiteaSourceMirrorReturnsMirrorURLWhenExternalMirrorAlreadyExists(t *testing.T) {
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
		t.Fatalf("ensure source mirror: %v", err)
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

func TestEnsureGiteaSourceMirrorReturnsErrorWhenExternalMigrationFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/repos/migrate" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		http.Error(w, "clone failed", http.StatusBadGateway)
	}))
	defer server.Close()

	mirrorURL, err := ensureGiteaSourceMirror(t.Context(), server.URL, "shop-dev-orders-api-source", model.Component{
		DeliveryMode:  "source",
		SourceRepoURL: "https://git.example.com/team/orders.git",
	})
	if err == nil {
		t.Fatalf("expected migration error")
	}
	if mirrorURL != "" {
		t.Fatalf("mirror URL = %q", mirrorURL)
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

func TestRunComponentSourceBuildFlowDoesNotPublishGitOpsManifests(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{baseURL: "http://jenkins.shop-dev-ci.svc.cluster.local:8080"}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	writtenPaths := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/migrate":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			writtenPaths[strings.TrimPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/")] = true
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks" || r.URL.Path == "/api/v1/repos/paap/shop-dev-orders-api-source/hooks"):
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks" || r.URL.Path == "/api/v1/repos/paap/shop-dev-orders-api-source/hooks"):
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	flow := componentFlowContext{
		App:                  model.Application{Identifier: "shop"},
		Env:                  model.Environment{Identifier: "dev"},
		Component:            model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "source", Image: "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1", Version: "v1", SourceRepoURL: "https://git.example.com/shop/orders.git", SourceBranch: "main", JenkinsJob: "shop-dev-orders-api-build", PipelineStatus: "planned"},
		Identifier:           "orders-api",
		Namespace:            "shop-dev",
		K8sClient:            kpackReadyClient(t),
		ToolNamespaces:       componentToolNamespaces{ArgoCD: "shop-dev-deploy", Gitea: "shop-dev-git", Jenkins: "shop-dev-ci"},
		ProjectName:          "shop-dev",
		RepositoryName:       "shop-dev-components",
		RepositoryPath:       "components/orders-api",
		ManifestPath:         "components/orders-api/deployment.yaml",
		GiteaBaseURL:         server.URL,
		RepositoryURL:        server.URL + "/paap/shop-dev-components.git",
		SourceMirrorName:     "shop-dev-orders-api-source",
		ArgoCDApplication:    "shop-dev-orders-api",
		DestinationNamespace: "shop-dev",
	}

	result, err := RunComponentSourceBuildFlow(t.Context(), flow)
	if err != nil {
		t.Fatalf("run source build flow: %v", err)
	}
	if result.ArgoCDApplication != "" {
		t.Fatalf("source build flow must not create ArgoCD application metadata before image exists: %#v", result)
	}
	for _, forbidden := range []string{
		"components/orders-api/deployment.yaml",
		"components/orders-api/service.yaml",
		"components/orders-api/config.yaml",
	} {
		if writtenPaths[forbidden] {
			t.Fatalf("source build flow must not write GitOps manifest %s; written=%#v", forbidden, writtenPaths)
		}
	}
	for _, required := range []string{
		"components/orders-api/Jenkinsfile",
		"components/orders-api/README.md",
	} {
		if !writtenPaths[required] {
			t.Fatalf("source build flow should write %s; written=%#v", required, writtenPaths)
		}
	}
	if jenkins.ensureCalls != 1 || jenkins.buildCalls != 1 {
		t.Fatalf("source build flow should configure and trigger Jenkins once, ensure=%d build=%d", jenkins.ensureCalls, jenkins.buildCalls)
	}
	if result.CIStatus != "running" {
		t.Fatalf("ci status = %q, want running", result.CIStatus)
	}
}

func TestRunComponentSourceDeliveryFlowRetriesPlannedBuildDespiteStaleKpackFailure(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{baseURL: "http://jenkins.shop-dev-ci.svc.cluster.local:8080"}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	writtenPaths := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/repos/migrate":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			writtenPaths[strings.TrimPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/")] = true
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks" || r.URL.Path == "/api/v1/repos/paap/shop-dev-orders-api-source/hooks"):
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && (r.URL.Path == "/api/v1/repos/paap/shop-dev-components/hooks" || r.URL.Path == "/api/v1/repos/paap/shop-dev-orders-api-source/hooks"):
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	redirectHTTPHostForTest(t, "shop-dev-git.shop-dev-git.svc.cluster.local:3000", server.URL)

	k8sClient := kpackReadyClient(t)
	if err := k8sClient.Create(t.Context(), kpackFailedImage("shop-dev-ci", "orders-api", "BuildFailed: failed to build")); err != nil {
		t.Fatalf("seed failed kpack image: %v", err)
	}
	flow := componentFlowContext{
		App:                  model.Application{Identifier: "shop"},
		Env:                  model.Environment{Identifier: "dev"},
		Component:            model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "source", Image: "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1", Version: "v1", SourceRepoURL: "https://git.example.com/shop/orders.git", SourceBranch: "main", JenkinsJob: "shop-dev-orders-api-build", PipelineStatus: "planned"},
		Identifier:           "orders-api",
		Namespace:            "shop-dev",
		K8sClient:            k8sClient,
		ToolNamespaces:       componentToolNamespaces{ArgoCD: "shop-dev-deploy", Gitea: "shop-dev-git", Jenkins: "shop-dev-ci"},
		ProjectName:          "shop-dev",
		RepositoryName:       "shop-dev-components",
		RepositoryPath:       "components/orders-api",
		ManifestPath:         "components/orders-api/deployment.yaml",
		GiteaBaseURL:         server.URL,
		RepositoryURL:        server.URL + "/paap/shop-dev-components.git",
		SourceMirrorName:     "shop-dev-orders-api-source",
		ArgoCDApplication:    "shop-dev-orders-api",
		DestinationNamespace: "shop-dev",
	}
	targets := []componentDeliveryTarget{
		{ServiceType: "git", Source: model.CapabilitySourceManaged, Service: &model.ServiceInstallation{ID: 1, ServiceType: "git", Status: "running", Namespace: "shop-dev-git"}},
		{ServiceType: "ci", Source: model.CapabilitySourceManaged, Service: &model.ServiceInstallation{ID: 2, ServiceType: "ci", Status: "running", Namespace: "shop-dev-ci"}},
	}

	result, err := RunComponentSourceDeliveryFlow(t.Context(), k8sClient, flow.App, flow.Env, flow.Component, flow.Identifier, flow.Namespace, targets)
	if err != nil {
		t.Fatalf("run source delivery retry: %v", err)
	}
	if result.CIStatus != "running" {
		t.Fatalf("planned source retry should resubmit build, status=%q warning=%q", result.CIStatus, result.CIWarning)
	}
	if jenkins.ensureCalls != 1 || jenkins.buildCalls != 1 {
		t.Fatalf("source retry should configure and trigger Jenkins once, ensure=%d build=%d", jenkins.ensureCalls, jenkins.buildCalls)
	}
	if !writtenPaths["components/orders-api/Jenkinsfile"] || !writtenPaths["components/orders-api/README.md"] {
		t.Fatalf("source retry should rewrite build files; written=%#v", writtenPaths)
	}
	if writtenPaths["components/orders-api/deployment.yaml"] || writtenPaths["components/orders-api/service.yaml"] {
		t.Fatalf("source retry must not publish GitOps manifests before image is built; written=%#v", writtenPaths)
	}
}

func TestRunComponentSourceDeliveryFlowKeepsRunningKpackWarningVisible(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{baseURL: "http://jenkins.shop-dev-ci.svc.cluster.local:8080"}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	k8sClient := kpackReadyClient(t)
	if err := k8sClient.Create(t.Context(), kpackRunningImage("shop-dev-ci", "orders-api", "Container export waiting")); err != nil {
		t.Fatalf("seed running kpack image: %v", err)
	}
	targets := []componentDeliveryTarget{
		{ServiceType: "git", Source: model.CapabilitySourceManaged, Service: &model.ServiceInstallation{ID: 1, ServiceType: "git", Status: "running", Namespace: "shop-dev-git"}},
		{ServiceType: "ci", Source: model.CapabilitySourceManaged, Service: &model.ServiceInstallation{ID: 2, ServiceType: "ci", Status: "running", Namespace: "shop-dev-ci"}},
	}

	result, err := RunComponentSourceDeliveryFlow(
		t.Context(),
		k8sClient,
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		model.Component{
			Name:                "Orders API",
			Type:                "backend",
			DeliveryMode:        "source",
			Image:               "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1",
			Version:             "v1",
			SourceRepoURL:       "https://git.example.com/shop/orders.git",
			SourceMirrorRepoURL: "http://gitea/paap/shop-dev-orders-api-source.git",
			SourceBranch:        "main",
			JenkinsJob:          "shop-dev-orders-api-build",
			PipelineStatus:      "running",
		},
		"orders-api",
		"shop-dev",
		targets,
	)
	if err != nil {
		t.Fatalf("run source delivery: %v", err)
	}
	if result.CIStatus != "running" || !strings.Contains(result.CIWarning, "Container export waiting") {
		t.Fatalf("running kpack warning should be surfaced, got status=%q warning=%q", result.CIStatus, result.CIWarning)
	}
	if jenkins.ensureCalls != 0 || jenkins.buildCalls != 0 {
		t.Fatalf("running kpack build must not reconfigure Jenkins, ensure=%d build=%d", jenkins.ensureCalls, jenkins.buildCalls)
	}
	if result.ArgoCDApplication != "" {
		t.Fatalf("running source build must not publish GitOps deployment yet: %#v", result)
	}
}

func TestRunComponentSourceBuildFlowRequiresGitAndCI(t *testing.T) {
	flow := componentFlowContext{
		App:                  model.Application{Identifier: "shop"},
		Env:                  model.Environment{Identifier: "dev"},
		Component:            model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "source", Image: "registry.example.com/shop/orders-api:v1", Version: "v1", SourceRepoURL: "https://git.example.com/shop/orders.git"},
		Identifier:           "orders-api",
		Namespace:            "shop-dev",
		RepositoryName:       "shop-dev-components",
		RepositoryPath:       "components/orders-api",
		ManifestPath:         "components/orders-api/deployment.yaml",
		DestinationNamespace: "shop-dev",
	}

	_, err := RunComponentSourceBuildFlow(t.Context(), flow)
	if err == nil || !strings.Contains(err.Error(), "environment git service is required before source delivery") {
		t.Fatalf("source build without git service error = %v", err)
	}

	flow.ToolNamespaces.Gitea = "shop-dev-git"
	flow.GiteaBaseURL = "http://gitea"
	flow.RepositoryURL = "http://gitea/paap/shop-dev-components.git"
	_, err = RunComponentSourceBuildFlow(t.Context(), flow)
	if err == nil || !strings.Contains(err.Error(), "environment ci service is required before source delivery") {
		t.Fatalf("source build without ci service error = %v", err)
	}
}

func TestRunComponentImageDeliveryFlowPublishesGitOpsAndConfiguresArgoCD(t *testing.T) {
	writtenPaths := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode gitea file body: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(body.Content)
			if err != nil {
				t.Fatalf("decode gitea content: %v", err)
			}
			writtenPaths[strings.TrimPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/")] = string(data)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	k8sClient := gitOpsFlowClient(t)
	app := model.Application{Identifier: "shop"}
	env := model.Environment{Identifier: "dev"}
	comp := model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "image", Image: "registry.shop-dev.svc.cluster.local:5000/shop-dev/orders-api:v1", Version: "v1", Replicas: 2}
	flow := componentFlowContext{
		App:                  app,
		Env:                  env,
		Component:            comp,
		Identifier:           "orders-api",
		Namespace:            "shop-dev",
		K8sClient:            k8sClient,
		ToolNamespaces:       componentToolNamespaces{ArgoCD: "shop-dev-deploy", Gitea: "shop-dev-git", Jenkins: "shop-dev-ci"},
		ProjectName:          "shop-dev",
		RepositoryName:       "shop-dev-components",
		RepositoryPath:       "components/orders-api",
		ManifestPath:         "components/orders-api/deployment.yaml",
		GiteaBaseURL:         server.URL,
		RepositoryURL:        server.URL + "/paap/shop-dev-components.git",
		ArgoCDApplication:    "shop-dev-orders-api",
		DestinationNamespace: "shop-dev",
	}

	result, err := RunComponentGitOpsDeploymentFlow(t.Context(), flow)
	if err != nil {
		t.Fatalf("run image delivery flow: %v", err)
	}
	if result.ArgoCDApplication != "shop-dev-orders-api" || result.RepositoryPath != "components/orders-api" {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, required := range []string{
		"components/orders-api/deployment.yaml",
		"components/orders-api/service.yaml",
	} {
		if strings.TrimSpace(writtenPaths[required]) == "" {
			t.Fatalf("image delivery should write %s; written=%#v", required, writtenPaths)
		}
	}
	if writtenPaths["components/orders-api/Jenkinsfile"] != "" {
		t.Fatalf("image delivery must not write Jenkinsfile; written=%#v", writtenPaths)
	}
	if !strings.Contains(writtenPaths["components/orders-api/deployment.yaml"], "image: registry.shop-dev.svc.cluster.local:5000/shop-dev/orders-api:v1") {
		t.Fatalf("deployment manifest should use selected image:\n%s", writtenPaths["components/orders-api/deployment.yaml"])
	}
	assertGitOpsConfigured(t, k8sClient, "shop-dev-deploy", "shop-dev", "shop-dev-orders-api")
}

func TestReconcileEnvironmentGitOpsPersistsImageDeliveryMetadata(t *testing.T) {
	writtenPaths := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode gitea file body: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(body.Content)
			if err != nil {
				t.Fatalf("decode gitea content: %v", err)
			}
			writtenPaths[strings.TrimPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/")] = string(data)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	redirectHTTPHostForTest(t, "shop-dev-git.shop-dev-git.svc.cluster.local:3000", server.URL)

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "shop-dev-deploy"}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create service installation: %v", err)
	}
	comp := model.Component{EnvironmentID: env.ID, Name: "Orders API", Type: "backend", DeliveryMode: "image", Image: "registry.shop-dev.svc.cluster.local:5000/shop-dev/orders-api:v1", Version: "v1", Replicas: 2, Status: "draft"}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	k8sClient := gitOpsFlowClient(t)
	workspace, errs := ReconcileEnvironmentGitOps(t.Context(), db, k8sClient, app, env, inst, []model.Component{comp})
	if len(errs) > 0 {
		t.Fatalf("reconcile errors: %#v", errs)
	}
	if workspace.Kind != "gitops" || len(workspace.Resources) != 1 {
		t.Fatalf("workspace should be rebuilt from persisted components, got %#v", workspace)
	}
	if strings.TrimSpace(writtenPaths["components/orders-api/deployment.yaml"]) == "" || strings.TrimSpace(writtenPaths["components/orders-api/service.yaml"]) == "" {
		t.Fatalf("reconcile should publish GitOps manifests; written=%#v", writtenPaths)
	}

	var saved model.Component
	if err := db.First(&saved, comp.ID).Error; err != nil {
		t.Fatalf("load saved component: %v", err)
	}
	if saved.GitRepoURL != "http://shop-dev-git.shop-dev-git.svc.cluster.local:3000/paap/shop-dev-components.git" ||
		saved.GitPath != "components/orders-api" ||
		saved.ArgoCDApp != "shop-dev-orders-api" ||
		saved.Status != "syncing" {
		t.Fatalf("component GitOps metadata was not persisted: %#v", saved)
	}
	assertGitOpsConfigured(t, k8sClient, "shop-dev-deploy", "shop-dev", "shop-dev-orders-api")
}

func TestReconcileEnvironmentGitOpsDeploysSourceComponentAfterKpackImageReady(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{baseURL: "http://jenkins.shop-dev-ci.svc.cluster.local:8080"}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	writtenPaths := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode gitea file body: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(body.Content)
			if err != nil {
				t.Fatalf("decode gitea content: %v", err)
			}
			writtenPaths[strings.TrimPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/")] = string(data)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	redirectHTTPHostForTest(t, "shop-dev-git.shop-dev-git.svc.cluster.local:3000", server.URL)

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "shop-dev-deploy"}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create service installation: %v", err)
	}
	comp := model.Component{
		EnvironmentID:       env.ID,
		Name:                "Orders API",
		Type:                "backend",
		DeliveryMode:        "source",
		Image:               "registry.shop-dev.svc.cluster.local:5000/shop-dev/orders-api:v2",
		RegistryImage:       "registry.shop-dev.svc.cluster.local:5000/shop-dev/orders-api:v2",
		Version:             "v2",
		Replicas:            2,
		Status:              "building",
		PipelineStatus:      "running",
		SourceMirrorRepoURL: server.URL + "/paap/shop-dev-orders-api-source.git",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	k8sClient := gitOpsFlowClient(t)
	if err := k8sClient.Create(t.Context(), kpackReadyImage("shop-dev-ci", "orders-api", comp.Image)); err != nil {
		t.Fatalf("create kpack image: %v", err)
	}
	workspace, errs := ReconcileEnvironmentGitOps(t.Context(), db, k8sClient, app, env, inst, []model.Component{comp})
	if len(errs) > 0 {
		t.Fatalf("reconcile errors: %#v", errs)
	}
	if workspace.Kind != "gitops" || len(workspace.Resources) != 1 {
		t.Fatalf("workspace should be rebuilt from persisted components, got %#v", workspace)
	}
	if jenkins.ensureCalls != 0 || jenkins.buildCalls != 0 {
		t.Fatalf("ready source build must not reconfigure Jenkins, ensure=%d build=%d", jenkins.ensureCalls, jenkins.buildCalls)
	}
	if strings.TrimSpace(writtenPaths["components/orders-api/deployment.yaml"]) == "" || strings.TrimSpace(writtenPaths["components/orders-api/service.yaml"]) == "" {
		t.Fatalf("ready source build should publish GitOps manifests; written=%#v", writtenPaths)
	}
	if writtenPaths["components/orders-api/Jenkinsfile"] != "" {
		t.Fatalf("ready source build must not rewrite Jenkinsfile; written=%#v", writtenPaths)
	}

	var saved model.Component
	if err := db.First(&saved, comp.ID).Error; err != nil {
		t.Fatalf("load saved component: %v", err)
	}
	if saved.PipelineStatus != "built" || saved.Status != "syncing" || saved.ArgoCDApp != "shop-dev-orders-api" {
		t.Fatalf("source component was not advanced to GitOps deployment: %#v", saved)
	}
	if saved.SourceMirrorRepoURL != comp.SourceMirrorRepoURL {
		t.Fatalf("source mirror should be retained, got %q", saved.SourceMirrorRepoURL)
	}
	assertGitOpsConfigured(t, k8sClient, "shop-dev-deploy", "shop-dev", "shop-dev-orders-api")
}

func TestCompletePendingComponentSourceDeliveryPublishesGitOpsAfterKpackReady(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{app}-{env}.paap.local")

	writtenPaths := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode gitea file body: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(body.Content)
			if err != nil {
				t.Fatalf("decode gitea content: %v", err)
			}
			writtenPaths[strings.TrimPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/")] = string(data)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()
	redirectHTTPHostForTest(t, "shop-dev-git.shop-dev-git.svc.cluster.local:3000", server.URL)

	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	for _, inst := range []model.ServiceInstallation{
		{EnvironmentID: env.ID, ServiceName: "dev-git", ServiceType: "git", Status: "running", Namespace: "shop-dev-git", ReleaseName: "shop-dev-git"},
		{EnvironmentID: env.ID, ServiceName: "dev-ci", ServiceType: "ci", Status: "running", Namespace: "shop-dev-ci", ReleaseName: "shop-dev-ci"},
		{EnvironmentID: env.ID, ServiceName: "dev-deploy", ServiceType: "deploy", Status: "running", Namespace: "shop-dev-deploy", ReleaseName: "shop-dev-deploy"},
		{EnvironmentID: env.ID, ServiceName: "dev-registry", ServiceType: "registry", Status: "running", Namespace: "shop-dev-registry", ReleaseName: "shop-dev-registry"},
	} {
		if err := db.Create(&inst).Error; err != nil {
			t.Fatalf("create service installation: %v", err)
		}
	}
	comp := model.Component{
		EnvironmentID:       env.ID,
		Name:                "Orders API",
		Type:                "backend",
		DeliveryMode:        "source",
		Image:               "registry.shop-dev.paap.local/shop-dev/orders-api:v2",
		RegistryImage:       "registry.shop-dev.paap.local/shop-dev/orders-api:v2",
		Version:             "v2",
		Replicas:            2,
		Status:              "building",
		PipelineStatus:      "running",
		SourceMirrorRepoURL: server.URL + "/paap/shop-dev-orders-api-source.git",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	k8sClient := gitOpsFlowClient(t)
	if err := k8sClient.Create(t.Context(), kpackReadyImage("shop-dev-ci", "orders-api", "10.96.190.247:5000/shop-dev/orders-api@sha256:abc")); err != nil {
		t.Fatalf("create kpack image: %v", err)
	}
	previous := k8s.GetClient()
	k8s.SetClient(k8sClient)
	t.Cleanup(func() { k8s.SetClient(previous) })

	done, err := completePendingComponentSourceDelivery(t.Context(), db, k8sClient, comp.ID)
	if err != nil {
		t.Fatalf("complete pending source delivery: %v", err)
	}
	if !done {
		t.Fatalf("ready kpack image should complete source delivery")
	}
	deployment := writtenPaths["components/orders-api/deployment.yaml"]
	if !strings.Contains(deployment, "image: registry.shop-dev.paap.local/shop-dev/orders-api:v2") {
		t.Fatalf("deployment should use runtime registry image, got:\n%s", deployment)
	}
	if strings.Contains(deployment, "10.96.190.247:5000") {
		t.Fatalf("deployment must not publish internal build image, got:\n%s", deployment)
	}
	var saved model.Component
	if err := db.First(&saved, comp.ID).Error; err != nil {
		t.Fatalf("load saved component: %v", err)
	}
	if saved.PipelineStatus != "built" || saved.Status != "syncing" || saved.ArgoCDApp != "shop-dev-orders-api" {
		t.Fatalf("component should be advanced to GitOps deployment: %#v", saved)
	}
	runtimeComp := &paapv1.Component{}
	if err := k8sClient.Get(t.Context(), types.NamespacedName{Name: "dev-orders-api", Namespace: "paap-app-shop"}, runtimeComp); err != nil {
		t.Fatalf("runtime component CR should be upserted: %v", err)
	}
	if runtimeComp.Spec.Deployment.Image != "registry.shop-dev.paap.local/shop-dev/orders-api:v2" {
		t.Fatalf("runtime component CR image = %q", runtimeComp.Spec.Deployment.Image)
	}
	assertGitOpsConfigured(t, k8sClient, "shop-dev-deploy", "shop-dev", "shop-dev-orders-api")
}

func TestReconcileEnvironmentGitOpsMarksFailedKpackSourceBuild(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	app := model.Application{Name: "Shop", Identifier: "shop"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "shop-dev-deploy"}
	ci := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "ci", Status: "running", Namespace: "shop-dev-ci"}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create deploy service: %v", err)
	}
	if err := db.Create(&ci).Error; err != nil {
		t.Fatalf("create ci service: %v", err)
	}
	comp := model.Component{
		EnvironmentID:       env.ID,
		Name:                "Orders API",
		Type:                "backend",
		DeliveryMode:        "source",
		Image:               "registry.shop-dev.paap.local/shop-dev/orders-api:v2",
		RegistryImage:       "registry.shop-dev.paap.local/shop-dev/orders-api:v2",
		Version:             "v2",
		Replicas:            1,
		Status:              "building",
		PipelineStatus:      "running",
		SourceMirrorRepoURL: "http://gitea/paap/shop-dev-orders-api-source.git",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	k8sClient := gitOpsFlowClient(t)
	if err := k8sClient.Create(t.Context(), kpackFailedImage("shop-dev-ci", "orders-api", "BuildFailed: failed to build: exit status 1")); err != nil {
		t.Fatalf("create failed kpack image: %v", err)
	}
	_, errs := ReconcileEnvironmentGitOps(t.Context(), db, k8sClient, app, env, inst, []model.Component{comp})
	if len(errs) > 0 {
		t.Fatalf("reconcile errors: %#v", errs)
	}

	var saved model.Component
	if err := db.First(&saved, comp.ID).Error; err != nil {
		t.Fatalf("load saved component: %v", err)
	}
	if saved.PipelineStatus != "failed" || saved.Status != "error" {
		t.Fatalf("failed build status not persisted: pipeline=%q status=%q", saved.PipelineStatus, saved.Status)
	}
	if !strings.Contains(saved.ErrorMessage, "BuildFailed") {
		t.Fatalf("error message should include kpack failure, got %q", saved.ErrorMessage)
	}
	if saved.Image != comp.Image || saved.RegistryImage != comp.RegistryImage {
		t.Fatalf("failed build must not rewrite declared image: image=%q registry=%q", saved.Image, saved.RegistryImage)
	}
}

func TestRunComponentImageDeliveryFlowRequiresGitAndCD(t *testing.T) {
	flow := componentFlowContext{
		App:                  model.Application{Identifier: "shop"},
		Env:                  model.Environment{Identifier: "dev"},
		Component:            model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "image", Image: "registry.example.com/shop/orders-api:v1", Version: "v1", Replicas: 1},
		Identifier:           "orders-api",
		Namespace:            "shop-dev",
		RepositoryName:       "shop-dev-components",
		RepositoryPath:       "components/orders-api",
		ManifestPath:         "components/orders-api/deployment.yaml",
		ArgoCDApplication:    "shop-dev-orders-api",
		DestinationNamespace: "shop-dev",
	}

	_, err := RunComponentGitOpsDeploymentFlow(t.Context(), flow)
	if err == nil || !strings.Contains(err.Error(), "environment git service is required before GitOps delivery") {
		t.Fatalf("image delivery without git service error = %v", err)
	}

	flow.ToolNamespaces.Gitea = "shop-dev-git"
	flow.GiteaBaseURL = "http://gitea"
	flow.RepositoryURL = "http://gitea/paap/shop-dev-components.git"
	_, err = RunComponentGitOpsDeploymentFlow(t.Context(), flow)
	if err == nil || !strings.Contains(err.Error(), "environment cd service is required before GitOps delivery") {
		t.Fatalf("image delivery without cd service error = %v", err)
	}
}

type rewriteHostRoundTripper struct {
	base   http.RoundTripper
	source string
	target *url.URL
}

func (r rewriteHostRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == r.source {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = r.target.Scheme
		clone.URL.Host = r.target.Host
		clone.Host = r.target.Host
		req = clone
	}
	return r.base.RoundTrip(req)
}

func redirectHTTPHostForTest(t *testing.T, sourceHost, targetURL string) {
	t.Helper()
	target, err := url.Parse(targetURL)
	if err != nil {
		t.Fatalf("parse target url: %v", err)
	}
	previous := http.DefaultTransport
	base := previous
	if base == nil {
		base = http.DefaultTransport
	}
	http.DefaultTransport = rewriteHostRoundTripper{base: base, source: sourceHost, target: target}
	t.Cleanup(func() { http.DefaultTransport = previous })
}

func TestRunComponentSourceDeliveryFlowDeploysGitOpsAfterImageIsBuilt(t *testing.T) {
	jenkins := &fakeComponentJenkinsClient{baseURL: "http://jenkins.shop-dev-ci.svc.cluster.local:8080"}
	previousFactory := newComponentJenkinsClient
	newComponentJenkinsClient = func(_ string) componentJenkinsClient {
		return jenkins
	}
	t.Cleanup(func() { newComponentJenkinsClient = previousFactory })

	writtenPaths := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/orders-api/"):
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode gitea file body: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(body.Content)
			if err != nil {
				t.Fatalf("decode gitea content: %v", err)
			}
			writtenPaths[strings.TrimPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/")] = string(data)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	k8sClient := gitOpsFlowClient(t)
	app := model.Application{Identifier: "shop"}
	env := model.Environment{Identifier: "dev"}
	comp := model.Component{Name: "Orders API", Type: "backend", DeliveryMode: "source", Image: "registry.shop-dev.svc.cluster.local:5000/shop-dev/orders-api:v2", Version: "v2", Replicas: 1, PipelineStatus: "built", SourceMirrorRepoURL: server.URL + "/paap/shop-dev-orders-api-source.git"}
	flow := componentFlowContext{
		App:                  app,
		Env:                  env,
		Component:            comp,
		Identifier:           "orders-api",
		Namespace:            "shop-dev",
		K8sClient:            k8sClient,
		ToolNamespaces:       componentToolNamespaces{ArgoCD: "shop-dev-deploy", Gitea: "shop-dev-git", Jenkins: "shop-dev-ci"},
		ProjectName:          "shop-dev",
		RepositoryName:       "shop-dev-components",
		RepositoryPath:       "components/orders-api",
		ManifestPath:         "components/orders-api/deployment.yaml",
		GiteaBaseURL:         server.URL,
		RepositoryURL:        server.URL + "/paap/shop-dev-components.git",
		ArgoCDApplication:    "shop-dev-orders-api",
		DestinationNamespace: "shop-dev",
	}

	result, err := RunComponentGitOpsDeploymentFlow(t.Context(), flow)
	if err != nil {
		t.Fatalf("run built source gitops flow: %v", err)
	}
	if result.SourceMirrorURL != comp.SourceMirrorRepoURL {
		t.Fatalf("source mirror URL should be retained, got %#v", result)
	}
	if jenkins.ensureCalls != 0 || jenkins.buildCalls != 0 {
		t.Fatalf("built source deployment must not configure Jenkins again, ensure=%d build=%d", jenkins.ensureCalls, jenkins.buildCalls)
	}
	if strings.TrimSpace(writtenPaths["components/orders-api/deployment.yaml"]) == "" || strings.TrimSpace(writtenPaths["components/orders-api/service.yaml"]) == "" {
		t.Fatalf("built source delivery should write deployment and service manifests; written=%#v", writtenPaths)
	}
	if writtenPaths["components/orders-api/Jenkinsfile"] != "" {
		t.Fatalf("built source deployment must not rewrite Jenkinsfile; written=%#v", writtenPaths)
	}
	assertGitOpsConfigured(t, k8sClient, "shop-dev-deploy", "shop-dev", "shop-dev-orders-api")
}

func TestRunComponentGitOpsDeploymentFlowUsesRuntimeImageForBuiltSource(t *testing.T) {
	t.Setenv(RegistryHostTemplateEnv, "registry.{app}-{env}.paap.local")

	writtenPaths := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "paap" || pass != "paap123456" {
			t.Fatalf("missing gitea basic auth")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/gateway/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/components/gateway/"):
			var body struct {
				Content string `json:"content"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode gitea file body: %v", err)
			}
			data, err := base64.StdEncoding.DecodeString(body.Content)
			if err != nil {
				t.Fatalf("decode gitea content: %v", err)
			}
			writtenPaths[strings.TrimPrefix(r.URL.Path, "/api/v1/repos/paap/shop-dev-components/contents/")] = string(data)
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	k8sClient := gitOpsFlowClient(t)
	flow := componentFlowContext{
		App:                  model.Application{Identifier: "piggymetrics"},
		Env:                  model.Environment{Identifier: "dev"},
		Component:            model.Component{Name: "gateway", Type: "frontend", DeliveryMode: "source", Image: "10.96.190.247:5000/piggymetrics-dev/gateway:6bb2cf9", RegistryImage: "10.96.190.247:5000/piggymetrics-dev/gateway:6bb2cf9", Version: "6bb2cf9", PipelineStatus: "built", Replicas: 1},
		Identifier:           "gateway",
		Namespace:            "piggymetrics-dev",
		K8sClient:            k8sClient,
		ToolNamespaces:       componentToolNamespaces{ArgoCD: "piggymetrics-dev-argocd", Gitea: "piggymetrics-dev-gitea"},
		ProjectName:          "piggymetrics-dev",
		RepositoryName:       "shop-dev-components",
		RepositoryPath:       "components/gateway",
		ManifestPath:         "components/gateway/deployment.yaml",
		GiteaBaseURL:         server.URL,
		RepositoryURL:        server.URL + "/paap/shop-dev-components.git",
		ArgoCDApplication:    "piggymetrics-dev-gateway",
		DestinationNamespace: "piggymetrics-dev",
		Targets: []componentDeliveryTarget{{
			Capability:  "registry",
			Source:      model.CapabilitySourceManaged,
			ServiceType: "registry",
			App:         model.Application{Identifier: "piggymetrics"},
			Env:         model.Environment{Identifier: "dev"},
		}},
	}

	if _, err := RunComponentGitOpsDeploymentFlow(t.Context(), flow); err != nil {
		t.Fatalf("run built source gitops flow: %v", err)
	}
	deployment := writtenPaths["components/gateway/deployment.yaml"]
	if !strings.Contains(deployment, "image: registry.piggymetrics-dev.paap.local/piggymetrics-dev/gateway:6bb2cf9") {
		t.Fatalf("built source deployment must use runtime registry image:\n%s", deployment)
	}
	if strings.Contains(deployment, "image: 10.96.190.247:5000/piggymetrics-dev/gateway:6bb2cf9") {
		t.Fatalf("built source deployment must not publish internal build image:\n%s", deployment)
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

func gitOpsFlowClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: "paap-app-shop",
			}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: "shop-dev",
				Labels: map[string]string{
					"paap.io/app":  "shop",
					"paap.io/env":  "dev",
					"paap.io/role": "workload",
				},
			}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: "shop-dev-deploy",
				Labels: map[string]string{
					"paap.io/app":          "shop",
					"paap.io/env":          "dev",
					"paap.io/role":         "tool",
					"paap.io/service-type": "deploy",
					"paap.io/tool":         "argocd",
				},
			}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: "shop-dev-git",
				Labels: map[string]string{
					"paap.io/app":          "shop",
					"paap.io/env":          "dev",
					"paap.io/role":         "tool",
					"paap.io/service-type": "git",
					"paap.io/tool":         "gitea",
				},
			}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
				Name: "shop-dev-ci",
				Labels: map[string]string{
					"paap.io/app":          "shop",
					"paap.io/env":          "dev",
					"paap.io/role":         "tool",
					"paap.io/service-type": "ci",
					"paap.io/tool":         "jenkins",
				},
			}},
		).
		Build()
}

func kpackReadyImage(namespace, name, latestImage string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kpack.io/v1alpha2",
		"kind":       "Image",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"status": map[string]interface{}{
			"latestImage": latestImage,
			"conditions": []interface{}{
				map[string]interface{}{
					"type":   "Ready",
					"status": "True",
				},
			},
		},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"})
	return obj
}

func kpackFailedImage(namespace, name, message string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kpack.io/v1alpha2",
		"kind":       "Image",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":    "Ready",
					"status":  "False",
					"reason":  "BuildFailed",
					"message": message,
				},
			},
		},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"})
	return obj
}

func kpackRunningImage(namespace, name, message string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kpack.io/v1alpha2",
		"kind":       "Image",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{
					"type":    "Ready",
					"status":  "Unknown",
					"message": message,
				},
			},
		},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"})
	return obj
}

func assertGitOpsConfigured(t *testing.T, k8sClient client.Client, namespace, projectName, applicationName string) {
	t.Helper()

	repoSecret := &corev1.Secret{}
	if err := k8sClient.Get(t.Context(), client.ObjectKey{Namespace: namespace, Name: "paap-repo-shop-dev-components"}, repoSecret); err != nil {
		t.Fatalf("get argocd repo secret: %v", err)
	}
	if string(repoSecret.Data["type"]) != "git" || string(repoSecret.Data["forceHttpBasicAuth"]) != "true" {
		t.Fatalf("repo secret should use HTTP basic auth, got %#v", repoSecret.Data)
	}

	project := &unstructured.Unstructured{}
	project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	if err := k8sClient.Get(t.Context(), client.ObjectKey{Namespace: namespace, Name: projectName}, project); err != nil {
		t.Fatalf("get argocd project: %v", err)
	}
	destinations, _, _ := unstructured.NestedSlice(project.Object, "spec", "destinations")
	destinationNamespaces := map[string]bool{}
	for _, item := range destinations {
		dest, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("invalid destination item: %#v", item)
		}
		namespaceValue, _ := dest["namespace"].(string)
		serverValue, _ := dest["server"].(string)
		if namespaceValue == "*" || serverValue == "*" {
			t.Fatalf("project must not allow wildcard destinations: %#v", destinations)
		}
		destinationNamespaces[namespaceValue] = true
	}
	if !destinationNamespaces["shop-dev"] {
		t.Fatalf("project should target workload namespace, got %#v", destinations)
	}
	for _, forbidden := range []string{"shop-dev-deploy", "shop-dev-git", "shop-dev-ci"} {
		if destinationNamespaces[forbidden] {
			t.Fatalf("project must not target tool namespace %s: %#v", forbidden, destinations)
		}
	}

	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := k8sClient.Get(t.Context(), client.ObjectKey{Namespace: namespace, Name: applicationName}, app); err != nil {
		t.Fatalf("get argocd application: %v", err)
	}
	if got, _, _ := unstructured.NestedString(app.Object, "spec", "project"); got != projectName {
		t.Fatalf("application project = %q, want %q", got, projectName)
	}
	if got := app.GetAnnotations()["argocd.argoproj.io/refresh"]; got != "hard" {
		t.Fatalf("application refresh annotation = %q, want hard", got)
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
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      k8s.KpackControllerDeploymentName,
					Namespace: k8s.KpackSystemNamespace,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas:     1,
					AvailableReplicas: 1,
				},
			},
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
