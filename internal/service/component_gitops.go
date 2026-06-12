package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	paapv1 "paap/api/v1"
	"paap/internal/k8s"
	"paap/internal/model"
)

const (
	giteaAdminUser           = "paap"
	giteaAdminPassword       = "paap123456"
	jenkinsKubectlImage      = "docker.io/alpine/k8s:1.34.1"
	jenkinsInboundAgentImage = "docker.io/jenkins/inbound-agent:3107.v665000b_51092-15"
)

var dnsLabelInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)

type componentJenkinsClient interface {
	Base() string
	EnsurePipelineJob(ctx context.Context, spec k8s.JenkinsPipelineJobSpec) error
	BuildJob(ctx context.Context, jobName string) error
}

var newComponentJenkinsClient = func(namespace string) componentJenkinsClient {
	return k8s.NewJenkinsClient(namespace)
}

type ComponentGitOpsResult struct {
	RepositoryURL       string
	SourceMirrorURL     string
	RepositoryPath      string
	ArgoCDApplication   string
	ArgoCDNamespace     string
	DeploymentNamespace string
	CIStatus            string
	CIWarning           string
}

type componentToolNamespaces struct {
	ArgoCD  string
	Gitea   string
	Jenkins string
}

type componentToolNamespaceMatch struct {
	name        string
	tool        string
	serviceType string
}

func ComponentIdentifier(name, componentType string, id uint) string {
	candidate := strings.ToLower(strings.TrimSpace(name))
	candidate = strings.ReplaceAll(candidate, "_", "-")
	candidate = dnsLabelInvalidChars.ReplaceAllString(candidate, "-")
	candidate = strings.Trim(candidate, "-")
	if candidate == "" || candidate[0] < 'a' || candidate[0] > 'z' {
		candidate = fmt.Sprintf("%s-%d", componentType, id)
	}
	if len(candidate) > 50 {
		candidate = strings.Trim(candidate[:50], "-")
	}
	if candidate == "" {
		return fmt.Sprintf("component-%d", id)
	}
	return candidate
}

func BuildComponentManifest(app model.Application, env model.Environment, comp model.Component, identifier, namespace string) string {
	return BuildComponentDeploymentManifest(app, env, comp, identifier, namespace) + "---\n" + BuildComponentServiceManifest(app, env, comp, identifier, namespace)
}

func BuildComponentDeploymentManifest(app model.Application, env model.Environment, comp model.Component, identifier, namespace string) string {
	replicas := comp.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	if componentUsesSourcePlaceholderImage(comp) {
		replicas = 0
	}
	image := comp.Image
	tag := comp.Version
	if tag == "" {
		tag = imageTag(image)
	}
	if !strings.Contains(lastImageSegment(image), ":") {
		image = image + ":" + tag
	}

	envVars, err := model.ComponentEnvVars(comp.Config)
	if err != nil {
		envVars = nil
	}
	cfg, err := model.ParseComponentConfig(comp.Config)
	if err != nil {
		cfg = model.ComponentConfig{}
	}
	labels := map[string]string{
		"app":                identifier,
		"paap.io/app":        app.Identifier,
		"paap.io/env":        env.Identifier,
		"paap.io/component":  identifier,
		"paap.io/managed-by": "argocd",
	}
	replicaCount := int32(replicas)
	deploy := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      identifier,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicaCount,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": identifier}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:    identifier,
						Image:   image,
						Command: cfg.Command,
						Args:    cfg.Args,
						Ports:   []corev1.ContainerPort{{ContainerPort: componentDefaultContainerPort(comp.Type)}},
						Resources: corev1.ResourceRequirements{
							Requests: componentResourceList(comp),
							Limits:   componentResourceList(comp),
						},
						Env: paapEnvVarsToCore(envVars),
					}},
				},
			},
		},
	}
	deployYAML, _ := yaml.Marshal(deploy)
	return string(deployYAML)
}

func BuildComponentServiceManifest(app model.Application, env model.Environment, comp model.Component, identifier, namespace string) string {
	labels := map[string]string{
		"app":                identifier,
		"paap.io/app":        app.Identifier,
		"paap.io/env":        env.Identifier,
		"paap.io/component":  identifier,
		"paap.io/managed-by": "argocd",
	}
	service := corev1.Service{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      identifier,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     componentDefaultServiceType(comp.Type),
			Selector: map[string]string{"app": identifier},
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       80,
				TargetPort: intstrFromInt(int(componentDefaultContainerPort(comp.Type))),
			}},
		},
	}
	serviceYAML, _ := yaml.Marshal(service)
	return string(serviceYAML)
}

func componentDefaultContainerPort(componentType string) int32 {
	if strings.EqualFold(strings.TrimSpace(componentType), "frontend") {
		return 80
	}
	return 8080
}

func componentDefaultServiceType(componentType string) corev1.ServiceType {
	if strings.EqualFold(strings.TrimSpace(componentType), "frontend") {
		return corev1.ServiceTypeNodePort
	}
	return corev1.ServiceTypeClusterIP
}

func componentResourceList(comp model.Component) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:    resourceQuantity(defaultResource(comp.CPU, "100m")),
		corev1.ResourceMemory: resourceQuantity(defaultResource(comp.Memory, "128Mi")),
	}
}

func resourceQuantity(value string) resource.Quantity {
	q, err := resource.ParseQuantity(value)
	if err != nil {
		return resource.MustParse("100m")
	}
	return q
}

func paapEnvVarsToCore(items []paapv1.EnvVar) []corev1.EnvVar {
	if len(items) == 0 {
		return nil
	}
	out := make([]corev1.EnvVar, 0, len(items))
	for _, item := range items {
		env := corev1.EnvVar{Name: item.Name, Value: item.Value}
		if item.ValueFrom != nil && item.ValueFrom.SecretKeyRef != nil {
			env.Value = ""
			env.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: item.ValueFrom.SecretKeyRef.Name},
					Key:                  item.ValueFrom.SecretKeyRef.Key,
				},
			}
		}
		if item.ValueFrom != nil && item.ValueFrom.ConfigMapKeyRef != nil {
			env.Value = ""
			env.ValueFrom = &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: item.ValueFrom.ConfigMapKeyRef.Name},
					Key:                  item.ValueFrom.ConfigMapKeyRef.Key,
				},
			}
		}
		out = append(out, env)
	}
	return out
}

func intstrFromInt(value int) intstr.IntOrString {
	return intstr.FromInt(value)
}

func componentUsesSourcePlaceholderImage(comp model.Component) bool {
	if comp.DeliveryMode != "source" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(comp.PipelineStatus), "built") {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(comp.Version), "manual") || strings.HasSuffix(strings.ToLower(strings.TrimSpace(comp.Image)), ":manual")
}

func lastImageSegment(image string) string {
	parts := strings.Split(image, "/")
	return parts[len(parts)-1]
}

func imageTag(image string) string {
	last := lastImageSegment(image)
	colon := strings.LastIndex(last, ":")
	if colon < 0 || colon == len(last)-1 {
		return ""
	}
	return last[colon+1:]
}

func ImageWithoutTag(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	colon := strings.LastIndex(last, ":")
	if colon < 0 {
		return image
	}
	parts[len(parts)-1] = last[:colon]
	return strings.Join(parts, "/")
}

func ImageWithTag(image, tag string) string {
	image = strings.TrimSpace(image)
	tag = strings.TrimSpace(tag)
	if image == "" || tag == "" {
		return image
	}
	return ImageWithoutTag(image) + ":" + tag
}

func defaultResource(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "核") || strings.Contains(value, "GB") || strings.Contains(value, "MB") {
		return fallback
	}
	return value
}

func EnsureComponentGitOps(ctx context.Context, k8sClient client.Client, app model.Application, env model.Environment, comp model.Component, identifier, namespace string) (ComponentGitOpsResult, error) {
	toolNamespaces := discoverComponentToolNamespaces(ctx, k8sClient, app.Identifier, env.Identifier)
	giteaNS := toolNamespaces.Gitea
	argocdNS := toolNamespaces.ArgoCD
	repoName := fmt.Sprintf("%s-%s-components", app.Identifier, env.Identifier)
	repoPath := fmt.Sprintf("components/%s", identifier)
	manifestPath := repoPath + "/deployment.yaml"
	baseURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:3000", giteaNS, giteaNS)
	repoURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:3000/%s/%s.git", giteaNS, giteaNS, giteaAdminUser, repoName)
	sourceMirrorName := fmt.Sprintf("%s-%s-%s-source", app.Identifier, env.Identifier, identifier)

	if comp.Version == "" && imageTag(comp.Image) == "" {
		return ComponentGitOpsResult{}, fmt.Errorf("component version is required when image tag is missing")
	}

	manifest := BuildComponentDeploymentManifest(app, env, comp, identifier, namespace)
	serviceManifest := BuildComponentServiceManifest(app, env, comp, identifier, namespace)
	if err := ensureGiteaRepository(ctx, baseURL, repoName); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if comp.DeliveryMode == "source" {
		sourceMirrorURL, err := ensureGiteaSourceMirror(ctx, baseURL, sourceMirrorName, comp)
		if err != nil {
			return ComponentGitOpsResult{}, err
		}
		comp.SourceMirrorRepoURL = sourceMirrorURL
	}
	if err := ensureArgoCDRepositorySecret(ctx, k8sClient, argocdNS, repoName, repoURL, giteaAdminUser, giteaAdminPassword); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if err := putGiteaFile(ctx, baseURL, repoName, manifestPath, manifest, fmt.Sprintf("deploy %s", identifier)); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if err := putGiteaFile(ctx, baseURL, repoName, repoPath+"/service.yaml", serviceManifest, fmt.Sprintf("sync service for %s", identifier)); err != nil {
		return ComponentGitOpsResult{}, err
	}
	kpackSpec, caWarning := buildComponentKpackSpecWithRegistryCA(ctx, k8sClient, app, env, identifier, comp.Image)
	hasRegistryCABinding := len(kpackSpec.RegistryCAPEM) > 0

	// Jenkinsfile is PAAP-managed and must be refreshed when registry trust or build inputs change.
	jenkinsfile := buildComponentJenkinsfile(comp, identifier, repoURL, hasRegistryCABinding)
	if err := putGiteaFile(ctx, baseURL, repoName, repoPath+"/Jenkinsfile", jenkinsfile, fmt.Sprintf("sync jenkinsfile for %s", identifier)); err != nil {
		return ComponentGitOpsResult{}, err
	}
	// README is scaffold-only so user edits are not overwritten.
	readme := buildComponentReadme(app, env, comp, identifier, repoURL, repoPath)
	if err := putGiteaFileIfNotExists(ctx, baseURL, repoName, repoPath+"/README.md", readme, fmt.Sprintf("init readme for %s", identifier)); err != nil {
		return ComponentGitOpsResult{}, err
	}
	ciStatus, ciWarning := ensureComponentJenkinsAutomation(ctx, k8sClient, app, env, comp, identifier, baseURL, repoName, repoURL, kpackSpec, caWarning)
	projectName := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	destinationNamespaces := discoverComponentGitOpsNamespaces(ctx, k8sClient, app.Identifier, env.Identifier, namespace)
	if err := ensureArgoCDLocalClusterSecret(ctx, k8sClient, argocdNS, destinationNamespaces); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if err := ensureArgoCDDefaultProjectDenied(ctx, k8sClient, argocdNS); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if err := ensureArgoCDProject(ctx, k8sClient, argocdNS, projectName, repoURL, destinationNamespaces); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if err := ensureArgoCDApplication(ctx, k8sClient, argocdNS, fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, identifier), projectName, repoURL, repoPath, namespace, app.Identifier, env.Identifier, identifier); err != nil {
		return ComponentGitOpsResult{}, err
	}

	return ComponentGitOpsResult{
		RepositoryURL:       repoURL,
		SourceMirrorURL:     comp.SourceMirrorRepoURL,
		RepositoryPath:      repoPath,
		ArgoCDApplication:   fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, identifier),
		ArgoCDNamespace:     argocdNS,
		DeploymentNamespace: namespace,
		CIStatus:            ciStatus,
		CIWarning:           ciWarning,
	}, nil
}

func ensureGiteaRepository(ctx context.Context, baseURL, repoName string) error {
	body := map[string]interface{}{
		"name":           repoName,
		"private":        false,
		"auto_init":      true,
		"default_branch": "main",
	}
	status, response, err := giteaRequest(ctx, http.MethodPost, baseURL+"/api/v1/user/repos", body)
	if err != nil {
		return err
	}
	if status == http.StatusCreated || status == http.StatusConflict || status == http.StatusUnprocessableEntity {
		return nil
	}
	return fmt.Errorf("create gitea repo failed: status=%d body=%s", status, string(response))
}

func ensureGiteaSourceMirror(ctx context.Context, baseURL, repoName string, comp model.Component) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	sourceURL := strings.TrimSpace(comp.SourceRepoURL)
	if sourceURL == "" {
		if err := ensureGiteaRepository(ctx, baseURL, repoName); err != nil {
			return "", err
		}
		return giteaRepoURL(baseURL, repoName), nil
	}
	if giteaRepoURLBelongsToBase(sourceURL, baseURL) {
		return sourceURL, nil
	}

	body := map[string]interface{}{
		"clone_addr":     sourceURL,
		"repo_name":      repoName,
		"repo_owner":     giteaAdminUser,
		"mirror":         true,
		"private":        false,
		"description":    fmt.Sprintf("PAAP source mirror for %s", comp.Name),
		"default_branch": componentBranch(comp),
	}
	status, response, err := giteaRequest(ctx, http.MethodPost, baseURL+"/api/v1/repos/migrate", body)
	if err != nil {
		return "", err
	}
	if status == http.StatusCreated || status == http.StatusOK || status == http.StatusConflict {
		return giteaRepoURL(baseURL, repoName), nil
	}
	return "", fmt.Errorf("migrate gitea source repo failed: status=%d body=%s", status, string(response))
}

func giteaRepoURLBelongsToBase(repoURL, baseURL string) bool {
	repoURL = strings.TrimSpace(repoURL)
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if repoURL == "" || baseURL == "" {
		return false
	}
	repo, err := url.Parse(repoURL)
	if err != nil {
		return false
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(repo.Scheme, base.Scheme) && strings.EqualFold(repo.Host, base.Host)
}

func giteaRepoURL(baseURL, repoName string) string {
	return strings.TrimRight(baseURL, "/") + "/" + giteaAdminUser + "/" + repoName + ".git"
}

func putGiteaFile(ctx context.Context, baseURL, repoName, path, content, message string) error {
	sha := ""
	status, response, err := giteaRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s", baseURL, giteaAdminUser, repoName, path), nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		var existing struct {
			SHA     string `json:"sha"`
			Content string `json:"content"`
		}
		_ = json.Unmarshal(response, &existing)
		sha = existing.SHA
		encoded := strings.ReplaceAll(strings.TrimSpace(existing.Content), "\n", "")
		if encoded != "" {
			if decoded, decodeErr := base64.StdEncoding.DecodeString(encoded); decodeErr == nil && string(decoded) == content {
				return nil
			}
		}
	}

	body := map[string]interface{}{
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"message": message,
		"branch":  "main",
	}
	if sha != "" {
		body["sha"] = sha
	}
	method := http.MethodPost
	if sha != "" {
		method = http.MethodPut
	}
	status, response, err = giteaRequest(ctx, method, fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s", baseURL, giteaAdminUser, repoName, path), body)
	if err != nil {
		return err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		return nil
	}
	return fmt.Errorf("write gitea file failed: status=%d body=%s", status, string(response))
}

func putGiteaFileIfNotExists(ctx context.Context, baseURL, repoName, path, content, message string) error {
	status, _, err := giteaRequest(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/repos/%s/%s/contents/%s", baseURL, giteaAdminUser, repoName, path), nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		return nil // file already exists, do not overwrite
	}
	return putGiteaFile(ctx, baseURL, repoName, path, content, message)
}

func ensureComponentJenkinsAutomation(ctx context.Context, k8sClient client.Client, app model.Application, env model.Environment, comp model.Component, identifier, giteaBaseURL, repoName, repoURL string, kpackSpec k8s.KpackBuildEnvironmentSpec, caWarning string) (string, string) {
	if !componentNeedsJenkins(comp) {
		return "", ""
	}
	kpackStatus := k8s.EnsureKpackBuildEnvironment(ctx, k8sClient, kpackSpec)
	ciStatus, ciWarning := componentCIStatusFromKpack(kpackStatus)
	ciWarning = appendCIWarning(ciWarning, caWarning)

	jenkinsNS := discoverComponentToolNamespaces(ctx, k8sClient, app.Identifier, env.Identifier).Jenkins
	jenkins := newComponentJenkinsClient(jenkinsNS)
	jobSpec := buildComponentJenkinsJobSpec(app, env, identifier, repoURL)
	if comp.JenkinsJob != "" {
		jobSpec.Name = comp.JenkinsJob
	}
	if jobSpec.BuildToken == "" {
		jobSpec.BuildToken = "paap-source-build"
	}
	if err := jenkins.EnsurePipelineJob(ctx, jobSpec); err != nil {
		return "pending", appendCIWarning(ciWarning, fmt.Sprintf("Jenkins job sync failed: %v", err))
	}
	hookURL := jenkinsNotifyCommitHookURL(jenkins.Base(), repoURL)
	if err := ensureGiteaPushWebhook(ctx, giteaBaseURL, repoName, hookURL); err != nil {
		return "pending", appendCIWarning(ciWarning, fmt.Sprintf("Gitea webhook sync failed: %v", err))
	}
	if sourceRepoName := sourceWebhookRepositoryName(giteaBaseURL, comp); sourceRepoName != "" {
		sourceHookURL := jenkinsRemoteBuildHookURL(jenkins.Base(), jobSpec.Name)
		if err := ensureGiteaPushWebhook(ctx, giteaBaseURL, sourceRepoName, sourceHookURL); err != nil {
			return "pending", appendCIWarning(ciWarning, fmt.Sprintf("Gitea source webhook sync failed: %v", err))
		}
	}
	if ciStatus == "pending" {
		return ciStatus, ciWarning
	}
	if componentShouldTriggerInitialBuild(comp) {
		if err := jenkins.BuildJob(ctx, jobSpec.Name); err != nil {
			return "pending", appendCIWarning(ciWarning, fmt.Sprintf("Jenkins initial build trigger failed: %v", err))
		}
		return "running", ""
	}
	return "configured", ""
}

func componentNeedsJenkins(comp model.Component) bool {
	return comp.DeliveryMode == "source" || comp.SourceRepoURL != "" || comp.JenkinsJob != ""
}

func componentShouldTriggerInitialBuild(comp model.Component) bool {
	switch strings.ToLower(strings.TrimSpace(comp.PipelineStatus)) {
	case "", "planned", "pending":
		return true
	default:
		return false
	}
}

func buildComponentJenkinsJobSpec(app model.Application, env model.Environment, identifier, repoURL string) k8s.JenkinsPipelineJobSpec {
	return k8s.JenkinsPipelineJobSpec{
		Name:       fmt.Sprintf("%s-%s-%s-build", app.Identifier, env.Identifier, identifier),
		RepoURL:    repoURL,
		Branch:     "main",
		ScriptPath: fmt.Sprintf("components/%s/Jenkinsfile", identifier),
	}
}

func buildComponentKpackSpec(app model.Application, env model.Environment, identifier string) k8s.KpackBuildEnvironmentSpec {
	spec, _ := buildComponentKpackSpecWithRegistryCA(context.Background(), nil, app, env, identifier)
	return spec
}

func buildComponentKpackSpecWithRegistryCA(ctx context.Context, k8sClient client.Client, app model.Application, env model.Environment, identifier string, imageRefs ...string) (k8s.KpackBuildEnvironmentSpec, string) {
	primaryNS := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	toolNamespaces := discoverComponentToolNamespaces(ctx, k8sClient, app.Identifier, env.Identifier)
	registryHost := RuntimeRegistryHost(app, env, "registry")
	if len(imageRefs) > 0 {
		if host := registryServerFromImageRef(imageRefs[0]); host != "" {
			registryHost = host
		}
	}
	spec := k8s.KpackBuildEnvironmentSpec{
		Namespace:       toolNamespaces.Jenkins,
		RegistryServer:  registryHost,
		GitServer:       fmt.Sprintf("http://%s.%s.svc.cluster.local:3000", toolNamespaces.Gitea, toolNamespaces.Gitea),
		GitUsername:     giteaAdminUser,
		GitPassword:     giteaAdminPassword,
		BuilderImage:    fmt.Sprintf("%s/%s/paap-builder:latest", registryHost, primaryNS),
		StackBuildImage: k8s.PaketoBuildJammyBaseImage,
		StackRunImage:   k8s.PaketoRunJammyBaseImage,
		BuildpackSources: []string{
			k8s.PaketoJavaBuildpackImage,
			k8s.PaketoNodeJSBuildpackImage,
			k8s.PaketoGoBuildpackImage,
			k8s.PaketoPythonBuildpackImage,
		},
	}
	if registryServiceTypeForHost(app, env, registryHost) == "harbor" {
		spec.RegistryUsername = "admin"
		spec.RegistryPassword = "Harbor12345"
	}
	if k8sClient == nil {
		return spec, ""
	}
	ca, warning := readEnvironmentRegistryCAWithClient(ctx, k8sClient, app, env, registryHost)
	spec.RegistryCAPEM = ca
	if len(ca) > 0 {
		spec.StackBuildImage = fmt.Sprintf("%s/%s/paap-build-jammy-base:registry-ca", registryHost, primaryNS)
		spec.StackRunImage = fmt.Sprintf("%s/%s/paap-run-jammy-base:registry-ca", registryHost, primaryNS)
	}
	return spec, warning
}

func registryServerFromImageRef(image string) string {
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

func readEnvironmentRegistryCA(ctx context.Context, app model.Application, env model.Environment) ([]byte, string) {
	for _, serviceType := range []string{"registry", "harbor"} {
		namespace := fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, serviceType)
		releaseName := namespace
		cert, _, err := k8s.ReadRegistryCACertificate(ctx, namespace, serviceType, releaseName)
		if err == nil && len(cert) > 0 {
			return cert, ""
		}
	}
	return nil, "registry CA was not found; install registry/Harbor first or check its TLS secret so PAAP can sync trust for kpack controller and build pods"
}

func readEnvironmentRegistryCAWithClient(ctx context.Context, k8sClient client.Client, app model.Application, env model.Environment, preferredHosts ...string) ([]byte, string) {
	if k8sClient == nil {
		return nil, ""
	}
	serviceTypes := []string{"registry", "harbor"}
	if len(preferredHosts) > 0 {
		if preferred := registryServiceTypeForHost(app, env, preferredHosts[0]); preferred != "" {
			serviceTypes = append([]string{preferred}, removeString(serviceTypes, preferred)...)
		}
	}
	for _, serviceType := range serviceTypes {
		namespace := fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, serviceType)
		releaseName := namespace
		cert, _, err := k8s.ReadRegistryCACertificateWithClient(ctx, k8sClient, namespace, serviceType, releaseName)
		if err == nil && len(cert) > 0 {
			return cert, ""
		}
	}
	return nil, "registry CA was not found; install registry/Harbor first or check its TLS secret so PAAP can sync trust for kpack controller and build pods"
}

func registryServiceTypeForHost(app model.Application, env model.Environment, host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	for _, serviceType := range []string{"harbor", "registry"} {
		if RuntimeRegistryHost(app, env, serviceType) == host {
			return serviceType
		}
	}
	return ""
}

func removeString(values []string, value string) []string {
	result := make([]string, 0, len(values))
	for _, item := range values {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}

func componentCIStatusFromKpack(status k8s.KpackBuildEnvironmentStatus) (string, string) {
	warning := appendCIWarning(status.Warning, status.RegistryWarning)
	if warning != "" {
		return "pending", warning
	}
	if status.Ready {
		return "configured", ""
	}
	return "pending", "kpack build environment is not ready"
}

func appendCIWarning(current, next string) string {
	current = strings.TrimSpace(current)
	next = strings.TrimSpace(next)
	if current == "" {
		return next
	}
	if next == "" {
		return current
	}
	return current + "; " + next
}

func jenkinsNotifyCommitHookURL(baseURL, repoURL string) string {
	return strings.TrimRight(baseURL, "/") + "/git/notifyCommit?url=" + url.QueryEscape(repoURL)
}

func jenkinsRemoteBuildHookURL(baseURL, jobName string) string {
	return strings.TrimRight(baseURL, "/") + k8s.JenkinsJobPath(jobName) + "/build?token=paap-source-build"
}

func sourceWebhookRepositoryName(giteaBaseURL string, comp model.Component) string {
	if comp.DeliveryMode != "source" {
		return ""
	}
	repoURL := strings.TrimSpace(comp.SourceMirrorRepoURL)
	if repoURL == "" {
		repoURL = strings.TrimSpace(comp.SourceRepoURL)
	}
	return giteaRepositoryNameFromURL(giteaBaseURL, repoURL)
}

func giteaRepositoryNameFromURL(baseURL, repoURL string) string {
	if !giteaRepoURLBelongsToBase(repoURL, baseURL) {
		return ""
	}
	parsed, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	name := strings.TrimSuffix(parts[len(parts)-1], ".git")
	return strings.TrimSpace(name)
}

func ensureGiteaPushWebhook(ctx context.Context, baseURL, repoName, hookURL string) error {
	endpoint := fmt.Sprintf("%s/api/v1/repos/%s/%s/hooks", strings.TrimRight(baseURL, "/"), giteaAdminUser, repoName)
	status, response, err := giteaRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if status == http.StatusOK {
		var hooks []struct {
			ID     int               `json:"id"`
			Config map[string]string `json:"config"`
		}
		if err := json.Unmarshal(response, &hooks); err == nil {
			for _, hook := range hooks {
				if hook.Config["url"] == hookURL {
					if err := deleteLegacyPAAPSourceWebhooks(ctx, endpoint, hooks); err != nil {
						return err
					}
					return nil
				}
			}
			if err := deleteLegacyPAAPSourceWebhooks(ctx, endpoint, hooks); err != nil {
				return err
			}
		}
	} else if status != http.StatusNotFound {
		return fmt.Errorf("list gitea hooks failed: status=%d body=%s", status, string(response))
	}

	body := map[string]interface{}{
		"type":   "gitea",
		"active": true,
		"events": []string{"push"},
		"config": map[string]string{
			"url":          hookURL,
			"content_type": "json",
		},
	}
	status, response, err = giteaRequest(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		return nil
	}
	return fmt.Errorf("create gitea hook failed: status=%d body=%s", status, string(response))
}

func deleteLegacyPAAPSourceWebhooks(ctx context.Context, endpoint string, hooks []struct {
	ID     int               `json:"id"`
	Config map[string]string `json:"config"`
}) error {
	for _, hook := range hooks {
		if hook.ID <= 0 || !isLegacyPAAPSourceWebhookURL(hook.Config["url"]) {
			continue
		}
		status, response, err := giteaRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/%d", endpoint, hook.ID), nil)
		if err != nil {
			return err
		}
		if status != http.StatusOK && status != http.StatusNoContent && status != http.StatusNotFound {
			return fmt.Errorf("delete legacy gitea source hook failed: status=%d body=%s", status, string(response))
		}
	}
	return nil
}

func isLegacyPAAPSourceWebhookURL(hookURL string) bool {
	hookURL = strings.ToLower(strings.TrimSpace(hookURL))
	return strings.Contains(hookURL, "paap-server") && strings.Contains(hookURL, "/source-webhook")
}

func buildComponentJenkinsfile(comp model.Component, identifier, fallbackRepoURL string, includeRegistryCABinding bool) string {
	image := ImageWithoutTag(comp.Image)
	if image == "" {
		image = fmt.Sprintf("registry.local/%s", identifier)
	}
	buildCtx := comp.BuildContext
	if buildCtx == "" {
		buildCtx = "."
	}
	sourceSubPath := ""
	if buildCtx != "." {
		sourceSubPath = "    subPath: ${BUILD_CONTEXT}\n"
	}
	sourceRepo := strings.TrimSpace(comp.SourceMirrorRepoURL)
	if sourceRepo == "" {
		sourceRepo = strings.TrimSpace(comp.SourceRepoURL)
	}
	if sourceRepo == "" {
		sourceRepo = strings.TrimSpace(comp.GitRepoURL)
	}
	if sourceRepo == "" {
		sourceRepo = fallbackRepoURL
	}
	gitopsPushURL := authenticatedGiteaURL(fallbackRepoURL)
	sourceBranch := strings.TrimSpace(comp.SourceBranch)
	if sourceBranch == "" {
		sourceBranch = "main"
	}
	gitopsPath := fmt.Sprintf("components/%s/deployment.yaml", identifier)
	kpackAPIVersion := "kpack.io/v1alpha2"
	kpackServiceAccountField := "serviceAccountName"
	if includeRegistryCABinding {
		kpackAPIVersion = "kpack.io/v1alpha1"
		kpackServiceAccountField = "serviceAccount"
	}
	buildEnv := `  build:
    env:
    - name: BP_GO_VERSION
      value: "1.25.*"
`
	if includeRegistryCABinding {
		buildEnv = `  build:
    env:
    - name: BP_GO_VERSION
      value: "1.25.*"
    cnbBindings:
    - name: paap-registry-ca
      secretRef:
        name: paap-kpack-registry-ca
`
	}
	return fmt.Sprintf(`podTemplate(defaultContainer: 'kubectl', yaml: '''
apiVersion: v1
kind: Pod
spec:
  serviceAccountName: paap-kpack-build
  containers:
  - name: kubectl
    image: %s
    imagePullPolicy: IfNotPresent
    command:
    - cat
    tty: true
  - name: jnlp
    image: %s
    imagePullPolicy: IfNotPresent
''') {
    node(POD_LABEL) {
        container('kubectl') {
            def IMAGE_NAME = "%s"
            def BUILD_CONTEXT = "%s"
            def SOURCE_REPO = "%s"
            def SOURCE_REVISION = "%s"
            def KPACK_IMAGE = "%s"
            def KPACK_BUILDER = "paap-builder"
            def KPACK_SERVICE_ACCOUNT = "paap-kpack-build"
            def GITOPS_DEPLOYMENT = "%s"
            def GITOPS_BRANCH = "main"
            def GITOPS_REPO_PUSH_URL = "%s"

            try {
        stage('Submit Buildpacks Image') {
                    def tag = env.BUILD_NUMBER ?: "%s"
                    writeFile file: 'kpack-image.yaml', text: """
apiVersion: %s
kind: Image
metadata:
  name: ${KPACK_IMAGE}
spec:
  tag: ${IMAGE_NAME}:${tag}
  %s: ${KPACK_SERVICE_ACCOUNT}
  builder:
    kind: Builder
    name: ${KPACK_BUILDER}
%s
  source:
%s    git:
      url: ${SOURCE_REPO}
      revision: ${SOURCE_REVISION}
"""
                    sh "kubectl apply -f kpack-image.yaml"
                    sh "kubectl wait image.kpack.io/${KPACK_IMAGE} --for=condition=Ready=True --timeout=30m"
        }
        stage('Update GitOps Manifest') {
                    def tag = env.BUILD_NUMBER ?: "%s"
                    sh """
git config user.email "paap@local"
git config user.name "PAAP CI"
sed -i -E "s#image: ${IMAGE_NAME}(:[^[:space:]]+)?#image: ${IMAGE_NAME}:${tag}#" "${GITOPS_DEPLOYMENT}"
git remote set-url origin ${GITOPS_REPO_PUSH_URL}
git add ${GITOPS_DEPLOYMENT}
git diff --cached --quiet && exit 0
git commit -m "build %s:${tag}"
git push origin HEAD:${GITOPS_BRANCH}
"""
        }
            } finally {
                deleteDir()
            }
        }
    }
}`, jenkinsKubectlImage, jenkinsInboundAgentImage, image, buildCtx, sourceRepo, sourceBranch, identifier, gitopsPath, gitopsPushURL, valueOrDefault(comp.Version, "manual"), kpackAPIVersion, kpackServiceAccountField, buildEnv, sourceSubPath, valueOrDefault(comp.Version, "manual"), identifier)
}

func authenticatedGiteaURL(repoURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return repoURL
	}
	parsed.User = url.UserPassword(giteaAdminUser, giteaAdminPassword)
	return parsed.String()
}

func buildComponentReadme(app model.Application, env model.Environment, comp model.Component, identifier, repoURL, repoPath string) string {
	return fmt.Sprintf(`# %s

> PAAP managed component: **%s**

## Overview

| Field      | Value |
|------------|-------|
| App        | %s    |
| Env        | %s    |
| Component  | %s    |
| Image      | %s    |
| Replicas   | %d    |

## GitOps

- Repository: %s
- Path: %s
- ArgoCD Application: %s-%s-%s

## CI/CD

PAAP manages the Jenkins Pipeline Job and Gitea push webhook for this component. Push code to the source repository to trigger the Jenkins job. The default build path is Buildpacks/kpack, so the Jenkins agent does not need Docker and the source repository does not need a Dockerfile.

## Deployment

The deployment manifest is managed by ArgoCD. Edit %s/deployment.yaml to customize.
`,
		identifier,
		comp.Name,
		app.Identifier,
		env.Identifier,
		identifier,
		comp.Image,
		comp.Replicas,
		repoURL,
		repoPath,
		app.Identifier, env.Identifier, identifier,
		repoPath,
	)
}

func giteaRequest(ctx context.Context, method, url string, body interface{}) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, nil, err
	}
	req.SetBasicAuth(giteaAdminUser, giteaAdminPassword)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	return res.StatusCode, data, nil
}

func ensureArgoCDRepositorySecret(ctx context.Context, k8sClient client.Client, namespace, repoName, repoURL, username, password string) error {
	secretName := "paap-repo-" + repoName
	labels := map[string]string{
		"argocd.argoproj.io/secret-type": "repository",
		"paap.io/managed-by":             "paap-server",
	}

	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret)
	if errors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Labels:    labels,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"type":               []byte("git"),
				"url":                []byte(repoURL),
				"username":           []byte(username),
				"password":           []byte(password),
				"forceHttpBasicAuth": []byte("true"),
				"insecure":           []byte("true"),
			},
		}
		return k8sClient.Create(ctx, secret)
	}
	if err != nil {
		return err
	}

	secret.Labels = mergeStringMap(secret.Labels, labels)
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data["type"] = []byte("git")
	secret.Data["url"] = []byte(repoURL)
	secret.Data["username"] = []byte(username)
	secret.Data["password"] = []byte(password)
	secret.Data["forceHttpBasicAuth"] = []byte("true")
	secret.Data["insecure"] = []byte("true")
	delete(secret.Data, "sshPrivateKey")
	return k8sClient.Update(ctx, secret)
}

func discoverComponentToolNamespaces(ctx context.Context, k8sClient client.Client, appIdentifier, envIdentifier string) componentToolNamespaces {
	base := fmt.Sprintf("%s-%s", appIdentifier, envIdentifier)
	result := componentToolNamespaces{
		ArgoCD:  base + "-argocd",
		Gitea:   base + "-gitea",
		Jenkins: base + "-jenkins",
	}
	if k8sClient == nil || appIdentifier == "" || envIdentifier == "" {
		return result
	}

	list := &corev1.NamespaceList{}
	if err := k8sClient.List(ctx, list, client.MatchingLabels{"paap.io/app": appIdentifier, "paap.io/env": envIdentifier}); err != nil {
		return result
	}

	matches := make([]componentToolNamespaceMatch, 0, len(list.Items))
	for _, item := range list.Items {
		tool := firstStringValue(item.Labels, item.Annotations, "paap.io/tool")
		serviceType := firstStringValue(item.Labels, item.Annotations, "paap.io/service-type")
		if serviceType == "" {
			serviceType = firstStringValue(item.Labels, item.Annotations, "paap.io/service")
		}
		matches = append(matches, componentToolNamespaceMatch{name: item.Name, tool: tool, serviceType: serviceType})
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].name < matches[j].name })

	if namespace := namespaceByTool(matches, "argocd"); namespace != "" {
		result.ArgoCD = namespace
	} else if namespace := namespaceByServiceType(matches, "deploy"); namespace != "" {
		result.ArgoCD = namespace
	}
	if namespace := namespaceByTool(matches, "gitea"); namespace != "" {
		result.Gitea = namespace
	} else if namespace := namespaceByServiceType(matches, "git"); namespace != "" {
		result.Gitea = namespace
	}
	if namespace := namespaceByTool(matches, "jenkins"); namespace != "" {
		result.Jenkins = namespace
	} else if namespace := namespaceByServiceType(matches, "ci"); namespace != "" {
		result.Jenkins = namespace
	}
	return result
}

func namespaceByTool(matches []componentToolNamespaceMatch, tool string) string {
	for _, item := range matches {
		if item.tool == tool {
			return item.name
		}
	}
	return ""
}

func namespaceByServiceType(matches []componentToolNamespaceMatch, serviceType string) string {
	for _, item := range matches {
		if item.serviceType == serviceType {
			return item.name
		}
	}
	return ""
}

func firstStringValue(labels, annotations map[string]string, key string) string {
	if strings.TrimSpace(labels[key]) != "" {
		return strings.TrimSpace(labels[key])
	}
	return strings.TrimSpace(annotations[key])
}

func discoverComponentGitOpsNamespaces(ctx context.Context, k8sClient client.Client, appIdentifier, envIdentifier, fallbackNamespace string) []string {
	namespaces := map[string]bool{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			namespaces[value] = true
		}
	}

	list := &corev1.NamespaceList{}
	if err := k8sClient.List(ctx, list, client.MatchingLabels{"paap.io/app": appIdentifier, "paap.io/env": envIdentifier}); err == nil {
		for _, item := range list.Items {
			if item.Labels["paap.io/role"] == "workload" {
				add(item.Name)
			}
		}
	}
	add(fallbackNamespace)
	if appIdentifier != "" && envIdentifier != "" {
		base := fmt.Sprintf("%s-%s", appIdentifier, envIdentifier)
		add(base)
		add(base + "-app")
	}

	result := make([]string, 0, len(namespaces))
	for namespace := range namespaces {
		result = append(result, namespace)
	}
	sort.Strings(result)
	return result
}

func ensureArgoCDLocalClusterSecret(ctx context.Context, k8sClient client.Client, namespace string, destinationNamespaces []string) error {
	if k8sClient == nil {
		return nil
	}
	namespaces := make([]string, 0, len(destinationNamespaces))
	seen := map[string]bool{}
	for _, item := range destinationNamespaces {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		namespaces = append(namespaces, item)
	}
	sort.Strings(namespaces)
	if len(namespaces) == 0 {
		return fmt.Errorf("argocd local cluster namespaces are required")
	}

	config := map[string]interface{}{
		"inCluster": true,
		"tlsClientConfig": map[string]interface{}{
			"insecure": false,
		},
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return err
	}

	secretName := "paap-local-cluster"
	labels := map[string]string{
		"argocd.argoproj.io/secret-type": "cluster",
		"paap.io/managed-by":             "paap-server",
	}
	data := map[string][]byte{
		"name":             []byte("in-cluster"),
		"server":           []byte("https://kubernetes.default.svc"),
		"namespaces":       []byte(strings.Join(namespaces, ",")),
		"clusterResources": []byte("false"),
		"config":           configJSON,
	}

	secret := &corev1.Secret{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: secretName, Namespace: namespace}, secret)
	if errors.IsNotFound(err) {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Labels:    labels,
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}
		return k8sClient.Create(ctx, secret)
	}
	if err != nil {
		return err
	}
	secret.Labels = mergeStringMap(secret.Labels, labels)
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	for key, value := range data {
		secret.Data[key] = value
	}
	return k8sClient.Update(ctx, secret)
}

func ensureArgoCDProject(ctx context.Context, k8sClient client.Client, namespace, name, repoURL string, destinationNamespaces []string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	key := types.NamespacedName{Name: name, Namespace: namespace}
	err := k8sClient.Get(ctx, key, obj)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	destinations := make([]interface{}, 0, len(destinationNamespaces))
	seen := map[string]bool{}
	for _, destinationNamespace := range destinationNamespaces {
		destinationNamespace = strings.TrimSpace(destinationNamespace)
		if destinationNamespace == "" || seen[destinationNamespace] {
			continue
		}
		seen[destinationNamespace] = true
		destinations = append(destinations, map[string]interface{}{
			"server":    "https://kubernetes.default.svc",
			"namespace": destinationNamespace,
		})
	}
	if len(destinations) == 0 {
		return fmt.Errorf("argocd project destination namespaces are required")
	}

	sourceRepos := []interface{}{repoURL}
	namespaceWhitelist := []interface{}{
		map[string]interface{}{"group": "", "kind": "ConfigMap"},
		map[string]interface{}{"group": "", "kind": "Endpoints"},
		map[string]interface{}{"group": "", "kind": "Event"},
		map[string]interface{}{"group": "", "kind": "PersistentVolumeClaim"},
		map[string]interface{}{"group": "", "kind": "Pod"},
		map[string]interface{}{"group": "", "kind": "Secret"},
		map[string]interface{}{"group": "", "kind": "Service"},
		map[string]interface{}{"group": "", "kind": "ServiceAccount"},
		map[string]interface{}{"group": "apps", "kind": "ControllerRevision"},
		map[string]interface{}{"group": "apps", "kind": "Deployment"},
		map[string]interface{}{"group": "apps", "kind": "ReplicaSet"},
		map[string]interface{}{"group": "apps", "kind": "StatefulSet"},
		map[string]interface{}{"group": "autoscaling", "kind": "HorizontalPodAutoscaler"},
		map[string]interface{}{"group": "batch", "kind": "CronJob"},
		map[string]interface{}{"group": "batch", "kind": "Job"},
		map[string]interface{}{"group": "discovery.k8s.io", "kind": "EndpointSlice"},
		map[string]interface{}{"group": "networking.k8s.io", "kind": "Ingress"},
	}
	spec := map[string]interface{}{
		"description":                "PAAP environment project",
		"sourceRepos":                sourceRepos,
		"destinations":               destinations,
		"clusterResourceWhitelist":   []interface{}{},
		"namespaceResourceWhitelist": namespaceWhitelist,
	}
	labels := map[string]string{
		"paap.io/managed-by": "paap-server",
	}

	if errors.IsNotFound(err) {
		obj = &unstructured.Unstructured{Object: map[string]interface{}{"spec": spec}}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
		obj.SetName(name)
		obj.SetNamespace(namespace)
		obj.SetLabels(labels)
		return k8sClient.Create(ctx, obj)
	}

	obj.SetLabels(mergeStringMap(obj.GetLabels(), labels))
	if err := unstructured.SetNestedField(obj.Object, spec, "spec"); err != nil {
		return err
	}
	obj.SetManagedFields(nil)
	return k8sClient.Update(ctx, obj)
}

func ensureArgoCDDefaultProjectDenied(ctx context.Context, k8sClient client.Client, namespace string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	key := types.NamespacedName{Name: "default", Namespace: namespace}
	err := k8sClient.Get(ctx, key, obj)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	spec := map[string]interface{}{
		"description":                "PAAP disables the default project; use the environment project instead.",
		"sourceRepos":                []interface{}{},
		"destinations":               []interface{}{},
		"clusterResourceWhitelist":   []interface{}{},
		"namespaceResourceWhitelist": []interface{}{},
	}
	labels := map[string]string{
		"paap.io/managed-by": "paap-server",
	}

	if errors.IsNotFound(err) {
		obj = &unstructured.Unstructured{Object: map[string]interface{}{"spec": spec}}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
		obj.SetName("default")
		obj.SetNamespace(namespace)
		obj.SetLabels(labels)
		return k8sClient.Create(ctx, obj)
	}

	obj.SetLabels(mergeStringMap(obj.GetLabels(), labels))
	if err := unstructured.SetNestedField(obj.Object, spec, "spec"); err != nil {
		return err
	}
	obj.SetManagedFields(nil)
	return k8sClient.Update(ctx, obj)
}

func ensureArgoCDApplication(ctx context.Context, k8sClient client.Client, namespace, name, projectName, repoURL, path, destinationNamespace, appIdentifier, envIdentifier, componentIdentifier string) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	key := types.NamespacedName{Name: name, Namespace: namespace}
	err := k8sClient.Get(ctx, key, obj)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	labels := map[string]string{
		"paap.io/app":        appIdentifier,
		"paap.io/env":        envIdentifier,
		"paap.io/component":  componentIdentifier,
		"paap.io/managed-by": "paap-server",
	}
	annotations := map[string]string{
		"argocd.argoproj.io/refresh": "hard",
		"paap.io/refreshed-at":       time.Now().UTC().Format(time.RFC3339Nano),
	}
	spec := map[string]interface{}{
		"project": projectName,
		"source": map[string]interface{}{
			"repoURL":        repoURL,
			"targetRevision": "main",
			"path":           path,
		},
		"destination": map[string]interface{}{
			"server":    "https://kubernetes.default.svc",
			"namespace": destinationNamespace,
		},
		"syncPolicy": map[string]interface{}{
			"automated": map[string]interface{}{
				"prune":    true,
				"selfHeal": true,
			},
		},
	}

	if errors.IsNotFound(err) {
		obj = &unstructured.Unstructured{Object: map[string]interface{}{"spec": spec}}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
		obj.SetName(name)
		obj.SetNamespace(namespace)
		obj.SetLabels(labels)
		obj.SetAnnotations(annotations)
		obj.SetFinalizers([]string{"resources-finalizer.argocd.argoproj.io"})
		return k8sClient.Create(ctx, obj)
	}

	obj.SetLabels(mergeStringMap(obj.GetLabels(), labels))
	obj.SetAnnotations(mergeStringMap(obj.GetAnnotations(), annotations))
	obj.SetFinalizers(appendMissingString(obj.GetFinalizers(), "resources-finalizer.argocd.argoproj.io"))
	if err := unstructured.SetNestedField(obj.Object, spec, "spec"); err != nil {
		return err
	}
	obj.SetManagedFields(nil)
	obj.SetResourceVersion(obj.GetResourceVersion())
	return k8sClient.Update(ctx, obj)
}

func mergeStringMap(current, desired map[string]string) map[string]string {
	if current == nil {
		current = make(map[string]string)
	}
	for k, v := range desired {
		current[k] = v
	}
	return current
}

func appendMissingString(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
