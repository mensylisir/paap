package k8s

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func int32Ptr(value int32) *int32 { return &value }

func TestListNamespaceRuntimeResources(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "billing-dev-redis"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "redis-0", Namespace: "billing-dev-redis"}, Status: corev1.PodStatus{Phase: corev1.PodRunning}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "redis-data", Namespace: "billing-dev-redis"}, Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound}},
	).Build())

	resources, err := ListNamespaceRuntimeResources(t.Context(), "billing-dev-redis")
	if err != nil {
		t.Fatalf("list runtime resources: %v", err)
	}
	if len(resources) != 3 {
		t.Fatalf("expected service, pod and pvc, got %#v", resources)
	}
	if resources[1].Name != "redis-0" || resources[1].Status != "Running" {
		t.Fatalf("unexpected pod resource: %#v", resources[1])
	}
}

func TestListNamespaceAdoptableResourcesDiscoversRealWorkloads(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "api-config", Namespace: "billing-dev"},
			Data:       map[string]string{"FEATURE_FLAG": "on", "application.yml": "server:\n  port: 8080\n"},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "api-secret", Namespace: "billing-dev"},
			Data:       map[string][]byte{"PASSWORD": []byte("secret"), "tls.key": []byte("secret-key")},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing-dev"},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(2),
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:    "api",
					Image:   "registry.local/billing/api:v1",
					Command: []string{"/app/server"},
					Args:    []string{"--port=8080"},
					Ports:   []corev1.ContainerPort{{ContainerPort: 8000}},
					Env: []corev1.EnvVar{
						{Name: "PLAIN", Value: "value"},
						{Name: "FEATURE_FLAG", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "api-config"}, Key: "FEATURE_FLAG"}}},
						{Name: "PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "api-secret"}, Key: "PASSWORD"}}},
					},
					EnvFrom: []corev1.EnvFromSource{
						{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "api-config"}}},
						{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "api-secret"}}},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "api-config-file", MountPath: "/etc/app", ReadOnly: true},
						{Name: "api-secret-file", MountPath: "/etc/tls", ReadOnly: true},
					},
				}},
					Volumes: []corev1.Volume{
						{Name: "api-config-file", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{Name: "api-config"},
							Items: []corev1.KeyToPath{{
								Key:  "application.yml",
								Path: "application.yml",
							}},
						}}},
						{Name: "api-secret-file", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
							SecretName: "api-secret",
							Items: []corev1.KeyToPath{{
								Key:  "tls.key",
								Path: "tls.key",
							}},
						}}},
					},
				}},
			},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
		},
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: "postgresql", Namespace: "billing-dev-postgresql"},
			Spec: appsv1.StatefulSetSpec{
				Replicas: int32Ptr(1),
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "postgresql", Image: "postgres:16"}}}},
			},
			Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
		},
	).Build())

	resources, err := ListNamespaceAdoptableResources(t.Context(), "billing-dev")
	if err != nil {
		t.Fatalf("list adoptable resources: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected only billing-dev resources, got %#v", resources)
	}
	got := resources[0]
	if got.Key != "billing-dev/deployment/api" || got.Name != "api" || got.Kind != "Deployment" || got.ComponentType != "backend" {
		t.Fatalf("unexpected adoptable resource identity: %#v", got)
	}
	if got.Status != "running" || got.RuntimeConfig == nil || got.RuntimeConfig.Image != "registry.local/billing/api:v1" {
		t.Fatalf("unexpected runtime config: %#v", got)
	}
	if len(got.RuntimeConfig.Env) != 3 || len(got.RuntimeConfig.ConfigMaps) != 1 || len(got.RuntimeConfig.Secrets) != 1 {
		t.Fatalf("runtime env/config refs not discovered: %#v", got.RuntimeConfig)
	}
	if len(got.RuntimeConfig.EnvFrom) != 2 {
		t.Fatalf("runtime envFrom refs not discovered: %#v", got.RuntimeConfig.EnvFrom)
	}
	if len(got.RuntimeConfig.Files) != 2 {
		t.Fatalf("runtime mounted config files not discovered: %#v", got.RuntimeConfig.Files)
	}
	if got.RuntimeConfig.Files[0].Kind != "configMap" || got.RuntimeConfig.Files[0].ObjectName != "api-config" || got.RuntimeConfig.Files[0].Key != "application.yml" || got.RuntimeConfig.Files[0].MountPath != "/etc/app/application.yml" {
		t.Fatalf("unexpected configmap file mount: %#v", got.RuntimeConfig.Files[0])
	}
	if got.RuntimeConfig.Files[1].Kind != "secret" || got.RuntimeConfig.Files[1].ObjectName != "api-secret" || got.RuntimeConfig.Files[1].Key != "tls.key" || got.RuntimeConfig.Files[1].MountPath != "/etc/tls/tls.key" {
		t.Fatalf("unexpected secret file mount: %#v", got.RuntimeConfig.Files[1])
	}
	if got.RuntimeConfig.Command[0] != "/app/server" || got.RuntimeConfig.Args[0] != "--port=8080" {
		t.Fatalf("command/args not discovered: %#v", got.RuntimeConfig)
	}
	if len(got.RuntimeConfig.Ports) != 1 || got.RuntimeConfig.Ports[0] != 8000 {
		t.Fatalf("container ports not discovered: %#v", got.RuntimeConfig.Ports)
	}
}

func TestListNamespaceExternalEndpointsDoesNotInventClusterIP(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing-dev"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80},
				},
			},
		},
	).Build())

	endpoints, err := ListNamespaceExternalEndpoints(t.Context(), "billing-dev")
	if err != nil {
		t.Fatalf("list external endpoints: %v", err)
	}
	if len(endpoints) != 0 {
		t.Fatalf("ClusterIP service must not produce external endpoints, got %#v", endpoints)
	}
}

func TestListNamespaceExternalEndpointsFindsNodePortAndIngress(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := networkingv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add networking scheme: %v", err)
	}

	pathType := networkingv1.PathTypePrefix
	SetClient(fake.NewClientBuilder().WithScheme(testScheme).WithObjects(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "kind-control-plane"},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "172.18.0.2"},
			}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing-dev"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80, NodePort: 30080},
				},
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing-dev"},
			Spec: networkingv1.IngressSpec{
				TLS: []networkingv1.IngressTLS{{Hosts: []string{"api.example.test"}}},
				Rules: []networkingv1.IngressRule{{
					Host: "api.example.test",
					IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/",
							PathType: &pathType,
						}},
					}},
				}},
			},
		},
	).Build())

	endpoints, err := ListNamespaceExternalEndpoints(t.Context(), "billing-dev")
	if err != nil {
		t.Fatalf("list external endpoints: %v", err)
	}
	got := map[string]bool{}
	for _, endpoint := range endpoints {
		got[endpoint.URL] = true
	}
	for _, want := range []string{"http://172.18.0.2:30080", "https://api.example.test/"} {
		if !got[want] {
			t.Fatalf("missing endpoint %s, got %#v", want, endpoints)
		}
	}
}

func TestSetNamespaceServiceExternalAccessPatchesRedisMasterServiceOnly(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "billing-dev-redis-headless",
				Namespace: "billing-dev-redis",
				Labels: map[string]string{
					"app.kubernetes.io/instance": "billing-dev-redis",
					"app.kubernetes.io/name":     "redis",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Ports:     []corev1.ServicePort{{Name: "redis", Port: 6379}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "billing-dev-redis-master",
				Namespace: "billing-dev-redis",
				Labels: map[string]string{
					"app.kubernetes.io/component": "master",
					"app.kubernetes.io/instance":  "billing-dev-redis",
					"app.kubernetes.io/name":      "redis",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.96.0.10",
				Ports:     []corev1.ServicePort{{Name: "redis", Port: 6379}},
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "billing-dev-redis-replicas",
				Namespace: "billing-dev-redis",
				Labels: map[string]string{
					"app.kubernetes.io/component": "replica",
					"app.kubernetes.io/instance":  "billing-dev-redis",
					"app.kubernetes.io/name":      "redis",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.96.0.11",
				Ports:     []corev1.ServicePort{{Name: "redis", Port: 6379}},
			},
		},
	).Build())

	updated, err := SetNamespaceServiceExternalAccess(t.Context(), "billing-dev-redis", "redis", true)
	if err != nil {
		t.Fatalf("enable external access: %v", err)
	}
	if updated.Name != "billing-dev-redis-master" || updated.Spec.Type != corev1.ServiceTypeNodePort {
		t.Fatalf("expected redis master NodePort, got %s/%s", updated.Name, updated.Spec.Type)
	}

	cl := GetClient()
	replica := &corev1.Service{}
	if err := cl.Get(t.Context(), types.NamespacedName{Namespace: "billing-dev-redis", Name: "billing-dev-redis-replicas"}, replica); err != nil {
		t.Fatalf("get replica service: %v", err)
	}
	if replica.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Fatalf("replica service must stay internal, got %s", replica.Spec.Type)
	}

	updated.Spec.Ports[0].NodePort = 30379
	if err := cl.Update(t.Context(), updated); err != nil {
		t.Fatalf("seed nodeport: %v", err)
	}
	disabled, err := SetNamespaceServiceExternalAccess(t.Context(), "billing-dev-redis", "redis", false)
	if err != nil {
		t.Fatalf("disable external access: %v", err)
	}
	if disabled.Spec.Type != corev1.ServiceTypeClusterIP || disabled.Spec.Ports[0].NodePort != 0 {
		t.Fatalf("expected ClusterIP with cleared nodePort, got type=%s nodePort=%d", disabled.Spec.Type, disabled.Spec.Ports[0].NodePort)
	}
}

func TestListNamespaceExternalEndpointsSkipsSSHAndDetectsHTTPSPorts(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	SetClient(fake.NewClientBuilder().WithObjects(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "kind-control-plane"},
			Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeInternalIP, Address: "172.18.0.2"},
			}},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "tool", Namespace: "billing-dev-tool"},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
				Ports: []corev1.ServicePort{
					{Name: "http", Port: 80, NodePort: 31767},
					{Name: "https", Port: 443, NodePort: 30969},
					{Name: "ssh", Port: 22, NodePort: 32748},
				},
			},
		},
	).Build())

	endpoints, err := ListNamespaceExternalEndpoints(t.Context(), "billing-dev-tool")
	if err != nil {
		t.Fatalf("list external endpoints: %v", err)
	}
	got := map[string]bool{}
	for _, endpoint := range endpoints {
		got[endpoint.URL] = true
	}
	for _, want := range []string{"http://172.18.0.2:31767", "https://172.18.0.2:30969"} {
		if !got[want] {
			t.Fatalf("missing endpoint %s, got %#v", want, endpoints)
		}
	}
	if got["http://172.18.0.2:32748"] {
		t.Fatalf("ssh nodePort must not be exposed as an HTTP endpoint, got %#v", endpoints)
	}
}

func TestListNamespaceExternalEndpointsPrefersGatewayIngressBeforeNodePort(t *testing.T) {
	previous := GetClient()
	t.Cleanup(func() { SetClient(previous) })

	testScheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(testScheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := networkingv1.AddToScheme(testScheme); err != nil {
		t.Fatalf("add networking scheme: %v", err)
	}

	gateway := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "Gateway",
		"metadata": map[string]interface{}{
			"name":      "public-gateway",
			"namespace": "billing-dev",
		},
		"spec": map[string]interface{}{
			"listeners": []interface{}{map[string]interface{}{
				"name":     "https",
				"hostname": "api.gateway.test",
				"protocol": "HTTPS",
				"port":     int64(443),
			}},
		},
	}}
	gateway.SetGroupVersionKind(schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "Gateway"})
	httpRoute := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1",
		"kind":       "HTTPRoute",
		"metadata": map[string]interface{}{
			"name":      "api-route",
			"namespace": "billing-dev",
		},
		"spec": map[string]interface{}{
			"parentRefs": []interface{}{map[string]interface{}{"name": "public-gateway", "sectionName": "https"}},
			"hostnames":  []interface{}{"api.gateway.test"},
			"rules": []interface{}{map[string]interface{}{
				"matches": []interface{}{map[string]interface{}{
					"path": map[string]interface{}{"type": "PathPrefix", "value": "/v1"},
				}},
			}},
		},
	}}
	httpRoute.SetGroupVersionKind(schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "HTTPRoute"})

	pathType := networkingv1.PathTypePrefix
	SetClient(fake.NewClientBuilder().WithScheme(testScheme).WithRuntimeObjects(
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
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing-dev"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "api.ingress.test",
					IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{Path: "/", PathType: &pathType}},
					}},
				}},
			},
		},
		gateway,
		httpRoute,
	).Build())

	endpoints, err := ListNamespaceExternalEndpoints(t.Context(), "billing-dev")
	if err != nil {
		t.Fatalf("list external endpoints: %v", err)
	}
	if len(endpoints) < 3 {
		t.Fatalf("expected gateway, ingress and nodeport endpoints, got %#v", endpoints)
	}
	if endpoints[0].Kind != "Gateway" || endpoints[0].URL != "https://api.gateway.test/v1" {
		t.Fatalf("gateway endpoint should be first, got %#v", endpoints)
	}
	if endpoints[1].Kind != "Ingress" || endpoints[1].URL != "http://api.ingress.test/" {
		t.Fatalf("ingress endpoint should be second, got %#v", endpoints)
	}
	if endpoints[2].Kind != "NodePort" || endpoints[2].URL != "http://172.18.0.2:30080" {
		t.Fatalf("nodeport endpoint should be after gateway and ingress, got %#v", endpoints)
	}
}
