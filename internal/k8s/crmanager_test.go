package k8s

import (
	"context"
	"testing"

	paapv1 "paap/api/v1"
	"paap/internal/model"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestUpsertServiceInstanceCRUpdatesExistingSpec(t *testing.T) {
	previousClient := k8sClient
	t.Cleanup(func() { k8sClient = previousClient })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	existing := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "staging-log",
			Namespace: "paap-app-test",
			Labels:    map[string]string{"paap.io/app": "test"},
		},
		Spec: paapv1.ServiceInstanceSpec{
			Type:          "log",
			ToolNamespace: "old-tool-ns",
			WorkloadRole:  paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{Resources: []string{"pods"}, Verbs: []string{"get"}}}},
			Helm: &paapv1.HelmInstallSpec{
				ReleaseName: "old-release",
				Namespace:   "old-tool-ns",
			},
		},
	}
	k8sClient = fake.NewClientBuilder().WithScheme(testScheme).WithObjects(existing).Build()

	updatedRole := paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"pods", "pods/log"},
		Verbs:     []string{"get", "list", "watch"},
	}}}
	helmSpec := &paapv1.HelmInstallSpec{
		ReleaseName: "test-staging-log",
		Namespace:   "test-staging-log",
		Values: map[string]string{
			"paap.envNamespaces": "test-staging,test-staging-app",
			"tool_namespace":     "test-staging-log",
		},
	}
	labels := map[string]string{
		"paap.io/service-type":  "log",
		"paap.io/tool":          "loki",
		"paap.io/category":      "tool",
		"paap.io/resource-role": "service-instance",
	}

	if err := UpsertServiceInstanceCR(context.Background(), "test", "staging", "log", updatedRole, updatedRole, nil, nil, nil, helmSpec, labels, nil); err != nil {
		t.Fatalf("upsert serviceinstance: %v", err)
	}

	var got paapv1.ServiceInstance
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "staging-log", Namespace: "paap-app-test"}, &got); err != nil {
		t.Fatalf("get updated serviceinstance: %v", err)
	}
	if got.Spec.ToolNamespace != "old-tool-ns" {
		t.Fatalf("tool namespace = %q", got.Spec.ToolNamespace)
	}
	if got.Spec.Helm == nil || got.Spec.Helm.Values["paap.envNamespaces"] != "test-staging,test-staging-app" {
		t.Fatalf("helm values not refreshed: %#v", got.Spec.Helm)
	}
	if got.Spec.Helm.Namespace != "old-tool-ns" || got.Spec.Helm.ReleaseName != "old-release" || got.Spec.Helm.Values["tool_namespace"] != "old-tool-ns" {
		t.Fatalf("existing install namespace/release must be preserved: %#v", got.Spec.Helm)
	}
	if got.Spec.WorkloadRole.Rules[0].Resources[1] != "pods/log" {
		t.Fatalf("workload role not refreshed: %#v", got.Spec.WorkloadRole)
	}
	if got.Labels["paap.io/service-type"] != "log" || got.Labels["paap.io/tool"] != "loki" || got.Labels["paap.io/category"] != "tool" {
		t.Fatalf("service metadata not refreshed: %#v", got.Labels)
	}
	if got.Annotations["paap.io/template-synced-at"] == "" {
		t.Fatalf("expected template sync annotation: %#v", got.Annotations)
	}
}

func TestCreateServiceInstanceCRUsesManifestMappedServiceAccount(t *testing.T) {
	previousClient := k8sClient
	t.Cleanup(func() { k8sClient = previousClient })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient = fake.NewClientBuilder().WithScheme(testScheme).Build()

	helmSpec := &paapv1.HelmInstallSpec{
		ReleaseName: "test-staging-redis",
		Namespace:   "test-staging-redis",
		PlatformManifest: `{
			"name":"redis",
			"version":"v1",
			"variable_mapping":[
				{"platform_var":"tool_namespace","helm_var":"serviceAccount.name"}
			]
		}`,
		Values: map[string]string{
			"serviceAccount.name": "test-staging-redis",
			"fullnameOverride":    "test-staging-redis",
		},
	}
	role := paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"secrets"},
		Verbs:     []string{"get"},
	}}}

	if err := CreateServiceInstanceCR(context.Background(), "test", "staging", "redis", paapv1.RoleSpec{}, role, nil, nil, nil, helmSpec, nil, nil); err != nil {
		t.Fatalf("create serviceinstance: %v", err)
	}

	var got paapv1.ServiceInstance
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "staging-redis", Namespace: "paap-app-test"}, &got); err != nil {
		t.Fatalf("get serviceinstance: %v", err)
	}
	if got.Spec.ServiceAccount.Name != got.Spec.Helm.Values["serviceAccount.name"] {
		t.Fatalf("service instance RBAC subject must match helm chart serviceAccount.name: spec=%q helm=%q", got.Spec.ServiceAccount.Name, got.Spec.Helm.Values["serviceAccount.name"])
	}
	if got.Spec.ServiceAccount.Name != "test-staging-redis" {
		t.Fatalf("service account = %q", got.Spec.ServiceAccount.Name)
	}
	if got.Spec.Helm.Values["serviceAccount.name"] != "test-staging-redis" {
		t.Fatalf("helm serviceAccount.name changed unexpectedly: %#v", got.Spec.Helm.Values)
	}
}

func TestUpsertComponentCRCreatesThenUpdates(t *testing.T) {
	previousClient := k8sClient
	t.Cleanup(func() { k8sClient = previousClient })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient = fake.NewClientBuilder().WithScheme(testScheme).Build()

	key := types.NamespacedName{Name: "staging-orders-api", Namespace: "paap-app-shop"}
	initial := &paapv1.Component{}
	if err := k8sClient.Get(context.Background(), key, initial); !apierrors.IsNotFound(err) {
		t.Fatalf("expected component to be absent before upsert, got %v", err)
	}

	if err := UpsertComponentCR(context.Background(), "shop", "staging", "Orders API", "orders-api", "backend", "registry/orders-api", "v1", 1, "shop-staging", "argocd", model.ComponentConfig{}); err != nil {
		t.Fatalf("create component cr: %v", err)
	}
	created := &paapv1.Component{}
	if err := k8sClient.Get(context.Background(), key, created); err != nil {
		t.Fatalf("get created component: %v", err)
	}
	if created.Spec.Deployment.Image != "registry/orders-api" || created.Spec.Deployment.Tag != "v1" || created.Spec.Deployment.Replicas != 1 {
		t.Fatalf("unexpected created deployment spec: %#v", created.Spec.Deployment)
	}

	if err := UpsertComponentCR(context.Background(), "shop", "staging", "Orders API", "orders-api", "backend", "registry/orders-api", "v2", 3, "shop-staging", "argocd", model.ComponentConfig{}); err != nil {
		t.Fatalf("update component cr: %v", err)
	}
	updated := &paapv1.Component{}
	if err := k8sClient.Get(context.Background(), key, updated); err != nil {
		t.Fatalf("get updated component: %v", err)
	}
	if updated.Spec.Deployment.Tag != "v2" || updated.Spec.Deployment.Replicas != 3 {
		t.Fatalf("unexpected updated deployment spec: %#v", updated.Spec.Deployment)
	}
}

func TestUpsertComponentCRExposesFrontendAsNodePort(t *testing.T) {
	previousClient := k8sClient
	t.Cleanup(func() { k8sClient = previousClient })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient = fake.NewClientBuilder().WithScheme(testScheme).Build()

	if err := UpsertComponentCR(context.Background(), "shop", "staging", "Web", "web", "frontend", "registry/web", "v1", 1, "shop-staging", "argocd", model.ComponentConfig{}); err != nil {
		t.Fatalf("create frontend component cr: %v", err)
	}
	if err := UpsertComponentCR(context.Background(), "shop", "staging", "API", "api", "backend", "registry/api", "v1", 1, "shop-staging", "argocd", model.ComponentConfig{}); err != nil {
		t.Fatalf("create backend component cr: %v", err)
	}

	var frontend paapv1.Component
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "staging-web", Namespace: "paap-app-shop"}, &frontend); err != nil {
		t.Fatalf("get frontend component: %v", err)
	}
	if frontend.Spec.Service == nil || frontend.Spec.Service.Type != "NodePort" {
		t.Fatalf("frontend service type = %#v, want NodePort", frontend.Spec.Service)
	}
	if frontend.Spec.Service.TargetPort != 80 {
		t.Fatalf("frontend targetPort = %d, want 80", frontend.Spec.Service.TargetPort)
	}

	var backend paapv1.Component
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "staging-api", Namespace: "paap-app-shop"}, &backend); err != nil {
		t.Fatalf("get backend component: %v", err)
	}
	if backend.Spec.Service == nil || backend.Spec.Service.Type != "ClusterIP" {
		t.Fatalf("backend service type = %#v, want ClusterIP", backend.Spec.Service)
	}
	if backend.Spec.Service.TargetPort != 8080 {
		t.Fatalf("backend targetPort = %d, want 8080", backend.Spec.Service.TargetPort)
	}
}

func TestUpsertComponentCRRepairsFrontendServiceDefaults(t *testing.T) {
	previousClient := k8sClient
	t.Cleanup(func() { k8sClient = previousClient })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	existing := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: "staging-web", Namespace: "paap-app-shop"},
		Spec: paapv1.ComponentSpec{
			Name:       "Web",
			Identifier: "web",
			Type:       "frontend",
			ManagedBy:  "argocd",
			Deployment: paapv1.DeploymentSpec{Namespace: "shop-staging", Image: "registry/web", Tag: "old", Replicas: 1},
			Service:    &paapv1.ServiceSpec{Port: 80, TargetPort: 8080, Type: "ClusterIP"},
		},
	}
	k8sClient = fake.NewClientBuilder().WithScheme(testScheme).WithObjects(existing).Build()

	if err := UpsertComponentCR(context.Background(), "shop", "staging", "Web", "web", "frontend", "registry/web", "v2", 1, "shop-staging", "argocd", model.ComponentConfig{}); err != nil {
		t.Fatalf("update frontend component cr: %v", err)
	}

	var updated paapv1.Component
	if err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "staging-web", Namespace: "paap-app-shop"}, &updated); err != nil {
		t.Fatalf("get updated frontend component: %v", err)
	}
	if updated.Spec.Service == nil || updated.Spec.Service.Type != "NodePort" || updated.Spec.Service.TargetPort != 80 {
		t.Fatalf("updated frontend service = %#v, want NodePort targetPort 80", updated.Spec.Service)
	}
}

func TestUpsertComponentCRPersistsEnvVars(t *testing.T) {
	previousClient := k8sClient
	t.Cleanup(func() { k8sClient = previousClient })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8sClient = fake.NewClientBuilder().WithScheme(testScheme).Build()

	envVars := []paapv1.EnvVar{
		{Name: "DATABASE_URL", Value: "postgres://orders"},
		{
			Name: "DB_PASSWORD",
			ValueFrom: &paapv1.EnvVarSource{
				SecretKeyRef: &paapv1.SecretKeySelector{Name: "orders-db", Key: "password"},
			},
		},
		{
			Name: "REDIS_HOST",
			ValueFrom: &paapv1.EnvVarSource{
				ConfigMapKeyRef: &paapv1.ConfigMapKeySelector{Name: "redis-config", Key: "host"},
			},
		},
	}
	if err := UpsertComponentCR(context.Background(), "shop", "staging", "Orders API", "orders-api", "backend", "registry/orders-api", "v1", 1, "shop-staging", "argocd", model.ComponentConfig{}, envVars); err != nil {
		t.Fatalf("create component cr: %v", err)
	}
	key := types.NamespacedName{Name: "staging-orders-api", Namespace: "paap-app-shop"}
	created := &paapv1.Component{}
	if err := k8sClient.Get(context.Background(), key, created); err != nil {
		t.Fatalf("get created component: %v", err)
	}
	if len(created.Spec.Deployment.Env) != 3 {
		t.Fatalf("created env count = %d, want 3", len(created.Spec.Deployment.Env))
	}
	if created.Spec.Deployment.Env[1].ValueFrom == nil || created.Spec.Deployment.Env[1].ValueFrom.SecretKeyRef.Name != "orders-db" {
		t.Fatalf("secret env not persisted: %#v", created.Spec.Deployment.Env[1])
	}
	if created.Spec.Deployment.Env[2].ValueFrom == nil || created.Spec.Deployment.Env[2].ValueFrom.ConfigMapKeyRef.Name != "redis-config" {
		t.Fatalf("configmap env not persisted: %#v", created.Spec.Deployment.Env[2])
	}

	if err := UpsertComponentCR(context.Background(), "shop", "staging", "Orders API", "orders-api", "backend", "registry/orders-api", "v2", 2, "shop-staging", "argocd", model.ComponentConfig{}, []paapv1.EnvVar{{Name: "FEATURE_FLAG", Value: "on"}}); err != nil {
		t.Fatalf("update component cr: %v", err)
	}
	updated := &paapv1.Component{}
	if err := k8sClient.Get(context.Background(), key, updated); err != nil {
		t.Fatalf("get updated component: %v", err)
	}
	if len(updated.Spec.Deployment.Env) != 1 || updated.Spec.Deployment.Env[0].Name != "FEATURE_FLAG" || updated.Spec.Deployment.Env[0].Value != "on" {
		t.Fatalf("updated env not replaced: %#v", updated.Spec.Deployment.Env)
	}
}

func TestServiceInstanceSpecNeedsRefreshIgnoresEqualSpec(t *testing.T) {
	spec := paapv1.ServiceInstanceSpec{
		Type:          "deploy",
		ToolNamespace: "test-staging-deploy",
		WorkloadRole:  paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{Resources: []string{"pods"}, Verbs: []string{"get"}}}},
		Helm: &paapv1.HelmInstallSpec{
			ReleaseName: "test-staging-deploy",
			Namespace:   "test-staging-deploy",
			Values: map[string]string{
				"paap.envNamespaces": "test-staging,test-staging-app",
			},
		},
	}

	if serviceInstanceSpecNeedsRefresh(spec, spec) {
		t.Fatalf("equal specs should not require refresh")
	}
}
