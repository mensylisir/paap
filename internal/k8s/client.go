package k8s

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"gopkg.in/yaml.v3"
)

// Client wraps kubectl/helm commands to operate on the local kind cluster.
type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func run(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)

	// Set Helm cache directory to /tmp to avoid permission issues
	if name == "helm" {
		cmd.Env = append(os.Environ(),
			"HELM_CACHE_HOME=/tmp/.helm/cache",
			"HELM_CONFIG_HOME=/tmp/.helm/config",
			"HELM_DATA_HOME=/tmp/.helm/data",
		)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("%s %s failed: %w, output: %s", name, strings.Join(args, " "), err, string(out))
	}
	return out, nil
}

// --- Namespace ---

// EnsureNamespace creates a namespace if not exists
func (c *Client) EnsureNamespace(name string) error {
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Namespace
metadata:
  name: %s
  labels:
    paap.managed: "true"
`, name)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	_, err := cmd.CombinedOutput()
	return err
}

// DeleteNamespace removes namespace and everything in it
func (c *Client) DeleteNamespace(name string) error {
	_, err := run(30*time.Second, "kubectl", "delete", "namespace", name, "--ignore-not-found=true")
	return err
}

// --- RBAC ---

// EnsureServiceAccount creates SA if not exists
func (c *Client) EnsureServiceAccount(name, namespace string) error {
	cmd := exec.Command("kubectl", "create", "serviceaccount", name, "--namespace", namespace, "--dry-run=client", "-o", "yaml")
	yaml, _ := cmd.Output()
	cmd2 := exec.Command("kubectl", "apply", "-f", "-")
	cmd2.Stdin = bytes.NewReader(yaml)
	_, err := cmd2.CombinedOutput()
	return err
}

// ApplyRole applies a Role YAML to a namespace
func (c *Client) ApplyRole(namespace string, roleYAML string) error {
	cmd := exec.Command("kubectl", "apply", "-n", namespace, "-f", "-")
	cmd.Stdin = strings.NewReader(roleYAML)
	_, err := cmd.CombinedOutput()
	return err
}

// BindRole binds a SA to a Role
func (c *Client) BindRole(bindingName, namespace, roleName, saName, saNamespace string) error {
	yaml := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %s
  namespace: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: %s
subjects:
  - kind: ServiceAccount
    name: %s
    namespace: %s
`, bindingName, namespace, roleName, saName, saNamespace)
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	_, err := cmd.CombinedOutput()
	return err
}

// --- Tool Installations ---

// InstallArgoCD installs a namespace-scoped ArgoCD
func (c *Client) InstallArgoCD(namespace string) error {
	// Use argocd namespace-install manifest
	_, err := run(120*time.Second, "kubectl", "apply", "-n", namespace, "-f",
		"https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/namespace-install.yaml")
	return err
}

// InstallTekton installs Tekton Pipelines
func (c *Client) InstallTekton(namespace string) error {
	_, err := run(120*time.Second, "kubectl", "apply", "-n", namespace, "-f",
		"https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml")
	return err
}

// EnsureArgoCDCRDs 删除所有 ArgoCD CRDs，让新的 Helm release 可以重新创建并拥有它们。
// CRDs 是集群级资源，只能被一个 Helm release 拥有，所以每次安装新环境时都需要先清理。
func (c *Client) EnsureArgoCDCRDs() error {
	client := GetClient()
	if client == nil {
		return fmt.Errorf("k8s client not initialized")
	}

	ctx := context.Background()
	apiextClient := apiextensionsclientset.NewForConfigOrDie(ctrl.GetConfigOrDie())

	argocdCRDs := []string{
		"applications.argoproj.io",
		"appprojects.argoproj.io",
		"applicationsets.argoproj.io",
	}

	for _, crdName := range argocdCRDs {
		_, err := apiextClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, metav1.GetOptions{})
		if err != nil {
			// CRD 不存在，无需处理
			continue
		}

		// 删除 CRD（无论是否有 Helm 标签），让新 release 重新创建并拥有
		if err := apiextClient.ApiextensionsV1().CustomResourceDefinitions().Delete(ctx, crdName, metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to delete CRD %s: %w", crdName, err)
		}
		log.Printf("[EnsureArgoCDCRDs] Deleted CRD %s", crdName)
	}

	return nil
}

// InstallPrometheus installs Prometheus+Grafana stack
func (c *Client) InstallPrometheus(namespace string) error {
	// Use prometheus-operator (simple deployment)
	_, err := run(120*time.Second, "kubectl", "apply", "-n", namespace, "-f",
		"https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml")
	return err
}

// --- Helm Infrastructure ---

func (c *Client) helm(args ...string) ([]byte, error) {
	return run(120*time.Second, "helm", args...)
}

// AddHelmRepo adds a helm repo
func (c *Client) AddHelmRepo(name, url string) error {
	_, err := c.helm("repo", "add", name, url)
	if err != nil {
		// repo may already exist, ignore
		return nil
	}
	_, _ = c.helm("repo", "update")
	return nil
}

// InstallHelmChart installs a chart using a values file to avoid --set-string comma splitting issues
func (c *Client) InstallHelmChart(releaseName, namespace, chart string, setValues map[string]string) error {
	// 写入临时 values 文件，避免 --set-string 对逗号的错误解析
	tmpFile, err := os.CreateTemp("", "paap-values-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp values file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// 构建嵌套的 values map，支持点号表示法
	nestedValues := make(map[string]interface{})
	for k, v := range setValues {
		setNestedValue(nestedValues, k, v)
	}

	// 写入 YAML 格式的 values
	encoder := yaml.NewEncoder(tmpFile)
	if err := encoder.Encode(nestedValues); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to encode values: %w", err)
	}
	encoder.Close()
	tmpFile.Close()

	args := []string{"upgrade", "--install", releaseName, chart, "--namespace", namespace, "--create-namespace", "--values", tmpFile.Name()}
	output, err := c.helm(args...)
	if err != nil {
		// Log the full output for debugging
		log.Printf("[InstallHelmChart] Helm install failed for %s: %v\nFull output: %s", releaseName, err, string(output))
		return err
	}
	return nil
}

// setNestedValue sets a value in a nested map using dot notation (e.g., "image.tag" -> map["image"]["tag"])
func setNestedValue(m map[string]interface{}, key, value string) {
	keys := strings.Split(key, ".")
	current := m
	for i, k := range keys {
		if i == len(keys)-1 {
			// Last key, set the value with proper type conversion
			current[k] = convertValue(value)
		} else {
			// Intermediate key, ensure it's a map
			if _, exists := current[k]; !exists {
				current[k] = make(map[string]interface{})
			}
			if next, ok := current[k].(map[string]interface{}); ok {
				current = next
			} else {
				// Key exists but is not a map, overwrite it
				current[k] = make(map[string]interface{})
				current = current[k].(map[string]interface{})
			}
		}
	}
}

// convertValue converts string values to appropriate types (bool, int, float, string)
func convertValue(value string) interface{} {
	// Try boolean
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	// Try integer
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}
	// Try float
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}
	// Default to string
	return value
}

// DeleteHelmRelease uninstalls a helm release
func (c *Client) DeleteHelmRelease(releaseName, namespace string) error {
	_, err := c.helm("uninstall", releaseName, "--namespace", namespace)
	return err
}

// InstallToolViaHelm is a generic function for installing any tool via Helm chart.
// It handles repo addition, update, and chart installation with version pinning.
func (c *Client) InstallToolViaHelm(releaseName, namespace, chartRepo, chartName, chartVersion string, setValues map[string]string) error {
	if chartRepo == "" || chartName == "" {
		return fmt.Errorf("chartRepo and chartName are required for helm install")
	}

	// Add and update the Helm repo
	repoName := extractRepoName(chartRepo)
	if err := c.AddHelmRepo(repoName, chartRepo); err != nil {
		return fmt.Errorf("failed to add helm repo %s: %w", chartRepo, err)
	}

	// Build chart reference: repo/chartName:version
	chart := repoName + "/" + chartName
	if chartVersion != "" {
		chart += ":" + chartVersion
	}

	return c.InstallHelmChart(releaseName, namespace, chart, setValues)
}

// extractRepoName derives a short repo name from a URL.
// e.g. "https://charts.bitnami.com/bitnami" → "bitnami"
// e.g. "https://helm.goharbor.io" → "goharbor"
func extractRepoName(repoURL string) string {
	// Remove trailing slash
	url := strings.TrimRight(repoURL, "/")
	// Split by / and take the last segment
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "custom"
}

// --- Infra Installers ---

func (c *Client) InstallPostgreSQL(namespace, releaseName string) error {
	if err := c.AddHelmRepo("bitnami", "https://charts.bitnami.com/bitnami"); err != nil {
		return err
	}
	return c.InstallHelmChart(releaseName, namespace, "bitnami/postgresql", map[string]string{
		"auth.username": "appuser",
		"auth.password": "changeme123",
		"auth.database": "appdb",
	})
}

func (c *Client) InstallRedis(namespace, releaseName string) error {
	if err := c.AddHelmRepo("bitnami", "https://charts.bitnami.com/bitnami"); err != nil {
		return err
	}
	return c.InstallHelmChart(releaseName, namespace, "bitnami/redis", map[string]string{
		"auth.enabled": "false",
	})
}

func (c *Client) InstallRabbitMQ(namespace, releaseName string) error {
	if err := c.AddHelmRepo("bitnami", "https://charts.bitnami.com/bitnami"); err != nil {
		return err
	}
	return c.InstallHelmChart(releaseName, namespace, "bitnami/rabbitmq", map[string]string{
		"auth.username": "user",
		"auth.password": "changeme123",
	})
}

func (c *Client) InstallMinIO(namespace, releaseName string) error {
	if err := c.AddHelmRepo("bitnami", "https://charts.bitnami.com/bitnami"); err != nil {
		return err
	}
	return c.InstallHelmChart(releaseName, namespace, "bitnami/minio", map[string]string{
		"auth.rootUser": "minioadmin",
		"auth.rootPassword": "minioadmin123",
	})
}

func (c *Client) InstallNacos(namespace, releaseName string) error {
	if err := c.AddHelmRepo("bitnami", "https://charts.bitnami.com/bitnami"); err != nil {
		return err
	}
	return c.InstallHelmChart(releaseName, namespace, "bitnami/nacos", map[string]string{
		"auth.enabled": "false",
	})
}

// --- Component Deployment ---

// DeployComponent creates a simple Deployment+Service for a component
func (c *Client) DeployComponent(name, namespace, image string, replicas int, memory string) error {
	yaml := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
    paap.managed: "true"
spec:
  replicas: %d
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
        paap.managed: "true"
    spec:
      containers:
        - name: app
          image: %s
          ports:
            - containerPort: 8080
          resources:
            requests:
              memory: "%s"
---
apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    app: %s
  ports:
    - port: 80
      targetPort: 8080
`, name, namespace, name, replicas, name, name, image, memory,
		name, namespace, name)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(yaml)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deploy failed: %w, output: %s", err, string(out))
	}
	return nil
}

func (c *Client) DeleteComponent(name, namespace string) error {
	_, _ = run(30*time.Second, "kubectl", "delete", "deployment", name, "-n", namespace, "--ignore-not-found=true")
	_, err := run(30*time.Second, "kubectl", "delete", "service", name, "-n", namespace, "--ignore-not-found=true")
	return err
}

func (c *Client) ScaleDeployment(name, namespace string, replicas int) error {
	_, err := run(30*time.Second, "kubectl", "scale", "deployment", name, "--namespace", namespace, "--replicas", fmt.Sprintf("%d", replicas))
	return err
}

func (c *Client) GetPodStatus(namespace string) (string, error) {
	out, err := run(30*time.Second, "kubectl", "get", "pods", "-n", namespace, "--selector=paap.managed=true", "-o", "wide")
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}
