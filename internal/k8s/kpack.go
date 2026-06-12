package k8s

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	KpackBuildServiceAccountName        = "paap-kpack-build"
	KpackRegistrySecretName             = "paap-kpack-registry"
	KpackRegistryCASecretName           = "paap-kpack-registry-ca"
	KpackGitSecretName                  = "paap-kpack-git"
	KpackSystemNamespace                = "kpack"
	KpackControllerDeploymentName       = "kpack-controller"
	KpackControllerContainerName        = "controller"
	KpackControllerRegistryCASecretName = "paap-kpack-controller-registry-ca"
	KpackControllerRegistryCAVolumeName = "paap-registry-ca"
	KpackBuilderName                    = "paap-builder"
	KpackClusterStoreName               = "paap-store"
	KpackClusterStackName               = "paap-stack"
	KpackClusterLifecycleName           = "default-lifecycle"
	PaketoBuildJammyBaseImage           = "paketobuildpacks/build-jammy-base:0.1.233"
	PaketoRunJammyBaseImage             = "paketobuildpacks/run-jammy-base:0.1.233"
	PaketoJavaBuildpackImage            = "paketobuildpacks/java:22.0.0"
	PaketoNodeJSBuildpackImage          = "paketobuildpacks/nodejs:10.3.2"
	PaketoGoBuildpackImage              = "paketobuildpacks/go:4.19.14"
	PaketoPythonBuildpackImage          = "paketobuildpacks/python:2.49.0"
)

type KpackBuildEnvironmentSpec struct {
	Namespace        string
	RegistryServer   string
	RegistryUsername string
	RegistryPassword string
	RegistryCAPEM    []byte
	GitServer        string
	GitUsername      string
	GitPassword      string
	BuilderImage     string
	StackBuildImage  string
	StackRunImage    string
	BuildpackSources []string
}

type KpackBuildEnvironmentStatus struct {
	Ready           bool
	Warning         string
	RegistryWarning string
}

func EnsureKpackBuildEnvironment(ctx context.Context, cl client.Client, spec KpackBuildEnvironmentSpec) KpackBuildEnvironmentStatus {
	if cl == nil {
		return KpackBuildEnvironmentStatus{Warning: "k8s client not initialized"}
	}
	spec = defaultKpackBuildEnvironmentSpec(spec)
	registryWarning := kpackRegistryCompatibilityWarning(spec.RegistryServer)
	warnings := make([]string, 0)
	if err := ensureKpackBuildServiceAccountWithRegistry(ctx, cl, spec); err != nil {
		return KpackBuildEnvironmentStatus{Warning: fmt.Sprintf("kpack build service account sync failed: %v", err), RegistryWarning: registryWarning}
	}
	missing := missingKpackCRDs(ctx, cl)
	if len(missing) > 0 {
		warnings = append(warnings, "missing kpack CRDs/controller: "+strings.Join(missing, ", "))
		return KpackBuildEnvironmentStatus{Warning: strings.Join(warnings, "; "), RegistryWarning: registryWarning}
	}
	if len(spec.RegistryCAPEM) > 0 {
		if err := ensureKpackControllerRegistryTrust(ctx, cl, spec.RegistryCAPEM); err != nil {
			return KpackBuildEnvironmentStatus{Warning: fmt.Sprintf("kpack controller registry trust sync failed: %v", err), RegistryWarning: registryWarning}
		}
	}
	if err := upsertKpackClusterStack(ctx, cl, spec.StackBuildImage, spec.StackRunImage); err != nil {
		return KpackBuildEnvironmentStatus{Warning: fmt.Sprintf("kpack ClusterStack sync failed: %v", err), RegistryWarning: registryWarning}
	}
	if err := upsertKpackClusterStore(ctx, cl, spec.BuildpackSources); err != nil {
		return KpackBuildEnvironmentStatus{Warning: fmt.Sprintf("kpack ClusterStore sync failed: %v", err), RegistryWarning: registryWarning}
	}
	if err := upsertKpackBuilder(ctx, cl, spec); err != nil {
		return KpackBuildEnvironmentStatus{Warning: fmt.Sprintf("kpack Builder sync failed: %v", err), RegistryWarning: registryWarning}
	}
	return KpackBuildEnvironmentStatus{Ready: true, RegistryWarning: registryWarning}
}

func defaultKpackBuildEnvironmentSpec(spec KpackBuildEnvironmentSpec) KpackBuildEnvironmentSpec {
	if strings.TrimSpace(spec.Namespace) == "" {
		spec.Namespace = "default"
	}
	if strings.TrimSpace(spec.RegistryServer) == "" {
		spec.RegistryServer = registryServerFromImage(spec.BuilderImage)
	}
	if strings.TrimSpace(spec.BuilderImage) == "" {
		spec.BuilderImage = "registry.local/paap-builder:latest"
	}
	if strings.TrimSpace(spec.StackBuildImage) == "" {
		spec.StackBuildImage = PaketoBuildJammyBaseImage
	}
	if strings.TrimSpace(spec.StackRunImage) == "" {
		spec.StackRunImage = PaketoRunJammyBaseImage
	}
	if len(spec.BuildpackSources) == 0 {
		spec.BuildpackSources = []string{
			PaketoJavaBuildpackImage,
			PaketoNodeJSBuildpackImage,
			PaketoGoBuildpackImage,
			PaketoPythonBuildpackImage,
		}
	}
	return spec
}

func ensureKpackBuildServiceAccountWithRegistry(ctx context.Context, cl client.Client, spec KpackBuildEnvironmentSpec) error {
	namespace := spec.Namespace
	registryServer := strings.TrimSpace(spec.RegistryServer)
	if registryServer == "" {
		registryServer = registryServerFromImage(spec.BuilderImage)
	}
	if registryServer != "" {
		if err := ensureKpackRegistrySecret(ctx, cl, namespace, registryServer, spec.RegistryUsername, spec.RegistryPassword); err != nil {
			return err
		}
	}
	if len(spec.RegistryCAPEM) > 0 {
		if err := ensureKpackRegistryCASecret(ctx, cl, namespace, spec.RegistryCAPEM); err != nil {
			return err
		}
	}
	if strings.TrimSpace(spec.GitServer) != "" {
		if err := ensureKpackGitSecret(ctx, cl, namespace, spec.GitServer, spec.GitUsername, spec.GitPassword); err != nil {
			return err
		}
	}
	sa := &corev1.ServiceAccount{}
	if err := cl.Get(ctx, types.NamespacedName{Name: KpackBuildServiceAccountName, Namespace: namespace}, sa); apierrors.IsNotFound(err) {
		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      KpackBuildServiceAccountName,
				Namespace: namespace,
				Labels:    kpackBootstrapLabels(),
			},
		}
		mountKpackSecrets(sa, registryServer, spec.RegistryCAPEM, spec.GitServer)
		if err := cl.Create(ctx, sa); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		sa.Labels = mergeLabels(sa.Labels, kpackBootstrapLabels())
		mountKpackSecrets(sa, registryServer, spec.RegistryCAPEM, spec.GitServer)
		if err := cl.Update(ctx, sa); err != nil {
			return err
		}
	}

	role := &rbacv1.Role{}
	roleRules := []rbacv1.PolicyRule{
		{APIGroups: []string{"kpack.io"}, Resources: []string{"images", "builds", "sourceresolvers"}, Verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}},
		{APIGroups: []string{""}, Resources: []string{"secrets", "configmaps", "pods", "pods/log"}, Verbs: []string{"get", "list", "watch"}},
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: KpackBuildServiceAccountName, Namespace: namespace}, role); apierrors.IsNotFound(err) {
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: KpackBuildServiceAccountName, Namespace: namespace, Labels: kpackBootstrapLabels()},
			Rules:      roleRules,
		}
		if err := cl.Create(ctx, role); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		role.Labels = mergeLabels(role.Labels, kpackBootstrapLabels())
		role.Rules = roleRules
		if err := cl.Update(ctx, role); err != nil {
			return err
		}
	}

	binding := &rbacv1.RoleBinding{}
	if err := cl.Get(ctx, types.NamespacedName{Name: KpackBuildServiceAccountName, Namespace: namespace}, binding); apierrors.IsNotFound(err) {
		binding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: KpackBuildServiceAccountName, Namespace: namespace, Labels: kpackBootstrapLabels()},
			Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: KpackBuildServiceAccountName, Namespace: namespace}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: KpackBuildServiceAccountName},
		}
		return cl.Create(ctx, binding)
	} else if err != nil {
		return err
	}
	binding.Labels = mergeLabels(binding.Labels, kpackBootstrapLabels())
	binding.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: KpackBuildServiceAccountName, Namespace: namespace}}
	binding.RoleRef = rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: KpackBuildServiceAccountName}
	return cl.Update(ctx, binding)
}

func ensureKpackGitSecret(ctx context.Context, cl client.Client, namespace, gitServer, username, password string) error {
	gitServer = strings.TrimRight(strings.TrimSpace(gitServer), "/")
	if gitServer == "" {
		return nil
	}
	data := map[string][]byte{
		corev1.BasicAuthUsernameKey: []byte(username),
		corev1.BasicAuthPasswordKey: []byte(password),
	}
	annotations := map[string]string{"kpack.io/git": gitServer}
	secret := &corev1.Secret{}
	err := cl.Get(ctx, types.NamespacedName{Name: KpackGitSecretName, Namespace: namespace}, secret)
	if apierrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        KpackGitSecretName,
				Namespace:   namespace,
				Labels:      kpackBootstrapLabels(),
				Annotations: annotations,
			},
			Type: corev1.SecretTypeBasicAuth,
			Data: data,
		}
		return cl.Create(ctx, secret)
	}
	if err != nil {
		return err
	}
	secret.Labels = mergeLabels(secret.Labels, kpackBootstrapLabels())
	secret.Annotations = mergeLabels(secret.Annotations, annotations)
	secret.Type = corev1.SecretTypeBasicAuth
	secret.Data = data
	return cl.Update(ctx, secret)
}

func ensureKpackRegistryCASecret(ctx context.Context, cl client.Client, namespace string, caPEM []byte) error {
	secret := &corev1.Secret{}
	data := map[string][]byte{
		"type":   []byte("ca-certificates"),
		"ca.crt": caPEM,
	}
	err := cl.Get(ctx, types.NamespacedName{Name: KpackRegistryCASecretName, Namespace: namespace}, secret)
	if apierrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      KpackRegistryCASecretName,
				Namespace: namespace,
				Labels:    kpackBootstrapLabels(),
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		return cl.Create(ctx, secret)
	}
	if err != nil {
		return err
	}
	secret.Labels = mergeLabels(secret.Labels, kpackBootstrapLabels())
	secret.Type = corev1.SecretTypeOpaque
	secret.Data = data
	return cl.Update(ctx, secret)
}

func ensureKpackControllerRegistryTrust(ctx context.Context, cl client.Client, caPEM []byte) error {
	if len(caPEM) == 0 {
		return nil
	}
	if err := ensureKpackControllerRegistryCASecret(ctx, cl, caPEM); err != nil {
		return err
	}
	return patchKpackControllerRegistryCAMount(ctx, cl)
}

func ensureKpackControllerRegistryCASecret(ctx context.Context, cl client.Client, caPEM []byte) error {
	secret := &corev1.Secret{}
	data := map[string][]byte{"ca.crt": caPEM}
	err := cl.Get(ctx, types.NamespacedName{Name: KpackControllerRegistryCASecretName, Namespace: KpackSystemNamespace}, secret)
	if apierrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      KpackControllerRegistryCASecretName,
				Namespace: KpackSystemNamespace,
				Labels:    kpackBootstrapLabels(),
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		return cl.Create(ctx, secret)
	}
	if err != nil {
		return err
	}
	secret.Labels = mergeLabels(secret.Labels, kpackBootstrapLabels())
	secret.Type = corev1.SecretTypeOpaque
	secret.Data = data
	return cl.Update(ctx, secret)
}

func patchKpackControllerRegistryCAMount(ctx context.Context, cl client.Client) error {
	deploy := &appsv1.Deployment{}
	if err := cl.Get(ctx, types.NamespacedName{Name: KpackControllerDeploymentName, Namespace: KpackSystemNamespace}, deploy); err != nil {
		return err
	}

	deploy.Spec.Template.Spec.Volumes = appendSecretVolume(
		deploy.Spec.Template.Spec.Volumes,
		KpackControllerRegistryCAVolumeName,
		KpackControllerRegistryCASecretName,
	)
	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name != KpackControllerContainerName {
			continue
		}
		deploy.Spec.Template.Spec.Containers[i].VolumeMounts = appendVolumeMount(
			deploy.Spec.Template.Spec.Containers[i].VolumeMounts,
			corev1.VolumeMount{
				Name:      KpackControllerRegistryCAVolumeName,
				MountPath: "/etc/ssl/certs/paap-registry-ca.crt",
				SubPath:   "ca.crt",
				ReadOnly:  true,
			},
		)
		deploy.Spec.Template.Spec.Containers[i].Env = appendEnvVar(
			deploy.Spec.Template.Spec.Containers[i].Env,
			corev1.EnvVar{Name: "SSL_CERT_FILE", Value: "/etc/ssl/certs/paap-registry-ca.crt"},
		)
	}
	return cl.Update(ctx, deploy)
}

func ensureKpackRegistrySecret(ctx context.Context, cl client.Client, namespace, registryServer, username, password string) error {
	auths := map[string]interface{}{
		registryServer: map[string]string{},
	}
	if username != "" || password != "" {
		auths[registryServer] = map[string]string{
			"username": username,
			"password": password,
			"auth":     base64.StdEncoding.EncodeToString([]byte(username + ":" + password)),
		}
	}
	data, err := json.Marshal(map[string]interface{}{"auths": auths})
	if err != nil {
		return err
	}
	secret := &corev1.Secret{}
	err = cl.Get(ctx, types.NamespacedName{Name: KpackRegistrySecretName, Namespace: namespace}, secret)
	if apierrors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      KpackRegistrySecretName,
				Namespace: namespace,
				Labels:    kpackBootstrapLabels(),
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{corev1.DockerConfigJsonKey: data},
		}
		return cl.Create(ctx, secret)
	}
	if err != nil {
		return err
	}
	secret.Labels = mergeLabels(secret.Labels, kpackBootstrapLabels())
	secret.Type = corev1.SecretTypeDockerConfigJson
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[corev1.DockerConfigJsonKey] = data
	return cl.Update(ctx, secret)
}

func mountKpackSecrets(sa *corev1.ServiceAccount, registryServer string, caPEM []byte, gitServer string) {
	if strings.TrimSpace(registryServer) == "" {
	} else {
		sa.Secrets = appendObjectReference(sa.Secrets, KpackRegistrySecretName)
		sa.ImagePullSecrets = appendLocalObjectReference(sa.ImagePullSecrets, KpackRegistrySecretName)
		if len(caPEM) > 0 {
			sa.Secrets = appendObjectReference(sa.Secrets, KpackRegistryCASecretName)
		}
	}
	if strings.TrimSpace(gitServer) != "" {
		sa.Secrets = appendObjectReference(sa.Secrets, KpackGitSecretName)
	}
}

func appendSecretVolume(items []corev1.Volume, name, secretName string) []corev1.Volume {
	for i := range items {
		if items[i].Name == name {
			items[i].VolumeSource = corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{SecretName: secretName},
			}
			return items
		}
	}
	return append(items, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{SecretName: secretName},
		},
	})
}

func appendVolumeMount(items []corev1.VolumeMount, desired corev1.VolumeMount) []corev1.VolumeMount {
	for i := range items {
		if items[i].Name == desired.Name || items[i].MountPath == desired.MountPath {
			items[i] = desired
			return items
		}
	}
	return append(items, desired)
}

func appendEnvVar(items []corev1.EnvVar, desired corev1.EnvVar) []corev1.EnvVar {
	for i := range items {
		if items[i].Name == desired.Name {
			items[i] = desired
			return items
		}
	}
	return append(items, desired)
}

func appendObjectReference(items []corev1.ObjectReference, name string) []corev1.ObjectReference {
	for _, item := range items {
		if item.Name == name {
			return items
		}
	}
	return append(items, corev1.ObjectReference{Name: name})
}

func appendLocalObjectReference(items []corev1.LocalObjectReference, name string) []corev1.LocalObjectReference {
	for _, item := range items {
		if item.Name == name {
			return items
		}
	}
	return append(items, corev1.LocalObjectReference{Name: name})
}

func registryServerFromImage(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	first := strings.Split(image, "/")[0]
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first
	}
	return ""
}

func kpackRegistryCompatibilityWarning(registryServer string) string {
	registryServer = strings.TrimSpace(registryServer)
	if strings.HasPrefix(registryServer, "registry.") && strings.Contains(registryServer, ".svc.cluster.local:5000") {
		return "kpack cannot reliably push to a registry exposed only through svc.cluster.local because node runtimes cannot pull that image reference and kpack v0.17.0 does not expose a PAAP-managed insecure registry option; use a node-reachable trusted TLS registry for source builds"
	}
	if registryServer != "registry.paap.local:5000" && registryServer != "registry.paap.local" && (strings.HasSuffix(registryServer, ".paap.local:5000") || strings.HasSuffix(registryServer, ".paap.local")) {
		return "PAAP placeholder registry host is not enough for source builds; set PAAP_REGISTRY_HOST_TEMPLATE to a node-reachable trusted TLS registry host"
	}
	return ""
}

func missingKpackCRDs(ctx context.Context, cl client.Client) []string {
	required := []string{
		"builds.kpack.io",
		"builders.kpack.io",
		"clusterstores.kpack.io",
		"clusterstacks.kpack.io",
		"clusterlifecycles.kpack.io",
		"images.kpack.io",
		"sourceresolvers.kpack.io",
	}
	missing := make([]string, 0)
	for _, name := range required {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := cl.Get(ctx, types.NamespacedName{Name: name}, crd); err != nil {
			missing = append(missing, name)
		}
	}
	return missing
}

func upsertKpackClusterStack(ctx context.Context, cl client.Client, buildImage, runImage string) error {
	obj := kpackObject("ClusterStack", "", KpackClusterStackName)
	obj.Object["spec"] = map[string]interface{}{
		"id": "io.buildpacks.stacks.jammy",
		"buildImage": map[string]interface{}{
			"image": buildImage,
		},
		"runImage": map[string]interface{}{
			"image": runImage,
		},
	}
	return upsertUnstructured(ctx, cl, obj)
}

func upsertKpackClusterStore(ctx context.Context, cl client.Client, buildpackSources []string) error {
	obj := kpackObject("ClusterStore", "", KpackClusterStoreName)
	sources := make([]interface{}, 0, len(buildpackSources))
	for _, source := range buildpackSources {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		sources = append(sources, map[string]interface{}{"image": source})
	}
	if len(sources) == 0 {
		for _, source := range []string{PaketoJavaBuildpackImage, PaketoNodeJSBuildpackImage, PaketoGoBuildpackImage, PaketoPythonBuildpackImage} {
			sources = append(sources, map[string]interface{}{"image": source})
		}
	}
	obj.Object["spec"] = map[string]interface{}{"sources": sources}
	return upsertUnstructured(ctx, cl, obj)
}

func upsertKpackBuilder(ctx context.Context, cl client.Client, spec KpackBuildEnvironmentSpec) error {
	obj := kpackObject("Builder", spec.Namespace, KpackBuilderName)
	order := make([]interface{}, 0, len(spec.BuildpackSources))
	for _, source := range spec.BuildpackSources {
		id := kpackBuildpackIDFromSource(source)
		if id == "" {
			continue
		}
		order = append(order, map[string]interface{}{
			"group": []interface{}{
				map[string]interface{}{"id": id},
			},
		})
	}
	if len(order) == 0 {
		for _, source := range []string{PaketoJavaBuildpackImage, PaketoNodeJSBuildpackImage, PaketoGoBuildpackImage, PaketoPythonBuildpackImage} {
			order = append(order, map[string]interface{}{
				"group": []interface{}{
					map[string]interface{}{"id": kpackBuildpackIDFromSource(source)},
				},
			})
		}
	}
	obj.Object["spec"] = map[string]interface{}{
		"tag":                spec.BuilderImage,
		"serviceAccountName": KpackBuildServiceAccountName,
		"stack": map[string]interface{}{
			"name": KpackClusterStackName,
			"kind": "ClusterStack",
		},
		"lifecycle": map[string]interface{}{
			"name": KpackClusterLifecycleName,
			"kind": "ClusterLifecycle",
		},
		"store": map[string]interface{}{
			"name": KpackClusterStoreName,
			"kind": "ClusterStore",
		},
		"order": order,
	}
	return upsertUnstructured(ctx, cl, obj)
}

func kpackBuildpackIDFromSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	repo := strings.Split(source, "@")[0]
	repo = strings.Split(repo, ":")[0]
	repo = strings.Trim(repo, "/")
	segments := strings.Split(repo, "/")
	if len(segments) < 2 {
		return ""
	}
	owner := segments[len(segments)-2]
	name := segments[len(segments)-1]
	if owner == "paketobuildpacks" {
		return "paketo-buildpacks/" + name
	}
	if owner == "paketo-buildpacks" {
		return owner + "/" + name
	}
	return ""
}

func kpackObject(kind, namespace, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "kpack.io", Version: "v1alpha2", Kind: kind})
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.SetLabels(kpackBootstrapLabels())
	return obj
}

func upsertUnstructured(ctx context.Context, cl client.Client, desired *unstructured.Unstructured) error {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	err := cl.Get(ctx, types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}, existing)
	if apierrors.IsNotFound(err) {
		return cl.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	existing.SetLabels(mergeLabels(existing.GetLabels(), desired.GetLabels()))
	existing.Object["spec"] = desired.Object["spec"]
	return cl.Update(ctx, existing)
}

func kpackBootstrapLabels() map[string]string {
	return map[string]string{
		"paap.io/managed-by": "paap-server",
		"paap.io/purpose":    "kpack-bootstrap",
	}
}

func mergeLabels(current, desired map[string]string) map[string]string {
	if current == nil {
		current = map[string]string{}
	}
	for k, v := range desired {
		current[k] = v
	}
	return current
}
