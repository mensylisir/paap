package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
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

const componentConfigChecksumAnnotation = "paap.io/config-checksum"

var dnsLabelInvalidChars = regexp.MustCompile(`[^a-z0-9-]+`)
var paapTemplateScalarToken = regexp.MustCompile(`\[\[\s*paap:([^\]\s]+)([^\]]*)\]\]`)
var paapTemplateIfBlock = regexp.MustCompile(`\[\[paap:if\s+([^\]\s]+)\]\]([\s\S]*?)\[\[paap:end\s+([^\]\s]+)\]\]`)
var paapTemplateForBlock = regexp.MustCompile(`\[\[paap:for\s+([^\]\s]+)\]\]([\s\S]*?)\[\[paap:end\s+([^\]\s]+)\]\]`)
var paapTemplateDefaultOption = regexp.MustCompile(`\bdefault=("[^"]*"|'[^']*'|[^\s\]]+)`)

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

type componentFlowContext struct {
	App                  model.Application
	Env                  model.Environment
	Component            model.Component
	Identifier           string
	Namespace            string
	K8sClient            client.Client
	Services             []model.ServiceInstallation
	Targets              []componentDeliveryTarget
	ToolNamespaces       componentToolNamespaces
	ProjectName          string
	RepositoryName       string
	RepositoryPath       string
	ManifestPath         string
	GiteaBaseURL         string
	RepositoryURL        string
	SourceMirrorName     string
	ArgoCDApplication    string
	DestinationNamespace string
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
	parts := make([]string, 0, 3)
	if config := BuildComponentConfigResourceManifest(app, env, comp, identifier, namespace); strings.TrimSpace(config) != "" {
		parts = append(parts, config)
	}
	parts = append(parts, BuildComponentDeploymentManifest(app, env, comp, identifier, namespace))
	parts = append(parts, BuildComponentServiceManifest(app, env, comp, identifier, namespace))
	return strings.Join(parts, "---\n")
}

func BuildComponentDeploymentManifest(app model.Application, env model.Environment, comp model.Component, identifier, namespace string) string {
	replicas := comp.Replicas
	if replicas <= 0 {
		replicas = 1
	}
	if componentUsesSourcePlaceholderImage(comp) {
		replicas = 0
	}
	image := componentDeploymentImage(comp)
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
	volumes, volumeMounts := componentConfigVolumes(cfg)
	podAnnotations := componentConfigPodTemplateAnnotations(cfg)
	containerPort := model.ResolveComponentContainerPort(comp.Type, cfg)
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
				ObjectMeta: metav1.ObjectMeta{Labels: labels, Annotations: podAnnotations},
				Spec: corev1.PodSpec{
					Volumes: volumes,
					Containers: []corev1.Container{{
						Name:         identifier,
						Image:        image,
						Command:      cfg.Command,
						Args:         cfg.Args,
						Ports:        []corev1.ContainerPort{{ContainerPort: containerPort}},
						VolumeMounts: volumeMounts,
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

func componentDeploymentImage(comp model.Component) string {
	if image := strings.TrimSpace(comp.RegistryImage); image != "" {
		return image
	}
	return strings.TrimSpace(comp.Image)
}

func BuildComponentConfigResourceManifest(app model.Application, env model.Environment, comp model.Component, identifier, namespace string) string {
	cfg, err := model.ParseComponentConfig(comp.Config)
	if err != nil {
		return ""
	}
	cfg = renderComponentConfigTemplatePlaceholders(cfg)
	labels := map[string]string{
		"app":                identifier,
		"paap.io/app":        app.Identifier,
		"paap.io/env":        env.Identifier,
		"paap.io/component":  identifier,
		"paap.io/managed-by": "argocd",
	}
	parts := make([]string, 0, len(cfg.ConfigMaps)+len(cfg.Secrets))
	for _, item := range cfg.ConfigMaps {
		cm := corev1.ConfigMap{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      item.Name,
				Namespace: namespace,
				Labels:    labels,
			},
			Data: item.Data,
		}
		cmYAML, _ := yaml.Marshal(cm)
		parts = append(parts, string(cmYAML))
	}
	for _, item := range cfg.Secrets {
		secret := corev1.Secret{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      item.Name,
				Namespace: namespace,
				Labels:    labels,
			},
			Type:       corev1.SecretTypeOpaque,
			StringData: item.Data,
		}
		secretYAML, _ := yaml.Marshal(secret)
		parts = append(parts, string(secretYAML))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "---\n") + "---\n"
}

func renderComponentConfigTemplatePlaceholders(cfg model.ComponentConfig) model.ComponentConfig {
	for i := range cfg.ConfigMaps {
		for key, value := range cfg.ConfigMaps[i].Data {
			cfg.ConfigMaps[i].Data[key] = renderPaapTemplateValue(value, nil)
		}
	}
	for i := range cfg.Secrets {
		for key, value := range cfg.Secrets[i].Data {
			cfg.Secrets[i].Data[key] = renderPaapTemplateValue(value, nil)
		}
	}
	return cfg
}

func renderPaapTemplateValue(value string, values map[string]string) string {
	rendered := value
	for {
		next := paapTemplateForBlock.ReplaceAllStringFunc(rendered, func(match string) string {
			parts := paapTemplateForBlock.FindStringSubmatch(match)
			if len(parts) != 4 || parts[1] != parts[3] {
				return match
			}
			return ""
		})
		next = paapTemplateIfBlock.ReplaceAllStringFunc(next, func(match string) string {
			parts := paapTemplateIfBlock.FindStringSubmatch(match)
			if len(parts) != 4 || parts[1] != parts[3] {
				return match
			}
			if templateTruthy(values[parts[1]]) {
				return renderPaapTemplateValue(parts[2], values)
			}
			return ""
		})
		if next == rendered {
			break
		}
		rendered = next
	}
	return paapTemplateScalarToken.ReplaceAllStringFunc(rendered, func(match string) string {
		parts := paapTemplateScalarToken.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		key := strings.TrimSpace(parts[1])
		if strings.HasPrefix(key, "for") || strings.HasPrefix(key, "if") || strings.HasPrefix(key, "end") {
			return ""
		}
		if values != nil {
			if value := strings.TrimSpace(values[key]); value != "" {
				return value
			}
		}
		return paapTemplateDefaultValue(parts[2])
	})
}

func paapTemplateDefaultValue(options string) string {
	match := paapTemplateDefaultOption.FindStringSubmatch(options)
	if len(match) != 2 {
		return ""
	}
	return strings.Trim(match[1], `"'`)
}

func templateTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on", "enabled":
		return true
	default:
		return false
	}
}

func BuildComponentServiceManifest(app model.Application, env model.Environment, comp model.Component, identifier, namespace string) string {
	cfg, err := model.ParseComponentConfig(comp.Config)
	if err != nil {
		cfg = model.ComponentConfig{}
	}
	containerPort := model.ResolveComponentContainerPort(comp.Type, cfg)
	servicePort := componentServicePort(cfg)
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
				Port:       servicePort,
				TargetPort: intstrFromInt(int(containerPort)),
			}},
		},
	}
	serviceYAML, _ := yaml.Marshal(service)
	return string(serviceYAML)
}

func componentServicePort(cfg model.ComponentConfig) int32 {
	if cfg.ServicePort > 0 {
		return cfg.ServicePort
	}
	return 80
}

func componentConfigPodTemplateAnnotations(cfg model.ComponentConfig) map[string]string {
	checksum := componentConfigChecksum(cfg)
	if checksum == "" {
		return nil
	}
	return map[string]string{componentConfigChecksumAnnotation: checksum}
}

func componentConfigChecksum(cfg model.ComponentConfig) string {
	if len(cfg.ConfigMaps) == 0 && len(cfg.Secrets) == 0 && len(cfg.Files) == 0 {
		return ""
	}
	payload := struct {
		ConfigMaps []model.ComponentConfigMap  `json:"configMaps,omitempty"`
		Secrets    []model.ComponentSecret     `json:"secrets,omitempty"`
		Files      []model.ComponentConfigFile `json:"files,omitempty"`
	}{
		ConfigMaps: cfg.ConfigMaps,
		Secrets:    cfg.Secrets,
		Files:      cfg.Files,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

func componentConfigVolumes(cfg model.ComponentConfig) ([]corev1.Volume, []corev1.VolumeMount) {
	if len(cfg.Files) == 0 {
		return nil, nil
	}
	volumes := make([]corev1.Volume, 0, len(cfg.Files))
	mounts := make([]corev1.VolumeMount, 0, len(cfg.Files))
	seenVolumes := map[string]struct{}{}
	for i, item := range cfg.Files {
		name := volumeNameForConfigFile(item, i)
		if _, exists := seenVolumes[name]; !exists {
			seenVolumes[name] = struct{}{}
			volumes = append(volumes, corev1.Volume{
				Name: name,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: item.ConfigMapName},
					},
				},
			})
		}
		mounts = append(mounts, corev1.VolumeMount{
			Name:      name,
			MountPath: item.MountPath,
			SubPath:   item.Key,
			ReadOnly:  true,
		})
	}
	return volumes, mounts
}

func volumeNameForConfigFile(item model.ComponentConfigFile, index int) string {
	name := dnsLabelInvalidChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(item.Name)), "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = dnsLabelInvalidChars.ReplaceAllString(strings.ToLower(strings.TrimSpace(item.ConfigMapName)), "-")
		name = strings.Trim(name, "-")
	}
	if name == "" {
		name = fmt.Sprintf("config-file-%d", index+1)
	}
	if len(name) > 50 {
		name = strings.Trim(name[:50], "-")
	}
	return name
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

func newComponentFlowContext(ctx context.Context, k8sClient client.Client, app model.Application, env model.Environment, comp model.Component, identifier, namespace string, targets []componentDeliveryTarget) componentFlowContext {
	toolNamespaces := discoverComponentToolNamespaces(ctx, k8sClient, app.Identifier, env.Identifier)
	services := componentDeliveryServicesFromTargets(targets)
	if serviceNamespaces := componentToolNamespacesFromServices(services); !componentToolNamespacesEmpty(serviceNamespaces) {
		toolNamespaces = mergeComponentToolNamespaces(toolNamespaces, serviceNamespaces)
	}
	repoName := fmt.Sprintf("%s-%s-components", app.Identifier, env.Identifier)
	repoPath := fmt.Sprintf("components/%s", identifier)
	giteaBaseURL := ""
	repositoryURL := ""
	if toolNamespaces.Gitea != "" {
		giteaBaseURL = fmt.Sprintf("http://%s.%s.svc.cluster.local:3000", toolNamespaces.Gitea, toolNamespaces.Gitea)
		repositoryURL = fmt.Sprintf("%s/%s/%s.git", giteaBaseURL, giteaAdminUser, repoName)
	}
	return componentFlowContext{
		App:                  app,
		Env:                  env,
		Component:            comp,
		Identifier:           identifier,
		Namespace:            namespace,
		K8sClient:            k8sClient,
		Services:             services,
		Targets:              targets,
		ToolNamespaces:       toolNamespaces,
		ProjectName:          fmt.Sprintf("%s-%s", app.Identifier, env.Identifier),
		RepositoryName:       repoName,
		RepositoryPath:       repoPath,
		ManifestPath:         repoPath + "/deployment.yaml",
		GiteaBaseURL:         giteaBaseURL,
		RepositoryURL:        repositoryURL,
		SourceMirrorName:     fmt.Sprintf("%s-%s-%s-source", app.Identifier, env.Identifier, identifier),
		ArgoCDApplication:    fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, identifier),
		DestinationNamespace: namespace,
	}
}

func componentDeliveryServicesFromTargets(targets []componentDeliveryTarget) []model.ServiceInstallation {
	services := make([]model.ServiceInstallation, 0, len(targets))
	seen := map[uint]bool{}
	for _, target := range targets {
		if target.Service == nil || target.Service.ID == 0 || seen[target.Service.ID] {
			continue
		}
		seen[target.Service.ID] = true
		services = append(services, *target.Service)
	}
	return services
}

func componentToolNamespacesFromServices(services []model.ServiceInstallation) componentToolNamespaces {
	result := componentToolNamespaces{}
	for _, inst := range services {
		if !componentDeliveryServiceIsReady(inst) {
			continue
		}
		serviceType := strings.ToLower(strings.TrimSpace(inst.ServiceType))
		switch serviceType {
		case "deploy":
			if result.ArgoCD == "" {
				result.ArgoCD = strings.TrimSpace(inst.Namespace)
			}
		case "git":
			if result.Gitea == "" {
				result.Gitea = strings.TrimSpace(inst.Namespace)
			}
		case "ci":
			if result.Jenkins == "" {
				result.Jenkins = strings.TrimSpace(inst.Namespace)
			}
		}
	}
	return result
}

func componentToolNamespacesEmpty(namespaces componentToolNamespaces) bool {
	return strings.TrimSpace(namespaces.ArgoCD) == "" &&
		strings.TrimSpace(namespaces.Gitea) == "" &&
		strings.TrimSpace(namespaces.Jenkins) == ""
}

func mergeComponentToolNamespaces(base, override componentToolNamespaces) componentToolNamespaces {
	if strings.TrimSpace(override.ArgoCD) != "" {
		base.ArgoCD = strings.TrimSpace(override.ArgoCD)
	}
	if strings.TrimSpace(override.Gitea) != "" {
		base.Gitea = strings.TrimSpace(override.Gitea)
	}
	if strings.TrimSpace(override.Jenkins) != "" {
		base.Jenkins = strings.TrimSpace(override.Jenkins)
	}
	return base
}

func componentSourceBuildIsPending(comp model.Component) bool {
	if comp.DeliveryMode != "source" {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(comp.PipelineStatus), "built")
}

func validateComponentGitOpsInput(comp model.Component) error {
	if comp.Version == "" && imageTag(comp.Image) == "" {
		return fmt.Errorf("component version is required when image tag is missing")
	}
	return nil
}

func validateComponentSourceBuildTools(flow componentFlowContext) error {
	if strings.TrimSpace(flow.ToolNamespaces.Gitea) == "" || strings.TrimSpace(flow.GiteaBaseURL) == "" || strings.TrimSpace(flow.RepositoryURL) == "" {
		return fmt.Errorf("environment git service is required before source delivery")
	}
	if strings.TrimSpace(flow.ToolNamespaces.Jenkins) == "" {
		return fmt.Errorf("environment ci service is required before source delivery")
	}
	return nil
}

func validateComponentGitOpsTools(flow componentFlowContext) error {
	if strings.TrimSpace(flow.ToolNamespaces.Gitea) == "" || strings.TrimSpace(flow.GiteaBaseURL) == "" || strings.TrimSpace(flow.RepositoryURL) == "" {
		return fmt.Errorf("environment git service is required before GitOps delivery")
	}
	if strings.TrimSpace(flow.ToolNamespaces.ArgoCD) == "" {
		return fmt.Errorf("environment cd service is required before GitOps delivery")
	}
	return nil
}

func RunComponentSourceDeliveryFlow(ctx context.Context, k8sClient client.Client, app model.Application, env model.Environment, comp model.Component, identifier, namespace string, targets []componentDeliveryTarget) (ComponentGitOpsResult, error) {
	flow := newComponentFlowContext(ctx, k8sClient, app, env, comp, identifier, namespace, targets)
	if componentSourceBuildIsPending(flow.Component) {
		if strings.EqualFold(strings.TrimSpace(flow.Component.PipelineStatus), "planned") {
			return RunComponentSourceBuildFlow(ctx, flow)
		}
		ready, warning, err := actionDetectSourceBuildCompletion(ctx, flow)
		if err != nil {
			return ComponentGitOpsResult{}, err
		}
		if ready {
			flow.Component.PipelineStatus = "built"
			result, err := RunComponentGitOpsDeploymentFlow(ctx, flow)
			if err != nil {
				return ComponentGitOpsResult{}, err
			}
			result.CIStatus = "built"
			result.CIWarning = warning
			return result, nil
		}
		if sourceBuildWarningIsFailed(warning) {
			return ComponentGitOpsResult{
				RepositoryURL:       flow.RepositoryURL,
				SourceMirrorURL:     flow.Component.SourceMirrorRepoURL,
				RepositoryPath:      flow.RepositoryPath,
				ArgoCDNamespace:     flow.ToolNamespaces.ArgoCD,
				DeploymentNamespace: flow.DestinationNamespace,
				CIStatus:            "failed",
				CIWarning:           warning,
			}, nil
		}
		if strings.TrimSpace(warning) != "" {
			return ComponentGitOpsResult{
				RepositoryURL:       flow.RepositoryURL,
				SourceMirrorURL:     flow.Component.SourceMirrorRepoURL,
				RepositoryPath:      flow.RepositoryPath,
				ArgoCDNamespace:     flow.ToolNamespaces.ArgoCD,
				DeploymentNamespace: flow.DestinationNamespace,
				CIStatus:            "running",
				CIWarning:           warning,
			}, nil
		}
		return RunComponentSourceBuildFlow(ctx, flow)
	}
	return RunComponentGitOpsDeploymentFlow(ctx, flow)
}

func sourceBuildWarningIsFailed(warning string) bool {
	warning = strings.ToLower(strings.TrimSpace(warning))
	return strings.Contains(warning, "buildfailed") || strings.Contains(warning, "failed to build")
}

func RunComponentImageDeliveryFlow(ctx context.Context, k8sClient client.Client, app model.Application, env model.Environment, comp model.Component, identifier, namespace string, targets []componentDeliveryTarget) (ComponentGitOpsResult, error) {
	return RunComponentGitOpsDeploymentFlow(ctx, newComponentFlowContext(ctx, k8sClient, app, env, comp, identifier, namespace, targets))
}

func RunComponentSourceBuildFlow(ctx context.Context, flow componentFlowContext) (ComponentGitOpsResult, error) {
	if err := validateComponentGitOpsInput(flow.Component); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if err := validateComponentSourceBuildTools(flow); err != nil {
		return ComponentGitOpsResult{}, err
	}
	sourceMirrorURL, ciStatus, ciWarning, err := stepPrepareSourceBuild(ctx, flow)
	if err != nil {
		return ComponentGitOpsResult{}, err
	}
	return ComponentGitOpsResult{
		RepositoryURL:       flow.RepositoryURL,
		SourceMirrorURL:     sourceMirrorURL,
		RepositoryPath:      flow.RepositoryPath,
		ArgoCDNamespace:     flow.ToolNamespaces.ArgoCD,
		DeploymentNamespace: flow.DestinationNamespace,
		CIStatus:            ciStatus,
		CIWarning:           ciWarning,
	}, nil
}

func RunComponentGitOpsDeploymentFlow(ctx context.Context, flow componentFlowContext) (ComponentGitOpsResult, error) {
	if err := validateComponentGitOpsInput(flow.Component); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if err := validateComponentGitOpsTools(flow); err != nil {
		return ComponentGitOpsResult{}, err
	}
	flow = prepareComponentGitOpsDeploymentContext(ctx, flow)
	if err := stepPublishComponentDeploymentFiles(ctx, flow); err != nil {
		return ComponentGitOpsResult{}, err
	}
	if err := stepConfigureComponentArgoCD(ctx, flow); err != nil {
		return ComponentGitOpsResult{}, err
	}
	return ComponentGitOpsResult{
		RepositoryURL:       flow.RepositoryURL,
		SourceMirrorURL:     flow.Component.SourceMirrorRepoURL,
		RepositoryPath:      flow.RepositoryPath,
		ArgoCDApplication:   flow.ArgoCDApplication,
		ArgoCDNamespace:     flow.ToolNamespaces.ArgoCD,
		DeploymentNamespace: flow.DestinationNamespace,
	}, nil
}

func prepareComponentGitOpsDeploymentContext(ctx context.Context, flow componentFlowContext) componentFlowContext {
	if flow.Component.DeliveryMode != "source" || !strings.EqualFold(strings.TrimSpace(flow.Component.PipelineStatus), "built") {
		return flow
	}
	version := strings.TrimSpace(flow.Component.Version)
	if version == "" {
		version = imageTag(componentDeploymentImage(flow.Component))
	}
	if version == "" {
		return flow
	}
	target, ok := componentSourceRegistryDeliveryTarget(ctx, flow)
	if !ok {
		return flow
	}
	flow.Component = applyComponentDeployVersionForRuntimeRegistryTarget(flow.App, flow.Env, flow.Component, flow.Identifier, version, target)
	return flow
}

func stepPrepareComponentRepository(ctx context.Context, flow componentFlowContext) error {
	return actionEnsureComponentRepository(ctx, flow)
}

func stepPrepareSourceBuild(ctx context.Context, flow componentFlowContext) (string, string, string, error) {
	if err := stepPrepareComponentRepository(ctx, flow); err != nil {
		return "", "", "", err
	}
	sourceMirrorURL, err := actionEnsureSourceMirror(ctx, flow)
	if err != nil {
		return "", "", "", err
	}
	flow.Component.SourceMirrorRepoURL = sourceMirrorURL
	internalBuildImage := componentSourceInternalBuildImage(ctx, flow)
	internalRegistryTarget := componentSourceRegistryTargetIsInternal(ctx, flow)
	kpackSpec, caWarning := buildComponentKpackSpecWithRegistryCAMirrors(ctx, flow.K8sClient, flow.App, flow.Env, flow.Identifier, internalRegistryTarget, internalBuildImage)
	kpackSpec.Namespace = flow.ToolNamespaces.Jenkins
	kpackSpec.GitServer = flow.GiteaBaseURL
	mirrorWarning := ""
	if internalRegistryTarget {
		mirrorWarning = actionEnsureKpackMirrorImages(ctx, flow, kpackSpec)
	}
	if err := actionWriteComponentJenkinsfile(ctx, flow, internalBuildImage, len(kpackSpec.RegistryCAPEM) > 0); err != nil {
		return "", "", "", err
	}
	if err := actionWriteComponentReadme(ctx, flow); err != nil {
		return "", "", "", err
	}
	ciStatus, ciWarning := actionConfigureComponentJenkins(ctx, flow, kpackSpec, caWarning)
	ciWarning = appendCIWarning(ciWarning, mirrorWarning)
	return sourceMirrorURL, ciStatus, ciWarning, nil
}

func stepPublishComponentDeploymentFiles(ctx context.Context, flow componentFlowContext) error {
	if err := stepPrepareComponentRepository(ctx, flow); err != nil {
		return err
	}
	if err := actionWriteComponentDeploymentManifest(ctx, flow); err != nil {
		return err
	}
	if err := actionWriteComponentConfigManifest(ctx, flow); err != nil {
		return err
	}
	return actionWriteComponentServiceManifest(ctx, flow)
}

func stepConfigureComponentArgoCD(ctx context.Context, flow componentFlowContext) error {
	if err := actionEnsureArgoCDRepository(ctx, flow); err != nil {
		return err
	}
	destinationNamespaces := actionDiscoverArgoCDDestinationNamespaces(ctx, flow)
	if err := actionEnsureArgoCDLocalCluster(ctx, flow, destinationNamespaces); err != nil {
		return err
	}
	if err := actionDenyArgoCDDefaultProject(ctx, flow); err != nil {
		return err
	}
	if err := actionEnsureArgoCDProject(ctx, flow, destinationNamespaces); err != nil {
		return err
	}
	return actionEnsureArgoCDApplication(ctx, flow)
}

func actionEnsureComponentRepository(ctx context.Context, flow componentFlowContext) error {
	return ensureGiteaRepository(ctx, flow.GiteaBaseURL, flow.RepositoryName)
}

func actionEnsureSourceMirror(ctx context.Context, flow componentFlowContext) (string, error) {
	return ensureGiteaSourceMirror(ctx, flow.GiteaBaseURL, flow.SourceMirrorName, flow.Component)
}

var copyComponentKpackMirrorImage = k8s.CopyContainerImageIfMissing
var componentKpackMirrorImageExists = k8s.ContainerImageExists
var startComponentKpackMirrorImage = scheduleComponentKpackMirrorImage
var componentKpackMirrorJobs sync.Map

func actionEnsureKpackMirrorImages(ctx context.Context, flow componentFlowContext, spec k8s.KpackBuildEnvironmentSpec) string {
	target, ok := componentSourceRegistryDeliveryTarget(ctx, flow)
	if !ok || target.Source == model.CapabilitySourceExternal {
		return ""
	}
	copyOptions := k8s.ContainerImageCopyOptions{
		TargetInsecure: strings.EqualFold(strings.TrimSpace(target.ServiceType), "registry"),
	}
	if strings.EqualFold(strings.TrimSpace(target.ServiceType), "harbor") {
		copyOptions.TargetUsername = firstNonEmpty(spec.RegistryUsername, "admin")
		copyOptions.TargetPassword = firstNonEmpty(spec.RegistryPassword, "Harbor12345")
	}

	missing := make([]string, 0)
	for _, pair := range componentKpackMirrorImagePairs(spec) {
		exists, _ := componentKpackMirrorImageExists(ctx, pair.target, copyOptions)
		if exists {
			continue
		}
		missing = append(missing, pair.target)
		startComponentKpackMirrorImage(pair, copyOptions)
	}
	if len(missing) == 0 {
		return ""
	}
	return "kpack base images are being mirrored into the selected internal registry; source deployment will continue after mirror completes: " + strings.Join(missing, ", ")
}

func scheduleComponentKpackMirrorImage(pair componentKpackMirrorImagePair, opts k8s.ContainerImageCopyOptions) {
	key := pair.source + "\n" + pair.target
	if _, loaded := componentKpackMirrorJobs.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	go func() {
		defer componentKpackMirrorJobs.Delete(key)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		if err := copyComponentKpackMirrorImage(ctx, pair.source, pair.target, opts); err != nil {
			log.Printf("[kpackMirror] failed to mirror %s -> %s: %v", pair.source, pair.target, err)
			return
		}
		log.Printf("[kpackMirror] mirrored %s -> %s", pair.source, pair.target)
	}()
}

type componentKpackMirrorImagePair struct {
	source string
	target string
}

func componentKpackMirrorImagePairs(spec k8s.KpackBuildEnvironmentSpec) []componentKpackMirrorImagePair {
	pairs := []componentKpackMirrorImagePair{
		{source: k8s.PaketoBuildJammyBaseImage, target: spec.StackBuildImage},
		{source: k8s.PaketoRunJammyBaseImage, target: spec.StackRunImage},
	}
	sources := []string{
		k8s.PaketoJavaBuildpackImage,
		k8s.PaketoNodeJSBuildpackImage,
		k8s.PaketoGoBuildpackImage,
		k8s.PaketoPythonBuildpackImage,
	}
	for i, target := range spec.BuildpackSources {
		if i >= len(sources) {
			break
		}
		pairs = append(pairs, componentKpackMirrorImagePair{source: sources[i], target: target})
	}
	out := make([]componentKpackMirrorImagePair, 0, len(pairs))
	for _, pair := range pairs {
		if strings.TrimSpace(pair.source) == "" || strings.TrimSpace(pair.target) == "" {
			continue
		}
		if pair.source == pair.target {
			continue
		}
		out = append(out, pair)
	}
	return out
}

func actionWriteComponentDeploymentManifest(ctx context.Context, flow componentFlowContext) error {
	manifest := BuildComponentDeploymentManifest(flow.App, flow.Env, flow.Component, flow.Identifier, flow.DestinationNamespace)
	return putGiteaFile(ctx, flow.GiteaBaseURL, flow.RepositoryName, flow.ManifestPath, manifest, fmt.Sprintf("deploy %s", flow.Identifier))
}

func actionWriteComponentConfigManifest(ctx context.Context, flow componentFlowContext) error {
	configManifest := BuildComponentConfigResourceManifest(flow.App, flow.Env, flow.Component, flow.Identifier, flow.DestinationNamespace)
	if strings.TrimSpace(configManifest) == "" {
		return nil
	}
	return putGiteaFile(ctx, flow.GiteaBaseURL, flow.RepositoryName, flow.RepositoryPath+"/config.yaml", configManifest, fmt.Sprintf("sync config for %s", flow.Identifier))
}

func actionWriteComponentServiceManifest(ctx context.Context, flow componentFlowContext) error {
	serviceManifest := BuildComponentServiceManifest(flow.App, flow.Env, flow.Component, flow.Identifier, flow.DestinationNamespace)
	return putGiteaFile(ctx, flow.GiteaBaseURL, flow.RepositoryName, flow.RepositoryPath+"/service.yaml", serviceManifest, fmt.Sprintf("sync service for %s", flow.Identifier))
}

func actionWriteComponentJenkinsfile(ctx context.Context, flow componentFlowContext, buildImage string, includeRegistryCABinding bool) error {
	componentForBuild := flow.Component
	if strings.TrimSpace(buildImage) != "" {
		componentForBuild.Image = strings.TrimSpace(buildImage)
		componentForBuild.RegistryImage = strings.TrimSpace(buildImage)
	}
	jenkinsfile := buildComponentJenkinsfile(componentForBuild, flow.Identifier, flow.RepositoryURL, includeRegistryCABinding)
	return putGiteaFile(ctx, flow.GiteaBaseURL, flow.RepositoryName, flow.RepositoryPath+"/Jenkinsfile", jenkinsfile, fmt.Sprintf("sync jenkinsfile for %s", flow.Identifier))
}

func actionWriteComponentReadme(ctx context.Context, flow componentFlowContext) error {
	readme := buildComponentReadme(flow.App, flow.Env, flow.Component, flow.Identifier, flow.RepositoryURL, flow.RepositoryPath)
	return putGiteaFileIfNotExists(ctx, flow.GiteaBaseURL, flow.RepositoryName, flow.RepositoryPath+"/README.md", readme, fmt.Sprintf("init readme for %s", flow.Identifier))
}

func actionConfigureComponentJenkins(ctx context.Context, flow componentFlowContext, kpackSpec k8s.KpackBuildEnvironmentSpec, caWarning string) (string, string) {
	return ensureComponentJenkinsAutomation(ctx, flow.K8sClient, flow.App, flow.Env, flow.Component, flow.Identifier, flow.GiteaBaseURL, flow.RepositoryName, flow.RepositoryURL, kpackSpec, caWarning)
}

func componentSourceInternalBuildImage(ctx context.Context, flow componentFlowContext) string {
	image := strings.TrimSpace(flow.Component.Image)
	host := componentSourceInternalRegistryHost(ctx, flow)
	if host == "" && image == "" {
		host = RuntimeRegistryHost(flow.App, flow.Env, "registry")
	}
	if host == "" {
		return image
	}
	repository := imageRepositoryPath(image)
	if repository == "" {
		primaryNS := fmt.Sprintf("%s-%s", flow.App.Identifier, flow.Env.Identifier)
		repository = fmt.Sprintf("%s/%s", primaryNS, flow.Identifier)
	}
	tag := imageTag(image)
	if tag == "" {
		tag = strings.TrimSpace(flow.Component.Version)
	}
	if tag == "" {
		tag = "manual"
	}
	return strings.TrimRight(host, "/") + "/" + strings.Trim(repository, "/") + ":" + tag
}

func componentSourceInternalRegistryHost(ctx context.Context, flow componentFlowContext) string {
	targets := flow.Targets
	if len(targets) == 0 {
		targets = componentDeliveryTargetsFromServices(flow.Services)
	}
	cfg, _ := model.ParseComponentConfig(flow.Component.Config)
	target, ok := preferredComponentRegistryDeliveryTarget(targets, cfg.RegistryTarget)
	if !ok {
		return registryServerFromImageRef(flow.Component.Image)
	}
	if target.Source == model.CapabilitySourceExternal {
		return externalEndpointHost(target.ExternalEndpoint)
	}
	if target.Service == nil || strings.TrimSpace(target.Service.Namespace) == "" {
		return registryServerFromImageRef(flow.Component.Image)
	}
	if !strings.EqualFold(target.ServiceType, "registry") && !strings.EqualFold(target.ServiceType, "harbor") {
		return registryServerFromImageRef(flow.Component.Image)
	}
	return internalServiceRegistryHost(ctx, *target.Service)
}

func componentSourceRegistryTargetIsInternal(ctx context.Context, flow componentFlowContext) bool {
	target, ok := componentSourceRegistryDeliveryTarget(ctx, flow)
	if !ok {
		return false
	}
	if target.Source == model.CapabilitySourceExternal {
		return false
	}
	if target.Service == nil || strings.TrimSpace(target.Service.Namespace) == "" {
		return false
	}
	return strings.EqualFold(target.ServiceType, "registry") || strings.EqualFold(target.ServiceType, "harbor")
}

func componentSourceRegistryDeliveryTarget(ctx context.Context, flow componentFlowContext) (componentDeliveryTarget, bool) {
	targets := flow.Targets
	if len(targets) == 0 {
		targets = componentDeliveryTargetsFromServices(flow.Services)
	}
	cfg, _ := model.ParseComponentConfig(flow.Component.Config)
	return preferredComponentRegistryDeliveryTarget(targets, cfg.RegistryTarget)
}

func internalServiceRegistryHost(ctx context.Context, inst model.ServiceInstallation) string {
	namespace := strings.TrimSpace(inst.Namespace)
	if namespace == "" {
		return ""
	}
	releaseName := strings.TrimSpace(inst.ReleaseName)
	if releaseName == "" {
		releaseName = namespace
	}
	if network, err := k8s.DiscoverRegistryServiceNetwork(ctx, namespace, releaseName); err == nil && network != nil && strings.TrimSpace(network.ServiceName) != "" {
		port := network.Port
		if port <= 0 {
			port = 5000
		}
		if clusterIP := strings.TrimSpace(network.ClusterIP); clusterIP != "" {
			return fmt.Sprintf("%s:%d", clusterIP, port)
		}
		return fmt.Sprintf("%s.%s.svc.cluster.local:%d", network.ServiceName, namespace, port)
	}
	return fmt.Sprintf("%s.%s.svc.cluster.local:5000", releaseName, namespace)
}

func imageRepositoryPath(image string) string {
	image = ImageWithoutTag(strings.TrimSpace(image))
	if image == "" {
		return ""
	}
	parts := strings.Split(image, "/")
	if len(parts) > 1 && imageReferenceFirstSegmentIsRegistry(parts[0]) {
		parts = parts[1:]
	}
	return strings.Join(parts, "/")
}

func actionDetectSourceBuildCompletion(ctx context.Context, flow componentFlowContext) (bool, string, error) {
	if strings.TrimSpace(flow.ToolNamespaces.Jenkins) == "" {
		return false, "", nil
	}
	status, err := k8s.GetKpackImageStatus(ctx, flow.K8sClient, flow.ToolNamespaces.Jenkins, flow.Identifier)
	if err != nil {
		return false, "", err
	}
	if !status.Exists || !status.Ready {
		return false, status.Warning, nil
	}
	return true, "", nil
}

func actionEnsureArgoCDRepository(ctx context.Context, flow componentFlowContext) error {
	return ensureArgoCDRepositorySecret(ctx, flow.K8sClient, flow.ToolNamespaces.ArgoCD, flow.RepositoryName, flow.RepositoryURL, giteaAdminUser, giteaAdminPassword)
}

func actionDiscoverArgoCDDestinationNamespaces(ctx context.Context, flow componentFlowContext) []string {
	return discoverComponentGitOpsNamespaces(ctx, flow.K8sClient, flow.App.Identifier, flow.Env.Identifier, flow.DestinationNamespace)
}

func actionEnsureArgoCDLocalCluster(ctx context.Context, flow componentFlowContext, destinationNamespaces []string) error {
	return ensureArgoCDLocalClusterSecret(ctx, flow.K8sClient, flow.ToolNamespaces.ArgoCD, destinationNamespaces)
}

func actionDenyArgoCDDefaultProject(ctx context.Context, flow componentFlowContext) error {
	return ensureArgoCDDefaultProjectDenied(ctx, flow.K8sClient, flow.ToolNamespaces.ArgoCD)
}

func actionEnsureArgoCDProject(ctx context.Context, flow componentFlowContext, destinationNamespaces []string) error {
	return ensureArgoCDProject(ctx, flow.K8sClient, flow.ToolNamespaces.ArgoCD, flow.ProjectName, flow.RepositoryURL, destinationNamespaces)
}

func actionEnsureArgoCDApplication(ctx context.Context, flow componentFlowContext) error {
	return ensureArgoCDApplication(ctx, flow.K8sClient, flow.ToolNamespaces.ArgoCD, flow.ArgoCDApplication, flow.ProjectName, flow.RepositoryURL, flow.RepositoryPath, flow.DestinationNamespace, flow.App.Identifier, flow.Env.Identifier, flow.Identifier)
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
	targetURL := giteaRepoURL(baseURL, repoName)
	body := map[string]interface{}{
		"clone_addr": sourceURL,
		"repo_name":  repoName,
		"repo_owner": giteaAdminUser,
		"mirror":     true,
		"private":    false,
	}
	status, response, err := giteaRequest(ctx, http.MethodPost, baseURL+"/api/v1/repos/migrate", body)
	if err != nil {
		return "", err
	}
	if status == http.StatusCreated || status == http.StatusConflict || status == http.StatusUnprocessableEntity {
		return targetURL, nil
	}
	return "", fmt.Errorf("migrate gitea source mirror failed: status=%d body=%s", status, string(response))
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

	jenkins := newComponentJenkinsClient(kpackSpec.Namespace)
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
	return buildComponentKpackSpecWithRegistryCAMirrors(ctx, k8sClient, app, env, identifier, false, imageRefs...)
}

func buildComponentKpackSpecWithRegistryCAMirrors(ctx context.Context, k8sClient client.Client, app model.Application, env model.Environment, identifier string, mirrorBuildpackImages bool, imageRefs ...string) (k8s.KpackBuildEnvironmentSpec, string) {
	primaryNS := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	toolNamespaces := discoverComponentToolNamespaces(ctx, k8sClient, app.Identifier, env.Identifier)
	registryHost := RuntimeRegistryHost(app, env, "registry")
	if len(imageRefs) > 0 {
		if host := registryServerFromImageRef(imageRefs[0]); host != "" {
			registryHost = host
		}
	}
	gitServer := ""
	if toolNamespaces.Gitea != "" {
		gitServer = fmt.Sprintf("http://%s.%s.svc.cluster.local:3000", toolNamespaces.Gitea, toolNamespaces.Gitea)
	}
	spec := k8s.KpackBuildEnvironmentSpec{
		Namespace:       toolNamespaces.Jenkins,
		RegistryServer:  registryHost,
		GitServer:       gitServer,
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
	if mirrorBuildpackImages {
		spec.StackBuildImage, spec.StackRunImage = mirroredPaketoStackImages(registryHost, primaryNS, false)
		spec.BuildpackSources = mirroredPaketoBuildpackSources(registryHost, primaryNS)
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
		spec.StackBuildImage, spec.StackRunImage = mirroredPaketoStackImages(registryHost, primaryNS, true)
		spec.BuildpackSources = mirroredPaketoBuildpackSources(registryHost, primaryNS)
	}
	return spec, warning
}

func mirroredPaketoStackImages(registryHost, namespace string, registryCA bool) (string, string) {
	buildTag := "0.1.233"
	runTag := "0.1.233"
	if registryCA {
		buildTag = "registry-ca"
		runTag = "registry-ca"
	}
	return fmt.Sprintf("%s/%s/paap-build-jammy-base:%s", registryHost, namespace, buildTag),
		fmt.Sprintf("%s/%s/paap-run-jammy-base:%s", registryHost, namespace, runTag)
}

func mirroredPaketoBuildpackSources(registryHost, namespace string) []string {
	return []string{
		fmt.Sprintf("%s/%s/paap-buildpack-java:22.0.0", registryHost, namespace),
		fmt.Sprintf("%s/%s/paap-buildpack-nodejs:10.3.2", registryHost, namespace),
		fmt.Sprintf("%s/%s/paap-buildpack-go:4.19.14", registryHost, namespace),
		fmt.Sprintf("%s/%s/paap-buildpack-python:2.49.0", registryHost, namespace),
	}
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
	return strings.TrimRight(baseURL, "/") + k8s.JenkinsJobPath(jobName) + "/buildWithParameters?token=paap-source-build"
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
	buildTag := valueOrDefault(comp.Version, imageTag(comp.Image))
	if buildTag == "" {
		buildTag = "manual"
	}
	buildCtx := comp.BuildContext
	if buildCtx == "" {
		buildCtx = "."
	}
	buildModule := strings.TrimSpace(comp.BuildModule)
	sourceSubPath := ""
	if buildCtx != "." {
		sourceSubPath = "    subPath: ${BUILD_CONTEXT}\n"
	}
	sourceRepo := strings.TrimSpace(comp.SourceMirrorRepoURL)
	if sourceRepo == "" {
		sourceRepo = strings.TrimSpace(comp.GitRepoURL)
	}
	if sourceRepo == "" {
		sourceRepo = fallbackRepoURL
	}
	sourceBranch := strings.TrimSpace(comp.SourceBranch)
	if sourceBranch == "" {
		sourceBranch = "main"
	}
	kpackAPIVersion := "kpack.io/v1alpha2"
	kpackServiceAccountField := "serviceAccountName"
	if includeRegistryCABinding {
		kpackAPIVersion = "kpack.io/v1alpha1"
		kpackServiceAccountField = "serviceAccount"
	}
	buildEnv := componentKpackBuildEnvYAML(buildModule, includeRegistryCABinding)
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
            def IMAGE_TAG = "%s"
            def KPACK_IMAGE = "%s"
            def BUILD_MODULE = "%s"
            def KPACK_BUILDER = "paap-builder"
            def KPACK_SERVICE_ACCOUNT = "paap-kpack-build"

            try {
        stage('Submit Buildpacks Image') {
                    writeFile file: 'kpack-image.yaml', text: """
apiVersion: %s
kind: Image
metadata:
  name: ${KPACK_IMAGE}
spec:
  tag: ${IMAGE_NAME}:${IMAGE_TAG}
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
                    sh """
if kubectl get image.kpack.io/${KPACK_IMAGE} >/dev/null 2>&1; then
  READY_STATUS=\$(kubectl get image.kpack.io/${KPACK_IMAGE} -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' || true)
  LATEST_IMAGE=\$(kubectl get image.kpack.io/${KPACK_IMAGE} -o jsonpath='{.status.latestImage}' || true)
  if [ "\${READY_STATUS}" != "True" ] && [ -z "\${LATEST_IMAGE}" ]; then
    kubectl delete image.kpack.io/${KPACK_IMAGE} --wait=true
    kubectl delete builds.kpack.io -l image.kpack.io/image=${KPACK_IMAGE} --ignore-not-found=true --wait=true
  fi
fi
"""
                    sh "kubectl apply -f kpack-image.yaml"
                    sh "kubectl wait image.kpack.io/${KPACK_IMAGE} --for=condition=Ready=True --timeout=30m"
        }
            } finally {
                deleteDir()
            }
        }
    }
}`, jenkinsKubectlImage, jenkinsInboundAgentImage, image, buildCtx, sourceRepo, sourceBranch, buildTag, identifier, buildModule, kpackAPIVersion, kpackServiceAccountField, buildEnv, sourceSubPath)
}

func componentKpackBuildEnvYAML(buildModule string, includeRegistryCABinding bool) string {
	var lines []string
	var envLines []string
	if strings.TrimSpace(buildModule) != "" {
		envLines = append(envLines,
			"    - name: BP_MAVEN_BUILT_MODULE",
			"      value: \"${BUILD_MODULE}\"",
			"    - name: BP_MAVEN_BUILD_ARGUMENTS",
			"      value: \"-pl ${BUILD_MODULE} -am -Dmaven.test.skip=true --no-transfer-progress package\"",
			"    - name: BP_JVM_VERSION",
			"      value: \"8.*\"",
		)
	}
	envLines = append(envLines, componentKpackProxyBuildEnvYAML()...)
	if len(envLines) == 0 && !includeRegistryCABinding {
		return ""
	}
	lines = append(lines, "  build:")
	if len(envLines) > 0 {
		lines = append(lines, "    env:")
		lines = append(lines, envLines...)
	}
	if includeRegistryCABinding {
		lines = append(lines,
			"    cnbBindings:",
			"    - name: paap-registry-ca",
			"      secretRef:",
			"        name: paap-kpack-registry-ca",
		)
	}
	return strings.Join(lines, "\n") + "\n"
}

func componentKpackProxyBuildEnvYAML() []string {
	pairs := []struct {
		name string
		env  string
	}{
		{name: "http_proxy", env: "PAAP_BUILDPACK_HTTP_PROXY"},
		{name: "https_proxy", env: "PAAP_BUILDPACK_HTTPS_PROXY"},
		{name: "no_proxy", env: "PAAP_BUILDPACK_NO_PROXY"},
		{name: "BP_JVM_TYPE", env: "PAAP_BUILDPACK_JVM_TYPE"},
		{name: "BP_LOG_LEVEL", env: "PAAP_BUILDPACK_LOG_LEVEL"},
	}
	lines := make([]string, 0, len(pairs)*2)
	for _, pair := range pairs {
		value := strings.TrimSpace(os.Getenv(pair.env))
		if value == "" {
			continue
		}
		lines = append(lines,
			fmt.Sprintf("    - name: %s", pair.name),
			fmt.Sprintf("      value: %q", value),
		)
	}
	return lines
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
	client := &http.Client{Timeout: 15 * time.Second}
	res, err := client.Do(req)
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
	result := componentToolNamespaces{}
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
