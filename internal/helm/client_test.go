package helm

import (
	"errors"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/storage/driver"
)

func TestApplyResourceMetadataAddsLabelsAndAnnotationsToEachDocument(t *testing.T) {
	input := `apiVersion: v1
kind: Service
metadata:
  name: registry
  labels:
    app.kubernetes.io/name: registry
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
spec:
  template:
    metadata:
      labels:
        app.kubernetes.io/name: registry
`

	output, err := applyResourceMetadata([]byte(input), ResourceMetadata{
		Labels: map[string]string{
			"paap.io/app":               "myapp",
			"paap.io/env":               "dev",
			"paap.io/service":           "registry",
			"paap.io/service-namespace": "myapp-dev-registry",
		},
		Annotations: map[string]string{
			"paap.io/tool-namespace": "myapp-dev-registry",
		},
	})
	if err != nil {
		t.Fatalf("applyResourceMetadata returned error: %v", err)
	}

	docs := strings.Split(string(output), "---")
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs, got %d:\n%s", len(docs), output)
	}
	for _, doc := range docs {
		var obj map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			t.Fatalf("failed to unmarshal output doc: %v\n%s", err, doc)
		}
		metadata := obj["metadata"].(map[string]interface{})
		labels := metadata["labels"].(map[string]interface{})
		if labels["paap.io/app"] != "myapp" || labels["paap.io/service-namespace"] != "myapp-dev-registry" {
			t.Fatalf("missing PAAP labels: %#v", labels)
		}
		annotations := metadata["annotations"].(map[string]interface{})
		if annotations["paap.io/tool-namespace"] != "myapp-dev-registry" {
			t.Fatalf("missing PAAP annotations: %#v", annotations)
		}
	}
}

func TestApplyResourceMetadataSetsNamespaceForNamespacedResources(t *testing.T) {
	input := `apiVersion: v1
kind: Service
metadata:
  name: registry
`

	output, err := applyResourceMetadata([]byte(input), ResourceMetadata{
		Namespace: "myapp-dev-git",
	})
	if err != nil {
		t.Fatalf("applyResourceMetadata returned error: %v", err)
	}

	var obj map[string]interface{}
	if err := yaml.Unmarshal(output, &obj); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}
	metadata := obj["metadata"].(map[string]interface{})
	if metadata["namespace"] != "myapp-dev-git" {
		t.Fatalf("expected namespace to be set, got %#v", metadata["namespace"])
	}
}

func TestBuildValuesPreservesDotKeysForNestedHelmValues(t *testing.T) {
	values, err := BuildValues("", map[string]string{
		"global.paap.envNamespaces": "test-staging,test-staging-app",
		"paap.envNamespaces":        "test-staging,test-staging-app",
	})
	if err != nil {
		t.Fatalf("BuildValues returned error: %v", err)
	}

	global, ok := values["global"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected global map, got %#v", values["global"])
	}
	paap, ok := global["paap"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected global.paap map, got %#v", global["paap"])
	}
	if got := paap["envNamespaces"]; got != "test-staging,test-staging-app" {
		t.Fatalf("global.paap.envNamespaces = %#v, want %q", got, "test-staging,test-staging-app")
	}

	topPaap, ok := values["paap"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected paap map, got %#v", values["paap"])
	}
	if got := topPaap["envNamespaces"]; got != "test-staging,test-staging-app" {
		t.Fatalf("paap.envNamespaces = %#v, want %q", got, "test-staging,test-staging-app")
	}
}

func TestConfigureUpgradeLimitsReleaseHistory(t *testing.T) {
	upgrade := &action.Upgrade{}

	configureUpgrade(upgrade)

	if upgrade.MaxHistory != releaseHistoryLimit {
		t.Fatalf("MaxHistory = %d, want %d", upgrade.MaxHistory, releaseHistoryLimit)
	}
}

func TestIsReleaseAlreadyExistsRecognizesHelmDriverError(t *testing.T) {
	if !IsReleaseAlreadyExists(driver.ErrReleaseExists) {
		t.Fatalf("expected driver.ErrReleaseExists to be recognized")
	}
	if !IsReleaseAlreadyExists(errors.New("release: already exists")) {
		t.Fatalf("expected Helm release exists message to be recognized")
	}
}

func TestIsReleaseOperationInProgressRecognizesHelmPendingError(t *testing.T) {
	err := errors.New("another operation (install/upgrade/rollback) is in progress")
	if !IsReleaseOperationInProgress(err) {
		t.Fatalf("expected Helm pending operation message to be recognized")
	}
}
