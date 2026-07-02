package k8s

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PlatformAddonNamespacedCheck struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Name      string `json:"name" yaml:"name"`
}

type PlatformAddonCheckSpec struct {
	CRDs        []string                       `json:"crds,omitempty" yaml:"crds,omitempty"`
	Deployments []PlatformAddonNamespacedCheck `json:"deployments,omitempty" yaml:"deployments,omitempty"`
	DaemonSets  []PlatformAddonNamespacedCheck `json:"daemonSets,omitempty" yaml:"daemonSets,omitempty"`
}

type PlatformAddonCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type PlatformAddonStatusResult struct {
	Status     string                   `json:"status"`
	Conditions []PlatformAddonCondition `json:"conditions"`
}

func ApplyPlatformAddonManifests(ctx context.Context, manifests []string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	objects, err := platformAddonManifestObjects(manifests)
	if err != nil {
		return err
	}

	// Separate CRDs from other objects. CRDs must be applied and reach Established
	// status before creating any resources whose Kind depends on them, because
	// apiserver registers new CRD types asynchronously.
	var crds []*unstructured.Unstructured
	var others []*unstructured.Unstructured
	for _, obj := range objects {
		if obj.GetKind() == "CustomResourceDefinition" {
			crds = append(crds, obj)
		} else {
			others = append(others, obj)
		}
	}

	for _, obj := range crds {
		if err := upsertPlatformAddonObject(ctx, cl, obj); err != nil {
			return fmt.Errorf("apply CRD %s: %w", obj.GetName(), err)
		}
	}

	for _, obj := range crds {
		name := obj.GetName()
		if err := waitForCRDEstablished(ctx, cl, name, 60*time.Second); err != nil {
			return fmt.Errorf("wait for CRD %s: %w", name, err)
		}
	}

	for _, obj := range others {
		if err := upsertPlatformAddonObject(ctx, cl, obj); err != nil {
			return err
		}
	}
	return nil
}

func waitForCRDEstablished(ctx context.Context, cl client.Client, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		var crd apiextensionsv1.CustomResourceDefinition
		if err := cl.Get(ctx, types.NamespacedName{Name: name}, &crd); err != nil {
			return err
		}
		for _, c := range crd.Status.Conditions {
			if c.Type == apiextensionsv1.Established && c.Status == apiextensionsv1.ConditionTrue {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout after %v waiting for CRD %q to become Established", timeout, name)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func DeletePlatformAddonManifests(ctx context.Context, manifests []string) error {
	cl, err := requireClient()
	if err != nil {
		return err
	}
	objects, err := platformAddonManifestObjects(manifests)
	if err != nil {
		return err
	}
	// Collect namespace names before deletion so we can clean up later.
	managedNamespaces := make(map[string]bool)
	for _, obj := range objects {
		if obj.GetKind() == "Namespace" && obj.GetName() != "" {
			managedNamespaces[obj.GetName()] = true
		}
	}
	for i := len(objects) - 1; i >= 0; i-- {
		obj := objects[i]
		if err := cl.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete %s %s/%s: %w", obj.GetKind(), obj.GetNamespace(), obj.GetName(), err)
		}
	}
	if len(managedNamespaces) > 0 {
		cleanupOrphanedAPIServices(ctx, cl, managedNamespaces)
	}
	return nil
}

var apiServiceGVR = schema.GroupVersionResource{
	Group:    "apiregistration.k8s.io",
	Version:  "v1",
	Resource: "apiservices",
}

func cleanupOrphanedAPIServices(ctx context.Context, cl client.Client, namespaces map[string]bool) {
	var list unstructured.UnstructuredList
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiregistration.k8s.io",
		Version: "v1",
		Kind:    "APIService",
	})
	if err := cl.List(ctx, &list); err != nil {
		return
	}
	for i := range list.Items {
		item := list.Items[i]
		svc, found, err := unstructured.NestedMap(item.Object, "spec", "service")
		if err != nil || !found {
			continue
		}
		ns, _ := svc["namespace"].(string)
		if ns == "" || !namespaces[ns] {
			continue
		}
		name, _ := svc["name"].(string)
		if name == "" {
			continue
		}
		var svcObj unstructured.Unstructured
		svcObj.SetAPIVersion("v1")
		svcObj.SetKind("Service")
		if err := cl.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, &svcObj); err == nil {
			continue
		}
		if !apierrors.IsNotFound(err) {
			continue
		}
		_ = cl.Delete(ctx, &item)
	}
}

func platformAddonManifestObjects(manifests []string) ([]*unstructured.Unstructured, error) {
	objects := make([]*unstructured.Unstructured, 0)
	for _, manifest := range manifests {
		decoder := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
		for {
			var raw map[string]interface{}
			err := decoder.Decode(&raw)
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("decode platform addon manifest: %w", err)
			}
			if len(raw) == 0 {
				continue
			}
			obj := &unstructured.Unstructured{Object: raw}
			if obj.GetAPIVersion() == "" || obj.GetKind() == "" || obj.GetName() == "" {
				return nil, fmt.Errorf("platform addon manifest must declare apiVersion, kind and metadata.name")
			}
			objects = append(objects, obj)
		}
	}
	return objects, nil
}

func upsertPlatformAddonObject(ctx context.Context, cl client.Client, desired *unstructured.Unstructured) error {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	if err := cl.Get(ctx, key, existing); err != nil {
		if apierrors.IsNotFound(err) {
			return cl.Create(ctx, desired)
		}
		return fmt.Errorf("get %s %s/%s: %w", desired.GetKind(), desired.GetNamespace(), desired.GetName(), err)
	}
	desired.SetResourceVersion(existing.GetResourceVersion())
	return cl.Update(ctx, desired)
}

func CheckPlatformAddonStatus(ctx context.Context, spec PlatformAddonCheckSpec) PlatformAddonStatusResult {
	cl, err := requireClient()
	if err != nil {
		return PlatformAddonStatusResult{
			Status:     "unknown",
			Conditions: []PlatformAddonCondition{{Type: "ClientReady", Status: "False", Message: err.Error()}},
		}
	}
	conditions := make([]PlatformAddonCondition, 0, len(spec.CRDs)+len(spec.Deployments)+len(spec.DaemonSets))
	for _, name := range spec.CRDs {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		condition := PlatformAddonCondition{Type: "CRDReady", Status: "True", Message: name + " found"}
		var crd apiextensionsv1.CustomResourceDefinition
		if err := cl.Get(ctx, types.NamespacedName{Name: name}, &crd); err != nil {
			condition.Status = "False"
			condition.Message = name + " not found"
		}
		conditions = append(conditions, condition)
	}
	for _, item := range spec.Deployments {
		conditions = append(conditions, checkAddonDeployment(ctx, cl, item))
	}
	for _, item := range spec.DaemonSets {
		conditions = append(conditions, checkAddonDaemonSet(ctx, cl, item))
	}
	return PlatformAddonStatusResult{Status: aggregateAddonStatus(conditions), Conditions: conditions}
}

func checkAddonDeployment(ctx context.Context, cl client.Client, item PlatformAddonNamespacedCheck) PlatformAddonCondition {
	condition := PlatformAddonCondition{
		Type:    "DeploymentReady",
		Status:  "True",
		Message: fmt.Sprintf("%s/%s available", item.Namespace, item.Name),
	}
	var deploy appsv1.Deployment
	if err := cl.Get(ctx, types.NamespacedName{Name: item.Name, Namespace: item.Namespace}, &deploy); err != nil {
		condition.Status = "False"
		condition.Message = fmt.Sprintf("%s/%s not found", item.Namespace, item.Name)
		return condition
	}
	if deploy.Status.AvailableReplicas <= 0 {
		condition.Status = "False"
		condition.Message = fmt.Sprintf("%s/%s has no available replicas", item.Namespace, item.Name)
	}
	return condition
}

func checkAddonDaemonSet(ctx context.Context, cl client.Client, item PlatformAddonNamespacedCheck) PlatformAddonCondition {
	condition := PlatformAddonCondition{
		Type:    "DaemonSetReady",
		Status:  "True",
		Message: fmt.Sprintf("%s/%s ready", item.Namespace, item.Name),
	}
	var ds appsv1.DaemonSet
	if err := cl.Get(ctx, types.NamespacedName{Name: item.Name, Namespace: item.Namespace}, &ds); err != nil {
		condition.Status = "False"
		condition.Message = fmt.Sprintf("%s/%s not found", item.Namespace, item.Name)
		return condition
	}
	if ds.Status.DesiredNumberScheduled > 0 && ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		condition.Status = "False"
		condition.Message = fmt.Sprintf("%s/%s ready %d/%d", item.Namespace, item.Name, ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
	}
	return condition
}

func aggregateAddonStatus(conditions []PlatformAddonCondition) string {
	if len(conditions) == 0 {
		return "unknown"
	}
	failed := 0
	for _, condition := range conditions {
		if condition.Status != "True" {
			failed++
		}
	}
	switch {
	case failed == 0:
		return "available"
	case failed == len(conditions):
		return "unavailable"
	default:
		return "degraded"
	}
}

func platformAddonGVK(apiVersion, kind string) schema.GroupVersionKind {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionKind{Version: apiVersion, Kind: kind}
	}
	return gv.WithKind(kind)
}
