package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
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

// InstallHelmChart installs a chart
func (c *Client) InstallHelmChart(releaseName, namespace, chart string, setValues map[string]string) error {
	args := []string{"upgrade", "--install", releaseName, chart, "--namespace", namespace, "--create-namespace"}
	for k, v := range setValues {
		args = append(args, "--set", fmt.Sprintf("%s=%s", k, v))
	}
	_, err := c.helm(args...)
	return err
}

// DeleteHelmRelease uninstalls a helm release
func (c *Client) DeleteHelmRelease(releaseName, namespace string) error {
	_, err := c.helm("uninstall", releaseName, "--namespace", namespace)
	return err
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
