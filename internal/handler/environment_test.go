package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	paapv1 "paap/api/v1"
	"paap/internal/database"
	"paap/internal/k8s"
	"paap/internal/model"
	svcservice "paap/internal/service"
)

func handlerInt32Ptr(value int32) *int32 { return &value }

func handlerStringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func handlerAssertAnnotation(t *testing.T, resource svcservice.ToolWorkspaceResource, key string, want interface{}) {
	t.Helper()
	got, ok := resource.Annotations[key]
	if !ok {
		t.Fatalf("missing annotation %s in %#v", key, resource.Annotations)
	}
	if got != want {
		t.Fatalf("annotation %s = %#v, want %#v", key, got, want)
	}
}

type fakeHarborProjectEnsurer struct {
	projects *[]string
}

func (f fakeHarborProjectEnsurer) EnsureProject(_ context.Context, name string) error {
	*f.projects = append(*f.projects, name)
	return nil
}

type fakeGiteaWorkspaceClient struct{}

func (fakeGiteaWorkspaceClient) Repositories(_ context.Context) ([]k8s.GiteaRepository, error) {
	return []k8s.GiteaRepository{{
		Name:          "billing-dev-components",
		HTMLURL:       "http://gitea/paap/billing-dev-components",
		CloneURL:      "ssh://git@gitea/paap/billing-dev-components.git",
		DefaultBranch: "main",
		Private:       true,
		Language:      "Go",
	}}, nil
}

func (fakeGiteaWorkspaceClient) RepositoryContents(_ context.Context, repo, path, ref string) ([]k8s.GiteaContent, error) {
	switch path {
	case "":
		return []k8s.GiteaContent{
			{Name: "README.md", Path: "README.md", Type: "file", Size: 18},
			{Name: "components", Path: "components", Type: "dir"},
		}, nil
	case "components":
		return []k8s.GiteaContent{
			{Name: "api", Path: "components/api", Type: "dir"},
		}, nil
	case "components/api":
		return []k8s.GiteaContent{
			{Name: "Jenkinsfile", Path: "components/api/Jenkinsfile", Type: "file", Size: 42},
		}, nil
	case "README.md":
		return []k8s.GiteaContent{{
			Name:     "README.md",
			Path:     "README.md",
			Type:     "file",
			Size:     18,
			Encoding: "base64",
			Content:  "IyBCaWxsaW5nCgpEZXBsb3kK",
		}}, nil
	case "components/api/Jenkinsfile":
		return []k8s.GiteaContent{{
			Name:     "Jenkinsfile",
			Path:     "components/api/Jenkinsfile",
			Type:     "file",
			Size:     42,
			Encoding: "base64",
			Content:  "cGlwZWxpbmUgeyAvKiBidWlsZCAqLyB9Cg==",
		}}, nil
	default:
		return nil, nil
	}
}

func (fakeGiteaWorkspaceClient) RepositoryCommits(_ context.Context, repo, branch string, limit int) ([]k8s.GiteaCommit, error) {
	commit := k8s.GiteaCommit{SHA: "abcdef123456"}
	commit.Commit.Message = "sync deployment"
	commit.Commit.Author.Name = "PAAP"
	commit.Commit.Author.Date = "2026-06-04T10:00:00Z"
	return []k8s.GiteaCommit{commit}, nil
}

type fakeJenkinsWorkspaceClient struct {
	jobs       []k8s.JenkinsJob
	consoleLog map[string]string
}

func (f fakeJenkinsWorkspaceClient) Jobs(context.Context) ([]k8s.JenkinsJob, error) {
	return f.jobs, nil
}

func (f fakeJenkinsWorkspaceClient) ConsoleText(_ context.Context, jobName string) (string, error) {
	return f.consoleLog[jobName], nil
}

type countingGiteaWorkspaceClient struct {
	repositoriesCalls int32
	contentsCalls     int32
}

func (c *countingGiteaWorkspaceClient) Repositories(ctx context.Context) ([]k8s.GiteaRepository, error) {
	atomic.AddInt32(&c.repositoriesCalls, 1)
	return fakeGiteaWorkspaceClient{}.Repositories(ctx)
}

func (c *countingGiteaWorkspaceClient) RepositoryContents(ctx context.Context, repo, path, ref string) ([]k8s.GiteaContent, error) {
	atomic.AddInt32(&c.contentsCalls, 1)
	return fakeGiteaWorkspaceClient{}.RepositoryContents(ctx, repo, path, ref)
}

func (c *countingGiteaWorkspaceClient) RepositoryCommits(ctx context.Context, repo, branch string, limit int) ([]k8s.GiteaCommit, error) {
	return fakeGiteaWorkspaceClient{}.RepositoryCommits(ctx, repo, branch, limit)
}

type slowGiteaWorkspaceClient struct {
	active    int32
	maxActive int32
}

func (s *slowGiteaWorkspaceClient) Repositories(_ context.Context) ([]k8s.GiteaRepository, error) {
	return []k8s.GiteaRepository{
		{Name: "repo-a", HTMLURL: "http://gitea/paap/repo-a", CloneURL: "http://gitea/paap/repo-a.git", DefaultBranch: "main"},
		{Name: "repo-b", HTMLURL: "http://gitea/paap/repo-b", CloneURL: "http://gitea/paap/repo-b.git", DefaultBranch: "main"},
	}, nil
}

func (s *slowGiteaWorkspaceClient) RepositoryContents(_ context.Context, repo, path, ref string) ([]k8s.GiteaContent, error) {
	s.enter()
	defer s.leave()
	time.Sleep(80 * time.Millisecond)
	if path == "" {
		return []k8s.GiteaContent{{Name: repo + ".txt", Path: repo + ".txt", Type: "file", Size: 1, Encoding: "base64", Content: "Cg=="}}, nil
	}
	return nil, nil
}

func (s *slowGiteaWorkspaceClient) RepositoryCommits(_ context.Context, repo, branch string, limit int) ([]k8s.GiteaCommit, error) {
	s.enter()
	defer s.leave()
	time.Sleep(80 * time.Millisecond)
	commit := k8s.GiteaCommit{SHA: repo + "-sha"}
	commit.Commit.Message = "commit " + repo
	return []k8s.GiteaCommit{commit}, nil
}

func (s *slowGiteaWorkspaceClient) enter() {
	current := atomic.AddInt32(&s.active, 1)
	for {
		max := atomic.LoadInt32(&s.maxActive)
		if current <= max || atomic.CompareAndSwapInt32(&s.maxActive, max, current) {
			return
		}
	}
}

func (s *slowGiteaWorkspaceClient) leave() {
	atomic.AddInt32(&s.active, -1)
}

func TestBuildHelmInstallSpecMapsEnvironmentNamespaces(t *testing.T) {
	manifest := model.PlatformManifest{
		Name:    "loki",
		Version: "v2.9.4",
		Permissions: model.PermissionsSpec{
			EnvironmentNamespaces: model.NamespacePermissionsSpec{
				Rules: []model.PolicyRuleSpec{
					{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch"}},
				},
			},
		},
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "env_namespaces", HelmVar: "paap.envNamespaces"},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	app := model.Application{Identifier: "test"}
	env := model.Environment{Identifier: "staging"}
	tmpl := model.ServiceTemplate{
		Type:                 "log",
		ChartArchivePath:     "/charts/loki.tar.gz",
		PlatformManifestJSON: string(manifestJSON),
	}

	spec := buildHelmInstallSpec(&app, &env, &tmpl, "log")
	if got := spec.Values["paap.envNamespaces"]; got != "test-staging,test-staging-app" {
		t.Fatalf("paap.envNamespaces = %#v, want %q", got, "test-staging,test-staging-app")
	}
}

func TestBuildHelmInstallSpecMapsMonitorNamespacesToPlatformValues(t *testing.T) {
	manifest := model.PlatformManifest{
		Name:    "custom-monitor",
		Version: "v1",
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "env_namespaces", HelmVar: "collector.namespaces"},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	app := model.Application{Identifier: "test"}
	env := model.Environment{Identifier: "staging"}
	tmpl := model.ServiceTemplate{
		Type:                 "monitor",
		ChartArchivePath:     "/charts/monitor.tar.gz",
		PlatformManifestJSON: string(manifestJSON),
	}

	spec := buildHelmInstallSpec(&app, &env, &tmpl, "monitor")
	if got := spec.Values["collector.namespaces"]; got != "test-staging,test-staging-app" {
		t.Fatalf("collector.namespaces = %#v, want %q", got, "test-staging,test-staging-app")
	}
}

func TestBuildHelmInstallSpecAppliesUserValuesAfterTemplateAndPlatformValues(t *testing.T) {
	manifest := model.PlatformManifest{
		Name:    "redis",
		Version: "v1",
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "tool_namespace", HelmVar: "fullnameOverride"},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "dev"}
	tmpl := model.ServiceTemplate{
		Type:                 "redis",
		Category:             "infra",
		ChartArchivePath:     "/charts/redis.tar.gz",
		DefaultValues:        `{"architecture":"standalone","service.type":"ClusterIP","master.persistence.size":"8Gi"}`,
		PlatformManifestJSON: string(manifestJSON),
	}

	spec := buildHelmInstallSpec(&app, &env, &tmpl, "redis", map[string]string{
		"architecture":                   "replication",
		"master.persistence.size":        "20Gi",
		"service.type":                   "NodePort",
		"master.service.nodePorts.redis": "30379",
	})

	if got := spec.Values["fullnameOverride"]; got != "billing-dev-redis" {
		t.Fatalf("platform value fullnameOverride = %#v, want billing-dev-redis", got)
	}
	if got := spec.Values["architecture"]; got != "replication" {
		t.Fatalf("user architecture override = %#v, want replication", got)
	}
	if got := spec.Values["master.persistence.size"]; got != "20Gi" {
		t.Fatalf("user storage override = %#v, want 20Gi", got)
	}
	if got := spec.Values["service.type"]; got != "ClusterIP" {
		t.Fatalf("runtime network values must not be user Helm overrides, got %#v", got)
	}
	if _, ok := spec.Values["master.service.nodePorts.redis"]; ok {
		t.Fatalf("nodePort value leaked into Helm values: %#v", spec.Values["master.service.nodePorts.redis"])
	}
}

func TestBuildHelmInstallSpecUsesAdvancedDatabaseCharts(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "dev"}
	mysqlTemplate := model.ServiceTemplate{
		Type:          "mysql",
		Category:      "infra",
		S3Bucket:      "paap-charts",
		S3Key:         "charts/mysql.tar.gz",
		DefaultValues: `{"architecture":"standalone","primary.persistence.size":"8Gi"}`,
	}
	postgresTemplate := model.ServiceTemplate{
		Type:          "postgresql",
		Category:      "infra",
		S3Bucket:      "paap-charts",
		S3Key:         "charts/postgresql.tar.gz",
		DefaultValues: `{"architecture":"standalone","primary.persistence.size":"8Gi"}`,
	}

	mysql := buildHelmInstallSpec(&app, &env, &mysqlTemplate, "mysql", map[string]string{
		"architecture":                  "dual-master",
		"replicaCount":                  "2",
		"persistence.enabled":           "true",
		"persistence.size":              "30Gi",
		"secondary.persistence.enabled": "true",
		"secondary.persistence.size":    "20Gi",
		"primary.persistence.size":      "20Gi",
		"primary.service.type":          "NodePort",
	})
	postgres := buildHelmInstallSpec(&app, &env, &postgresTemplate, "postgresql", map[string]string{
		"architecture":                     "ha-cluster",
		"readReplicas.replicaCount":        "3",
		"readReplicas.persistence.enabled": "true",
		"readReplicas.persistence.size":    "20Gi",
		"primary.persistence.size":         "20Gi",
		"postgresql.replicaCount":          "3",
		"pgpool.replicaCount":              "2",
		"persistence.enabled":              "true",
		"persistence.size":                 "40Gi",
	})

	if mysql.S3Key != "charts/mysql-galera.tar.gz" {
		t.Fatalf("mysql advanced chart = %q, want charts/mysql-galera.tar.gz", mysql.S3Key)
	}
	if postgres.S3Key != "charts/postgresql-ha.tar.gz" {
		t.Fatalf("postgres advanced chart = %q, want charts/postgresql-ha.tar.gz", postgres.S3Key)
	}
	if got := mysql.Values["paap.architecture"]; got != "dual-master" {
		t.Fatalf("mysql paap.architecture = %#v, want dual-master", got)
	}
	if got := postgres.Values["paap.architecture"]; got != "ha-cluster" {
		t.Fatalf("postgres paap.architecture = %#v, want ha-cluster", got)
	}
	if got := mysql.Values["replicaCount"]; got != "2" {
		t.Fatalf("mysql replicaCount = %#v, want 2", got)
	}
	if got := mysql.Values["persistence.size"]; got != "30Gi" {
		t.Fatalf("mysql galera storage = %#v, want 30Gi", got)
	}
	if got := postgres.Values["postgresql.replicaCount"]; got != "3" {
		t.Fatalf("postgresql replicaCount = %#v, want 3", got)
	}
	if got := postgres.Values["pgpool.replicaCount"]; got != "2" {
		t.Fatalf("pgpool replicaCount = %#v, want 2", got)
	}
	if got := postgres.Values["persistence.size"]; got != "40Gi" {
		t.Fatalf("postgres ha storage = %#v, want 40Gi", got)
	}
	for key := range mysql.Values {
		if key == "architecture" || strings.HasPrefix(key, "secondary.") || strings.HasPrefix(key, "primary.") {
			t.Fatalf("mysql base-chart value %q leaked into Galera values: %#v", key, mysql.Values[key])
		}
	}
	for key := range postgres.Values {
		if key == "architecture" || strings.HasPrefix(key, "readReplicas.") || strings.HasPrefix(key, "primary.") {
			t.Fatalf("postgres base-chart value %q leaked into HA values: %#v", key, postgres.Values[key])
		}
	}
}

func TestBuildHelmInstallSpecUsesProductEvidenceForValueAllowlist(t *testing.T) {
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "dev"}
	harborTemplate := model.ServiceTemplate{
		Type:          "registry",
		Name:          "Harbor (官方)",
		Category:      "tool",
		S3Bucket:      "paap-charts",
		S3Key:         "charts/harbor.tar.gz",
		DefaultValues: `{"persistence.enabled":"false"}`,
	}
	registryTemplate := model.ServiceTemplate{
		Type:          "registry",
		Name:          "Docker Registry v2",
		Category:      "tool",
		S3Bucket:      "paap-charts",
		S3Key:         "charts/registry.tar.gz",
		DefaultValues: `{"persistence.enabled":"false","deleteEnabled":"false"}`,
	}

	harbor := buildHelmInstallSpec(&app, &env, &harborTemplate, "registry", map[string]string{
		"persistence.enabled":                             "true",
		"persistence.persistentVolumeClaim.registry.size": "50Gi",
		"deleteEnabled":                                   "true",
	})
	registry := buildHelmInstallSpec(&app, &env, &registryTemplate, "registry", map[string]string{
		"persistence.enabled": "true",
		"persistence.size":    "10Gi",
		"deleteEnabled":       "true",
		"core.replicas":       "2",
	})

	if got := harbor.Values["persistence.persistentVolumeClaim.registry.size"]; got != "50Gi" {
		t.Fatalf("harbor registry pvc size = %#v, want 50Gi", got)
	}
	if _, ok := harbor.Values["deleteEnabled"]; ok {
		t.Fatalf("docker-registry-only value leaked into Harbor values: %#v", harbor.Values["deleteEnabled"])
	}
	if got := registry.Values["deleteEnabled"]; got != "true" {
		t.Fatalf("registry deleteEnabled = %#v, want true", got)
	}
	if _, ok := registry.Values["core.replicas"]; ok {
		t.Fatalf("harbor-only value leaked into Docker Registry values: %#v", registry.Values["core.replicas"])
	}
}

func TestBuildHelmInstallSpecUsesRedisClusterChartForClusterMode(t *testing.T) {
	manifest := model.PlatformManifest{
		Name:    "redis",
		Version: "v1",
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "tool_namespace", HelmVar: "fullnameOverride"},
			{PlatformVar: "tool_namespace", HelmVar: "serviceAccount.name"},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "dev"}
	tmpl := model.ServiceTemplate{
		Type:                 "redis",
		Category:             "infra",
		S3Bucket:             "paap-charts",
		S3Key:                "charts/redis.tar.gz",
		PlatformManifestJSON: string(manifestJSON),
	}

	spec := buildHelmInstallSpec(&app, &env, &tmpl, "redis", map[string]string{
		"architecture":     "cluster",
		"cluster.nodes":    "6",
		"cluster.replicas": "1",
	})

	if spec.ReleaseName != "billing-dev-redis" || spec.Namespace != "billing-dev-redis" {
		t.Fatalf("cluster mode must keep redis identity, got release=%q namespace=%q", spec.ReleaseName, spec.Namespace)
	}
	if spec.S3Key != "charts/redis-cluster.tar.gz" {
		t.Fatalf("cluster mode chart = %q, want charts/redis-cluster.tar.gz", spec.S3Key)
	}
	if got := spec.Values["fullnameOverride"]; got != "billing-dev-redis" {
		t.Fatalf("fullnameOverride = %#v, want billing-dev-redis", got)
	}
	if got := spec.Values["serviceAccount.name"]; got != "billing-dev-redis" {
		t.Fatalf("serviceAccount.name = %#v, want billing-dev-redis", got)
	}
	if got := spec.Values["architecture"]; got != "cluster" {
		t.Fatalf("architecture = %#v, want cluster", got)
	}
}

func TestBuildHelmInstallSpecKeepsTemplatePlatformManifest(t *testing.T) {
	manifest := model.PlatformManifest{
		Name:    "loki",
		Version: "v2.9.4",
		Permissions: model.PermissionsSpec{
			EnvironmentNamespaces: model.NamespacePermissionsSpec{
				Rules: []model.PolicyRuleSpec{
					{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch"}},
				},
			},
		},
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "env_namespaces", HelmVar: "promtail.config.paap.envNamespaces"},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	app := model.Application{Identifier: "test"}
	env := model.Environment{Identifier: "staging"}
	tmpl := model.ServiceTemplate{
		Type:                 "log",
		S3Bucket:             "paap-charts",
		S3Key:                "charts/loki.tar.gz",
		PlatformManifestJSON: string(manifestJSON),
	}

	spec := buildHelmInstallSpec(&app, &env, &tmpl, "log")
	if spec.PlatformManifest != string(manifestJSON) {
		t.Fatalf("platform manifest changed, got %q want %q", spec.PlatformManifest, string(manifestJSON))
	}
}

func TestGetWorkloadRoleDefaultsToNoProjectedPermissions(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	if got := getWorkloadRole("missing"); len(got.Rules) != 0 {
		t.Fatalf("missing template should not get projected workload permissions: %#v", got.Rules)
	}

	if err := db.Create(&model.ServiceTemplate{Type: "redis", WorkloadRolePolicy: ""}).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	if got := getWorkloadRole("redis"); len(got.Rules) != 0 {
		t.Fatalf("empty workload policy should not get projected workload permissions: %#v", got.Rules)
	}
}

func TestGetEnvironmentRoleReadsCustomTemplatePolicy(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	policy := `[{"apiGroups":["*"],"resources":["*"],"verbs":["*"]}]`
	if err := db.Create(&model.ServiceTemplate{
		Type:                  "custom-monitor",
		WorkloadRolePolicy:    "[]",
		EnvironmentRolePolicy: policy,
	}).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}

	if got := getWorkloadRole("custom-monitor"); len(got.Rules) != 0 {
		t.Fatalf("custom monitor should not get workload permissions: %#v", got.Rules)
	}
	got := getEnvironmentRole("custom-monitor")
	if got == nil || len(got.Rules) != 1 {
		t.Fatalf("custom monitor should get environment permissions, got %#v", got)
	}
	if got.Rules[0].APIGroups[0] != "*" || got.Rules[0].Resources[0] != "*" || got.Rules[0].Verbs[0] != "*" {
		t.Fatalf("unexpected environment policy: %#v", got.Rules[0])
	}
}

func TestServiceToolNamespaceUsesTemplateToolIdentity(t *testing.T) {
	app := model.Application{Identifier: "myapp"}
	env := model.Environment{Identifier: "prod"}

	argocdManifest, _ := json.Marshal(model.PlatformManifest{Name: "argocd", Version: "v1"})
	argocdTemplate := model.ServiceTemplate{Type: "deploy", PlatformManifestJSON: string(argocdManifest)}
	if got := serviceToolNamespace(app, env, &argocdTemplate, "deploy"); got != "myapp-prod-argocd" {
		t.Fatalf("expected argocd deploy template namespace, got %q", got)
	}

	fluxManifest, _ := json.Marshal(model.PlatformManifest{Name: "flux", Version: "v1"})
	fluxTemplate := model.ServiceTemplate{Type: "deploy", PlatformManifestJSON: string(fluxManifest)}
	if got := serviceToolNamespace(app, env, &fluxTemplate, "deploy"); got != "myapp-prod-flux" {
		t.Fatalf("expected flux deploy template namespace, got %q", got)
	}
}

func TestServiceResourceMetadataClassifiesTemplate(t *testing.T) {
	manifest, _ := json.Marshal(model.PlatformManifest{Name: "argocd", Version: "v1"})
	tmpl := model.ServiceTemplate{Type: "deploy", Category: "tool", PlatformManifestJSON: string(manifest)}

	labels := serviceResourceLabels("billing", "dev", &tmpl, "deploy")
	if labels["paap.io/service-type"] != "deploy" || labels["paap.io/tool"] != "argocd" || labels["paap.io/category"] != "tool" {
		t.Fatalf("service labels must expose service type, implementation, and category: %#v", labels)
	}
}

func TestSeedEnvTemplatesRefreshesBuiltInFoundation(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.EnvTemplate{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	if err := db.Create(&model.EnvTemplate{
		Name:     "生产环境标准",
		Services: toJSON([]string{"deploy", "monitor", "log"}),
		Infra:    toJSON([]string{"postgresql"}),
	}).Error; err != nil {
		t.Fatalf("create old built-in template: %v", err)
	}
	if err := db.Create(&model.EnvTemplate{
		Name:     "自定义环境",
		Services: toJSON([]string{"deploy"}),
	}).Error; err != nil {
		t.Fatalf("create custom template: %v", err)
	}

	SeedEnvTemplates()

	var prod model.EnvTemplate
	if err := db.Where("name = ?", "生产环境标准").First(&prod).Error; err != nil {
		t.Fatalf("load production template: %v", err)
	}
	var services []string
	if err := json.Unmarshal([]byte(prod.Services), &services); err != nil {
		t.Fatalf("decode services: %v", err)
	}
	if strings.Join(services, ",") != strings.Join(foundationServiceTypes(), ",") {
		t.Fatalf("production services = %#v, want %#v", services, foundationServiceTypes())
	}

	var custom model.EnvTemplate
	if err := db.Where("name = ?", "自定义环境").First(&custom).Error; err != nil {
		t.Fatalf("load custom template: %v", err)
	}
	if custom.Services != toJSON([]string{"deploy"}) {
		t.Fatalf("custom template should not be rewritten, got %s", custom.Services)
	}
}

func TestInstallTemplateServicesInstallsServicesAndInfra(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.EnvTemplate{}, &model.ServiceTemplate{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).Build())

	app := model.Application{Name: "Test", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Staging", Identifier: "staging", Namespace: "test-staging", Status: "creating"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	tmpl := model.EnvTemplate{
		Name:     "Full stack",
		Services: toJSON([]string{"deploy"}),
		Infra:    toJSON([]string{"postgresql", "redis"}),
	}
	if err := db.Create(&tmpl).Error; err != nil {
		t.Fatalf("create env template: %v", err)
	}
	for _, item := range []model.ServiceTemplate{
		{Type: "git", Installer: "helm", Category: "tool", PlatformManifestJSON: `{"name":"gitea","version":"v1"}`},
		{Type: "registry", Installer: "helm", Category: "tool", PlatformManifestJSON: `{"name":"registry","version":"v1"}`},
		{Type: "deploy", Installer: "helm", Category: "tool", PlatformManifestJSON: `{"name":"argocd","version":"v1"}`},
		{Type: "monitor", Installer: "helm", Category: "tool", PlatformManifestJSON: `{"name":"kube-prometheus-stack","version":"v1"}`},
		{Type: "log", Installer: "helm", Category: "tool", PlatformManifestJSON: `{"name":"loki","version":"v1"}`},
		{Type: "postgresql", Installer: "helm", Category: "infra", PlatformManifestJSON: `{"name":"postgresql","version":"v1"}`},
		{Type: "redis", Installer: "helm", Category: "infra", PlatformManifestJSON: `{"name":"redis","version":"v1"}`},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create service template %s: %v", item.Type, err)
		}
	}

	installTemplateServices(&env, &app, env.Identifier, tmpl.ID)

	var installs []model.ServiceInstallation
	if err := db.Order("service_type").Find(&installs).Error; err != nil {
		t.Fatalf("list installs: %v", err)
	}
	gotTypes := make([]string, 0, len(installs))
	for _, inst := range installs {
		gotTypes = append(gotTypes, inst.ServiceType)
	}
	if strings.Join(gotTypes, ",") != "deploy,git,log,monitor,postgresql,redis,registry" {
		t.Fatalf("installed service types = %#v", gotTypes)
	}

	wantCRNames := map[string]string{
		"deploy":     "staging-argocd",
		"git":        "staging-gitea",
		"log":        "staging-loki",
		"monitor":    "staging-kube-prometheus-stack",
		"postgresql": "staging-postgresql",
		"redis":      "staging-redis",
		"registry":   "staging-registry",
	}
	for _, serviceType := range []string{"deploy", "git", "log", "monitor", "postgresql", "redis", "registry"} {
		var svc paapv1.ServiceInstance
		if err := k8s.GetClient().Get(t.Context(), ctrlclient.ObjectKey{Name: wantCRNames[serviceType], Namespace: "paap-app-test"}, &svc); err != nil {
			t.Fatalf("service instance %s not created: %v", serviceType, err)
		}
		if svc.Labels["paap.io/service-type"] != serviceType {
			t.Fatalf("service instance %s labels = %#v", serviceType, svc.Labels)
		}
	}
}

func TestCreateServiceDraftDoesNotCreateServiceInstanceCR(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceTemplate{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "billing-dev", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	tmpl := model.ServiceTemplate{Type: "rabbitmq", Name: "RabbitMQ", Installer: "helm", Category: "infra", PlatformManifestJSON: `{"name":"rabbitmq","version":"v1"}`}
	if err := db.Create(&tmpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/services/drafts", CreateServiceDraft)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/services/drafts", strings.NewReader(`{"serviceType":"rabbitmq"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var inst model.ServiceInstallation
	if err := db.Where("environment_id = ? AND service_type = ?", env.ID, "rabbitmq").First(&inst).Error; err != nil {
		t.Fatalf("service draft not saved: %v", err)
	}
	if inst.Status != "draft" {
		t.Fatalf("service status = %q, want draft", inst.Status)
	}
	if inst.Namespace != "billing-dev-rabbitmq" || inst.ReleaseName != "billing-dev-rabbitmq" {
		t.Fatalf("unexpected draft namespace/release: %#v", inst)
	}

	var svc paapv1.ServiceInstance
	err = k8s.GetClient().Get(t.Context(), ctrlclient.ObjectKey{Name: "dev-rabbitmq", Namespace: "paap-app-billing"}, &svc)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("draft creation must not create ServiceInstance CR, got err=%v svc=%#v", err, svc)
	}
}

func TestInstallServiceDeploysExistingDraft(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceTemplate{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "billing-dev", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	tmpl := model.ServiceTemplate{Type: "redis", Name: "Redis", Installer: "helm", Category: "infra", PlatformManifestJSON: `{"name":"redis","version":"v1"}`}
	if err := db.Create(&tmpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "redis",
		ServiceName:   "redis",
		Status:        "draft",
		Namespace:     "billing-dev-redis",
		ReleaseName:   "billing-dev-redis",
		Values:        `{"architecture":"standalone"}`,
	}).Error; err != nil {
		t.Fatalf("create draft: %v", err)
	}

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/services", InstallService)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/services", strings.NewReader(`{"serviceType":"redis","values":{"architecture":"standalone"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var inst model.ServiceInstallation
	if err := db.Where("environment_id = ? AND service_type = ?", env.ID, "redis").First(&inst).Error; err != nil {
		t.Fatalf("service installation not saved: %v", err)
	}
	if inst.Status != "installing" {
		t.Fatalf("service status = %q, want installing", inst.Status)
	}

	var svc paapv1.ServiceInstance
	if err := k8s.GetClient().Get(t.Context(), ctrlclient.ObjectKey{Name: "dev-redis", Namespace: "paap-app-billing"}, &svc); err != nil {
		t.Fatalf("deploying a draft should create ServiceInstance CR: %v", err)
	}
	if svc.Spec.Type != "redis" {
		t.Fatalf("service instance type = %q, want redis", svc.Spec.Type)
	}
}

func TestUpdateServiceDraftSavesValuesWithoutCreatingCR(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceTemplate{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "billing-dev", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	tmpl := model.ServiceTemplate{Type: "redis", Name: "Redis", Installer: "helm", Category: "infra", PlatformManifestJSON: `{"name":"redis","version":"v1"}`}
	if err := db.Create(&tmpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	inst := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "redis",
		ServiceName:   "redis",
		Status:        "draft",
		Namespace:     "billing-dev-redis",
		ReleaseName:   "billing-dev-redis",
		Values:        `{"architecture":"standalone"}`,
	}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create draft: %v", err)
	}

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/services/:serviceId", UpdateService)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/environments/1/services/%d", inst.ID), strings.NewReader(`{"values":{"architecture":"replication","replica.replicaCount":"2"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var saved model.ServiceInstallation
	if err := db.First(&saved, inst.ID).Error; err != nil {
		t.Fatalf("load saved service: %v", err)
	}
	if saved.Status != "draft" {
		t.Fatalf("service status = %q, want draft", saved.Status)
	}
	if !strings.Contains(saved.Values, `"architecture":"replication"`) || !strings.Contains(saved.Values, `"replica.replicaCount":"2"`) {
		t.Fatalf("service values not saved: %s", saved.Values)
	}

	var svc paapv1.ServiceInstance
	err = k8s.GetClient().Get(t.Context(), ctrlclient.ObjectKey{Name: "dev-redis", Namespace: "paap-app-billing"}, &svc)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("saving draft config must not create ServiceInstance CR, got err=%v svc=%#v", err, svc)
	}
}

func TestUpdateRunningServiceReconcilesServiceInstanceCR(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sqlite db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceTemplate{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "billing-dev", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	manifest := model.PlatformManifest{
		Name:    "redis",
		Version: "v1",
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "tool_namespace", HelmVar: "fullnameOverride"},
			{PlatformVar: "tool_namespace", HelmVar: "serviceAccount.name"},
		},
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	tmpl := model.ServiceTemplate{
		Type:                 "redis",
		Name:                 "Redis",
		Installer:            "helm",
		Category:             "infra",
		S3Bucket:             "paap-charts",
		S3Key:                "charts/redis.tar.gz",
		PlatformManifestJSON: string(manifestJSON),
	}
	if err := db.Create(&tmpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	inst := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "redis",
		ServiceName:   "redis",
		Status:        "running",
		Namespace:     "billing-dev-redis",
		ReleaseName:   "billing-dev-redis",
		Values:        `{"architecture":"standalone"}`,
	}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create running service: %v", err)
	}

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := paapv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	existingCR := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "dev-redis", Namespace: "paap-app-billing"},
		Spec: paapv1.ServiceInstanceSpec{
			EnvironmentRef: paapv1.ObjectReference{Name: "dev"},
			Type:           "redis",
			ToolNamespace:  "billing-dev-redis",
			WorkloadRole:   paapv1.RoleSpec{},
			Helm: &paapv1.HelmInstallSpec{
				ReleaseName: "billing-dev-redis",
				Namespace:   "billing-dev-redis",
				S3Bucket:    "paap-charts",
				S3Key:       "charts/redis.tar.gz",
				Values:      map[string]string{"architecture": "standalone"},
			},
		},
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).WithObjects(existingCR).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/services/:serviceId", UpdateService)

	body := `{"values":{"architecture":"cluster","cluster.nodes":"6","cluster.replicas":"1"}}`
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/environments/1/services/%d", inst.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var saved model.ServiceInstallation
	if err := db.First(&saved, inst.ID).Error; err != nil {
		t.Fatalf("load saved service: %v", err)
	}
	if saved.Status != "running" {
		t.Fatalf("service status = %q, want running", saved.Status)
	}
	if !strings.Contains(saved.Values, `"architecture":"cluster"`) || !strings.Contains(saved.Values, `"cluster.nodes":"6"`) {
		t.Fatalf("service values not saved: %s", saved.Values)
	}

	var svc paapv1.ServiceInstance
	if err := k8s.GetClient().Get(t.Context(), ctrlclient.ObjectKey{Name: "dev-redis", Namespace: "paap-app-billing"}, &svc); err != nil {
		t.Fatalf("service instance CR not updated: %v", err)
	}
	if svc.Spec.Helm == nil {
		t.Fatalf("service instance Helm spec is nil")
	}
	if svc.Spec.Helm.S3Key != "charts/redis-cluster.tar.gz" {
		t.Fatalf("service instance chart = %q, want redis cluster chart", svc.Spec.Helm.S3Key)
	}
	if got := svc.Spec.Helm.Values["architecture"]; got != "cluster" {
		t.Fatalf("service instance architecture = %#v, want cluster", got)
	}
	if got := svc.Spec.Helm.Values["cluster.nodes"]; got != "6" {
		t.Fatalf("service instance cluster.nodes = %#v, want 6", got)
	}
}

func TestComponentDeleteIdentifierMatchesCreateIdentifier(t *testing.T) {
	comp := model.Component{ID: 42, Name: "订单服务", Type: "backend"}

	if got := componentDeleteIdentifier(model.Application{}, model.Environment{}, comp); got != "backend-42" {
		t.Fatalf("expected delete identifier to match create identifier, got %q", got)
	}
}

func TestImageReferenceHasTagDetectsOnlyLastPathSegment(t *testing.T) {
	cases := map[string]bool{
		"registry.local:5000/order":        false,
		"registry.local:5000/order:v1.0.0": true,
		"order:v1.0.0":                     true,
		"order":                            false,
	}

	for image, want := range cases {
		if got := imageReferenceHasTag(image); got != want {
			t.Fatalf("imageReferenceHasTag(%q) = %v, want %v", image, got, want)
		}
	}
}

func TestCreateEnvironmentGeneratesIdentifierWhenMissing(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}

	body, _ := json.Marshal(CreateEnvRequest{Name: "预发环境", FromEmpty: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/1/environments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/applications/:id/environments", withTestAuthUserRole(1, "admin", CreateEnvironment))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got struct {
		Data model.Environment `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.Identifier != "env" {
		t.Fatalf("identifier = %q, want env", got.Data.Identifier)
	}
	if got.Data.Namespace != "test-env" {
		t.Fatalf("namespace = %q, want test-env", got.Data.Namespace)
	}
}

func TestListApplicationEnvironmentsRejectsNonMembers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.Environment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := db.Create(&model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod"}).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications/1/environments", nil)
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/applications/:id/environments", withTestAuthUser(2, ListApplicationEnvironments))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestCreateEnvironmentRejectsNonMembers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.Environment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}

	body, _ := json.Marshal(CreateEnvRequest{Name: "生产环境", FromEmpty: true})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications/1/environments", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/applications/:id/environments", withTestAuthUser(2, CreateEnvironment))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	var count int64
	if err := db.Model(&model.Environment{}).Where("application_id = ?", app.ID).Count(&count).Error; err != nil {
		t.Fatalf("count envs: %v", err)
	}
	if count != 0 {
		t.Fatalf("env count = %d, want none created", count)
	}
}

func TestCreateApplicationGeneratesIdentifierForDuplicateNames(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	if err := db.Create(&model.Application{Name: "App", Identifier: "app", OwnerID: 1}).Error; err != nil {
		t.Fatalf("seed app: %v", err)
	}

	body, _ := json.Marshal(CreateAppRequest{Name: "App"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/applications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/applications", withTestAuthUser(1, CreateApplication))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got struct {
		Data model.Application `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Data.Identifier != "app-2" {
		t.Fatalf("identifier = %q, want app-2", got.Data.Identifier)
	}
}

func TestGetEnvironmentReturnsApplicationAndServiceExternalAccess(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	previousSync := syncClusterState
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
		syncClusterState = previousSync
	})
	syncClusterState = func(context.Context, *gorm.DB, ctrlclient.Client) error { return nil }

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}, &model.InfraInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Status: "running", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "git", Status: "running", Namespace: "billing-dev-git"}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).WithObjects(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "kind-control-plane"},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "172.18.0.2"},
			}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing-dev"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{{Name: "http", Port: 80, NodePort: 30080}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "internal-only", Namespace: "billing-dev"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Name: "http", Port: 8080}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "gitea-http", Namespace: "billing-dev-git"},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeNodePort,
				ClusterIP: "10.96.0.31",
				Ports:     []corev1.ServicePort{{Name: "http", Port: 3000, NodePort: 30091}},
			},
		},
	).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/environments/:id", withTestAuthUserRole(1, "admin", GetEnvironment))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Data struct {
			ExternalAccess []struct {
				Name        string `json:"name"`
				Kind        string `json:"kind"`
				URL         string `json:"url"`
				Namespace   string `json:"namespace"`
				Scope       string `json:"scope"`
				ServiceID   uint   `json:"serviceId"`
				ServiceType string `json:"serviceType"`
			} `json:"externalAccess"`
			Services []struct {
				ServiceType        string `json:"serviceType"`
				RuntimeServiceName string `json:"runtimeServiceName"`
				ClusterIP          string `json:"clusterIP"`
				LoadBalancerIP     string `json:"loadBalancerIP"`
			} `json:"services"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data.Services) != 1 {
		t.Fatalf("services length = %d, want 1: %#v", len(body.Data.Services), body.Data.Services)
	}
	if body.Data.Services[0].RuntimeServiceName != "gitea-http" {
		t.Fatalf("runtime service name = %q, want gitea-http", body.Data.Services[0].RuntimeServiceName)
	}
	if body.Data.Services[0].ClusterIP != "10.96.0.31" {
		t.Fatalf("cluster IP = %q, want 10.96.0.31", body.Data.Services[0].ClusterIP)
	}
	gotURLs := map[string]struct{}{}
	for _, endpoint := range body.Data.ExternalAccess {
		gotURLs[endpoint.URL] = struct{}{}
		if endpoint.Name == "internal-only" {
			t.Fatalf("ClusterIP service leaked as external endpoint: %#v", endpoint)
		}
	}
	for _, want := range []string{"http://172.18.0.2:30080", "http://172.18.0.2:30091"} {
		if _, ok := gotURLs[want]; !ok {
			t.Fatalf("missing external URL %s, got %#v", want, body.Data.ExternalAccess)
		}
	}
	var foundServiceEndpoint bool
	for _, endpoint := range body.Data.ExternalAccess {
		if endpoint.URL == "http://172.18.0.2:30080" && (endpoint.Scope != "environment" || endpoint.Namespace != "billing-dev") {
			t.Fatalf("environment endpoint metadata not set: %#v", endpoint)
		}
		if endpoint.URL == "http://172.18.0.2:30091" {
			foundServiceEndpoint = true
			if endpoint.Scope != "service" || endpoint.Namespace != "billing-dev-git" || endpoint.ServiceType != "git" {
				t.Fatalf("service endpoint metadata not set: %#v", endpoint)
			}
		}
	}
	if !foundServiceEndpoint {
		t.Fatalf("service endpoint not returned: %#v", body.Data.ExternalAccess)
	}
}

func TestGetEnvironmentRejectsNonMembers(t *testing.T) {
	previousDB := database.DB
	previousSync := syncClusterState
	t.Cleanup(func() {
		database.DB = previousDB
		syncClusterState = previousSync
	})
	syncClusterState = func(context.Context, *gorm.DB, ctrlclient.Client) error { return nil }

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.Environment{}, &model.Component{}, &model.ServiceInstallation{}, &model.InfraInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "hidden-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1", nil)
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/environments/:id", withTestAuthUser(2, GetEnvironment))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestSetServiceExternalAccessPatchesLiveServiceWithoutChangingHelmValues(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Status: "running", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{
		EnvironmentID: env.ID,
		ServiceType:   "redis",
		ServiceName:   "redis",
		Status:        "running",
		Namespace:     "billing-dev-redis",
		Values:        `{"architecture":"replication","master.persistence.enabled":"false"}`,
	}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create service installation: %v", err)
	}

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).WithObjects(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "billing-dev-redis-master",
				Namespace: "billing-dev-redis",
				Labels: map[string]string{
					"app.kubernetes.io/component": "master",
					"app.kubernetes.io/name":      "redis",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.96.0.10",
				Ports:     []corev1.ServicePort{{Name: "redis", Port: 6379}},
			},
		},
	).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/services/:serviceId/external-access", SetServiceExternalAccess)
	path := fmt.Sprintf("/api/v1/environments/%d/services/%d/external-access", env.ID, inst.ID)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(`{"enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	liveService := &corev1.Service{}
	if err := k8s.GetClient().Get(t.Context(), types.NamespacedName{Namespace: "billing-dev-redis", Name: "billing-dev-redis-master"}, liveService); err != nil {
		t.Fatalf("get redis service: %v", err)
	}
	if liveService.Spec.Type != corev1.ServiceTypeNodePort {
		t.Fatalf("expected live Service to become NodePort, got %s", liveService.Spec.Type)
	}
	var saved model.ServiceInstallation
	if err := db.First(&saved, inst.ID).Error; err != nil {
		t.Fatalf("reload installation: %v", err)
	}
	if saved.Values != inst.Values {
		t.Fatalf("external access must not mutate Helm values, got %s want %s", saved.Values, inst.Values)
	}

	liveService.Spec.Ports[0].NodePort = 30379
	if err := k8s.GetClient().Update(t.Context(), liveService); err != nil {
		t.Fatalf("seed nodePort: %v", err)
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, path, strings.NewReader(`{"enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if err := k8s.GetClient().Get(t.Context(), types.NamespacedName{Namespace: "billing-dev-redis", Name: "billing-dev-redis-master"}, liveService); err != nil {
		t.Fatalf("get redis service after disable: %v", err)
	}
	if liveService.Spec.Type != corev1.ServiceTypeClusterIP || liveService.Spec.Ports[0].NodePort != 0 {
		t.Fatalf("expected ClusterIP with cleared NodePort, got type=%s nodePort=%d", liveService.Spec.Type, liveService.Spec.Ports[0].NodePort)
	}
}

func TestGetServiceCredentialsReadsRealSecrets(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	previousSync := syncClusterState
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
		syncClusterState = previousSync
	})
	syncClusterState = func(context.Context, *gorm.DB, ctrlclient.Client) error { return nil }

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: 1, ServiceType: "postgresql", Status: "running", Namespace: "billing-dev-postgresql"}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).WithObjects(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "postgresql", Namespace: "billing-dev-postgresql"},
			Type:       corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"username":          []byte("appuser"),
				"postgres-password": []byte("secret-password"),
				"token":             []byte("not-a-credential"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "tls", Namespace: "billing-dev-postgresql"},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{"tls.key": []byte("private-key")},
		},
	).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/environments/:id/services/:serviceId/credentials", GetServiceCredentials)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/services/1/credentials", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Data struct {
			Credentials []ServiceCredential `json:"credentials"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	got := map[string]string{}
	for _, credential := range body.Data.Credentials {
		got[credential.Secret+"/"+credential.Key] = credential.Value
		if credential.Key == "token" || credential.Secret == "tls" {
			t.Fatalf("non credential secret data leaked: %#v", credential)
		}
	}
	if got["postgresql/username"] != "appuser" || got["postgresql/postgres-password"] != "secret-password" {
		t.Fatalf("credentials = %#v", body.Data.Credentials)
	}
}

func TestGetServiceWorkspaceReturnsBackendWorkspace(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "开发", Identifier: "dev", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "billing-dev-deploy"}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	if err := db.Create(&model.Component{EnvironmentID: env.ID, Name: "api", Type: "backend", ArgoCDApp: "billing-dev-api"}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/environments/:id/services/:serviceId/workspace", GetServiceWorkspace)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/services/1/workspace", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Data struct {
			Kind      string `json:"kind"`
			Resources []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"resources"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.Kind != "gitops" {
		t.Fatalf("kind = %q, want gitops", body.Data.Kind)
	}
	if len(body.Data.Resources) != 1 || body.Data.Resources[0].Name != "billing-dev-api" || body.Data.Resources[0].Type != "Application" {
		t.Fatalf("unexpected resources: %#v", body.Data.Resources)
	}
}

func TestDeleteComponentRemovesArgoCDApplicationAndRuntimeResources(t *testing.T) {
	previousDB := database.DB
	previousK8sClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousK8sClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}, &model.EnvironmentCanvasState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Test", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Staging", Identifier: "staging", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "test-staging-argocd"}).Error; err != nil {
		t.Fatalf("create deploy service: %v", err)
	}
	comp := model.Component{EnvironmentID: env.ID, Name: "backend", Type: "backend", ArgoCDApp: "test-staging-backend"}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	identifier := svcservice.ComponentIdentifier(comp.Name, comp.Type, comp.ID)
	canvas := model.EnvironmentCanvasState{
		EnvironmentID: env.ID,
		Positions:     fmt.Sprintf(`{"component:%d":{"x":10,"y":20},"service:2":{"x":30,"y":40}}`, comp.ID),
		Edges:         fmt.Sprintf(`[{"fromKey":"component:%d","toKey":"service:2"},{"fromKey":"service:2","toKey":"component:%d"}]`, comp.ID, comp.ID),
	}
	if err := db.Create(&canvas).Error; err != nil {
		t.Fatalf("create canvas state: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	componentCR := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: "staging-" + identifier, Namespace: "paap-app-test"},
		Spec: paapv1.ComponentSpec{
			Identifier: identifier,
			Deployment: paapv1.DeploymentSpec{
				Namespace: "test-staging",
			},
		},
	}
	argoApp := &unstructured.Unstructured{Object: map[string]interface{}{}}
	argoApp.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	argoApp.SetName("test-staging-backend")
	argoApp.SetNamespace("test-staging-argocd")
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: identifier, Namespace: "test-staging"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": identifier}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": identifier}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: identifier, Image: "registry/backend:v1"}}},
			},
		},
	}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: identifier, Namespace: "test-staging"}}
	orphanDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "historical-backend-workload",
			Namespace: "test-staging",
			Labels:    map[string]string{"paap.io/component": identifier},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"paap.io/component": identifier}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"paap.io/component": identifier}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: identifier, Image: "registry/backend:v1"}}},
			},
		},
	}
	orphanRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-rs", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": identifier}},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"paap.io/component": identifier}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"paap.io/component": identifier}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: identifier, Image: "registry/backend:v1"}}},
			},
		},
	}
	orphanPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-pod", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": identifier}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: identifier, Image: "registry/backend:v1"}}},
	}
	orphanStatefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-stateful", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": identifier}},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"paap.io/component": identifier}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"paap.io/component": identifier}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: identifier, Image: "registry/backend:v1"}}},
			},
		},
	}
	orphanDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-daemon", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": identifier}},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"paap.io/component": identifier}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"paap.io/component": identifier}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: identifier, Image: "registry/backend:v1"}}},
			},
		},
	}
	orphanSvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "historical-backend-service", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": identifier}}}
	orphanIngress := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "historical-backend-ingress", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": identifier}}}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(componentCR, argoApp, deploy, svc, orphanDeploy, orphanRS, orphanPod, orphanStatefulSet, orphanDaemonSet, orphanSvc, orphanIngress).Build()
	k8s.SetClient(fakeClient)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/components/:id", DeleteComponent)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/components/%d", comp.ID), nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var remaining model.Component
	if err := db.First(&remaining, comp.ID).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("component database row should be deleted, got %v", err)
	}
	deletingArgoApp := &unstructured.Unstructured{}
	deletingArgoApp.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := fakeClient.Get(context.Background(), ctrlclient.ObjectKey{Name: "test-staging-backend", Namespace: "test-staging-argocd"}, deletingArgoApp); err != nil {
		t.Fatalf("argocd application should remain in deleting state while resource finalizer runs, got %v", err)
	}
	if deletingArgoApp.GetDeletionTimestamp().IsZero() || !handlerStringSliceContains(deletingArgoApp.GetFinalizers(), "resources-finalizer.argocd.argoproj.io") {
		t.Fatalf("argocd application should be deleting with resource finalizer, deletionTimestamp=%v finalizers=%#v", deletingArgoApp.GetDeletionTimestamp(), deletingArgoApp.GetFinalizers())
	}
	if err := fakeClient.Get(context.Background(), ctrlclient.ObjectKey{Name: "staging-" + identifier, Namespace: "paap-app-test"}, componentCR); !apierrors.IsNotFound(err) {
		t.Fatalf("component CR should be deleted, got %v", err)
	}
	if err := fakeClient.Get(context.Background(), ctrlclient.ObjectKey{Name: identifier, Namespace: "test-staging"}, deploy); !apierrors.IsNotFound(err) {
		t.Fatalf("deployment should be deleted, got %v", err)
	}
	if err := fakeClient.Get(context.Background(), ctrlclient.ObjectKey{Name: identifier, Namespace: "test-staging"}, svc); !apierrors.IsNotFound(err) {
		t.Fatalf("service should be deleted, got %v", err)
	}
	for _, item := range []ctrlclient.Object{orphanDeploy, orphanRS, orphanPod, orphanStatefulSet, orphanDaemonSet, orphanSvc, orphanIngress} {
		err := fakeClient.Get(context.Background(), ctrlclient.ObjectKeyFromObject(item), item)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("labeled orphan %T/%s should be deleted, got %v", item, item.GetName(), err)
		}
	}
	var savedCanvas model.EnvironmentCanvasState
	if err := db.First(&savedCanvas, "environment_id = ?", env.ID).Error; err != nil {
		t.Fatalf("load canvas state: %v", err)
	}
	if strings.Contains(savedCanvas.Positions, fmt.Sprintf("component:%d", comp.ID)) || strings.Contains(savedCanvas.Edges, fmt.Sprintf("component:%d", comp.ID)) {
		t.Fatalf("canvas still references deleted component: positions=%s edges=%s", savedCanvas.Positions, savedCanvas.Edges)
	}
}

func TestDeleteComponentUsesArgoCDApplicationIdentifierForRuntimeCleanup(t *testing.T) {
	previousDB := database.DB
	previousK8sClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousK8sClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}, &model.EnvironmentCanvasState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Test", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Staging", Identifier: "staging", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "test-staging-argocd"}).Error; err != nil {
		t.Fatalf("create deploy service: %v", err)
	}
	comp := model.Component{
		EnvironmentID: env.ID,
		Name:          "订单服务",
		Type:          "backend",
		ArgoCDApp:     "test-staging-backend-3",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	argoApp := &unstructured.Unstructured{Object: map[string]interface{}{}}
	argoApp.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	argoApp.SetName("test-staging-backend-3")
	argoApp.SetNamespace("test-staging-argocd")
	replicas := int32(1)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backend-3",
			Namespace: "test-staging",
			Labels:    map[string]string{"paap.io/component": "backend-3", "app": "backend-3"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend-3"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "backend-3", "paap.io/component": "backend-3"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "backend-3", Image: "registry/backend:v1"}}},
			},
		},
	}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "backend-3", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": "backend-3", "app": "backend-3"}}}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(argoApp, deploy, svc).Build()
	k8s.SetClient(fakeClient)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/components/:id", DeleteComponent)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/components/%d", comp.ID), nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if err := fakeClient.Get(context.Background(), ctrlclient.ObjectKey{Name: "backend-3", Namespace: "test-staging"}, deploy); !apierrors.IsNotFound(err) {
		t.Fatalf("runtime deployment backend-3 should be deleted, got %v", err)
	}
	if err := fakeClient.Get(context.Background(), ctrlclient.ObjectKey{Name: "backend-3", Namespace: "test-staging"}, svc); !apierrors.IsNotFound(err) {
		t.Fatalf("runtime service backend-3 should be deleted, got %v", err)
	}
}

func TestDeleteEnvironmentRejectsNonMembers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.AppMember{},
		&model.Environment{},
		&model.EnvironmentCanvasState{},
		&model.ServiceInstallation{},
		&model.InfraInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "hidden-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.Component{EnvironmentID: env.ID, Name: "api", Type: "backend"}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/environments/1", nil)
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/environments/:id", withTestAuthUser(2, DeleteEnvironment))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	var count int64
	if err := db.Model(&model.Environment{}).Where("id = ?", env.ID).Count(&count).Error; err != nil {
		t.Fatalf("count environment: %v", err)
	}
	if count != 1 {
		t.Fatalf("environment count = %d, want unchanged", count)
	}
	if err := db.Model(&model.Component{}).Where("environment_id = ?", env.ID).Count(&count).Error; err != nil {
		t.Fatalf("count components: %v", err)
	}
	if count != 1 {
		t.Fatalf("component count = %d, want unchanged", count)
	}
}

func TestDeleteEnvironmentRemovesClusterCRsNamespacesAndDatabaseRows(t *testing.T) {
	previousDB := database.DB
	previousK8sClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousK8sClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.Environment{},
		&model.EnvironmentCanvasState{},
		&model.ServiceInstallation{},
		&model.InfraInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "开发", Identifier: "dev", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "redis", Status: "running", Namespace: "billing-dev-redis"}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	comp := model.Component{EnvironmentID: env.ID, Name: "api", Type: "backend", Image: "registry/api:v1", Version: "v1"}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := db.Create(&model.EnvironmentCanvasState{EnvironmentID: env.ID, Positions: `{}`, Edges: `[]`}).Error; err != nil {
		t.Fatalf("create canvas state: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}
	compIdentifier := componentDeleteIdentifier(app, env, comp)
	envCR := &paapv1.Environment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev",
			Namespace: "paap-app-billing",
			Labels:    map[string]string{"paap.io/app": "billing", "paap.io/env": "dev"},
		},
		Spec: paapv1.EnvironmentSpec{Identifier: "dev", PrimaryNamespace: "billing-dev"},
	}
	svcCR := &paapv1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev-redis",
			Namespace: "paap-app-billing",
			Labels:    map[string]string{"paap.io/app": "billing", "paap.io/env": "dev"},
		},
		Spec: paapv1.ServiceInstanceSpec{Type: "redis", ToolNamespace: "billing-dev-redis"},
	}
	compCR := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dev-" + compIdentifier,
			Namespace: "paap-app-billing",
			Labels:    map[string]string{"paap.io/app": "billing", "paap.io/env": "dev", "paap.io/component": compIdentifier},
		},
		Spec: paapv1.ComponentSpec{Identifier: compIdentifier, Deployment: paapv1.DeploymentSpec{Namespace: "billing-dev"}},
	}
	nsApp := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "paap-app-billing", Labels: map[string]string{"paap.io/app": "billing", "paap.io/managed-by": "paap-operator"}}}
	nsPrimary := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "billing-dev", Labels: map[string]string{"paap.io/app": "billing", "paap.io/env": "dev"}}}
	nsAppWorkload := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-app", Labels: map[string]string{"paap.io/app": "billing", "paap.io/env": "dev"}}}
	nsTool := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-redis", Labels: map[string]string{"paap.io/app": "billing", "paap.io/env": "dev", "paap.io/service-type": "redis"}}}
	nsOtherEnv := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "billing-prod", Labels: map[string]string{"paap.io/app": "billing", "paap.io/env": "prod"}}}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(envCR, svcCR, compCR, nsApp, nsPrimary, nsAppWorkload, nsTool, nsOtherEnv).Build()
	k8s.SetClient(fakeClient)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.DELETE("/api/v1/environments/:id", withTestAuthUserRole(1, "admin", DeleteEnvironment))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/environments/%d", env.ID), nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	for _, obj := range []ctrlclient.Object{envCR, svcCR, compCR, nsPrimary, nsAppWorkload, nsTool} {
		err := fakeClient.Get(context.Background(), ctrlclient.ObjectKeyFromObject(obj), obj)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("%T/%s should be deleted, got %v", obj, obj.GetName(), err)
		}
	}
	if err := fakeClient.Get(context.Background(), ctrlclient.ObjectKeyFromObject(nsApp), nsApp); err != nil {
		t.Fatalf("app CR namespace should remain while deleting one environment: %v", err)
	}
	if err := fakeClient.Get(context.Background(), ctrlclient.ObjectKeyFromObject(nsOtherEnv), nsOtherEnv); err != nil {
		t.Fatalf("other environment namespace should remain: %v", err)
	}
	for _, table := range []interface{}{&model.Environment{}, &model.ServiceInstallation{}, &model.Component{}, &model.EnvironmentCanvasState{}} {
		var count int64
		if err := db.Model(table).Count(&count).Error; err != nil {
			t.Fatalf("count %T: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("%T rows = %d, want 0", table, count)
		}
	}
}

func TestApplyArgoCDApplicationRejectsNamespaceOutsideEnvironment(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	previousSync := syncClusterState
	t.Cleanup(func() { syncClusterState = previousSync })
	syncClusterState = func(context.Context, *gorm.DB, ctrlclient.Client) error { return nil }

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "开发", Identifier: "dev", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "billing-dev-deploy"}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "billing-dev",
		Labels: map[string]string{"paap.io/app": "billing", "paap.io/env": "dev"},
	}}).Build())

	body, _ := json.Marshal(WorkspaceActionRequest{
		Action: "apply_argocd_application",
		Params: map[string]string{
			"name":                 "billing-dev-api",
			"project":              "default",
			"repoURL":              "http://gitea/paap/billing-dev-components.git",
			"path":                 "components/api",
			"destinationNamespace": "kube-system",
		},
	})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/services/:serviceId/workspace/actions", RunServiceWorkspaceAction)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/services/1/workspace/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestApplyArgoCDApplicationForcesEnvironmentProject(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	previousSync := syncClusterState
	t.Cleanup(func() { syncClusterState = previousSync })
	syncClusterState = func(context.Context, *gorm.DB, ctrlclient.Client) error { return nil }

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "开发", Identifier: "dev", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "billing-dev-deploy"}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	cl := fake.NewClientBuilder().WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "billing-dev",
		Labels: map[string]string{"paap.io/app": "billing", "paap.io/env": "dev"},
	}}).Build()
	k8s.SetClient(cl)

	body, _ := json.Marshal(WorkspaceActionRequest{
		Action: "apply_argocd_application",
		Params: map[string]string{
			"name":                 "billing-dev-api",
			"project":              "default",
			"repoURL":              "http://gitea/paap/billing-dev-components.git",
			"path":                 "components/api",
			"destinationNamespace": "billing-dev",
		},
	})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/services/:serviceId/workspace/actions", RunServiceWorkspaceAction)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/services/1/workspace/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := cl.Get(t.Context(), ctrlclient.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev-api"}, got); err != nil {
		t.Fatalf("get application: %v", err)
	}
	project, _, _ := unstructured.NestedString(got.Object, "spec", "project")
	if project != "billing-dev" {
		t.Fatalf("project = %q, want billing-dev", project)
	}
	assertEnvironmentArgoCDProject(t, cl, "billing-dev-deploy", "billing-dev", []string{"billing-dev"})
	assertDefaultArgoCDProjectDenied(t, cl, "billing-dev-deploy")
}

func TestAllowedArgoCDDestinationNamespacesIncludesEnvironmentNamespaces(t *testing.T) {
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })

	app := model.Application{Identifier: "billing"}
	env := model.Environment{Identifier: "dev", Namespace: "billing-dev"}
	cl := fake.NewClientBuilder().WithObjects(
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
			Name: "billing-dev-monitor",
			Labels: map[string]string{
				"paap.io/app":     "billing",
				"paap.io/env":     "dev",
				"paap.io/role":    "tool",
				"paap.io/service": "monitor",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
			Labels: map[string]string{
				"paap.io/app":  "billing",
				"paap.io/env":  "dev",
				"paap.io/role": "workload",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			Labels: map[string]string{
				"paap.io/app":  "billing",
				"paap.io/env":  "dev",
				"paap.io/role": "tool",
			},
		}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "billing-prod-monitor",
			Labels: map[string]string{
				"paap.io/app":  "billing",
				"paap.io/env":  "prod",
				"paap.io/role": "tool",
			},
		}},
	).Build()
	k8s.SetClient(cl)

	got := allowedArgoCDDestinationNamespaces(t.Context(), app, env)
	want := []string{"billing-dev", "billing-dev-app", "billing-dev-deploy", "billing-dev-monitor"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("allowed namespaces = %#v, want environment namespaces %#v", got, want)
	}
}

func TestApplyArgoCDApplicationSetForcesEnvironmentProject(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	previousSync := syncClusterState
	t.Cleanup(func() { syncClusterState = previousSync })
	syncClusterState = func(context.Context, *gorm.DB, ctrlclient.Client) error { return nil }

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "开发", Identifier: "dev", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "billing-dev-deploy"}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	cl := fake.NewClientBuilder().WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "billing-dev",
		Labels: map[string]string{"paap.io/app": "billing", "paap.io/env": "dev"},
	}}).Build()
	k8s.SetClient(cl)

	body, _ := json.Marshal(WorkspaceActionRequest{
		Action: "apply_argocd_applicationset",
		Params: map[string]string{
			"name":                 "billing-dev-components",
			"project":              "default",
			"repoURL":              "http://gitea/paap/billing-dev-components.git",
			"path":                 "components/*",
			"targetRevision":       "main",
			"destinationNamespace": "billing-dev",
		},
	})
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/services/:serviceId/workspace/actions", RunServiceWorkspaceAction)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/services/1/workspace/actions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationSet"})
	if err := cl.Get(t.Context(), ctrlclient.ObjectKey{Namespace: "billing-dev-deploy", Name: "billing-dev-components"}, got); err != nil {
		t.Fatalf("get applicationset: %v", err)
	}
	project, _, _ := unstructured.NestedString(got.Object, "spec", "template", "spec", "project")
	destNS, _, _ := unstructured.NestedString(got.Object, "spec", "template", "spec", "destination", "namespace")
	if project != "billing-dev" || destNS != "billing-dev" {
		t.Fatalf("ApplicationSet must bind to env project and namespace, project=%q destNS=%q spec=%#v", project, destNS, got.Object["spec"])
	}
	assertEnvironmentArgoCDProject(t, cl, "billing-dev-deploy", "billing-dev", []string{"billing-dev"})
	assertDefaultArgoCDProjectDenied(t, cl, "billing-dev-deploy")
}

func assertEnvironmentArgoCDProject(t *testing.T, cl ctrlclient.Client, namespace, name string, wantDestinations []string) {
	t.Helper()
	project := &unstructured.Unstructured{}
	project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	if err := cl.Get(t.Context(), ctrlclient.ObjectKey{Namespace: namespace, Name: name}, project); err != nil {
		t.Fatalf("get environment AppProject: %v", err)
	}
	destinations, _, _ := unstructured.NestedSlice(project.Object, "spec", "destinations")
	got := map[string]bool{}
	for _, item := range destinations {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ns, _, _ := unstructured.NestedString(m, "namespace")
		server, _, _ := unstructured.NestedString(m, "server")
		if ns == "*" || server == "*" {
			t.Fatalf("environment AppProject must not allow wildcard destinations: %#v", destinations)
		}
		if ns != "" {
			got[ns] = true
		}
	}
	for _, ns := range wantDestinations {
		if !got[ns] {
			t.Fatalf("environment AppProject missing destination %q, got %#v", ns, destinations)
		}
	}
	clusterWhitelist, _, _ := unstructured.NestedSlice(project.Object, "spec", "clusterResourceWhitelist")
	if len(clusterWhitelist) != 0 {
		t.Fatalf("environment AppProject must not whitelist cluster-scoped resources: %#v", clusterWhitelist)
	}
}

func assertDefaultArgoCDProjectDenied(t *testing.T, cl ctrlclient.Client, namespace string) {
	t.Helper()
	project := &unstructured.Unstructured{}
	project.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "AppProject"})
	if err := cl.Get(t.Context(), ctrlclient.ObjectKey{Namespace: namespace, Name: "default"}, project); err != nil {
		t.Fatalf("get default AppProject: %v", err)
	}
	destinations, _, _ := unstructured.NestedSlice(project.Object, "spec", "destinations")
	sourceRepos, _, _ := unstructured.NestedSlice(project.Object, "spec", "sourceRepos")
	clusterWhitelist, _, _ := unstructured.NestedSlice(project.Object, "spec", "clusterResourceWhitelist")
	if len(destinations) != 0 || len(sourceRepos) != 0 || len(clusterWhitelist) != 0 {
		t.Fatalf("default AppProject must be denied, destinations=%#v repos=%#v cluster=%#v", destinations, sourceRepos, clusterWhitelist)
	}
}

func TestBuildLiveMonitorWorkspaceAddsObjectLevelDashboards(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Application{},
		&model.Environment{},
		&model.ServiceInstallation{},
		&model.Component{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "billing-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	monitor := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "monitor", Status: "running", Namespace: "billing-prod-monitor"}
	git := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "git", ServiceName: "Gitea", Status: "running", Namespace: "billing-prod-git", ReleaseName: "billing-prod-git"}
	redis := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "redis", ServiceName: "Redis", Status: "running", Namespace: "billing-prod-redis", ReleaseName: "billing-prod-redis"}
	if err := db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	if err := db.Create(&git).Error; err != nil {
		t.Fatalf("create git: %v", err)
	}
	if err := db.Create(&redis).Error; err != nil {
		t.Fatalf("create redis: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-7586d9fd4f-9x2mk", Namespace: "billing-prod"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "redis-0", Namespace: "billing-prod-redis"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	).Build())

	workspace := buildLiveToolWorkspace(app, env, monitor, []model.Component{{Name: "api", Type: "backend", Status: "running"}})

	subjects := map[string]map[string]interface{}{}
	urls := map[string]string{}
	for _, resource := range workspace.Resources {
		if resource.Type == "Monitor Subject" {
			subjects[resource.Name] = resource.Annotations
			urls[resource.Name] = resource.ExternalURL
		}
	}
	if subjects["api"]["dashboardUid"] != "paap-pod-workload" {
		t.Fatalf("component dashboard annotations = %#v", subjects["api"])
	}
	if !strings.Contains(urls["api"], "var-namespace=billing-prod") || !strings.Contains(urls["api"], "var-workload=api") {
		t.Fatalf("component dashboard URL should carry Grafana variables, got %q", urls["api"])
	}
	if subjects["Gitea"]["dashboardUid"] != "paap-gitea" || !strings.Contains(urls["Gitea"], "/proxy/d/paap-gitea") {
		t.Fatalf("gitea monitor subject annotations=%#v url=%q", subjects["Gitea"], urls["Gitea"])
	}
	if subjects["Redis"]["dashboardUid"] != "paap-redis" || subjects["Redis"]["subjectKind"] != "middleware" {
		t.Fatalf("redis monitor subject annotations = %#v", subjects["Redis"])
	}
	if subjects["api-7586d9fd4f-9x2mk"]["resourceKind"] != "pod" || subjects["api-7586d9fd4f-9x2mk"]["subjectKind"] != "component" {
		t.Fatalf("runtime component pod should be present as monitor subject: %#v", subjects["api-7586d9fd4f-9x2mk"])
	}
	if subjects["redis-0"]["resourceKind"] != "pod" || subjects["redis-0"]["subjectKind"] != "middleware" {
		t.Fatalf("runtime middleware pod should be present as monitor subject: %#v", subjects["redis-0"])
	}
}

func TestBuiltInGrafanaWorkloadDashboardsContainOperationalPanels(t *testing.T) {
	dashboards := buildDefaultGrafanaDashboards(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "prod"},
		[]model.Component{{Name: "api", Type: "backend", ID: 7}},
	)

	byTitle := map[string]string{}
	for _, dashboard := range dashboards {
		byTitle[dashboard.Title] = dashboard.JSON
	}
	assertDashboardPanels(t, byTitle["PAAP Pod Workload"], 30, []string{
		"Pods Ready",
		"Desired Replicas",
		"Unavailable Replicas",
		"CPU Usage",
		"CPU Throttling",
		"Memory Working Set",
		"Memory Limit %",
		"Restarts",
		"Network Receive",
		"Network Transmit",
		"Filesystem Usage",
		"PVCs",
		"Container Count",
		"Waiting Containers",
		"CPU Usage / Requests",
		"Memory RSS",
		"Network Receive Packets",
		"Filesystem Reads",
		"Pod Phases",
	})
	assertDashboardTemplatingHidden(t, byTitle["PAAP Pod Workload"], []string{"namespace", "workload"})
	assertDashboardExprs(t, byTitle["PAAP Pod Workload"], []string{
		"kube_pod_status_ready",
		"kube_deployment_spec_replicas",
		"container_cpu_cfs_throttled_seconds_total",
		"kube_pod_container_status_waiting",
		"container_network_receive_packets_total",
		"container_fs_reads_bytes_total",
		"container_fs_usage_bytes",
		"kube_persistentvolumeclaim_info",
	})
	assertDashboardExprsAbsent(t, byTitle["PAAP Pod Workload"], []string{
		"kubelet_volume_stats_used_bytes",
		"kube_event_count",
	})
	assertDashboardPanels(t, byTitle["PAAP MySQL"], 16, []string{
		"CPU Usage",
		"Memory Working Set",
		"Restarts",
		"Network Receive",
		"PVCs",
		"Database Connections",
		"Slow Queries",
		"Replication Lag",
	})
	assertDashboardExprs(t, byTitle["PAAP RabbitMQ"], []string{
		"rabbitmq_queue_messages_ready",
		"rabbitmq_connections",
	})
	assertDashboardExprs(t, byTitle["PAAP Kafka"], []string{
		"kafka_server_brokertopicmetrics_messagesin_total",
		"kafka_consumergroup_lag",
	})
	assertDashboardExprs(t, byTitle["PAAP MinIO"], []string{
		"minio_cluster_capacity_usable_total_bytes",
		"minio_s3_requests_total",
	})
}

func TestBuiltInGrafanaEnvironmentOverviewContainsFleetPanels(t *testing.T) {
	dashboardJSON := buildDefaultGrafanaDashboard(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "prod"},
		[]model.Component{{Name: "api", Type: "backend", ID: 7}},
	)

	assertDashboardPanels(t, dashboardJSON, 12, []string{
		"Environment Namespaces",
		"Workload Namespaces",
		"Workloads",
		"Running Pods",
		"Unready Pods",
		"PVCs",
		"CPU Usage by Namespace",
		"Memory Usage by Namespace",
		"Restarts by Namespace",
		"Network Receive by Namespace",
		"Network Transmit by Namespace",
		"PVCs by Namespace",
	})
	assertDashboardExprs(t, dashboardJSON, []string{
		`kube_namespace_status_phase{namespace=~\"billing-prod.*\", phase=\"Active\"}`,
		`kube_namespace_status_phase{namespace=\"billing-prod\", phase=\"Active\"}`,
		"kube_pod_owner",
		"kube_pod_status_ready",
		"container_cpu_usage_seconds_total",
		"kube_persistentvolumeclaim_info",
		`* on(namespace, pod) group_left() max by (namespace, pod) (kube_pod_status_phase{namespace=~\"billing-prod.*\", phase=\"Running\"})`,
		`owner_kind!=\"Job\"`,
	})
	assertDashboardExprsAbsent(t, dashboardJSON, []string{
		"kube_pod_labels",
		"kubelet_volume_stats_used_bytes",
		"kube_event_count",
	})
}

func TestBuiltInGrafanaDashboardsUseZeroFallbackForEveryPanel(t *testing.T) {
	dashboards := buildDefaultGrafanaDashboards(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "prod"},
		[]model.Component{{Name: "api", Type: "backend", ID: 7}},
	)
	dashboards = append(dashboards, defaultGrafanaDashboard{
		Title: "Environment Overview",
		JSON: buildDefaultGrafanaDashboard(
			model.Application{Identifier: "billing"},
			model.Environment{Identifier: "prod"},
			[]model.Component{{Name: "api", Type: "backend", ID: 7}},
		),
	})

	for _, dashboard := range dashboards {
		for _, expr := range dashboardTargetExpressions(t, dashboard.JSON) {
			if !strings.Contains(expr, "or vector(0)") {
				t.Fatalf("dashboard %q has target without zero fallback: %s", dashboard.Title, expr)
			}
		}
	}
}

func TestBuiltInGrafanaDashboardsGuardRatioDenominators(t *testing.T) {
	dashboards := buildDefaultGrafanaDashboards(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "prod"},
		[]model.Component{{Name: "api", Type: "backend", ID: 7}},
	)

	for _, dashboard := range dashboards {
		for _, expr := range dashboardTargetExpressions(t, dashboard.JSON) {
			if strings.Contains(expr, " / sum(") && !strings.Contains(expr, "clamp_min(") {
				t.Fatalf("dashboard %q has ratio expression without denominator guard: %s", dashboard.Title, expr)
			}
		}
	}
}

func TestEnsureDefaultGrafanaDashboardsOverwritesExistingBuiltIns(t *testing.T) {
	importedUIDs := map[string]bool{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/dashboards/db" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var payload struct {
			Dashboard json.RawMessage `json:"dashboard"`
			Overwrite bool            `json:"overwrite"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode import payload: %v", err)
		}
		if !payload.Overwrite {
			t.Fatalf("overwrite = false, want true")
		}
		var dashboard struct {
			UID string `json:"uid"`
		}
		if err := json.Unmarshal(payload.Dashboard, &dashboard); err != nil {
			t.Fatalf("decode dashboard: %v", err)
		}
		importedUIDs[dashboard.UID] = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer server.Close()

	grafana := &k8s.GrafanaClient{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}
	imported := ensureDefaultGrafanaDashboards(
		grafana,
		[]k8s.GrafanaDashboard{{UID: "paap-pod-workload", URL: "/d/paap-pod-workload"}},
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "prod"},
		[]model.Component{{Name: "api", Type: "backend", ID: 7}},
	)

	if !imported {
		t.Fatalf("imported = false, want true")
	}
	if !importedUIDs["paap-pod-workload"] {
		t.Fatalf("existing built-in dashboard was not overwritten; imported UIDs=%v", importedUIDs)
	}
}

func assertDashboardPanels(t *testing.T, dashboardJSON string, minPanels int, titles []string) {
	t.Helper()
	var dashboard struct {
		Panels []struct {
			Title string `json:"title"`
		} `json:"panels"`
	}
	if err := json.Unmarshal([]byte(dashboardJSON), &dashboard); err != nil {
		t.Fatalf("unmarshal dashboard: %v", err)
	}
	if len(dashboard.Panels) < minPanels {
		t.Fatalf("dashboard has %d panels, want at least %d", len(dashboard.Panels), minPanels)
	}
	haystack := dashboardJSON
	for _, title := range titles {
		if !strings.Contains(haystack, title) {
			t.Fatalf("dashboard missing panel title %q", title)
		}
	}
}

func assertDashboardExprs(t *testing.T, dashboardJSON string, exprParts []string) {
	t.Helper()
	for _, expr := range exprParts {
		if !strings.Contains(dashboardJSON, expr) {
			t.Fatalf("dashboard missing expression fragment %q", expr)
		}
	}
}

func assertDashboardExprsAbsent(t *testing.T, dashboardJSON string, exprParts []string) {
	t.Helper()
	for _, expr := range exprParts {
		if strings.Contains(dashboardJSON, expr) {
			t.Fatalf("dashboard contains unsupported expression fragment %q", expr)
		}
	}
}

func assertDashboardTemplatingHidden(t *testing.T, dashboardJSON string, names []string) {
	t.Helper()
	var dashboard struct {
		Templating struct {
			List []struct {
				Name string `json:"name"`
				Hide int    `json:"hide"`
			} `json:"list"`
		} `json:"templating"`
	}
	if err := json.Unmarshal([]byte(dashboardJSON), &dashboard); err != nil {
		t.Fatalf("unmarshal dashboard: %v", err)
	}
	hidden := map[string]int{}
	for _, item := range dashboard.Templating.List {
		hidden[item.Name] = item.Hide
	}
	for _, name := range names {
		if hidden[name] != 2 {
			t.Fatalf("templating variable %q hide = %d, want 2", name, hidden[name])
		}
	}
}

func dashboardTargetExpressions(t *testing.T, dashboardJSON string) []string {
	t.Helper()
	var dashboard struct {
		Panels []struct {
			Targets []struct {
				Expr string `json:"expr"`
			} `json:"targets"`
		} `json:"panels"`
	}
	if err := json.Unmarshal([]byte(dashboardJSON), &dashboard); err != nil {
		t.Fatalf("unmarshal dashboard: %v", err)
	}
	exprs := []string{}
	for _, panel := range dashboard.Panels {
		for _, target := range panel.Targets {
			exprs = append(exprs, target.Expr)
		}
	}
	return exprs
}

func TestDownloadRegistryCACertificateReturnsPublicCA(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "开发", Identifier: "dev", Namespace: "billing-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	inst := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "registry", Status: "running", Namespace: "billing-dev-registry", ReleaseName: "billing-dev-registry"}
	if err := db.Create(&inst).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-registry-tls", Namespace: "billing-dev-registry"},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"ca.crt":  []byte("-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n"),
			"tls.key": []byte("-----BEGIN PRIVATE KEY-----\nsecret\n-----END PRIVATE KEY-----\n"),
		},
	}).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/environments/:id/services/:serviceId/registry-ca.crt", DownloadRegistryCACertificate)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/services/1/registry-ca.crt", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("X-PAAP-Certificate-Source"); got != "billing-dev-registry-tls/ca.crt" {
		t.Fatalf("certificate source = %q", got)
	}
	if body := rec.Body.String(); body != "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n" {
		t.Fatalf("unexpected body: %q", body)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("PRIVATE KEY")) {
		t.Fatalf("response leaked private key: %s", rec.Body.String())
	}
}

func TestJenkinsBuildTargetUsesRequestedJob(t *testing.T) {
	jobs := []k8s.JenkinsJob{
		{Name: "api-build"},
		{Name: "web-build"},
	}

	got, ok := jenkinsBuildTarget(jobs, "web-build")
	if !ok || got != "web-build" {
		t.Fatalf("target = %q %v, want web-build true", got, ok)
	}

	got, ok = jenkinsBuildTarget(jobs, "")
	if !ok || got != "api-build" {
		t.Fatalf("default target = %q %v, want api-build true", got, ok)
	}

	if got, ok = jenkinsBuildTarget(jobs, "missing"); ok || got != "" {
		t.Fatalf("missing target = %q %v, want empty false", got, ok)
	}
}

func TestEnrichJenkinsWorkspaceDoesNotKeepFallbackJobsWhenJenkinsIsEmpty(t *testing.T) {
	previous := newJenkinsWorkspaceClient
	t.Cleanup(func() { newJenkinsWorkspaceClient = previous })
	newJenkinsWorkspaceClient = func(namespace string) jenkinsWorkspaceClient {
		if namespace != "billing-dev-ci" {
			t.Fatalf("namespace = %q, want billing-dev-ci", namespace)
		}
		return fakeJenkinsWorkspaceClient{}
	}

	workspace := svcservice.BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "dev"},
		model.ServiceInstallation{ID: 10, EnvironmentID: 1, ServiceType: "ci", Namespace: "billing-dev-ci", Status: "running"},
		[]model.Component{{Name: "api", JenkinsJob: "billing-dev-api-build", PipelineStatus: "built"}},
	)
	if len(workspace.Resources) != 1 {
		t.Fatalf("test setup expected fallback job resource, got %#v", workspace.Resources)
	}

	enriched := enrichJenkinsWorkspace(t.Context(), workspace, model.ServiceInstallation{ID: 10, EnvironmentID: 1, ServiceType: "ci", Namespace: "billing-dev-ci"})
	if len(enriched.Resources) != 1 {
		t.Fatalf("live Jenkins workspace must expose one real empty catalog resource, got %#v", enriched.Resources)
	}
	resource := enriched.Resources[0]
	if resource.Name != "jenkins-jobs" || resource.Type != "流水线目录" || resource.Status != "Empty" {
		t.Fatalf("unexpected empty catalog resource: %#v", resource)
	}
}

func TestEnrichJenkinsWorkspaceMergesRealJobWithComponentContext(t *testing.T) {
	previous := newJenkinsWorkspaceClient
	t.Cleanup(func() { newJenkinsWorkspaceClient = previous })
	newJenkinsWorkspaceClient = func(namespace string) jenkinsWorkspaceClient {
		if namespace != "billing-dev-ci" {
			t.Fatalf("namespace = %q, want billing-dev-ci", namespace)
		}
		return fakeJenkinsWorkspaceClient{
			jobs: []k8s.JenkinsJob{{
				Name:      "billing-dev-api-build",
				URL:       "http://jenkins/job/billing-dev-api-build/",
				Color:     "blue",
				Status:    "success",
				LastBuild: &k8s.JenkinsBuild{Number: 42, URL: "http://jenkins/job/billing-dev-api-build/42/", Result: "SUCCESS"},
			}},
			consoleLog: map[string]string{"billing-dev-api-build": "Started by PAAP\nFinished: SUCCESS\n"},
		}
	}

	workspace := svcservice.BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "dev"},
		model.ServiceInstallation{ID: 10, EnvironmentID: 1, ServiceType: "ci", Namespace: "billing-dev-ci", Status: "running"},
		[]model.Component{{
			Name:          "api",
			JenkinsJob:    "billing-dev-api-build",
			RegistryImage: "registry.example.test/billing-dev/api:v1",
			SourceBranch:  "release",
		}},
	)

	enriched := enrichJenkinsWorkspace(t.Context(), workspace, model.ServiceInstallation{ID: 10, EnvironmentID: 1, ServiceType: "ci", Namespace: "billing-dev-ci"})
	if len(enriched.Resources) != 1 {
		t.Fatalf("resources = %#v", enriched.Resources)
	}
	job := enriched.Resources[0]
	handlerAssertAnnotation(t, job, "component", "api")
	handlerAssertAnnotation(t, job, "branch", "release")
	handlerAssertAnnotation(t, job, "image", "registry.example.test/billing-dev/api:v1")
	handlerAssertAnnotation(t, job, "lastBuildNumber", 42)
	handlerAssertAnnotation(t, job, "consoleLog", "Started by PAAP\nFinished: SUCCESS\n")
}

func TestEnrichGiteaWorkspaceKeepsCloneURLAndRepoActionsWithoutEagerTree(t *testing.T) {
	previous := newGiteaWorkspaceClient
	t.Cleanup(func() {
		newGiteaWorkspaceClient = previous
		clearGiteaWorkspaceCache()
	})
	clearGiteaWorkspaceCache()
	newGiteaWorkspaceClient = func(namespace string) giteaWorkspaceClient {
		if namespace != "billing-dev-git" {
			t.Fatalf("namespace = %q, want billing-dev-git", namespace)
		}
		return fakeGiteaWorkspaceClient{}
	}

	workspace := svcservice.BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "dev"},
		model.ServiceInstallation{ServiceType: "git", Namespace: "billing-dev-git", Status: "running"},
		nil,
	)
	enriched := enrichGiteaWorkspace(t.Context(), workspace, model.ServiceInstallation{ID: 3, EnvironmentID: 1, ServiceType: "git", Namespace: "billing-dev-git"})

	if len(enriched.Resources) != 1 {
		t.Fatalf("resources = %#v", enriched.Resources)
	}
	repo := enriched.Resources[0]
	if repo.ExternalURL != "/api/v1/environments/1/services/3/proxy/paap/billing-dev-components" {
		t.Fatalf("external URL = %q", repo.ExternalURL)
	}
	if repo.Annotations["cloneURL"] != "ssh://git@gitea/paap/billing-dev-components.git" {
		t.Fatalf("cloneURL annotation = %#v", repo.Annotations)
	}
	if len(repo.Children) != 0 {
		t.Fatalf("repository workspace should not eagerly fetch file tree children: %#v", repo.Children)
	}
	if _, ok := repo.Annotations["commits"]; ok {
		t.Fatalf("repository workspace should not eagerly fetch commits: %#v", repo.Annotations["commits"])
	}
	if len(repo.Actions) != 1 || repo.Actions[0].Key != "reconcile_gitops" || repo.Actions[0].Target != "billing-dev-components" {
		t.Fatalf("actions = %#v", repo.Actions)
	}
}

func TestBuildLiveToolWorkspacePrefersIngressExternalURLForWorkspaceResources(t *testing.T) {
	previous := newGiteaWorkspaceClient
	t.Cleanup(func() {
		newGiteaWorkspaceClient = previous
		clearGiteaWorkspaceCache()
	})
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	clearGiteaWorkspaceCache()
	newGiteaWorkspaceClient = func(namespace string) giteaWorkspaceClient {
		if namespace != "billing-dev-git" {
			t.Fatalf("namespace = %q, want billing-dev-git", namespace)
		}
		return fakeGiteaWorkspaceClient{}
	}
	pathType := networkingv1.PathTypePrefix
	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).WithObjects(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "kind-control-plane"},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{
				Type:    corev1.NodeInternalIP,
				Address: "172.18.0.2",
			}}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-git", Namespace: "billing-dev-git"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{{Name: "http", Port: 3000, NodePort: 30091}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-argocd-server", Namespace: "billing-dev-argocd"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80, NodePort: 31767},
					{Name: "https", Port: 443, NodePort: 30969},
				},
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-git", Namespace: "billing-dev-git"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "git.billing-dev.example.test",
					IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{Path: "/", PathType: &pathType}},
					}},
				}},
			},
		},
	).Build())

	workspace := buildLiveToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{ID: 2, Identifier: "dev", Namespace: "billing-dev"},
		model.ServiceInstallation{ID: 3, EnvironmentID: 2, ServiceType: "git", Namespace: "billing-dev-git", Status: "running"},
		nil,
	)

	if len(workspace.Resources) != 1 {
		t.Fatalf("resources = %#v", workspace.Resources)
	}
	repo := workspace.Resources[0]
	if repo.ExternalURL != "http://git.billing-dev.example.test/paap/billing-dev-components" {
		t.Fatalf("external URL should use ingress before proxy/nodeport, got %q", repo.ExternalURL)
	}
	if repo.Annotations["proxyURL"] != "/api/v1/environments/2/services/3/proxy/paap/billing-dev-components" {
		t.Fatalf("proxyURL annotation = %#v", repo.Annotations)
	}
	foundExternalConfig := false
	for _, cfg := range workspace.Config {
		if strings.HasPrefix(cfg.Label, "外部访问地址") && cfg.Value == "http://git.billing-dev.example.test/" {
			foundExternalConfig = true
		}
	}
	if !foundExternalConfig {
		t.Fatalf("workspace config missing ingress external URL: %#v", workspace.Config)
	}
}

type fakeArgoCDWorkspaceClient struct{}

func (fakeArgoCDWorkspaceClient) ResourceTree(context.Context, string) ([]k8s.ArgoCDResource, error) {
	return nil, nil
}

func TestBuildLiveToolWorkspaceAddsExternalRepoURLForArgoCDApplications(t *testing.T) {
	previousClient := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previousClient) })
	previousArgoCDClient := newArgoCDWorkspaceClient
	t.Cleanup(func() { newArgoCDWorkspaceClient = previousArgoCDClient })
	newArgoCDWorkspaceClient = func(namespace string) argoCDWorkspaceClient {
		if namespace != "billing-dev-argocd" {
			t.Fatalf("namespace = %q, want billing-dev-argocd", namespace)
		}
		return fakeArgoCDWorkspaceClient{}
	}

	pathType := networkingv1.PathTypePrefix
	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := networkingv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add networking scheme: %v", err)
	}
	appResource := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]interface{}{
			"name":      "billing-dev-api",
			"namespace": "billing-dev-argocd",
		},
		"spec": map[string]interface{}{
			"source": map[string]interface{}{
				"repoURL": "http://billing-dev-git.billing-dev-git.svc.cluster.local:3000/paap/billing-dev-components.git",
				"path":    "components/api",
			},
			"destination": map[string]interface{}{
				"server":    "https://kubernetes.default.svc",
				"namespace": "billing-dev",
			},
		},
		"status": map[string]interface{}{
			"sync":   map[string]interface{}{"status": "Synced", "revision": "abcdef123456"},
			"health": map[string]interface{}{"status": "Healthy"},
		},
	}}
	appResource.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	k8s.SetClient(fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(
		appResource,
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "kind-control-plane"},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{
				Type:    corev1.NodeInternalIP,
				Address: "172.18.0.2",
			}}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-git", Namespace: "billing-dev-git"},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{{Name: "http", Port: 3000, NodePort: 30091}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-argocd-server", Namespace: "billing-dev-argocd"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80, NodePort: 31767},
					{Name: "https", Port: 443, NodePort: 30969},
				},
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-git", Namespace: "billing-dev-git"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "git.billing-dev.example.test",
					IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{Path: "/", PathType: &pathType}},
					}},
				}},
			},
		},
	).Build())

	workspace := buildLiveToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{ID: 2, Identifier: "dev", Namespace: "billing-dev"},
		model.ServiceInstallation{ID: 8, EnvironmentID: 2, ServiceType: "deploy", Namespace: "billing-dev-argocd", Status: "running"},
		nil,
	)

	if len(workspace.Resources) != 1 {
		t.Fatalf("resources = %#v", workspace.Resources)
	}
	annotations := workspace.Resources[0].Annotations
	internalRepo := "http://billing-dev-git.billing-dev-git.svc.cluster.local:3000/paap/billing-dev-components.git"
	if annotations["repoURL"] != internalRepo {
		t.Fatalf("repoURL must remain the ArgoCD internal source URL, got %#v", annotations["repoURL"])
	}
	if annotations["externalRepoURL"] != "http://git.billing-dev.example.test/paap/billing-dev-components.git" {
		t.Fatalf("externalRepoURL should use ingress for browser-facing display, annotations=%#v", annotations)
	}
	if workspace.Resources[0].Description != "Source: http://git.billing-dev.example.test/paap/billing-dev-components.git components/api" {
		t.Fatalf("description should use browser-facing source URL, got %q", workspace.Resources[0].Description)
	}
	if workspace.Resources[0].ExternalURL != "http://172.18.0.2:31767/applications/billing-dev-argocd/billing-dev-api?view=tree&resource=" {
		t.Fatalf("argocd application external URL should open the resource tree, got %q", workspace.Resources[0].ExternalURL)
	}
}

func TestEnrichGiteaWorkspacePreservesComponentRepositoryPath(t *testing.T) {
	previous := newGiteaWorkspaceClient
	t.Cleanup(func() {
		newGiteaWorkspaceClient = previous
		clearGiteaWorkspaceCache()
	})
	clearGiteaWorkspaceCache()
	newGiteaWorkspaceClient = func(namespace string) giteaWorkspaceClient {
		if namespace != "billing-dev-git" {
			t.Fatalf("namespace = %q, want billing-dev-git", namespace)
		}
		return fakeGiteaWorkspaceClient{}
	}

	workspace := svcservice.BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "dev"},
		model.ServiceInstallation{ServiceType: "git", Namespace: "billing-dev-git", Status: "running"},
		[]model.Component{{
			Name:       "api",
			Type:       "backend",
			GitRepoURL: "http://gitea/paap/billing-dev-components.git",
			GitPath:    "components/api",
		}},
	)
	enriched := enrichGiteaWorkspace(t.Context(), workspace, model.ServiceInstallation{ID: 3, EnvironmentID: 1, ServiceType: "git", Namespace: "billing-dev-git"})

	if len(enriched.Resources) != 1 {
		t.Fatalf("resources = %#v", enriched.Resources)
	}
	repo := enriched.Resources[0]
	if repo.Name != "billing-dev-components/components/api" {
		t.Fatalf("component repository resource name = %q", repo.Name)
	}
	if repo.Annotations["path"] != "components/api" {
		t.Fatalf("component repository path annotation lost: %#v", repo.Annotations)
	}
	if repo.ExternalURL != "/api/v1/environments/1/services/3/proxy/paap/billing-dev-components" {
		t.Fatalf("external URL = %q", repo.ExternalURL)
	}
	if repo.Annotations["cloneURL"] != "ssh://git@gitea/paap/billing-dev-components.git" {
		t.Fatalf("cloneURL annotation = %#v", repo.Annotations)
	}
	if len(repo.Actions) != 1 || repo.Actions[0].Target != "api" {
		t.Fatalf("component action target must be preserved, actions = %#v", repo.Actions)
	}
}

func TestEnrichGiteaWorkspaceDoesNotFetchRepositoryTreesOnInitialLoad(t *testing.T) {
	previous := newGiteaWorkspaceClient
	client := &slowGiteaWorkspaceClient{}
	t.Cleanup(func() {
		newGiteaWorkspaceClient = previous
		clearGiteaWorkspaceCache()
	})
	clearGiteaWorkspaceCache()
	newGiteaWorkspaceClient = func(namespace string) giteaWorkspaceClient {
		return client
	}

	workspace := svcservice.BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "dev"},
		model.ServiceInstallation{ServiceType: "git", Namespace: "billing-dev-git", Status: "running"},
		nil,
	)

	start := time.Now()
	enriched := enrichGiteaWorkspace(t.Context(), workspace, model.ServiceInstallation{ID: 3, EnvironmentID: 1, ServiceType: "git", Namespace: "billing-dev-git"})
	elapsed := time.Since(start)

	if len(enriched.Resources) != 2 {
		t.Fatalf("resources = %#v", enriched.Resources)
	}
	if atomic.LoadInt32(&client.maxActive) != 0 {
		t.Fatalf("initial repository list must not fetch contents or commits, max active = %d", client.maxActive)
	}
	if elapsed >= 80*time.Millisecond {
		t.Fatalf("enrichment took %s, expected repository-only load to stay below slow content fetch cost", elapsed)
	}
}

func TestEnrichGiteaWorkspaceUsesRecentRealTreeCache(t *testing.T) {
	previous := newGiteaWorkspaceClient
	client := &countingGiteaWorkspaceClient{}
	t.Cleanup(func() {
		newGiteaWorkspaceClient = previous
		clearGiteaWorkspaceCache()
	})
	clearGiteaWorkspaceCache()
	newGiteaWorkspaceClient = func(namespace string) giteaWorkspaceClient {
		return client
	}

	workspace := svcservice.BuildToolWorkspace(
		model.Application{Identifier: "billing"},
		model.Environment{Identifier: "dev"},
		model.ServiceInstallation{ServiceType: "git", Namespace: "billing-dev-git", Status: "running"},
		nil,
	)
	inst := model.ServiceInstallation{ID: 3, EnvironmentID: 1, ServiceType: "git", Namespace: "billing-dev-git"}

	first := enrichGiteaWorkspace(t.Context(), workspace, inst)
	second := enrichGiteaWorkspace(t.Context(), workspace, inst)

	if len(first.Resources) != 1 || len(second.Resources) != 1 {
		t.Fatalf("resources = %#v / %#v", first.Resources, second.Resources)
	}
	if got := atomic.LoadInt32(&client.repositoriesCalls); got != 1 {
		t.Fatalf("repositories calls = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&client.contentsCalls); got != 0 {
		t.Fatalf("contents calls = %d, want no tree fetch on initial workspace load", got)
	}
}

func TestServiceProxyURLUsesBrowserReachableSameOriginPath(t *testing.T) {
	inst := model.ServiceInstallation{ID: 9, EnvironmentID: 1}

	if got := serviceProxyURL(inst, "http://grafana.monitor.svc.cluster.local:3000/d/env?orgId=1"); got != "/api/v1/environments/1/services/9/proxy/d/env?orgId=1" {
		t.Fatalf("absolute proxy URL = %q", got)
	}
	if got := serviceProxyURL(inst, "/job/api-build"); got != "/api/v1/environments/1/services/9/proxy/job/api-build" {
		t.Fatalf("relative proxy URL = %q", got)
	}
	if got := serviceProxyURL(model.ServiceInstallation{}, "/d/env"); got != "" {
		t.Fatalf("missing identifiers should not create proxy URL, got %q", got)
	}
}

func TestComponentProxyURLUsesBrowserReachableSameOriginPath(t *testing.T) {
	comp := model.Component{ID: 14}

	if got := componentProxyURL(1, comp, "/"); got != "/api/v1/environments/1/components/14/proxy/" {
		t.Fatalf("root component proxy URL = %q", got)
	}
	if got := componentProxyURL(1, comp, "/assets/app.js"); got != "/api/v1/environments/1/components/14/proxy/assets/app.js" {
		t.Fatalf("nested component proxy URL = %q", got)
	}
	if got := componentProxyURL(0, comp, "/"); got != "" {
		t.Fatalf("missing environment id should not create proxy URL, got %q", got)
	}
	if got := componentProxyURL(1, model.Component{}, "/"); got != "" {
		t.Fatalf("missing component id should not create proxy URL, got %q", got)
	}
}

func TestComponentExternalURLDoesNotExposeProxyForDraftWithoutRuntime(t *testing.T) {
	comp := model.Component{ID: 14, Name: "web", Type: "frontend"}

	if got := componentExternalURL(1, comp, "web", nil, nil); got != "" {
		t.Fatalf("draft component external URL = %q, want empty", got)
	}

	got := componentExternalURL(1, comp, "web", nil, &k8s.RuntimeConfig{
		Namespace:    "test-staging",
		WorkloadKind: "Deployment",
		WorkloadName: "web",
	})
	if got != "/api/v1/environments/1/components/14/proxy/" {
		t.Fatalf("runtime component proxy URL = %q", got)
	}
}

func TestComponentExternalURLPrefersIngressBeforeNodePort(t *testing.T) {
	comp := model.Component{ID: 14, Name: "web", Type: "frontend"}
	access := []EnvironmentExternalAccess{
		{Name: "web", Kind: "NodePort", URL: "http://172.18.0.2:30080", Scope: "environment"},
		{Name: "web", Kind: "Ingress", URL: "https://web.example.test", Scope: "environment"},
	}

	if got := componentExternalURL(1, comp, "web", access, nil); got != "https://web.example.test" {
		t.Fatalf("component external URL = %q, want ingress", got)
	}
}

func TestPrepareToolProxyRequestRewritesGrafanaLiveOrigin(t *testing.T) {
	target, err := url.Parse("http://test-staging-monitor-grafana.test-staging-monitor.svc.cluster.local")
	if err != nil {
		t.Fatalf("parse target: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9090/api/v1/environments/1/services/9/proxy/api/live/ws", nil)
	req.Header.Set("Origin", "http://127.0.0.1:9090")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	prepareToolProxyRequest(req, target, "/api/v1/environments/1/services/9/proxy", "api/live/ws", model.ServiceInstallation{
		ServiceType: "monitor",
		Namespace:   "test-staging-monitor",
	})

	if got, want := req.Header.Get("Origin"), "http://test-staging-monitor-grafana.test-staging-monitor.svc.cluster.local"; got != want {
		t.Fatalf("grafana live origin = %q, want %q", got, want)
	}
	if got := req.Header.Get("Host"); got != "" {
		t.Fatalf("request Host header must not be set as a regular header, got %q", got)
	}
	if got, want := req.Host, "test-staging-monitor-grafana.test-staging-monitor.svc.cluster.local"; got != want {
		t.Fatalf("request host = %q, want %q", got, want)
	}
}

func TestLokiSubjectIdentityPrefersPodOverApp(t *testing.T) {
	name, kind, namespace := lokiSubjectIdentity("shop-dev-log", map[string]string{
		"namespace": "shop-dev",
		"app":       "api",
		"pod":       "api-6f4d7c9c8d-rx2zs",
	})

	if name != "api-6f4d7c9c8d-rx2zs" || kind != "pod" || namespace != "shop-dev" {
		t.Fatalf("subject identity = (%q, %q, %q), want pod identity", name, kind, namespace)
	}
}

func TestMonitorSubjectKindRecognizesToolNamespacesFromKubePrometheusStack(t *testing.T) {
	monitorNamespace := "shop-dev-kube-prometheus-stack"

	cases := map[string]string{
		"shop-dev":                       "component",
		"shop-dev-git":                   "tool",
		"shop-dev-gitea":                 "tool",
		"shop-dev-deploy":                "tool",
		"shop-dev-argocd":                "tool",
		"shop-dev-monitor":               "tool",
		"shop-dev-kube-prometheus-stack": "tool",
		"shop-dev-postgresql":            "middleware",
		"shop-dev-redis":                 "middleware",
		"shop-dev-rabbitmq":              "middleware",
		"shop-dev-random-tool":           "middleware",
	}
	for namespace, want := range cases {
		if got := monitorSubjectKindForNamespace(monitorNamespace, namespace); got != want {
			t.Fatalf("namespace %s classified as %s, want %s", namespace, got, want)
		}
	}
}

func TestLogQueryForPodUsesExactPodLabel(t *testing.T) {
	got := logQueryForSubject("pod", "shop-dev", "api-6f4d7c9c8d-rx2zs")
	want := `{namespace="shop-dev", pod="api-6f4d7c9c8d-rx2zs"}`
	if got != want {
		t.Fatalf("pod log query = %q, want %q", got, want)
	}
}

func TestKubernetesPodLogSubjectsFillPodsMissingFromLoki(t *testing.T) {
	previous := k8s.GetClient()
	t.Cleanup(func() { k8s.SetClient(previous) })
	k8s.SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-6f4d7c9c8d-rx2zs", Namespace: "shop-dev"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
	).Build())

	subjects := kubernetesPodLogSubjects(t.Context(), []string{"shop-dev"})
	if len(subjects) != 1 {
		t.Fatalf("expected one pod subject, got %#v", subjects)
	}
	subject := subjects[0]
	if subject.Name != "api-6f4d7c9c8d-rx2zs" || subject.Status != "Empty" {
		t.Fatalf("unexpected pod subject: %#v", subject)
	}
	if subject.Annotations["logQuery"] != `{namespace="shop-dev", pod="api-6f4d7c9c8d-rx2zs"}` {
		t.Fatalf("unexpected log query: %#v", subject.Annotations)
	}

	merged := mergeKubernetesPodLogSubjects([]svcservice.ToolWorkspaceResource{subject}, subjects)
	if len(merged) != 1 {
		t.Fatalf("expected duplicate pod subject to be merged, got %#v", merged)
	}
}

func TestRewriteToolProxyHTMLPrefixesGiteaRootAssetsAndLinks(t *testing.T) {
	prefix := "/api/v1/environments/1/services/3/proxy"
	html := `<base href="/" /><link rel="stylesheet" href="/assets/css/index.css?v=1.23.8"><script src="/assets/js/index.js?v=1.23.8"></script><link rel="stylesheet" href="public/build/grafana.dark.css"><script src="public/build/app.js"></script><a href="/paap/test-staging-components">repo</a><form action="/user/login"></form><script>window.config = {assetUrlPrefix: '\/assets'}; window.grafanaBootData = {"settings":{"appUrl":"http://localhost:3000/","appSubUrl":"","datasources":{"Prometheus":{"url":"/api/datasources/proxy/uid/prometheus","meta":{"baseUrl":"public/app/plugins/datasource/prometheus"}}}},"assets":{"dark":"public/build/grafana.dark.css"}};</script>`

	rewritten := rewriteToolProxyHTML(html, prefix, false)

	for _, want := range []string{
		`<base href="/api/v1/environments/1/services/3/proxy/" />`,
		`href="/api/v1/environments/1/services/3/proxy/assets/css/index.css?v=1.23.8"`,
		`src="/api/v1/environments/1/services/3/proxy/assets/js/index.js?v=1.23.8"`,
		`href="/api/v1/environments/1/services/3/proxy/public/build/grafana.dark.css"`,
		`src="/api/v1/environments/1/services/3/proxy/public/build/app.js"`,
		`href="/api/v1/environments/1/services/3/proxy/paap/test-staging-components"`,
		`action="/api/v1/environments/1/services/3/proxy/user/login"`,
		`assetUrlPrefix: '\/api\/v1\/environments\/1\/services\/3\/proxy\/assets'`,
		`"dark":"/api/v1/environments/1/services/3/proxy/public/build/grafana.dark.css"`,
		`"appUrl":"/api/v1/environments/1/services/3/proxy/"`,
		`"appSubUrl":"/api/v1/environments/1/services/3/proxy"`,
		`"url":"/api/v1/environments/1/services/3/proxy/api/datasources/proxy/uid/prometheus"`,
		`"baseUrl":"/api/v1/environments/1/services/3/proxy/public/app/plugins/datasource/prometheus"`,
	} {
		if !strings.Contains(rewritten, want) {
			t.Fatalf("rewritten HTML missing %q:\n%s", want, rewritten)
		}
	}
	if strings.Contains(rewritten, `href="/assets/`) || strings.Contains(rewritten, `src="/assets/`) || strings.Contains(rewritten, `href="public/`) {
		t.Fatalf("rewritten HTML still contains unproxied root assets:\n%s", rewritten)
	}
}

func TestRewriteToolProxyHTMLCanInjectGrafanaEmbedChromeHider(t *testing.T) {
	prefix := "/api/v1/environments/1/services/9/proxy"
	html := `<html><head><title>Grafana</title></head><body><div id="reactRoot"></div></body></html>`

	plain := rewriteToolProxyHTML(html, prefix, false)
	if strings.Contains(plain, "paap-grafana-embed-style") {
		t.Fatalf("plain proxy HTML should not inject PAAP embed style:\n%s", plain)
	}

	embedded := rewriteToolProxyHTML(html, prefix, true)
	for _, want := range []string{
		`id="paap-grafana-embed-style"`,
		`[aria-label="Navigation"]`,
		`[data-testid*="sidemenu"]`,
		`[data-testid*="navigation mega-menu"]`,
		`paap-grafana-embed-script`,
	} {
		if !strings.Contains(embedded, want) {
			t.Fatalf("embedded HTML missing %q:\n%s", want, embedded)
		}
	}
	for _, forbidden := range []string{
		`[class*="page-panes"]`,
		`[class*="scrollbar"]`,
		`.main-view`,
		`.dashboard-container`,
		`[role="main"]`,
	} {
		if strings.Contains(embedded, forbidden) {
			t.Fatalf("embedded HTML should not override Grafana content layout selector %q:\n%s", forbidden, embedded)
		}
	}
	if strings.Count(injectPAAPGrafanaEmbedStyle(embedded), "paap-grafana-embed-style") != 1 {
		t.Fatalf("embed style should not be injected twice:\n%s", embedded)
	}
}

func TestDecorateLogSubjectsUsesGrafanaProxyForExplore(t *testing.T) {
	grafanaInst := model.ServiceInstallation{ID: 9, EnvironmentID: 1, ServiceType: "monitor"}
	resources := decorateLogSubjects([]svcservice.ToolWorkspaceResource{{
		Name:   "api-6f4d7c9c8d-rx2zs",
		Type:   "Log Subject",
		Status: "Ready",
		Annotations: map[string]interface{}{
			"subjectKind": "pod",
			"namespace":   "shop-dev",
			"selector":    "api-6f4d7c9c8d-rx2zs",
			"logQuery":    `{namespace="shop-dev", pod="api-6f4d7c9c8d-rx2zs"}`,
		},
	}}, grafanaInst)

	if len(resources) != 1 {
		t.Fatalf("expected one decorated subject, got %#v", resources)
	}
	got := resources[0].ExternalURL
	if !strings.Contains(got, "/api/v1/environments/1/services/9/proxy/explore?") {
		t.Fatalf("log subject should use Grafana proxy explore URL, got %q", got)
	}
	if strings.Contains(got, "/services/8/proxy/explore") {
		t.Fatalf("log subject must not point Explore at the Loki service proxy: %q", got)
	}
}

func TestMonitorSubjectsAsLogSubjectsKeepsLogQueries(t *testing.T) {
	resources := monitorSubjectsAsLogSubjects([]svcservice.ToolWorkspaceResource{{
		Name:   "api",
		Type:   "Monitor Subject",
		Status: "Ready",
		Annotations: map[string]interface{}{
			"subjectKind": "component",
			"namespace":   "shop-dev",
			"selector":    "api",
			"logQuery":    `{namespace="shop-dev", pod=~"api.*"}`,
		},
	}})
	resources = decorateLogSubjects(resources, model.ServiceInstallation{ID: 9, EnvironmentID: 1, ServiceType: "monitor"})

	if len(resources) != 1 || resources[0].Type != "Log Subject" {
		t.Fatalf("resources = %#v", resources)
	}
	if !strings.Contains(resources[0].ExternalURL, "/api/v1/environments/1/services/9/proxy/explore?") {
		t.Fatalf("log subject should use Grafana Explore proxy URL, got %q", resources[0].ExternalURL)
	}
	if resources[0].Annotations["logQuery"] != `{namespace="shop-dev", pod=~"api.*"}` {
		t.Fatalf("log query annotation changed: %#v", resources[0].Annotations)
	}
}

func TestGrafanaExploreLokiPathUsesTwentyFourHourRange(t *testing.T) {
	path := grafanaExploreLokiPath(`{namespace="shop-dev", pod="api"}`)
	parsed, err := url.Parse(path)
	if err != nil {
		t.Fatalf("parse path: %v", err)
	}
	left := parsed.Query().Get("left")
	if !strings.Contains(left, `"from":"now-24h"`) || !strings.Contains(left, `"to":"now"`) {
		t.Fatalf("explore path should use 24h range, got %s", left)
	}
}

func TestSortLogSubjectsForEnvironmentKeepsApplicationFirst(t *testing.T) {
	subjects := []svcservice.ToolWorkspaceResource{
		{Name: "loki", Type: "Log Subject", Annotations: map[string]interface{}{"subjectKind": "pod", "namespace": "shop-dev-log", "entryCount": 40}},
		{Name: "api-7d9f", Type: "Log Subject", Annotations: map[string]interface{}{"subjectKind": "pod", "namespace": "shop-dev", "entryCount": 0}},
		{Name: "api", Type: "Log Subject", Annotations: map[string]interface{}{"subjectKind": "component", "namespace": "shop-dev", "entryCount": 1}},
	}

	sorted := sortLogSubjectsForEnvironment(subjects, "shop-dev")

	if sorted[0].Name != "api" || sorted[1].Name != "api-7d9f" {
		t.Fatalf("application log subjects should stay before tool logs, got %#v", sorted)
	}
}

func TestInstalledServiceMonitorSubjectsExposeServiceID(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Shop", Identifier: "shop", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	redis := model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "redis", ServiceName: "redis", Namespace: "shop-dev-redis", Status: "running"}
	if err := db.Create(&redis).Error; err != nil {
		t.Fatalf("create redis install: %v", err)
	}

	subjects := installedServiceMonitorSubjects(app, env, model.ServiceInstallation{ID: 9, EnvironmentID: env.ID, ServiceType: "monitor", Namespace: "shop-dev-monitor"})

	var redisSubject *svcservice.ToolWorkspaceResource
	for i := range subjects {
		if subjects[i].Annotations["serviceType"] == "redis" {
			redisSubject = &subjects[i]
			break
		}
	}
	if redisSubject == nil {
		t.Fatalf("redis monitor subject not found: %#v", subjects)
	}
	if got := redisSubject.Annotations["serviceId"]; got != redis.ID {
		t.Fatalf("serviceId annotation = %#v, want %d", got, redis.ID)
	}
}

func TestRewriteGrafanaLokiLogsVolumeBodyRemovesUnsupportedDropStage(t *testing.T) {
	body := []byte(`{"queries":[{"refId":"log-volume-A","expr":"sum by (level, detected_level) (count_over_time({namespace=\"test-staging\", pod=\"api\"} | drop __error__[$__auto]))","queryType":"range","supportingQueryType":"logsVolume"}],"from":"1","to":"2"}`)

	rewritten := string(rewriteGrafanaLokiLogsVolumeBody(body))
	if strings.Contains(rewritten, "| drop __error__") {
		t.Fatalf("logs volume body should remove unsupported drop stage: %s", rewritten)
	}
	if !strings.Contains(rewritten, `[$__auto]`) {
		t.Fatalf("logs volume body should preserve range selector: %s", rewritten)
	}

	normalQuery := []byte(`{"queries":[{"expr":"{namespace=\"test-staging\"} | drop __error__","queryType":"range"}]}`)
	if got := string(rewriteGrafanaLokiLogsVolumeBody(normalQuery)); got != string(normalQuery) {
		t.Fatalf("non logs-volume query should not be rewritten: %s", got)
	}

	spacedBody := []byte(`{"queries":[{"refId":"log-volume-A","expr":"sum(count_over_time({namespace=\"test-staging\"} | drop __error__[$__auto]))","supportingQueryType": "logsVolume"}]}`)
	if got := string(rewriteGrafanaLokiLogsVolumeBody(spacedBody)); strings.Contains(got, "| drop __error__") {
		t.Fatalf("logs volume body with spaced JSON should remove unsupported drop stage: %s", got)
	}
}

func TestToolHTTPBaseURLUsesInstalledReleaseServiceNames(t *testing.T) {
	cases := []struct {
		serviceType string
		namespace   string
		want        string
	}{
		{serviceType: "git", namespace: "test-staging-git", want: "http://test-staging-git.test-staging-git.svc.cluster.local:3000"},
		{serviceType: "deploy", namespace: "test-staging-deploy", want: "http://test-staging-deploy-argocd-server.test-staging-deploy.svc.cluster.local"},
		{serviceType: "monitor", namespace: "test-staging-monitor", want: "http://test-staging-monitor-grafana.test-staging-monitor.svc.cluster.local"},
		{serviceType: "log", namespace: "test-staging-log", want: "http://test-staging-log-loki.test-staging-log.svc.cluster.local:3100"},
		{serviceType: "log", namespace: "test-staging-loki", want: "http://test-staging-loki.test-staging-loki.svc.cluster.local:3100"},
		{serviceType: "ci", namespace: "test-staging-ci", want: "http://test-staging-ci.test-staging-ci.svc.cluster.local:8080"},
		{serviceType: "registry", namespace: "test-staging-registry", want: "https://test-staging-registry.test-staging-registry.svc.cluster.local:5000"},
	}
	for _, tc := range cases {
		got := toolHTTPBaseURL(model.ServiceInstallation{ServiceType: tc.serviceType, Namespace: tc.namespace})
		if got != tc.want {
			t.Fatalf("%s base URL = %q, want %q", tc.serviceType, got, tc.want)
		}
	}
}

func TestServiceSelectorForDashboardUsesReleasePrefixForArgoCD(t *testing.T) {
	inst := model.ServiceInstallation{
		ServiceType: "deploy",
		Namespace:   "shop-dev-argocd",
		ReleaseName: "shop-dev-argocd",
	}
	if got := serviceSelectorForDashboard(inst); got != "shop-dev-argocd" {
		t.Fatalf("argocd dashboard selector = %q, want release prefix", got)
	}
}

func TestDatabaseTableTargetRoundTrips(t *testing.T) {
	target := databaseTableTarget("appdb", "public.orders")
	databaseName, tableName, ok := parseDatabaseTableTarget(target)
	if !ok || databaseName != "appdb" || tableName != "public.orders" {
		t.Fatalf("unexpected target parse: %q %q %v", databaseName, tableName, ok)
	}
	if _, _, ok := parseDatabaseTableTarget("missing-separator"); ok {
		t.Fatalf("expected invalid target")
	}
}

func TestBuildHelmInstallSpecUsesRefreshedManifestMappings(t *testing.T) {
	refreshedManifest := model.PlatformManifest{
		Name:    "loki",
		Version: "v2",
		Permissions: model.PermissionsSpec{
			EnvironmentNamespaces: model.NamespacePermissionsSpec{
				Rules: []model.PolicyRuleSpec{
					{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch"}},
				},
			},
		},
		VariableMapping: []model.VariableMappingEntry{
			{PlatformVar: "env_namespaces", HelmVar: "promtail.config.paap.envNamespaces"},
		},
	}
	refreshedJSON, err := json.Marshal(refreshedManifest)
	if err != nil {
		t.Fatalf("marshal refreshed manifest: %v", err)
	}
	template := model.ServiceTemplate{
		Type:                 "log",
		Name:                 "Loki+Promtail (官方)",
		Category:             "tool",
		Installer:            "helm",
		S3Bucket:             "paap-charts",
		S3Key:                "charts/loki.tar.gz",
		PlatformManifestJSON: string(refreshedJSON),
		Enabled:              true,
	}
	app := model.Application{Identifier: "test"}
	env := model.Environment{Identifier: "staging"}

	spec := buildHelmInstallSpec(&app, &env, &template, "log")
	if spec.PlatformManifest != string(refreshedJSON) {
		t.Fatalf("platform manifest not refreshed: got %q want %q", spec.PlatformManifest, string(refreshedJSON))
	}
	if got := spec.Values["promtail.config.paap.envNamespaces"]; got != "test-staging,test-staging-app" {
		t.Fatalf("env namespaces not refreshed: got %q", got)
	}
}

func TestCreateComponentRejectsImplicitLatestBeforeCreatingRecord(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "测试环境", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	body, _ := json.Marshal(CreateComponentRequest{
		Name:     "订单服务",
		Type:     "backend",
		Image:    "registry.local:5000/order:latest",
		Version:  "v1.0.0",
		Replicas: 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/components", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/components", CreateComponent)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var count int64
	if err := db.Model(&model.Component{}).Count(&count).Error; err != nil {
		t.Fatalf("count components: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no component to be created, got %d", count)
	}
}

func TestCreateComponentCreatesDraftWithoutDeploying(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
	})

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db
	k8s.SetClient(nil)

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "测试环境", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	body, _ := json.Marshal(CreateComponentRequest{
		Name:         "订单服务",
		Type:         "backend",
		Image:        "registry.local:5000/order:v1.0.0",
		DeliveryMode: "image",
		Replicas:     2,
		CPU:          "250m",
		Memory:       "256Mi",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/components", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/components", CreateComponent)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var response struct {
		Data model.Component `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != "draft" {
		t.Fatalf("component status = %q, want draft", response.Data.Status)
	}
	if response.Data.ArgoCDApp != "" || response.Data.GitPath != "" || response.Data.GitRepoURL != "" {
		t.Fatalf("draft component must not create GitOps metadata before deploy: %#v", response.Data)
	}
	var count int64
	if err := db.Model(&model.Component{}).Count(&count).Error; err != nil {
		t.Fatalf("count components: %v", err)
	}
	if count != 1 {
		t.Fatalf("component records = %d, want 1", count)
	}
}

func TestCreateComponentAllowsCanvasDraftWithoutImage(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "测试环境", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	body, _ := json.Marshal(CreateComponentRequest{
		Name:      "画布组件",
		Type:      "backend",
		DraftOnly: true,
		Replicas:  1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/components", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/v1/environments/:id/components", CreateComponent)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var response struct {
		Data model.Component `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != "draft" || response.Data.Image != "" || response.Data.Version != "" {
		t.Fatalf("unexpected draft component: %#v", response.Data)
	}
}

func TestEnvironmentCanvasStatePersistsPositionsAndEdges(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.EnvironmentCanvasState{}, &model.Component{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "测试环境", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.Component{ID: 1, EnvironmentID: env.ID, Name: "api", Type: "backend"}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{ID: 2, EnvironmentID: env.ID, ServiceType: "postgresql", ServiceName: "db"}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	body := []byte(`{
		"positions": {
			"component:1": {"x": 120, "y": 88},
			"component:999": {"x": 999, "y": 999},
			"service:2": {"x": 360, "y": 144},
			"service:999": {"x": 999, "y": 999}
		},
		"edges": [
			{"fromKey":"component:1","toKey":"service:2"},
			{"fromKey":"component:1","toKey":"service:2"},
			{"fromKey":"component:999","toKey":"service:2"},
			{"fromKey":"component:1","toKey":"component:1"}
		]
	}`)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/canvas-state", withTestAuthUserRole(1, "admin", SaveEnvironmentCanvasState))
	router.GET("/api/v1/environments/:id/canvas-state", withTestAuthUserRole(1, "admin", GetEnvironmentCanvasState))

	saveReq := httptest.NewRequest(http.MethodPut, "/api/v1/environments/1/canvas-state", bytes.NewReader(body))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	router.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d; body=%s", saveRec.Code, http.StatusOK, saveRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/canvas-state", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var response struct {
		Data struct {
			Positions map[string]CanvasNodePosition `json:"positions"`
			Edges     []CanvasManualEdge            `json:"edges"`
		} `json:"data"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := response.Data.Positions["component:1"]; got.X != 120 || got.Y != 88 {
		t.Fatalf("component position not persisted: %#v", got)
	}
	if _, ok := response.Data.Positions["component:999"]; ok {
		t.Fatalf("orphan component position should be filtered: %#v", response.Data.Positions)
	}
	if _, ok := response.Data.Positions["service:999"]; ok {
		t.Fatalf("orphan service position should be filtered: %#v", response.Data.Positions)
	}
	if len(response.Data.Edges) != 1 || response.Data.Edges[0].FromKey != "component:1" || response.Data.Edges[0].ToKey != "service:2" {
		t.Fatalf("edges not normalized and persisted: %#v", response.Data.Edges)
	}
}

func TestEnvironmentCanvasStatePersistsDisplayNames(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.EnvironmentCanvasState{}, &model.Component{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "测试环境", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.Component{ID: 1, EnvironmentID: env.ID, Name: "api", Type: "backend"}).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{ID: 2, EnvironmentID: env.ID, ServiceType: "redis", ServiceName: "cache"}).Error; err != nil {
		t.Fatalf("create service: %v", err)
	}

	body := []byte(`{
		"positions": {"component:1": {"x": 100, "y": 80}},
		"edges": [],
		"names": {
			"component:1": "用户前端",
			"service:2": "主缓存",
			"  ": "ignored",
			"component:999": "ghost"
		}
	}`)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/canvas-state", withTestAuthUserRole(1, "admin", SaveEnvironmentCanvasState))
	router.GET("/api/v1/environments/:id/canvas-state", withTestAuthUserRole(1, "admin", GetEnvironmentCanvasState))

	saveReq := httptest.NewRequest(http.MethodPut, "/api/v1/environments/1/canvas-state", bytes.NewReader(body))
	saveReq.Header.Set("Content-Type", "application/json")
	saveRec := httptest.NewRecorder()
	router.ServeHTTP(saveRec, saveReq)
	if saveRec.Code != http.StatusOK {
		t.Fatalf("save status = %d, want %d; body=%s", saveRec.Code, http.StatusOK, saveRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/canvas-state", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}
	var response struct {
		Data struct {
			Names map[string]string `json:"names"`
		} `json:"data"`
	}
	if err := json.Unmarshal(getRec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Names["component:1"] != "用户前端" {
		t.Fatalf("component display name not persisted: %q", response.Data.Names["component:1"])
	}
	if response.Data.Names["service:2"] != "主缓存" {
		t.Fatalf("service display name not persisted: %q", response.Data.Names["service:2"])
	}
	if _, ok := response.Data.Names[" "]; ok {
		t.Fatalf("whitespace-only key should be filtered: %#v", response.Data.Names)
	}
	if _, ok := response.Data.Names["component:999"]; !ok {
		t.Fatalf("display names are not cleaned by valid keys (unlike positions): %#v", response.Data.Names)
	}
}

func TestGetEnvironmentCanvasStateRejectsNonMembers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.Environment{}, &model.EnvironmentCanvasState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "hidden-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.EnvironmentCanvasState{EnvironmentID: env.ID, Positions: `{"component:1":{"x":10,"y":20}}`, Edges: `[]`, Names: `{"component:1":"隐藏组件"}`}).Error; err != nil {
		t.Fatalf("create canvas state: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/canvas-state", nil)
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/environments/:id/canvas-state", withTestAuthUser(2, GetEnvironmentCanvasState))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestSaveEnvironmentCanvasStateRejectsNonMembers(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.AppMember{}, &model.Environment{}, &model.EnvironmentCanvasState{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "隐藏应用", Identifier: "hidden", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "生产", Identifier: "prod", Namespace: "hidden-prod"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}

	body := []byte(`{"positions":{"component:1":{"x":10,"y":20}},"edges":[],"names":{"component:1":"被改名"}}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/environments/1/canvas-state", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/environments/:id/canvas-state", withTestAuthUser(2, SaveEnvironmentCanvasState))
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	var count int64
	if err := db.Model(&model.EnvironmentCanvasState{}).Where("environment_id = ?", env.ID).Count(&count).Error; err != nil {
		t.Fatalf("count canvas state: %v", err)
	}
	if count != 0 {
		t.Fatalf("canvas state count = %d, want none created", count)
	}
}

func TestUpdateComponentPersistsRuntimeConfig(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "测试环境", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	comp := model.Component{
		EnvironmentID: env.ID,
		Name:          "orders",
		Type:          "backend",
		Image:         "registry.local/orders:v1",
		Version:       "v1",
		Replicas:      1,
		Status:        "draft",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	body := []byte(`{
		"replicas": 2,
		"cpu": "250m",
		"memory": "256Mi",
		"config": {
			"env": [
				{"name":"DATABASE_URL","value":"postgres://orders"},
				{"name":"DB_PASSWORD","secretName":"orders-db","secretKey":"password"},
				{"name":"REDIS_HOST","configMapName":"redis-config","configMapKey":"host"}
			],
			"dependencies": ["redis"]
		}
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/components/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/components/:id", UpdateComponent)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var saved model.Component
	if err := db.First(&saved, comp.ID).Error; err != nil {
		t.Fatalf("load component: %v", err)
	}
	if saved.Status != "draft" {
		t.Fatalf("update must not deploy component, status = %q", saved.Status)
	}
	if saved.Replicas != 2 || saved.CPU != "250m" || saved.Memory != "256Mi" {
		t.Fatalf("runtime sizing not persisted: %#v", saved)
	}
	cfg, err := model.ParseComponentConfig(saved.Config)
	if err != nil {
		t.Fatalf("parse saved config: %v", err)
	}
	if len(cfg.Env) != 3 {
		t.Fatalf("env count = %d, want 3", len(cfg.Env))
	}
	if cfg.Env[1].SecretName != "orders-db" || cfg.Env[1].SecretKey != "password" {
		t.Fatalf("secret env not persisted: %#v", cfg.Env[1])
	}
	if cfg.Env[2].ConfigMapName != "redis-config" || cfg.Env[2].ConfigMapKey != "host" {
		t.Fatalf("configmap env not persisted: %#v", cfg.Env[2])
	}
	if len(cfg.Dependencies) != 1 || cfg.Dependencies[0] != "redis" {
		t.Fatalf("dependencies not persisted: %#v", cfg.Dependencies)
	}
}

func TestUpdateComponentKeepsRegistryImageInSyncForImageDelivery(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "测试环境", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	comp := model.Component{
		EnvironmentID:  env.ID,
		Name:           "orders",
		Type:           "backend",
		Image:          "paap-real-backend:v1",
		RegistryImage:  "paap-real-backend:v1",
		Version:        "v1",
		DeliveryMode:   "image",
		PipelineStatus: "built",
		Replicas:       1,
		Status:         "running",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	body := []byte(`{"image":"paap-real-backend:v2","version":"v2","replicas":1}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/components/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/components/:id", UpdateComponent)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var saved model.Component
	if err := db.First(&saved, comp.ID).Error; err != nil {
		t.Fatalf("load component: %v", err)
	}
	if saved.Image != "paap-real-backend:v2" || saved.RegistryImage != "paap-real-backend:v2" || saved.Version != "v2" {
		t.Fatalf("image fields not synced: image=%q registry=%q version=%q", saved.Image, saved.RegistryImage, saved.Version)
	}
}

func TestUpdateComponentCanSwitchDraftToSourceDelivery(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Shop", Identifier: "shop", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Status: "running", Namespace: "shop-dev"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	comp := model.Component{
		EnvironmentID: env.ID,
		Name:          "orders-api",
		Type:          "backend",
		Image:         "registry.local/orders:v1",
		RegistryImage: "registry.local/orders:v1",
		Version:       "v1",
		DeliveryMode:  "image",
		Replicas:      1,
		Status:        "draft",
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	body := []byte(`{
		"deliveryMode": "source",
		"sourceRepoUrl": "https://git.example.com/shop/orders.git",
		"sourceBranch": "main",
		"buildContext": "services/orders",
		"version": "v2",
		"replicas": 1
	}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/components/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/components/:id", UpdateComponent)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var saved model.Component
	if err := db.First(&saved, comp.ID).Error; err != nil {
		t.Fatalf("load component: %v", err)
	}
	if saved.DeliveryMode != "source" || saved.SourceRepoURL != "https://git.example.com/shop/orders.git" || saved.SourceBranch != "main" || saved.BuildContext != "services/orders" {
		t.Fatalf("source delivery fields not persisted: %#v", saved)
	}
	if saved.Image != "" || saved.RegistryImage != "" {
		t.Fatalf("source delivery draft must clear image fields before deployment, image=%q registry=%q", saved.Image, saved.RegistryImage)
	}
	if saved.JenkinsJob != "shop-dev-orders-api-build" || saved.PipelineStatus != "planned" {
		t.Fatalf("source delivery pipeline fields not planned: job=%q status=%q", saved.JenkinsJob, saved.PipelineStatus)
	}
	if saved.Version != "v2" {
		t.Fatalf("version = %q, want v2", saved.Version)
	}
}

func TestUpdateComponentKeepsExistingConfigWhenConfigOmitted(t *testing.T) {
	previousDB := database.DB
	t.Cleanup(func() { database.DB = previousDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "测试应用", Identifier: "test", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "测试环境", Identifier: "staging", Status: "running", Namespace: "test-staging"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	existingConfig, err := model.ComponentConfig{
		Env: []model.ComponentEnvVar{{Name: "FEATURE_FLAG", Value: "on"}},
	}.JSON()
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	comp := model.Component{
		EnvironmentID: env.ID,
		Name:          "orders",
		Type:          "backend",
		Image:         "registry.local/orders:v1",
		Version:       "v1",
		Replicas:      1,
		Status:        "draft",
		Config:        existingConfig,
	}
	if err := db.Create(&comp).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/components/1", strings.NewReader(`{"replicas":3}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.PUT("/api/v1/components/:id", UpdateComponent)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var saved model.Component
	if err := db.First(&saved, comp.ID).Error; err != nil {
		t.Fatalf("load component: %v", err)
	}
	if saved.Config != existingConfig {
		t.Fatalf("config changed when omitted: got %q want %q", saved.Config, existingConfig)
	}
	if saved.Replicas != 3 {
		t.Fatalf("replicas = %d, want 3", saved.Replicas)
	}
}

func TestPlanComponentDeliveryBuildsSourceToGitOpsFlow(t *testing.T) {
	app := model.Application{Identifier: "shop"}
	env := model.Environment{Identifier: "dev"}
	req := CreateComponentRequest{
		Name:           "Orders API",
		Type:           "backend",
		Version:        "v1.2.3",
		DeliveryMode:   "source",
		SourceRepoURL:  "https://git.example.com/team/orders.git",
		SourceBranch:   "",
		BuildContext:   "",
		DockerfilePath: "",
	}
	comp := model.Component{
		ID:       42,
		Name:     req.Name,
		Type:     req.Type,
		Version:  req.Version,
		Replicas: 1,
	}

	planned, err := planComponentDelivery(app, env, req, comp, "shop-dev")
	if err != nil {
		t.Fatalf("plan delivery: %v", err)
	}

	if planned.DeliveryMode != "source" {
		t.Fatalf("delivery mode = %q, want source", planned.DeliveryMode)
	}
	if planned.SourceRepoURL != req.SourceRepoURL {
		t.Fatalf("source repo = %q, want %q", planned.SourceRepoURL, req.SourceRepoURL)
	}
	if planned.SourceMirrorRepoURL != "http://shop-dev-git.shop-dev-git.svc.cluster.local:3000/paap/shop-dev-orders-api-source.git" {
		t.Fatalf("source mirror repo = %q", planned.SourceMirrorRepoURL)
	}
	if planned.SourceBranch != "main" {
		t.Fatalf("source branch = %q, want main", planned.SourceBranch)
	}
	if planned.BuildContext != "." || planned.DockerfilePath != "" {
		t.Fatalf("unexpected build inputs: context=%q dockerfile=%q", planned.BuildContext, planned.DockerfilePath)
	}
	if planned.JenkinsJob != "shop-dev-orders-api-build" {
		t.Fatalf("jenkins job = %q, want shop-dev-orders-api-build", planned.JenkinsJob)
	}
	if planned.RegistryImage != "registry.shop-dev.paap.local/shop-dev/orders-api:v1.2.3" {
		t.Fatalf("registry image = %q", planned.RegistryImage)
	}
	if planned.Image != planned.RegistryImage {
		t.Fatalf("component image = %q, want registry image %q", planned.Image, planned.RegistryImage)
	}
	if planned.GitRepoURL != "http://shop-dev-git.shop-dev-git.svc.cluster.local:3000/paap/shop-dev-components.git" {
		t.Fatalf("git repo URL = %q", planned.GitRepoURL)
	}
	if planned.GitPath != "components/orders-api" {
		t.Fatalf("git path = %q, want components/orders-api", planned.GitPath)
	}
	if planned.ArgoCDApp != "shop-dev-orders-api" {
		t.Fatalf("argocd app = %q, want shop-dev-orders-api", planned.ArgoCDApp)
	}
	if planned.PipelineStatus != "planned" {
		t.Fatalf("pipeline status = %q, want planned", planned.PipelineStatus)
	}
}

func TestSourceDeliveryAllowsOmittedVersionForJenkinsBuildNumber(t *testing.T) {
	req := CreateComponentRequest{
		Name:          "Orders API",
		Type:          "backend",
		DeliveryMode:  "source",
		SourceRepoURL: "https://git.example.com/team/orders.git",
	}
	if err := validateComponentDeliveryRequest(req); err != nil {
		t.Fatalf("source delivery should allow omitted version: %v", err)
	}

	planned, err := planComponentDelivery(
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		req,
		model.Component{ID: 42, Name: req.Name, Type: req.Type, Replicas: 1},
		"shop-dev",
	)
	if err != nil {
		t.Fatalf("plan delivery: %v", err)
	}
	if planned.Version != "manual" {
		t.Fatalf("version = %q, want manual fallback for first Jenkins build", planned.Version)
	}
	if planned.RegistryImage != "registry.shop-dev.paap.local/shop-dev/orders-api:manual" {
		t.Fatalf("registry image = %q", planned.RegistryImage)
	}
	if planned.PipelineStatus != "planned" {
		t.Fatalf("pipeline status = %q, want planned", planned.PipelineStatus)
	}
}

func TestPlanComponentDeliveryUsesSelectedHarborRegistry(t *testing.T) {
	t.Setenv(svcservice.RegistryHostTemplateEnv, "{service}-{app}-{env}.corp.example.com:5443")
	req := CreateComponentRequest{
		Name:          "Orders API",
		Type:          "backend",
		DeliveryMode:  "source",
		SourceRepoURL: "https://git.example.com/team/orders.git",
		Version:       "v1.2.3",
	}

	planned, err := planComponentDelivery(
		model.Application{Identifier: "shop"},
		model.Environment{Identifier: "dev"},
		req,
		model.Component{ID: 42, Name: req.Name, Type: req.Type, Replicas: 1, Version: req.Version},
		"shop-dev",
		"harbor",
	)
	if err != nil {
		t.Fatalf("plan delivery: %v", err)
	}
	if planned.RegistryImage != "harbor-shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3" {
		t.Fatalf("registry image = %q", planned.RegistryImage)
	}
}

func TestPreferredSourceRegistryServiceTypePrefersRunningHarbor(t *testing.T) {
	services := []model.ServiceInstallation{
		{ServiceType: "registry", Status: "running"},
		{ServiceType: "harbor", Status: "running"},
	}

	if got := preferredSourceRegistryServiceType(services); got != "harbor" {
		t.Fatalf("registry service type = %q, want harbor", got)
	}
}

func TestEnsureSourceHarborProjectUsesSelectedHarborInstance(t *testing.T) {
	previous := newHarborProjectEnsurer
	t.Cleanup(func() { newHarborProjectEnsurer = previous })

	var namespaces []string
	var projects []string
	newHarborProjectEnsurer = func(namespace string) harborProjectEnsurer {
		namespaces = append(namespaces, namespace)
		return fakeHarborProjectEnsurer{projects: &projects}
	}

	services := []model.ServiceInstallation{
		{ServiceType: "registry", Status: "running", Namespace: "shop-dev-registry"},
		{ServiceType: "harbor", Status: "running", Namespace: "shop-dev-harbor"},
	}
	comp := model.Component{DeliveryMode: "source"}

	if err := ensureSourceHarborProject(t.Context(), comp, "shop-dev", services); err != nil {
		t.Fatalf("ensure harbor project: %v", err)
	}
	if len(namespaces) != 1 || namespaces[0] != "shop-dev-harbor" {
		t.Fatalf("harbor namespaces = %#v, want shop-dev-harbor", namespaces)
	}
	if len(projects) != 1 || projects[0] != "shop-dev" {
		t.Fatalf("harbor projects = %#v, want shop-dev", projects)
	}
}

func TestEnsureSourceHarborProjectSkipsImageAndRegistryOnlyFlows(t *testing.T) {
	previous := newHarborProjectEnsurer
	t.Cleanup(func() { newHarborProjectEnsurer = previous })

	var calls int
	newHarborProjectEnsurer = func(namespace string) harborProjectEnsurer {
		calls++
		return fakeHarborProjectEnsurer{projects: &[]string{}}
	}

	registryOnly := []model.ServiceInstallation{
		{ServiceType: "registry", Status: "running", Namespace: "shop-dev-registry"},
	}
	if err := ensureSourceHarborProject(t.Context(), model.Component{DeliveryMode: "source"}, "shop-dev", registryOnly); err != nil {
		t.Fatalf("registry-only flow should not fail: %v", err)
	}
	if err := ensureSourceHarborProject(t.Context(), model.Component{DeliveryMode: "image"}, "shop-dev", []model.ServiceInstallation{
		{ServiceType: "harbor", Status: "running", Namespace: "shop-dev-harbor"},
	}); err != nil {
		t.Fatalf("image flow should not fail: %v", err)
	}
	if calls != 0 {
		t.Fatalf("harbor ensurer calls = %d, want 0", calls)
	}
}

func TestPlanComponentDeliveryKeepsImageFlow(t *testing.T) {
	app := model.Application{Identifier: "shop"}
	env := model.Environment{Identifier: "dev"}
	req := CreateComponentRequest{
		Name:         "Orders API",
		Type:         "backend",
		Image:        "registry.local/orders:v1.2.3",
		Version:      "v1.2.3",
		DeliveryMode: "",
	}
	comp := model.Component{ID: 42, Name: req.Name, Type: req.Type, Image: req.Image, Version: req.Version}

	planned, err := planComponentDelivery(app, env, req, comp, "shop-dev")
	if err != nil {
		t.Fatalf("plan delivery: %v", err)
	}

	if planned.DeliveryMode != "image" {
		t.Fatalf("delivery mode = %q, want image", planned.DeliveryMode)
	}
	if planned.Image != req.Image {
		t.Fatalf("image = %q, want %q", planned.Image, req.Image)
	}
	if planned.JenkinsJob != "" || planned.PipelineStatus != "" {
		t.Fatalf("image flow should not create pipeline metadata: job=%q status=%q", planned.JenkinsJob, planned.PipelineStatus)
	}
}

func TestApplyComponentDeployVersionReplacesExistingImageTag(t *testing.T) {
	comp := model.Component{
		Image:         "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
		Version:       "v1.2.3",
		RegistryImage: "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:v1.2.3",
	}

	updated := applyComponentDeployVersion(comp, "17")

	wantImage := "registry.shop-dev.corp.example.com:5443/shop-dev/orders-api:17"
	if updated.Image != wantImage {
		t.Fatalf("image = %q, want %q", updated.Image, wantImage)
	}
	if updated.Version != "17" {
		t.Fatalf("version = %q, want 17", updated.Version)
	}
	if updated.RegistryImage != wantImage {
		t.Fatalf("registry image = %q, want %q", updated.RegistryImage, wantImage)
	}
	if updated.PipelineStatus != "built" {
		t.Fatalf("pipeline status = %q, want built", updated.PipelineStatus)
	}
}

func TestApplyComponentDeployVersionRecomputesSourceRegistryHost(t *testing.T) {
	t.Setenv(svcservice.RegistryHostTemplateEnv, "registry.paap.local:5000")
	comp := model.Component{
		Name:          "source-smoke",
		DeliveryMode:  "source",
		Image:         "registry.test-staging.paap.local:5000/test-staging/source-smoke:manual",
		Version:       "manual",
		RegistryImage: "registry.test-staging.paap.local:5000/test-staging/source-smoke:manual",
	}

	updated := applyComponentDeployVersionForRuntimeRegistry(
		model.Application{Identifier: "test"},
		model.Environment{Identifier: "staging"},
		comp,
		"source-smoke",
		"manual",
		"registry",
	)

	wantImage := "registry.paap.local:5000/test-staging/source-smoke:manual"
	if updated.Image != wantImage {
		t.Fatalf("image = %q, want %q", updated.Image, wantImage)
	}
	if updated.RegistryImage != wantImage {
		t.Fatalf("registry image = %q, want %q", updated.RegistryImage, wantImage)
	}
	if updated.PipelineStatus != "built" {
		t.Fatalf("pipeline status = %q, want built", updated.PipelineStatus)
	}
}

func TestAdoptResourceDiscoversAndCreatesDraftFromRealWorkload(t *testing.T) {
	previousDB := database.DB
	previousClient := k8s.GetClient()
	t.Cleanup(func() {
		database.DB = previousDB
		k8s.SetClient(previousClient)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Application{}, &model.Environment{}, &model.Component{}, &model.ServiceInstallation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	database.DB = db

	app := model.Application{Name: "Billing", Identifier: "billing", OwnerID: 1}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app: %v", err)
	}
	env := model.Environment{ApplicationID: app.ID, Name: "Dev", Identifier: "dev", Namespace: "billing-dev", Status: "running"}
	if err := db.Create(&env).Error; err != nil {
		t.Fatalf("create env: %v", err)
	}
	if err := db.Create(&model.Component{EnvironmentID: env.ID, Name: "managed-api", Type: "backend"}).Error; err != nil {
		t.Fatalf("create managed component: %v", err)
	}
	if err := db.Create(&model.ServiceInstallation{EnvironmentID: env.ID, ServiceType: "deploy", Status: "running", Namespace: "billing-dev-argocd"}).Error; err != nil {
		t.Fatalf("create service installation: %v", err)
	}

	k8s.SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "api-config", Namespace: "billing-dev"}, Data: map[string]string{"FEATURE_FLAG": "on", "application.yml": "server:\n  port: 8080\n"}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "api-secret", Namespace: "billing-dev"}, Data: map[string][]byte{"PASSWORD": []byte("secret")}},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "external-api", Namespace: "billing-dev"},
			Spec: appsv1.DeploymentSpec{
				Replicas: handlerInt32Ptr(2),
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:    "api",
					Image:   "registry.local/billing/external-api:v7",
					Command: []string{"/server"},
					Args:    []string{"--listen=:8080"},
					Env: []corev1.EnvVar{
						{Name: "FEATURE_FLAG", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "api-config"}, Key: "FEATURE_FLAG"}}},
						{Name: "PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "api-secret"}, Key: "PASSWORD"}}},
					},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "api-config-file",
						MountPath: "/etc/app",
						ReadOnly:  true,
					}},
				}},
					Volumes: []corev1.Volume{{
						Name: "api-config-file",
						VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "api-config"},
							Items: []corev1.KeyToPath{{
								Key:  "application.yml",
								Path: "application.yml",
							}},
						}},
					}},
				}},
			},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "other-api", Namespace: "other-dev"},
			Spec: appsv1.DeploymentSpec{
				Replicas: handlerInt32Ptr(1),
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: "registry.local/other:v1"}}}},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "billing-dev-argocd-server", Namespace: "billing-dev-argocd"},
			Spec: appsv1.DeploymentSpec{
				Replicas: handlerInt32Ptr(1),
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "argocd-server", Image: "quay.io/argoproj/argocd:v2.13.1"}}}},
			},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
		},
	).Build())

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/environments/:id/adoptable-resources", ListAdoptableResources)
	router.POST("/api/v1/environments/:id/adoptable-resources", AdoptResource)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/environments/1/adoptable-resources", nil)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRec.Code, listRec.Body.String())
	}
	var listBody struct {
		Data []k8s.AdoptableResource `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listBody.Data) != 1 || listBody.Data[0].Key != "billing-dev/deployment/external-api" {
		t.Fatalf("unexpected adoptable list: %#v", listBody.Data)
	}

	body, _ := json.Marshal(AdoptResourceRequest{Key: "billing-dev/deployment/external-api"})
	adoptReq := httptest.NewRequest(http.MethodPost, "/api/v1/environments/1/adoptable-resources", bytes.NewReader(body))
	adoptReq.Header.Set("Content-Type", "application/json")
	adoptRec := httptest.NewRecorder()
	router.ServeHTTP(adoptRec, adoptReq)
	if adoptRec.Code != http.StatusCreated {
		t.Fatalf("adopt status = %d, body=%s", adoptRec.Code, adoptRec.Body.String())
	}
	var saved model.Component
	if err := db.Where("name = ?", "external-api").First(&saved).Error; err != nil {
		t.Fatalf("load adopted component: %v", err)
	}
	if saved.Status != "draft" || saved.Image != "registry.local/billing/external-api:v7" || saved.Version != "v7" || saved.Replicas != 2 {
		t.Fatalf("unexpected adopted component: %#v", saved)
	}
	cfg, err := model.ParseComponentConfig(saved.Config)
	if err != nil {
		t.Fatalf("parse adopted config: %v", err)
	}
	if len(cfg.Env) != 2 || cfg.Env[0].ConfigMapName != "api-config" || cfg.Env[1].SecretName != "api-secret" {
		t.Fatalf("adopted env refs not preserved: %#v", cfg.Env)
	}
	if len(cfg.Files) != 1 || cfg.Files[0].ConfigMapName != "api-config" || cfg.Files[0].Key != "application.yml" || cfg.Files[0].MountPath != "/etc/app/application.yml" {
		t.Fatalf("adopted config file mounts not preserved: %#v", cfg.Files)
	}
	if len(cfg.Command) != 1 || cfg.Command[0] != "/server" || len(cfg.Args) != 1 || cfg.Args[0] != "--listen=:8080" {
		t.Fatalf("adopted command/args not preserved: %#v", cfg)
	}
}

func TestDatabaseTableContextResourcesPreserveFullDatabaseCatalog(t *testing.T) {
	resources := appendDatabaseTableResources(
		databaseCatalogResources([]string{"information_schema", "mysql", "sys"}),
		"mysql",
		[]svcservice.DatabaseTable{{Name: "user", Type: "BASE TABLE"}, {Name: "db", Type: "BASE TABLE"}},
	)
	var databaseNames []string
	var tableNames []string
	var tableAnnotations map[string]interface{}
	var tableActionCount int
	for _, resource := range resources {
		if resource.Type == "Database" {
			databaseNames = append(databaseNames, resource.Name)
		}
		if resource.Type == "Table" {
			tableNames = append(tableNames, resource.Name)
			tableAnnotations = resource.Annotations
			tableActionCount = len(resource.Actions)
		}
	}
	if strings.Join(databaseNames, ",") != "information_schema,mysql,sys" {
		t.Fatalf("database catalog must stay stable after drilling into a table, got %#v", databaseNames)
	}
	if strings.Join(tableNames, ",") != "user,db" {
		t.Fatalf("table catalog must stay stable after previewing one table, got %#v", tableNames)
	}
	if tableAnnotations["database"] != "mysql" {
		t.Fatalf("table must keep database annotation, got %#v", tableAnnotations)
	}
	if tableActionCount == 0 {
		t.Fatalf("table context must keep preview/columns actions")
	}
}
