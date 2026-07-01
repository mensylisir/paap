package k8s

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplyPlatformAddonManifestsCreatesObjects(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })
	SetClient(fake.NewClientBuilder().WithScheme(scheme).Build())

	err := ApplyPlatformAddonManifests(context.Background(), []string{`apiVersion: v1
kind: Namespace
metadata:
  name: platform-tools
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: plugin-settings
  namespace: platform-tools
data:
  enabled: "true"
`})
	if err != nil {
		t.Fatalf("apply manifests: %v", err)
	}

	var ns corev1.Namespace
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "platform-tools"}, &ns); err != nil {
		t.Fatalf("namespace was not applied: %v", err)
	}
	var cm corev1.ConfigMap
	if err := GetClient().Get(context.Background(), types.NamespacedName{Name: "plugin-settings", Namespace: "platform-tools"}, &cm); err != nil {
		t.Fatalf("configmap was not applied: %v", err)
	}
	if cm.Data["enabled"] != "true" {
		t.Fatalf("configmap data = %#v", cm.Data)
	}
}

func TestDeletePlatformAddonManifestsDeletesObjects(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })
	SetClient(fake.NewClientBuilder().WithScheme(scheme).Build())

	manifest := `apiVersion: v1
kind: Namespace
metadata:
  name: platform-tools
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: plugin-settings
  namespace: platform-tools
data:
  enabled: "true"
`
	if err := ApplyPlatformAddonManifests(context.Background(), []string{manifest}); err != nil {
		t.Fatalf("apply manifests: %v", err)
	}
	if err := DeletePlatformAddonManifests(context.Background(), []string{manifest}); err != nil {
		t.Fatalf("delete manifests: %v", err)
	}

	var cm corev1.ConfigMap
	err := GetClient().Get(context.Background(), types.NamespacedName{Name: "plugin-settings", Namespace: "platform-tools"}, &cm)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("configmap should be deleted, got %v", err)
	}
	var ns corev1.Namespace
	err = GetClient().Get(context.Background(), types.NamespacedName{Name: "platform-tools"}, &ns)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("namespace should be deleted, got %v", err)
	}
}

func TestCheckPlatformAddonStatusReportsAvailableWhenChecksPass(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiextensions scheme: %v", err)
	}
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })
	SetClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "scaledobjects.keda.sh"}},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "keda-operator", Namespace: "keda"},
			Status:     appsv1.DeploymentStatus{AvailableReplicas: 1},
		},
	).Build())

	status := CheckPlatformAddonStatus(context.Background(), PlatformAddonCheckSpec{
		CRDs: []string{"scaledobjects.keda.sh"},
		Deployments: []PlatformAddonNamespacedCheck{
			{Namespace: "keda", Name: "keda-operator"},
		},
	})

	if status.Status != "available" {
		t.Fatalf("status = %q, want available; conditions=%#v", status.Status, status.Conditions)
	}
	if len(status.Conditions) != 2 {
		t.Fatalf("conditions = %#v, want CRD and Deployment checks", status.Conditions)
	}
	for _, condition := range status.Conditions {
		if condition.Status != "True" {
			t.Fatalf("condition = %#v, want true", condition)
		}
	}
}
