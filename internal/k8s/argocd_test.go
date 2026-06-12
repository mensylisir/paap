package k8s

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type finalizerRecordingClient struct {
	client.Client
	deletedFinalizers []string
}

func (c *finalizerRecordingClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deletedFinalizers = append([]string{}, obj.GetFinalizers()...)
	return c.Client.Delete(ctx, obj, opts...)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestListArgoCDApplicationsReadsStatus(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      "billing-dev-api",
				"namespace": "billing-dev-deploy",
			},
			"spec": map[string]interface{}{
				"source": map[string]interface{}{
					"repoURL": "http://gitea/paap/billing-dev-components.git",
					"path":    "components/api",
				},
			},
			"status": map[string]interface{}{
				"sync":   map[string]interface{}{"status": "Synced"},
				"health": map[string]interface{}{"status": "Healthy"},
			},
		},
	}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	SetClient(fake.NewClientBuilder().WithObjects(app).Build())

	apps, err := ListArgoCDApplications(t.Context(), "billing-dev-deploy")
	if err != nil {
		t.Fatalf("list applications: %v", err)
	}
	if len(apps) != 1 || apps[0].Name != "billing-dev-api" || apps[0].SyncStatus != "Synced" || apps[0].HealthStatus != "Healthy" {
		t.Fatalf("unexpected applications: %#v", apps)
	}
	if apps[0].RepoURL == "" || apps[0].Path != "components/api" {
		t.Fatalf("unexpected source: %#v", apps[0])
	}
}

func TestApplyArgoCDApplicationSetsResourceFinalizer(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().Build())

	err := ApplyArgoCDApplication(t.Context(), ArgoCDApplicationSpec{
		Name:                 "billing-dev-api",
		Namespace:            "billing-dev-argocd",
		Project:              "billing-dev",
		RepoURL:              "http://gitea/paap/billing-dev-components.git",
		Path:                 "components/api",
		DestinationNamespace: "billing-dev",
		Automated:            true,
	})
	if err != nil {
		t.Fatalf("apply application: %v", err)
	}

	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := GetClient().Get(t.Context(), client.ObjectKey{Name: "billing-dev-api", Namespace: "billing-dev-argocd"}, app); err != nil {
		t.Fatalf("get application: %v", err)
	}
	if !containsString(app.GetFinalizers(), argoCDResourcesFinalizer) {
		t.Fatalf("expected resource finalizer, got %#v", app.GetFinalizers())
	}
}

func TestDeleteArgoCDApplicationAddsResourceFinalizerBeforeDelete(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	app := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]interface{}{
			"name":      "billing-dev-api",
			"namespace": "billing-dev-argocd",
		},
	}}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	base := fake.NewClientBuilder().WithObjects(app).Build()
	recorder := &finalizerRecordingClient{Client: base}
	SetClient(recorder)

	if err := DeleteArgoCDApplication(t.Context(), "billing-dev-argocd", "billing-dev-api"); err != nil {
		t.Fatalf("delete application: %v", err)
	}
	if !containsString(recorder.deletedFinalizers, argoCDResourcesFinalizer) {
		t.Fatalf("expected delete with resource finalizer, got %#v", recorder.deletedFinalizers)
	}
}

func TestArgoCDTreeNodesPreserveParentRefsAndGraphResources(t *testing.T) {
	resources := argoCDTreeNodesToResources([]argoCDResourceTreeNode{
		{
			Kind:      "Deployment",
			Name:      "api",
			Namespace: "billing-dev",
			Group:     "apps",
			UID:       "deploy-uid",
			Health:    map[string]interface{}{"status": "Healthy"},
		},
		{
			Kind:      "ReplicaSet",
			Name:      "api-6c7f",
			Namespace: "billing-dev",
			Group:     "apps",
			UID:       "rs-uid",
			ParentRefs: []argoCDResourceRef{{
				Group:     "apps",
				Kind:      "Deployment",
				Name:      "api",
				Namespace: "billing-dev",
				UID:       "deploy-uid",
			}},
			Health: map[string]interface{}{"status": "Healthy"},
		},
		{
			Kind:      "Pod",
			Name:      "api-6c7f-x",
			Namespace: "billing-dev",
			UID:       "pod-uid",
			ParentRefs: []argoCDResourceRef{{
				Group:     "apps",
				Kind:      "ReplicaSet",
				Name:      "api-6c7f",
				Namespace: "billing-dev",
				UID:       "rs-uid",
			}},
			Health: map[string]interface{}{"status": "Healthy"},
		},
	}, nil)

	if len(resources) != 1 || resources[0].Kind != "Deployment" {
		t.Fatalf("expected deployment root, got %#v", resources)
	}
	rs := resources[0].Children[0]
	if rs.Kind != "ReplicaSet" || len(rs.ParentRefs) != 1 || rs.ParentRefs[0].UID != "deploy-uid" {
		t.Fatalf("expected replicaset parent ref to be preserved, got %#v", rs)
	}
	pod := rs.Children[0]
	if pod.Kind != "Pod" || len(pod.ParentRefs) != 1 || pod.ParentRefs[0].UID != "rs-uid" {
		t.Fatalf("expected pod parent ref to be preserved, got %#v", pod)
	}
}

func TestSyncArgoCDApplicationSetsOperation(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	app := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "argoproj.io/v1alpha1",
			"kind":       "Application",
			"metadata": map[string]interface{}{
				"name":      "billing-dev-api",
				"namespace": "billing-dev-deploy",
			},
		},
	}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	cl := fake.NewClientBuilder().WithObjects(app).Build()
	SetClient(cl)

	if err := SyncArgoCDApplication(t.Context(), "billing-dev-deploy", "billing-dev-api"); err != nil {
		t.Fatalf("sync application: %v", err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev-api"}, got); err != nil {
		t.Fatalf("get application: %v", err)
	}
	revision, _, _ := unstructured.NestedString(got.Object, "operation", "sync", "revision")
	prune, _, _ := unstructured.NestedBool(got.Object, "operation", "sync", "prune")
	if revision != "HEAD" || !prune {
		t.Fatalf("unexpected sync operation: %#v", got.Object["operation"])
	}
}

func TestApplyArgoCDApplicationCreatesApplicationSpec(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	cl := fake.NewClientBuilder().Build()
	SetClient(cl)

	spec := ArgoCDApplicationSpec{
		Name:                 "billing-dev-api",
		Namespace:            "billing-dev-deploy",
		Project:              "billing-dev",
		RepoURL:              "http://gitea/paap/billing-dev-components.git",
		Path:                 "components/api",
		TargetRevision:       "main",
		DestinationServer:    "https://kubernetes.default.svc",
		DestinationNamespace: "billing-dev",
		Automated:            true,
	}
	if err := ApplyArgoCDApplication(t.Context(), spec); err != nil {
		t.Fatalf("apply application: %v", err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev-api"}, got); err != nil {
		t.Fatalf("get application: %v", err)
	}
	repoURL, _, _ := unstructured.NestedString(got.Object, "spec", "source", "repoURL")
	path, _, _ := unstructured.NestedString(got.Object, "spec", "source", "path")
	destNS, _, _ := unstructured.NestedString(got.Object, "spec", "destination", "namespace")
	prune, _, _ := unstructured.NestedBool(got.Object, "spec", "syncPolicy", "automated", "prune")
	if repoURL != spec.RepoURL || path != spec.Path || destNS != spec.DestinationNamespace || !prune {
		t.Fatalf("unexpected application spec: %#v", got.Object["spec"])
	}
}

func TestApplyArgoCDApplicationRejectsDefaultProjectAndSystemNamespace(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().Build())

	base := ArgoCDApplicationSpec{
		Name:                 "billing-dev-api",
		Namespace:            "billing-dev-deploy",
		Project:              "billing-dev",
		RepoURL:              "http://gitea/paap/billing-dev-components.git",
		Path:                 "components/api",
		DestinationNamespace: "billing-dev",
	}

	defaultProject := base
	defaultProject.Project = "default"
	if err := ApplyArgoCDApplication(t.Context(), defaultProject); err == nil {
		t.Fatalf("expected default project to be rejected")
	}

	systemNamespace := base
	systemNamespace.DestinationNamespace = "kube-system"
	if err := ApplyArgoCDApplication(t.Context(), systemNamespace); err == nil {
		t.Fatalf("expected kube-system destination namespace to be rejected")
	}

	defaultNamespace := base
	defaultNamespace.DestinationNamespace = "default"
	if err := ApplyArgoCDApplication(t.Context(), defaultNamespace); err == nil {
		t.Fatalf("expected default destination namespace to be rejected")
	}
}

func TestEnsureArgoCDEnvironmentProjectRestrictsDestinationsAndClusterResources(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	cl := fake.NewClientBuilder().Build()
	SetClient(cl)

	if err := EnsureArgoCDEnvironmentProject(
		t.Context(),
		"billing-dev-deploy",
		"billing-dev",
		"http://gitea/paap/billing-dev-components.git",
		[]string{"billing-dev", "billing-dev-app", "billing-dev"},
	); err != nil {
		t.Fatalf("ensure environment project: %v", err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev"}, got); err != nil {
		t.Fatalf("get project: %v", err)
	}
	sourceRepos, _, _ := unstructured.NestedSlice(got.Object, "spec", "sourceRepos")
	destinations, _, _ := unstructured.NestedSlice(got.Object, "spec", "destinations")
	clusterWhitelist, _, _ := unstructured.NestedSlice(got.Object, "spec", "clusterResourceWhitelist")
	namespaceWhitelist, _, _ := unstructured.NestedSlice(got.Object, "spec", "namespaceResourceWhitelist")
	if len(sourceRepos) != 1 || sourceRepos[0] != "http://gitea/paap/billing-dev-components.git" {
		t.Fatalf("unexpected sourceRepos: %#v", sourceRepos)
	}
	if len(destinations) != 2 {
		t.Fatalf("expected deduplicated destinations, got %#v", destinations)
	}
	for _, item := range destinations {
		destination, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected destination item: %#v", item)
		}
		if destination["namespace"] == "*" || destination["server"] == "*" {
			t.Fatalf("project must not allow wildcard destinations: %#v", destinations)
		}
	}
	if len(clusterWhitelist) != 0 {
		t.Fatalf("project must not whitelist cluster resources: %#v", clusterWhitelist)
	}
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
		if !containsArgoCDWhitelist(namespaceWhitelist, want["group"], want["kind"]) {
			t.Fatalf("project namespace whitelist missing %s/%s: %#v", want["group"], want["kind"], namespaceWhitelist)
		}
	}
}

func TestEnsureArgoCDLocalClusterSecretScopesCacheToEnvironmentNamespaces(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	cl := fake.NewClientBuilder().Build()
	SetClient(cl)

	if err := EnsureArgoCDLocalClusterSecret(
		t.Context(),
		"billing-dev-argocd",
		[]string{"billing-dev-monitor", "billing-dev", "billing-dev"},
	); err != nil {
		t.Fatalf("ensure local cluster secret: %v", err)
	}

	var secret corev1.Secret
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-argocd", Name: "paap-local-cluster"}, &secret); err != nil {
		t.Fatalf("get local cluster secret: %v", err)
	}
	if secret.Labels["argocd.argoproj.io/secret-type"] != "cluster" {
		t.Fatalf("missing argocd cluster secret label: %#v", secret.Labels)
	}
	if string(secret.Data["namespaces"]) != "billing-dev,billing-dev-monitor" {
		t.Fatalf("namespaces = %q", secret.Data["namespaces"])
	}
	if string(secret.Data["clusterResources"]) != "false" {
		t.Fatalf("clusterResources = %q", secret.Data["clusterResources"])
	}
	if !strings.Contains(string(secret.Data["config"]), `"inCluster":true`) {
		t.Fatalf("config should use in-cluster auth, got %s", secret.Data["config"])
	}
}

func containsArgoCDWhitelist(items []interface{}, group, kind string) bool {
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

func TestEnsureArgoCDEnvironmentProjectRemovesWildcardSourceRepoOnUpdate(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	project := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "AppProject",
		"metadata": map[string]interface{}{
			"name":      "billing-dev",
			"namespace": "billing-dev-deploy",
		},
		"spec": map[string]interface{}{
			"sourceRepos": []interface{}{"*", "http://gitea/paap/other.git"},
			"destinations": []interface{}{
				map[string]interface{}{"server": "*", "namespace": "*"},
			},
			"clusterResourceWhitelist": []interface{}{
				map[string]interface{}{"group": "*", "kind": "*"},
			},
		},
	}}
	project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	cl := fake.NewClientBuilder().WithObjects(project).Build()
	SetClient(cl)

	if err := EnsureArgoCDEnvironmentProject(
		t.Context(),
		"billing-dev-deploy",
		"billing-dev",
		"http://gitea/paap/billing-dev-components.git",
		[]string{"billing-dev"},
	); err != nil {
		t.Fatalf("ensure environment project: %v", err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev"}, got); err != nil {
		t.Fatalf("get project: %v", err)
	}
	sourceRepos, _, _ := unstructured.NestedSlice(got.Object, "spec", "sourceRepos")
	destinations, _, _ := unstructured.NestedSlice(got.Object, "spec", "destinations")
	clusterWhitelist, _, _ := unstructured.NestedSlice(got.Object, "spec", "clusterResourceWhitelist")
	for _, repo := range sourceRepos {
		if repo == "*" {
			t.Fatalf("project must remove wildcard source repo: %#v", sourceRepos)
		}
	}
	if len(destinations) != 1 {
		t.Fatalf("project must replace wildcard destinations with environment namespaces, got %#v", destinations)
	}
	destination, _ := destinations[0].(map[string]interface{})
	if destination["namespace"] != "billing-dev" || destination["server"] == "*" {
		t.Fatalf("unexpected destination: %#v", destinations)
	}
	if len(clusterWhitelist) != 0 {
		t.Fatalf("project must clear cluster resource whitelist: %#v", clusterWhitelist)
	}
}

func TestEnsureArgoCDDefaultProjectDeniedClearsPermissions(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	project := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "AppProject",
		"metadata": map[string]interface{}{
			"name":      "default",
			"namespace": "billing-dev-deploy",
		},
		"spec": map[string]interface{}{
			"sourceRepos": []interface{}{"*"},
			"destinations": []interface{}{
				map[string]interface{}{"server": "*", "namespace": "*"},
			},
			"clusterResourceWhitelist": []interface{}{
				map[string]interface{}{"group": "*", "kind": "*"},
			},
		},
	}}
	project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	cl := fake.NewClientBuilder().WithObjects(project).Build()
	SetClient(cl)

	if err := EnsureArgoCDDefaultProjectDenied(t.Context(), "billing-dev-deploy"); err != nil {
		t.Fatalf("deny default project: %v", err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "default"}, got); err != nil {
		t.Fatalf("get default project: %v", err)
	}
	for _, path := range [][]string{
		{"spec", "sourceRepos"},
		{"spec", "destinations"},
		{"spec", "clusterResourceWhitelist"},
		{"spec", "namespaceResourceWhitelist"},
	} {
		values, _, _ := unstructured.NestedSlice(got.Object, path...)
		if len(values) != 0 {
			t.Fatalf("%s must be empty, got %#v", path[len(path)-1], values)
		}
	}
}

func TestApplyArgoCDApplicationSetCreatesNamespacedEnvironmentProjectSpec(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	cl := fake.NewClientBuilder().Build()
	SetClient(cl)

	spec := ArgoCDApplicationSetSpec{
		Name:                 "billing-dev-components",
		Namespace:            "billing-dev-deploy",
		Project:              "billing-dev",
		RepoURL:              "http://gitea/paap/billing-dev-components.git",
		Path:                 "components/*",
		TargetRevision:       "main",
		DestinationNamespace: "billing-dev",
	}
	if err := ApplyArgoCDApplicationSet(t.Context(), spec); err != nil {
		t.Fatalf("apply applicationset: %v", err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationSet"})
	if err := cl.Get(t.Context(), client.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev-components"}, got); err != nil {
		t.Fatalf("get applicationset: %v", err)
	}
	project, _, _ := unstructured.NestedString(got.Object, "spec", "template", "spec", "project")
	destNS, _, _ := unstructured.NestedString(got.Object, "spec", "template", "spec", "destination", "namespace")
	if project != "billing-dev" || destNS != "billing-dev" {
		t.Fatalf("ApplicationSet must bind to env project and namespace, project=%q destNS=%q spec=%#v", project, destNS, got.Object["spec"])
	}
	directories, ok, _ := unstructured.NestedSlice(got.Object, "spec", "generators", "0", "git", "directories")
	if !ok {
		generators, _, _ := unstructured.NestedSlice(got.Object, "spec", "generators")
		if len(generators) > 0 {
			generator, _ := generators[0].(map[string]interface{})
			git, _ := generator["git"].(map[string]interface{})
			directories, ok = git["directories"].([]interface{})
		}
	}
	if !ok || len(directories) != 1 {
		t.Fatalf("ApplicationSet git generator must use directories list, spec=%#v", got.Object["spec"])
	}
	directory, ok := directories[0].(map[string]interface{})
	if !ok || directory["path"] != "components/*" {
		t.Fatalf("unexpected ApplicationSet git directories: %#v", directories)
	}
}

func TestApplyArgoCDApplicationSetRejectsDefaultProjectAndSystemNamespace(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().Build())

	base := ArgoCDApplicationSetSpec{
		Name:                 "billing-dev-components",
		Namespace:            "billing-dev-deploy",
		Project:              "billing-dev",
		RepoURL:              "http://gitea/paap/billing-dev-components.git",
		Path:                 "components/*",
		DestinationNamespace: "billing-dev",
	}

	defaultProject := base
	defaultProject.Project = "default"
	if err := ApplyArgoCDApplicationSet(t.Context(), defaultProject); err == nil {
		t.Fatalf("expected default project to be rejected")
	}

	systemNamespace := base
	systemNamespace.DestinationNamespace = "kube-system"
	if err := ApplyArgoCDApplicationSet(t.Context(), systemNamespace); err == nil {
		t.Fatalf("expected kube-system destination namespace to be rejected")
	}

	defaultNamespace := base
	defaultNamespace.DestinationNamespace = "default"
	if err := ApplyArgoCDApplicationSet(t.Context(), defaultNamespace); err == nil {
		t.Fatalf("expected default destination namespace to be rejected")
	}
}
