package helm

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/storage/driver"
)

var releaseLocks sync.Map

const releaseHistoryLimit = 5

type Client struct {
	settings *cli.EnvSettings
}

type ResourceMetadata struct {
	Namespace   string
	Labels      map[string]string
	Annotations map[string]string
}

type metadataPostRenderer struct {
	metadata ResourceMetadata
}

func NewClient() *Client {
	settings := cli.New()
	settings.RepositoryCache = "/tmp/.helm/cache"
	settings.RepositoryConfig = "/tmp/.helm/repositories.yaml"
	settings.RegistryConfig = "/tmp/.helm/registry/config.json"
	return &Client{settings: settings}
}

func (c *Client) UpgradeInstall(releaseName, namespace, chartPath string, values map[string]interface{}) error {
	return c.UpgradeInstallWithMetadata(releaseName, namespace, chartPath, values, ResourceMetadata{})
}

func (c *Client) UpgradeInstallWithMetadata(releaseName, namespace, chartPath string, values map[string]interface{}, metadata ResourceMetadata) error {
	if releaseName == "" {
		return fmt.Errorf("releaseName is required")
	}
	if namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if chartPath == "" {
		return fmt.Errorf("chart path is required")
	}
	if err := os.MkdirAll("/tmp/.helm/cache", 0755); err != nil {
		return err
	}
	if err := os.MkdirAll("/tmp/.helm/registry", 0755); err != nil {
		return err
	}

	unlock := lockRelease(namespace, releaseName)
	defer unlock()

	cfg := new(action.Configuration)
	if err := cfg.Init(c.settings.RESTClientGetter(), namespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {}); err != nil {
		return err
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return err
	}

	hist := action.NewHistory(cfg)
	hist.Max = 1
	if _, err := hist.Run(releaseName); err == nil {
		return runUpgrade(cfg, releaseName, namespace, chart, values, metadata)
	} else if !errors.Is(err, driver.ErrReleaseNotFound) {
		return err
	}

	install := action.NewInstall(cfg)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.CreateNamespace = true
	install.Wait = false
	install.PostRenderer = metadataPostRenderer{metadata: metadata}
	_, err = install.Run(chart, values)
	if IsReleaseAlreadyExists(err) {
		return runUpgrade(cfg, releaseName, namespace, chart, values, metadata)
	}
	return err
}

func runUpgrade(cfg *action.Configuration, releaseName, namespace string, loadedChart *chart.Chart, values map[string]interface{}, metadata ResourceMetadata) error {
	upgrade := action.NewUpgrade(cfg)
	configureUpgrade(upgrade)
	upgrade.Namespace = namespace
	upgrade.Install = true
	upgrade.Wait = false
	upgrade.PostRenderer = metadataPostRenderer{metadata: metadata}
	_, err := upgrade.Run(releaseName, loadedChart, values)
	return err
}

func configureUpgrade(upgrade *action.Upgrade) {
	upgrade.MaxHistory = releaseHistoryLimit
}

func IsReleaseAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, driver.ErrReleaseExists) || strings.Contains(err.Error(), "release: already exists")
}

func IsReleaseOperationInProgress(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "another operation (install/upgrade/rollback) is in progress")
}

func (c *Client) RenderTemplate(releaseName, namespace, chartPath string, values map[string]interface{}) (string, error) {
	if releaseName == "" {
		return "", fmt.Errorf("releaseName is required")
	}
	if namespace == "" {
		return "", fmt.Errorf("namespace is required")
	}
	if chartPath == "" {
		return "", fmt.Errorf("chart path is required")
	}
	cfg := new(action.Configuration)
	if err := cfg.Init(c.settings.RESTClientGetter(), namespace, "memory", func(format string, v ...interface{}) {}); err != nil {
		return "", err
	}
	install := action.NewInstall(cfg)
	install.ReleaseName = releaseName
	install.Namespace = namespace
	install.DryRun = true
	install.ClientOnly = true
	install.IncludeCRDs = true
	chart, err := loader.Load(chartPath)
	if err != nil {
		return "", err
	}
	release, err := install.Run(chart, values)
	if err != nil {
		return "", err
	}
	return release.Manifest, nil
}

func lockRelease(namespace, releaseName string) func() {
	key := namespace + "/" + releaseName
	lockValue, _ := releaseLocks.LoadOrStore(key, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

func (r metadataPostRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	if len(r.metadata.Labels) == 0 && len(r.metadata.Annotations) == 0 {
		return renderedManifests, nil
	}
	output, err := applyResourceMetadata(renderedManifests.Bytes(), r.metadata)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(output), nil
}

func applyResourceMetadata(input []byte, metadata ResourceMetadata) ([]byte, error) {
	docs := strings.Split(string(input), "\n---")
	rendered := make([]string, 0, len(docs))
	for _, doc := range docs {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		obj := make(map[string]interface{})
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			return nil, err
		}
		if len(obj) == 0 {
			continue
		}

		mergeMetadata(obj, metadata)
		setNamespaceIfNamespaced(obj, metadata.Namespace)
		mergePodTemplateMetadata(obj, metadata)

		data, err := yaml.Marshal(obj)
		if err != nil {
			return nil, err
		}
		rendered = append(rendered, strings.TrimSpace(string(data)))
	}
	return []byte(strings.Join(rendered, "\n---\n") + "\n"), nil
}

func setNamespaceIfNamespaced(obj map[string]interface{}, namespace string) {
	if namespace == "" || isClusterScoped(obj) {
		return
	}
	meta := ensureMap(obj, "metadata")
	if _, exists := meta["namespace"]; !exists {
		meta["namespace"] = namespace
	}
}

func isClusterScoped(obj map[string]interface{}) bool {
	kind, _ := obj["kind"].(string)
	apiVersion, _ := obj["apiVersion"].(string)
	if apiVersion == "v1" && kind == "Namespace" {
		return true
	}
	switch kind {
	case "ClusterRole", "ClusterRoleBinding", "CustomResourceDefinition", "StorageClass", "PersistentVolume", "Node", "MutatingWebhookConfiguration", "ValidatingWebhookConfiguration":
		return true
	default:
		return false
	}
}

func mergeMetadata(obj map[string]interface{}, metadata ResourceMetadata) {
	meta := ensureMap(obj, "metadata")
	labels := ensureMap(meta, "labels")
	for k, v := range metadata.Labels {
		labels[k] = v
	}
	annotations := ensureMap(meta, "annotations")
	for k, v := range metadata.Annotations {
		annotations[k] = v
	}
}

func mergePodTemplateMetadata(obj map[string]interface{}, metadata ResourceMetadata) {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return
	}
	template, ok := spec["template"].(map[string]interface{})
	if !ok {
		return
	}
	meta := ensureMap(template, "metadata")
	labels := ensureMap(meta, "labels")
	for k, v := range metadata.Labels {
		labels[k] = v
	}
	annotations := ensureMap(meta, "annotations")
	for k, v := range metadata.Annotations {
		annotations[k] = v
	}
}

func ensureMap(parent map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := parent[key].(map[string]interface{}); ok {
		return existing
	}
	next := make(map[string]interface{})
	parent[key] = next
	return next
}

func BuildValues(presetValues string, flatValues map[string]string) (map[string]interface{}, error) {
	values := make(map[string]interface{})
	if presetValues != "" {
		if err := yaml.Unmarshal([]byte(presetValues), &values); err != nil {
			return nil, err
		}
	}
	for k, v := range flatValues {
		setNestedValue(values, k, convertValue(v))
	}
	return values, nil
}

func setNestedValue(m map[string]interface{}, key string, value interface{}) {
	keys := strings.Split(key, ".")
	current := m
	for i, k := range keys {
		if i == len(keys)-1 {
			current[k] = value
			return
		}
		next, ok := current[k].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			current[k] = next
		}
		current = next
	}
}

func convertValue(value string) interface{} {
	if value == "true" {
		return true
	}
	if value == "false" {
		return false
	}
	if intVal, err := strconv.Atoi(value); err == nil {
		return intVal
	}
	if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
		return floatVal
	}
	return value
}
