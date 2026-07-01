package service

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"paap/internal/model"
)

func TestParsePlatformAddonDirReadsMetadataReadmeAndManifests(t *testing.T) {
	dir := writePlatformAddonDir(t, "keda", map[string]string{
		"addon.yaml": `name: keda
displayName: KEDA
namespace: keda
version: v2.17.0
installMode: manifest
s3Key: platform-addons/keda.tar.gz
dependsOn:
  - metrics-server
capabilities:
  - event-driven-autoscaling
checks:
  crds:
    - scaledobjects.keda.sh
  deployments:
    - namespace: keda
      name: keda-operator
description: Event-driven autoscaling.
`,
		"README.md": "# KEDA\n\n事件驱动伸缩能力。",
		"manifests/00-namespace.yaml": `apiVersion: v1
kind: Namespace
metadata:
  name: keda
`,
		"manifests/10-operator.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: keda-operator
  namespace: keda
spec:
  selector:
    matchLabels:
      app: keda-operator
  template:
    metadata:
      labels:
        app: keda-operator
    spec:
      containers:
        - name: operator
          image: ghcr.io/kedacore/keda:2.17.0
`,
	})

	pkg, err := ParsePlatformAddonDir(dir)
	if err != nil {
		t.Fatalf("parse dir: %v", err)
	}
	if pkg.Spec.Name != "keda" || pkg.Spec.DisplayName != "KEDA" || pkg.Spec.S3Key != "platform-addons/keda.tar.gz" {
		t.Fatalf("unexpected spec: %#v", pkg.Spec)
	}
	if len(pkg.Spec.DependsOn) != 1 || pkg.Spec.DependsOn[0] != "metrics-server" {
		t.Fatalf("dependencies = %#v", pkg.Spec.DependsOn)
	}
	if !strings.Contains(pkg.Readme, "事件驱动伸缩能力") {
		t.Fatalf("readme = %q", pkg.Readme)
	}
	if len(pkg.Manifests) != 2 || !strings.Contains(pkg.Manifests[0], "kind: Namespace") || !strings.Contains(pkg.Manifests[1], "kind: Deployment") {
		t.Fatalf("manifests = %#v", pkg.Manifests)
	}
}

func TestParsePlatformAddonArchiveUsesSamePackageFormatForCustomUploads(t *testing.T) {
	archivePath := writePlatformAddonArchive(t, map[string]string{
		"addon.yaml": `name: custom-scaler
displayName: Custom Scaler
namespace: custom-scaler
version: v1.0.0
installMode: manifest
s3Key: platform-addons/custom/custom-scaler.tar.gz
description: Custom uploaded scaler.
`,
		"README.md": "# Custom Scaler\n\n自定义上传插件。",
		"manifests/install.yaml": `apiVersion: v1
kind: Namespace
metadata:
  name: custom-scaler
`,
	})

	pkg, err := ParsePlatformAddonArchive(archivePath)
	if err != nil {
		t.Fatalf("parse archive: %v", err)
	}
	if pkg.Spec.Name != "custom-scaler" || pkg.Spec.S3Key != "platform-addons/custom/custom-scaler.tar.gz" {
		t.Fatalf("unexpected custom package: %#v", pkg.Spec)
	}
	if !strings.Contains(pkg.Readme, "自定义上传插件") {
		t.Fatalf("readme = %q", pkg.Readme)
	}
}

func TestClusterAddonFromPackageKeepsBuiltinAndCustomOnSameModel(t *testing.T) {
	pkg := PlatformAddonPackage{
		Spec: PlatformAddonArchiveSpec{
			Name:         "custom-scaler",
			DisplayName:  "Custom Scaler",
			Namespace:    "custom-scaler",
			Version:      "v1.0.0",
			InstallMode:  "manifest",
			S3Key:        "platform-addons/custom/custom-scaler.tar.gz",
			Capabilities: []string{"custom-scale"},
			Description:  "Custom uploaded scaler.",
		},
		Readme: "# Custom Scaler",
	}
	addon := ClusterAddonFromPackage(pkg, model.PlatformAddonSourceCustom)
	if addon.Name != "custom-scaler" || addon.Source != model.PlatformAddonSourceCustom {
		t.Fatalf("addon identity = %#v", addon)
	}
	if addon.DesiredState != model.PlatformAddonDesiredDisabled || addon.Status != model.PlatformAddonStatusUnknown {
		t.Fatalf("initial lifecycle state = desired:%q status:%q", addon.DesiredState, addon.Status)
	}
	if !strings.Contains(addon.Capabilities, "custom-scale") {
		t.Fatalf("capabilities json = %q", addon.Capabilities)
	}
}

func writePlatformAddonDir(t *testing.T, name string, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), name)
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return root
}

func writePlatformAddonArchive(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "addon.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		data := []byte(body)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatalf("write file %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return path
}
