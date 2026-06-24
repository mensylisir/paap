package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	paapv1 "paap/api/v1"
	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/model"
	"paap/internal/service"
)

var k8sClient = k8s.NewClient()

type harborProjectEnsurer interface {
	EnsureProject(ctx context.Context, name string) error
}

var newHarborProjectEnsurer = func(namespace string) harborProjectEnsurer {
	return k8s.NewHarborClient(namespace)
}

type giteaWorkspaceClient interface {
	Repositories(ctx context.Context) ([]k8s.GiteaRepository, error)
	RepositoryContents(ctx context.Context, repo, path, ref string) ([]k8s.GiteaContent, error)
	RepositoryCommits(ctx context.Context, repo, branch string, limit int) ([]k8s.GiteaCommit, error)
}

var newGiteaWorkspaceClient = func(namespace string) giteaWorkspaceClient {
	return k8s.NewGiteaClient(namespace)
}

type argoCDWorkspaceClient interface {
	ResourceTree(ctx context.Context, application string) ([]k8s.ArgoCDResource, error)
}

var newArgoCDWorkspaceClient = func(namespace string) argoCDWorkspaceClient {
	return k8s.NewArgoCDClient(namespace)
}

type jenkinsWorkspaceClient interface {
	Jobs(ctx context.Context) ([]k8s.JenkinsJob, error)
	ConsoleText(ctx context.Context, jobName string) (string, error)
}

var newJenkinsWorkspaceClient = func(namespace string) jenkinsWorkspaceClient {
	return k8s.NewJenkinsClient(namespace)
}

type cachedGiteaWorkspace struct {
	repositories []k8s.GiteaRepository
	expiresAt    time.Time
}

var (
	giteaWorkspaceCacheMu sync.Mutex
	giteaWorkspaceCache   = map[string]cachedGiteaWorkspace{}
)

const giteaWorkspaceCacheTTL = 15 * time.Second

type CreateEnvRequest struct {
	Name       string `json:"name" binding:"required"`
	Identifier string `json:"identifier"`
	TemplateID uint   `json:"templateId"`
	FromEmpty  bool   `json:"fromEmpty"`
}

type EnvironmentExternalAccess struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	URL         string `json:"url"`
	Namespace   string `json:"namespace"`
	Scope       string `json:"scope"`
	ServiceID   uint   `json:"serviceId,omitempty"`
	ServiceType string `json:"serviceType,omitempty"`
}

type ServiceCredential struct {
	Secret string `json:"secret"`
	Key    string `json:"key"`
	Value  string `json:"value"`
	Kind   string `json:"kind"`
}

type ComponentView struct {
	model.Component
	RuntimeConfig *k8s.RuntimeConfig `json:"runtimeConfig,omitempty"`
	ExternalURL   string             `json:"externalUrl,omitempty"`
	IngressURL    string             `json:"ingressUrl"`
	NodePortURL   string             `json:"nodePortUrl"`
}

type ServiceInstallationView struct {
	model.ServiceInstallation
	RuntimeConfig      *k8s.RuntimeConfig `json:"runtimeConfig,omitempty"`
	ExternalURL        string             `json:"externalUrl,omitempty"`
	RuntimeServiceName string             `json:"runtimeServiceName,omitempty"`
	RuntimeServiceType string             `json:"runtimeServiceType,omitempty"`
	ClusterIP          string             `json:"clusterIP,omitempty"`
	LoadBalancerIP     string             `json:"loadBalancerIP,omitempty"`
}

type CreateComponentRequest struct {
	Name           string `json:"name" binding:"required"`
	Type           string `json:"type" binding:"required"`
	Image          string `json:"image"`
	Version        string `json:"version"`
	Replicas       int    `json:"replicas"`
	CPU            string `json:"cpu"`
	Memory         string `json:"memory"`
	DeliveryMode   string `json:"deliveryMode"`
	DraftOnly      bool   `json:"draftOnly"`
	SourceRepoURL  string `json:"sourceRepoUrl"`
	SourceBranch   string `json:"sourceBranch"`
	BuildContext   string `json:"buildContext"`
	DockerfilePath string `json:"dockerfilePath"`
}

type UpdateComponentRequest struct {
	Name           string                 `json:"name"`
	Type           string                 `json:"type"`
	Image          string                 `json:"image"`
	Version        string                 `json:"version"`
	Replicas       int                    `json:"replicas"`
	CPU            string                 `json:"cpu"`
	Memory         string                 `json:"memory"`
	DeliveryMode   string                 `json:"deliveryMode"`
	SourceRepoURL  string                 `json:"sourceRepoUrl"`
	SourceBranch   string                 `json:"sourceBranch"`
	BuildContext   string                 `json:"buildContext"`
	DockerfilePath string                 `json:"dockerfilePath"`
	Config         *model.ComponentConfig `json:"config"`
}

type CanvasNodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type CanvasManualEdge struct {
	FromKey string `json:"fromKey"`
	ToKey   string `json:"toKey"`
}

type EnvironmentCanvasStateRequest struct {
	Positions map[string]CanvasNodePosition `json:"positions"`
	Edges     []CanvasManualEdge            `json:"edges"`
	Names     map[string]string             `json:"names"`
}

type AdoptResourceRequest struct {
	Key string `json:"key" binding:"required"`
}

type InstallServiceRequest struct {
	ServiceType string            `json:"serviceType" binding:"required"`
	AppVersion  string            `json:"appVersion"`
	Values      map[string]string `json:"values"`
}

type UpdateServiceRequest struct {
	Values map[string]string `json:"values"`
}

type ServiceExternalAccessRequest struct {
	Enabled bool `json:"enabled"`
}

type WorkspaceActionRequest struct {
	Action string            `json:"action" binding:"required"`
	Target string            `json:"target"`
	Params map[string]string `json:"params"`
}

func serviceToolNamespace(app model.Application, env model.Environment, svcTmpl *model.ServiceTemplate, serviceType string) string {
	toolName := serviceToolIdentity(svcTmpl, serviceType)
	return fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, toolName)
}

func serviceToolIdentity(svcTmpl *model.ServiceTemplate, serviceType string) string {
	if svcTmpl != nil {
		if svcTmpl.PlatformManifestJSON != "" {
			var manifest model.PlatformManifest
			if err := json.Unmarshal([]byte(svcTmpl.PlatformManifestJSON), &manifest); err == nil {
				if name := normalizeIdentifier(manifest.Name, serviceType, 50); name != "" {
					return name
				}
			}
		}
		for _, candidate := range []string{svcTmpl.ChartName, svcTmpl.S3Key, svcTmpl.ChartArchivePath} {
			candidate = strings.TrimSuffix(filepath.Base(strings.TrimSpace(candidate)), ".tar.gz")
			candidate = strings.TrimSuffix(candidate, ".tgz")
			if name := normalizeIdentifier(candidate, serviceType, 50); name != "" {
				return name
			}
		}
	}
	return normalizeIdentifier(serviceType, "tool", 50)
}

func serviceResourceLabels(appIdentifier, envIdentifier string, svcTmpl *model.ServiceTemplate, serviceType string) map[string]string {
	category := "tool"
	if svcTmpl != nil && strings.TrimSpace(svcTmpl.Category) != "" {
		category = strings.TrimSpace(svcTmpl.Category)
	}
	toolName := serviceToolIdentity(svcTmpl, serviceType)
	return map[string]string{
		"paap.io/app":           appIdentifier,
		"paap.io/env":           envIdentifier,
		"paap.io/service":       serviceType,
		"paap.io/service-type":  serviceType,
		"paap.io/tool":          toolName,
		"paap.io/category":      category,
		"paap.io/resource-role": "service-instance",
	}
}

func serviceResourceAnnotations(appIdentifier, envIdentifier string, svcTmpl *model.ServiceTemplate, serviceType string) map[string]string {
	annotations := serviceResourceLabels(appIdentifier, envIdentifier, svcTmpl, serviceType)
	delete(annotations, "paap.io/resource-role")
	return annotations
}

func buildHelmInstallSpec(app *model.Application, env *model.Environment, svcTmpl *model.ServiceTemplate, svcType string, userValues ...map[string]string) *paapv1.HelmInstallSpec {
	releaseName := serviceToolNamespace(*app, *env, svcTmpl, svcType)
	toolNS := serviceToolNamespace(*app, *env, svcTmpl, svcType)
	userValueOverrides := normalizeServiceUserValues(svcType, firstUserValues(userValues...), *svcTmpl)
	selectedTemplate := serviceTemplateForValues(*svcTmpl, svcType, userValueOverrides)

	manifestJSON := selectedTemplate.PlatformManifestJSON
	values := mergeDefaultValues(selectedTemplate.DefaultValues, nil)

	if selectedTemplate.S3Bucket != "" || selectedTemplate.ChartArchivePath != "" {
		primaryNS := app.Identifier + "-" + env.Identifier
		allNS := []string{primaryNS, primaryNS + "-app"}
		platformVars := map[string]string{
			"current_env_name":          env.Identifier,
			"primary_namespace":         primaryNS,
			"all_namespaces":            strings.Join(allNS, ","),
			"env_namespaces":            strings.Join(allNS, ","),
			"workload_namespaces":       primaryNS,
			"paap.envNamespaces":        strings.Join(allNS, ","),
			"global.paap.envNamespaces": strings.Join(allNS, ","),
			"tool_namespace":            toolNS,
		}
		if manifestJSON != "" {
			var manifest model.PlatformManifest
			if err := json.Unmarshal([]byte(manifestJSON), &manifest); err == nil {
				for k, v := range manifest.BuildHelmValues(platformVars) {
					values[k] = v
				}
			}
		}
	}
	if len(userValueOverrides) > 0 {
		for k, v := range userValueOverrides {
			if strings.TrimSpace(k) == "" {
				continue
			}
			values[k] = v
		}
	}

	return &paapv1.HelmInstallSpec{
		ReleaseName:      releaseName,
		Namespace:        toolNS,
		ChartRepo:        selectedTemplate.ChartRepo,
		ChartName:        selectedTemplate.ChartName,
		ChartVersion:     selectedTemplate.ChartVersion,
		ChartArchivePath: selectedTemplate.ChartArchivePath,
		S3Bucket:         selectedTemplate.S3Bucket,
		S3Key:            selectedTemplate.S3Key,
		PresetValues:     selectedTemplate.PresetValues,
		PlatformManifest: manifestJSON,
		Values:           values,
	}
}

func firstUserValues(userValues ...map[string]string) map[string]string {
	if len(userValues) == 0 {
		return nil
	}
	return userValues[0]
}

func normalizeServiceUserValues(svcType string, values map[string]string, templates ...model.ServiceTemplate) map[string]string {
	if len(values) == 0 {
		return nil
	}
	normalizedType := serviceValueProfileKey(svcType, templates...)
	allowed, known := serviceAllowedUserValueKeys(normalizedType)
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if known {
			if _, ok := allowed[key]; !ok {
				continue
			}
		}
		out[key] = strings.TrimSpace(value)
	}

	switch normalizedType {
	case "redis":
		normalizeRedisUserValues(out)
	case "mysql":
		normalizeMySQLUserValues(out)
	case "postgresql":
		normalizePostgreSQLUserValues(out)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func serviceValueProfileKey(svcType string, templates ...model.ServiceTemplate) string {
	candidates := make([]string, 0, 1+len(templates)*5)
	for _, tmpl := range templates {
		candidates = append(candidates,
			tmpl.Name,
			tmpl.ChartName,
			tmpl.S3Key,
			tmpl.ChartArchivePath,
			tmpl.Type,
		)
	}
	candidates = append(candidates, svcType)
	for _, candidate := range candidates {
		if key := normalizeServiceValueProfileKey(candidate); key != "" {
			return key
		}
	}
	return strings.ToLower(strings.TrimSpace(svcType))
}

func normalizeServiceValueProfileKey(value string) string {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return ""
	}
	switch {
	case strings.Contains(text, "harbor"):
		return "harbor"
	case strings.Contains(text, "docker-registry") || strings.Contains(text, "registry.tar.gz") || text == "registry" || strings.Contains(text, "docker registry"):
		return "registry"
	case strings.Contains(text, "gitea") || text == "git":
		return "git"
	case strings.Contains(text, "jenkins") || text == "ci":
		return "ci"
	case strings.Contains(text, "argocd") || strings.Contains(text, "argo cd") || text == "deploy":
		return "deploy"
	case strings.Contains(text, "loki") || strings.Contains(text, "promtail") || text == "log":
		return "log"
	case strings.Contains(text, "prometheus") || strings.Contains(text, "grafana") || strings.Contains(text, "monitor.tar.gz") || text == "monitor":
		return "monitor"
	case strings.Contains(text, "postgres"):
		return "postgresql"
	case strings.Contains(text, "mysql"):
		return "mysql"
	case strings.Contains(text, "mongo"):
		return "mongodb"
	case strings.Contains(text, "redis"):
		return "redis"
	case strings.Contains(text, "rabbit"):
		return "rabbitmq"
	case strings.Contains(text, "kafka"):
		return "kafka"
	case strings.Contains(text, "minio"):
		return "minio"
	}
	return text
}

func serviceAllowedUserValueKeys(profileKey string) (map[string]struct{}, bool) {
	keys := map[string][]string{
		"git":      {"replicaCount", "persistence.enabled", "persistence.size"},
		"ci":       {"controller.numExecutors", "controller.javaOpts", "persistence.enabled", "persistence.size"},
		"deploy":   {"controller.replicas", "server.replicas", "repoServer.replicas", "applicationSet.replicas"},
		"monitor":  {"prometheus.prometheusSpec.replicas", "prometheus.prometheusSpec.shards", "grafana.persistence.enabled", "grafana.persistence.size"},
		"log":      {"loki.replicas", "loki.persistence.enabled", "loki.persistence.size"},
		"registry": {"replicaCount", "persistence.enabled", "persistence.size", "deleteEnabled"},
		"harbor": {
			"core.replicas", "portal.replicas", "registry.replicas", "jobservice.replicas",
			"trivy.enabled", "trivy.replicas", "persistence.enabled",
			"persistence.persistentVolumeClaim.registry.size",
			"persistence.persistentVolumeClaim.database.size",
			"persistence.persistentVolumeClaim.redis.size",
			"persistence.persistentVolumeClaim.jobservice.jobLog.size",
			"persistence.persistentVolumeClaim.trivy.size",
		},
		"redis": {
			"architecture", "replica.replicaCount", "sentinel.enabled", "sentinel.masterSet",
			"cluster.init", "cluster.nodes", "cluster.replicas", "usePassword",
			"persistence.enabled", "persistence.size",
			"master.persistence.enabled", "master.persistence.size",
			"replica.persistence.enabled", "replica.persistence.size",
		},
		"mysql": {
			"architecture", "secondary.replicaCount",
			"primary.persistence.enabled", "primary.persistence.size",
			"secondary.persistence.enabled", "secondary.persistence.size",
			"paap.architecture", "replicaCount", "persistence.enabled", "persistence.size",
		},
		"postgresql": {
			"architecture", "readReplicas.replicaCount",
			"primary.persistence.enabled", "primary.persistence.size",
			"readReplicas.persistence.enabled", "readReplicas.persistence.size",
			"paap.architecture", "postgresql.replicaCount", "pgpool.replicaCount", "persistence.enabled", "persistence.size",
		},
		"mongodb":  {"architecture", "replicaCount", "persistence.enabled", "persistence.size"},
		"rabbitmq": {"replicaCount", "persistence.enabled", "persistence.size"},
		"kafka": {
			"controller.replicaCount", "broker.replicaCount",
			"controller.persistence.enabled", "controller.persistence.size",
			"broker.persistence.enabled", "broker.persistence.size",
		},
		"minio": {"mode", "statefulset.replicaCount", "persistence.enabled", "persistence.size"},
	}
	allowedKeys, ok := keys[profileKey]
	if !ok {
		return nil, false
	}
	allowed := make(map[string]struct{}, len(allowedKeys))
	for _, key := range allowedKeys {
		allowed[key] = struct{}{}
	}
	return allowed, true
}

func normalizeDatabaseUserValues(values map[string]string, replicaPrefix string) {
	if len(values) == 0 {
		return
	}
	architecture := strings.ToLower(strings.TrimSpace(values["architecture"]))
	if architecture != "replication" {
		if _, ok := values["architecture"]; ok || hasUserValuesWithPrefix(values, replicaPrefix) {
			values["architecture"] = "standalone"
		}
		deleteUserValuesWithPrefix(values, replicaPrefix)
		return
	}
	values["architecture"] = "replication"
}

func normalizeMySQLUserValues(values map[string]string) {
	if len(values) == 0 {
		return
	}
	switch selectedDatabaseArchitecture(values) {
	case "dual-master", "galera", "cluster":
		architecture := selectedDatabaseArchitecture(values)
		if architecture == "cluster" {
			architecture = "galera"
		}
		minNodes := 3
		if architecture == "dual-master" {
			minNodes = 2
		}
		values["paap.architecture"] = architecture
		values["replicaCount"] = atLeastIntString(values["replicaCount"], minNodes)
		delete(values, "architecture")
		deleteUserValuesWithPrefix(values, "primary.")
		deleteUserValuesWithPrefix(values, "secondary.")
	case "replication":
		values["architecture"] = "replication"
		delete(values, "paap.architecture")
		delete(values, "replicaCount")
		delete(values, "persistence.enabled")
		delete(values, "persistence.size")
	default:
		if _, ok := values["architecture"]; ok || strings.TrimSpace(values["paap.architecture"]) != "" || hasUserValuesWithPrefix(values, "secondary.") {
			values["architecture"] = "standalone"
		}
		delete(values, "paap.architecture")
		delete(values, "replicaCount")
		delete(values, "persistence.enabled")
		delete(values, "persistence.size")
		deleteUserValuesWithPrefix(values, "secondary.")
	}
}

func normalizePostgreSQLUserValues(values map[string]string) {
	if len(values) == 0 {
		return
	}
	switch selectedDatabaseArchitecture(values) {
	case "ha-cluster", "cluster", "ha":
		values["paap.architecture"] = "ha-cluster"
		values["postgresql.replicaCount"] = atLeastIntString(values["postgresql.replicaCount"], 3)
		values["pgpool.replicaCount"] = atLeastIntString(values["pgpool.replicaCount"], 1)
		delete(values, "architecture")
		deleteUserValuesWithPrefix(values, "primary.")
		deleteUserValuesWithPrefix(values, "readReplicas.")
	case "replication":
		values["architecture"] = "replication"
		delete(values, "paap.architecture")
		delete(values, "postgresql.replicaCount")
		delete(values, "pgpool.replicaCount")
		delete(values, "persistence.enabled")
		delete(values, "persistence.size")
	default:
		if _, ok := values["architecture"]; ok || strings.TrimSpace(values["paap.architecture"]) != "" || hasUserValuesWithPrefix(values, "readReplicas.") {
			values["architecture"] = "standalone"
		}
		delete(values, "paap.architecture")
		delete(values, "postgresql.replicaCount")
		delete(values, "pgpool.replicaCount")
		delete(values, "persistence.enabled")
		delete(values, "persistence.size")
		deleteUserValuesWithPrefix(values, "readReplicas.")
	}
}

func selectedDatabaseArchitecture(values map[string]string) string {
	if values == nil {
		return ""
	}
	if architecture := strings.ToLower(strings.TrimSpace(values["paap.architecture"])); architecture != "" {
		return architecture
	}
	return strings.ToLower(strings.TrimSpace(values["architecture"]))
}

func normalizeRedisUserValues(values map[string]string) {
	if len(values) == 0 {
		return
	}
	switch strings.ToLower(strings.TrimSpace(values["architecture"])) {
	case "cluster":
		values["architecture"] = "cluster"
		values["cluster.init"] = "true"
		values["usePassword"] = "true"
		values["sentinel.enabled"] = "false"
		clusterReplicas := nonNegativeIntString(values["cluster.replicas"], 1)
		minNodes := 3 * (clusterReplicas + 1)
		if minNodes < 3 {
			minNodes = 3
		}
		clusterNodes := nonNegativeIntString(values["cluster.nodes"], minNodes)
		if clusterNodes < minNodes {
			clusterNodes = minNodes
		}
		values["cluster.replicas"] = fmt.Sprintf("%d", clusterReplicas)
		values["cluster.nodes"] = fmt.Sprintf("%d", clusterNodes)
		deleteUserValuesWithPrefix(values, "master.")
		deleteUserValuesWithPrefix(values, "replica.")
	case "sentinel":
		values["architecture"] = "replication"
		values["sentinel.enabled"] = "true"
		if strings.TrimSpace(values["sentinel.masterSet"]) == "" {
			values["sentinel.masterSet"] = "mymaster"
		}
	case "replication":
		values["architecture"] = "replication"
		values["sentinel.enabled"] = "false"
	default:
		if _, ok := values["architecture"]; ok {
			values["architecture"] = "standalone"
		}
		values["sentinel.enabled"] = "false"
		deleteUserValuesWithPrefix(values, "replica.")
	}
}

func hasUserValuesWithPrefix(values map[string]string, prefix string) bool {
	for key := range values {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func deleteUserValuesWithPrefix(values map[string]string, prefix string) {
	for key := range values {
		if strings.HasPrefix(key, prefix) {
			delete(values, key)
		}
	}
}

func nonNegativeIntString(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return fallback
	}
	return parsed
}

func atLeastIntString(value string, minimum int) string {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minimum {
		parsed = minimum
	}
	return fmt.Sprintf("%d", parsed)
}

func serviceTemplateForValues(svcTmpl model.ServiceTemplate, svcType string, values map[string]string) model.ServiceTemplate {
	switch svcType {
	case "redis":
		if strings.ToLower(strings.TrimSpace(values["architecture"])) == "cluster" {
			return serviceTemplateWithBuiltInChart(svcTmpl, "redis-cluster")
		}
	case "mysql":
		switch selectedDatabaseArchitecture(values) {
		case "dual-master", "galera", "cluster":
			return serviceTemplateWithBuiltInChart(svcTmpl, "mysql-galera")
		}
	case "postgresql":
		switch selectedDatabaseArchitecture(values) {
		case "ha-cluster", "cluster", "ha":
			return serviceTemplateWithBuiltInChart(svcTmpl, "postgresql-ha")
		}
	}
	return svcTmpl
}

func serviceTemplateWithBuiltInChart(svcTmpl model.ServiceTemplate, chartName string) model.ServiceTemplate {
	next := svcTmpl
	next.ChartRepo = ""
	next.ChartName = ""
	next.ChartVersion = ""
	next.ChartArchivePath = ""
	next.DefaultValues = ""
	next.S3Bucket = "paap-charts"
	next.S3Key = fmt.Sprintf("charts/%s.tar.gz", chartName)
	next.PresetValues = ""
	if manifestJSON := builtInManifestJSON(chartName); manifestJSON != "" {
		next.PlatformManifestJSON = manifestJSON
	}
	return next
}

// getWorkloadRole returns the RBAC whitelist for a given service type,
// read from the ServiceTemplate's WorkloadRolePolicy field.
// Falls back to no workload permissions so services cannot escape their own namespace
// unless the template explicitly declares workload namespace permissions.
func getWorkloadRole(svcType string) paapv1.RoleSpec {
	var tmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", svcType).First(&tmpl).Error; err != nil {
		return noWorkloadRole()
	}
	if tmpl.WorkloadRolePolicy == "" {
		return noWorkloadRole()
	}
	var rules []paapv1.PolicyRule
	if err := json.Unmarshal([]byte(tmpl.WorkloadRolePolicy), &rules); err != nil {
		return noWorkloadRole()
	}
	return paapv1.RoleSpec{Rules: rules}
}

// getEnvironmentRole returns RBAC rules projected into other namespaces owned
// by the same environment, such as middleware and tool namespaces.
func getEnvironmentRole(svcType string) *paapv1.RoleSpec {
	var tmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", svcType).First(&tmpl).Error; err != nil {
		return nil
	}
	if tmpl.EnvironmentRolePolicy == "" {
		return nil
	}
	var rules []paapv1.PolicyRule
	if err := json.Unmarshal([]byte(tmpl.EnvironmentRolePolicy), &rules); err != nil || len(rules) == 0 {
		return nil
	}
	return &paapv1.RoleSpec{Rules: rules}
}

// getToolNamespaceRole returns the RBAC rules for the tool's own namespace
// (for example, access to CRDs or config resources used by the tool itself).
func getToolNamespaceRole(svcType string) paapv1.RoleSpec {
	var tmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", svcType).First(&tmpl).Error; err != nil {
		return defaultSafeRole()
	}

	if tmpl.PlatformManifestJSON != "" {
		var manifest model.PlatformManifest
		if err := json.Unmarshal([]byte(tmpl.PlatformManifestJSON), &manifest); err == nil {
			var rules []paapv1.PolicyRule
			if err := json.Unmarshal([]byte(manifest.ToToolNamespaceRoleJSON()), &rules); err == nil && len(rules) > 0 {
				return paapv1.RoleSpec{Rules: rules}
			}
		}
	}

	return defaultSafeRole()
}

// getClusterRole returns the cluster-scoped read-only permissions declared by a template.
func getClusterRole(svcType string) *paapv1.RoleSpec {
	var tmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", svcType).First(&tmpl).Error; err != nil {
		return nil
	}
	if tmpl.PlatformManifestJSON == "" {
		return nil
	}
	var manifest model.PlatformManifest
	if err := json.Unmarshal([]byte(tmpl.PlatformManifestJSON), &manifest); err != nil {
		return nil
	}
	var rules []paapv1.PolicyRule
	if err := json.Unmarshal([]byte(manifest.ToClusterRoleJSON()), &rules); err != nil || len(rules) == 0 {
		return nil
	}
	return &paapv1.RoleSpec{Rules: rules}
}

// defaultSafeRole returns a minimal read-only role as a safe fallback.
func defaultSafeRole() paapv1.RoleSpec {
	return paapv1.RoleSpec{
		Rules: []paapv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "endpoints"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}

func noWorkloadRole() paapv1.RoleSpec {
	return paapv1.RoleSpec{Rules: []paapv1.PolicyRule{}}
}

// mergeDefaultValues parses the template's DefaultValues JSON and merges with user overrides.
func mergeDefaultValues(defaultsJSON string, overrides map[string]string) map[string]string {
	result := make(map[string]string)
	if defaultsJSON != "" {
		json.Unmarshal([]byte(defaultsJSON), &result)
	}
	for k, v := range overrides {
		result[k] = v
	}
	return result
}

func serviceValuesJSON(values map[string]string) string {
	if len(values) == 0 {
		return "{}"
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// buildContextValues builds platform context variables that are injected into
// every custom template's Helm values. Users can reference these in their charts
// via {{ .Values.global.envNamespaces }} etc.
func buildContextValues(envNS string, allNamespaces []string, envIdentifier string) map[string]string {
	return map[string]string{
		"global.platformManaged":  "true",
		"global.environmentId":    envIdentifier,
		"global.envNamespaces":    strings.Join(allNamespaces, ","),
		"global.primaryNamespace": envNS,
	}
}

// installCustomChart extracts a custom template's chart archive and installs it via Helm.
// Values merge order (later wins):
//  1. preset-values.yaml (disable built-in RBAC, set SA name, etc.)
//  2. platform context (global.envNamespaces, global.environmentId, etc.)
//  3. variable_mapping from platform-manifest (custom mappings)
//  4. user parameters (highest priority, not yet implemented)
func installCustomChart(releaseName, toolNS, chartArchivePath, s3Bucket, s3Key, platformManifestJSON, presetValues string, envIdentifier, primaryNS string, allNamespaces []string, extraValues map[string]string) error {
	tmpDir, err := os.MkdirTemp("", "paap-chart-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var localChartPath string
	if s3Bucket != "" && s3Key != "" {
		// Download from S3
		s3Client, err := getOrCreateS3Client()
		if err != nil {
			return fmt.Errorf("failed to create S3 client: %w", err)
		}
		localPath := filepath.Join(tmpDir, "chart.tar.gz")
		if err := s3Client.DownloadFile(context.Background(), s3Key, localPath); err != nil {
			return fmt.Errorf("failed to download chart from S3: %w", err)
		}
		localChartPath = localPath
	} else {
		// Use local file
		localChartPath = chartArchivePath
	}

	if err := extractTarGz(localChartPath, tmpDir); err != nil {
		return fmt.Errorf("failed to extract chart: %w", err)
	}

	chartPath := filepath.Join(tmpDir, "chart")

	// Layer 1: Start with preset-values (e.g. rbac.create=false)
	values := make(map[string]string)

	// First try to read preset-values.yaml from the extracted chart
	presetValuesPath := filepath.Join(tmpDir, "preset-values.yaml")
	if presetValuesData, err := os.ReadFile(presetValuesPath); err == nil {
		log.Printf("[installCustomChart] Read preset-values.yaml: %s", string(presetValuesData))
		values = parseYAMLToMap(string(presetValuesData))
		log.Printf("[installCustomChart] Parsed values: %+v", values)
	} else {
		log.Printf("[installCustomChart] preset-values.yaml not found: %v", err)
		if presetValues != "" {
			// Fallback to database field if file doesn't exist
			values = parseYAMLToMap(presetValues)
		}
	}

	// Layer 2: Add platform context
	for k, v := range buildContextValues(primaryNS, allNamespaces, envIdentifier) {
		values[k] = v
	}

	// Layer 3: Apply variable_mapping from platform-manifest
	if platformManifestJSON != "" {
		var manifest model.PlatformManifest
		if err := json.Unmarshal([]byte(platformManifestJSON), &manifest); err == nil {
			platformVars := map[string]string{
				"current_env_name":    envIdentifier,
				"primary_namespace":   primaryNS,
				"all_namespaces":      strings.Join(allNamespaces, ","),
				"env_namespaces":      strings.Join(allNamespaces, ","),
				"workload_namespaces": primaryNS,
				"tool_namespace":      toolNS,
			}
			for k, v := range manifest.BuildHelmValues(platformVars) {
				values[k] = v
			}
		}
	}

	// Layer 4: Apply extra values (e.g. skip CRDs for ArgoCD)
	for k, v := range extraValues {
		values[k] = v
	}

	if err := k8sClient.InstallHelmChart(releaseName, toolNS, chartPath, values); err != nil {
		return err
	}

	// Provision Grafana dashboards asynchronously
	dashboardsDir := filepath.Join(tmpDir, "dashboards")
	go provisionDashboards(toolNS, dashboardsDir)

	return nil
}

// parseYAMLToMap parses a simple flat YAML string into a map[string]string.
// Only handles top-level key: value pairs (not nested structures).
// Nested keys use dot notation: "rbac.create: false" → {"rbac.create": "false"}
func parseYAMLToMap(yamlContent string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(yamlContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Remove quotes
		val = strings.Trim(val, `"'`)
		if key != "" && val != "" {
			result[key] = val
		}
	}
	return result
}

// provisionDashboards waits for Grafana to be ready and imports all dashboard JSON files.
// Runs as a goroutine with retry logic since Grafana may take time to start.
func provisionDashboards(grafanaNS, dashboardsDir string) {
	if _, err := os.Stat(dashboardsDir); os.IsNotExist(err) {
		return // No dashboards directory
	}

	entries, err := os.ReadDir(dashboardsDir)
	if err != nil || len(entries) == 0 {
		return
	}

	grafana := k8s.NewGrafanaClient(grafanaNS)

	// Wait up to 2 minutes for Grafana to be ready
	for i := 0; i < 24; i++ {
		if err := grafana.HealthCheck(); err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dashboardsDir, entry.Name()))
		if err != nil {
			log.Printf("[provisionDashboards] failed to read %s: %v", entry.Name(), err)
			continue
		}
		if err := grafana.ImportDashboard(string(content), entry.Name()); err != nil {
			log.Printf("[provisionDashboards] failed to import dashboard %s: %v", entry.Name(), err)
		} else {
			log.Printf("[provisionDashboards] imported dashboard: %s", entry.Name())
		}
	}
}

// extractTarGz extracts a tar.gz archive to the target directory.
func extractTarGz(archivePath, targetDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(targetDir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}
			io.Copy(outFile, tr)
			outFile.Close()
		}
	}
	return nil
}

// ListApplicationEnvironments returns environments for an application
func ListApplicationEnvironments(c *gin.Context) {
	syncClusterStateIfPossible()

	appID, _ := strconv.Atoi(c.Param("id"))
	var app model.Application
	if err := database.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	if !requireApplicationAccess(c, app.ID) {
		return
	}

	var envs []model.Environment
	if err := database.DB.Where("application_id = ?", app.ID).Find(&envs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": envs})
}

// CreateEnvironment creates a new environment for an application.
// Every environment gets the foundation toolchain. Templates add business infra on top.
func CreateEnvironment(c *gin.Context) {
	appID, _ := strconv.Atoi(c.Param("id"))
	var req CreateEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 查出应用信息
	var app model.Application
	if err := database.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	if !requireApplicationAccess(c, app.ID) {
		return
	}
	identifier, err := uniqueIdentifierWithFallback(database.DB, firstNonEmpty(req.Identifier, req.Name), "env", 50, func(db *gorm.DB, candidate string) (bool, error) {
		var count int64
		if err := db.Model(&model.Environment{}).Where("application_id = ? AND identifier = ?", appID, candidate).Count(&count).Error; err != nil {
			return false, err
		}
		return count > 0, nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	env := model.Environment{
		ApplicationID: uint(appID),
		Name:          req.Name,
		Identifier:    identifier,
		TemplateID:    req.TemplateID,
		Status:        "creating",
		Namespace:     app.Identifier + "-" + identifier,
	}

	if req.FromEmpty {
		env.TemplateID = 0
	}

	if err := database.DB.Create(&env).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 1) 创建 K8s Environment CR（Operator 会自动创建 namespace + NetworkPolicy + Quota）
	ctx := context.Background()
	primaryNS := app.Identifier + "-" + identifier
	additionalNS := []paapv1.AdditionalNamespace{
		{Suffix: "app", Purpose: "workload"},
	}
	if err := k8s.CreateEnvironmentCR(ctx, app.Identifier, req.Name, identifier, primaryNS, additionalNS); err != nil {
		database.DB.Model(&env).Update("status", "error").Update("error_message", err.Error())
		c.JSON(http.StatusCreated, gin.H{"data": env, "warning": "Environment CR creation failed: " + err.Error()})
		return
	}

	// 2) 安装环境基座；模板只是在基座之外追加数据库、中间件等。
	go installEnvironmentServices(&env, &app, identifier, env.TemplateID)
	env.Status = "creating"
	database.DB.Model(&env).Update("status", "creating")

	c.JSON(http.StatusCreated, gin.H{"data": env})
}

func foundationServiceTypes() []string {
	return []string{"git", "registry", "deploy", "monitor", "log"}
}

func installEnvironmentServices(env *model.Environment, app *model.Application, envIdentifier string, templateID uint) {
	log.Printf("[installEnvironmentServices] Starting for env=%s templateID=%d", envIdentifier, templateID)
	services := foundationServiceTypes()
	if templateID > 0 {
		var tmpl model.EnvTemplate
		if err := database.DB.First(&tmpl, templateID).Error; err != nil {
			log.Printf("[installEnvironmentServices] Template %d not found: %v", templateID, err)
			database.DB.Model(env).Update("status", "error").Update("error_message", "template not found")
			return
		}
		log.Printf("[installEnvironmentServices] Found template: name=%s services=%s infra=%s", tmpl.Name, tmpl.Services, tmpl.Infra)
		services = appendServiceTypes(services, templateInstallServiceTypes(tmpl)...)
	}
	installServiceTypes(env, app, envIdentifier, services)
}

// installTemplateServices creates ServiceInstance CRs for foundation and template services.
func installTemplateServices(env *model.Environment, app *model.Application, envIdentifier string, templateID uint) {
	installEnvironmentServices(env, app, envIdentifier, templateID)
}

func appendServiceTypes(base []string, extra ...string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(base)+len(extra))
	for _, value := range append(base, extra...) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func installServiceTypes(env *model.Environment, app *model.Application, envIdentifier string, services []string) {
	ctx := context.Background()
	services = appendServiceTypes(nil, services...)
	log.Printf("[installServiceTypes] Will install %d services: %v", len(services), services)
	for _, svc := range services {
		// 查找模板获取安装方式
		var svcTmpl model.ServiceTemplate
		if err := database.DB.Where("type = ?", svc).First(&svcTmpl).Error; err != nil {
			log.Printf("[installServiceTypes] template for service %s not found: %v", svc, err)
			continue
		}
		toolNS := serviceToolNamespace(*app, *env, &svcTmpl, svc)
		inst := model.ServiceInstallation{
			EnvironmentID: env.ID,
			ServiceType:   svc,
			ServiceName:   serviceToolIdentity(&svcTmpl, svc),
			Status:        "installing",
			Namespace:     toolNS,
			ReleaseName:   toolNS,
		}
		database.DB.Create(&inst)
		log.Printf("[installServiceTypes] Service %s: installer=%s s3Bucket=%s s3Key=%s chartRepo=%s",
			svc, svcTmpl.Installer, svcTmpl.S3Bucket, svcTmpl.S3Key, svcTmpl.ChartRepo)

		if svcTmpl.Installer != "helm" {
			inst.Status = "failed"
			inst.ErrorMessage = "only helm service templates are supported"
			database.DB.Save(&inst)
			continue
		}

		workloadRole := getWorkloadRole(svc)
		toolNamespaceRole := getToolNamespaceRole(svc)
		environmentRole := getEnvironmentRole(svc)
		clusterRole := getClusterRole(svc)
		helmSpec := buildHelmInstallSpec(app, env, &svcTmpl, svc)
		resourceLabels := serviceResourceLabels(app.Identifier, envIdentifier, &svcTmpl, svc)
		resourceAnnotations := serviceResourceAnnotations(app.Identifier, envIdentifier, &svcTmpl, svc)

		if err := k8s.CreateServiceInstanceCR(ctx, app.Identifier, envIdentifier, svc, workloadRole, toolNamespaceRole, environmentRole, clusterRole, nil, helmSpec, resourceLabels, resourceAnnotations); err != nil {
			inst.Status = "failed"
			inst.ErrorMessage = err.Error()
			database.DB.Save(&inst)
		} else {
			inst.Status = "installing"
			database.DB.Save(&inst)
		}
	}

	database.DB.Model(env).Update("status", "running")
}

func templateInstallServiceTypes(tmpl model.EnvTemplate) []string {
	seen := map[string]bool{}
	result := make([]string, 0)
	appendList := func(raw, field string) {
		var values []string
		if strings.TrimSpace(raw) == "" {
			return
		}
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			log.Printf("[installTemplateServices] Failed to unmarshal %s: %v", field, err)
			return
		}
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			result = append(result, value)
		}
	}
	appendList(tmpl.Services, "services")
	appendList(tmpl.Infra, "infra")
	return result
}

// renderAndStoreManifests renders a service template and stores it in a ConfigMap
func renderAndStoreManifests(ctx context.Context, crNS, svcType string, app *model.Application, env *model.Environment, envIdentifier, toolNS string, parameters map[string]string) *paapv1.ConfigMapReference {
	// 查找 ServiceTemplate
	var svcTmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", svcType).First(&svcTmpl).Error; err != nil {
		log.Printf("[renderAndStoreManifests] template not found for type %s: %v", svcType, err)
		return nil
	}

	if svcTmpl.RawYamlTemplate == "" {
		log.Printf("[renderAndStoreManifests] template %s has no rawYamlTemplate", svcType)
		return nil
	}

	// 合并默认参数
	defaultParams := make(map[string]string)
	if svcTmpl.DefaultValues != "" {
		json.Unmarshal([]byte(svcTmpl.DefaultValues), &defaultParams)
	}
	for k, v := range parameters {
		defaultParams[k] = v
	}

	// 构建 namespace 列表（主空间 + 附加空间）
	primaryNS := app.Identifier + "-" + envIdentifier
	namespaces := []string{primaryNS}
	// 默认附加 namespace: {primaryNS}-app (工作负载空间)
	additionalNS := primaryNS + "-app"
	namespaces = append(namespaces, additionalNS)

	// 渲染模板
	renderer := service.NewTemplateRenderer()
	vars := service.BuildVariables(
		app.ID, app.Name, app.Identifier,
		env.ID, env.Name, envIdentifier,
		primaryNS, toolNS, namespaces,
		fmt.Sprintf("%s-%s-%s", app.Identifier, envIdentifier, svcType),
		toolNS,
		defaultParams,
		service.RoleRules{},
	)

	rendered, err := renderer.RenderServiceTemplate(svcTmpl.RawYamlTemplate, vars)
	if err != nil {
		log.Printf("[renderAndStoreManifests] render error for %s: %v", svcType, err)
		return nil
	}

	log.Printf("[renderAndStoreManifests] rendered %d bytes for %s", len(rendered), svcType)

	// 创建 ConfigMap 存储渲染后的 manifests
	cmName := fmt.Sprintf("%s-%s-manifests", envIdentifier, svcType)
	cmLabels := map[string]string{
		"paap.io/app":  app.Identifier,
		"paap.io/env":  envIdentifier,
		"paap.io/tool": svcType,
		"paap.io/type": "manifests",
	}
	if err := k8s.CreateConfigMap(ctx, crNS, cmName, map[string]string{"manifests.yaml": rendered}, cmLabels); err != nil {
		log.Printf("[renderAndStoreManifests] create configmap error: %v", err)
		return nil
	}

	log.Printf("[renderAndStoreManifests] created ConfigMap %s/%s", crNS, cmName)
	return &paapv1.ConfigMapReference{
		Name:      cmName,
		Namespace: crNS,
	}
}

// GetEnvironment returns environment details with components, services, infra
func GetEnvironment(c *gin.Context) {
	syncClusterStateNow()

	id, _ := strconv.Atoi(c.Param("id"))
	var env model.Environment
	if err := database.DB.First(&env, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}

	var components []model.Component
	database.DB.Where("environment_id = ?", env.ID).Find(&components)
	var services []model.ServiceInstallation
	database.DB.Where("environment_id = ?", env.ID).Find(&services)
	var infra []model.InfraInstallation
	database.DB.Where("environment_id = ?", env.ID).Find(&infra)
	ctx := context.Background()
	access := collectEnvironmentExternalAccess(ctx, env)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"environment":    env,
			"components":     enrichComponentViews(ctx, env, components, access),
			"services":       enrichServiceInstallationViews(ctx, services, access),
			"infra":          infra,
			"externalAccess": access,
		},
	})
}

func GetEnvironmentCanvasState(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	if envID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return
	}
	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}
	var state model.EnvironmentCanvasState
	if err := database.DB.Where("environment_id = ?", env.ID).First(&state).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"data": gin.H{"positions": gin.H{}, "edges": []CanvasManualEdge{}, "names": gin.H{}}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	positions, edges := cleanCanvasStateForEnvironment(env.ID, valueOrDefaultString(state.Positions, "{}"), valueOrDefaultString(state.Edges, "[]"))
	if positions != state.Positions || edges != state.Edges {
		state.Positions = positions
		state.Edges = edges
		_ = database.DB.Save(&state).Error
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"positions": json.RawMessage(positions),
		"edges":     json.RawMessage(edges),
		"names":     json.RawMessage(valueOrDefaultString(state.Names, "{}")),
	}})
}

func SaveEnvironmentCanvasState(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	if envID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid environment id"})
		return
	}
	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}
	var req EnvironmentCanvasStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	positionsJSON, err := json.Marshal(normalizeCanvasPositions(req.Positions))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid canvas positions"})
		return
	}
	edgesJSON, err := json.Marshal(normalizeCanvasEdges(req.Edges))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid canvas edges"})
		return
	}
	namesJSON, err := json.Marshal(normalizeCanvasNames(req.Names))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid canvas names"})
		return
	}
	var state model.EnvironmentCanvasState
	err = database.DB.Where("environment_id = ?", env.ID).First(&state).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	state.EnvironmentID = env.ID
	state.Positions = string(positionsJSON)
	state.Edges = string(edgesJSON)
	state.Names = string(namesJSON)
	if state.ID == 0 {
		err = database.DB.Create(&state).Error
	} else {
		err = database.DB.Save(&state).Error
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"positions": json.RawMessage(state.Positions),
		"edges":     json.RawMessage(state.Edges),
		"names":     json.RawMessage(state.Names),
	}})
}

func normalizeCanvasPositions(input map[string]CanvasNodePosition) map[string]CanvasNodePosition {
	out := map[string]CanvasNodePosition{}
	for key, pos := range input {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = CanvasNodePosition{X: pos.X, Y: pos.Y}
	}
	return out
}

func cleanCanvasStateForEnvironment(envID uint, positionsJSON, edgesJSON string) (string, string) {
	validKeys := currentCanvasNodeKeys(envID)
	var positions map[string]CanvasNodePosition
	if err := json.Unmarshal([]byte(valueOrDefaultString(positionsJSON, "{}")), &positions); err != nil {
		positions = map[string]CanvasNodePosition{}
	}
	cleanPositions := map[string]CanvasNodePosition{}
	for key, pos := range normalizeCanvasPositions(positions) {
		if validKeys[key] {
			cleanPositions[key] = pos
		}
	}

	var edges []CanvasManualEdge
	if err := json.Unmarshal([]byte(valueOrDefaultString(edgesJSON, "[]")), &edges); err != nil {
		edges = nil
	}
	cleanEdges := make([]CanvasManualEdge, 0, len(edges))
	for _, edge := range normalizeCanvasEdges(edges) {
		if validKeys[edge.FromKey] && validKeys[edge.ToKey] {
			cleanEdges = append(cleanEdges, edge)
		}
	}

	positionsBytes, err := json.Marshal(cleanPositions)
	if err != nil {
		positionsBytes = []byte("{}")
	}
	edgesBytes, err := json.Marshal(cleanEdges)
	if err != nil {
		edgesBytes = []byte("[]")
	}
	return string(positionsBytes), string(edgesBytes)
}

func currentCanvasNodeKeys(envID uint) map[string]bool {
	keys := map[string]bool{}
	var components []model.Component
	if err := database.DB.Where("environment_id = ?", envID).Find(&components).Error; err == nil {
		for _, comp := range components {
			keys[fmt.Sprintf("component:%d", comp.ID)] = true
		}
	}
	var services []model.ServiceInstallation
	if err := database.DB.Where("environment_id = ?", envID).Find(&services).Error; err == nil {
		for _, svc := range services {
			keys[fmt.Sprintf("service:%d", svc.ID)] = true
		}
	}
	var infra []model.InfraInstallation
	if err := database.DB.Where("environment_id = ?", envID).Find(&infra).Error; err == nil {
		for _, item := range infra {
			keys[fmt.Sprintf("infra:%d", item.ID)] = true
		}
	}
	return keys
}

func normalizeCanvasEdges(input []CanvasManualEdge) []CanvasManualEdge {
	seen := map[string]struct{}{}
	out := make([]CanvasManualEdge, 0, len(input))
	for _, edge := range input {
		from := strings.TrimSpace(edge.FromKey)
		to := strings.TrimSpace(edge.ToKey)
		if from == "" || to == "" || from == to {
			continue
		}
		key := from + "\x00" + to
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, CanvasManualEdge{FromKey: from, ToKey: to})
	}
	return out
}

func normalizeCanvasNames(input map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func collectEnvironmentExternalAccess(ctx context.Context, env model.Environment) []EnvironmentExternalAccess {
	access := make([]EnvironmentExternalAccess, 0)
	seenURLs := map[string]struct{}{}

	appendNamespace := func(namespace, scope string) {
		namespace = strings.TrimSpace(namespace)
		if namespace == "" {
			return
		}
		endpoints, err := k8s.ListNamespaceExternalEndpoints(ctx, namespace)
		if err != nil {
			log.Printf("[GetEnvironment] list external access failed namespace=%s: %v", namespace, err)
			return
		}
		for _, endpoint := range endpoints {
			url := strings.TrimSpace(endpoint.URL)
			if url == "" {
				continue
			}
			key := namespace + "\x00" + url
			if _, ok := seenURLs[key]; ok {
				continue
			}
			seenURLs[key] = struct{}{}
			item := EnvironmentExternalAccess{
				Name:      endpoint.Name,
				Kind:      endpoint.Kind,
				URL:       url,
				Namespace: namespace,
				Scope:     scope,
			}
			access = append(access, item)
		}
	}

	appendNamespace(env.Namespace, "environment")
	var services []model.ServiceInstallation
	_ = database.DB.Where("environment_id = ?", env.ID).Find(&services).Error
	for _, svc := range services {
		before := len(access)
		appendNamespace(svc.Namespace, "service")
		for i := before; i < len(access); i++ {
			access[i].ServiceID = svc.ID
			access[i].ServiceType = svc.ServiceType
		}
	}

	sort.Slice(access, func(i, j int) bool {
		if access[i].Scope != access[j].Scope {
			return access[i].Scope < access[j].Scope
		}
		if access[i].Namespace != access[j].Namespace {
			return access[i].Namespace < access[j].Namespace
		}
		if access[i].ServiceType != access[j].ServiceType {
			return access[i].ServiceType < access[j].ServiceType
		}
		if access[i].Name != access[j].Name {
			return access[i].Name < access[j].Name
		}
		return access[i].URL < access[j].URL
	})
	return access
}

func enrichComponentViews(ctx context.Context, env model.Environment, components []model.Component, access []EnvironmentExternalAccess) []ComponentView {
	views := make([]ComponentView, 0, len(components))
	for _, comp := range components {
		view := ComponentView{Component: comp}
		identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
		if cfg, err := k8s.DiscoverComponentRuntimeConfig(ctx, env.Namespace, identifier); err == nil {
			view.RuntimeConfig = cfg
		} else if err != nil {
			log.Printf("[GetEnvironment] discover component runtime config failed component=%s namespace=%s: %v", identifier, env.Namespace, err)
		}
		view.ExternalURL = componentExternalURL(env.ID, comp, identifier, access, view.RuntimeConfig)
		view.IngressURL = componentIngressURL(identifier, access)
		view.NodePortURL = componentNodePortURL(identifier, access)
		views = append(views, view)
	}
	return views
}

func enrichServiceInstallationViews(ctx context.Context, services []model.ServiceInstallation, access []EnvironmentExternalAccess) []ServiceInstallationView {
	views := make([]ServiceInstallationView, 0, len(services))
	for _, inst := range services {
		view := ServiceInstallationView{ServiceInstallation: inst}
		if cfg, err := k8s.DiscoverNamespaceRuntimeConfig(ctx, inst.Namespace); err == nil {
			view.RuntimeConfig = cfg
		} else if err != nil {
			log.Printf("[GetEnvironment] discover service runtime config failed service=%s namespace=%s: %v", inst.ServiceType, inst.Namespace, err)
		}
		if network, err := k8s.DiscoverNamespaceServiceNetwork(ctx, inst.Namespace, inst.ServiceType); err == nil && network != nil {
			view.RuntimeServiceName = network.ServiceName
			view.RuntimeServiceType = network.ServiceType
			view.ClusterIP = network.ClusterIP
			view.LoadBalancerIP = network.LoadBalancerIP
		} else if err != nil {
			log.Printf("[GetEnvironment] discover service network failed service=%s namespace=%s: %v", inst.ServiceType, inst.Namespace, err)
		}
		view.ExternalURL = serviceExternalURL(inst, access)
		views = append(views, view)
	}
	return views
}

func componentIngressURL(identifier string, access []EnvironmentExternalAccess) string {
	ingressName := "comp-" + identifier
	for _, item := range access {
		if item.Kind == "Ingress" && (item.Name == ingressName || strings.HasPrefix(item.Name, identifier+"-")) {
			return item.URL
		}
	}
	return ""
}

func componentNodePortURL(identifier string, access []EnvironmentExternalAccess) string {
	for _, item := range access {
		if item.Kind == "NodePort" && strings.Contains(item.Name, identifier) {
			return item.URL
		}
	}
	return ""
}

func componentExternalURL(envID uint, comp model.Component, identifier string, access []EnvironmentExternalAccess, runtime *k8s.RuntimeConfig) string {
	names := []string{identifier, comp.Name}
	matches := make([]EnvironmentExternalAccess, 0)
	for _, item := range access {
		if item.Scope == "service" {
			continue
		}
		for _, name := range names {
			if name != "" && strings.Contains(strings.ToLower(item.Name), strings.ToLower(name)) {
				matches = append(matches, item)
				break
			}
		}
	}
	if len(matches) > 0 {
		sort.SliceStable(matches, func(i, j int) bool {
			if externalAccessKindPriority(matches[i].Kind) != externalAccessKindPriority(matches[j].Kind) {
				return externalAccessKindPriority(matches[i].Kind) < externalAccessKindPriority(matches[j].Kind)
			}
			return matches[i].URL < matches[j].URL
		})
		return matches[0].URL
	}
	if strings.EqualFold(comp.Type, "frontend") {
		for _, item := range access {
			if item.Scope != "service" && strings.TrimSpace(item.URL) != "" {
				return item.URL
			}
		}
		if runtime != nil && strings.TrimSpace(runtime.WorkloadName) != "" {
			if proxy := componentProxyURL(envID, comp, ""); proxy != "" {
				return proxy
			}
		}
	}
	return ""
}

func externalAccessKindPriority(kind string) int {
	switch strings.TrimSpace(kind) {
	case "Gateway":
		return 0
	case "Ingress":
		return 1
	case "LoadBalancer":
		return 2
	case "NodePort":
		return 3
	default:
		return 9
	}
}

func serviceExternalURL(inst model.ServiceInstallation, access []EnvironmentExternalAccess) string {
	// 对于日志服务，返回Grafana的Loki Explore URL
	if inst.ServiceType == "log" {
		if grafanaInst, ok := environmentServiceByType(inst.EnvironmentID, "monitor"); ok {
			if grafanaURL := serviceExternalURL(grafanaInst, access); grafanaURL != "" {
				return grafanaURL
			}
		}
	}

	for _, item := range access {
		if item.Scope == "service" && item.ServiceID == inst.ID && strings.TrimSpace(item.URL) != "" {
			return item.URL
		}
	}
	for _, item := range access {
		if item.Scope == "service" && item.Namespace == inst.Namespace && strings.TrimSpace(item.URL) != "" {
			return item.URL
		}
	}
	return ""
}

func loadEnvironmentAndApp(c *gin.Context, envID uint) (model.Environment, model.Application, bool) {
	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return model.Environment{}, model.Application{}, false
	}
	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return model.Environment{}, model.Application{}, false
	}
	if !requireApplicationAccess(c, app.ID) {
		return model.Environment{}, model.Application{}, false
	}
	return env, app, true
}

func discoverAdoptableResourcesForEnvironment(ctx context.Context, app model.Application, env model.Environment) ([]k8s.AdoptableResource, error) {
	namespaces := adoptableEnvironmentNamespaces(env)
	var services []model.ServiceInstallation
	_ = database.DB.Where("environment_id = ?", env.ID).Find(&services).Error

	managed := managedAdoptableResourceKeys(env, services)
	out := make([]k8s.AdoptableResource, 0)
	seen := map[string]struct{}{}
	for _, namespace := range uniqueStrings(namespaces) {
		items, err := k8s.ListNamespaceAdoptableResources(ctx, namespace)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			if !adoptableResourceBelongsToEnvironment(item, app, env) {
				continue
			}
			if _, exists := managed[item.Key]; exists {
				continue
			}
			if _, exists := seen[item.Key]; exists {
				continue
			}
			seen[item.Key] = struct{}{}
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func adoptableEnvironmentNamespaces(env model.Environment) []string {
	base := strings.TrimSpace(env.Namespace)
	if base == "" {
		return nil
	}
	return []string{base, base + "-app"}
}

func managedAdoptableResourceKeys(env model.Environment, services []model.ServiceInstallation) map[string]struct{} {
	managed := map[string]struct{}{}
	var components []model.Component
	_ = database.DB.Where("environment_id = ?", env.ID).Find(&components).Error
	for _, comp := range components {
		identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
		for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet"} {
			managed[strings.ToLower(env.Namespace+"/"+kind+"/"+identifier)] = struct{}{}
			managed[strings.ToLower(env.Namespace+"/"+kind+"/"+comp.Name)] = struct{}{}
		}
	}
	for _, inst := range services {
		name := strings.TrimSpace(inst.ReleaseName)
		if name == "" {
			name = strings.TrimSpace(inst.ServiceName)
		}
		if name == "" {
			name = strings.TrimSpace(inst.ServiceType)
		}
		for _, kind := range []string{"Deployment", "StatefulSet", "DaemonSet"} {
			managed[strings.ToLower(inst.Namespace+"/"+kind+"/"+name)] = struct{}{}
			managed[strings.ToLower(inst.Namespace+"/"+kind+"/"+inst.ServiceType)] = struct{}{}
		}
	}
	return managed
}

func adoptableResourceBelongsToEnvironment(item k8s.AdoptableResource, app model.Application, env model.Environment) bool {
	namespace := strings.TrimSpace(item.Namespace)
	if namespace == "" {
		return false
	}
	base := strings.TrimSpace(env.Namespace)
	if base != "" && (namespace == base || strings.HasPrefix(namespace, base+"-")) {
		return true
	}
	prefix := strings.Trim(strings.TrimSpace(app.Identifier)+"-"+strings.TrimSpace(env.Identifier), "-")
	return prefix != "" && (namespace == prefix || strings.HasPrefix(namespace, prefix+"-"))
}

func componentFromAdoptableResource(env model.Environment, item k8s.AdoptableResource) (model.Component, error) {
	cfg := componentConfigFromRuntimeConfig(item.RuntimeConfig)
	configJSON, err := cfg.JSON()
	if err != nil {
		return model.Component{}, err
	}
	replicas := int32(1)
	if item.RuntimeConfig != nil && item.RuntimeConfig.Replicas != nil {
		replicas = *item.RuntimeConfig.Replicas
	}
	if replicas < 0 {
		replicas = 0
	}
	comp := model.Component{
		EnvironmentID: env.ID,
		Name:          item.Name,
		Type:          valueOrDefaultString(item.ComponentType, "backend"),
		Image:         runtimeImageOrDescription(item),
		Version:       imageReferenceTag(runtimeImageOrDescription(item)),
		Replicas:      int(replicas),
		Status:        "draft",
		DeliveryMode:  "image",
		Config:        configJSON,
	}
	if comp.Replicas == 0 && !strings.EqualFold(item.Status, "stopped") {
		comp.Replicas = 1
	}
	if item.RuntimeConfig != nil {
		comp.CPU = item.RuntimeConfig.Resources.Requests["cpu"]
		comp.Memory = item.RuntimeConfig.Resources.Requests["memory"]
	}
	return comp, nil
}

func componentConfigFromRuntimeConfig(cfg *k8s.RuntimeConfig) model.ComponentConfig {
	if cfg == nil {
		return model.ComponentConfig{}
	}
	out := model.ComponentConfig{
		Command: append([]string{}, cfg.Command...),
		Args:    append([]string{}, cfg.Args...),
		Env:     make([]model.ComponentEnvVar, 0, len(cfg.Env)),
		Files:   make([]model.ComponentConfigFile, 0, len(cfg.Files)),
	}
	if len(cfg.Ports) > 0 {
		out.ContainerPort = cfg.Ports[0]
	}
	for _, env := range cfg.Env {
		out.Env = append(out.Env, model.ComponentEnvVar{
			Name:          env.Name,
			Value:         env.Value,
			SecretName:    env.SecretName,
			SecretKey:     env.SecretKey,
			ConfigMapName: env.ConfigMapName,
			ConfigMapKey:  env.ConfigMapKey,
		})
	}
	for _, file := range cfg.Files {
		if file.Kind != "configMap" || strings.TrimSpace(file.ObjectName) == "" || strings.TrimSpace(file.Key) == "" || strings.TrimSpace(file.MountPath) == "" {
			continue
		}
		out.Files = append(out.Files, model.ComponentConfigFile{
			Name:          strings.Trim(strings.TrimSpace(file.ObjectName)+"-"+strings.TrimSpace(file.Key), "-"),
			ConfigMapName: strings.TrimSpace(file.ObjectName),
			Key:           strings.TrimSpace(file.Key),
			MountPath:     strings.TrimSpace(file.MountPath),
			ReadOnly:      true,
		})
	}
	return out
}

func runtimeImageOrDescription(item k8s.AdoptableResource) string {
	if item.RuntimeConfig != nil && strings.TrimSpace(item.RuntimeConfig.Image) != "" {
		return strings.TrimSpace(item.RuntimeConfig.Image)
	}
	return strings.TrimSpace(item.Description)
}

func uniqueStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

// DeleteEnvironment deletes an environment and its K8s CR
func DeleteEnvironment(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	var env model.Environment
	if err := database.DB.First(&env, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}

	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err == nil {
		ctx := context.Background()
		if err := k8s.DeleteEnvironmentScopedResources(ctx, app.Identifier, env.Identifier); err != nil {
			c.Header("X-CR-Warning", "Environment cluster cleanup failed: "+err.Error())
		}
	}

	// 删除数据库记录（硬删除，避免唯一约束冲突）
	database.DB.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.ServiceInstallation{})
	database.DB.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.InfraInstallation{})
	database.DB.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.Component{})
	database.DB.Unscoped().Where("environment_id = ?", env.ID).Delete(&model.EnvironmentCanvasState{})
	database.DB.Unscoped().Delete(&env)

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListEnvironmentComponents returns components in an environment
func ListEnvironmentComponents(c *gin.Context) {
	syncClusterStateNow()

	envID, _ := strconv.Atoi(c.Param("id"))
	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}
	var components []model.Component
	if err := database.DB.Where("environment_id = ?", env.ID).Find(&components).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := context.Background()
	c.JSON(http.StatusOK, gin.H{"data": enrichComponentViews(ctx, env, components, collectEnvironmentExternalAccess(ctx, env))})
}

func ListAdoptableResources(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	env, app, ok := loadEnvironmentAndApp(c, uint(envID))
	if !ok {
		return
	}
	resources, err := discoverAdoptableResourcesForEnvironment(context.Background(), app, env)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resources})
}

func AdoptResource(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	env, app, ok := loadEnvironmentAndApp(c, uint(envID))
	if !ok {
		return
	}
	var req AdoptResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resources, err := discoverAdoptableResourcesForEnvironment(context.Background(), app, env)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	var selected *k8s.AdoptableResource
	for i := range resources {
		if resources[i].Key == req.Key {
			selected = &resources[i]
			break
		}
	}
	if selected == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "adoptable resource not found"})
		return
	}
	comp, err := componentFromAdoptableResource(env, *selected)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Create(&comp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": ComponentView{Component: comp, RuntimeConfig: selected.RuntimeConfig}})
}

func componentDeliveryMode(req CreateComponentRequest) string {
	mode := strings.ToLower(strings.TrimSpace(req.DeliveryMode))
	if mode == "source" {
		return "source"
	}
	return "image"
}

func validateComponentDeliveryRequest(req CreateComponentRequest) error {
	if req.DraftOnly {
		if strings.TrimSpace(req.Name) == "" {
			return fmt.Errorf("component name is required")
		}
		if strings.TrimSpace(req.Type) == "" {
			return fmt.Errorf("component type is required")
		}
		if strings.EqualFold(strings.TrimSpace(req.Version), "latest") {
			return fmt.Errorf("component image tag must be explicit; latest is not allowed")
		}
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(req.Image)), ":latest") {
			return fmt.Errorf("component image tag must be explicit; latest is not allowed")
		}
		return nil
	}
	mode := componentDeliveryMode(req)
	version := strings.TrimSpace(req.Version)
	if strings.EqualFold(version, "latest") {
		return fmt.Errorf("component image tag must be explicit; latest is not allowed")
	}
	if mode == "source" {
		if strings.TrimSpace(req.SourceRepoURL) == "" {
			return fmt.Errorf("source repository URL is required")
		}
		return nil
	}

	if strings.TrimSpace(req.Image) == "" {
		return fmt.Errorf("component image is required")
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(req.Image)), ":latest") {
		return fmt.Errorf("component image tag must be explicit; latest is not allowed")
	}
	if version == "" && !imageReferenceHasTag(req.Image) {
		return fmt.Errorf("component version is required when image has no tag")
	}
	return nil
}

func planComponentDelivery(app model.Application, env model.Environment, req CreateComponentRequest, comp model.Component, primaryNS string, registryServiceTypes ...string) (model.Component, error) {
	mode := componentDeliveryMode(req)
	identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	registryServiceType := "registry"
	if len(registryServiceTypes) > 0 && strings.TrimSpace(registryServiceTypes[0]) != "" {
		registryServiceType = strings.TrimSpace(registryServiceTypes[0])
	}
	version := strings.TrimSpace(comp.Version)
	if version == "" {
		version = strings.TrimSpace(req.Version)
	}
	if version == "" {
		version = imageReferenceTag(req.Image)
	}
	if mode == "source" && version == "" {
		version = "manual"
	}
	comp.Version = version
	comp.DeliveryMode = mode

	repoName := fmt.Sprintf("%s-%s-components", app.Identifier, env.Identifier)
	comp.GitRepoURL = fmt.Sprintf("http://%s-%s-git.%s-%s-git.svc.cluster.local:3000/paap/%s.git", app.Identifier, env.Identifier, app.Identifier, env.Identifier, repoName)
	comp.GitPath = fmt.Sprintf("components/%s", identifier)
	comp.ArgoCDApp = fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, identifier)

	if mode == "image" {
		if comp.Image == "" {
			comp.Image = strings.TrimSpace(req.Image)
		}
		return comp, nil
	}

	comp.SourceRepoURL = strings.TrimSpace(req.SourceRepoURL)
	comp.SourceMirrorRepoURL = fmt.Sprintf("http://%s-%s-git.%s-%s-git.svc.cluster.local:3000/paap/%s-%s-%s-source.git", app.Identifier, env.Identifier, app.Identifier, env.Identifier, app.Identifier, env.Identifier, identifier)
	comp.SourceBranch = valueOrDefaultString(req.SourceBranch, "main")
	comp.BuildContext = valueOrDefaultString(req.BuildContext, ".")
	comp.DockerfilePath = strings.TrimSpace(req.DockerfilePath)
	comp.JenkinsJob = fmt.Sprintf("%s-%s-%s-build", app.Identifier, env.Identifier, identifier)
	comp.RegistryImage = service.RuntimeRegistryImage(app, env, registryServiceType, fmt.Sprintf("%s-%s/%s", app.Identifier, env.Identifier, identifier), version)
	comp.Image = comp.RegistryImage
	comp.PipelineStatus = "planned"
	return comp, nil
}

func preferredSourceRegistryServiceType(services []model.ServiceInstallation) string {
	serviceType, _, _ := preferredSourceRegistrySelection(services)
	return serviceType
}

func preferredSourceRegistrySelection(services []model.ServiceInstallation) (string, model.ServiceInstallation, bool) {
	for _, status := range []string{"running", ""} {
		for _, serviceType := range []string{"harbor", "registry"} {
			for _, inst := range services {
				if inst.ServiceType != serviceType {
					continue
				}
				if status == "" || strings.EqualFold(inst.Status, status) {
					return serviceType, inst, true
				}
			}
		}
	}
	return "registry", model.ServiceInstallation{}, false
}

func valueOrDefaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func ensureSourceHarborProject(ctx context.Context, comp model.Component, primaryNS string, services []model.ServiceInstallation) error {
	if comp.DeliveryMode != "source" {
		return nil
	}
	serviceType, inst, found := preferredSourceRegistrySelection(services)
	if serviceType != "harbor" || !found {
		return nil
	}
	if strings.TrimSpace(inst.Namespace) == "" {
		return fmt.Errorf("harbor namespace is required before preparing project %s", primaryNS)
	}
	return newHarborProjectEnsurer(inst.Namespace).EnsureProject(ctx, primaryNS)
}

// CreateComponent creates a draft component. Deployment is an explicit action.
func CreateComponent(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}

	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var req CreateComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateComponentDeliveryRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		version = imageReferenceTag(req.Image)
	}

	comp := model.Component{
		EnvironmentID: uint(envID),
		Name:          req.Name,
		Type:          req.Type,
		Image:         req.Image,
		Version:       version,
		Replicas:      req.Replicas,
		CPU:           req.CPU,
		Memory:        req.Memory,
		Status:        "draft",
		DeliveryMode:  componentDeliveryMode(req),
	}

	if comp.Replicas == 0 {
		comp.Replicas = 1
	}
	if comp.DeliveryMode == "source" {
		comp.SourceRepoURL = strings.TrimSpace(req.SourceRepoURL)
		comp.SourceBranch = valueOrDefaultString(req.SourceBranch, "main")
		comp.BuildContext = valueOrDefaultString(req.BuildContext, ".")
		comp.DockerfilePath = strings.TrimSpace(req.DockerfilePath)
		comp.Image = ""
		comp.RegistryImage = ""
		comp.PipelineStatus = "draft"
	}

	if err := database.DB.Create(&comp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": comp})
}

// UpdateComponent updates draft/runtime configuration. Deployment remains explicit.
func UpdateComponent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var comp model.Component
	if err := database.DB.First(&comp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, comp.EnvironmentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}
	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var req UpdateComponentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		comp.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Type) != "" {
		comp.Type = strings.TrimSpace(req.Type)
	}
	if strings.TrimSpace(req.DeliveryMode) != "" {
		mode := strings.ToLower(strings.TrimSpace(req.DeliveryMode))
		switch mode {
		case "source":
			if strings.TrimSpace(req.SourceRepoURL) == "" && strings.TrimSpace(comp.SourceRepoURL) == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "source repository URL is required"})
				return
			}
			comp.DeliveryMode = "source"
			if strings.TrimSpace(req.SourceRepoURL) != "" {
				comp.SourceRepoURL = strings.TrimSpace(req.SourceRepoURL)
			}
			comp.SourceBranch = valueOrDefaultString(req.SourceBranch, valueOrDefaultString(comp.SourceBranch, "main"))
			comp.BuildContext = valueOrDefaultString(req.BuildContext, valueOrDefaultString(comp.BuildContext, "."))
			comp.DockerfilePath = strings.TrimSpace(req.DockerfilePath)
			identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
			comp.JenkinsJob = fmt.Sprintf("%s-%s-%s-build", app.Identifier, env.Identifier, identifier)
			comp.Image = ""
			comp.RegistryImage = ""
			comp.PipelineStatus = "planned"
		case "image":
			comp.DeliveryMode = "image"
			comp.SourceRepoURL = ""
			comp.SourceMirrorRepoURL = ""
			comp.SourceBranch = ""
			comp.BuildContext = ""
			comp.DockerfilePath = ""
			comp.JenkinsJob = ""
			if comp.PipelineStatus == "planned" || comp.PipelineStatus == "pending" {
				comp.PipelineStatus = ""
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "deliveryMode must be image or source"})
			return
		}
	}
	if strings.TrimSpace(req.Image) != "" {
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(req.Image)), ":latest") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "component image tag must be explicit; latest is not allowed"})
			return
		}
		comp.Image = strings.TrimSpace(req.Image)
		if componentUsesDirectImageReference(comp) {
			comp.RegistryImage = comp.Image
		}
		if tag := imageReferenceTag(comp.Image); tag != "" {
			comp.Version = tag
		}
	}
	if strings.TrimSpace(req.Version) != "" {
		if strings.EqualFold(req.Version, "latest") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "component image tag must be explicit; latest is not allowed"})
			return
		}
		comp.Version = strings.TrimSpace(req.Version)
		if componentUsesDirectImageReference(comp) {
			if strings.TrimSpace(comp.RegistryImage) != "" {
				comp.RegistryImage = service.ImageWithTag(comp.RegistryImage, comp.Version)
				comp.Image = comp.RegistryImage
			} else if strings.TrimSpace(comp.Image) != "" {
				comp.Image = service.ImageWithTag(comp.Image, comp.Version)
			}
		}
	}
	if req.Replicas > 0 {
		comp.Replicas = req.Replicas
	}
	if strings.TrimSpace(req.CPU) != "" {
		comp.CPU = strings.TrimSpace(req.CPU)
	}
	if strings.TrimSpace(req.Memory) != "" {
		comp.Memory = strings.TrimSpace(req.Memory)
	}
	if req.Config != nil {
		configJSON, err := req.Config.JSON()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		comp.Config = configJSON
	}

	if err := database.DB.Save(&comp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": comp})
}

func componentUsesDirectImageReference(comp model.Component) bool {
	return strings.TrimSpace(comp.DeliveryMode) != "source" && strings.TrimSpace(comp.JenkinsJob) == ""
}

func imageReferenceHasTag(image string) bool {
	return imageReferenceTag(image) != ""
}

func imageReferenceTag(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	colon := strings.LastIndex(last, ":")
	if colon < 0 || colon == len(last)-1 {
		return ""
	}
	return last[colon+1:]
}

func componentDeleteIdentifier(app model.Application, env model.Environment, comp model.Component) string {
	if identifier := componentIdentifierFromArgoCDApp(app, env, comp.ArgoCDApp); identifier != "" {
		return identifier
	}
	if identifier := componentIdentifierFromGitPath(comp.GitPath); identifier != "" {
		return identifier
	}
	return service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
}

func componentIdentifierFromArgoCDApp(app model.Application, env model.Environment, appName string) string {
	appName = strings.TrimSpace(appName)
	if appName == "" {
		return ""
	}
	prefix := strings.Trim(strings.TrimSpace(app.Identifier)+"-"+strings.TrimSpace(env.Identifier)+"-", "-")
	if prefix == "" || !strings.HasPrefix(appName, prefix) {
		return ""
	}
	return strings.Trim(strings.TrimPrefix(appName, prefix), "-")
}

func componentIdentifierFromGitPath(gitPath string) string {
	gitPath = strings.Trim(strings.TrimSpace(gitPath), "/")
	if gitPath == "" {
		return ""
	}
	if strings.HasPrefix(gitPath, "components/") {
		gitPath = strings.TrimPrefix(gitPath, "components/")
	}
	if slash := strings.Index(gitPath, "/"); slash >= 0 {
		gitPath = gitPath[:slash]
	}
	return strings.Trim(gitPath, "-")
}

func componentArgoCDApplicationName(app model.Application, env model.Environment, comp model.Component, identifier string) string {
	if strings.TrimSpace(comp.ArgoCDApp) != "" {
		return strings.TrimSpace(comp.ArgoCDApp)
	}
	return fmt.Sprintf("%s-%s-%s", app.Identifier, env.Identifier, identifier)
}

func componentArgoCDNamespace(envID uint, app model.Application, env model.Environment) string {
	var inst model.ServiceInstallation
	if err := database.DB.
		Where("environment_id = ? AND service_type = ?", envID, "deploy").
		First(&inst).Error; err == nil && strings.TrimSpace(inst.Namespace) != "" {
		return strings.TrimSpace(inst.Namespace)
	}
	return fmt.Sprintf("%s-%s-argocd", app.Identifier, env.Identifier)
}

func removeComponentFromCanvasState(envID uint, componentID uint) {
	componentKey := fmt.Sprintf("component:%d", componentID)
	var state model.EnvironmentCanvasState
	if err := database.DB.First(&state, "environment_id = ?", envID).Error; err != nil {
		return
	}

	var positions map[string]json.RawMessage
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Positions, "{}")), &positions); err == nil {
		delete(positions, componentKey)
		if data, err := json.Marshal(positions); err == nil {
			state.Positions = string(data)
		}
	}

	var edges []map[string]interface{}
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Edges, "[]")), &edges); err == nil {
		filtered := edges[:0]
		for _, edge := range edges {
			if edge["fromKey"] == componentKey || edge["toKey"] == componentKey {
				continue
			}
			filtered = append(filtered, edge)
		}
		if data, err := json.Marshal(filtered); err == nil {
			state.Edges = string(data)
		}
	}

	var names map[string]string
	if err := json.Unmarshal([]byte(valueOrDefaultString(state.Names, "{}")), &names); err == nil {
		delete(names, componentKey)
		if data, err := json.Marshal(names); err == nil {
			state.Names = string(data)
		}
	}

	_ = database.DB.Save(&state).Error
}

// DeployComponent updates a component version and re-runs GitOps
func DeployComponent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var comp model.Component
	if err := database.DB.First(&comp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, comp.EnvironmentID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}
	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var req struct {
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Version = strings.TrimSpace(req.Version)
	if req.Version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version is required"})
		return
	}

	identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	comp.Version = req.Version
	if comp.DeliveryMode == "source" {
		var registryInstallations []model.ServiceInstallation
		_ = database.DB.
			Where("environment_id = ? AND service_type IN ?", env.ID, []string{"registry", "harbor"}).
			Find(&registryInstallations).Error
		comp = applyComponentDeployVersionForRuntimeRegistry(
			app,
			env,
			comp,
			identifier,
			req.Version,
			preferredSourceRegistryServiceType(registryInstallations),
		)
	} else {
		comp = applyComponentDeployVersion(comp, req.Version)
	}

	ctx := context.Background()
	primaryNS := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	cfg, err := model.ParseComponentConfig(comp.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "component config invalid: " + err.Error()})
		return
	}
	if detected := detectComponentContainerPort(ctx, env, identifier, comp.Image, cfg); detected > 0 {
		cfg.ContainerPort = detected
		configJSON, err := cfg.JSON()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "component config invalid: " + err.Error()})
			return
		}
		comp.Config = configJSON
	}
	database.DB.Save(&comp)

	result, err := service.EnsureComponentGitOps(ctx, k8s.GetClient(), app, env, comp, identifier, primaryNS)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": "GitOps update failed: " + err.Error()})
		return
	}
	comp.GitRepoURL = result.RepositoryURL
	if result.SourceMirrorURL != "" {
		comp.SourceMirrorRepoURL = result.SourceMirrorURL
	}
	comp.GitPath = result.RepositoryPath
	comp.ArgoCDApp = result.ArgoCDApplication
	if result.CIStatus != "" {
		comp.PipelineStatus = result.CIStatus
	}
	if result.CIWarning != "" {
		comp.ErrorMessage = result.CIWarning
	} else {
		comp.ErrorMessage = ""
	}

	envVars, err := model.ComponentEnvVars(comp.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "component config invalid: " + err.Error()})
		return
	}
	if err := k8s.UpsertComponentCR(ctx, app.Identifier, env.Identifier, comp.Name, identifier, comp.Type, comp.Image, comp.Version, int32(comp.Replicas), primaryNS, "argocd", cfg, envVars); err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": "Component CR upsert failed: " + err.Error()})
		return
	}

	comp.Status = "syncing"
	database.DB.Save(&comp)
	c.JSON(http.StatusOK, gin.H{"data": comp})
}

func applyComponentDeployVersion(comp model.Component, version string) model.Component {
	version = strings.TrimSpace(version)
	if version == "" {
		return comp
	}
	comp.Version = version
	if strings.TrimSpace(comp.RegistryImage) != "" {
		comp.RegistryImage = service.ImageWithTag(comp.RegistryImage, version)
		comp.Image = comp.RegistryImage
	} else if strings.TrimSpace(comp.Image) != "" {
		comp.Image = service.ImageWithTag(comp.Image, version)
	}
	if comp.DeliveryMode == "source" || comp.JenkinsJob != "" || comp.RegistryImage != "" {
		comp.PipelineStatus = "built"
	}
	return comp
}

func detectComponentContainerPort(ctx context.Context, env model.Environment, identifier, image string, cfg model.ComponentConfig) int32 {
	if cfg.ContainerPort > 0 {
		return 0
	}
	if detected := detectComponentImageContainerPort(ctx, env.ID, image); detected > 0 {
		return detected
	}
	if runtime, err := k8s.DiscoverComponentRuntimeConfig(ctx, env.Namespace, identifier); err == nil && runtime != nil && len(runtime.Ports) > 0 {
		return runtime.Ports[0]
	}
	return 0
}

func detectComponentImageContainerPort(ctx context.Context, envID uint, image string) int32 {
	repository, reference := imageRepositoryAndReference(image)
	if repository == "" || reference == "" {
		return 0
	}
	var registries []model.ServiceInstallation
	if err := database.DB.
		Where("environment_id = ? AND service_type IN ?", envID, []string{"registry", "harbor"}).
		Find(&registries).Error; err != nil {
		return 0
	}
	for _, inst := range registries {
		ports, err := k8s.NewRegistryClient(inst.Namespace).ExposedPorts(ctx, repository, reference)
		if err != nil || len(ports) == 0 {
			continue
		}
		return ports[0]
	}
	return 0
}

func imageRepositoryAndReference(image string) (string, string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	if digestAt := strings.Index(image, "@"); digestAt >= 0 {
		repository := strings.Trim(strings.TrimSpace(image[:digestAt]), "/")
		reference := strings.TrimSpace(image[digestAt+1:])
		if slash := strings.Index(repository, "/"); slash >= 0 && imageReferenceFirstSegmentIsRegistry(repository[:slash]) {
			repository = repository[slash+1:]
		}
		return repository, reference
	}
	parts := strings.Split(image, "/")
	last := parts[len(parts)-1]
	colon := strings.LastIndex(last, ":")
	if colon < 0 || colon == len(last)-1 {
		return "", ""
	}
	reference := last[colon+1:]
	parts[len(parts)-1] = last[:colon]
	if len(parts) > 1 && imageReferenceFirstSegmentIsRegistry(parts[0]) {
		parts = parts[1:]
	}
	return strings.Trim(strings.Join(parts, "/"), "/"), reference
}

func imageReferenceFirstSegmentIsRegistry(segment string) bool {
	segment = strings.ToLower(strings.TrimSpace(segment))
	return segment == "localhost" || strings.Contains(segment, ".") || strings.Contains(segment, ":")
}

func applyComponentDeployVersionForRuntimeRegistry(app model.Application, env model.Environment, comp model.Component, identifier, version, registryServiceType string) model.Component {
	version = strings.TrimSpace(version)
	if version == "" {
		return comp
	}
	if strings.TrimSpace(registryServiceType) == "" {
		registryServiceType = "registry"
	}
	repository := fmt.Sprintf("%s-%s/%s", app.Identifier, env.Identifier, identifier)
	comp.Version = version
	comp.RegistryImage = service.RuntimeRegistryImage(app, env, registryServiceType, repository, version)
	comp.Image = comp.RegistryImage
	if comp.DeliveryMode == "source" || comp.JenkinsJob != "" || comp.RegistryImage != "" {
		comp.PipelineStatus = "built"
	}
	return comp
}

// DeleteComponent deletes a component and its K8s resources
func DeleteComponent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var comp model.Component
	if err := database.DB.First(&comp, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
		return
	}

	var env model.Environment
	var app model.Application
	if err := database.DB.First(&env, comp.EnvironmentID).Error; err == nil {
		if err := database.DB.First(&app, env.ApplicationID).Error; err == nil {
			ctx := context.Background()
			identifier := componentDeleteIdentifier(app, env, comp)
			argocdNamespace := componentArgoCDNamespace(env.ID, app, env)
			argocdApp := componentArgoCDApplicationName(app, env, comp, identifier)
			if err := k8s.DeleteArgoCDApplication(ctx, argocdNamespace, argocdApp); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": "ArgoCD Application delete failed: " + err.Error()})
				return
			}
			if err := k8s.DeleteComponentCR(ctx, app.Identifier, env.Identifier, identifier); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": "Component CR delete failed: " + err.Error()})
				return
			}
			generatedIdentifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
			if err := k8s.DeleteComponentRuntimeResources(ctx, env.Namespace, identifier, generatedIdentifier, comp.Name, comp.GitPath); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": "Component runtime resources delete failed: " + err.Error()})
				return
			}
		}
	}

	if err := database.DB.Delete(&comp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	removeComponentFromCanvasState(comp.EnvironmentID, comp.ID)
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListServiceInstances returns service installations for an environment
func ListServiceInstances(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}

	var services []model.ServiceInstallation
	if err := database.DB.Where("environment_id = ?", env.ID).Find(&services).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	enrichServiceInstallationsWithCRStatus(ctx, envID, services)
	access := collectEnvironmentExternalAccess(ctx, env)
	c.JSON(http.StatusOK, gin.H{"data": enrichServiceInstallationViews(ctx, services, access)})
}

func enrichServiceInstallationsWithCRStatus(ctx context.Context, envID int, services []model.ServiceInstallation) {
	if len(services) == 0 {
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		log.Printf("[ListServiceInstances] environment lookup warning: %v", err)
		return
	}
	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		log.Printf("[ListServiceInstances] application lookup warning: %v", err)
		return
	}

	for i := range services {
		crStatus, err := k8s.GetServiceInstanceCRStatus(ctx, app.Identifier, env.Identifier, services[i].ServiceType)
		if err != nil {
			log.Printf("[ListServiceInstances] CR status query warning for %s: %v", services[i].ServiceType, err)
			continue
		}
		if crStatus == nil {
			continue
		}
		if status := normalizeServicePhase(crStatus.Phase, services[i].Status); status != "" {
			services[i].Status = status
		}
		services[i].ErrorMessage = serviceStatusErrorMessage(crStatus)
	}
}

func normalizeServicePhase(phase, fallback string) string {
	phase = strings.ToLower(strings.TrimSpace(phase))
	if phase == "" {
		return fallback
	}
	return phase
}

func serviceStatusErrorMessage(status *paapv1.ServiceInstanceStatus) string {
	if status == nil {
		return ""
	}
	for _, cond := range status.Conditions {
		if strings.EqualFold(cond.Type, "Ready") && cond.Status == "False" {
			if strings.TrimSpace(cond.Message) != "" {
				return cond.Message
			}
			if strings.TrimSpace(cond.Reason) != "" {
				return cond.Reason
			}
		}
	}
	return ""
}

// GetServiceInstance returns a single service installation enriched with K8s CR status.
func GetServiceInstance(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}

	var inst model.ServiceInstallation
	if err := database.DB.Where("id = ? AND environment_id = ?", serviceID, envID).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "application not found"})
		return
	}

	crStatus, err := k8s.GetServiceInstanceCRStatus(context.Background(), app.Identifier, env.Identifier, inst.ServiceType)
	if err != nil {
		log.Printf("[GetServiceInstance] CR status query warning: %v", err)
	}
	access := collectEnvironmentExternalAccess(context.Background(), env)
	view := enrichServiceInstallationViews(context.Background(), []model.ServiceInstallation{inst}, access)[0]

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"installation": view,
			"crStatus":     crStatus,
		},
	})
}

// GetServiceCredentials returns credential-like values from real Kubernetes Secrets
// in the service namespace. It does not synthesize defaults.
func GetServiceCredentials(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}

	var inst model.ServiceInstallation
	if err := database.DB.Where("id = ? AND environment_id = ?", serviceID, envID).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	credentials, err := discoverServiceCredentials(context.Background(), inst.Namespace)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"credentials": credentials}})
}

func discoverServiceCredentials(ctx context.Context, namespace string) ([]ServiceCredential, error) {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil, fmt.Errorf("service namespace is empty")
	}
	cl := k8s.GetClient()
	if cl == nil {
		return nil, fmt.Errorf("kubernetes client is not initialized")
	}
	secrets := &corev1.SecretList{}
	if err := cl.List(ctx, secrets, ctrlclient.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("list secrets: %w", err)
	}
	credentials := make([]ServiceCredential, 0)
	for _, secret := range secrets.Items {
		if shouldSkipCredentialSecret(secret) {
			continue
		}
		for key, raw := range secret.Data {
			kind, ok := credentialKeyKind(key)
			if !ok || len(raw) == 0 {
				continue
			}
			credentials = append(credentials, ServiceCredential{
				Secret: secret.Name,
				Key:    key,
				Value:  string(raw),
				Kind:   kind,
			})
		}
	}
	sort.Slice(credentials, func(i, j int) bool {
		if credentials[i].Secret != credentials[j].Secret {
			return credentials[i].Secret < credentials[j].Secret
		}
		return credentials[i].Key < credentials[j].Key
	})
	return credentials, nil
}

func shouldSkipCredentialSecret(secret corev1.Secret) bool {
	switch secret.Type {
	case corev1.SecretTypeServiceAccountToken, corev1.SecretTypeTLS, corev1.SecretTypeDockerConfigJson, corev1.SecretTypeDockercfg:
		return true
	default:
		return false
	}
}

func credentialKeyKind(key string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(key))
	switch {
	case normalized == "username", normalized == "user", strings.HasSuffix(normalized, "-username"):
		return "username", true
	case normalized == "password", normalized == "passwd", normalized == "root-password", strings.Contains(normalized, "password"):
		return "password", true
	case normalized == "accesskey", normalized == "access-key", normalized == "access_key", strings.Contains(normalized, "accesskey"):
		return "accessKey", true
	case normalized == "secretkey", normalized == "secret-key", normalized == "secret_key", strings.Contains(normalized, "secretkey"):
		return "secretKey", true
	default:
		return "", false
	}
}

func loadServiceWorkspaceContext(envID, serviceID int) (model.Application, model.Environment, model.ServiceInstallation, []model.Component, error) {
	var inst model.ServiceInstallation
	if err := database.DB.Where("id = ? AND environment_id = ?", serviceID, envID).First(&inst).Error; err != nil {
		return model.Application{}, model.Environment{}, model.ServiceInstallation{}, nil, err
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		return model.Application{}, model.Environment{}, model.ServiceInstallation{}, nil, err
	}
	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		return model.Application{}, model.Environment{}, model.ServiceInstallation{}, nil, err
	}
	var components []model.Component
	if err := database.DB.Where("environment_id = ?", env.ID).Find(&components).Error; err != nil {
		return model.Application{}, model.Environment{}, model.ServiceInstallation{}, nil, err
	}
	return app, env, inst, components, nil
}

func environmentArgoCDProjectName(app model.Application, env model.Environment) string {
	return fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
}

func allowedArgoCDDestinationNamespaces(ctx context.Context, app model.Application, env model.Environment) []string {
	namespaces := map[string]bool{}
	add := func(namespace string) {
		namespace = strings.TrimSpace(namespace)
		if namespace != "" && !isReservedKubernetesNamespace(namespace) {
			namespaces[namespace] = true
		}
	}

	if cl := k8s.GetClient(); cl != nil {
		list := &corev1.NamespaceList{}
		if err := cl.List(ctx, list, ctrlclient.MatchingLabels{"paap.io/app": app.Identifier, "paap.io/env": env.Identifier}); err == nil {
			for _, namespace := range list.Items {
				add(namespace.Name)
			}
		}
	}
	add(env.Namespace)
	projectName := environmentArgoCDProjectName(app, env)
	add(projectName)
	add(projectName + "-app")

	result := make([]string, 0, len(namespaces))
	for namespace := range namespaces {
		result = append(result, namespace)
	}
	sort.Strings(result)
	return result
}

func isReservedKubernetesNamespace(namespace string) bool {
	switch strings.TrimSpace(namespace) {
	case "default", "kube-system", "kube-public", "kube-node-lease":
		return true
	default:
		return false
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// GetServiceWorkspace returns a tool-specific workspace backed by database and cluster state.
func GetServiceWorkspace(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service workspace not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}

	app, env, inst, components, err := loadServiceWorkspaceContext(envID, serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service workspace not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
}

func GetServiceRuntimeMetrics(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var accessEnv model.Environment
	if err := database.DB.First(&accessEnv, envID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !requireApplicationAccess(c, accessEnv.ApplicationID) {
		return
	}

	_, env, inst, _, err := loadServiceWorkspaceContext(envID, serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(inst.Namespace) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service namespace is not available"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	cfg, _ := k8s.DiscoverNamespaceRuntimeConfig(ctx, inst.Namespace)
	metrics, err := k8s.GetRuntimeMetrics(ctx, inst.Namespace, cfg)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	if !metrics.Available {
		metrics = k8s.EnrichRuntimeMetricsFromPrometheus(ctx, metrics, monitorNamespaceForEnvironment(env.ID))
	}
	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

func GetServiceRuntimeLogs(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var accessEnv model.Environment
	if err := database.DB.First(&accessEnv, envID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !requireApplicationAccess(c, accessEnv.ApplicationID) {
		return
	}

	_, _, inst, _, err := loadServiceWorkspaceContext(envID, serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(inst.Namespace) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service namespace is not available"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	cfg, _ := k8s.DiscoverNamespaceRuntimeConfig(ctx, inst.Namespace)
	logs, err := k8s.GetRuntimeLogs(ctx, inst.Namespace, cfg, runtimeLogTailLines(c))
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func GetComponentRuntimeMetrics(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	componentID, _ := strconv.Atoi(c.Param("componentId"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	var comp model.Component
	if err := database.DB.Where("id = ? AND environment_id = ?", componentID, envID).First(&comp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	cfg, _ := k8s.DiscoverComponentRuntimeConfig(ctx, env.Namespace, identifier)
	if cfg == nil {
		cfg = &k8s.RuntimeConfig{Namespace: env.Namespace, WorkloadName: identifier}
	}
	metrics, err := k8s.GetRuntimeMetrics(ctx, env.Namespace, cfg)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	if !metrics.Available {
		metrics = k8s.EnrichRuntimeMetricsFromPrometheus(ctx, metrics, monitorNamespaceForEnvironment(env.ID))
	}
	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

func GetComponentRuntimeLogs(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	componentID, _ := strconv.Atoi(c.Param("componentId"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	var comp model.Component
	if err := database.DB.Where("id = ? AND environment_id = ?", componentID, envID).First(&comp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	cfg, _ := k8s.DiscoverComponentRuntimeConfig(ctx, env.Namespace, identifier)
	if cfg == nil {
		cfg = &k8s.RuntimeConfig{Namespace: env.Namespace, WorkloadName: identifier}
	}
	logs, err := k8s.GetRuntimeLogs(ctx, env.Namespace, cfg, runtimeLogTailLines(c))
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func runtimeLogTailLines(c *gin.Context) int64 {
	raw := strings.TrimSpace(c.Query("tail"))
	if raw == "" {
		return 200
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 200
	}
	if value > 1000 {
		return 1000
	}
	return value
}

func monitorNamespaceForEnvironment(envID uint) string {
	var inst model.ServiceInstallation
	if err := database.DB.
		Where("environment_id = ? AND service_type = ?", envID, "monitor").
		Order("id DESC").
		First(&inst).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(inst.Namespace)
}

func ProxyServiceInstance(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))
	_, _, inst, _, err := loadServiceWorkspaceContext(envID, serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	baseURL := toolHTTPBaseURL(inst)
	if baseURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "service does not expose a browser proxy"})
		return
	}
	target, err := url.Parse(baseURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	proxyPrefix := fmt.Sprintf("/api/v1/environments/%d/services/%d/proxy", envID, serviceID)
	paapEmbeddedGrafana := inst.ServiceType == "monitor" && c.Query("paap_embed") != ""
	proxy := httputil.NewSingleHostReverseProxy(target)
	if inst.ServiceType == "registry" {
		proxy.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} //nolint:gosec
	}
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		prepareToolProxyRequest(req, target, proxyPrefix, c.Param("path"), inst)
	}
	proxy.ModifyResponse = func(res *http.Response) error {
		res.Header.Del("X-Frame-Options")
		res.Header.Del("Content-Security-Policy")
		location := res.Header.Get("Location")
		if location != "" {
			rewritten := rewriteProxyLocation(location, target, proxyPrefix)
			if rewritten != "" {
				res.Header.Set("Location", rewritten)
			}
		}
		if strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/html") {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			_ = res.Body.Close()
			rewritten := rewriteToolProxyHTML(string(body), proxyPrefix, paapEmbeddedGrafana)
			res.Body = io.NopCloser(strings.NewReader(rewritten))
			res.ContentLength = int64(len(rewritten))
			res.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
			res.Header.Del("Content-Encoding")
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, "proxy failed: "+err.Error(), http.StatusBadGateway)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func ProxyComponent(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	componentID, _ := strconv.Atoi(c.Param("componentId"))

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	var comp model.Component
	if err := database.DB.Where("id = ? AND environment_id = ?", componentID, envID).First(&comp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
		return
	}

	target, err := componentProxyTarget(c.Request.Context(), env, comp)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}
	proxyPrefix := fmt.Sprintf("/api/v1/environments/%d/components/%d/proxy", envID, componentID)
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		prepareComponentProxyRequest(req, target, proxyPrefix, c.Param("path"))
	}
	proxy.ModifyResponse = func(res *http.Response) error {
		res.Header.Del("X-Frame-Options")
		res.Header.Del("Content-Security-Policy")
		location := res.Header.Get("Location")
		if location != "" {
			if rewritten := rewriteProxyLocation(location, target, proxyPrefix); rewritten != "" {
				res.Header.Set("Location", rewritten)
			}
		}
		if strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "text/html") {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			_ = res.Body.Close()
			rewritten := rewriteToolProxyHTML(string(body), proxyPrefix, false)
			res.Body = io.NopCloser(strings.NewReader(rewritten))
			res.ContentLength = int64(len(rewritten))
			res.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
			res.Header.Del("Content-Encoding")
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		http.Error(w, "proxy failed: "+err.Error(), http.StatusBadGateway)
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func componentProxyTarget(ctx context.Context, env model.Environment, comp model.Component) (*url.URL, error) {
	namespace := strings.TrimSpace(env.Namespace)
	if namespace == "" {
		return nil, fmt.Errorf("environment namespace is empty")
	}
	identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	svc, err := componentServiceForProxy(ctx, namespace, identifier, comp.Name)
	if err != nil {
		return nil, err
	}
	port, ok := componentProxyServicePort(svc)
	if !ok {
		return nil, fmt.Errorf("component service %s/%s has no browser HTTP port", svc.Namespace, svc.Name)
	}
	scheme := componentProxyPortScheme(port)
	return url.Parse(fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d", scheme, svc.Name, svc.Namespace, port.Port))
}

func componentServiceForProxy(ctx context.Context, namespace string, names ...string) (corev1.Service, error) {
	cl := k8s.GetClient()
	if cl == nil {
		return corev1.Service{}, fmt.Errorf("kubernetes client is not initialized")
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var svc corev1.Service
		if err := cl.Get(ctx, ctrlclient.ObjectKey{Name: name, Namespace: namespace}, &svc); err == nil {
			return svc, nil
		}
	}
	return corev1.Service{}, fmt.Errorf("component service not found in namespace %s", namespace)
}

func componentProxyServicePort(svc corev1.Service) (corev1.ServicePort, bool) {
	for _, port := range svc.Spec.Ports {
		name := strings.ToLower(strings.TrimSpace(port.Name))
		if name == "ssh" || strings.Contains(name, "ssh") || port.Port == 22 {
			continue
		}
		if port.Port == 80 || port.Port == 443 || strings.Contains(name, "http") {
			return port, true
		}
	}
	for _, port := range svc.Spec.Ports {
		if port.Port != 22 {
			return port, true
		}
	}
	return corev1.ServicePort{}, false
}

func componentProxyPortScheme(port corev1.ServicePort) string {
	name := strings.ToLower(strings.TrimSpace(port.Name))
	appProtocol := ""
	if port.AppProtocol != nil {
		appProtocol = strings.ToLower(strings.TrimSpace(*port.AppProtocol))
	}
	if port.Port == 443 || name == "https" || strings.Contains(name, "https") || appProtocol == "https" {
		return "https"
	}
	return "http"
}

func prepareComponentProxyRequest(req *http.Request, target *url.URL, proxyPrefix, path string) {
	forwardedHost := req.Host
	req.URL.Path = strings.TrimPrefix(path, "/")
	if req.URL.Path == "" {
		req.URL.Path = "/"
	} else {
		req.URL.Path = "/" + req.URL.Path
	}
	req.Host = target.Host
	req.Header.Del("Host")
	req.Header.Set("X-Forwarded-Host", forwardedHost)
	req.Header.Set("X-Forwarded-Prefix", proxyPrefix)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Del("Accept-Encoding")
}

func prepareToolProxyRequest(req *http.Request, target *url.URL, proxyPrefix, path string, inst model.ServiceInstallation) {
	forwardedHost := req.Host
	req.URL.Path = strings.TrimPrefix(path, "/")
	if req.URL.Path == "" {
		req.URL.Path = "/"
	} else {
		req.URL.Path = "/" + req.URL.Path
	}
	req.Host = target.Host
	req.Header.Del("Host")
	req.Header.Set("X-Forwarded-Host", forwardedHost)
	req.Header.Set("X-Forwarded-Prefix", proxyPrefix)
	req.Header.Set("X-Forwarded-Proto", "http")
	req.Header.Del("Accept-Encoding")
	if inst.ServiceType == "monitor" && isWebSocketUpgrade(req) {
		req.Header.Set("Origin", target.Scheme+"://"+target.Host)
	}
	addToolProxyAuth(req, inst)
	if inst.ServiceType == "monitor" {
		normalizeGrafanaLokiLogsVolumeRequest(req)
	}
}

func isWebSocketUpgrade(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Upgrade"), "websocket") && strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade")
}

func normalizeGrafanaLokiLogsVolumeRequest(req *http.Request) {
	if req == nil || req.Body == nil || req.Method != http.MethodPost {
		return
	}
	if !strings.HasSuffix(req.URL.Path, "/api/ds/query") || req.URL.Query().Get("ds_type") != "loki" {
		return
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		req.Body = io.NopCloser(strings.NewReader(""))
		req.ContentLength = 0
		req.Header.Set("Content-Length", "0")
		return
	}
	_ = req.Body.Close()
	rewritten := rewriteGrafanaLokiLogsVolumeBody(body)
	req.Body = io.NopCloser(bytes.NewReader(rewritten))
	req.ContentLength = int64(len(rewritten))
	req.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
}

func rewriteGrafanaLokiLogsVolumeBody(body []byte) []byte {
	if !bytes.Contains(body, []byte(`| drop __error__`)) {
		return body
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}
	queries, ok := payload["queries"].([]interface{})
	if !ok {
		return body
	}
	changed := false
	for _, item := range queries {
		query, ok := item.(map[string]interface{})
		if !ok || query["supportingQueryType"] != "logsVolume" {
			continue
		}
		expr, ok := query["expr"].(string)
		if !ok {
			continue
		}
		normalized := unsupportedLokiDropStagePattern.ReplaceAllString(expr, "")
		if normalized != expr {
			query["expr"] = normalized
			changed = true
		}
	}
	if !changed {
		return body
	}
	rewritten, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return rewritten
}

var unsupportedLokiDropStagePattern = regexp.MustCompile(`\s*\|\s*drop\s+__error__\s*`)

func rewriteToolProxyHTML(html, proxyPrefix string, paapEmbeddedGrafana bool) string {
	prefix := "/" + strings.Trim(strings.TrimSpace(proxyPrefix), "/")
	if prefix == "/" {
		return html
	}
	escapedPrefix := strings.ReplaceAll(prefix, "/", `\/`)
	replacer := strings.NewReplacer(
		`<base href="/" />`, `<base href="`+prefix+`/" />`,
		`<base href="/">`, `<base href="`+prefix+`/">`,
		`href="/`, `href="`+prefix+`/`,
		`href="public/`, `href="`+prefix+`/public/`,
		`href='/`, `href='`+prefix+`/`,
		`href='public/`, `href='`+prefix+`/public/`,
		`src="/`, `src="`+prefix+`/`,
		`src="public/`, `src="`+prefix+`/public/`,
		`src='/`, `src='`+prefix+`/`,
		`src='public/`, `src='`+prefix+`/public/`,
		`action="/`, `action="`+prefix+`/`,
		`action='/`, `action='`+prefix+`/`,
		`data-url="/`, `data-url="`+prefix+`/`,
		`data-url='/`, `data-url='`+prefix+`/`,
		`data-search-url="/`, `data-search-url="`+prefix+`/`,
		`data-search-url='/`, `data-search-url='`+prefix+`/`,
		`hx-get="/`, `hx-get="`+prefix+`/`,
		`hx-get='/`, `hx-get='`+prefix+`/`,
		`hx-post="/`, `hx-post="`+prefix+`/`,
		`hx-post='/`, `hx-post='`+prefix+`/`,
		`url(/`, `url(`+prefix+`/`,
		`assetUrlPrefix: '\/assets'`, `assetUrlPrefix: '`+escapedPrefix+`\/assets'`,
		`"appUrl":"http://localhost:3000/"`, `"appUrl":"`+prefix+`/"`,
		`"appSubUrl":""`, `"appSubUrl":"`+prefix+`"`,
		`"filePath":"public/`, `"filePath":"`+prefix+`/public/`,
		`"baseUrl":"public/`, `"baseUrl":"`+prefix+`/public/`,
		`"dark":"public/`, `"dark":"`+prefix+`/public/`,
		`"light":"public/`, `"light":"`+prefix+`/public/`,
		`"path":"public/`, `"path":"`+prefix+`/public/`,
		`"url":"/`, `"url":"`+prefix+`/`,
		`url":"\/`, `url":"`+escapedPrefix+`\/`,
	)
	rewritten := replacer.Replace(html)
	if paapEmbeddedGrafana {
		rewritten = injectPAAPGrafanaEmbedStyle(rewritten)
	}
	return rewritten
}

func injectPAAPGrafanaEmbedStyle(html string) string {
	if strings.Contains(html, "paap-grafana-embed-style") {
		return html
	}
	style := `<style id="paap-grafana-embed-style">
html, body, #reactRoot { background: #fff !important; overflow: auto !important; }
header, [aria-label="Navigation"], [data-testid*="sidemenu"], [data-testid*="navigation mega-menu"], .sidemenu, nav[class*="sidemenu"], aside[class*="sidemenu"] { display: none !important; width: 0 !important; min-width: 0 !important; flex-basis: 0 !important; }
[data-testid*="Nav toolbar"], div[class*="NavToolbar"], div[class*="PageToolbar"] { display: none !important; height: 0 !important; min-height: 0 !important; }
.css-1v5jvd { padding-top: 0 !important; }
@media (min-width: 769px) { .css-1bavtc9 { top: 0 !important; } }
</style>
<script id="paap-grafana-embed-script">
(function () {
  function hideGrafanaChrome() {
    var selectors = [
      '[aria-label="Navigation"]',
      'header',
      '[data-testid*="sidemenu"]',
      '[data-testid*="navigation mega-menu"]',
      '[data-testid*="Nav toolbar"]'
    ];
    selectors.forEach(function (selector) {
      document.querySelectorAll(selector).forEach(function (node) {
        node.style.setProperty('display', 'none', 'important');
        node.style.setProperty('width', '0', 'important');
        node.style.setProperty('min-width', '0', 'important');
        node.style.setProperty('height', '0', 'important');
      });
    });
  }
  hideGrafanaChrome();
  new MutationObserver(hideGrafanaChrome).observe(document.documentElement, { childList: true, subtree: true });
})();
</script>`
	if strings.Contains(html, "</head>") {
		return strings.Replace(html, "</head>", style+"</head>", 1)
	}
	if strings.Contains(html, "</body>") {
		return strings.Replace(html, "</body>", style+"</body>", 1)
	}
	return style + html
}

func addToolProxyAuth(req *http.Request, inst model.ServiceInstallation) {
	switch inst.ServiceType {
	case "git":
		req.SetBasicAuth("paap", "paap123456")
	case "ci":
		req.SetBasicAuth("admin", "admin123")
	case "monitor":
		grafana := k8s.NewGrafanaClient(inst.Namespace)
		req.SetBasicAuth(grafana.Username, grafana.Password)
	}
}

func rewriteProxyLocation(location string, target *url.URL, proxyPrefix string) string {
	parsed, err := url.Parse(location)
	if err != nil {
		return ""
	}
	if parsed.IsAbs() {
		if parsed.Host != target.Host {
			return location
		}
		return proxyPrefix + parsed.RequestURI()
	}
	if strings.HasPrefix(location, "/") {
		return proxyPrefix + location
	}
	return proxyPrefix + "/" + location
}

// DownloadRegistryCACertificate returns the public CA certificate for an
// environment registry/Harbor service so node administrators can configure
// containerd/Docker trust. It never returns TLS private keys.
func DownloadRegistryCACertificate(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var accessEnv model.Environment
	if err := database.DB.First(&accessEnv, envID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !requireApplicationAccess(c, accessEnv.ApplicationID) {
		return
	}

	_, _, inst, _, err := loadServiceWorkspaceContext(envID, serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if inst.ServiceType != "registry" && inst.ServiceType != "harbor" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "certificate download is only supported for registry and harbor services"})
		return
	}
	if inst.Namespace == "" {
		c.JSON(http.StatusFailedDependency, gin.H{"error": "service namespace is empty"})
		return
	}

	cert, source, err := k8s.ReadRegistryCACertificate(c.Request.Context(), inst.Namespace, inst.ServiceType, inst.ReleaseName)
	if err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("%s-%s-ca.crt", inst.Namespace, inst.ServiceType)
	c.Header("Content-Type", "application/x-pem-file")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("X-PAAP-Certificate-Source", source)
	c.String(http.StatusOK, string(cert))
}

// RunServiceWorkspaceAction executes a tool-specific operation. The first
// concrete action is GitOps reconciliation, which creates/repairs Gitea repo
// content and ArgoCD Applications for every component in the environment.
func RunServiceWorkspaceAction(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var req WorkspaceActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var accessEnv model.Environment
	if err := database.DB.First(&accessEnv, envID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service workspace not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !requireApplicationAccess(c, accessEnv.ApplicationID) {
		return
	}

	app, env, inst, components, err := loadServiceWorkspaceContext(envID, serviceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service workspace not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	switch req.Action {
	case "refresh":
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "create_gitea_repository", "add_gitea_user_key", "add_gitea_deploy_key":
		if inst.ServiceType != "git" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for git services"})
			return
		}
		gitea := k8s.NewGiteaClient(inst.Namespace)
		ctx := c.Request.Context()
		switch req.Action {
		case "create_gitea_repository":
			if err := gitea.CreateRepository(ctx, workspaceActionParam(req, "name"), workspaceActionParam(req, "description"), workspaceActionBool(req, "private", true)); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
				return
			}
		case "add_gitea_user_key":
			if err := gitea.AddUserKey(ctx, workspaceActionParam(req, "title"), workspaceActionParam(req, "key")); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
				return
			}
		case "add_gitea_deploy_key":
			if err := gitea.AddDeployKey(ctx, workspaceActionParam(req, "repository"), workspaceActionParam(req, "title"), workspaceActionParam(req, "key"), workspaceActionBool(req, "readOnly", true)); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "check_grafana_health":
		if inst.ServiceType != "monitor" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for monitor services"})
			return
		}
		if err := k8s.NewGrafanaClient(inst.Namespace).HealthCheck(); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		inst.Status = "running"
		inst.ErrorMessage = ""
		_ = database.DB.Save(&inst).Error
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "provision_grafana_dashboard":
		if inst.ServiceType != "monitor" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for monitor services"})
			return
		}
		grafana := k8s.NewGrafanaClient(inst.Namespace)
		for _, dashboard := range buildDefaultGrafanaDashboards(app, env, components) {
			if err := grafana.ImportDashboard(dashboard.JSON, dashboard.Title); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "list_grafana_dashboards":
		if inst.ServiceType != "monitor" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for monitor services"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "list_prometheus_targets", "list_prometheus_alerts", "list_prometheus_rules":
		if inst.ServiceType != "monitor" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for monitor services"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "apply_argocd_application":
		if inst.ServiceType != "deploy" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for deploy services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		projectName := environmentArgoCDProjectName(app, env)
		destinationNamespace := strings.TrimSpace(workspaceActionParam(req, "destinationNamespace"))
		if destinationNamespace == "" {
			destinationNamespace = env.Namespace
		}
		allowedNamespaces := allowedArgoCDDestinationNamespaces(ctx, app, env)
		if !stringSliceContains(allowedNamespaces, destinationNamespace) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("destination namespace %q is outside environment %s; allowed namespaces: %s", destinationNamespace, projectName, strings.Join(allowedNamespaces, ",")),
				"data":  buildLiveToolWorkspace(app, env, inst, components),
			})
			return
		}
		argoSpec := k8s.ArgoCDApplicationSpec{
			Name:                 workspaceActionParam(req, "name"),
			Namespace:            inst.Namespace,
			Project:              projectName,
			RepoURL:              workspaceActionParam(req, "repoURL"),
			Path:                 workspaceActionParam(req, "path"),
			TargetRevision:       workspaceActionParam(req, "targetRevision"),
			DestinationServer:    "https://kubernetes.default.svc",
			DestinationNamespace: destinationNamespace,
			Automated:            workspaceActionBool(req, "automated", true),
		}
		if argoSpec.Name == "" {
			argoSpec.Name = strings.TrimSpace(req.Target)
		}
		if err := k8s.EnsureArgoCDDefaultProjectDenied(ctx, inst.Namespace); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if err := k8s.EnsureArgoCDEnvironmentProject(ctx, inst.Namespace, projectName, argoSpec.RepoURL, allowedNamespaces); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if err := k8s.EnsureArgoCDLocalClusterSecret(ctx, inst.Namespace, allowedNamespaces); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if err := k8s.ApplyArgoCDApplication(ctx, argoSpec); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "apply_argocd_applicationset":
		if inst.ServiceType != "deploy" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for deploy services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		projectName := environmentArgoCDProjectName(app, env)
		destinationNamespace := strings.TrimSpace(workspaceActionParam(req, "destinationNamespace"))
		if destinationNamespace == "" {
			destinationNamespace = env.Namespace
		}
		allowedNamespaces := allowedArgoCDDestinationNamespaces(ctx, app, env)
		if !stringSliceContains(allowedNamespaces, destinationNamespace) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": fmt.Sprintf("destination namespace %q is outside environment %s; allowed namespaces: %s", destinationNamespace, projectName, strings.Join(allowedNamespaces, ",")),
				"data":  buildLiveToolWorkspace(app, env, inst, components),
			})
			return
		}
		argoSpec := k8s.ArgoCDApplicationSetSpec{
			Name:                 workspaceActionParam(req, "name"),
			Namespace:            inst.Namespace,
			Project:              projectName,
			RepoURL:              workspaceActionParam(req, "repoURL"),
			Path:                 workspaceActionParam(req, "path"),
			TargetRevision:       workspaceActionParam(req, "targetRevision"),
			DestinationServer:    "https://kubernetes.default.svc",
			DestinationNamespace: destinationNamespace,
		}
		if argoSpec.Name == "" {
			argoSpec.Name = strings.TrimSpace(req.Target)
		}
		if err := k8s.EnsureArgoCDDefaultProjectDenied(ctx, inst.Namespace); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if err := k8s.EnsureArgoCDEnvironmentProject(ctx, inst.Namespace, projectName, argoSpec.RepoURL, allowedNamespaces); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if err := k8s.EnsureArgoCDLocalClusterSecret(ctx, inst.Namespace, allowedNamespaces); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if err := k8s.ApplyArgoCDApplicationSet(ctx, argoSpec); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "sync_argocd_application":
		if inst.ServiceType != "deploy" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for deploy services"})
			return
		}
		target := strings.TrimSpace(req.Target)
		if target == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target application is required"})
			return
		}
		if err := k8s.SyncArgoCDApplication(context.Background(), inst.Namespace, target); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "delete_argocd_application":
		if inst.ServiceType != "deploy" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for deploy services"})
			return
		}
		target := strings.TrimSpace(req.Target)
		if target == "" {
			target = workspaceActionParam(req, "name")
		}
		if target == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target application is required"})
			return
		}
		if err := k8s.DeleteArgoCDApplication(context.Background(), inst.Namespace, target); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "check_loki_health":
		if inst.ServiceType != "log" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for log services"})
			return
		}
		if err := k8s.NewLokiClient(inst.Namespace).HealthCheck(context.Background()); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		inst.Status = "running"
		inst.ErrorMessage = ""
		_ = database.DB.Save(&inst).Error
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "query_loki_streams":
		if inst.ServiceType != "log" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for log services"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "query_loki_logs":
		if inst.ServiceType != "log" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for log services"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "check_jenkins_health":
		if inst.ServiceType != "ci" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for ci services"})
			return
		}
		if err := k8s.NewJenkinsClient(inst.Namespace).HealthCheck(context.Background()); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		inst.Status = "running"
		inst.ErrorMessage = ""
		_ = database.DB.Save(&inst).Error
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "trigger_jenkins_build":
		if inst.ServiceType != "ci" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for ci services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		jenkins := k8s.NewJenkinsClient(inst.Namespace)
		jobs, err := jenkins.Jobs(ctx)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		jobName, ok := jenkinsBuildTarget(jobs, req.Target)
		if !ok {
			c.JSON(http.StatusFailedDependency, gin.H{"error": "no Jenkins jobs available to trigger", "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if err := jenkins.BuildJob(ctx, jobName); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "check_registry_health":
		if inst.ServiceType != "registry" && inst.ServiceType != "harbor" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for registry services"})
			return
		}
		path := "/v2/"
		if inst.ServiceType == "harbor" {
			path = "/api/v2.0/health"
		}
		if err := checkRegistryHealth(inst, path); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		inst.Status = "running"
		inst.ErrorMessage = ""
		_ = database.DB.Save(&inst).Error
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "delete_registry_tag":
		if inst.ServiceType != "registry" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for Docker Registry services"})
			return
		}
		repository := workspaceActionParam(req, "repository")
		if repository == "" {
			repository = strings.Trim(strings.TrimSpace(req.Target), "/")
		}
		tag := workspaceActionParam(req, "tag")
		if repository == "" || tag == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "repository and tag are required"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if _, err := k8s.NewRegistryClient(inst.Namespace).DeleteTag(ctx, repository, tag); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "check_database_connection":
		if !isSQLDatabaseService(inst.ServiceType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for mysql and postgresql services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		info, err := k8s.DiscoverDatabaseConnection(ctx, inst.Namespace, inst.ServiceType)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if err := service.CheckDatabaseConnection(ctx, info); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		inst.Status = "running"
		inst.ErrorMessage = ""
		_ = database.DB.Save(&inst).Error
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "list_databases":
		if !isSQLDatabaseService(inst.ServiceType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for mysql and postgresql services"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "create_database_backup":
		if !isSQLDatabaseService(inst.ServiceType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for mysql and postgresql services"})
			return
		}
		databaseName := workspaceActionParam(req, "database")
		if databaseName == "" {
			databaseName = strings.TrimSpace(req.Target)
		}
		if databaseName == "" {
			databaseName = defaultDatabaseName(inst.ServiceType)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		info, err := k8s.DiscoverDatabaseConnection(ctx, inst.Namespace, inst.ServiceType)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		document, summary, err := service.ExportDatabaseBackup(ctx, info, databaseName)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if _, err := k8s.StoreDatabaseBackup(ctx, inst.Namespace, inst.ServiceType, document, k8s.DatabaseBackupMetadata{
			Engine:     summary.Engine,
			Database:   summary.Database,
			CreatedAt:  summary.CreatedAt,
			TableCount: summary.TableCount,
			RowCount:   summary.RowCount,
		}); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		c.JSON(http.StatusOK, gin.H{"data": enrichDatabaseWorkspace(ctx, workspace, inst)})
		return
	case "create_database", "drop_database":
		if !isSQLDatabaseService(inst.ServiceType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for mysql and postgresql services"})
			return
		}
		databaseName := workspaceActionParam(req, "database")
		if databaseName == "" {
			databaseName = strings.TrimSpace(req.Target)
		}
		if databaseName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "database is required"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		info, err := k8s.DiscoverDatabaseConnection(ctx, inst.Namespace, inst.ServiceType)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		if req.Action == "create_database" {
			err = service.CreateDatabase(ctx, info, databaseName)
		} else {
			err = service.DropDatabase(ctx, info, databaseName)
		}
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": buildLiveToolWorkspace(app, env, inst, components)})
		return
	case "create_table", "drop_table", "insert_table_row", "update_table_row", "delete_table_row":
		if !isSQLDatabaseService(inst.ServiceType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for mysql and postgresql services"})
			return
		}
		databaseName := workspaceActionParam(req, "database")
		tableName := workspaceActionParam(req, "table")
		if databaseName == "" || tableName == "" {
			targetDatabase, targetTable, ok := parseDatabaseTableTarget(req.Target)
			if ok {
				if databaseName == "" {
					databaseName = targetDatabase
				}
				if tableName == "" {
					tableName = targetTable
				}
			}
		}
		if databaseName == "" || tableName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "database and table are required"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		info, err := k8s.DiscoverDatabaseConnection(ctx, inst.Namespace, inst.ServiceType)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		err = runDatabaseTableAction(ctx, req, info, databaseName, tableName)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		c.JSON(http.StatusOK, gin.H{"data": enrichDatabaseTablesWorkspace(ctx, workspace, inst, databaseName)})
		return
	case "list_database_tables":
		if !isSQLDatabaseService(inst.ServiceType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for mysql and postgresql services"})
			return
		}
		target := strings.TrimSpace(req.Target)
		if target == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target database is required"})
			return
		}
		workspace := buildLiveToolWorkspace(app, env, inst, components)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		c.JSON(http.StatusOK, gin.H{"data": enrichDatabaseTablesWorkspace(ctx, workspace, inst, target)})
		return
	case "list_table_columns", "preview_table_rows":
		if !isSQLDatabaseService(inst.ServiceType) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for mysql and postgresql services"})
			return
		}
		databaseName, tableName, ok := parseDatabaseTableTarget(req.Target)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "target database/table is required"})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if req.Action == "list_table_columns" {
			c.JSON(http.StatusOK, gin.H{"data": enrichTableColumnsWorkspace(ctx, workspace, inst, databaseName, tableName)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": enrichTablePreviewWorkspace(ctx, workspace, inst, databaseName, tableName)})
		return
	case "check_redis_health", "inspect_redis":
		if inst.ServiceType != "redis" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for redis services"})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		c.JSON(http.StatusOK, gin.H{"data": enrichRedisWorkspace(ctx, workspace, inst)})
		return
	case "list_redis_keys", "get_redis_key", "set_redis_key", "delete_redis_key", "expire_redis_key":
		if inst.ServiceType != "redis" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for redis services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		info, err := k8s.DiscoverRedisConnection(ctx, inst.Namespace)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		switch req.Action {
		case "list_redis_keys":
			c.JSON(http.StatusOK, gin.H{"data": enrichRedisKeysWorkspace(ctx, workspace, info, workspaceActionParam(req, "pattern"), workspaceActionInt(req, "limit", 50))})
			return
		case "get_redis_key":
			key := workspaceActionParam(req, "key")
			if key == "" {
				key = strings.TrimSpace(req.Target)
			}
			c.JSON(http.StatusOK, gin.H{"data": enrichRedisKeyWorkspace(ctx, workspace, info, key)})
			return
		case "set_redis_key":
			if err := service.SetRedisKey(ctx, info, workspaceActionParam(req, "key"), workspaceActionParam(req, "value"), workspaceActionInt(req, "ttlSeconds", 0)); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": enrichRedisWorkspace(ctx, workspace, inst)})
				return
			}
		case "delete_redis_key":
			key := workspaceActionParam(req, "key")
			if key == "" {
				key = strings.TrimSpace(req.Target)
			}
			if err := service.DeleteRedisKey(ctx, info, key); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": enrichRedisWorkspace(ctx, workspace, inst)})
				return
			}
		case "expire_redis_key":
			key := workspaceActionParam(req, "key")
			if key == "" {
				key = strings.TrimSpace(req.Target)
			}
			if err := service.ExpireRedisKey(ctx, info, key, workspaceActionInt(req, "ttlSeconds", 0)); err != nil {
				c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": enrichRedisWorkspace(ctx, workspace, inst)})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"data": enrichRedisWorkspace(ctx, workspace, inst)})
		return
	case "list_minio_buckets", "create_minio_bucket", "delete_minio_bucket", "list_minio_objects", "delete_minio_object":
		if inst.ServiceType != "minio" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for minio services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		info, err := k8s.DiscoverMinIOConnection(ctx, inst.Namespace)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		switch req.Action {
		case "list_minio_buckets":
			c.JSON(http.StatusOK, gin.H{"data": enrichMinIOWorkspace(ctx, workspace, inst)})
			return
		case "create_minio_bucket":
			err = service.CreateMinIOBucket(ctx, info, workspaceActionParam(req, "bucket"))
		case "delete_minio_bucket":
			bucket := workspaceActionParam(req, "bucket")
			if bucket == "" {
				bucket = strings.TrimSpace(req.Target)
			}
			err = service.DeleteMinIOBucket(ctx, info, bucket)
		case "list_minio_objects":
			bucket := workspaceActionParam(req, "bucket")
			if bucket == "" {
				bucket = strings.TrimSpace(req.Target)
			}
			c.JSON(http.StatusOK, gin.H{"data": enrichMinIOObjectsWorkspace(ctx, workspace, info, bucket, workspaceActionParam(req, "prefix"), workspaceActionInt(req, "limit", 50))})
			return
		case "delete_minio_object":
			bucket := workspaceActionParam(req, "bucket")
			object := workspaceActionParam(req, "object")
			if bucket == "" || object == "" {
				bucket, object, _ = parseDatabaseTableTarget(req.Target)
			}
			err = service.DeleteMinIOObject(ctx, info, bucket, object)
		}
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": enrichMinIOWorkspace(ctx, workspace, inst)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": enrichMinIOWorkspace(ctx, workspace, inst)})
		return
	case "list_mongodb_databases", "list_mongodb_collections", "create_mongodb_collection", "drop_mongodb_collection", "preview_mongodb_documents", "insert_mongodb_document", "update_mongodb_documents", "delete_mongodb_documents":
		if inst.ServiceType != "mongodb" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for mongodb services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		info, err := k8s.DiscoverMongoDBConnection(ctx, inst.Namespace)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		databaseName := workspaceActionParam(req, "database")
		collectionName := workspaceActionParam(req, "collection")
		if databaseName == "" || collectionName == "" {
			targetDatabase, targetCollection, ok := parseDatabaseTableTarget(req.Target)
			if ok {
				if databaseName == "" {
					databaseName = targetDatabase
				}
				if collectionName == "" {
					collectionName = targetCollection
				}
			}
		}
		switch req.Action {
		case "list_mongodb_databases":
			c.JSON(http.StatusOK, gin.H{"data": enrichMongoDBWorkspace(ctx, workspace, inst)})
			return
		case "list_mongodb_collections":
			if databaseName == "" {
				databaseName = strings.TrimSpace(req.Target)
			}
			c.JSON(http.StatusOK, gin.H{"data": enrichMongoDBCollectionsWorkspace(ctx, workspace, info, databaseName)})
			return
		case "preview_mongodb_documents":
			c.JSON(http.StatusOK, gin.H{"data": enrichMongoDBDocumentsWorkspace(ctx, workspace, info, databaseName, collectionName, workspaceActionInt(req, "limit", 20))})
			return
		case "create_mongodb_collection":
			err = service.CreateMongoDBCollection(ctx, info, databaseName, collectionName)
		case "drop_mongodb_collection":
			err = service.DropMongoDBCollection(ctx, info, databaseName, collectionName)
		case "insert_mongodb_document":
			err = service.InsertMongoDBDocument(ctx, info, databaseName, collectionName, workspaceActionParam(req, "document"))
		case "update_mongodb_documents":
			err = service.UpdateMongoDBDocuments(ctx, info, databaseName, collectionName, workspaceActionParam(req, "filter"), workspaceActionParam(req, "update"))
		case "delete_mongodb_documents":
			err = service.DeleteMongoDBDocuments(ctx, info, databaseName, collectionName, workspaceActionParam(req, "filter"))
		}
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": enrichMongoDBWorkspace(ctx, workspace, inst)})
			return
		}
		if databaseName != "" {
			c.JSON(http.StatusOK, gin.H{"data": enrichMongoDBCollectionsWorkspace(ctx, workspace, info, databaseName)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": enrichMongoDBWorkspace(ctx, workspace, inst)})
		return
	case "list_rabbitmq_queues", "list_rabbitmq_exchanges", "list_rabbitmq_vhosts", "list_rabbitmq_bindings",
		"create_rabbitmq_queue", "delete_rabbitmq_queue", "purge_rabbitmq_queue", "get_rabbitmq_messages",
		"create_rabbitmq_exchange", "delete_rabbitmq_exchange", "create_rabbitmq_vhost", "delete_rabbitmq_vhost",
		"create_rabbitmq_binding", "delete_rabbitmq_binding", "publish_rabbitmq_message":
		if inst.ServiceType != "rabbitmq" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for rabbitmq services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		info, err := k8s.DiscoverRabbitMQConnection(ctx, inst.Namespace)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		switch req.Action {
		case "create_rabbitmq_queue":
			err = service.CreateRabbitMQQueue(ctx, info, workspaceActionParam(req, "vhost"), workspaceActionParam(req, "queue"), workspaceActionBool(req, "durable", true))
		case "delete_rabbitmq_queue":
			queue := workspaceActionParam(req, "queue")
			vhost := workspaceActionParam(req, "vhost")
			if queue == "" {
				vhost, queue, _ = parseDatabaseTableTarget(req.Target)
			}
			err = service.DeleteRabbitMQQueue(ctx, info, vhost, queue)
		case "purge_rabbitmq_queue":
			queue := workspaceActionParam(req, "queue")
			vhost := workspaceActionParam(req, "vhost")
			if queue == "" {
				vhost, queue, _ = parseDatabaseTableTarget(req.Target)
			}
			err = service.PurgeRabbitMQQueue(ctx, info, vhost, queue)
		case "get_rabbitmq_messages":
			queue := workspaceActionParam(req, "queue")
			vhost := workspaceActionParam(req, "vhost")
			if queue == "" {
				vhost, queue, _ = parseDatabaseTableTarget(req.Target)
			}
			workspace = enrichRabbitMQWorkspace(ctx, workspace, inst)
			c.JSON(http.StatusOK, gin.H{"data": enrichRabbitMQMessagesWorkspace(ctx, workspace, info, vhost, queue, workspaceActionInt(req, "count", 10), workspaceActionBool(req, "requeue", true))})
			return
		case "create_rabbitmq_exchange":
			err = service.CreateRabbitMQExchange(ctx, info, workspaceActionParam(req, "vhost"), workspaceActionParam(req, "exchange"), workspaceActionParam(req, "type"), workspaceActionBool(req, "durable", true))
		case "delete_rabbitmq_exchange":
			exchange := workspaceActionParam(req, "exchange")
			vhost := workspaceActionParam(req, "vhost")
			if exchange == "" {
				vhost, exchange, _ = parseDatabaseTableTarget(req.Target)
			}
			err = service.DeleteRabbitMQExchange(ctx, info, vhost, exchange)
		case "create_rabbitmq_vhost":
			err = service.CreateRabbitMQVHost(ctx, info, workspaceActionParam(req, "vhost"))
		case "delete_rabbitmq_vhost":
			vhost := workspaceActionParam(req, "vhost")
			if vhost == "" {
				vhost = strings.TrimSpace(req.Target)
			}
			err = service.DeleteRabbitMQVHost(ctx, info, vhost)
		case "create_rabbitmq_binding":
			arguments, argErr := workspaceActionJSONMap(req, "arguments")
			if argErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": argErr.Error(), "data": enrichRabbitMQWorkspace(ctx, workspace, inst)})
				return
			}
			vhost := workspaceActionParam(req, "vhost")
			source := workspaceActionParam(req, "source")
			if source == "" && strings.TrimSpace(req.Target) != "" {
				targetVHost, targetSource, ok := parseDatabaseTableTarget(req.Target)
				if ok {
					vhost = valueOrFallback(vhost, targetVHost)
					source = targetSource
				}
			}
			err = service.CreateRabbitMQBinding(ctx, info,
				vhost,
				source,
				workspaceActionParam(req, "destinationType"),
				workspaceActionParam(req, "destination"),
				workspaceActionParam(req, "routingKey"),
				arguments,
			)
		case "delete_rabbitmq_binding":
			vhost := workspaceActionParam(req, "vhost")
			source := workspaceActionParam(req, "source")
			destinationType := workspaceActionParam(req, "destinationType")
			destination := workspaceActionParam(req, "destination")
			propertiesKey := workspaceActionParam(req, "propertiesKey")
			if propertiesKey == "" {
				vhost, source, destinationType, destination, propertiesKey, _ = parseRabbitMQBindingTarget(req.Target)
			}
			err = service.DeleteRabbitMQBinding(ctx, info, vhost, source, destinationType, destination, propertiesKey)
		case "publish_rabbitmq_message":
			vhost := workspaceActionParam(req, "vhost")
			exchange := workspaceActionParam(req, "exchange")
			routingKey := workspaceActionParam(req, "routingKey")
			if exchange == "" && strings.TrimSpace(req.Target) != "" {
				targetVHost, targetName, ok := parseDatabaseTableTarget(req.Target)
				if ok {
					vhost = valueOrFallback(vhost, targetVHost)
					if routingKey == "" {
						routingKey = targetName
					} else {
						exchange = targetName
					}
				}
			}
			properties, propErr := workspaceActionJSONMap(req, "properties")
			if propErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": propErr.Error(), "data": enrichRabbitMQWorkspace(ctx, workspace, inst)})
				return
			}
			_, err = service.PublishRabbitMQMessage(ctx, info, vhost, exchange, routingKey, workspaceActionParam(req, "payload"), properties)
		}
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": enrichRabbitMQWorkspace(ctx, workspace, inst)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": enrichRabbitMQWorkspace(ctx, workspace, inst)})
		return
	case "list_kafka_topics", "create_kafka_topic", "delete_kafka_topic", "read_kafka_messages", "produce_kafka_message":
		if inst.ServiceType != "kafka" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for kafka services"})
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		info, err := k8s.DiscoverKafkaConnection(ctx, inst.Namespace)
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": buildLiveToolWorkspace(app, env, inst, components)})
			return
		}
		workspace := service.BuildToolWorkspace(app, env, inst, components)
		switch req.Action {
		case "create_kafka_topic":
			err = service.CreateKafkaTopic(ctx, info, workspaceActionParam(req, "topic"), workspaceActionInt(req, "partitions", 1))
		case "delete_kafka_topic":
			topic := workspaceActionParam(req, "topic")
			if topic == "" {
				topic = strings.TrimSpace(req.Target)
			}
			err = service.DeleteKafkaTopic(ctx, info, topic)
		case "read_kafka_messages":
			topic := workspaceActionParam(req, "topic")
			if topic == "" {
				topic = strings.TrimSpace(req.Target)
			}
			workspace = enrichKafkaWorkspace(ctx, workspace, inst)
			c.JSON(http.StatusOK, gin.H{"data": enrichKafkaMessagesWorkspace(ctx, workspace, info, topic, workspaceActionInt(req, "partition", -1), workspaceActionParam(req, "offset"), workspaceActionInt(req, "limit", 10))})
			return
		case "produce_kafka_message":
			topic := workspaceActionParam(req, "topic")
			if topic == "" {
				topic = strings.TrimSpace(req.Target)
			}
			err = service.ProduceKafkaMessage(ctx, info, topic, workspaceActionParam(req, "key"), workspaceActionParam(req, "value"), workspaceActionInt(req, "partition", -1))
		}
		if err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error(), "data": enrichKafkaWorkspace(ctx, workspace, inst)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": enrichKafkaWorkspace(ctx, workspace, inst)})
		return
	case "reconcile_gitops":
		if inst.ServiceType != "git" && inst.ServiceType != "deploy" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "action is only supported for git and deploy services"})
			return
		}
		workspace, reconcileErrs := reconcileEnvironmentGitOps(app, env, inst, components)
		if len(reconcileErrs) > 0 {
			c.JSON(http.StatusFailedDependency, gin.H{"error": strings.Join(reconcileErrs, "; "), "data": workspace})
			return
		}
		c.JSON(http.StatusOK, gin.H{"data": workspace})
		return
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported workspace action"})
		return
	}
}

func buildLiveToolWorkspace(app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) service.ToolWorkspace {
	workspace := service.BuildToolWorkspace(app, env, inst, components)
	if inst.Namespace == "" {
		return workspace
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	workspace = enrichExternalAccessConfig(ctx, workspace, inst)

	switch inst.ServiceType {
	case "git":
		return applyWorkspaceExternalAccess(ctx, enrichGiteaWorkspace(ctx, workspace, inst), inst)
	case "deploy":
		return applyWorkspaceExternalAccess(ctx, enrichArgoCDWorkspace(ctx, workspace, inst), inst)
	case "monitor":
		return applyWorkspaceExternalAccess(ctx, enrichGrafanaWorkspace(ctx, workspace, app, env, inst, components), inst)
	case "registry":
		return applyWorkspaceExternalAccess(ctx, enrichRegistryWorkspace(ctx, workspace, inst), inst)
	case "harbor":
		return applyWorkspaceExternalAccess(ctx, enrichHarborWorkspace(ctx, workspace, inst), inst)
	case "ci":
		return applyWorkspaceExternalAccess(ctx, enrichJenkinsWorkspace(ctx, workspace, inst), inst)
	case "log":
		return applyWorkspaceExternalAccess(ctx, enrichLokiWorkspace(ctx, workspace, app, env, inst), inst)
	default:
		if isSQLDatabaseService(inst.ServiceType) {
			return applyWorkspaceExternalAccess(ctx, enrichDatabaseWorkspace(ctx, workspace, inst), inst)
		}
		if inst.ServiceType == "redis" {
			return applyWorkspaceExternalAccess(ctx, enrichRedisWorkspace(ctx, workspace, inst), inst)
		}
		if inst.ServiceType == "minio" {
			return applyWorkspaceExternalAccess(ctx, enrichMinIOWorkspace(ctx, workspace, inst), inst)
		}
		if inst.ServiceType == "mongodb" {
			return applyWorkspaceExternalAccess(ctx, enrichMongoDBWorkspace(ctx, workspace, inst), inst)
		}
		if inst.ServiceType == "rabbitmq" {
			return applyWorkspaceExternalAccess(ctx, enrichRabbitMQWorkspace(ctx, workspace, inst), inst)
		}
		if inst.ServiceType == "kafka" {
			return applyWorkspaceExternalAccess(ctx, enrichKafkaWorkspace(ctx, workspace, inst), inst)
		}
		return applyWorkspaceExternalAccess(ctx, workspace, inst)
	}
}

func enrichExternalAccessConfig(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	endpoints, err := k8s.ListNamespaceExternalEndpoints(ctx, inst.Namespace)
	if err != nil {
		workspace.Config = append(workspace.Config, service.ToolWorkspaceConfig{Label: "外部访问地址", Value: "查询失败: " + err.Error()})
		return workspace
	}
	if len(endpoints) == 0 {
		workspace.Config = append(workspace.Config, service.ToolWorkspaceConfig{Label: "外部访问地址", Value: "未配置 NodePort/Ingress/LoadBalancer"})
		return workspace
	}
	for i, endpoint := range endpoints {
		label := "外部访问地址"
		if len(endpoints) > 1 {
			label = fmt.Sprintf("外部访问地址 %d", i+1)
		}
		workspace.Config = append(workspace.Config, service.ToolWorkspaceConfig{
			Label: label,
			Value: endpoint.URL,
		})
	}
	return workspace
}

func applyWorkspaceExternalAccess(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	baseURL := preferredWorkspaceExternalBaseURL(ctx, inst.Namespace)
	if baseURL == "" {
		return workspace
	}
	for i := range workspace.Resources {
		workspace.Resources[i] = applyResourceExternalAccess(workspace.Resources[i], inst, baseURL)
	}
	return workspace
}

func preferredWorkspaceExternalBaseURL(ctx context.Context, namespace string) string {
	endpoints, err := k8s.ListNamespaceExternalEndpoints(ctx, namespace)
	if err != nil || len(endpoints) == 0 {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(endpoints[0].URL), "/")
}

func applyResourceExternalAccess(resource service.ToolWorkspaceResource, inst model.ServiceInstallation, baseURL string) service.ToolWorkspaceResource {
	if rewritten := externalizedWorkspaceURL(resource.ExternalURL, inst, baseURL); rewritten != resource.ExternalURL {
		if resource.Annotations == nil {
			resource.Annotations = map[string]interface{}{}
		}
		resource.Annotations["proxyURL"] = resource.ExternalURL
		resource.ExternalURL = rewritten
	}
	for i := range resource.Children {
		resource.Children[i] = applyResourceExternalAccess(resource.Children[i], inst, baseURL)
	}
	return resource
}

func externalizedWorkspaceURL(raw string, inst model.ServiceInstallation, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || baseURL == "" {
		return raw
	}
	prefix := serviceProxyURL(inst, "")
	if prefix == "" || !strings.HasPrefix(raw, prefix) {
		return raw
	}
	rest := strings.TrimPrefix(raw, prefix)
	rest = "/" + strings.TrimLeft(rest, "/")
	return strings.TrimRight(baseURL, "/") + rest
}

func isRuntimeResourceWorkspace(serviceType string) bool {
	switch serviceType {
	case "mysql", "postgresql", "mongodb", "redis", "rabbitmq", "kafka", "minio":
		return true
	default:
		return false
	}
}

func isSQLDatabaseService(serviceType string) bool {
	return serviceType == "mysql" || serviceType == "postgresql"
}

func defaultDatabaseName(serviceType string) string {
	if serviceType == "mysql" {
		return "mysql"
	}
	return "postgres"
}

func workspaceActionParam(req WorkspaceActionRequest, key string) string {
	if req.Params == nil {
		return ""
	}
	return strings.TrimSpace(req.Params[key])
}

func workspaceActionInt(req WorkspaceActionRequest, key string, fallback int) int {
	value := workspaceActionParam(req, key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func workspaceActionBool(req WorkspaceActionRequest, key string, fallback bool) bool {
	value := strings.ToLower(workspaceActionParam(req, key))
	if value == "" {
		return fallback
	}
	return value == "true" || value == "1" || value == "yes" || value == "on"
}

func workspaceActionJSONMap(req WorkspaceActionRequest, key string) (map[string]interface{}, error) {
	value := workspaceActionParam(req, key)
	if value == "" {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		return nil, fmt.Errorf("%s must be valid JSON object: %w", key, err)
	}
	if out == nil {
		out = map[string]interface{}{}
	}
	return out, nil
}

func runDatabaseTableAction(ctx context.Context, req WorkspaceActionRequest, info k8s.DatabaseConnectionInfo, databaseName, tableName string) error {
	switch req.Action {
	case "create_table":
		columns, err := service.ParseTableColumns(workspaceActionParam(req, "columns"))
		if err != nil {
			return err
		}
		return service.CreateTable(ctx, info, databaseName, tableName, columns)
	case "drop_table":
		return service.DropTable(ctx, info, databaseName, tableName)
	case "insert_table_row":
		values, err := service.ParseSQLObject(workspaceActionParam(req, "values"))
		if err != nil {
			return err
		}
		return service.InsertTableRow(ctx, info, databaseName, tableName, values)
	case "update_table_row":
		values, err := service.ParseSQLObject(workspaceActionParam(req, "values"))
		if err != nil {
			return err
		}
		where, err := service.ParseSQLObject(workspaceActionParam(req, "where"))
		if err != nil {
			return err
		}
		return service.UpdateTableRow(ctx, info, databaseName, tableName, values, where)
	case "delete_table_row":
		where, err := service.ParseSQLObject(workspaceActionParam(req, "where"))
		if err != nil {
			return err
		}
		return service.DeleteTableRow(ctx, info, databaseName, tableName, where)
	default:
		return fmt.Errorf("unsupported database action %s", req.Action)
	}
}

func enrichRuntimeResourceWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	runtimeResources, err := k8s.ListNamespaceRuntimeResources(ctx, inst.Namespace)
	if err != nil || len(runtimeResources) == 0 {
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(runtimeResources))
	for _, resource := range runtimeResources {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        resource.Name,
			Type:        resource.Type,
			Status:      resource.Status,
			Description: resource.Description,
		})
	}
	workspace.Resources = resources
	return workspace
}

func enrichDatabaseWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	info, err := k8s.DiscoverDatabaseConnection(ctx, inst.Namespace, inst.ServiceType)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	databases, err := service.ListDatabases(ctx, info)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	backups, backupErr := k8s.ListDatabaseBackups(ctx, inst.Namespace, inst.ServiceType)
	resources := make([]service.ToolWorkspaceResource, 0, len(databases)+len(backups)+1)
	resources = append(resources, databaseConnectionResource("Ready", "Database connection is healthy."))
	resources = append(resources, databaseCatalogResources(databases)...)
	for _, backup := range backups {
		resources = append(resources, databaseBackupResource(backup))
	}
	workspace.Resources = resources
	workspace.Config = append(workspace.Config, service.ToolWorkspaceConfig{Label: "备份存储", Value: publicDatabaseBackupStorage("")})
	if backupErr != nil {
		workspace.Config = append(workspace.Config, service.ToolWorkspaceConfig{Label: "备份状态", Value: "查询失败: " + backupErr.Error()})
	}
	return workspace
}

func enrichDatabaseTablesWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation, databaseName string) service.ToolWorkspace {
	info, err := k8s.DiscoverDatabaseConnection(ctx, inst.Namespace, inst.ServiceType)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	resources, err := databaseCatalogWithTables(ctx, info, databaseName)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	workspace.Resources = resources
	return workspace
}

func enrichTableColumnsWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation, databaseName, tableName string) service.ToolWorkspace {
	info, err := k8s.DiscoverDatabaseConnection(ctx, inst.Namespace, inst.ServiceType)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	columns, err := service.ListTableColumns(ctx, info, databaseName, tableName)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	resources, err := databaseTableContextResources(ctx, info, databaseName)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	for _, column := range columns {
		description := column.DataType
		if column.Nullable != "" {
			description += ", nullable=" + column.Nullable
		}
		if column.Default != "" {
			description += ", default=" + column.Default
		}
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        column.Name,
			Type:        "Column",
			Status:      "Ready",
			Description: description,
		})
	}
	workspace.Resources = resources
	return workspace
}

func enrichTablePreviewWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation, databaseName, tableName string) service.ToolWorkspace {
	info, err := k8s.DiscoverDatabaseConnection(ctx, inst.Namespace, inst.ServiceType)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	rows, err := service.PreviewTableRows(ctx, info, databaseName, tableName, 20)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	resources, err := databaseTableContextResources(ctx, info, databaseName)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{databaseConnectionResource("Partial", err.Error())}
		return workspace
	}
	for i, row := range rows {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        fmt.Sprintf("row-%d", i+1),
			Type:        "Row",
			Status:      "Ready",
			Description: service.MarshalPreviewRow(row),
		})
	}
	workspace.Resources = resources
	return workspace
}

func databaseTableContextResources(ctx context.Context, info k8s.DatabaseConnectionInfo, databaseName string) ([]service.ToolWorkspaceResource, error) {
	return databaseCatalogWithTables(ctx, info, databaseName)
}

func databaseCatalogWithTables(ctx context.Context, info k8s.DatabaseConnectionInfo, databaseName string) ([]service.ToolWorkspaceResource, error) {
	databases, err := service.ListDatabases(ctx, info)
	if err != nil {
		return nil, err
	}
	tables, err := service.ListDatabaseTables(ctx, info, databaseName)
	if err != nil {
		return nil, err
	}
	resources := databaseCatalogResources(databases)
	return appendDatabaseTableResources(resources, databaseName, tables), nil
}

func databaseCatalogResources(databases []string) []service.ToolWorkspaceResource {
	resources := make([]service.ToolWorkspaceResource, 0, len(databases))
	for _, databaseName := range databases {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        databaseName,
			Type:        "Database",
			Status:      "Ready",
			Description: "Database catalog",
			Actions: []service.ToolWorkspaceAction{
				{Key: "list_database_tables", Label: "表", Description: "查看该数据库的表。", Target: databaseName},
				{Key: "drop_database", Label: "删除", Description: "删除该数据库。", Tone: "danger", Target: databaseName},
			},
		})
	}
	return resources
}

func appendDatabaseTableResources(resources []service.ToolWorkspaceResource, databaseName string, tables []service.DatabaseTable) []service.ToolWorkspaceResource {
	for _, table := range tables {
		target := databaseTableTarget(databaseName, table.Name)
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        table.Name,
			Type:        "Table",
			Status:      "Ready",
			Description: table.Type,
			Annotations: map[string]interface{}{"database": databaseName},
			Actions: []service.ToolWorkspaceAction{
				{Key: "list_table_columns", Label: "字段", Description: "查看该表字段。", Target: target},
				{Key: "preview_table_rows", Label: "预览", Description: "预览该表前 20 行。", Target: target},
				{Key: "insert_table_row", Label: "新增行", Description: "向该表插入一行。", Target: target, Fields: []service.ToolWorkspaceActionField{{Name: "values", Label: "数据 JSON", Type: "textarea", Required: true, Placeholder: `{"column":"value"}`}}},
				{Key: "update_table_row", Label: "更新行", Description: "按条件更新该表行。", Target: target, Fields: []service.ToolWorkspaceActionField{
					{Name: "values", Label: "数据 JSON", Type: "textarea", Required: true, Placeholder: `{"name":"updated"}`},
					{Name: "where", Label: "WHERE JSON", Type: "textarea", Required: true, Placeholder: `{"id":"1"}`},
				}},
				{Key: "delete_table_row", Label: "删除行", Description: "按条件删除该表行。", Tone: "danger", Target: target, Fields: []service.ToolWorkspaceActionField{{Name: "where", Label: "WHERE JSON", Type: "textarea", Required: true, Placeholder: `{"id":"1"}`}}},
				{Key: "drop_table", Label: "删表", Description: "删除该表。", Tone: "danger", Target: target},
			},
		})
	}
	return resources
}

func databaseTableTarget(databaseName, tableName string) string {
	return databaseName + "\t" + tableName
}

func parseDatabaseTableTarget(target string) (string, string, bool) {
	parts := strings.SplitN(target, "\t", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func rabbitMQBindingTarget(vhost, source, destinationType, destination, propertiesKey string) string {
	return strings.Join([]string{vhost, source, destinationType, destination, propertiesKey}, "\t")
}

func parseRabbitMQBindingTarget(target string) (string, string, string, string, string, bool) {
	parts := strings.SplitN(target, "\t", 5)
	if len(parts) != 5 {
		return "", "", "", "", "", false
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if parts[0] == "" || parts[3] == "" || parts[4] == "" {
		return "", "", "", "", "", false
	}
	return parts[0], parts[1], parts[2], parts[3], parts[4], true
}

func databaseConnectionResource(status, description string) service.ToolWorkspaceResource {
	return service.ToolWorkspaceResource{
		Name:        "database-connection",
		Type:        "Connection",
		Status:      status,
		Description: description,
	}
}

func databaseBackupResource(backup k8s.DatabaseBackupMetadata) service.ToolWorkspaceResource {
	return service.ToolWorkspaceResource{
		Name:        backup.Name,
		Type:        "Backup",
		Status:      "Ready",
		Description: fmt.Sprintf("%s backup for %s, %d tables / %d rows", backup.Engine, backup.Database, backup.TableCount, backup.RowCount),
		Annotations: map[string]interface{}{
			"database":            backup.Database,
			"engine":              backup.Engine,
			"createdAt":           backup.CreatedAt,
			"storage":             publicDatabaseBackupStorage(backup.Storage),
			"originalSizeBytes":   backup.OriginalSizeBytes,
			"compressedSizeBytes": backup.CompressedSizeBytes,
			"size":                humanSize(backup.CompressedSizeBytes),
			"tables":              backup.TableCount,
			"rows":                backup.RowCount,
		},
	}
}

func publicDatabaseBackupStorage(storage string) string {
	storage = strings.TrimSpace(storage)
	if storage == "" || strings.EqualFold(storage, "Kubernetes Secret") {
		return "平台备份"
	}
	return storage
}

func humanSize(value int) string {
	if value >= 1024*1024 {
		return fmt.Sprintf("%.1f MiB", float64(value)/(1024*1024))
	}
	if value >= 1024 {
		return fmt.Sprintf("%.1f KiB", float64(value)/1024)
	}
	return fmt.Sprintf("%d B", value)
}

func enrichArgoCDWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	apps, err := k8s.ListArgoCDApplications(ctx, inst.Namespace)
	if err != nil || len(apps) == 0 {
		return workspace
	}
	argocd := newArgoCDWorkspaceClient(inst.Namespace)
	resources := make([]service.ToolWorkspaceResource, 0, len(apps))
	for _, app := range apps {
		status := app.SyncStatus
		if app.HealthStatus != "" && app.HealthStatus != "Unknown" {
			status = app.SyncStatus + "/" + app.HealthStatus
		}
		treeResources := app.Resources
		treeSource := "application-status"
		if apiResources, treeErr := argocd.ResourceTree(ctx, app.Name); treeErr == nil && len(apiResources) > 0 {
			treeResources = apiResources
			treeSource = "argocd-resource-tree-api"
		} else if treeErr != nil {
			log.Printf("[ArgoCDWorkspace] resource tree fallback app=%s namespace=%s: %v", app.Name, inst.Namespace, treeErr)
		}

		annot := map[string]interface{}{
			"repoURL":    app.RepoURL,
			"path":       app.Path,
			"syncStatus": app.SyncStatus,
			"health":     app.HealthStatus,
			"namespace":  app.Namespace,
			"server":     app.Server,
			"revision":   app.Revision,
			"resources":  countArgoCDResources(treeResources),
			"treeSource": treeSource,
		}
		descriptionRepoURL := app.RepoURL
		if externalRepoURL := externalRepoURLForDisplay(ctx, app.RepoURL); externalRepoURL != "" {
			annot["externalRepoURL"] = externalRepoURL
			descriptionRepoURL = externalRepoURL
		}
		description := "ArgoCD Application"
		source := strings.Trim(strings.TrimSpace(descriptionRepoURL)+" "+strings.TrimSpace(app.Path), " ")
		if source != "" {
			description = "Source: " + source
		}
		treeNodes, treeEdges := argoCDResourcesToWorkspaceGraph(treeResources)
		if len(treeNodes) > 0 {
			annot["treeNodes"] = treeNodes
			annot["treeEdges"] = treeEdges
		}

		childResources := argoCDResourcesToWorkspaceResources(treeResources)

		resources = append(resources, service.ToolWorkspaceResource{
			Name:        app.Name,
			Type:        "Application",
			Status:      status,
			Description: description,
			ExternalURL: argoCDApplicationExternalURL(inst, app.Name),
			Annotations: annot,
			Actions: []service.ToolWorkspaceAction{
				{Key: "sync_argocd_application", Label: "同步", Description: "触发该 ArgoCD Application 同步。"},
				{Key: "delete_argocd_application", Label: "删除", Description: "删除该 ArgoCD Application。", Tone: "danger"},
			},
			Children: childResources,
		})
	}
	workspace.Resources = resources
	return workspace
}

func argoCDApplicationExternalURL(inst model.ServiceInstallation, application string) string {
	application = strings.TrimSpace(application)
	if application == "" {
		return ""
	}
	return serviceProxyURL(inst, "/applications/"+url.PathEscape(inst.Namespace)+"/"+url.PathEscape(application)+"?view=tree&resource=")
}

func externalRepoURLForDisplay(ctx context.Context, repoURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	namespace := namespaceFromClusterServiceHost(parsed.Hostname())
	if namespace == "" {
		return ""
	}
	baseURL := preferredWorkspaceExternalBaseURL(ctx, namespace)
	if baseURL == "" {
		return ""
	}
	path := parsed.EscapedPath()
	if path == "" {
		path = parsed.Path
	}
	if path == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func namespaceFromClusterServiceHost(host string) string {
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	parts := strings.Split(host, ".")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i+1] == "svc" && parts[i+2] == "cluster" && i >= 1 {
			return parts[i]
		}
		if parts[i+1] == "svc" && i >= 1 {
			return parts[i]
		}
	}
	return ""
}

func countArgoCDResources(resources []k8s.ArgoCDResource) int {
	total := 0
	for _, resource := range resources {
		total++
		total += countArgoCDResources(resource.Children)
	}
	return total
}

func argoCDResourcesToWorkspaceResources(resources []k8s.ArgoCDResource) []service.ToolWorkspaceResource {
	out := make([]service.ToolWorkspaceResource, 0, len(resources))
	for _, r := range resources {
		status := r.Status
		if strings.TrimSpace(status) == "" {
			status = r.Health
		}
		out = append(out, service.ToolWorkspaceResource{
			Name:        r.Name,
			Type:        r.Kind,
			Status:      status,
			Description: r.Namespace,
			Annotations: map[string]interface{}{
				"health":     r.Health,
				"namespace":  r.Namespace,
				"group":      r.Group,
				"version":    r.Version,
				"uid":        r.UID,
				"key":        argoCDResourceKey(r),
				"parentRefs": argoCDParentRefsAnnotation(r.ParentRefs),
				"orphaned":   r.Orphaned,
			},
			Children: argoCDResourcesToWorkspaceResources(r.Children),
		})
	}
	return out
}

func argoCDResourcesToWorkspaceGraph(resources []k8s.ArgoCDResource) ([]service.ToolWorkspaceResource, []map[string]string) {
	nodes := make([]service.ToolWorkspaceResource, 0)
	edges := make([]map[string]string, 0)
	seenNodes := map[string]bool{}
	seenEdges := map[string]bool{}

	var walk func(resource k8s.ArgoCDResource, fallbackParentKey string)
	walk = func(resource k8s.ArgoCDResource, fallbackParentKey string) {
		key := argoCDResourceKey(resource)
		if key == "" {
			return
		}
		if !seenNodes[key] {
			status := strings.TrimSpace(resource.Status)
			if status == "" {
				status = resource.Health
			}
			nodes = append(nodes, service.ToolWorkspaceResource{
				Name:        resource.Name,
				Type:        resource.Kind,
				Status:      status,
				Description: resource.Namespace,
				Annotations: map[string]interface{}{
					"health":     resource.Health,
					"namespace":  resource.Namespace,
					"group":      resource.Group,
					"version":    resource.Version,
					"uid":        resource.UID,
					"key":        key,
					"parentRefs": argoCDParentRefsAnnotation(resource.ParentRefs),
					"orphaned":   resource.Orphaned,
				},
			})
			seenNodes[key] = true
		}

		hasParentRef := false
		for _, ref := range resource.ParentRefs {
			parentKey := argoCDResourceRefKey(ref)
			if parentKey == "" {
				continue
			}
			hasParentRef = true
			addArgoCDWorkspaceEdge(&edges, seenEdges, parentKey, key)
		}
		if !hasParentRef && fallbackParentKey != "" {
			addArgoCDWorkspaceEdge(&edges, seenEdges, fallbackParentKey, key)
		}
		for _, child := range resource.Children {
			walk(child, key)
		}
	}

	for _, resource := range resources {
		walk(resource, "")
	}
	return nodes, edges
}

func addArgoCDWorkspaceEdge(edges *[]map[string]string, seen map[string]bool, from, to string) {
	if from == "" || to == "" || from == to {
		return
	}
	key := from + "->" + to
	if seen[key] {
		return
	}
	seen[key] = true
	*edges = append(*edges, map[string]string{"from": from, "to": to})
}

func argoCDParentRefsAnnotation(refs []k8s.ArgoCDResourceRef) []map[string]string {
	out := make([]map[string]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, map[string]string{
			"kind":      ref.Kind,
			"name":      ref.Name,
			"namespace": ref.Namespace,
			"group":     ref.Group,
			"uid":       ref.UID,
			"key":       argoCDResourceRefKey(ref),
		})
	}
	return out
}

func argoCDResourceKey(resource k8s.ArgoCDResource) string {
	if strings.TrimSpace(resource.UID) != "" {
		return "uid:" + strings.TrimSpace(resource.UID)
	}
	return strings.Join([]string{
		strings.TrimSpace(resource.Group),
		strings.TrimSpace(resource.Kind),
		strings.TrimSpace(resource.Namespace),
		strings.TrimSpace(resource.Name),
	}, "/")
}

func argoCDResourceRefKey(ref k8s.ArgoCDResourceRef) string {
	if strings.TrimSpace(ref.UID) != "" {
		return "uid:" + strings.TrimSpace(ref.UID)
	}
	return strings.Join([]string{
		strings.TrimSpace(ref.Group),
		strings.TrimSpace(ref.Kind),
		strings.TrimSpace(ref.Namespace),
		strings.TrimSpace(ref.Name),
	}, "/")
}

func enrichGiteaWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	repos, ok := getCachedGiteaWorkspaceRepositories(inst)
	if !ok {
		gitea := newGiteaWorkspaceClient(inst.Namespace)
		var err error
		repos, err = gitea.Repositories(ctx)
		if err != nil || len(repos) == 0 {
			return workspace
		}
		setCachedGiteaWorkspaceRepositories(inst, repos)
	}
	if len(workspace.Resources) == 0 {
		resources := make([]service.ToolWorkspaceResource, len(repos))
		for i, repo := range repos {
			resources[i] = buildGiteaRepositoryResource(inst, repo)
		}
		workspace.Resources = resources
		return workspace
	}
	workspace.Resources = mergeGiteaRepositoryMetadata(inst, workspace.Resources, repos)
	return workspace
}

func getCachedGiteaWorkspaceRepositories(inst model.ServiceInstallation) ([]k8s.GiteaRepository, bool) {
	key := giteaWorkspaceCacheKey(inst)
	if key == "" {
		return nil, false
	}
	giteaWorkspaceCacheMu.Lock()
	defer giteaWorkspaceCacheMu.Unlock()
	cached, ok := giteaWorkspaceCache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(cached.expiresAt) {
		delete(giteaWorkspaceCache, key)
		return nil, false
	}
	return cloneGiteaRepositories(cached.repositories), true
}

func setCachedGiteaWorkspaceRepositories(inst model.ServiceInstallation, repositories []k8s.GiteaRepository) {
	key := giteaWorkspaceCacheKey(inst)
	if key == "" || len(repositories) == 0 {
		return
	}
	giteaWorkspaceCacheMu.Lock()
	defer giteaWorkspaceCacheMu.Unlock()
	giteaWorkspaceCache[key] = cachedGiteaWorkspace{
		repositories: cloneGiteaRepositories(repositories),
		expiresAt:    time.Now().Add(giteaWorkspaceCacheTTL),
	}
}

func clearGiteaWorkspaceCache() {
	giteaWorkspaceCacheMu.Lock()
	defer giteaWorkspaceCacheMu.Unlock()
	giteaWorkspaceCache = map[string]cachedGiteaWorkspace{}
}

func giteaWorkspaceCacheKey(inst model.ServiceInstallation) string {
	if inst.ID <= 0 || inst.Namespace == "" {
		return ""
	}
	return fmt.Sprintf("%d/%s", inst.ID, inst.Namespace)
}

func cloneWorkspaceResources(resources []service.ToolWorkspaceResource) []service.ToolWorkspaceResource {
	if resources == nil {
		return nil
	}
	cloned := make([]service.ToolWorkspaceResource, len(resources))
	for i, resource := range resources {
		cloned[i] = resource
		if resource.Annotations != nil {
			cloned[i].Annotations = make(map[string]interface{}, len(resource.Annotations))
			for key, value := range resource.Annotations {
				cloned[i].Annotations[key] = value
			}
		}
		if resource.Actions != nil {
			cloned[i].Actions = append([]service.ToolWorkspaceAction(nil), resource.Actions...)
		}
		cloned[i].Children = cloneWorkspaceResources(resource.Children)
	}
	return cloned
}

func cloneGiteaRepositories(repositories []k8s.GiteaRepository) []k8s.GiteaRepository {
	if repositories == nil {
		return nil
	}
	return append([]k8s.GiteaRepository(nil), repositories...)
}

func mergeGiteaRepositoryMetadata(inst model.ServiceInstallation, resources []service.ToolWorkspaceResource, repos []k8s.GiteaRepository) []service.ToolWorkspaceResource {
	byName := make(map[string]k8s.GiteaRepository, len(repos))
	for _, repo := range repos {
		if repo.Name != "" {
			byName[repo.Name] = repo
		}
		for _, raw := range []string{repo.CloneURL, repo.HTMLURL} {
			if name := giteaRepositoryNameFromURL(raw); name != "" {
				byName[name] = repo
			}
		}
	}

	merged := cloneWorkspaceResources(resources)
	for i := range merged {
		if merged[i].Type != "Repository" {
			continue
		}
		repoName := workspaceResourceRepositoryName(merged[i])
		repo, ok := byName[repoName]
		if !ok {
			continue
		}
		merged[i] = mergeGiteaRepositoryResource(inst, merged[i], repo)
	}
	return merged
}

func mergeGiteaRepositoryResource(inst model.ServiceInstallation, resource service.ToolWorkspaceResource, repo k8s.GiteaRepository) service.ToolWorkspaceResource {
	actual := buildGiteaRepositoryResource(inst, repo)
	if actual.ExternalURL != "" {
		resource.ExternalURL = actual.ExternalURL
	}
	if resource.Annotations == nil {
		resource.Annotations = map[string]interface{}{}
	}
	for key, value := range actual.Annotations {
		if key == "repositoryRole" || key == "path" || key == "defaultPath" {
			continue
		}
		resource.Annotations[key] = value
	}
	if paths := stringInterfaceSlice(resource.Annotations["componentPaths"]); len(paths) == 1 {
		resource.Name = strings.TrimSuffix(repo.Name, ".git") + "/" + strings.Trim(paths[0], "/")
		resource.Annotations["path"] = paths[0]
		resource.Annotations["defaultPath"] = paths[0]
		if components := stringInterfaceSlice(resource.Annotations["components"]); len(components) == 1 {
			target := strings.TrimPrefix(strings.Trim(paths[0], "/"), "components/")
			if target == "" {
				target = components[0]
			}
			resource.Actions = []service.ToolWorkspaceAction{
				{Key: "reconcile_gitops", Label: "修复组件交付", Description: "重新生成该组件的交付内容。", Target: target},
			}
		}
	}
	return resource
}

func stringInterfaceSlice(value interface{}) []string {
	switch items := value.(type) {
	case []string:
		out := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(items))
		for _, item := range items {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func workspaceResourceRepositoryName(resource service.ToolWorkspaceResource) string {
	if resource.Annotations != nil {
		for _, key := range []string{"cloneURL", "htmlURL"} {
			if name := giteaRepositoryNameFromURL(fmt.Sprint(resource.Annotations[key])); name != "" {
				return name
			}
		}
	}
	if name := giteaRepositoryNameFromURL(resource.ExternalURL); name != "" {
		return name
	}
	name := strings.TrimSpace(resource.Name)
	if idx := strings.Index(name, "/"); idx > 0 {
		return name[:idx]
	}
	return name
}

func giteaRepositoryNameFromURL(raw string) string {
	raw = strings.TrimSpace(strings.TrimSuffix(raw, ".git"))
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	path := raw
	if err == nil && parsed.Path != "" {
		path = parsed.Path
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func buildGiteaRepositoryResource(inst model.ServiceInstallation, repo k8s.GiteaRepository) service.ToolWorkspaceResource {
	externalURL := serviceProxyURL(inst, giteaRepoBrowserPath(repo))
	branch := repo.DefaultBranch
	if branch == "" {
		branch = "main"
	}
	updated := repo.UpdatedAt
	if len(updated) > 10 {
		updated = updated[:10]
	}
	return service.ToolWorkspaceResource{
		Name:        repo.Name,
		Type:        "Repository",
		Status:      "Ready",
		Description: branch,
		ExternalURL: externalURL,
		Annotations: map[string]interface{}{
			"branch":   branch,
			"stars":    repo.Stars,
			"forks":    repo.Forks,
			"issues":   repo.OpenIssues,
			"updated":  updated,
			"language": repo.Language,
			"private":  repo.Private,
			"size":     repo.Size,
			"cloneURL": valueOrFallback(repo.CloneURL, repo.HTMLURL+".git"),
			"htmlURL":  valueOrFallback(repo.HTMLURL, strings.TrimSuffix(repo.CloneURL, ".git")),
		},
		Actions: []service.ToolWorkspaceAction{
			{Key: "reconcile_gitops", Label: "修复仓库", Description: "重新生成该仓库的 PAAP 管理内容。", Target: repo.Name},
		},
	}
}

func giteaRepoBrowserPath(repo k8s.GiteaRepository) string {
	for _, raw := range []string{repo.HTMLURL, strings.TrimSuffix(repo.CloneURL, ".git")} {
		parsed, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || parsed.Path == "" {
			continue
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 2 {
			return strings.Join(parts[len(parts)-2:], "/")
		}
	}
	if repo.Name == "" {
		return ""
	}
	return "paap/" + repo.Name
}

func giteaRepositoryChildren(ctx context.Context, gitea giteaWorkspaceClient, repoName, branch string) []service.ToolWorkspaceResource {
	return giteaRepositoryTree(ctx, gitea, repoName, "", branch, 0)
}

func giteaRepositoryTree(ctx context.Context, gitea giteaWorkspaceClient, repoName, path, branch string, depth int) []service.ToolWorkspaceResource {
	const maxDepth = 3
	const maxItemsPerDir = 60
	items, err := gitea.RepositoryContents(ctx, repoName, path, branch)
	if err != nil || len(items) == 0 {
		return nil
	}
	if len(items) > maxItemsPerDir {
		items = items[:maxItemsPerDir]
	}
	children := make([]service.ToolWorkspaceResource, len(items))
	var wg sync.WaitGroup
	for i, item := range items {
		i, item := i, item
		wg.Add(1)
		go func() {
			defer wg.Done()
			resourceType := "File"
			if strings.EqualFold(item.Type, "dir") || strings.EqualFold(item.Type, "directory") {
				resourceType = "Directory"
			}
			name := item.Name
			if name == "" {
				name = item.Path
			}
			annotations := map[string]interface{}{"path": item.Path}
			if item.Size > 0 {
				annotations["size"] = item.Size
			}
			if item.DownloadURL != "" {
				annotations["downloadURL"] = item.DownloadURL
			}
			var nested []service.ToolWorkspaceResource
			if resourceType == "Directory" && depth < maxDepth {
				nested = giteaRepositoryTree(ctx, gitea, repoName, item.Path, branch, depth+1)
			}
			children[i] = service.ToolWorkspaceResource{
				Name:        name,
				Type:        resourceType,
				Status:      "Ready",
				Description: giteaContentDescription(resourceType, item),
				Annotations: annotations,
				Children:    nested,
			}
		}()
	}
	wg.Wait()
	return children
}

func giteaContentDescription(resourceType string, item k8s.GiteaContent) string {
	path := valueOrFallback(item.Path, item.Name)
	if resourceType == "Directory" {
		return "目录 " + path
	}
	if item.Size > 0 {
		return fmt.Sprintf("文件 %s · %d bytes", path, item.Size)
	}
	return "文件 " + path
}

func enrichGrafanaWorkspace(ctx context.Context, workspace service.ToolWorkspace, app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) service.ToolWorkspace {
	grafana := k8s.NewGrafanaClient(inst.Namespace)
	if lokiInst, ok := environmentServiceByType(env.ID, "log"); ok {
		if err := grafana.EnsureLokiDatasource(ctx, toolHTTPBaseURL(lokiInst)); err != nil {
			log.Printf("[enrichGrafanaWorkspace] failed to ensure Loki datasource: %v", err)
		}
	}
	dashboards, err := grafana.Dashboards(ctx)
	prometheus := k8s.NewPrometheusClient(inst.Namespace)
	targets, targetErr := prometheus.Targets(ctx)
	alerts, alertErr := prometheus.Alerts(ctx)
	rules, ruleErr := prometheus.Rules(ctx)

	resources := append([]service.ToolWorkspaceResource{}, workspace.Resources...)
	resources = mergeMonitorSubjects(resources, installedServiceMonitorSubjects(app, env, inst))
	resources = mergeMonitorSubjects(resources, kubernetesPodMonitorSubjects(ctx, app, env))

	if err == nil {
		if ensureDefaultGrafanaDashboards(grafana, dashboards, app, env, components) {
			if refreshed, refreshErr := grafana.Dashboards(ctx); refreshErr == nil {
				dashboards = refreshed
			}
		}
		for _, dashboard := range dashboards {
			name := dashboard.Title
			if name == "" {
				name = dashboard.UID
			}
			description := "Grafana dashboard"
			if len(dashboard.Tags) > 0 {
				description = "Tags: " + strings.Join(dashboard.Tags, ", ")
			}
			dashboardResource := service.ToolWorkspaceResource{
				Name:        name,
				Type:        "Dashboard",
				Status:      "Ready",
				Description: description,
				ExternalURL: serviceProxyURL(inst, dashboard.URL),
				Annotations: map[string]interface{}{
					"dashboardUid": dashboard.UID,
				},
			}
			if kind := dashboardKindFromUID(dashboard.UID); kind != "" {
				dashboardResource.Annotations["subjectKind"] = kind
			}
			resources = append(resources, dashboardResource)
		}
	} else {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        "grafana-dashboards",
			Type:        "Dashboard",
			Status:      "Partial",
			Description: err.Error(),
		})
	}
	if targetErr == nil {
		resources = mergeMonitorSubjects(resources, prometheusMonitorSubjects(inst.Namespace, targets))
		for _, target := range targets {
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        prometheusTargetName(target),
				Type:        "Prometheus Target",
				Status:      prometheusHealthStatus(target.Health),
				Description: prometheusTargetDescription(target),
				Annotations: prometheusTargetAnnotations(target),
			})
		}
	} else {
		resources = append(resources, service.ToolWorkspaceResource{Name: "prometheus-targets", Type: "Prometheus Target", Status: "Partial", Description: targetErr.Error()})
	}
	if alertErr == nil {
		if len(alerts) == 0 {
			resources = append(resources, service.ToolWorkspaceResource{Name: "prometheus-alerts", Type: "Alert", Status: "Ready", Description: "No active alerts"})
		}
		for _, alert := range alerts {
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        alert.Name(),
				Type:        "Alert",
				Status:      valueOrFallback(alert.State, "unknown"),
				Description: prometheusLabelsDescription(alert.Labels),
			})
		}
	} else {
		resources = append(resources, service.ToolWorkspaceResource{Name: "prometheus-alerts", Type: "Alert", Status: "Partial", Description: alertErr.Error()})
	}
	if ruleErr == nil {
		for _, rule := range rules {
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        rule.Name,
				Type:        "Rule",
				Status:      valueOrFallback(rule.State, "loaded"),
				Description: strings.Trim(strings.TrimSpace(rule.Group)+" "+strings.TrimSpace(rule.Type), " "),
			})
		}
	} else {
		resources = append(resources, service.ToolWorkspaceResource{Name: "prometheus-rules", Type: "Rule", Status: "Partial", Description: ruleErr.Error()})
	}
	resources = decorateMonitorSubjects(resources, app, env, inst)
	resources = applyGrafanaDashboardURLsToSubjects(resources, dashboards, inst)
	workspace.Resources = resources
	return workspace
}

func applyGrafanaDashboardURLsToSubjects(resources []service.ToolWorkspaceResource, dashboards []k8s.GrafanaDashboard, inst model.ServiceInstallation) []service.ToolWorkspaceResource {
	byUID := map[string]string{}
	for _, dashboard := range dashboards {
		if strings.TrimSpace(dashboard.UID) == "" || strings.TrimSpace(dashboard.URL) == "" {
			continue
		}
		byUID[dashboard.UID] = dashboard.URL
	}
	applied := make([]service.ToolWorkspaceResource, 0, len(resources))
	for _, resource := range resources {
		if resource.Type != "Monitor Subject" {
			applied = append(applied, resource)
			continue
		}
		uid := annotationString(resource.Annotations, "dashboardUid")
		if dashboardURL := byUID[uid]; dashboardURL != "" {
			if resource.Annotations == nil {
				resource.Annotations = map[string]interface{}{}
			}
			resource.Annotations["dashboardPath"] = dashboardURL
			resource.ExternalURL = serviceProxyURL(inst, grafanaDashboardSubjectPath(
				dashboardURL,
				annotationString(resource.Annotations, "namespace"),
				annotationString(resource.Annotations, "selector"),
			))
		}
		applied = append(applied, resource)
	}
	return applied
}

func ensureDefaultGrafanaDashboards(grafana *k8s.GrafanaClient, dashboards []k8s.GrafanaDashboard, app model.Application, env model.Environment, components []model.Component) bool {
	existing := map[string]bool{}
	for _, dashboard := range dashboards {
		if strings.TrimSpace(dashboard.UID) != "" {
			existing[dashboard.UID] = true
		}
	}
	imported := false
	for _, dashboard := range buildDefaultGrafanaDashboards(app, env, components) {
		uid := grafanaDashboardUID(dashboard.JSON)
		if uid == "" {
			continue
		}
		if err := grafana.ImportDashboard(dashboard.JSON, dashboard.Title); err != nil {
			log.Printf("[ensureDefaultGrafanaDashboards] failed to import dashboard %s: %v", dashboard.Title, err)
			continue
		}
		existing[uid] = true
		imported = true
	}
	return imported
}

func grafanaDashboardUID(dashboardJSON string) string {
	var dashboard struct {
		UID string `json:"uid"`
	}
	if err := json.Unmarshal([]byte(dashboardJSON), &dashboard); err != nil {
		return ""
	}
	return strings.TrimSpace(dashboard.UID)
}

func mergeMonitorSubjects(resources []service.ToolWorkspaceResource, subjects []service.ToolWorkspaceResource) []service.ToolWorkspaceResource {
	if len(subjects) == 0 {
		return resources
	}
	merged := make([]service.ToolWorkspaceResource, 0, len(resources)+len(subjects))
	seen := map[string]bool{}
	for _, resource := range resources {
		key := monitorSubjectKey(resource)
		if resource.Type == "Monitor Subject" {
			seen[key] = true
		}
		merged = append(merged, resource)
	}
	for _, subject := range subjects {
		key := monitorSubjectKey(subject)
		if seen[key] {
			continue
		}
		merged = append(merged, subject)
	}
	return merged
}

func monitorSubjectKey(resource service.ToolWorkspaceResource) string {
	if resource.Type != "Monitor Subject" {
		return resource.Type + "/" + resource.Name
	}
	return resource.Type + "/" + annotationString(resource.Annotations, "subjectKind") + "/" + annotationString(resource.Annotations, "namespace") + "/" + valueOrFallback(annotationString(resource.Annotations, "selector"), resource.Name)
}

func kubernetesPodMonitorSubjects(ctx context.Context, app model.Application, env model.Environment) []service.ToolWorkspaceResource {
	return kubernetesPodMonitorSubjectsForNamespaces(ctx, fmt.Sprintf("%s-%s-monitor", app.Identifier, env.Identifier), logSubjectNamespaces(app, env))
}

func kubernetesPodMonitorSubjectsForNamespaces(ctx context.Context, monitorNamespace string, namespaces []string) []service.ToolWorkspaceResource {
	subjects := make([]service.ToolWorkspaceResource, 0)
	for _, namespace := range namespaces {
		runtimeResources, err := k8s.ListNamespaceRuntimeResources(ctx, namespace)
		if err != nil {
			continue
		}
		for _, resource := range runtimeResources {
			if resource.Type != "Pod" {
				continue
			}
			kind := monitorSubjectKindForNamespace(monitorNamespace, namespace)
			subjects = append(subjects, service.ToolWorkspaceResource{
				Name:        resource.Name,
				Type:        "Monitor Subject",
				Status:      observabilityStatus(resource.Status),
				Description: fmt.Sprintf("Kubernetes Pod %s · %s", resource.Status, resource.Description),
				Annotations: map[string]interface{}{
					"subjectKind":  kind,
					"resourceKind": "pod",
					"namespace":    namespace,
					"selector":     resource.Name,
					"logQuery":     logQueryForSubject("pod", namespace, resource.Name),
				},
			})
		}
	}
	sort.SliceStable(subjects, func(i, j int) bool {
		left := annotationString(subjects[i].Annotations, "namespace") + "/" + subjects[i].Name
		right := annotationString(subjects[j].Annotations, "namespace") + "/" + subjects[j].Name
		return left < right
	})
	return subjects
}

func installedServiceMonitorSubjects(app model.Application, env model.Environment, monitorInst model.ServiceInstallation) []service.ToolWorkspaceResource {
	var installations []model.ServiceInstallation
	if err := database.DB.Where("environment_id = ?", env.ID).Find(&installations).Error; err != nil {
		return nil
	}
	subjects := make([]service.ToolWorkspaceResource, 0, len(installations))
	for _, inst := range installations {
		if strings.TrimSpace(inst.Namespace) == "" {
			continue
		}
		kind := "tool"
		if isObservabilityMiddlewareService(inst.ServiceType) {
			kind = "middleware"
		}
		status := observabilityStatus(inst.Status)
		subjects = append(subjects, service.ToolWorkspaceResource{
			Name:        valueOrFallback(inst.ServiceName, valueOrFallback(inst.ReleaseName, inst.ServiceType)),
			Type:        "Monitor Subject",
			Status:      status,
			Description: fmt.Sprintf("%s %s 的专用 Grafana 监控面板。", monitorSubjectKindLabel(kind), inst.ServiceType),
			Annotations: map[string]interface{}{
				"subjectKind": kind,
				"serviceId":   inst.ID,
				"serviceType": inst.ServiceType,
				"namespace":   inst.Namespace,
				"selector":    serviceSelectorForDashboard(inst),
			},
		})
	}
	return decorateMonitorSubjects(subjects, app, env, monitorInst)
}

func decorateMonitorSubjects(resources []service.ToolWorkspaceResource, app model.Application, env model.Environment, inst model.ServiceInstallation) []service.ToolWorkspaceResource {
	decorated := make([]service.ToolWorkspaceResource, 0, len(resources))
	for _, resource := range resources {
		if resource.Type != "Monitor Subject" {
			decorated = append(decorated, resource)
			continue
		}
		if resource.Annotations == nil {
			resource.Annotations = map[string]interface{}{}
		}
		namespace := annotationString(resource.Annotations, "namespace")
		if namespace == "" {
			namespace = fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
			resource.Annotations["namespace"] = namespace
		}
		selector := annotationString(resource.Annotations, "selector")
		if selector == "" {
			selector = resource.Name
			resource.Annotations["selector"] = selector
		}
		kind := annotationString(resource.Annotations, "subjectKind")
		serviceType := annotationString(resource.Annotations, "serviceType")
		uid := annotationString(resource.Annotations, "dashboardUid")
		if uid == "" {
			uid = dashboardUIDForSubject(kind, serviceType)
		}
		path := annotationString(resource.Annotations, "dashboardPath")
		if path == "" {
			path = "/d/" + uid
		}
		resource.Annotations["dashboardUid"] = uid
		resource.Annotations["dashboardPath"] = path
		resource.Annotations["logQuery"] = logQueryForSubject(kind, namespace, selector)
		resource.ExternalURL = serviceProxyURL(inst, grafanaDashboardSubjectPath(path, namespace, selector))
		decorated = append(decorated, resource)
	}
	return decorated
}

func grafanaDashboardSubjectPath(path, namespace, selector string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	parsed, err := url.Parse(path)
	if err != nil {
		return path
	}
	values := parsed.Query()
	if namespace = strings.TrimSpace(namespace); namespace != "" {
		values.Set("var-namespace", namespace)
	}
	if selector = strings.TrimSpace(selector); selector != "" {
		values.Set("var-workload", selector)
	}
	values.Set("orgId", "1")
	values.Set("theme", "light")
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func dashboardUIDForSubject(kind, serviceType string) string {
	serviceType = strings.ToLower(strings.TrimSpace(serviceType))
	if uid := specificDashboardUID(serviceType); uid != "" {
		return uid
	}
	switch kind {
	case "component":
		return "paap-pod-workload"
	case "middleware":
		return "paap-middleware-workload"
	case "tool":
		return "paap-tool-workload"
	default:
		return "paap-environment-overview"
	}
}

func specificDashboardUID(serviceType string) string {
	switch serviceType {
	case "git":
		return "paap-gitea"
	case "deploy":
		return "paap-argocd"
	case "ci":
		return "paap-jenkins"
	case "registry", "harbor":
		return "paap-registry"
	case "log":
		return "paap-loki"
	case "monitor":
		return "paap-grafana"
	case "mysql":
		return "paap-mysql"
	case "postgresql":
		return "paap-postgresql"
	case "mongodb":
		return "paap-mongodb"
	case "redis":
		return "paap-redis"
	case "rabbitmq":
		return "paap-rabbitmq"
	case "kafka":
		return "paap-kafka"
	case "minio":
		return "paap-minio"
	default:
		return ""
	}
}

func dashboardKindFromUID(uid string) string {
	switch uid {
	case "paap-pod-workload":
		return "component"
	case "paap-middleware-workload", "paap-mysql", "paap-postgresql", "paap-mongodb", "paap-redis", "paap-rabbitmq", "paap-kafka", "paap-minio":
		return "middleware"
	case "paap-tool-workload", "paap-gitea", "paap-argocd", "paap-jenkins", "paap-registry", "paap-loki", "paap-grafana":
		return "tool"
	default:
		return "environment"
	}
}

func isObservabilityMiddlewareService(serviceType string) bool {
	switch serviceType {
	case "mysql", "postgresql", "mongodb", "redis", "rabbitmq", "kafka", "minio":
		return true
	default:
		return false
	}
}

func serviceSelectorForDashboard(inst model.ServiceInstallation) string {
	switch inst.ServiceType {
	case "deploy":
		return valueOrFallback(inst.ReleaseName, inst.Namespace)
	case "monitor":
		return "(grafana|prometheus|alertmanager)"
	case "git", "ci", "registry":
		return valueOrFallback(inst.ReleaseName, inst.Namespace)
	default:
		return valueOrFallback(inst.ReleaseName, inst.ServiceType)
	}
}

func logQueryForSubject(kind, namespace, selector string) string {
	namespace = strings.TrimSpace(namespace)
	selector = strings.TrimSpace(selector)
	if namespace == "" {
		namespace = ".+"
	}
	switch kind {
	case "pod":
		return fmt.Sprintf(`{namespace="%s", pod="%s"}`, namespace, selector)
	case "component":
		return fmt.Sprintf(`{namespace="%s", pod=~"%s.*"}`, namespace, selector)
	default:
		if selector == "" {
			return fmt.Sprintf(`{namespace="%s"}`, namespace)
		}
		return fmt.Sprintf(`{namespace="%s", pod=~".*%s.*"}`, namespace, selector)
	}
}

func annotationString(annotations map[string]interface{}, key string) string {
	if annotations == nil {
		return ""
	}
	value, ok := annotations[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func observabilityStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "ready", "healthy", "synced":
		return "Ready"
	case "failed", "error":
		return "Error"
	case "":
		return "Pending"
	default:
		return status
	}
}

func prometheusMonitorSubjects(monitorNamespace string, targets []k8s.PrometheusTarget) []service.ToolWorkspaceResource {
	type aggregate struct {
		name      string
		kind      string
		namespace string
		ready     int
		total     int
		children  []service.ToolWorkspaceResource
	}
	aggregates := map[string]*aggregate{}
	for _, target := range targets {
		name, kind, namespace := prometheusSubjectIdentity(monitorNamespace, target)
		if name == "" {
			continue
		}
		key := kind + "/" + namespace + "/" + name
		current := aggregates[key]
		if current == nil {
			current = &aggregate{name: name, kind: kind, namespace: namespace}
			aggregates[key] = current
		}
		current.total++
		if strings.EqualFold(target.Health, "up") {
			current.ready++
		}
		current.children = append(current.children, service.ToolWorkspaceResource{
			Name:        prometheusTargetName(target),
			Type:        "Prometheus Target",
			Status:      prometheusHealthStatus(target.Health),
			Description: prometheusTargetDescription(target),
			Annotations: prometheusTargetAnnotations(target),
		})
	}
	subjects := make([]service.ToolWorkspaceResource, 0, len(aggregates))
	for _, item := range aggregates {
		status := "Ready"
		if item.ready == 0 && item.total > 0 {
			status = "Down"
		} else if item.ready < item.total {
			status = "Partial"
		}
		subjects = append(subjects, service.ToolWorkspaceResource{
			Name:        item.name,
			Type:        "Monitor Subject",
			Status:      status,
			Description: fmt.Sprintf("%s · %d/%d targets up", monitorSubjectKindLabel(item.kind), item.ready, item.total),
			Annotations: map[string]interface{}{
				"subjectKind":  item.kind,
				"namespace":    item.namespace,
				"targetCount":  item.total,
				"readyTargets": item.ready,
			},
			Children: item.children,
		})
	}
	return subjects
}

func prometheusSubjectIdentity(monitorNamespace string, target k8s.PrometheusTarget) (string, string, string) {
	labels := target.Labels
	if labels == nil {
		return valueOrFallback(target.ScrapePool, "prometheus"), "tool", monitorNamespace
	}
	namespace := valueOrFallback(labels["namespace"], monitorNamespace)
	if pod := firstNonEmpty(labels["pod"], labels["pod_name"]); pod != "" {
		return pod, monitorSubjectKindForNamespace(monitorNamespace, namespace), namespace
	}
	if serviceName := firstNonEmpty(labels["service"], labels["service_name"]); serviceName != "" {
		return serviceName, monitorSubjectKindForNamespace(monitorNamespace, namespace), namespace
	}
	if appName := firstNonEmpty(labels["app"], labels["app_kubernetes_io_name"], labels["job"]); appName != "" {
		return appName, monitorSubjectKindForNamespace(monitorNamespace, namespace), namespace
	}
	return valueOrFallback(target.ScrapePool, namespace), monitorSubjectKindForNamespace(monitorNamespace, namespace), namespace
}

func monitorSubjectKindForNamespace(monitorNamespace, namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" || namespace == monitorNamespace {
		return "tool"
	}
	base := environmentBaseNamespace(monitorNamespace)
	if namespace == base {
		return "component"
	}
	for _, suffix := range []string{"git", "gitea", "deploy", "argocd", "ci", "jenkins", "registry", "docker-registry", "harbor", "log", "loki", "monitor", "kube-prometheus-stack", "prometheus-grafana"} {
		if namespace == base+"-"+suffix {
			return "tool"
		}
	}
	return "middleware"
}

func environmentBaseNamespace(namespace string) string {
	base := strings.TrimSpace(namespace)
	for _, suffix := range []string{
		"-kube-prometheus-stack",
		"-prometheus-grafana",
		"-monitor",
		"-loki",
		"-log",
	} {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	return base
}

func monitorSubjectKindLabel(kind string) string {
	switch kind {
	case "component":
		return "组件"
	case "middleware":
		return "数据库/中间件"
	case "tool":
		return "平台工具"
	default:
		return "监控对象"
	}
}

func prometheusTargetAnnotations(target k8s.PrometheusTarget) map[string]interface{} {
	annotations := map[string]interface{}{
		"scrapePool": target.ScrapePool,
		"health":     target.Health,
	}
	for key, value := range target.Labels {
		if strings.TrimSpace(value) != "" {
			annotations[key] = value
		}
	}
	if target.LastError != "" {
		annotations["lastError"] = target.LastError
	}
	return annotations
}

func prometheusTargetName(target k8s.PrometheusTarget) string {
	if target.Labels != nil {
		for _, key := range []string{"job", "app", "service", "pod", "namespace"} {
			if value := target.Labels[key]; value != "" {
				return value
			}
		}
	}
	return valueOrFallback(target.ScrapePool, "target")
}

func prometheusHealthStatus(health string) string {
	if strings.EqualFold(health, "up") {
		return "Ready"
	}
	if strings.EqualFold(health, "down") {
		return "Down"
	}
	return valueOrFallback(health, "Unknown")
}

func prometheusTargetDescription(target k8s.PrometheusTarget) string {
	description := prometheusLabelsDescription(target.Labels)
	if target.LastError != "" {
		if description != "" {
			description += "; "
		}
		description += target.LastError
	}
	return valueOrFallback(description, valueOrFallback(target.ScrapePool, "Prometheus target"))
}

func prometheusLabelsDescription(labels map[string]string) string {
	parts := make([]string, 0, len(labels))
	for key, value := range labels {
		if key != "" && value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) > 8 {
		parts = append(parts[:8], fmt.Sprintf("+%d more", len(parts)-8))
	}
	return strings.Join(parts, ", ")
}

func enrichHarborWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	harbor := k8s.NewHarborClient(inst.Namespace)
	projects, err := harbor.Projects(ctx)
	if err != nil || len(projects) == 0 {
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0)
	for _, project := range projects {
		repos, repoErr := harbor.Repositories(ctx, project.Name)
		if repoErr != nil {
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        project.Name,
				Type:        "Project",
				Status:      "Partial",
				Description: repoErr.Error(),
			})
			continue
		}
		if len(repos) == 0 {
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        project.Name,
				Type:        "Project",
				Status:      "Empty",
				Description: "No repositories",
			})
			continue
		}
		for _, repo := range repos {
			description := fmt.Sprintf("%d artifacts", repo.ArtifactCount)
			ann := map[string]interface{}{
				"project":       project.Name,
				"artifactCount": repo.ArtifactCount,
			}
			if artifacts, artifactErr := harbor.Artifacts(ctx, project.Name, repo.Name); artifactErr == nil && len(artifacts) > 0 {
				description = harborArtifactsDescription(artifacts)
				tags := make([]string, 0)
				for _, a := range artifacts {
					for _, t := range a.Tags {
						if t.Name != "" {
							tags = append(tags, t.Name)
						}
					}
				}
				if len(tags) > 0 {
					ann["tags"] = tags
				}
				if artifacts[0].Digest != "" {
					ann["digest"] = artifacts[0].Digest
				}
			}
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        repo.Name,
				Type:        "Harbor Repository",
				Status:      "Ready",
				Description: description,
				ExternalURL: serviceProxyURL(inst, "/harbor/projects/"+url.PathEscape(project.Name)+"/repositories/"+url.PathEscape(repo.Name)),
				Annotations: ann,
			})
		}
	}
	workspace.Resources = resources
	return workspace
}

func harborArtifactsDescription(artifacts []k8s.HarborArtifact) string {
	tags := make([]string, 0)
	for _, artifact := range artifacts {
		for _, tag := range artifact.Tags {
			if tag.Name != "" {
				tags = append(tags, tag.Name)
			}
		}
	}
	if len(tags) == 0 {
		return fmt.Sprintf("%d artifacts, no tags", len(artifacts))
	}
	if len(tags) > 8 {
		tags = append(tags[:8], fmt.Sprintf("+%d more", len(tags)-8))
	}
	return fmt.Sprintf("%d artifacts. Tags: %s", len(artifacts), strings.Join(tags, ", "))
}

func enrichRegistryWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	registry := k8s.NewRegistryClient(inst.Namespace)
	repos, err := registry.Catalog(ctx)
	if err != nil || len(repos) == 0 {
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(repos)+1)
	for _, repo := range repos {
		tags, tagErr := registry.Tags(ctx, repo)
		status := "Ready"
		description := "Registry repository"
		annotations := map[string]interface{}{
			"serviceType": "registry",
			"tags":        tags,
		}
		if tagErr == nil && len(tags) > 0 {
			description = "Tags: " + strings.Join(tags, ", ")
		}
		if tagErr != nil {
			status = "Partial"
			description = tagErr.Error()
			annotations["tags"] = []string{}
		}
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        repo,
			Type:        "Image Repository",
			Status:      status,
			Description: description,
			ExternalURL: serviceProxyURL(inst, "/v2/"+strings.Trim(repo, "/")+"/tags/list"),
			Actions:     registryResourceTagActions(inst.ServiceType, repo),
			Annotations: annotations,
		})
	}
	for _, resource := range workspace.Resources {
		if resource.Type == "Runtime Trust" {
			resources = append(resources, resource)
			break
		}
	}
	workspace.Resources = resources
	return workspace
}

func registryResourceTagActions(serviceType, target string) []service.ToolWorkspaceAction {
	if serviceType != "registry" {
		return nil
	}
	return []service.ToolWorkspaceAction{{
		Key:         "delete_registry_tag",
		Label:       "删除 Tag",
		Description: "按当前 repository 和指定 tag 查询 manifest digest 并删除；需要 Registry 已启用 deleteEnabled。",
		Tone:        "danger",
		Target:      target,
		Fields:      []service.ToolWorkspaceActionField{{Name: "tag", Label: "Tag", Required: true, Placeholder: "v1.0.0"}},
	}}
}

func enrichJenkinsWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	jenkins := newJenkinsWorkspaceClient(inst.Namespace)
	jobs, err := jenkins.Jobs(ctx)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{
			Name:        "jenkins-connection",
			Type:        "Connection",
			Status:      "Partial",
			Description: err.Error(),
		}}
		return workspace
	}
	if len(jobs) == 0 {
		workspace.Resources = []service.ToolWorkspaceResource{{
			Name:        "jenkins-jobs",
			Type:        "流水线目录",
			Status:      "Empty",
			Description: "Jenkins 当前没有返回任何流水线任务。",
			ExternalURL: serviceProxyURL(inst, "/"),
		}}
		return workspace
	}
	componentContext := map[string]service.ToolWorkspaceResource{}
	for _, resource := range workspace.Resources {
		if resource.Type == "Job" {
			componentContext[resource.Name] = resource
		}
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(jobs))
	for _, job := range jobs {
		annotations := map[string]interface{}{"color": job.Color, "url": job.URL}
		if existing, ok := componentContext[job.Name]; ok {
			for key, value := range existing.Annotations {
				annotations[key] = value
			}
		}
		annotations["color"] = job.Color
		annotations["url"] = job.URL
		if job.LastBuild != nil {
			annotations["lastBuildNumber"] = job.LastBuild.Number
			annotations["lastBuildURL"] = job.LastBuild.URL
			annotations["lastBuildResult"] = job.LastBuild.Result
			if console, consoleErr := jenkins.ConsoleText(ctx, job.Name); consoleErr == nil && strings.TrimSpace(console) != "" {
				annotations["consoleLog"] = console
			}
		}
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        job.Name,
			Type:        "Job",
			Status:      job.Status,
			Description: "Jenkins job: " + jenkinsStatusLabel(job.Status, job.Color),
			ExternalURL: serviceProxyURL(inst, "/job/"+url.PathEscape(job.Name)+"/"),
			Annotations: annotations,
			Actions: []service.ToolWorkspaceAction{
				{Key: "trigger_jenkins_build", Label: "触发", Description: "触发该 Jenkins Job 构建。"},
			},
		})
	}
	workspace.Resources = resources
	return workspace
}

func jenkinsStatusLabel(status, color string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" || normalized == "unknown" {
		c := strings.ToLower(strings.TrimSpace(color))
		switch {
		case strings.Contains(c, "anime"):
			normalized = "running"
		case strings.HasPrefix(c, "blue"):
			normalized = "success"
		case strings.HasPrefix(c, "red"):
			normalized = "failed"
		case strings.HasPrefix(c, "yellow"):
			normalized = "unstable"
		case c == "disabled":
			normalized = "disabled"
		default:
			normalized = "unknown"
		}
	}
	switch normalized {
	case "success", "ready":
		return "成功"
	case "failed", "failure", "error":
		return "失败"
	case "running", "building":
		return "构建中"
	case "unstable":
		return "不稳定"
	case "disabled":
		return "已禁用"
	default:
		return "未知"
	}
}

func jenkinsBuildTarget(jobs []k8s.JenkinsJob, target string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == "" {
		if len(jobs) == 0 {
			return "", false
		}
		return jobs[0].Name, true
	}
	for _, job := range jobs {
		if job.Name == target {
			return job.Name, true
		}
	}
	return "", false
}

func enrichLokiWorkspace(ctx context.Context, workspace service.ToolWorkspace, app model.Application, env model.Environment, inst model.ServiceInstallation) service.ToolWorkspace {
	loki := k8s.NewLokiClient(inst.Namespace)
	labels, labelErr := loki.Labels(ctx)
	match := fmt.Sprintf(`{namespace=~"%s-%s.*"}`, app.Identifier, env.Identifier)
	series, seriesErr := loki.Series(ctx, match)
	logs, logsErr := loki.QueryRange(ctx, match, 100)

	resources := monitorSubjectsAsLogSubjects(workspace.Resources)
	var grafanaInst model.ServiceInstallation
	hasGrafana := false
	if foundGrafana, ok := environmentServiceByType(env.ID, "monitor"); ok {
		grafanaInst = foundGrafana
		hasGrafana = true
		if err := k8s.NewGrafanaClient(grafanaInst.Namespace).EnsureLokiDatasource(ctx, toolHTTPBaseURL(inst)); err != nil {
			log.Printf("[enrichLokiWorkspace] failed to ensure Loki datasource: %v", err)
		}
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        fmt.Sprintf("%s-%s-loki-explore", app.Identifier, env.Identifier),
			Type:        "Grafana Loki Panel",
			Status:      "Ready",
			Description: "通过 Grafana Explore 查看当前环境 Loki 日志。",
			ExternalURL: serviceProxyURL(grafanaInst, grafanaExploreLokiPath(match)),
			Annotations: map[string]interface{}{
				"query": match,
				"mode":  "Explore",
			},
		})
	}
	if labelErr != nil && seriesErr != nil && logsErr != nil {
		if hasGrafana {
			resources = decorateLogSubjects(resources, grafanaInst)
		}
		workspace.Resources = resources
		return workspace
	}
	if labelErr == nil {
		description := "No labels"
		if len(labels) > 0 {
			if len(labels) > 12 {
				labels = append(labels[:12], fmt.Sprintf("+%d more", len(labels)-12))
			}
			description = "Labels: " + strings.Join(labels, ", ")
		}
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        fmt.Sprintf("%s-%s-labels", app.Identifier, env.Identifier),
			Type:        "Loki Labels",
			Status:      "Ready",
			Description: description,
		})
	}
	if seriesErr != nil {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        match,
			Type:        "Log Streams",
			Status:      "Partial",
			Description: seriesErr.Error(),
		})
	} else if len(series) == 0 {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        match,
			Type:        "Log Streams",
			Status:      "Empty",
			Description: "No streams matched the current environment.",
		})
	} else {
		for _, stream := range series {
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        lokiSeriesName(stream),
				Type:        "Log Stream",
				Status:      "Ready",
				Description: lokiSeriesDescription(stream),
				Annotations: lokiStreamAnnotations(stream),
			})
		}
	}
	if logsErr == nil && len(logs) == 0 {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        match,
			Type:        "Log Entry",
			Status:      "Empty",
			Description: "当前 Loki 查询没有返回日志行。",
			Annotations: map[string]interface{}{"query": match},
		})
	}
	if logsErr == nil {
		for _, entry := range logs {
			ts := entry.Timestamp
			if ts != "" {
				if t, err := strconv.ParseInt(ts, 10, 64); err == nil {
					ts = time.Unix(0, t).Format("15:04:05")
				}
			}
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        lokiSeriesName(entry.Stream),
				Type:        "Log Entry",
				Status:      "Recent",
				Description: truncateLogLine(entry.Line, 180),
				Annotations: map[string]interface{}{
					"time":    ts,
					"message": entry.Line,
					"stream":  lokiSeriesDescription(entry.Stream),
					"subject": lokiSeriesName(entry.Stream),
				},
			})
			for key, value := range entry.Stream {
				if strings.TrimSpace(value) != "" {
					resources[len(resources)-1].Annotations[key] = value
				}
			}
		}
	}
	primaryNamespace := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	logSubjects := lokiLogSubjects(inst.Namespace, series, logs)
	logSubjects = mergeKubernetesPodLogSubjects(logSubjects, kubernetesPodLogSubjects(ctx, logSubjectNamespaces(app, env)))
	logSubjects = sortLogSubjectsForEnvironment(logSubjects, primaryNamespace)
	resources = mergeLogSubjects(resources, logSubjects)
	if hasGrafana {
		resources = decorateLogSubjects(resources, grafanaInst)
	}
	workspace.Resources = resources
	return workspace
}

func monitorSubjectsAsLogSubjects(resources []service.ToolWorkspaceResource) []service.ToolWorkspaceResource {
	out := make([]service.ToolWorkspaceResource, 0, len(resources))
	for _, resource := range resources {
		if resource.Type != "Monitor Subject" {
			out = append(out, resource)
			continue
		}
		resource.Type = "Log Subject"
		if resource.Description == "" {
			resource.Description = "日志查询对象。"
		}
		out = append(out, resource)
	}
	return out
}

func mergeLogSubjects(resources []service.ToolWorkspaceResource, subjects []service.ToolWorkspaceResource) []service.ToolWorkspaceResource {
	if len(subjects) == 0 {
		return resources
	}
	return append(subjects, resources...)
}

func logSubjectNamespaces(app model.Application, env model.Environment) []string {
	prefix := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	seen := map[string]bool{}
	namespaces := make([]string, 0, 8)
	add := func(namespace string) {
		namespace = strings.TrimSpace(namespace)
		if namespace == "" || seen[namespace] {
			return
		}
		seen[namespace] = true
		namespaces = append(namespaces, namespace)
	}
	add(prefix)
	var services []model.ServiceInstallation
	if err := database.DB.Where("environment_id = ?", env.ID).Find(&services).Error; err == nil {
		for _, inst := range services {
			add(inst.Namespace)
		}
	}
	sort.Strings(namespaces)
	return namespaces
}

func kubernetesPodLogSubjects(ctx context.Context, namespaces []string) []service.ToolWorkspaceResource {
	subjects := make([]service.ToolWorkspaceResource, 0)
	for _, namespace := range namespaces {
		runtimeResources, err := k8s.ListNamespaceRuntimeResources(ctx, namespace)
		if err != nil {
			continue
		}
		for _, resource := range runtimeResources {
			if resource.Type != "Pod" {
				continue
			}
			query := logQueryForSubject("pod", namespace, resource.Name)
			subjects = append(subjects, service.ToolWorkspaceResource{
				Name:        resource.Name,
				Type:        "Log Subject",
				Status:      "Empty",
				Description: fmt.Sprintf("Kubernetes Pod %s · 0 streams · 0 entries · Loki 未返回日志流", resource.Status),
				Annotations: map[string]interface{}{
					"subjectKind": "pod",
					"namespace":   namespace,
					"streamCount": 0,
					"entryCount":  0,
					"selector":    resource.Name,
					"logQuery":    query,
				},
			})
		}
	}
	sort.SliceStable(subjects, func(i, j int) bool {
		left := annotationString(subjects[i].Annotations, "namespace") + "/" + subjects[i].Name
		right := annotationString(subjects[j].Annotations, "namespace") + "/" + subjects[j].Name
		return left < right
	})
	return subjects
}

func mergeKubernetesPodLogSubjects(subjects []service.ToolWorkspaceResource, pods []service.ToolWorkspaceResource) []service.ToolWorkspaceResource {
	if len(pods) == 0 {
		return subjects
	}
	seen := map[string]bool{}
	for _, subject := range subjects {
		seen[logSubjectKey(subject)] = true
	}
	merged := make([]service.ToolWorkspaceResource, 0, len(subjects)+len(pods))
	merged = append(merged, subjects...)
	for _, pod := range pods {
		if seen[logSubjectKey(pod)] {
			continue
		}
		merged = append(merged, pod)
	}
	return merged
}

func sortLogSubjectsForEnvironment(subjects []service.ToolWorkspaceResource, primaryNamespace string) []service.ToolWorkspaceResource {
	sorted := append([]service.ToolWorkspaceResource{}, subjects...)
	sort.SliceStable(sorted, func(i, j int) bool {
		leftScore := logSubjectPriority(sorted[i], primaryNamespace)
		rightScore := logSubjectPriority(sorted[j], primaryNamespace)
		if leftScore != rightScore {
			return leftScore < rightScore
		}
		leftEntries := annotationInt(sorted[i].Annotations, "entryCount")
		rightEntries := annotationInt(sorted[j].Annotations, "entryCount")
		if leftEntries != rightEntries {
			return leftEntries > rightEntries
		}
		leftStreams := annotationInt(sorted[i].Annotations, "streamCount")
		rightStreams := annotationInt(sorted[j].Annotations, "streamCount")
		if leftStreams != rightStreams {
			return leftStreams > rightStreams
		}
		left := annotationString(sorted[i].Annotations, "namespace") + "/" + sorted[i].Name
		right := annotationString(sorted[j].Annotations, "namespace") + "/" + sorted[j].Name
		return left < right
	})
	return sorted
}

func logSubjectPriority(resource service.ToolWorkspaceResource, primaryNamespace string) int {
	kind := annotationString(resource.Annotations, "subjectKind")
	namespace := annotationString(resource.Annotations, "namespace")
	switch {
	case kind == "environment":
		return 0
	case kind == "component":
		return 1
	case namespace == primaryNamespace:
		return 2
	case kind == "pod" && namespace == primaryNamespace:
		return 2
	case kind == "middleware":
		return 3
	case kind == "tool":
		return 4
	default:
		return 5
	}
}

func annotationInt(annotations map[string]interface{}, key string) int {
	if annotations == nil {
		return 0
	}
	switch value := annotations[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		parsed, _ := strconv.Atoi(value)
		return parsed
	default:
		return 0
	}
}

func logSubjectKey(resource service.ToolWorkspaceResource) string {
	return annotationString(resource.Annotations, "subjectKind") + "/" + annotationString(resource.Annotations, "namespace") + "/" + resource.Name
}

func lokiLogSubjects(logNamespace string, series []map[string]string, logs []k8s.LokiLogEntry) []service.ToolWorkspaceResource {
	type aggregate struct {
		name      string
		kind      string
		namespace string
		streams   int
		entries   int
		children  []service.ToolWorkspaceResource
	}
	aggregates := map[string]*aggregate{}
	ensure := func(labels map[string]string) *aggregate {
		name, kind, namespace := lokiSubjectIdentity(logNamespace, labels)
		key := kind + "/" + namespace + "/" + name
		item := aggregates[key]
		if item == nil {
			item = &aggregate{name: name, kind: kind, namespace: namespace}
			aggregates[key] = item
		}
		return item
	}
	for _, stream := range series {
		item := ensure(stream)
		item.streams++
		item.children = append(item.children, service.ToolWorkspaceResource{
			Name:        lokiSeriesName(stream),
			Type:        "Log Stream",
			Status:      "Ready",
			Description: lokiSeriesDescription(stream),
			Annotations: lokiStreamAnnotations(stream),
		})
	}
	for _, entry := range logs {
		item := ensure(entry.Stream)
		item.entries++
	}
	subjects := make([]service.ToolWorkspaceResource, 0, len(aggregates))
	for _, item := range aggregates {
		subjects = append(subjects, service.ToolWorkspaceResource{
			Name:        item.name,
			Type:        "Log Subject",
			Status:      "Ready",
			Description: fmt.Sprintf("%s · %d streams · %d entries", monitorSubjectKindLabel(item.kind), item.streams, item.entries),
			Annotations: map[string]interface{}{
				"subjectKind": item.kind,
				"namespace":   item.namespace,
				"streamCount": item.streams,
				"entryCount":  item.entries,
				"selector":    item.name,
				"logQuery":    logQueryForSubject(item.kind, item.namespace, item.name),
			},
			Children: item.children,
		})
	}
	sort.SliceStable(subjects, func(i, j int) bool {
		leftEntries, _ := subjects[i].Annotations["entryCount"].(int)
		rightEntries, _ := subjects[j].Annotations["entryCount"].(int)
		if leftEntries != rightEntries {
			return leftEntries > rightEntries
		}
		leftStreams, _ := subjects[i].Annotations["streamCount"].(int)
		rightStreams, _ := subjects[j].Annotations["streamCount"].(int)
		if leftStreams != rightStreams {
			return leftStreams > rightStreams
		}
		return subjects[i].Name < subjects[j].Name
	})
	return subjects
}

func decorateLogSubjects(resources []service.ToolWorkspaceResource, grafanaInst model.ServiceInstallation) []service.ToolWorkspaceResource {
	decorated := make([]service.ToolWorkspaceResource, 0, len(resources))
	for _, resource := range resources {
		if resource.Type != "Log Subject" {
			decorated = append(decorated, resource)
			continue
		}
		if resource.Annotations == nil {
			resource.Annotations = map[string]interface{}{}
		}
		query := annotationString(resource.Annotations, "logQuery")
		if query == "" {
			query = logQueryForSubject(
				annotationString(resource.Annotations, "subjectKind"),
				annotationString(resource.Annotations, "namespace"),
				annotationString(resource.Annotations, "selector"),
			)
			resource.Annotations["logQuery"] = query
		}
		resource.ExternalURL = serviceProxyURL(grafanaInst, grafanaExploreLokiPath(query))
		decorated = append(decorated, resource)
	}
	return decorated
}

func lokiSubjectIdentity(logNamespace string, labels map[string]string) (string, string, string) {
	namespace := valueOrFallback(labels["namespace"], logNamespace)
	name := firstNonEmpty(labels["pod"], labels["app"], labels["component"], labels["container"], labels["job"])
	if name == "" {
		name = namespace
	}
	kind := monitorSubjectKindForNamespace(strings.TrimSuffix(logNamespace, "-log")+"-monitor", namespace)
	if strings.TrimSpace(labels["pod"]) != "" {
		kind = "pod"
	}
	return name, kind, namespace
}

func lokiStreamAnnotations(stream map[string]string) map[string]interface{} {
	annotations := map[string]interface{}{}
	for key, value := range stream {
		if strings.TrimSpace(value) != "" {
			annotations[key] = value
		}
	}
	return annotations
}

func truncateLogLine(line string, max int) string {
	line = strings.TrimSpace(line)
	if len(line) <= max {
		return line
	}
	return line[:max] + "..."
}

func lokiSeriesName(stream map[string]string) string {
	for _, key := range []string{"app", "component", "container", "pod"} {
		if value := stream[key]; value != "" {
			return value
		}
	}
	return "log-stream"
}

func lokiSeriesDescription(stream map[string]string) string {
	parts := make([]string, 0, len(stream))
	for key, value := range stream {
		if key != "" && value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if len(parts) == 0 {
		return "Loki stream"
	}
	if len(parts) > 8 {
		parts = append(parts[:8], fmt.Sprintf("+%d more", len(parts)-8))
	}
	return strings.Join(parts, ", ")
}

func reconcileEnvironmentGitOps(app model.Application, env model.Environment, inst model.ServiceInstallation, components []model.Component) (service.ToolWorkspace, []string) {
	ctx := context.Background()
	primaryNS := app.Identifier + "-" + env.Identifier
	var errs []string

	for i := range components {
		comp := components[i]
		identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
		result, err := service.EnsureComponentGitOps(ctx, k8s.GetClient(), app, env, comp, identifier, primaryNS)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", comp.Name, err))
			continue
		}
		comp.GitRepoURL = result.RepositoryURL
		if result.SourceMirrorURL != "" {
			comp.SourceMirrorRepoURL = result.SourceMirrorURL
		}
		comp.GitPath = result.RepositoryPath
		comp.ArgoCDApp = result.ArgoCDApplication
		if result.CIStatus != "" {
			comp.PipelineStatus = result.CIStatus
		}
		if result.CIWarning != "" {
			comp.ErrorMessage = result.CIWarning
		}
		comp.Status = "syncing"
		if result.CIWarning == "" {
			comp.ErrorMessage = ""
		}
		if err := database.DB.Save(&comp).Error; err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", comp.Name, err))
			continue
		}
		components[i] = comp
	}

	return service.BuildToolWorkspace(app, env, inst, components), errs
}

func enrichRedisWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	info, err := k8s.DiscoverRedisConnection(ctx, inst.Namespace)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{
			Name:        "redis-connection",
			Type:        "Connection",
			Status:      "Partial",
			Description: err.Error(),
		}}
		return workspace
	}
	summary, err := service.InspectRedis(ctx, info)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{
			Name:        "redis-connection",
			Type:        "Connection",
			Status:      "Partial",
			Description: err.Error(),
		}}
		return workspace
	}
	workspace.Resources = []service.ToolWorkspaceResource{
		{Name: "redis-ping", Type: "Health", Status: "Ready", Description: "PING: " + summary.Ping},
		{Name: "keys", Type: "Keyspace", Status: "Ready", Description: fmt.Sprintf("%d keys in current database", summary.KeyCount)},
		{Name: "redis-version", Type: "Info", Status: "Ready", Description: valueOrFallback(summary.Version, "-")},
		{Name: "used-memory", Type: "Info", Status: "Ready", Description: valueOrFallback(summary.UsedMemory, "-")},
		{Name: "connected-clients", Type: "Info", Status: "Ready", Description: valueOrFallback(summary.Connected, "-")},
	}
	return workspace
}

func enrichRedisKeysWorkspace(ctx context.Context, workspace service.ToolWorkspace, info k8s.RedisConnectionInfo, pattern string, limit int) service.ToolWorkspace {
	keys, err := service.ListRedisKeys(ctx, info, pattern, limit)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{
			Name:        "redis-keys",
			Type:        "Keyspace",
			Status:      "Partial",
			Description: err.Error(),
		}}
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(keys)+1)
	if pattern == "" {
		pattern = "*"
	}
	resources = append(resources, service.ToolWorkspaceResource{
		Name:        pattern,
		Type:        "Key Pattern",
		Status:      "Ready",
		Description: fmt.Sprintf("%d keys matched", len(keys)),
	})
	for _, key := range keys {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        key,
			Type:        "Redis Key",
			Status:      "Ready",
			Description: "Redis key",
			Actions: []service.ToolWorkspaceAction{
				{Key: "get_redis_key", Label: "读取", Description: "读取该 Key。", Target: key},
				{Key: "expire_redis_key", Label: "TTL", Description: "设置该 Key 的过期时间。", Target: key, Fields: []service.ToolWorkspaceActionField{{Name: "ttlSeconds", Label: "TTL 秒", Type: "number", Required: true, Placeholder: "3600"}}},
				{Key: "delete_redis_key", Label: "删除", Description: "删除该 Key。", Tone: "danger", Target: key},
			},
		})
	}
	workspace.Resources = resources
	return workspace
}

func enrichRedisKeyWorkspace(ctx context.Context, workspace service.ToolWorkspace, info k8s.RedisConnectionInfo, key string) service.ToolWorkspace {
	item, err := service.GetRedisKey(ctx, info, key)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{
			Name:        valueOrFallback(key, "redis-key"),
			Type:        "Redis Key",
			Status:      "Partial",
			Description: err.Error(),
		}}
		return workspace
	}
	workspace.Resources = []service.ToolWorkspaceResource{
		{Name: item.Key, Type: "Redis Key", Status: "Ready", Description: "type=" + item.Type},
		{Name: "ttl", Type: "TTL", Status: "Ready", Description: strconv.FormatInt(item.TTL, 10)},
		{Name: "value", Type: "Value", Status: "Ready", Description: item.Value},
	}
	return workspace
}

func enrichMinIOWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	info, err := k8s.DiscoverMinIOConnection(ctx, inst.Namespace)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: "minio-connection", Type: "Connection", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	buckets, err := service.ListMinIOBuckets(ctx, info)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: "minio-connection", Type: "Connection", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(buckets))
	for _, bucket := range buckets {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        bucket.Name,
			Type:        "Bucket",
			Status:      "Ready",
			Description: "MinIO bucket",
			Actions: []service.ToolWorkspaceAction{
				{Key: "list_minio_objects", Label: "对象", Description: "查看该 bucket 的对象。", Target: bucket.Name},
				{Key: "delete_minio_bucket", Label: "删除", Description: "删除该空 bucket。", Tone: "danger", Target: bucket.Name},
			},
		})
	}
	if len(resources) == 0 {
		resources = append(resources, service.ToolWorkspaceResource{Name: "buckets", Type: "Bucket", Status: "Empty", Description: "No buckets"})
	}
	workspace.Resources = resources
	return workspace
}

func enrichMinIOObjectsWorkspace(ctx context.Context, workspace service.ToolWorkspace, info k8s.MinIOConnectionInfo, bucket, prefix string, limit int) service.ToolWorkspace {
	objects, err := service.ListMinIOObjects(ctx, info, bucket, prefix, limit)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: valueOrFallback(bucket, "objects"), Type: "Object", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(objects)+1)
	resources = append(resources, service.ToolWorkspaceResource{Name: bucket, Type: "Bucket", Status: "Ready", Description: fmt.Sprintf("%d objects", len(objects))})
	for _, object := range objects {
		target := databaseTableTarget(bucket, object.Key)
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        object.Key,
			Type:        "Object",
			Status:      "Ready",
			Description: fmt.Sprintf("%d bytes", object.Size),
			Annotations: map[string]interface{}{"size": object.Size},
			Actions: []service.ToolWorkspaceAction{
				{Key: "delete_minio_object", Label: "删除", Description: "删除该对象。", Tone: "danger", Target: target},
			},
		})
	}
	workspace.Resources = resources
	return workspace
}

func enrichMongoDBWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	info, err := k8s.DiscoverMongoDBConnection(ctx, inst.Namespace)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: "mongodb-connection", Type: "Connection", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	databases, err := service.ListMongoDBDatabases(ctx, info)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: "mongodb-connection", Type: "Connection", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(databases))
	for _, databaseName := range databases {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        databaseName,
			Type:        "Database",
			Status:      "Ready",
			Description: "MongoDB database",
			Actions: []service.ToolWorkspaceAction{
				{Key: "list_mongodb_collections", Label: "集合", Description: "查看该数据库集合。", Target: databaseName},
			},
		})
	}
	if len(resources) == 0 {
		resources = append(resources, service.ToolWorkspaceResource{Name: "databases", Type: "Database", Status: "Empty", Description: "No databases"})
	}
	workspace.Resources = resources
	return workspace
}

func enrichMongoDBCollectionsWorkspace(ctx context.Context, workspace service.ToolWorkspace, info k8s.MongoDBConnectionInfo, databaseName string) service.ToolWorkspace {
	collections, err := service.ListMongoDBCollections(ctx, info, databaseName)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: valueOrFallback(databaseName, "collections"), Type: "Collection", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(collections)+1)
	resources = append(resources, service.ToolWorkspaceResource{Name: databaseName, Type: "Database", Status: "Ready", Description: "Selected database"})
	for _, collection := range collections {
		target := databaseTableTarget(databaseName, collection)
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        collection,
			Type:        "Collection",
			Status:      "Ready",
			Description: "MongoDB collection",
			Actions: []service.ToolWorkspaceAction{
				{Key: "preview_mongodb_documents", Label: "文档", Description: "预览该集合文档。", Target: target},
				{Key: "insert_mongodb_document", Label: "新增", Description: "插入文档。", Target: target, Fields: []service.ToolWorkspaceActionField{{Name: "document", Label: "文档 JSON", Type: "textarea", Required: true, Placeholder: `{"field":"value"}`}}},
				{Key: "update_mongodb_documents", Label: "更新", Description: "按 filter 更新文档。", Target: target, Fields: []service.ToolWorkspaceActionField{
					{Name: "filter", Label: "Filter JSON", Type: "textarea", Required: true, Placeholder: `{"field":"value"}`},
					{Name: "update", Label: "Update JSON", Type: "textarea", Required: true, Placeholder: `{"status":"active"}`},
				}},
				{Key: "delete_mongodb_documents", Label: "删除文档", Description: "按 filter 删除文档。", Tone: "danger", Target: target, Fields: []service.ToolWorkspaceActionField{{Name: "filter", Label: "Filter JSON", Type: "textarea", Required: true, Placeholder: `{"field":"value"}`}}},
				{Key: "drop_mongodb_collection", Label: "删集合", Description: "删除该集合。", Tone: "danger", Target: target},
			},
		})
	}
	workspace.Resources = resources
	return workspace
}

func enrichMongoDBDocumentsWorkspace(ctx context.Context, workspace service.ToolWorkspace, info k8s.MongoDBConnectionInfo, databaseName, collection string, limit int) service.ToolWorkspace {
	documents, err := service.PreviewMongoDBDocuments(ctx, info, databaseName, collection, int64(limit))
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: valueOrFallback(collection, "documents"), Type: "Document", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(documents)+1)
	resources = append(resources, service.ToolWorkspaceResource{Name: collection, Type: "Collection", Status: "Ready", Description: fmt.Sprintf("%d documents", len(documents))})
	for i, document := range documents {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        fmt.Sprintf("document-%d", i+1),
			Type:        "Document",
			Status:      "Ready",
			Description: truncateLogLine(document, 220),
		})
	}
	workspace.Resources = resources
	return workspace
}

func enrichRabbitMQWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	info, err := k8s.DiscoverRabbitMQConnection(ctx, inst.Namespace)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: "rabbitmq-connection", Type: "Connection", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	queues, queueErr := service.ListRabbitMQQueues(ctx, info)
	exchanges, exchangeErr := service.ListRabbitMQExchanges(ctx, info)
	vhosts, vhostErr := service.ListRabbitMQVHosts(ctx, info)
	bindings, bindingErr := service.ListRabbitMQBindings(ctx, info)
	resources := make([]service.ToolWorkspaceResource, 0, len(queues)+len(exchanges)+len(vhosts)+len(bindings))
	if queueErr == nil {
		for _, queue := range queues {
			target := databaseTableTarget(queue.VHost, queue.Name)
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        queue.Name,
				Type:        "Queue",
				Status:      valueOrFallback(queue.State, "Ready"),
				Description: fmt.Sprintf("vhost=%s messages=%d", queue.VHost, queue.Messages),
				Annotations: map[string]interface{}{"vhost": queue.VHost, "messages": queue.Messages},
				Actions: []service.ToolWorkspaceAction{
					{Key: "get_rabbitmq_messages", Label: "查看消息", Description: "读取该队列中的消息。", Target: target, Fields: []service.ToolWorkspaceActionField{
						{Name: "count", Label: "数量", Type: "number", Default: "10", Placeholder: "10"},
						{Name: "requeue", Label: "读后放回", Type: "checkbox", Default: "true"},
					}},
					{Key: "publish_rabbitmq_message", Label: "发布", Description: "通过默认交换机向该队列发布消息。", Target: target, Fields: []service.ToolWorkspaceActionField{
						{Name: "payload", Label: "消息内容", Type: "textarea", Required: true, Placeholder: `{"id":1}`},
						{Name: "properties", Label: "Properties JSON", Type: "textarea", Placeholder: `{"content_type":"application/json"}`},
					}},
					{Key: "purge_rabbitmq_queue", Label: "清空", Description: "清空该队列中的所有消息。", Tone: "danger", Target: target},
					{Key: "delete_rabbitmq_queue", Label: "删除", Description: "删除该队列。", Tone: "danger", Target: target},
				},
			})
		}
	} else {
		resources = append(resources, service.ToolWorkspaceResource{Name: "rabbitmq-queues", Type: "Queue", Status: "Partial", Description: queueErr.Error()})
	}
	if exchangeErr == nil {
		for _, exchange := range exchanges {
			if strings.HasPrefix(exchange.Name, "amq.") {
				continue
			}
			if strings.TrimSpace(exchange.Name) == "" {
				continue
			}
			target := databaseTableTarget(exchange.VHost, exchange.Name)
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        exchange.Name,
				Type:        "Exchange",
				Status:      "Ready",
				Description: "type=" + exchange.Type + " vhost=" + exchange.VHost,
				Annotations: map[string]interface{}{"vhost": exchange.VHost, "exchangeType": exchange.Type},
				Actions: []service.ToolWorkspaceAction{
					{Key: "publish_rabbitmq_message", Label: "发布", Description: "向该交换机发布消息。", Target: target, Fields: []service.ToolWorkspaceActionField{
						{Name: "vhost", Label: "VHost", Default: exchange.VHost, Placeholder: "/"},
						{Name: "exchange", Label: "交换机", Default: exchange.Name, Placeholder: "orders.events"},
						{Name: "routingKey", Label: "Routing key", Placeholder: "orders.created"},
						{Name: "payload", Label: "消息内容", Type: "textarea", Required: true, Placeholder: `{"id":1}`},
						{Name: "properties", Label: "Properties JSON", Type: "textarea", Placeholder: `{"content_type":"application/json"}`},
					}},
					{Key: "create_rabbitmq_binding", Label: "绑定", Description: "从该交换机绑定到队列或交换机。", Target: target, Fields: []service.ToolWorkspaceActionField{
						{Name: "vhost", Label: "VHost", Default: exchange.VHost, Placeholder: "/"},
						{Name: "source", Label: "源交换机", Default: exchange.Name, Required: true, Placeholder: "orders.events"},
						{Name: "destinationType", Label: "目标类型", Default: "queue", Placeholder: "queue 或 exchange"},
						{Name: "destination", Label: "目标", Required: true, Placeholder: "orders.created"},
						{Name: "routingKey", Label: "Routing key", Placeholder: "orders.#"},
						{Name: "arguments", Label: "Arguments JSON", Type: "textarea", Placeholder: `{"x-match":"all"}`},
					}},
					{Key: "delete_rabbitmq_exchange", Label: "删除", Description: "删除该交换机。", Tone: "danger", Target: target},
				},
			})
		}
	} else {
		resources = append(resources, service.ToolWorkspaceResource{Name: "rabbitmq-exchanges", Type: "Exchange", Status: "Partial", Description: exchangeErr.Error()})
	}
	if vhostErr == nil {
		for _, vhost := range vhosts {
			actions := []service.ToolWorkspaceAction{}
			if vhost.Name != "/" {
				actions = append(actions, service.ToolWorkspaceAction{Key: "delete_rabbitmq_vhost", Label: "删除", Description: "删除该 VHost。", Tone: "danger", Target: vhost.Name})
			}
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        vhost.Name,
				Type:        "VHost",
				Status:      "Ready",
				Description: "RabbitMQ virtual host",
				Actions:     actions,
			})
		}
	} else {
		resources = append(resources, service.ToolWorkspaceResource{Name: "rabbitmq-vhosts", Type: "VHost", Status: "Partial", Description: vhostErr.Error()})
	}
	if bindingErr == nil {
		for _, binding := range bindings {
			if binding.Source == "" || binding.PropertiesKey == "" {
				continue
			}
			name := fmt.Sprintf("%s -> %s", binding.Source, binding.Destination)
			target := rabbitMQBindingTarget(binding.VHost, binding.Source, binding.DestinationType, binding.Destination, binding.PropertiesKey)
			resources = append(resources, service.ToolWorkspaceResource{
				Name:        name,
				Type:        "Binding",
				Status:      "Ready",
				Description: fmt.Sprintf("vhost=%s routing_key=%s", binding.VHost, binding.RoutingKey),
				Annotations: map[string]interface{}{
					"vhost":           binding.VHost,
					"source":          binding.Source,
					"destination":     binding.Destination,
					"destinationType": binding.DestinationType,
					"routingKey":      binding.RoutingKey,
					"propertiesKey":   binding.PropertiesKey,
				},
				Actions: []service.ToolWorkspaceAction{
					{Key: "delete_rabbitmq_binding", Label: "删除", Description: "删除该绑定。", Tone: "danger", Target: target},
				},
			})
		}
	} else {
		resources = append(resources, service.ToolWorkspaceResource{Name: "rabbitmq-bindings", Type: "Binding", Status: "Partial", Description: bindingErr.Error()})
	}
	workspace.Resources = resources
	return workspace
}

func enrichRabbitMQMessagesWorkspace(ctx context.Context, workspace service.ToolWorkspace, info k8s.RabbitMQConnectionInfo, vhost, queue string, count int, requeue bool) service.ToolWorkspace {
	messages, err := service.GetRabbitMQMessages(ctx, info, vhost, queue, count, requeue)
	if err != nil {
		workspace.Resources = append(workspace.Resources, service.ToolWorkspaceResource{Name: valueOrFallback(queue, "messages"), Type: "Message", Status: "Partial", Description: err.Error()})
		return workspace
	}
	for idx, message := range messages {
		name := fmt.Sprintf("%s#%d", valueOrFallback(queue, "message"), idx+1)
		payload := message.Payload
		if payload == "" {
			payload = message.PayloadString
		}
		workspace.Resources = append(workspace.Resources, service.ToolWorkspaceResource{
			Name:        name,
			Type:        "Message",
			Status:      "Ready",
			Description: truncateLogLine(payload, 220),
			Annotations: map[string]interface{}{
				"vhost":        valueOrFallback(vhost, "/"),
				"queue":        queue,
				"exchange":     message.Exchange,
				"routingKey":   message.RoutingKey,
				"payloadBytes": message.PayloadBytes,
				"remaining":    message.MessageCount,
				"redelivered":  message.Redelivered,
			},
		})
	}
	return workspace
}

func enrichKafkaWorkspace(ctx context.Context, workspace service.ToolWorkspace, inst model.ServiceInstallation) service.ToolWorkspace {
	info, err := k8s.DiscoverKafkaConnection(ctx, inst.Namespace)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: "kafka-connection", Type: "Connection", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	topics, err := service.ListKafkaTopics(ctx, info)
	if err != nil {
		workspace.Resources = []service.ToolWorkspaceResource{{Name: "kafka-topics", Type: "Topic", Status: "Partial", Description: err.Error()}}
		return workspace
	}
	resources := make([]service.ToolWorkspaceResource, 0, len(topics))
	for _, topic := range topics {
		resources = append(resources, service.ToolWorkspaceResource{
			Name:        topic.Name,
			Type:        "Topic",
			Status:      "Ready",
			Description: fmt.Sprintf("%d partitions", topic.Partitions),
			Annotations: map[string]interface{}{"partitions": topic.Partitions},
			Actions: []service.ToolWorkspaceAction{
				{Key: "read_kafka_messages", Label: "读取消息", Description: "从该 Topic 读取消息。", Target: topic.Name, Fields: []service.ToolWorkspaceActionField{
					{Name: "partition", Label: "Partition", Type: "number", Placeholder: "留空读取全部"},
					{Name: "offset", Label: "Offset", Default: "first", Placeholder: "first / latest / 0"},
					{Name: "limit", Label: "数量", Type: "number", Default: "10"},
				}},
				{Key: "produce_kafka_message", Label: "写入消息", Description: "向该 Topic 写入一条消息。", Target: topic.Name, Fields: []service.ToolWorkspaceActionField{
					{Name: "key", Label: "Key", Placeholder: "order-1"},
					{Name: "value", Label: "消息内容", Type: "textarea", Required: true, Placeholder: `{"id":1}`},
					{Name: "partition", Label: "Partition", Type: "number", Placeholder: "留空自动分配"},
				}},
				{Key: "delete_kafka_topic", Label: "删除", Description: "删除该 topic。", Tone: "danger", Target: topic.Name},
			},
		})
	}
	if len(resources) == 0 {
		resources = append(resources, service.ToolWorkspaceResource{Name: "topics", Type: "Topic", Status: "Empty", Description: "No topics"})
	}
	workspace.Resources = resources
	return workspace
}

func enrichKafkaMessagesWorkspace(ctx context.Context, workspace service.ToolWorkspace, info k8s.KafkaConnectionInfo, topic string, partition int, offset string, limit int) service.ToolWorkspace {
	messages, err := service.ReadKafkaMessages(ctx, info, topic, partition, offset, limit)
	if err != nil {
		workspace.Resources = append(workspace.Resources, service.ToolWorkspaceResource{Name: valueOrFallback(topic, "messages"), Type: "Message", Status: "Partial", Description: err.Error()})
		return workspace
	}
	if len(messages) == 0 {
		workspace.Resources = append(workspace.Resources, service.ToolWorkspaceResource{
			Name:        valueOrFallback(topic, "messages"),
			Type:        "Message",
			Status:      "Empty",
			Description: "No messages returned for the selected topic/offset.",
			Annotations: map[string]interface{}{
				"topic":     topic,
				"partition": partition,
				"offset":    valueOrFallback(offset, "first"),
			},
		})
		return workspace
	}
	for _, message := range messages {
		workspace.Resources = append(workspace.Resources, service.ToolWorkspaceResource{
			Name:        fmt.Sprintf("%s/%d/%d", valueOrFallback(message.Topic, topic), message.Partition, message.Offset),
			Type:        "Message",
			Status:      "Ready",
			Description: truncateLogLine(message.Value, 220),
			Annotations: map[string]interface{}{
				"topic":     message.Topic,
				"partition": message.Partition,
				"offset":    message.Offset,
				"key":       message.Key,
				"time":      message.Time.Format(time.RFC3339),
				"value":     message.Value,
			},
		})
	}
	return workspace
}

func valueOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func serviceProxyURL(inst model.ServiceInstallation, path string) string {
	if inst.EnvironmentID == 0 || inst.ID == 0 {
		return ""
	}
	return fmt.Sprintf("/api/v1/environments/%d/services/%d/proxy/%s", inst.EnvironmentID, inst.ID, strings.TrimLeft(proxyRequestPath(path), "/"))
}

func componentProxyURL(envID uint, comp model.Component, path string) string {
	if envID == 0 || comp.ID == 0 {
		return ""
	}
	return fmt.Sprintf("/api/v1/environments/%d/components/%d/proxy/%s", envID, comp.ID, strings.TrimLeft(proxyRequestPath(path), "/"))
}

func environmentServiceByType(environmentID uint, serviceType string) (model.ServiceInstallation, bool) {
	var inst model.ServiceInstallation
	if err := database.DB.Where("environment_id = ? AND service_type = ?", environmentID, serviceType).First(&inst).Error; err != nil {
		return model.ServiceInstallation{}, false
	}
	return inst, true
}

func grafanaExploreLokiPath(query string) string {
	left := map[string]interface{}{
		"datasource": "Loki",
		"queries": []map[string]interface{}{
			{"refId": "A", "expr": query, "queryType": "range"},
		},
		"range": map[string]string{"from": "now-24h", "to": "now"},
	}
	encoded, _ := json.Marshal(left)
	values := url.Values{}
	values.Set("orgId", "1")
	values.Set("left", string(encoded))
	values.Set("kiosk", "")
	values.Set("theme", "light")
	return "/explore?" + values.Encode()
}

func proxyRequestPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	parsed, err := url.Parse(path)
	if err == nil && parsed.IsAbs() {
		return strings.TrimLeft(parsed.RequestURI(), "/")
	}
	return strings.TrimLeft(path, "/")
}

func checkToolHTTP(inst model.ServiceInstallation, path string) error {
	baseURL := toolHTTPBaseURL(inst)
	if baseURL == "" {
		return fmt.Errorf("unsupported service type %s", inst.ServiceType)
	}
	client := http.Client{Timeout: 10 * time.Second}
	res, err := client.Get(baseURL + path)
	if err != nil {
		return fmt.Errorf("%s health check failed: %w", inst.ServiceType, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 500 {
		return fmt.Errorf("%s health check returned %d", inst.ServiceType, res.StatusCode)
	}
	return nil
}

func checkRegistryHealth(inst model.ServiceInstallation, path string) error {
	if inst.ServiceType == "registry" {
		return k8s.NewRegistryClient(inst.Namespace).HealthCheck(context.Background())
	}
	if inst.ServiceType == "harbor" {
		return k8s.NewHarborClient(inst.Namespace).HealthCheck(context.Background())
	}
	return checkToolHTTP(inst, path)
}

func toolHTTPBaseURL(inst model.ServiceInstallation) string {
	if inst.Namespace == "" {
		return ""
	}
	switch inst.ServiceType {
	case "git":
		return fmt.Sprintf("http://%s.%s.svc.cluster.local:3000", inst.Namespace, inst.Namespace)
	case "deploy":
		return fmt.Sprintf("http://%s-argocd-server.%s.svc.cluster.local", inst.Namespace, inst.Namespace)
	case "monitor":
		return fmt.Sprintf("http://%s-grafana.%s.svc.cluster.local", inst.Namespace, inst.Namespace)
	case "log":
		return k8s.NewLokiClient(inst.Namespace).BaseURL
	case "ci":
		return fmt.Sprintf("http://%s.%s.svc.cluster.local:8080", inst.Namespace, inst.Namespace)
	case "registry":
		return fmt.Sprintf("https://%s.%s.svc.cluster.local:5000", inst.Namespace, inst.Namespace)
	case "harbor":
		return fmt.Sprintf("http://harbor-portal.%s.svc.cluster.local", inst.Namespace)
	case "minio":
		return fmt.Sprintf("http://minio.%s.svc.cluster.local:9001", inst.Namespace)
	default:
		return ""
	}
}

type defaultGrafanaDashboard struct {
	Title string
	JSON  string
}

func buildDefaultGrafanaDashboards(app model.Application, env model.Environment, components []model.Component) []defaultGrafanaDashboard {
	dashboards := []defaultGrafanaDashboard{{
		Title: fmt.Sprintf("%s-%s-overview", app.Identifier, env.Identifier),
		JSON:  buildDefaultGrafanaDashboard(app, env, components),
	}}
	for _, spec := range builtinGrafanaDashboardSpecs() {
		dashboards = append(dashboards, defaultGrafanaDashboard{
			Title: spec.Title,
			JSON:  buildWorkloadGrafanaDashboard(spec),
		})
	}
	return dashboards
}

type grafanaDashboardSpec struct {
	UID         string
	Title       string
	Description string
	Tags        []string
	Matcher     string
	ServiceType string
}

func builtinGrafanaDashboardSpecs() []grafanaDashboardSpec {
	return []grafanaDashboardSpec{
		{UID: "paap-pod-workload", Title: "PAAP Pod Workload", Description: "组件 Pod CPU、内存、重启和网络。", Tags: []string{"paap", "component"}, Matcher: "$workload.*"},
		{UID: "paap-tool-workload", Title: "PAAP Tool Workload", Description: "平台工具通用 Kubernetes 工作负载指标。", Tags: []string{"paap", "tool"}, Matcher: ".*$workload.*"},
		{UID: "paap-middleware-workload", Title: "PAAP Middleware Workload", Description: "数据库和中间件通用 Kubernetes 工作负载指标。", Tags: []string{"paap", "middleware"}, Matcher: ".*$workload.*"},
		{UID: "paap-gitea", Title: "PAAP Gitea", Description: "Gitea 服务 Pod、CPU、内存、重启和网络。", Tags: []string{"paap", "tool", "gitea"}, Matcher: ".*gitea.*|.*$workload.*", ServiceType: "gitea"},
		{UID: "paap-argocd", Title: "PAAP ArgoCD", Description: "ArgoCD server/repo/application controller 工作负载指标。", Tags: []string{"paap", "tool", "argocd"}, Matcher: ".*argocd.*|.*$workload.*", ServiceType: "argocd"},
		{UID: "paap-jenkins", Title: "PAAP Jenkins", Description: "Jenkins controller 和 agent 工作负载指标。", Tags: []string{"paap", "tool", "jenkins"}, Matcher: ".*jenkins.*|.*$workload.*", ServiceType: "jenkins"},
		{UID: "paap-registry", Title: "PAAP Registry", Description: "Registry/Harbor 工作负载指标。", Tags: []string{"paap", "tool", "registry"}, Matcher: ".*registry.*|.*harbor.*|.*$workload.*", ServiceType: "registry"},
		{UID: "paap-loki", Title: "PAAP Loki", Description: "Loki/Promtail 日志系统工作负载指标。", Tags: []string{"paap", "tool", "loki"}, Matcher: ".*loki.*|.*promtail.*|.*$workload.*", ServiceType: "loki"},
		{UID: "paap-grafana", Title: "PAAP Grafana Prometheus", Description: "Grafana、Prometheus 和 Alertmanager 监控系统工作负载指标。", Tags: []string{"paap", "tool", "grafana", "prometheus"}, Matcher: ".*grafana.*|.*prometheus.*|.*alertmanager.*|.*$workload.*", ServiceType: "grafana"},
		{UID: "paap-mysql", Title: "PAAP MySQL", Description: "MySQL 工作负载指标。", Tags: []string{"paap", "middleware", "mysql"}, Matcher: ".*mysql.*|.*$workload.*", ServiceType: "mysql"},
		{UID: "paap-postgresql", Title: "PAAP PostgreSQL", Description: "PostgreSQL 工作负载指标。", Tags: []string{"paap", "middleware", "postgresql"}, Matcher: ".*postgres.*|.*$workload.*", ServiceType: "postgresql"},
		{UID: "paap-mongodb", Title: "PAAP MongoDB", Description: "MongoDB 工作负载指标。", Tags: []string{"paap", "middleware", "mongodb"}, Matcher: ".*mongo.*|.*$workload.*", ServiceType: "mongodb"},
		{UID: "paap-redis", Title: "PAAP Redis", Description: "Redis 工作负载指标。", Tags: []string{"paap", "middleware", "redis"}, Matcher: ".*redis.*|.*$workload.*", ServiceType: "redis"},
		{UID: "paap-rabbitmq", Title: "PAAP RabbitMQ", Description: "RabbitMQ 工作负载指标。", Tags: []string{"paap", "middleware", "rabbitmq"}, Matcher: ".*rabbitmq.*|.*$workload.*", ServiceType: "rabbitmq"},
		{UID: "paap-kafka", Title: "PAAP Kafka", Description: "Kafka 工作负载指标。", Tags: []string{"paap", "middleware", "kafka"}, Matcher: ".*kafka.*|.*$workload.*", ServiceType: "kafka"},
		{UID: "paap-minio", Title: "PAAP MinIO", Description: "MinIO 工作负载指标。", Tags: []string{"paap", "middleware", "minio"}, Matcher: ".*minio.*|.*$workload.*", ServiceType: "minio"},
	}
}

func buildWorkloadGrafanaDashboard(spec grafanaDashboardSpec) string {
	panels := workloadGrafanaPanels(spec)
	dashboard := map[string]interface{}{
		"title":         spec.Title,
		"uid":           spec.UID,
		"description":   spec.Description,
		"schemaVersion": 39,
		"tags":          spec.Tags,
		"time":          map[string]string{"from": "now-1h", "to": "now"},
		"templating": map[string]interface{}{
			"list": []map[string]interface{}{
				{"name": "namespace", "type": "textbox", "hide": 2, "current": map[string]string{"text": "", "value": ""}},
				{"name": "workload", "type": "textbox", "hide": 2, "current": map[string]string{"text": "", "value": ""}},
			},
		},
		"panels":      panels,
		"annotations": map[string]interface{}{"list": []interface{}{}},
	}
	data, _ := json.Marshal(dashboard)
	return string(data)
}

func workloadGrafanaPanels(spec grafanaDashboardSpec) []map[string]interface{} {
	matcher := spec.Matcher
	panels := []map[string]interface{}{
		grafanaPanel(1, "Pods", "stat", 0, 0, 4, 4, fmt.Sprintf(`count(kube_pod_info{namespace="$namespace", pod=~"%s"})`, matcher)),
		grafanaPanel(2, "Pods Ready", "stat", 4, 0, 4, 4, fmt.Sprintf(`sum(kube_pod_status_ready{namespace="$namespace", pod=~"%s", condition="true"})`, matcher)),
		grafanaPanel(3, "Desired Replicas", "stat", 8, 0, 4, 4, `sum(kube_deployment_spec_replicas{namespace="$namespace", deployment=~"$workload.*"}) + sum(kube_statefulset_replicas{namespace="$namespace", statefulset=~"$workload.*"})`),
		grafanaPanel(4, "Available Replicas", "stat", 12, 0, 4, 4, `sum(kube_deployment_status_replicas_available{namespace="$namespace", deployment=~"$workload.*"}) + sum(kube_statefulset_status_replicas_ready{namespace="$namespace", statefulset=~"$workload.*"})`),
		grafanaPanel(5, "Unavailable Replicas", "stat", 16, 0, 4, 4, `sum(kube_deployment_status_replicas_unavailable{namespace="$namespace", deployment=~"$workload.*"})`),
		grafanaPanel(6, "Restarts 1h", "stat", 20, 0, 4, 4, fmt.Sprintf(`sum(increase(kube_pod_container_status_restarts_total{namespace="$namespace", pod=~"%s"}[1h]))`, matcher)),
		grafanaPanel(7, "CPU Usage", "timeseries", 0, 4, 8, 7, fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="$namespace", pod=~"%s", container!="POD", container!=""}[5m])) by (pod)`, matcher)),
		grafanaPanel(8, "CPU Requests", "timeseries", 8, 4, 8, 7, fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="$namespace", pod=~"%s", resource="cpu"}) by (pod)`, matcher)),
		grafanaPanel(9, "CPU Throttling", "timeseries", 16, 4, 8, 7, fmt.Sprintf(`sum(rate(container_cpu_cfs_throttled_seconds_total{namespace="$namespace", pod=~"%s", container!="POD", container!=""}[5m])) by (pod)`, matcher)),
		grafanaPanel(10, "Memory Working Set", "timeseries", 0, 11, 8, 7, fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="$namespace", pod=~"%s", container!="POD", container!=""}) by (pod)`, matcher)),
		grafanaPanel(11, "Memory Requests", "timeseries", 8, 11, 8, 7, fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="$namespace", pod=~"%s", resource="memory"}) by (pod)`, matcher)),
		grafanaPanel(12, "Memory Limit %", "timeseries", 16, 11, 8, 7, ratioPromQL(
			fmt.Sprintf(`sum(container_memory_working_set_bytes{namespace="$namespace", pod=~"%s", container!="POD", container!=""}) by (pod)`, matcher),
			fmt.Sprintf(`sum(kube_pod_container_resource_limits{namespace="$namespace", pod=~"%s", resource="memory"}) by (pod)`, matcher),
			100,
		)),
		grafanaPanel(13, "Restarts", "timeseries", 0, 18, 8, 7, fmt.Sprintf(`sum(increase(kube_pod_container_status_restarts_total{namespace="$namespace", pod=~"%s"}[1h])) by (pod, container)`, matcher)),
		grafanaPanel(14, "Network Receive", "timeseries", 8, 18, 8, 7, fmt.Sprintf(`sum(rate(container_network_receive_bytes_total{namespace="$namespace", pod=~"%s"}[5m])) by (pod)`, matcher)),
		grafanaPanel(15, "Network Transmit", "timeseries", 16, 18, 8, 7, fmt.Sprintf(`sum(rate(container_network_transmit_bytes_total{namespace="$namespace", pod=~"%s"}[5m])) by (pod)`, matcher)),
		grafanaPanel(16, "Filesystem Usage", "timeseries", 0, 25, 8, 7, fmt.Sprintf(`sum(container_fs_usage_bytes{namespace="$namespace", pod=~"%s", container!="POD", container!=""}) by (pod)`, matcher)),
		grafanaPanel(17, "PVCs", "stat", 8, 25, 8, 7, `count(kube_persistentvolumeclaim_info{namespace="$namespace", persistentvolumeclaim=~".*$workload.*"}) or vector(0)`),
		grafanaPanel(18, "Container Count", "stat", 16, 25, 8, 7, fmt.Sprintf(`count(kube_pod_container_info{namespace="$namespace", pod=~"%s"})`, matcher)),
		grafanaPanel(19, "Waiting Containers", "stat", 0, 32, 4, 4, fmt.Sprintf(`sum(kube_pod_container_status_waiting{namespace="$namespace", pod=~"%s"}) or vector(0)`, matcher)),
		grafanaPanel(20, "Terminated Containers", "stat", 4, 32, 4, 4, fmt.Sprintf(`sum(kube_pod_container_status_terminated{namespace="$namespace", pod=~"%s"}) or vector(0)`, matcher)),
		grafanaPanel(21, "CPU Usage / Requests", "timeseries", 8, 32, 8, 7, ratioPromQL(
			fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="$namespace", pod=~"%s", container!="POD", container!=""}[5m])) by (pod)`, matcher),
			fmt.Sprintf(`sum(kube_pod_container_resource_requests{namespace="$namespace", pod=~"%s", resource="cpu"}) by (pod)`, matcher),
			100,
		)),
		grafanaPanel(22, "CPU Throttled Periods", "timeseries", 16, 32, 8, 7, fmt.Sprintf(`sum(rate(container_cpu_cfs_throttled_periods_total{namespace="$namespace", pod=~"%s", container!="POD", container!=""}[5m])) by (pod)`, matcher)),
		grafanaPanel(23, "Memory RSS", "timeseries", 0, 39, 8, 7, fmt.Sprintf(`sum(container_memory_rss{namespace="$namespace", pod=~"%s", container!="POD", container!=""}) by (pod)`, matcher)),
		grafanaPanel(24, "Memory Cache", "timeseries", 8, 39, 8, 7, fmt.Sprintf(`sum(container_memory_cache{namespace="$namespace", pod=~"%s", container!="POD", container!=""}) by (pod)`, matcher)),
		grafanaPanel(25, "OOM Events", "timeseries", 16, 39, 8, 7, fmt.Sprintf(`sum(increase(container_oom_events_total{namespace="$namespace", pod=~"%s"}[1h])) by (pod)`, matcher)),
		grafanaPanel(26, "Network Receive Packets", "timeseries", 0, 46, 6, 7, fmt.Sprintf(`sum(rate(container_network_receive_packets_total{namespace="$namespace", pod=~"%s"}[5m])) by (pod)`, matcher)),
		grafanaPanel(27, "Network Transmit Packets", "timeseries", 6, 46, 6, 7, fmt.Sprintf(`sum(rate(container_network_transmit_packets_total{namespace="$namespace", pod=~"%s"}[5m])) by (pod)`, matcher)),
		grafanaPanel(28, "Network Receive Errors", "timeseries", 12, 46, 6, 7, fmt.Sprintf(`sum(rate(container_network_receive_errors_total{namespace="$namespace", pod=~"%s"}[5m])) by (pod)`, matcher)),
		grafanaPanel(29, "Network Transmit Errors", "timeseries", 18, 46, 6, 7, fmt.Sprintf(`sum(rate(container_network_transmit_errors_total{namespace="$namespace", pod=~"%s"}[5m])) by (pod)`, matcher)),
		grafanaPanel(30, "Filesystem Reads", "timeseries", 0, 53, 8, 7, fmt.Sprintf(`sum(rate(container_fs_reads_bytes_total{namespace="$namespace", pod=~"%s", container!="POD", container!=""}[5m])) by (pod)`, matcher)),
		grafanaPanel(31, "Filesystem Writes", "timeseries", 8, 53, 8, 7, fmt.Sprintf(`sum(rate(container_fs_writes_bytes_total{namespace="$namespace", pod=~"%s", container!="POD", container!=""}[5m])) by (pod)`, matcher)),
		grafanaPanel(32, "Pod Phases", "timeseries", 16, 53, 8, 7, fmt.Sprintf(`sum(kube_pod_status_phase{namespace="$namespace", pod=~"%s"}) by (pod, phase)`, matcher)),
	}
	nextID := 33
	nextY := 60
	for _, panel := range serviceSpecificGrafanaPanels(spec.ServiceType, matcher, &nextID, &nextY) {
		panels = append(panels, panel)
	}
	return panels
}

func serviceSpecificGrafanaPanels(serviceType, matcher string, nextID *int, nextY *int) []map[string]interface{} {
	add := func(title, panelType string, x, y, w, h int, expr string) map[string]interface{} {
		panel := grafanaPanel(*nextID, title, panelType, x, y, w, h, expr)
		*nextID++
		return panel
	}
	row := func(title1, expr1, title2, expr2, title3, expr3 string) []map[string]interface{} {
		y := *nextY
		*nextY += 7
		return []map[string]interface{}{
			add(title1, "timeseries", 0, y, 8, 7, expr1),
			add(title2, "timeseries", 8, y, 8, 7, expr2),
			add(title3, "timeseries", 16, y, 8, 7, expr3),
		}
	}
	switch serviceType {
	case "mysql":
		return append(
			row("Database Connections", `sum(mysql_global_status_threads_connected{namespace="$namespace"})`, "Queries", `sum(rate(mysql_global_status_questions{namespace="$namespace"}[5m]))`, "Slow Queries", `sum(rate(mysql_global_status_slow_queries{namespace="$namespace"}[5m]))`),
			row("Replication Lag", `max(mysql_slave_status_seconds_behind_master{namespace="$namespace"})`, "InnoDB Buffer Pool Hit", fmt.Sprintf(`100 * (1 - %s)`, ratioPromQL(`sum(rate(mysql_global_status_innodb_buffer_pool_reads{namespace="$namespace"}[5m]))`, `sum(rate(mysql_global_status_innodb_buffer_pool_read_requests{namespace="$namespace"}[5m]))`, 1)), "Table Locks Waited", `sum(rate(mysql_global_status_table_locks_waited{namespace="$namespace"}[5m]))`)...,
		)
	case "postgresql":
		return append(
			row("Database Connections", `sum(pg_stat_activity_count{namespace="$namespace"})`, "Transactions", `sum(rate(pg_stat_database_xact_commit{namespace="$namespace"}[5m]) + rate(pg_stat_database_xact_rollback{namespace="$namespace"}[5m]))`, "Cache Hit Ratio", ratioPromQL(`sum(pg_stat_database_blks_hit{namespace="$namespace"})`, `(sum(pg_stat_database_blks_hit{namespace="$namespace"}) + sum(pg_stat_database_blks_read{namespace="$namespace"}))`, 100)),
			row("Deadlocks", `sum(increase(pg_stat_database_deadlocks{namespace="$namespace"}[1h]))`, "Replication Lag", `max(pg_replication_lag{namespace="$namespace"})`, "Locks", `sum(pg_locks_count{namespace="$namespace"})`)...,
		)
	case "mongodb":
		return append(
			row("Connections", `sum(mongodb_connections{namespace="$namespace", state="current"})`, "Operations", `sum(rate(mongodb_op_counters_total{namespace="$namespace"}[5m])) by (type)`, "Replication Lag", `max(mongodb_mongod_replset_member_replication_lag{namespace="$namespace"})`),
			row("WiredTiger Cache Used", `sum(mongodb_mongod_wiredtiger_cache_bytes{namespace="$namespace", type="bytes currently in the cache"})`, "Queued Operations", `sum(mongodb_mongod_global_lock_current_queue{namespace="$namespace"})`, "Page Faults", `sum(rate(mongodb_extra_info_page_faults_total{namespace="$namespace"}[5m]))`)...,
		)
	case "redis":
		return append(
			row("Connected Clients", `sum(redis_connected_clients{namespace="$namespace"})`, "Commands", `sum(rate(redis_commands_processed_total{namespace="$namespace"}[5m]))`, "Memory Used", `sum(redis_memory_used_bytes{namespace="$namespace"})`),
			row("Keyspace Hits", `sum(rate(redis_keyspace_hits_total{namespace="$namespace"}[5m]))`, "Keyspace Misses", `sum(rate(redis_keyspace_misses_total{namespace="$namespace"}[5m]))`, "Evicted Keys", `sum(rate(redis_evicted_keys_total{namespace="$namespace"}[5m]))`)...,
		)
	case "rabbitmq":
		return append(
			row("Queue Ready Messages", `sum(rabbitmq_queue_messages_ready{namespace="$namespace"}) by (queue)`, "Queue Unacked Messages", `sum(rabbitmq_queue_messages_unacked{namespace="$namespace"}) by (queue)`, "Consumers", `sum(rabbitmq_queue_consumers{namespace="$namespace"}) by (queue)`),
			row("Publish Rate", `sum(rate(rabbitmq_queue_messages_published_total{namespace="$namespace"}[5m])) by (queue)`, "Deliver Rate", `sum(rate(rabbitmq_queue_messages_delivered_total{namespace="$namespace"}[5m])) by (queue)`, "Connections", `sum(rabbitmq_connections{namespace="$namespace"})`)...,
		)
	case "kafka":
		return append(
			row("Messages In", `sum(rate(kafka_server_brokertopicmetrics_messagesin_total{namespace="$namespace"}[5m])) by (topic)`, "Bytes In", `sum(rate(kafka_server_brokertopicmetrics_bytesin_total{namespace="$namespace"}[5m])) by (topic)`, "Bytes Out", `sum(rate(kafka_server_brokertopicmetrics_bytesout_total{namespace="$namespace"}[5m])) by (topic)`),
			row("Consumer Lag", `sum(kafka_consumergroup_lag{namespace="$namespace"}) by (consumergroup, topic)`, "Under Replicated Partitions", `sum(kafka_server_replicamanager_underreplicatedpartitions{namespace="$namespace"})`, "Offline Partitions", `sum(kafka_controller_kafkacontroller_offlinepartitionscount{namespace="$namespace"})`)...,
		)
	case "minio":
		return append(
			row("Usable Capacity", `sum(minio_cluster_capacity_usable_total_bytes{namespace="$namespace"})`, "Used Capacity", `sum(minio_cluster_capacity_usable_total_bytes{namespace="$namespace"} - minio_cluster_capacity_usable_free_bytes{namespace="$namespace"})`, "S3 Requests", `sum(rate(minio_s3_requests_total{namespace="$namespace"}[5m])) by (api)`),
			row("S3 Errors", `sum(rate(minio_s3_requests_errors_total{namespace="$namespace"}[5m])) by (api)`, "Objects", `sum(minio_bucket_objects_count{namespace="$namespace"}) by (bucket)`, "Traffic", `sum(rate(minio_s3_traffic_received_bytes{namespace="$namespace"}[5m]) + rate(minio_s3_traffic_sent_bytes{namespace="$namespace"}[5m])) by (server)`)...,
		)
	case "gitea":
		return row("HTTP Requests", `sum(rate(gitea_http_request_duration_seconds_count{namespace="$namespace"}[5m])) by (route)`, "Request Latency P95", `histogram_quantile(0.95, sum(rate(gitea_http_request_duration_seconds_bucket{namespace="$namespace"}[5m])) by (le, route))`, "Git Operations", `sum(rate(gitea_git_request_duration_seconds_count{namespace="$namespace"}[5m])) by (operation)`)
	case "argocd":
		return row("Applications", `sum(argocd_app_info{namespace="$namespace"})`, "Out Of Sync Apps", `sum(argocd_app_info{namespace="$namespace", sync_status!="Synced"})`, "Unhealthy Apps", `sum(argocd_app_info{namespace="$namespace", health_status!="Healthy"})`)
	case "jenkins":
		return row("Builds", `sum(rate(jenkins_runs_total{namespace="$namespace"}[5m])) by (job)`, "Build Failures", `sum(rate(jenkins_runs_failure_total{namespace="$namespace"}[5m])) by (job)`, "Queue Size", `sum(jenkins_queue_size_value{namespace="$namespace"})`)
	case "registry":
		return row("Registry Requests", `sum(rate(registry_http_requests_total{namespace="$namespace"}[5m])) by (code, method)`, "Registry Latency P95", `histogram_quantile(0.95, sum(rate(registry_http_request_duration_seconds_bucket{namespace="$namespace"}[5m])) by (le, handler))`, "Storage Used", `sum(registry_storage_cache_total{namespace="$namespace"})`)
	case "loki":
		return row("Ingest Rate", `sum(rate(loki_distributor_bytes_received_total{namespace="$namespace"}[5m]))`, "Log Lines", `sum(rate(loki_distributor_lines_received_total{namespace="$namespace"}[5m]))`, "Query Latency P95", `histogram_quantile(0.95, sum(rate(loki_request_duration_seconds_bucket{namespace="$namespace"}[5m])) by (le, route))`)
	case "grafana":
		return row("Grafana Requests", `sum(rate(grafana_http_request_duration_seconds_count{namespace="$namespace"}[5m])) by (handler, status_code)`, "Prometheus Samples", `sum(rate(prometheus_tsdb_head_samples_appended_total{namespace="$namespace"}[5m]))`, "Alertmanager Alerts", `sum(alertmanager_alerts{namespace="$namespace"})`)
	default:
		return nil
	}
}

func grafanaPanel(id int, title, panelType string, x, y, w, h int, expr string) map[string]interface{} {
	return map[string]interface{}{
		"id":      id,
		"title":   title,
		"type":    panelType,
		"gridPos": map[string]int{"x": x, "y": y, "w": w, "h": h},
		"targets": []map[string]string{{"expr": promQLZeroFallback(expr)}},
	}
}

func promQLZeroFallback(expr string) string {
	return fmt.Sprintf("(%s) or vector(0)", expr)
}

func ratioPromQL(numerator, denominator string, multiplier int) string {
	if multiplier == 1 {
		return fmt.Sprintf(`(%s) / clamp_min((%s), 1)`, numerator, denominator)
	}
	return fmt.Sprintf(`%d * (%s) / clamp_min((%s), 1)`, multiplier, numerator, denominator)
}

func buildDefaultGrafanaDashboard(app model.Application, env model.Environment, components []model.Component) string {
	componentTargets := make([]string, 0, len(components))
	for _, comp := range components {
		componentTargets = append(componentTargets, service.ComponentIdentifier(comp.Name, comp.Type, comp.ID))
	}
	title := fmt.Sprintf("PAAP %s/%s Overview", app.Identifier, env.Identifier)
	baseNamespace := fmt.Sprintf("%s-%s", app.Identifier, env.Identifier)
	namespacePattern := regexp.QuoteMeta(baseNamespace) + `.*`
	environmentNamespaceSelector := fmt.Sprintf(`kube_namespace_status_phase{namespace=~"%s", phase="Active"}`, namespacePattern)
	workloadNamespaceSelector := fmt.Sprintf(`kube_namespace_status_phase{namespace="%s", phase="Active"}`, baseNamespace)
	dashboard := map[string]interface{}{
		"title":         title,
		"uid":           fmt.Sprintf("paap-%s-%s-overview", app.Identifier, env.Identifier),
		"schemaVersion": 39,
		"tags":          []string{"paap", app.Identifier, env.Identifier},
		"time": map[string]string{
			"from": "now-6h",
			"to":   "now",
		},
		"templating": map[string]interface{}{
			"list": []map[string]interface{}{
				{
					"name":    "environment",
					"type":    "constant",
					"current": map[string]string{"text": env.Identifier, "value": env.Identifier},
				},
			},
		},
		"panels": []map[string]interface{}{
			grafanaPanel(1, "Environment Namespaces", "stat", 0, 0, 4, 4, fmt.Sprintf(`count(%s)`, environmentNamespaceSelector)),
			grafanaPanel(2, "Workload Namespaces", "stat", 4, 0, 4, 4, fmt.Sprintf(`count(%s)`, workloadNamespaceSelector)),
			grafanaPanel(3, "Workloads", "stat", 8, 0, 4, 4, fmt.Sprintf(`count(count by (namespace, owner_kind, owner_name) (%s))`, runningPodMetricExpr(`kube_pod_owner{namespace=~"%s", owner_is_controller="true"}`, namespacePattern))),
			grafanaPanel(4, "Running Pods", "stat", 12, 0, 4, 4, fmt.Sprintf(`sum(kube_pod_status_phase{namespace=~"%s", phase="Running"})`, namespacePattern)),
			grafanaPanel(5, "Unready Pods", "stat", 16, 0, 4, 4, fmt.Sprintf(`sum(%s)`, runningPodMetricExpr(`kube_pod_status_ready{namespace=~"%s", condition="false"}`, namespacePattern))),
			grafanaPanel(6, "PVCs", "stat", 20, 0, 4, 4, fmt.Sprintf(`count(kube_persistentvolumeclaim_info{namespace=~"%s"}) or vector(0)`, namespacePattern)),
			grafanaPanel(16, "Containers", "stat", 0, 22, 4, 4, fmt.Sprintf(`count(%s)`, runningPodMetricExpr(`kube_pod_container_info{namespace=~"%s"}`, namespacePattern))),
			grafanaPanel(7, "CPU Usage by Namespace", "timeseries", 0, 4, 8, 7, fmt.Sprintf(`sum(%s) by (namespace)`, runningPodMetricExpr(`rate(container_cpu_usage_seconds_total{namespace=~"%s", container!="POD", container!=""}[5m])`, namespacePattern))),
			grafanaPanel(8, "Memory Usage by Namespace", "timeseries", 8, 4, 8, 7, fmt.Sprintf(`sum(%s) by (namespace)`, runningPodMetricExpr(`container_memory_working_set_bytes{namespace=~"%s", container!="POD", container!=""}`, namespacePattern))),
			grafanaPanel(9, "Restarts by Namespace", "timeseries", 16, 4, 8, 7, fmt.Sprintf(`sum(%s) by (namespace)`, runningPodMetricExpr(`increase(kube_pod_container_status_restarts_total{namespace=~"%s"}[1h])`, namespacePattern))),
			grafanaPanel(10, "Network Receive by Namespace", "timeseries", 0, 11, 8, 7, fmt.Sprintf(`sum(%s) by (namespace)`, runningPodMetricExpr(`rate(container_network_receive_bytes_total{namespace=~"%s"}[5m])`, namespacePattern))),
			grafanaPanel(11, "Network Transmit by Namespace", "timeseries", 8, 11, 8, 7, fmt.Sprintf(`sum(%s) by (namespace)`, runningPodMetricExpr(`rate(container_network_transmit_bytes_total{namespace=~"%s"}[5m])`, namespacePattern))),
			grafanaPanel(12, "PVCs by Namespace", "timeseries", 16, 11, 8, 7, fmt.Sprintf(`count(kube_persistentvolumeclaim_info{namespace=~"%s"}) by (namespace)`, namespacePattern)),
			grafanaPanel(13, "Pod Readiness by Namespace", "timeseries", 0, 18, 8, 7, fmt.Sprintf(`sum(%s) by (namespace)`, runningPodMetricExpr(`kube_pod_status_ready{namespace=~"%s", condition="true"}`, namespacePattern))),
			grafanaPanel(14, "Pod Restarts by Pod", "timeseries", 8, 18, 8, 7, fmt.Sprintf(`sum(%s) by (namespace, pod)`, runningPodMetricExpr(`increase(kube_pod_container_status_restarts_total{namespace=~"%s"}[1h])`, namespacePattern))),
			grafanaPanel(15, "Workloads by Namespace", "timeseries", 16, 18, 8, 7, fmt.Sprintf(`count(%s) by (namespace, owner_kind)`, runningPodMetricExpr(`kube_pod_owner{namespace=~"%s", owner_is_controller="true"}`, namespacePattern))),
		},
		"annotations": map[string]interface{}{
			"list": []interface{}{},
		},
	}
	if len(componentTargets) > 0 {
		dashboard["description"] = "Components: " + strings.Join(componentTargets, ", ")
	}
	data, _ := json.Marshal(dashboard)
	return string(data)
}

func runningPodStatusExpr(namespacePattern string) string {
	return fmt.Sprintf(`max by (namespace, pod) (kube_pod_status_phase{namespace=~"%s", phase="Running"})`, namespacePattern)
}

func runningPodMetricExpr(metricPattern, namespacePattern string) string {
	return fmt.Sprintf(`(%s) * on(namespace, pod) group_left() %s * on(namespace, pod) group_left() %s`, fmt.Sprintf(metricPattern, namespacePattern), runningPodStatusExpr(namespacePattern), nonJobPodOwnerExpr(namespacePattern))
}

func nonJobPodOwnerExpr(namespacePattern string) string {
	return fmt.Sprintf(`max by (namespace, pod) (kube_pod_owner{namespace=~"%s", owner_is_controller="true", owner_kind!="Job"})`, namespacePattern)
}

// CreateServiceDraft adds a service card to the environment canvas without
// creating any Kubernetes runtime resources. Deployment remains explicit.
func CreateServiceDraft(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))

	var req InstallServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}

	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var svcTmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", req.ServiceType).First(&svcTmpl).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service template not found"})
		return
	}

	toolNS := serviceToolNamespace(app, env, &svcTmpl, req.ServiceType)
	helmSpec := buildHelmInstallSpec(&app, &env, &svcTmpl, req.ServiceType, req.Values)

	var inst model.ServiceInstallation
	err := database.DB.Unscoped().
		Where("environment_id = ? AND service_type = ?", env.ID, req.ServiceType).
		Order("CASE WHEN deleted_at IS NULL THEN 0 ELSE 1 END, id").
		First(&inst).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	statusCode := http.StatusCreated
	if err == nil {
		statusCode = http.StatusOK
		if inst.DeletedAt.Valid {
			inst.DeletedAt = gorm.DeletedAt{}
			statusCode = http.StatusCreated
		}
		switch strings.ToLower(strings.TrimSpace(inst.Status)) {
		case "running", "installing", "deleting":
			database.DB.Save(&inst)
			c.JSON(http.StatusOK, gin.H{"data": inst})
			return
		}
	} else {
		inst = model.ServiceInstallation{
			EnvironmentID: env.ID,
			ServiceType:   req.ServiceType,
		}
	}

	inst.Status = "draft"
	inst.ServiceName = serviceToolIdentity(&svcTmpl, req.ServiceType)
	inst.Namespace = valueOrFallback(helmSpec.Namespace, toolNS)
	inst.ReleaseName = valueOrFallback(helmSpec.ReleaseName, inst.Namespace)
	inst.ErrorMessage = ""
	inst.Values = serviceValuesJSON(helmSpec.Values)
	if err := database.DB.Save(&inst).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Unscoped().
		Where("environment_id = ? AND service_type = ? AND id <> ?", env.ID, req.ServiceType, inst.ID).
		Delete(&model.ServiceInstallation{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(statusCode, gin.H{"data": inst})
}

// InstallService installs a service (tool) in an environment via CR
func InstallService(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))

	var req InstallServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	// 查找模板获取安装方式
	var svcTmpl model.ServiceTemplate
	query := database.DB.Where("type = ? AND enabled = ?", req.ServiceType, true)
	if req.AppVersion != "" {
		query = query.Where("app_version = ?", req.AppVersion)
	}
	if err := query.Order("install_order").First(&svcTmpl).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service template not found"})
		return
	}
	toolNS := serviceToolNamespace(app, env, &svcTmpl, req.ServiceType)
	var inst model.ServiceInstallation
	err := database.DB.Unscoped().
		Where("environment_id = ? AND service_type = ?", env.ID, req.ServiceType).
		Order("CASE WHEN deleted_at IS NULL THEN 0 ELSE 1 END, id").
		First(&inst).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err == nil {
		if inst.DeletedAt.Valid {
			inst.DeletedAt = gorm.DeletedAt{}
		}
		if len(req.Values) == 0 && (inst.Status == "running" || inst.Status == "installing") {
			database.DB.Save(&inst)
			c.JSON(http.StatusOK, gin.H{"data": inst})
			return
		}
	} else {
		inst = model.ServiceInstallation{
			EnvironmentID: env.ID,
			ServiceType:   req.ServiceType,
		}
	}

	inst.Status = "installing"
	inst.ServiceName = serviceToolIdentity(&svcTmpl, req.ServiceType)
	inst.Namespace = toolNS
	inst.ReleaseName = toolNS
	inst.ErrorMessage = ""

	ctx := context.Background()
	var manifestsRef *paapv1.ConfigMapReference
	// 统一走 Operator：Helm chart 交给 ServiceInstance spec 记录，operator 负责安装
	helmSpec := buildHelmInstallSpec(&app, &env, &svcTmpl, req.ServiceType, req.Values)
	inst.Values = serviceValuesJSON(helmSpec.Values)
	if err := database.DB.Save(&inst).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := database.DB.Unscoped().
		Where("environment_id = ? AND service_type = ? AND id <> ?", env.ID, req.ServiceType, inst.ID).
		Delete(&model.ServiceInstallation{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	workloadRole := getWorkloadRole(req.ServiceType)
	toolNamespaceRole := getToolNamespaceRole(req.ServiceType)
	environmentRole := getEnvironmentRole(req.ServiceType)
	clusterRole := getClusterRole(req.ServiceType)
	resourceLabels := serviceResourceLabels(app.Identifier, env.Identifier, &svcTmpl, req.ServiceType)
	resourceAnnotations := serviceResourceAnnotations(app.Identifier, env.Identifier, &svcTmpl, req.ServiceType)

	if err := k8s.CreateServiceInstanceCR(ctx, app.Identifier, env.Identifier, req.ServiceType, workloadRole, toolNamespaceRole, environmentRole, clusterRole, manifestsRef, helmSpec, resourceLabels, resourceAnnotations); err != nil {
		inst.Status = "failed"
		inst.ErrorMessage = err.Error()
		database.DB.Save(&inst)
		c.JSON(http.StatusCreated, gin.H{"data": inst, "warning": "ServiceInstance CR creation failed: " + err.Error()})
		return
	}

	inst.Status = "installing"
	database.DB.Save(&inst)
	c.JSON(http.StatusCreated, gin.H{"data": inst})
}

// UpdateService saves a service's deployable configuration. Draft services
// remain database-only; services that already have runtime state reconcile the
// ServiceInstance CR so Helm sees the new desired values.
func UpdateService(c *gin.Context) {
	syncClusterStateIfPossible()

	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var req UpdateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var inst model.ServiceInstallation
	if err := database.DB.Where("id = ? AND environment_id = ?", serviceID, envID).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service installation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	var svcTmpl model.ServiceTemplate
	if err := database.DB.Where("type = ?", inst.ServiceType).First(&svcTmpl).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service template not found"})
		return
	}

	helmSpec := buildHelmInstallSpec(&app, &env, &svcTmpl, inst.ServiceType, req.Values)
	applyStoredServiceRuntimeIdentity(&inst, helmSpec)
	if serviceStatusShouldReconcile(inst.Status) {
		workloadRole := getWorkloadRole(inst.ServiceType)
		toolNamespaceRole := getToolNamespaceRole(inst.ServiceType)
		environmentRole := getEnvironmentRole(inst.ServiceType)
		clusterRole := getClusterRole(inst.ServiceType)
		resourceLabels := serviceResourceLabels(app.Identifier, env.Identifier, &svcTmpl, inst.ServiceType)
		resourceAnnotations := serviceResourceAnnotations(app.Identifier, env.Identifier, &svcTmpl, inst.ServiceType)
		if err := k8s.UpsertServiceInstanceCR(context.Background(), app.Identifier, env.Identifier, inst.ServiceType, workloadRole, toolNamespaceRole, environmentRole, clusterRole, nil, helmSpec, resourceLabels, resourceAnnotations); err != nil {
			c.JSON(http.StatusFailedDependency, gin.H{"error": "ServiceInstance CR update failed: " + err.Error()})
			return
		}
	}
	inst.ServiceName = serviceToolIdentity(&svcTmpl, inst.ServiceType)
	inst.Namespace = helmSpec.Namespace
	inst.ReleaseName = helmSpec.ReleaseName
	inst.Values = serviceValuesJSON(helmSpec.Values)
	if strings.TrimSpace(inst.Status) == "" || strings.EqualFold(inst.Status, "pending") {
		inst.Status = "draft"
	}
	if strings.EqualFold(inst.Status, "draft") {
		inst.ErrorMessage = ""
	}
	if err := database.DB.Save(&inst).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": inst})
}

func serviceStatusShouldReconcile(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running", "installing":
		return true
	default:
		return false
	}
}

func applyStoredServiceRuntimeIdentity(inst *model.ServiceInstallation, helmSpec *paapv1.HelmInstallSpec) {
	if inst == nil || helmSpec == nil {
		return
	}
	if namespace := strings.TrimSpace(inst.Namespace); namespace != "" {
		helmSpec.Namespace = namespace
		rewriteServiceTemplateToolNamespaceValues(helmSpec, namespace)
	}
	if releaseName := strings.TrimSpace(inst.ReleaseName); releaseName != "" {
		helmSpec.ReleaseName = releaseName
		if helmSpec.Values != nil {
			if _, ok := helmSpec.Values["fullnameOverride"]; ok {
				helmSpec.Values["fullnameOverride"] = releaseName
			}
		}
	}
}

// SetServiceExternalAccess toggles external access for the live Kubernetes
// Service behind an installed service. This is intentionally runtime state,
// not Helm values.
func SetServiceExternalAccess(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var req ServiceExternalAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var inst model.ServiceInstallation
	if err := database.DB.Where("id = ? AND environment_id = ?", serviceID, envID).First(&inst).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "service installation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(inst.Namespace) == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "service namespace is not ready"})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	ctx := context.Background()
	if _, err := k8s.SetNamespaceServiceExternalAccess(ctx, inst.Namespace, inst.ServiceType, req.Enabled); err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}

	access := collectEnvironmentExternalAccess(ctx, env)
	view := enrichServiceInstallationViews(ctx, []model.ServiceInstallation{inst}, access)[0]
	c.JSON(http.StatusOK, gin.H{"data": view, "externalAccess": access})
}

func SetComponentExternalAccess(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	compID, _ := strconv.Atoi(c.Param("componentId"))

	var req ServiceExternalAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if !requireApplicationAccess(c, env.ApplicationID) {
		return
	}
	if strings.TrimSpace(env.Namespace) == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "environment namespace is not ready"})
		return
	}

	var comp model.Component
	if err := database.DB.Where("id = ? AND environment_id = ?", compID, envID).First(&comp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	if err := k8s.SetComponentExternalAccess(ctx, env.Namespace, identifier, req.Enabled); err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}

	access := collectEnvironmentExternalAccess(ctx, env)
	view := enrichComponentViews(ctx, env, []model.Component{comp}, access)[0]
	c.JSON(http.StatusOK, gin.H{"data": view, "externalAccess": access})
}

func SetComponentNodePortAccess(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	compID, _ := strconv.Atoi(c.Param("componentId"))

	var req ServiceExternalAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var comp model.Component
	if err := database.DB.Where("id = ? AND environment_id = ?", compID, envID).First(&comp).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "component not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}
	if strings.TrimSpace(env.Namespace) == "" {
		c.JSON(http.StatusConflict, gin.H{"error": "environment namespace is not ready"})
		return
	}

	ctx := context.Background()
	identifier := service.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	if err := k8s.SetComponentNodePortAccess(ctx, env.Namespace, identifier, req.Enabled); err != nil {
		c.JSON(http.StatusFailedDependency, gin.H{"error": err.Error()})
		return
	}

	access := collectEnvironmentExternalAccess(ctx, env)
	view := enrichComponentViews(ctx, env, []model.Component{comp}, access)[0]
	c.JSON(http.StatusOK, gin.H{"data": view, "externalAccess": access})
}

// InstallInfra installs infrastructure in an environment
func InstallInfra(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		InfraType string            `json:"infraType" binding:"required"`
		Config    map[string]string `json:"config"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	inst := model.InfraInstallation{
		EnvironmentID: env.ID,
		InfraType:     req.InfraType,
		Status:        "installing",
	}
	database.DB.Create(&inst)

	go func() {
		var err error
		switch req.InfraType {
		case "postgresql":
			err = k8sClient.InstallPostgreSQL(env.Namespace, env.Name+"-db")
		case "redis":
			err = k8sClient.InstallRedis(env.Namespace, env.Name+"-redis")
		case "rabbitmq":
			err = k8sClient.InstallRabbitMQ(env.Namespace, env.Name+"-mq")
		case "minio":
			err = k8sClient.InstallMinIO(env.Namespace, env.Name+"-minio")
		case "nacos":
			err = k8sClient.InstallNacos(env.Namespace, env.Name+"-nacos")
		default:
			err = fmt.Errorf("unknown infra type: %s", req.InfraType)
		}

		if err != nil {
			inst.Status = "failed"
			inst.ErrorMessage = err.Error()
		} else {
			inst.Status = "running"
		}
		database.DB.Save(&inst)
	}()

	c.JSON(http.StatusCreated, gin.H{"data": inst})
}

// UninstallService removes a service installation from an environment
func UninstallService(c *gin.Context) {
	envID, _ := strconv.Atoi(c.Param("id"))
	serviceID, _ := strconv.Atoi(c.Param("serviceId"))

	var inst model.ServiceInstallation
	if err := database.DB.First(&inst, serviceID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "service installation not found"})
		return
	}

	var env model.Environment
	if err := database.DB.First(&env, envID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "environment not found"})
		return
	}

	var app model.Application
	if err := database.DB.First(&app, env.ApplicationID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}

	ctx := context.Background()

	// Delete ServiceInstance CR and namespace. Tool resources, including Helm release
	// secrets, are scoped to the tool namespace and are removed with it.
	if err := k8s.DeleteServiceInstanceCR(ctx, app.Identifier, env.Identifier, inst.ServiceType); err != nil {
		log.Printf("[UninstallService] CR delete warning: %v", err)
	}

	// Delete the namespace
	if err := k8sClient.DeleteNamespace(inst.Namespace); err != nil {
		log.Printf("[UninstallService] namespace delete warning: %v", err)
	}

	// Delete from database
	database.DB.Delete(&inst)

	c.JSON(http.StatusOK, gin.H{"message": "service uninstalled successfully"})
}
