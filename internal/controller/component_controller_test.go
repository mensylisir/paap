package controller

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	paapv1 "paap/api/v1"
)

type deleteNotFoundClient struct {
	client.Client
	target types.NamespacedName
}

func (c deleteNotFoundClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if _, ok := obj.(*appsv1.Deployment); ok {
		key := types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
		if key == c.target {
			return apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, obj.GetName())
		}
	}
	return c.Client.Delete(ctx, obj, opts...)
}

func TestComponentReconcileUsesIdentifierDeploymentNameForArgoCDManagedComponents(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	replicas := int32(1)
	comp := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: "staging-backend-3", Namespace: "paap-app-test"},
		Spec: paapv1.ComponentSpec{
			Name:       "订单服务",
			Identifier: "backend-3",
			Type:       "backend",
			ManagedBy:  "argocd",
			Deployment: paapv1.DeploymentSpec{
				Namespace: "test-staging",
				Image:     "registry",
				Tag:       "2.8.3",
				Replicas:  replicas,
			},
			Service: &paapv1.ServiceSpec{Port: 80, TargetPort: 8080, Type: "ClusterIP"},
		},
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-3", Namespace: "test-staging"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend-3"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "backend-3"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "backend-3", Image: "registry:2.8.3"}}},
			},
		},
		Status: appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1, UpdatedReplicas: 1},
	}

	r := &ComponentReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(comp, deploy).
			WithStatusSubresource(&paapv1.Component{}, &appsv1.Deployment{}).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(comp)}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	got := &paapv1.Component{}
	if err := r.Get(context.Background(), client.ObjectKeyFromObject(comp), got); err != nil {
		t.Fatalf("get component: %v", err)
	}
	if got.Status.Deployment == nil || got.Status.Deployment.Name != "backend-3" {
		t.Fatalf("expected argocd deployment status for backend-3, got %#v", got.Status.Deployment)
	}
	if got.Status.Phase != "Running" {
		t.Fatalf("expected Running phase, got %q", got.Status.Phase)
	}
}

func TestComponentReconcileDoesNotMutateArgoCDManagedService(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	replicas := int32(1)
	comp := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "staging-frontend-1",
			Namespace: "paap-app-test",
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
		Spec: paapv1.ComponentSpec{
			Name:       "frontend-1",
			Identifier: "frontend-1",
			Type:       "frontend",
			ManagedBy:  "argocd",
			Deployment: paapv1.DeploymentSpec{
				Namespace: "test-staging",
				Image:     "nginx",
				Tag:       "alpine",
				Replicas:  replicas,
			},
			Service: &paapv1.ServiceSpec{Port: 80, TargetPort: 80, Type: "NodePort"},
		},
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-1", Namespace: "test-staging"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend-1"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "frontend-1"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "frontend-1", Image: "nginx:alpine"}}},
			},
		},
		Status: appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1, UpdatedReplicas: 1},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend-1",
			Namespace: "test-staging",
			Labels: map[string]string{
				"app":                "frontend-1",
				"paap.io/managed-by": "argocd",
			},
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeNodePort,
			Selector: map[string]string{"app": "frontend-1"},
			Ports:    []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromInt(80)}},
		},
	}

	r := &ComponentReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(comp, deploy, svc).
			WithStatusSubresource(&paapv1.Component{}, &appsv1.Deployment{}).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(comp)}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	gotSvc := &corev1.Service{}
	if err := r.Get(context.Background(), client.ObjectKey{Name: "frontend-1", Namespace: "test-staging"}, gotSvc); err != nil {
		t.Fatalf("get service: %v", err)
	}
	if gotSvc.Labels["paap.io/managed-by"] != "argocd" {
		t.Fatalf("argocd managed service label was mutated: %#v", gotSvc.Labels)
	}
	if _, exists := gotSvc.Annotations["paap.io/managed-by"]; exists {
		t.Fatalf("argocd managed service annotations were mutated: %#v", gotSvc.Annotations)
	}
}

func TestComponentDeletionRemovesArgoCDManagedDeploymentName(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	replicas := int32(1)
	now := metav1.Now()
	comp := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "staging-backend-3",
			Namespace:         "paap-app-test",
			Finalizers:        []string{compFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: paapv1.ComponentSpec{
			Identifier: "backend-3",
			ManagedBy:  "argocd",
			Deployment: paapv1.DeploymentSpec{
				Namespace: "test-staging",
				Replicas:  replicas,
			},
			Service: &paapv1.ServiceSpec{Port: 80, TargetPort: 8080},
		},
	}
	controllerutil.AddFinalizer(comp, compFinalizer)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-3", Namespace: "test-staging"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "backend-3"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "backend-3"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "backend-3", Image: "registry:2.8.3"}}},
			},
		},
	}
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "backend-3", Namespace: "test-staging"}}
	orphanDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "historical-backend-3",
			Namespace: "test-staging",
			Labels:    map[string]string{"paap.io/component": "backend-3"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"paap.io/component": "backend-3"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"paap.io/component": "backend-3"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "backend-3", Image: "registry:2.8.3"}}},
			},
		},
	}
	orphanRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-3-rs", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": "backend-3"}},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"paap.io/component": "backend-3"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"paap.io/component": "backend-3"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "backend-3", Image: "registry:2.8.3"}}},
			},
		},
	}
	orphanPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-3-pod", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": "backend-3"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "backend-3", Image: "registry:2.8.3"}}},
	}
	orphanSvc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "historical-backend-3", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": "backend-3"}}}
	orphanConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-3-config", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": "backend-3"}},
		Data:       map[string]string{"application.yaml": "server.port: 8080"},
	}
	orphanSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-3-secret", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": "backend-3"}},
		StringData: map[string]string{"DB_PASSWORD": "secret"},
	}
	sharedConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-config", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": "other"}},
		Data:       map[string]string{"key": "value"},
	}
	sharedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-secret", Namespace: "test-staging", Labels: map[string]string{"paap.io/component": "other"}},
		StringData: map[string]string{"key": "value"},
	}

	r := &ComponentReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(comp, deploy, svc, orphanDeploy, orphanRS, orphanPod, orphanSvc, orphanConfig, orphanSecret, sharedConfig, sharedSecret).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.handleDeletion(context.Background(), comp); err != nil {
		t.Fatalf("handle deletion: %v", err)
	}

	gotDeploy := &appsv1.Deployment{}
	if err := r.Get(context.Background(), client.ObjectKey{Name: "backend-3", Namespace: "test-staging"}, gotDeploy); !apierrors.IsNotFound(err) {
		t.Fatalf("expected argocd deployment to be deleted, got %v", err)
	}
	gotSvc := &corev1.Service{}
	if err := r.Get(context.Background(), client.ObjectKey{Name: "backend-3", Namespace: "test-staging"}, gotSvc); !apierrors.IsNotFound(err) {
		t.Fatalf("expected service to be deleted, got %v", err)
	}
	for _, item := range []client.Object{orphanDeploy, orphanRS, orphanPod, orphanSvc, orphanConfig, orphanSecret} {
		err := r.Get(context.Background(), client.ObjectKeyFromObject(item), item)
		if !apierrors.IsNotFound(err) {
			t.Fatalf("expected labeled orphan %T/%s to be deleted, got %v", item, item.GetName(), err)
		}
	}
	for _, item := range []client.Object{sharedConfig, sharedSecret} {
		err := r.Get(context.Background(), client.ObjectKeyFromObject(item), item)
		if err != nil {
			t.Fatalf("expected unrelated %T/%s to remain, got %v", item, item.GetName(), err)
		}
	}
}

func TestComponentDeletionKeepsUnownedGeneratedConfigNames(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	now := metav1.Now()
	comp := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "staging-backend-3",
			Namespace:         "paap-app-test",
			Finalizers:        []string{compFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: paapv1.ComponentSpec{
			Identifier: "backend-3",
			ManagedBy:  "argocd",
			Deployment: paapv1.DeploymentSpec{
				Namespace: "test-staging",
				Env: []paapv1.EnvVar{
					{
						Name: "SPRING_CONFIG_IMPORT",
						ValueFrom: &paapv1.EnvVarSource{
							ConfigMapKeyRef: &paapv1.ConfigMapKeySelector{Name: "backend-3-config", Key: "application.yaml"},
						},
					},
					{
						Name: "DB_PASSWORD",
						ValueFrom: &paapv1.EnvVarSource{
							SecretKeyRef: &paapv1.SecretKeySelector{Name: "backend-3-secret", Key: "password"},
						},
					},
				},
			},
		},
	}
	config := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-3-config", Namespace: "test-staging"},
		Data:       map[string]string{"application.yaml": "server.port: 8080"},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-3-secret", Namespace: "test-staging"},
		StringData: map[string]string{"password": "secret"},
	}

	r := &ComponentReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(comp, config, secret).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.handleDeletion(context.Background(), comp); err != nil {
		t.Fatalf("handle deletion: %v", err)
	}

	for _, item := range []client.Object{config, secret} {
		err := r.Get(context.Background(), client.ObjectKeyFromObject(item), item)
		if err != nil {
			t.Fatalf("expected unowned %T/%s to remain, got %v", item, item.GetName(), err)
		}
	}
}

func TestComponentDeletionRemovesArgoCDApplicationRef(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	now := metav1.Now()
	comp := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "staging-backend-3",
			Namespace:         "paap-app-test",
			Finalizers:        []string{compFinalizer},
			DeletionTimestamp: &now,
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
		Spec: paapv1.ComponentSpec{
			Identifier:     "backend-3",
			ManagedBy:      "argocd",
			ArgoCDAppRef:   &paapv1.ObjectReference{Name: "test-staging-backend-3"},
			EnvironmentRef: paapv1.ObjectReference{Name: "staging"},
			Deployment: paapv1.DeploymentSpec{
				Namespace: "test-staging",
			},
		},
	}
	app := &unstructured.Unstructured{Object: map[string]interface{}{}}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	app.SetName("test-staging-backend-3")
	app.SetNamespace("test-staging-deploy")

	r := &ComponentReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(comp, app).
			Build(),
		Scheme: scheme,
	}

	if _, err := r.handleDeletion(context.Background(), comp); err != nil {
		t.Fatalf("handle deletion: %v", err)
	}

	gotApp := &unstructured.Unstructured{}
	gotApp.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	if err := r.Get(context.Background(), client.ObjectKey{Name: "test-staging-backend-3", Namespace: "test-staging-deploy"}, gotApp); !apierrors.IsNotFound(err) {
		t.Fatalf("expected argocd application to be deleted, got %v", err)
	}
}

func TestComponentDeletionIgnoresDeploymentNotFoundRace(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := paapv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add paap scheme: %v", err)
	}

	replicas := int32(1)
	now := metav1.Now()
	comp := &paapv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "staging-frontend-1",
			Namespace:         "paap-app-test",
			Finalizers:        []string{compFinalizer},
			DeletionTimestamp: &now,
			Labels: map[string]string{
				"paap.io/app": "test",
				"paap.io/env": "staging",
			},
		},
		Spec: paapv1.ComponentSpec{
			Identifier: "frontend-1",
			ManagedBy:  "argocd",
			Deployment: paapv1.DeploymentSpec{
				Namespace: "test-staging",
				Replicas:  replicas,
			},
			Service: &paapv1.ServiceSpec{Port: 80, TargetPort: 80, Type: "NodePort"},
		},
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-1", Namespace: "test-staging"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend-1"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "frontend-1"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "frontend-1", Image: "nginx:alpine"}}},
			},
		},
	}
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(comp, deploy).
		Build()
	r := &ComponentReconciler{
		Client: deleteNotFoundClient{
			Client: baseClient,
			target: types.NamespacedName{Name: "frontend-1", Namespace: "test-staging"},
		},
		Scheme: scheme,
	}

	if _, err := r.handleDeletion(context.Background(), comp); err != nil {
		t.Fatalf("handle deletion should ignore deployment NotFound race, got %v", err)
	}
}
