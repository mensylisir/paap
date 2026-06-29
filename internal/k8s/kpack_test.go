package k8s

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnsureKpackBuildEnvironmentCreatesServiceAccountAndBuilderResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiextensions scheme: %v", err)
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			kpackCRD("builders.kpack.io"),
			kpackCRD("builds.kpack.io"),
			kpackCRD("clusterstores.kpack.io"),
			kpackCRD("clusterstacks.kpack.io"),
			kpackCRD("images.kpack.io"),
			kpackCRD("sourceresolvers.kpack.io"),
			kpackCRD("clusterlifecycles.kpack.io"),
			kpackControllerDeployment(),
		).
		Build()

	status := EnsureKpackBuildEnvironment(context.Background(), cl, KpackBuildEnvironmentSpec{
		Namespace:        "shop-dev-ci",
		BuilderImage:     "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:v1",
		RegistryServer:   "registry.shop-dev.corp.example.com:5443",
		RegistryCAPEM:    []byte("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"),
		GitServer:        "http://gitea.shop-dev.svc.cluster.local:3000",
		GitUsername:      "paap",
		GitPassword:      "paap123456",
		StackBuildImage:  PaketoBuildJammyBaseImage,
		StackRunImage:    PaketoRunJammyBaseImage,
		BuildpackSources: []string{PaketoJavaBuildpackImage, PaketoNodeJSBuildpackImage},
	})
	if !status.Ready || status.Warning != "" {
		t.Fatalf("unexpected bootstrap status: %#v", status)
	}

	var sa corev1.ServiceAccount
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackBuildServiceAccountName, Namespace: "shop-dev-ci"}, &sa); err != nil {
		t.Fatalf("expected build service account: %v", err)
	}
	foundRegistrySecret := false
	for _, ref := range sa.Secrets {
		if ref.Name == KpackRegistrySecretName {
			foundRegistrySecret = true
		}
	}
	if !foundRegistrySecret {
		t.Fatalf("expected registry secret to be mounted on service account secrets: %#v", sa.Secrets)
	}
	if len(sa.ImagePullSecrets) != 1 || sa.ImagePullSecrets[0].Name != KpackRegistrySecretName {
		t.Fatalf("expected registry secret to be mounted on imagePullSecrets: %#v", sa.ImagePullSecrets)
	}
	foundCASecret := false
	for _, ref := range sa.Secrets {
		if ref.Name == KpackRegistryCASecretName {
			foundCASecret = true
		}
	}
	if !foundCASecret {
		t.Fatalf("expected registry CA secret to be mounted on service account secrets: %#v", sa.Secrets)
	}
	foundGitSecret := false
	for _, ref := range sa.Secrets {
		if ref.Name == KpackGitSecretName {
			foundGitSecret = true
		}
	}
	if !foundGitSecret {
		t.Fatalf("expected git secret to be mounted on service account secrets: %#v", sa.Secrets)
	}
	var registrySecret corev1.Secret
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackRegistrySecretName, Namespace: "shop-dev-ci"}, &registrySecret); err != nil {
		t.Fatalf("expected registry docker config secret: %v", err)
	}
	if registrySecret.Type != corev1.SecretTypeDockerConfigJson {
		t.Fatalf("registry secret type = %q", registrySecret.Type)
	}
	dockerConfig := string(registrySecret.Data[corev1.DockerConfigJsonKey])
	var dockerConfigJSON struct {
		Auths map[string]map[string]string `json:"auths"`
	}
	if err := json.Unmarshal(registrySecret.Data[corev1.DockerConfigJsonKey], &dockerConfigJSON); err != nil {
		t.Fatalf("docker config should be valid json: %v", err)
	}
	if len(dockerConfigJSON.Auths) != 0 {
		t.Fatalf("public registry docker config must not contain empty auth entries, got %s", dockerConfig)
	}
	var caSecret corev1.Secret
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackRegistryCASecretName, Namespace: "shop-dev-ci"}, &caSecret); err != nil {
		t.Fatalf("expected registry CA binding secret: %v", err)
	}
	if string(caSecret.Data["type"]) != "ca-certificates" {
		t.Fatalf("registry CA binding type = %q", caSecret.Data["type"])
	}
	if string(caSecret.Data["ca.crt"]) != "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n" {
		t.Fatalf("registry CA data = %q", caSecret.Data["ca.crt"])
	}
	var gitSecret corev1.Secret
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackGitSecretName, Namespace: "shop-dev-ci"}, &gitSecret); err != nil {
		t.Fatalf("expected git basic auth secret: %v", err)
	}
	if gitSecret.Type != corev1.SecretTypeBasicAuth {
		t.Fatalf("git secret type = %q", gitSecret.Type)
	}
	if string(gitSecret.Data[corev1.BasicAuthUsernameKey]) != "paap" || string(gitSecret.Data[corev1.BasicAuthPasswordKey]) != "paap123456" {
		t.Fatalf("unexpected git secret credentials: %#v", gitSecret.Data)
	}
	if gitSecret.Annotations["kpack.io/git"] != "http://gitea.shop-dev.svc.cluster.local:3000" {
		t.Fatalf("git secret annotation = %#v", gitSecret.Annotations)
	}
	var controllerCA corev1.Secret
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackControllerRegistryCASecretName, Namespace: KpackSystemNamespace}, &controllerCA); err != nil {
		t.Fatalf("expected kpack controller registry CA secret: %v", err)
	}
	if string(controllerCA.Data["ca.crt"]) != "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n" {
		t.Fatalf("controller registry CA data = %q", controllerCA.Data["ca.crt"])
	}
	var controller appsv1.Deployment
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackControllerDeploymentName, Namespace: KpackSystemNamespace}, &controller); err != nil {
		t.Fatalf("expected kpack controller deployment: %v", err)
	}
	if !deploymentHasRegistryCAVolume(controller) {
		t.Fatalf("controller deployment should mount registry CA secret: %#v", controller.Spec.Template.Spec)
	}
	if !deploymentHasEnv(controller, KpackControllerContainerName, "SSL_CERT_FILE", "/etc/ssl/certs/paap-registry-ca.crt") {
		t.Fatalf("controller deployment should point SSL_CERT_FILE at registry CA: %#v", controller.Spec.Template.Spec.Containers)
	}
	var role rbacv1.Role
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackBuildServiceAccountName, Namespace: "shop-dev-ci"}, &role); err != nil {
		t.Fatalf("expected build role: %v", err)
	}

	builder := &unstructured.Unstructured{}
	builder.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Builder"})
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackBuilderName, Namespace: "shop-dev-ci"}, builder); err != nil {
		t.Fatalf("expected kpack builder: %v", err)
	}
	tag, _, _ := unstructured.NestedString(builder.Object, "spec", "tag")
	if tag != "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:v1" {
		t.Fatalf("builder tag = %q", tag)
	}
	stackName, _, _ := unstructured.NestedString(builder.Object, "spec", "stack", "name")
	if stackName != KpackClusterStackName {
		t.Fatalf("unexpected builder refs: %#v", builder.Object["spec"])
	}
	lifecycleName, _, _ := unstructured.NestedString(builder.Object, "spec", "lifecycle", "name")
	if lifecycleName != KpackClusterLifecycleName {
		t.Fatalf("unexpected lifecycle ref: %#v", builder.Object["spec"])
	}
	const expectedClusterStoreName = "paap-store"
	storeName, ok, _ := unstructured.NestedString(builder.Object, "spec", "store", "name")
	if !ok || storeName != expectedClusterStoreName {
		t.Fatalf("builder should reference ClusterStore %q, got %#v", expectedClusterStoreName, builder.Object["spec"])
	}
	order, ok, _ := unstructured.NestedSlice(builder.Object, "spec", "order")
	if !ok || len(order) != 2 {
		t.Fatalf("expected one optional order group per buildpack, got %#v", builder.Object["spec"])
	}
	firstGroup, ok := order[0].(map[string]interface{})["group"].([]interface{})
	if !ok || len(firstGroup) != 1 {
		t.Fatalf("expected first order entry to contain one buildpack, got %#v", order[0])
	}
	firstBuildpack, ok := firstGroup[0].(map[string]interface{})
	if !ok || firstBuildpack["id"] != "paketo-buildpacks/java" {
		t.Fatalf("unexpected first buildpack ref: %#v", firstGroup[0])
	}
	if _, hasImage := firstBuildpack["image"]; hasImage {
		t.Fatalf("builder order must reference buildpack ids from ClusterStore, not direct images: %#v", firstBuildpack)
	}

	store := &unstructured.Unstructured{}
	store.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "ClusterStore"})
	if err := cl.Get(context.Background(), types.NamespacedName{Name: expectedClusterStoreName}, store); err != nil {
		t.Fatalf("expected cluster store: %v", err)
	}
	sources, ok, _ := unstructured.NestedSlice(store.Object, "spec", "sources")
	if !ok || len(sources) != 2 {
		t.Fatalf("expected store sources for buildpack images, got %#v", store.Object["spec"])
	}
	firstSource, ok := sources[0].(map[string]interface{})
	if !ok || firstSource["image"] != PaketoJavaBuildpackImage {
		t.Fatalf("unexpected first store source: %#v", sources[0])
	}

	stack := &unstructured.Unstructured{}
	stack.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "ClusterStack"})
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackClusterStackName}, stack); err != nil {
		t.Fatalf("expected cluster stack: %v", err)
	}
	runImage, _, _ := unstructured.NestedString(stack.Object, "spec", "runImage", "image")
	if runImage != PaketoRunJammyBaseImage {
		t.Fatalf("run image = %q", runImage)
	}
}

func TestKpackBuildpackRefFromMirroredSourceWithRegistryPort(t *testing.T) {
	ref := kpackBuildpackRefFromSource("piggymetrics-dev-registry.piggymetrics-dev-registry.svc.cluster.local:5000/piggymetrics-dev/paap-buildpack-java:22.0.0")
	if ref["id"] != "paketo-buildpacks/java" {
		t.Fatalf("mirrored buildpack id = %#v", ref)
	}
	if ref["version"] != "22.0.0" {
		t.Fatalf("mirrored buildpack version = %#v", ref)
	}
	repo := imageReferenceRepository("piggymetrics-dev-registry.piggymetrics-dev-registry.svc.cluster.local:5000/piggymetrics-dev/paap-buildpack-nodejs:10.3.2")
	if repo != "piggymetrics-dev-registry.piggymetrics-dev-registry.svc.cluster.local:5000/piggymetrics-dev/paap-buildpack-nodejs" {
		t.Fatalf("repository parsed incorrectly: %q", repo)
	}
}

func TestEnsureKpackBuildEnvironmentRecreatesStaleFailedBuilder(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiextensions scheme: %v", err)
	}
	staleBuilder := kpackObject("Builder", "shop-dev-ci", KpackBuilderName)
	staleBuilder.SetGeneration(3)
	staleBuilder.Object["spec"] = map[string]interface{}{
		"tag": "registry.old.example.com/shop-dev/paap-builder:latest",
	}
	staleBuilder.Object["status"] = map[string]interface{}{
		"observedGeneration": int64(2),
		"conditions": []interface{}{
			map[string]interface{}{
				"type":    "UpToDate",
				"status":  "False",
				"reason":  "ReconcileFailed",
				"message": "could not find buildpack with id 'paketo-buildpacks/java'",
			},
		},
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			kpackCRD("builders.kpack.io"),
			kpackCRD("builds.kpack.io"),
			kpackCRD("clusterstores.kpack.io"),
			kpackCRD("clusterstacks.kpack.io"),
			kpackCRD("images.kpack.io"),
			kpackCRD("sourceresolvers.kpack.io"),
			kpackCRD("clusterlifecycles.kpack.io"),
			kpackControllerDeployment(),
			staleBuilder,
		).
		Build()

	status := EnsureKpackBuildEnvironment(context.Background(), cl, KpackBuildEnvironmentSpec{
		Namespace:        "shop-dev-ci",
		BuilderImage:     "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest",
		RegistryServer:   "registry.shop-dev.corp.example.com:5443",
		BuildpackSources: []string{PaketoJavaBuildpackImage},
	})
	if !status.Ready || status.Warning != "" {
		t.Fatalf("unexpected bootstrap status: %#v", status)
	}

	builder := kpackObject("Builder", "shop-dev-ci", KpackBuilderName)
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackBuilderName, Namespace: "shop-dev-ci"}, builder); err != nil {
		t.Fatalf("expected recreated builder: %v", err)
	}
	if _, ok, _ := unstructured.NestedSlice(builder.Object, "status", "conditions"); ok {
		t.Fatalf("recreated builder should not keep stale failed status: %#v", builder.Object["status"])
	}
	tag, _, _ := unstructured.NestedString(builder.Object, "spec", "tag")
	if tag != "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest" {
		t.Fatalf("builder tag = %q", tag)
	}
}

func TestKpackRegistryCompatibilityAllowsClusterServiceRegistryForInternalBuilds(t *testing.T) {
	warning := kpackRegistryCompatibilityWarning("registry.shop-dev-registry.svc.cluster.local:5000")
	if warning != "" {
		t.Fatalf("warning = %q", warning)
	}
}

func TestKpackRegistryCompatibilityWarnsForDefaultPlaceholderHost(t *testing.T) {
	warning := kpackRegistryCompatibilityWarning("registry.shop-dev.paap.local:5000")
	if !strings.Contains(warning, "PAAP placeholder registry host") || !strings.Contains(warning, "PAAP_REGISTRY_HOST_TEMPLATE") {
		t.Fatalf("warning = %q", warning)
	}
}

func TestKpackRegistryCompatibilityAllowsConfiguredKindRegistryHost(t *testing.T) {
	if warning := kpackRegistryCompatibilityWarning("registry.paap.local:5000"); warning != "" {
		t.Fatalf("warning = %q", warning)
	}
}

func TestKpackRegistryCompatibilityAllowsTrustedExternalHost(t *testing.T) {
	if warning := kpackRegistryCompatibilityWarning("registry.shop-dev.corp.example.com:5443"); warning != "" {
		t.Fatalf("warning = %q", warning)
	}
}

func TestGetKpackImageStatusReadsReadyConditionAndLatestImage(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(kpackImage("shop-dev-ci", "orders-api", "registry.example.com/shop/orders-api:v2", "True", "", "")).
		Build()

	status, err := GetKpackImageStatus(context.Background(), cl, "shop-dev-ci", "orders-api")
	if err != nil {
		t.Fatalf("get kpack image status: %v", err)
	}
	if !status.Exists || !status.Ready || status.LatestImage != "registry.example.com/shop/orders-api:v2" || status.Warning != "" {
		t.Fatalf("unexpected kpack image status: %#v", status)
	}
}

func TestGetKpackImageStatusReportsNotReadyWarning(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(kpackImage("shop-dev-ci", "orders-api", "", "False", "BuildFailed", "compile failed")).
		Build()

	status, err := GetKpackImageStatus(context.Background(), cl, "shop-dev-ci", "orders-api")
	if err != nil {
		t.Fatalf("get kpack image status: %v", err)
	}
	if !status.Exists || status.Ready || !strings.Contains(status.Warning, "BuildFailed") || !strings.Contains(status.Warning, "compile failed") {
		t.Fatalf("unexpected not-ready status: %#v", status)
	}
}

func TestEnsureKpackBuildEnvironmentReportsMissingCRDs(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiextensions scheme: %v", err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()

	status := EnsureKpackBuildEnvironment(context.Background(), cl, KpackBuildEnvironmentSpec{
		Namespace:     "shop-dev-ci",
		RegistryCAPEM: []byte("registry-ca"),
	})
	if status.Ready {
		t.Fatalf("expected bootstrap to be pending without CRDs")
	}
	if !strings.Contains(status.Warning, "builders.kpack.io") {
		t.Fatalf("warning should name missing CRD, got %q", status.Warning)
	}

	var sa corev1.ServiceAccount
	if err := cl.Get(context.Background(), types.NamespacedName{Name: KpackBuildServiceAccountName, Namespace: "shop-dev-ci"}, &sa); err != nil {
		t.Fatalf("service account should still be created: %v", err)
	}
}

func TestEnsureKpackBuildEnvironmentRequiresBuildAndSourceResolverCRDs(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiextensions scheme: %v", err)
	}
	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			kpackCRD("builders.kpack.io"),
			kpackCRD("clusterstores.kpack.io"),
			kpackCRD("clusterstacks.kpack.io"),
			kpackCRD("clusterlifecycles.kpack.io"),
			kpackCRD("images.kpack.io"),
		).
		Build()

	status := EnsureKpackBuildEnvironment(context.Background(), cl, KpackBuildEnvironmentSpec{
		Namespace:      "shop-dev-ci",
		RegistryServer: "registry.shop-dev.corp.example.com:5443",
		BuilderImage:   "registry.shop-dev.corp.example.com:5443/shop-dev/paap-builder:latest",
	})
	if status.Ready {
		t.Fatalf("expected bootstrap to stay pending without Build and SourceResolver CRDs")
	}
	if !strings.Contains(status.Warning, "builds.kpack.io") || !strings.Contains(status.Warning, "sourceresolvers.kpack.io") {
		t.Fatalf("warning should name missing Build and SourceResolver CRDs, got %q", status.Warning)
	}
}

func kpackCRD(name string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

func kpackImage(namespace, name, latestImage, readyStatus, reason, message string) *unstructured.Unstructured {
	condition := map[string]interface{}{
		"type":   "Ready",
		"status": readyStatus,
	}
	if reason != "" {
		condition["reason"] = reason
	}
	if message != "" {
		condition["message"] = message
	}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "kpack.io/v1alpha2",
		"kind":       "Image",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"status": map[string]interface{}{
			"latestImage": latestImage,
			"conditions":  []interface{}{condition},
		},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: "Image"})
	return obj
}

func kpackControllerDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: KpackControllerDeploymentName, Namespace: KpackSystemNamespace},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: KpackControllerContainerName}},
				},
			},
		},
	}
}

func deploymentHasRegistryCAVolume(deploy appsv1.Deployment) bool {
	hasVolume := false
	for _, volume := range deploy.Spec.Template.Spec.Volumes {
		if volume.Name == KpackControllerRegistryCAVolumeName && volume.Secret != nil && volume.Secret.SecretName == KpackControllerRegistryCASecretName {
			hasVolume = true
		}
	}
	hasMount := false
	for _, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name != KpackControllerContainerName {
			continue
		}
		for _, mount := range container.VolumeMounts {
			if mount.Name == KpackControllerRegistryCAVolumeName && strings.Contains(mount.MountPath, "paap-registry-ca.crt") && mount.SubPath == "ca.crt" {
				hasMount = true
			}
		}
	}
	return hasVolume && hasMount
}

func deploymentHasEnv(deploy appsv1.Deployment, containerName, name, value string) bool {
	for _, container := range deploy.Spec.Template.Spec.Containers {
		if container.Name != containerName {
			continue
		}
		for _, env := range container.Env {
			if env.Name == name && env.Value == value {
				return true
			}
		}
	}
	return false
}
