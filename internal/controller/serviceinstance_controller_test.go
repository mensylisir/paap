package controller

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	paapv1 "paap/api/v1"
	paaphelm "paap/internal/helm"
	"paap/internal/model"
)

type fakeHelmInstaller struct {
	err error
}

func (f fakeHelmInstaller) UpgradeInstallWithMetadata(string, string, string, map[string]interface{}, paaphelm.ResourceMetadata) error {
	return f.err
}

func TestEnsureToolNamespaceIsMarkedForEnvironmentMonitoring(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}

	md := serviceMetadata{
		AppIdentifier: "test",
		EnvIdentifier: "staging",
		ServiceType:   "log",
		Tool:          "loki",
		Category:      "tool",
		ResourceRole:  "tool",
		ToolNamespace: "test-staging-log",
	}
	if err := r.ensureToolNamespace(context.Background(), "test-staging-log", md); err != nil {
		t.Fatalf("ensure tool namespace: %v", err)
	}

	ns := &corev1.Namespace{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "test-staging-log"}, ns); err != nil {
		t.Fatalf("get namespace: %v", err)
	}
	if ns.Labels["paap-managed"] != "true" {
		t.Fatalf("tool namespace missing paap-managed label: %#v", ns.Labels)
	}
	if ns.Labels["paap.io/role"] != "tool" || ns.Labels["paap.io/app"] != "test" || ns.Labels["paap.io/env"] != "staging" {
		t.Fatalf("tool namespace missing PAAP ownership labels: %#v", ns.Labels)
	}
	if ns.Labels["paap.io/service-type"] != "log" || ns.Labels["paap.io/tool"] != "loki" || ns.Labels["paap.io/category"] != "tool" {
		t.Fatalf("tool namespace missing service classification labels: %#v", ns.Labels)
	}
}

func TestCollectToolComponentStatusReportsInstallingUntilWorkloadsReady(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	replicas := int32(1)
	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "harbor-core",
					Namespace: "test-staging-registry",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
				Status: appsv1.DeploymentStatus{
					Replicas:      1,
					ReadyReplicas: 0,
				},
			}).
			WithStatusSubresource(&appsv1.Deployment{}).
			Build(),
		Scheme: scheme,
	}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "staging-registry",
			Namespace: "paap-app-test",
		},
		Spec: paapv1.ServiceInstanceSpec{
			ToolNamespace: "test-staging-registry",
		},
	}

	components, ready, err := r.collectToolComponentStatus(context.Background(), svc)
	if err != nil {
		t.Fatalf("collect tool status: %v", err)
	}
	if ready {
		t.Fatalf("expected tool not ready while deployment has zero ready replicas")
	}
	if len(components) != 1 {
		t.Fatalf("expected one component, got %d", len(components))
	}
	if components[0].Ready {
		t.Fatalf("expected component to be not ready: %#v", components[0])
	}
	if components[0].Replicas != "0/1" {
		t.Fatalf("expected replica summary 0/1, got %q", components[0].Replicas)
	}
}

func TestServiceInstanceStatusRequeueAfterRefreshesInstallingToolsSooner(t *testing.T) {
	if got := serviceInstanceStatusRequeueAfter(false); got != 5*time.Second {
		t.Fatalf("installing tool requeue = %v, want 5s", got)
	}
	if got := serviceInstanceStatusRequeueAfter(true); got != 60*time.Second {
		t.Fatalf("ready tool requeue = %v, want 60s", got)
	}
}

func TestServiceInstancesForNamespaceMapsWorkloadNamespaceToEnvironmentTools(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(
				&paapv1.ServiceInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "staging-deploy",
						Namespace: "paap-app-test",
						Labels: map[string]string{
							"paap.io/app": "test",
							"paap.io/env": "staging",
						},
					},
				},
				&paapv1.ServiceInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dev-deploy",
						Namespace: "paap-app-test",
						Labels: map[string]string{
							"paap.io/app": "test",
							"paap.io/env": "dev",
						},
					},
				},
			).
			Build(),
		Scheme: scheme,
	}

	requests := r.serviceInstancesForNamespace(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-staging-extra",
			Labels: map[string]string{
				"paap.io/app":  "test",
				"paap.io/env":  "staging",
				"paap.io/role": "workload",
			},
		},
	})

	if len(requests) != 1 {
		t.Fatalf("expected one request, got %#v", requests)
	}
	if requests[0].Name != "staging-deploy" || requests[0].Namespace != "paap-app-test" {
		t.Fatalf("unexpected request: %#v", requests[0])
	}
}

func TestServiceInstancesForNamespaceMapsToolNamespaceToEnvironmentTools(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&paapv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-deploy",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "staging",
					},
				},
			}).
			Build(),
		Scheme: scheme,
	}

	requests := r.serviceInstancesForNamespace(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-staging-deploy",
			Labels: map[string]string{
				"paap.io/app":  "test",
				"paap.io/env":  "staging",
				"paap.io/role": "tool",
			},
		},
	})

	if len(requests) != 1 {
		t.Fatalf("expected one request for tool namespace, got %#v", requests)
	}
	if requests[0].Name != "staging-deploy" || requests[0].Namespace != "paap-app-test" {
		t.Fatalf("unexpected request: %#v", requests[0])
	}
}

func TestServiceInstancesForNamespaceMapsEnvironmentNamespaceWithoutRole(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(&paapv1.ServiceInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging-monitor",
					Namespace: "paap-app-test",
					Labels: map[string]string{
						"paap.io/app": "test",
						"paap.io/env": "staging",
					},
				},
			}).
			Build(),
		Scheme: scheme,
	}

	requests := r.serviceInstancesForNamespace(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-staging-postgresql",
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
	})

	if len(requests) != 1 {
		t.Fatalf("expected one request for same-environment namespace without role, got %#v", requests)
	}
	if requests[0].Name != "staging-monitor" || requests[0].Namespace != "paap-app-test" {
		t.Fatalf("unexpected request: %#v", requests[0])
	}
}

func TestServiceInstancesForNamespaceIgnoresNonEnvironmentNamespace(t *testing.T) {
	r := &ServiceInstanceReconciler{}

	requests := r.serviceInstancesForNamespace(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unrelated",
			Labels: map[string]string{
				"paap.io/app": "test",
			},
		},
	})

	if len(requests) != 0 {
		t.Fatalf("expected no requests for incomplete environment labels, got %#v", requests)
	}
}

func TestDiscoverEnvironmentNamespacesIncludesToolAndWorkloadNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-staging-log",
						Labels: map[string]string{
							"paap.io/app":  "test",
							"paap.io/env":  "staging",
							"paap.io/role": "tool",
						},
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-staging",
						Labels: map[string]string{
							"paap.io/app":  "test",
							"paap.io/env":  "staging",
							"paap.io/role": "workload",
						},
					},
				},
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "kube-system",
						Labels: map[string]string{
							"paap.io/app":  "test",
							"paap.io/env":  "staging",
							"paap.io/role": "workload",
						},
					},
				},
			).
			Build(),
		Scheme: scheme,
	}

	namespaces := r.discoverEnvironmentNamespaces(context.Background(), "test", "staging")
	if len(namespaces) != 2 {
		t.Fatalf("expected 2 namespaces, got %#v", namespaces)
	}
	if namespaces[0] != "test-staging" || namespaces[1] != "test-staging-log" {
		t.Fatalf("expected stable sorted namespace list, got %#v", namespaces)
	}
	workloadNamespaces := r.discoverWorkloadNamespaces(context.Background(), "test", "staging")
	if len(workloadNamespaces) != 1 || workloadNamespaces[0] != "test-staging" {
		t.Fatalf("system namespaces must not be treated as workload namespaces, got %#v", workloadNamespaces)
	}
}

func TestWorkloadRBACTargetNamespacesAlwaysProjectsOnlyWorkloadNamespaces(t *testing.T) {
	environmentNamespaces := []string{
		"test-staging-registry",
		"test-staging",
		"test-staging-git",
		"test-staging-monitor",
		"test-staging-postgresql",
	}
	workloadNamespaces := []string{"test-staging"}
	broadRole := paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	}}}

	for _, toolType := range []string{"monitor", "log", "deploy", "argocd"} {
		got := workloadRBACTargetNamespaces(toolType, broadRole, environmentNamespaces, workloadNamespaces)
		want := []string{"test-staging"}
		if !stringSliceEqual(got, want) {
			t.Fatalf("%s RBAC namespaces = %#v, want %#v", toolType, got, want)
		}
	}
}

func TestWorkloadRBACTargetNamespacesEmptyRoleTargetsNoNamespaces(t *testing.T) {
	environmentNamespaces := []string{
		"test-staging",
		"test-staging-monitor",
		"test-staging-postgresql",
	}
	workloadNamespaces := []string{"test-staging"}

	got := workloadRBACTargetNamespaces("monitor", paapv1.RoleSpec{}, environmentNamespaces, workloadNamespaces)
	if len(got) != 0 {
		t.Fatalf("empty workload role targets = %#v, want no namespaces", got)
	}
}

func TestEnvironmentRBACTargetNamespacesProjectsAcrossEnvironmentNamespacesExceptOwnTool(t *testing.T) {
	environmentNamespaces := []string{
		"test-staging-registry",
		"test-staging",
		"test-staging-postgresql",
		"test-staging-monitor",
		"test-staging-redis",
	}
	got := environmentRBACTargetNamespaces("test-staging-monitor", environmentNamespaces)
	want := []string{"test-staging", "test-staging-postgresql", "test-staging-redis", "test-staging-registry"}
	if !stringSliceEqual(got, want) {
		t.Fatalf("environment RBAC namespaces = %#v, want %#v", got, want)
	}
}

func TestWorkloadRBACTargetNamespacesKeepsDeployWritePermissionsInWorkloadNamespaces(t *testing.T) {
	environmentNamespaces := []string{
		"test-staging-registry",
		"test-staging",
		"test-staging-git",
		"test-staging-monitor",
	}
	workloadNamespaces := []string{"test-staging"}
	broadRole := paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	}}}

	for _, toolType := range []string{"deploy", "argocd"} {
		got := workloadRBACTargetNamespaces(toolType, broadRole, environmentNamespaces, workloadNamespaces)
		want := []string{"test-staging"}
		if !stringSliceEqual(got, want) {
			t.Fatalf("%s RBAC namespaces = %#v, want %#v", toolType, got, want)
		}
	}
}

func TestWorkloadRBACTargetNamespacesKeepsOtherToolsInWorkloadNamespaces(t *testing.T) {
	environmentNamespaces := []string{
		"test-staging",
		"test-staging-git",
		"test-staging-registry",
	}
	workloadNamespaces := []string{"test-staging"}
	broadRole := paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
		APIGroups: []string{"*"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	}}}

	got := workloadRBACTargetNamespaces("registry", broadRole, environmentNamespaces, workloadNamespaces)
	want := []string{"test-staging"}
	if !stringSliceEqual(got, want) {
		t.Fatalf("registry RBAC namespaces = %#v, want %#v", got, want)
	}
}

func TestEnvironmentNamespaceHelmValueKeysUsesManifestVariableMapping(t *testing.T) {
	manifest := model.PlatformManifest{
		Name:    "custom-log",
		Version: "v1",
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "env_namespaces", HelmVar: "agent.watchNamespaces"},
			{PlatformVar: "workload_namespaces", HelmVar: "agent.workloadNamespaces"},
			{PlatformVar: "all_namespaces", HelmVar: "collector.namespaces"},
			{PlatformVar: "tool_namespace", HelmVar: "agent.serviceAccount.name"},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	keys := environmentNamespaceHelmValueKeys(&paapv1.HelmInstallSpec{PlatformManifest: string(data)})
	got := map[string]bool{}
	for _, key := range keys {
		got[key] = true
	}
	for _, want := range []string{"agent.watchNamespaces", "agent.workloadNamespaces", "collector.namespaces"} {
		if !got[want] {
			t.Fatalf("expected %s in keys, got %#v", want, keys)
		}
	}
	if got["agent.serviceAccount.name"] {
		t.Fatalf("tool_namespace mapping should not be updated as env namespace: %#v", keys)
	}
}

func TestNamespaceHelmValueTargetsSeparatesEnvironmentAndWorkloadMappings(t *testing.T) {
	manifest := model.PlatformManifest{
		Name:    "custom-deploy",
		Version: "v1",
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "env_namespaces", HelmVar: "collector.namespaces"},
			{PlatformVar: "workload_namespaces", HelmVar: "controller.namespaces"},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	targets := namespaceHelmValueTargets(
		&paapv1.HelmInstallSpec{PlatformManifest: string(data)},
		[]string{"test-staging-monitor", "test-staging"},
		[]string{"test-staging"},
	)
	if got := targets["collector.namespaces"]; got != "test-staging,test-staging-monitor" {
		t.Fatalf("env namespace mapping = %q", got)
	}
	if got := targets["controller.namespaces"]; got != "test-staging" {
		t.Fatalf("workload namespace mapping = %q", got)
	}
}

func TestSyncEnvironmentNamespacesInHelmValuesWritesStableSortedList(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "staging-monitor",
			Namespace:  "paap-app-test",
			Generation: 1,
		},
		Spec: paapv1.ServiceInstanceSpec{
			Helm: &paapv1.HelmInstallSpec{
				Values: map[string]string{},
			},
		},
	}
	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(svc).
			Build(),
		Scheme: scheme,
	}

	changed := r.syncEnvironmentNamespacesInHelmValues(context.Background(), svc, []string{
		"test-staging-monitor",
		"test-staging",
		"test-staging-log",
	})
	if !changed {
		t.Fatalf("expected values to change")
	}

	latest := &paapv1.ServiceInstance{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "staging-monitor", Namespace: "paap-app-test"}, latest); err != nil {
		t.Fatalf("get service instance: %v", err)
	}
	expected := "test-staging,test-staging-log,test-staging-monitor"
	if latest.Spec.Helm.Values["env_namespaces"] != expected {
		t.Fatalf("expected sorted env_namespaces %q, got %q", expected, latest.Spec.Helm.Values["env_namespaces"])
	}

	if r.syncEnvironmentNamespacesInHelmValues(context.Background(), latest, []string{
		"test-staging-log",
		"test-staging-monitor",
		"test-staging",
	}) {
		t.Fatalf("expected no change for same namespace set in different order")
	}
}

func TestReconcileUpdatesProjectedRBACWhenEnvironmentNamespacesChange(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	roleName := "test-staging-monitor-environment-manager"
	env := &paapv1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "staging", Namespace: "paap-app-test"},
		Status:     paapv1.EnvironmentStatus{Phase: "Running"},
	}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "staging-monitor",
			Namespace:  "paap-app-test",
			Generation: 2,
			Finalizers: []string{svcFinalizer},
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
			Type:           "monitor",
			ToolNamespace:  "test-staging-monitor",
			ServiceAccount: paapv1.ServiceAccountSpec{
				Name:      "test-staging-monitor",
				Namespace: "test-staging-monitor",
			},
			WorkloadRole: paapv1.RoleSpec{},
			EnvironmentRole: &paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"get", "list", "watch"},
			}}},
			Helm: &paapv1.HelmInstallSpec{
				ReleaseName: "monitor",
				Namespace:   "test-staging-monitor",
				Values: map[string]string{
					"env_namespaces": "test-staging,test-staging-monitor,test-staging-old",
				},
			},
		},
		Status: paapv1.ServiceInstanceStatus{
			ObservedGeneration: 2,
			RBACNamespaces: []paapv1.RBACNamespaceStatus{
				{Namespace: "test-staging-old", RoleCreated: true, RoleBindingCreated: true},
			},
		},
	}
	primaryNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "test-staging",
		Labels: map[string]string{
			"paap.io/app":  "test",
			"paap.io/env":  "staging",
			"paap.io/role": "workload",
		},
	}}
	newNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "test-staging-extra",
		Labels: map[string]string{
			"paap.io/app":  "test",
			"paap.io/env":  "staging",
			"paap.io/role": "workload",
		},
	}}
	otherToolNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "test-staging-git",
		Labels: map[string]string{
			"paap.io/app":     "test",
			"paap.io/env":     "staging",
			"paap.io/role":    "tool",
			"paap.io/service": "git",
		},
	}}
	postgresNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "test-staging-postgresql",
		Labels: map[string]string{
			"paap.io/app":     "test",
			"paap.io/env":     "staging",
			"paap.io/role":    "tool",
			"paap.io/service": "postgresql",
		},
	}}
	oldRole := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: "test-staging-old"}}
	oldBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: "test-staging-old"}}
	staleToolRole := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: otherToolNS.Name}}
	staleToolBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: otherToolNS.Name}}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(env, svc, primaryNS, newNS, otherToolNS, postgresNS, oldRole, oldBinding, staleToolRole, staleToolBinding).
			WithStatusSubresource(&paapv1.ServiceInstance{}).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	for _, ns := range []string{"test-staging", "test-staging-extra", "test-staging-git", "test-staging-postgresql"} {
		role := &rbacv1.Role{}
		if err := r.Get(context.Background(), types.NamespacedName{Name: roleName, Namespace: ns}, role); err != nil {
			t.Fatalf("expected projected role in %s: %v", ns, err)
		}
		if len(role.Rules) != 1 || role.Rules[0].Resources[0] != "pods" {
			t.Fatalf("unexpected role rules in %s: %#v", ns, role.Rules)
		}
		binding := &rbacv1.RoleBinding{}
		if err := r.Get(context.Background(), types.NamespacedName{Name: roleName, Namespace: ns}, binding); err != nil {
			t.Fatalf("expected projected rolebinding in %s: %v", ns, err)
		}
		if len(binding.Subjects) != 1 || binding.Subjects[0].Namespace != "test-staging-monitor" {
			t.Fatalf("unexpected rolebinding subject in %s: %#v", ns, binding.Subjects)
		}
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: roleName, Namespace: "test-staging-old"}, &rbacv1.Role{}); err == nil {
		t.Fatalf("expected stale role in removed namespace to be deleted")
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: roleName, Namespace: "test-staging-old"}, &rbacv1.RoleBinding{}); err == nil {
		t.Fatalf("expected stale rolebinding in removed namespace to be deleted")
	}

	latest := &paapv1.ServiceInstance{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, latest); err != nil {
		t.Fatalf("get latest service instance: %v", err)
	}
	if got := latest.Spec.Helm.Values["env_namespaces"]; got != "test-staging,test-staging-extra,test-staging-git,test-staging-monitor,test-staging-postgresql" {
		t.Fatalf("env_namespaces = %q", got)
	}
}

func TestReconcileDeletesLegacyServiceTypeWorkloadRBACAfterToolRename(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	env := &paapv1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "staging", Namespace: "paap-app-test"},
		Status:     paapv1.EnvironmentStatus{Phase: "Running"},
	}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "staging-monitor",
			Namespace:  "paap-app-test",
			Generation: 2,
			Finalizers: []string{svcFinalizer},
			Labels: map[string]string{
				"paap.io/app":          "test",
				"paap.io/env":          "staging",
				"paap.io/service-type": "monitor",
				"paap.io/tool":         "kube-prometheus-stack",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
			Type:           "monitor",
			ToolNamespace:  "test-staging-monitor",
			ServiceAccount: paapv1.ServiceAccountSpec{Name: "test-staging-monitor", Namespace: "test-staging-monitor"},
			WorkloadRole: paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			}}},
			Helm: &paapv1.HelmInstallSpec{
				ReleaseName: "monitor",
				Namespace:   "test-staging-monitor",
				Values:      map[string]string{},
			},
		},
	}
	workloadNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "test-staging",
		Labels: map[string]string{
			"paap.io/app":  "test",
			"paap.io/env":  "staging",
			"paap.io/role": "workload",
		},
	}}
	legacyRoleName := "test-staging-monitor-workload-manager"
	currentRoleName := "test-staging-kube-prometheus-stack-workload-manager"
	legacyRole := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
		Name:      legacyRoleName,
		Namespace: "test-staging",
		Annotations: map[string]string{
			"paap.io/service-namespace": "test-staging-monitor",
		},
		Labels: map[string]string{
			"paap.io/app":          "test",
			"paap.io/env":          "staging",
			"paap.io/service-type": "monitor",
			"paap.io/tool":         "monitor",
		},
	}}
	legacyBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
		Name:      legacyRoleName,
		Namespace: "test-staging",
		Annotations: map[string]string{
			"paap.io/service-namespace": "test-staging-monitor",
		},
		Labels: map[string]string{
			"paap.io/app":          "test",
			"paap.io/env":          "staging",
			"paap.io/service-type": "monitor",
			"paap.io/tool":         "monitor",
		},
	}}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(env, svc, workloadNS, legacyRole, legacyBinding).
			WithStatusSubresource(&paapv1.ServiceInstance{}).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: legacyRoleName, Namespace: "test-staging"}, &rbacv1.Role{}); err == nil {
		t.Fatalf("expected legacy service-type role to be deleted")
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: legacyRoleName, Namespace: "test-staging"}, &rbacv1.RoleBinding{}); err == nil {
		t.Fatalf("expected legacy service-type rolebinding to be deleted")
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: currentRoleName, Namespace: "test-staging"}, &rbacv1.Role{}); err != nil {
		t.Fatalf("expected current tool role to exist: %v", err)
	}
}

func TestReconcileDoesNotProjectWriteWorkloadRBACIntoToolNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	roleName := "test-staging-deploy-workload-manager"
	env := &paapv1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "staging", Namespace: "paap-app-test"},
		Status:     paapv1.EnvironmentStatus{Phase: "Running"},
	}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "staging-deploy",
			Namespace:  "paap-app-test",
			Generation: 1,
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
			Type:           "deploy",
			ToolNamespace:  "test-staging-deploy",
			ServiceAccount: paapv1.ServiceAccountSpec{
				Name:      "test-staging-deploy",
				Namespace: "test-staging-deploy",
			},
			WorkloadRole: paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			}}},
			Helm: &paapv1.HelmInstallSpec{
				ReleaseName: "deploy",
				Namespace:   "test-staging-deploy",
				Values:      map[string]string{},
			},
		},
	}
	workloadNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "test-staging",
		Labels: map[string]string{"paap.io/app": "test", "paap.io/env": "staging", "paap.io/role": "workload"},
	}}
	toolNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "test-staging-log",
		Labels: map[string]string{"paap.io/app": "test", "paap.io/env": "staging", "paap.io/role": "tool"},
	}}
	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(env, svc, workloadNS, toolNS).
			WithStatusSubresource(&paapv1.ServiceInstance{}).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: roleName, Namespace: workloadNS.Name}, &rbacv1.Role{}); err != nil {
		t.Fatalf("expected projected role in workload namespace: %v", err)
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: roleName, Namespace: toolNS.Name}, &rbacv1.Role{}); err == nil {
		t.Fatalf("write workload role must not be projected into tool namespace")
	}
}

func TestApplyRuntimeRegistryValuesInjectsRegistryTLSHost(t *testing.T) {
	t.Setenv("PAAP_REGISTRY_HOST_TEMPLATE", "registry.{app}-{env}.corp.example.com:5443")
	values := map[string]interface{}{}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"paap.io/app":     "shop",
				"paap.io/env":     "dev",
				"paap.io/service": "registry",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{Type: "registry"},
	}

	applyRuntimeRegistryValues(svc, values)

	tlsValues, ok := values["tls"].(map[string]interface{})
	if !ok || tlsValues["commonName"] != "registry.shop-dev.corp.example.com" {
		t.Fatalf("expected registry tls commonName, got %#v", values)
	}
	ingressValues, ok := values["ingress"].(map[string]interface{})
	if !ok || ingressValues["host"] != "registry.shop-dev.corp.example.com" {
		t.Fatalf("expected registry ingress host, got %#v", values)
	}
}

func TestApplyRuntimeRegistryValuesInjectsHarborHTTPSHost(t *testing.T) {
	t.Setenv("PAAP_REGISTRY_HOST_TEMPLATE", "harbor-{app}-{env}.corp.example.com")
	values := map[string]interface{}{}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"paap.io/app":     "shop",
				"paap.io/env":     "dev",
				"paap.io/service": "harbor",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{Type: "harbor"},
	}

	applyRuntimeRegistryValues(svc, values)

	if values["externalURL"] != "https://harbor-shop-dev.corp.example.com" {
		t.Fatalf("externalURL = %#v", values["externalURL"])
	}
	expose, ok := values["expose"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing expose values: %#v", values)
	}
	ingress := expose["ingress"].(map[string]interface{})
	hosts := ingress["hosts"].(map[string]interface{})
	if hosts["core"] != "harbor-shop-dev.corp.example.com" {
		t.Fatalf("harbor ingress host = %#v", values)
	}
}

func TestApplyRuntimeRegistryValuesKeepsHarborExternalURLPortOutOfTLSHost(t *testing.T) {
	t.Setenv("PAAP_REGISTRY_HOST_TEMPLATE", "harbor-{app}-{env}.corp.example.com:5443")
	values := map[string]interface{}{}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"paap.io/app":     "shop",
				"paap.io/env":     "dev",
				"paap.io/service": "harbor",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{Type: "harbor"},
	}

	applyRuntimeRegistryValues(svc, values)

	if values["externalURL"] != "https://harbor-shop-dev.corp.example.com:5443" {
		t.Fatalf("externalURL = %#v", values["externalURL"])
	}
	expose := values["expose"].(map[string]interface{})
	tlsValues := expose["tls"].(map[string]interface{})
	auto := tlsValues["auto"].(map[string]interface{})
	if auto["commonName"] != "harbor-shop-dev.corp.example.com" {
		t.Fatalf("harbor TLS commonName = %#v", values)
	}
}

func TestApplyRuntimeRegistryValuesLeavesOtherServicesUnchanged(t *testing.T) {
	original := os.Getenv("PAAP_REGISTRY_HOST_TEMPLATE")
	t.Setenv("PAAP_REGISTRY_HOST_TEMPLATE", original)
	values := map[string]interface{}{}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"paap.io/app":     "shop",
				"paap.io/env":     "dev",
				"paap.io/service": "jenkins",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{Type: "jenkins"},
	}

	applyRuntimeRegistryValues(svc, values)

	if len(values) != 0 {
		t.Fatalf("values changed for non-registry service: %#v", values)
	}
}

func TestEnsureClusterRoleBindsToolServiceAccount(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "staging-deploy",
			Namespace: "paap-app-test",
			Labels: map[string]string{
				"paap.io/app":          "test",
				"paap.io/env":          "staging",
				"paap.io/service-type": "deploy",
				"paap.io/tool":         "argocd",
				"paap.io/category":     "tool",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			Type:          "deploy",
			ToolNamespace: "test-staging-deploy",
			ServiceAccount: paapv1.ServiceAccountSpec{
				Name:      "test-staging-deploy",
				Namespace: "test-staging-deploy",
			},
			ClusterRole: &paapv1.RoleSpec{Rules: []paapv1.PolicyRule{
				{APIGroups: []string{"apiregistration.k8s.io"}, Resources: []string{"apiservices"}, Verbs: []string{"get", "list", "watch"}},
			}},
		},
	}
	md := metadataFromServiceInstance(svc)

	if err := r.ensureClusterRole(context.Background(), svc, md); err != nil {
		t.Fatalf("ensure cluster role: %v", err)
	}
	if err := r.ensureClusterRoleBinding(context.Background(), svc, md); err != nil {
		t.Fatalf("ensure cluster role binding: %v", err)
	}

	role := &rbacv1.ClusterRole{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "test-staging-argocd-cluster-manager"}, role); err != nil {
		t.Fatalf("expected cluster role: %v", err)
	}
	if len(role.Rules) != 1 || role.Rules[0].Resources[0] != "apiservices" {
		t.Fatalf("unexpected cluster role rules: %#v", role.Rules)
	}
	if role.Labels["paap.io/service-type"] != "deploy" || role.Labels["paap.io/tool"] != "argocd" {
		t.Fatalf("cluster role missing service classification labels: %#v", role.Labels)
	}

	binding := &rbacv1.ClusterRoleBinding{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "test-staging-argocd-cluster-manager"}, binding); err != nil {
		t.Fatalf("expected cluster role binding: %v", err)
	}
	if binding.RoleRef.Kind != "ClusterRole" || binding.RoleRef.Name != role.Name {
		t.Fatalf("unexpected role ref: %#v", binding.RoleRef)
	}
	if len(binding.Subjects) != 1 || binding.Subjects[0].Name != "test-staging-deploy" || binding.Subjects[0].Namespace != "test-staging-deploy" {
		t.Fatalf("unexpected subjects: %#v", binding.Subjects)
	}
}

func TestHandleDeletionCleansProjectedRBACFromEnvironmentToolNamespaces(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	roleName := "test-staging-log-workload-manager"
	now := metav1.Now()
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "staging-log",
			Namespace:         "paap-app-test",
			DeletionTimestamp: &now,
			Finalizers:        []string{svcFinalizer},
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			Type:          "log",
			ToolNamespace: "test-staging-log",
		},
	}
	otherToolNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "test-staging-monitor",
		Labels: map[string]string{
			"paap.io/app":  "test",
			"paap.io/env":  "staging",
			"paap.io/role": "tool",
		},
	}}
	workloadNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "test-staging",
		Labels: map[string]string{
			"paap.io/app":  "test",
			"paap.io/env":  "staging",
			"paap.io/role": "workload",
		},
	}}
	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(
				svc,
				&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-staging-log"}},
				otherToolNS,
				workloadNS,
				&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: otherToolNS.Name}},
				&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: otherToolNS.Name}},
				&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: workloadNS.Name}},
				&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: workloadNS.Name}},
			).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.handleDeletion(context.Background(), svc); err != nil {
		t.Fatalf("handle deletion: %v", err)
	}

	for _, ns := range []string{otherToolNS.Name, workloadNS.Name} {
		role := &rbacv1.Role{}
		if err := r.Get(context.Background(), types.NamespacedName{Name: roleName, Namespace: ns}, role); err == nil {
			t.Fatalf("expected projected role in %s to be deleted", ns)
		}
		binding := &rbacv1.RoleBinding{}
		if err := r.Get(context.Background(), types.NamespacedName{Name: roleName, Namespace: ns}, binding); err == nil {
			t.Fatalf("expected projected rolebinding in %s to be deleted", ns)
		}
	}
}

func TestReconcileDoesNotProjectWorkloadRBACForToolOnlyServices(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	env := &paapv1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "staging", Namespace: "paap-app-test"},
		Status:     paapv1.EnvironmentStatus{Phase: "Running"},
	}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "staging-redis",
			Namespace:  "paap-app-test",
			Generation: 1,
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
			Type:           "redis",
			ToolNamespace:  "test-staging-redis",
			ServiceAccount: paapv1.ServiceAccountSpec{
				Name:      "test-staging-redis",
				Namespace: "test-staging-redis",
			},
			ToolNamespaceRole: &paapv1.RoleSpec{Rules: []paapv1.PolicyRule{{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			}}},
			WorkloadRole: paapv1.RoleSpec{},
			Helm: &paapv1.HelmInstallSpec{
				ReleaseName: "redis",
				Namespace:   "test-staging-redis",
				Values:      map[string]string{},
			},
		},
	}
	workloadNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "test-staging",
		Labels: map[string]string{
			"paap.io/app":  "test",
			"paap.io/env":  "staging",
			"paap.io/role": "workload",
		},
	}}
	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(env, svc, workloadNS).
			WithStatusSubresource(&paapv1.ServiceInstance{}).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	workloadRoleName := "test-staging-redis-workload-manager"
	role := &rbacv1.Role{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: workloadRoleName, Namespace: workloadNS.Name}, role); err == nil {
		t.Fatalf("tool-only service should not project workload role: %#v", role)
	}
	binding := &rbacv1.RoleBinding{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: workloadRoleName, Namespace: workloadNS.Name}, binding); err == nil {
		t.Fatalf("tool-only service should not project workload rolebinding: %#v", binding)
	}

	toolRole := &rbacv1.Role{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "test-staging-redis-tool-ns-manager", Namespace: svc.Spec.ToolNamespace}, toolRole); err != nil {
		t.Fatalf("expected tool namespace role to be created: %v", err)
	}
}

func TestReconcileKeepsRunningStatusWhenHelmOperationIsInProgressAndComponentsReady(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	replicas := int32(1)
	env := &paapv1.Environment{
		ObjectMeta: metav1.ObjectMeta{Name: "staging", Namespace: "paap-app-test"},
		Status:     paapv1.EnvironmentStatus{Phase: "Running"},
	}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "staging-deploy",
			Namespace:  "paap-app-test",
			Generation: 2,
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
		Spec: paapv1.ServiceInstanceSpec{
			EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
			Type:           "deploy",
			ToolNamespace:  "test-staging-deploy",
			ServiceAccount: paapv1.ServiceAccountSpec{
				Name:      "test-staging-deploy",
				Namespace: "test-staging-deploy",
			},
			Helm: &paapv1.HelmInstallSpec{
				ReleaseName: "test-staging-deploy",
				Namespace:   "test-staging-deploy",
				ChartName:   "/tmp/chart",
				Values: map[string]string{
					"all_namespaces":            "test-staging-deploy",
					"env_namespaces":            "test-staging-deploy",
					"paap.envNamespaces":        "test-staging-deploy",
					"global.paap.envNamespaces": "test-staging-deploy",
				},
			},
		},
		Status: paapv1.ServiceInstanceStatus{ObservedGeneration: 1},
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "argocd-server", Namespace: "test-staging-deploy"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	}
	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(env, svc, deploy).
			WithStatusSubresource(&paapv1.ServiceInstance{}, &appsv1.Deployment{}).
			Build(),
		Scheme:     scheme,
		HelmClient: fakeHelmInstaller{err: errors.New("another operation (install/upgrade/rollback) is in progress")},
	}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}}); err != nil {
		t.Fatalf("reconcile returned recoverable Helm error: %v", err)
	}

	latest := &paapv1.ServiceInstance{}
	if err := r.Get(context.Background(), types.NamespacedName{Name: svc.Name, Namespace: svc.Namespace}, latest); err != nil {
		t.Fatalf("get latest service instance: %v", err)
	}
	if latest.Status.Phase != "Running" {
		t.Fatalf("phase = %q, want Running", latest.Status.Phase)
	}
	if latest.Status.ObservedGeneration != svc.Generation {
		t.Fatalf("observedGeneration = %d, want %d", latest.Status.ObservedGeneration, svc.Generation)
	}
}

func TestCollectToolComponentStatusReportsReadyWhenNoWorkloadsExist(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
	}
	svc := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "empty", Namespace: "paap-app-test"},
		Spec:       paapv1.ServiceInstanceSpec{ToolNamespace: "empty-tool"},
	}

	components, ready, err := r.collectToolComponentStatus(context.Background(), svc)
	if err != nil {
		t.Fatalf("collect tool status: %v", err)
	}
	if !ready {
		t.Fatalf("expected ready when no workloads have been created")
	}
	if len(components) != 0 {
		t.Fatalf("expected no components, got %#v", components)
	}
}

func TestCollectToolComponentStatusIncludesReadyStatefulSet(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	replicas := int32(1)
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "tool-ns"},
		Spec:       appsv1.StatefulSetSpec{Replicas: &replicas},
		Status:     appsv1.StatefulSetStatus{Replicas: 1, ReadyReplicas: 1},
	}
	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(sts).
			WithStatusSubresource(&appsv1.StatefulSet{}).
			Build(),
		Scheme: scheme,
	}

	components, ready, err := r.collectToolComponentStatus(context.Background(), &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "git", Namespace: "paap-app-test"},
		Spec:       paapv1.ServiceInstanceSpec{ToolNamespace: "tool-ns"},
	})
	if err != nil {
		t.Fatalf("collect tool status: %v", err)
	}
	if !ready {
		t.Fatalf("expected ready when all statefulsets are ready")
	}
	if len(components) != 1 || components[0].Kind != "StatefulSet" || components[0].Name != "redis" || components[0].Replicas != "1/1" {
		t.Fatalf("unexpected components: %#v", components)
	}

	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: "redis", Namespace: "tool-ns"}, &appsv1.StatefulSet{}); err != nil {
		t.Fatalf("sanity check statefulset exists: %v", err)
	}
}

func TestCollectToolComponentStatusIncludesDaemonSetAndReplicaSet(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "promtail", Namespace: "tool-ns"},
		Status:     appsv1.DaemonSetStatus{DesiredNumberScheduled: 2, NumberReady: 2},
	}
	rsReplicas := int32(1)
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "app-rs", Namespace: "tool-ns"},
		Spec:       appsv1.ReplicaSetSpec{Replicas: &rsReplicas},
		Status:     appsv1.ReplicaSetStatus{Replicas: 1, ReadyReplicas: 1},
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(ds, rs).
			Build(),
		Scheme: scheme,
	}

	components, ready, err := r.collectToolComponentStatus(context.Background(), &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "log", Namespace: "paap-app-test"},
		Spec:       paapv1.ServiceInstanceSpec{ToolNamespace: "tool-ns"},
	})
	if err != nil {
		t.Fatalf("collect tool status: %v", err)
	}
	if !ready {
		t.Fatalf("expected ready when daemonset and replicaset are ready")
	}
	if len(components) != 2 {
		t.Fatalf("expected two components, got %#v", components)
	}
	kinds := map[string]bool{}
	for _, comp := range components {
		kinds[comp.Kind] = true
	}
	if !kinds["DaemonSet"] || !kinds["ReplicaSet"] {
		t.Fatalf("expected daemonset and replicaset in components, got %#v", components)
	}
}

func TestCollectToolComponentStatusIgnoresInactiveReplicaSets(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	inactiveReplicas := int32(0)
	activeReplicas := int32(1)
	inactive := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "old-rs", Namespace: "tool-ns"},
		Spec:       appsv1.ReplicaSetSpec{Replicas: &inactiveReplicas},
		Status:     appsv1.ReplicaSetStatus{Replicas: 0, ReadyReplicas: 0},
	}
	active := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "new-rs", Namespace: "tool-ns"},
		Spec:       appsv1.ReplicaSetSpec{Replicas: &activeReplicas},
		Status:     appsv1.ReplicaSetStatus{Replicas: 1, ReadyReplicas: 1},
	}

	r := &ServiceInstanceReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(inactive, active).
			Build(),
		Scheme: scheme,
	}

	components, ready, err := r.collectToolComponentStatus(context.Background(), &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "monitor", Namespace: "paap-app-test"},
		Spec:       paapv1.ServiceInstanceSpec{ToolNamespace: "tool-ns"},
	})
	if err != nil {
		t.Fatalf("collect tool status: %v", err)
	}
	if !ready {
		t.Fatalf("expected ready when only active replicaset is ready")
	}
	if len(components) != 1 || components[0].Name != "new-rs" {
		t.Fatalf("expected only active replicaset, got %#v", components)
	}
}
